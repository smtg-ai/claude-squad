package app

import (
	"claude-squad/config"
	"claude-squad/keys"
	"claude-squad/log"
	"claude-squad/session"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const GlobalInstanceLimit = 10

// Run is the main entrypoint into the application.
func Run(ctx context.Context, program string, autoYes bool) error {
	p := tea.NewProgram(
		newHome(ctx, program, autoYes),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // Mouse scroll
	)
	_, err := p.Run()
	return err
}

type state int

const (
	stateDefault state = iota
	// stateNew is the state when the user is creating a new instance.
	stateNew
	// statePrompt is the state when the user is entering a prompt.
	statePrompt
	// stateHelp is the state when a help screen is displayed.
	stateHelp
	// stateConfirm is the state when a confirmation modal is displayed.
	stateConfirm
	// stateNewTopic is the state when the user is creating a new topic.
	stateNewTopic
	// stateNewTopicConfirm is the state when confirming shared worktree for a new topic.
	stateNewTopicConfirm
	// stateSearch is the state when the user is searching topics/instances.
	stateSearch
	// stateMoveTo is the state when the user is moving an instance to a topic.
	stateMoveTo
	// stateContextMenu is the state when a right-click context menu is shown.
	stateContextMenu
)

type home struct {
	ctx context.Context

	// -- Storage and Configuration --

	program string
	autoYes bool

	// storage is the interface for saving/loading data to/from the app's state
	storage *session.Storage
	// appConfig stores persistent application configuration
	appConfig *config.Config
	// appState stores persistent application state like seen help screens
	appState config.AppState

	// -- State --

	// state is the current discrete state of the application
	state state
	// newInstanceFinalizer is called when the state is stateNew and then you press enter.
	// It registers the new instance in the list after the instance has been started.
	newInstanceFinalizer func()

	// promptAfterName tracks if we should enter prompt mode after naming
	promptAfterName bool

	// keySent is used to manage underlining menu items
	keySent bool

	// -- UI Components --

	// list displays the list of instances
	list *ui.List
	// menu displays the bottom menu
	menu *ui.Menu
	// tabbedWindow displays the tabbed window with preview and diff panes
	tabbedWindow *ui.TabbedWindow
	// errBox displays error messages
	errBox *ui.ErrBox
	// global spinner instance. we plumb this down to where it's needed
	spinner spinner.Model
	// textInputOverlay handles text input with state
	textInputOverlay *overlay.TextInputOverlay
	// textOverlay displays text information
	textOverlay *overlay.TextOverlay
	// confirmationOverlay displays confirmation modals
	confirmationOverlay *overlay.ConfirmationOverlay

	// sidebar displays the topic sidebar
	sidebar *ui.Sidebar
	// topics is the list of all topics
	topics []*session.Topic
	// focusedPanel tracks which panel has keyboard focus (0=sidebar, 1=instance list)
	focusedPanel int
	// pendingTopicName stores the topic name during the two-step creation flow
	pendingTopicName string

	// contextMenu is the right-click context menu overlay
	contextMenu *overlay.ContextMenu
	// pickerOverlay is the topic picker overlay for move-to-topic
	pickerOverlay *overlay.PickerOverlay

	// Layout dimensions for mouse hit-testing
	sidebarWidth  int
	listWidth     int
	contentHeight int
}

func newHome(ctx context.Context, program string, autoYes bool) *home {
	// Load application config
	appConfig := config.LoadConfig()

	// Load application state
	appState := config.LoadState()

	// Initialize storage
	storage, err := session.NewStorage(appState)
	if err != nil {
		fmt.Printf("Failed to initialize storage: %v\n", err)
		os.Exit(1)
	}

	h := &home{
		ctx:          ctx,
		spinner:      spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		menu:         ui.NewMenu(),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane()),
		errBox:       ui.NewErrBox(),
		storage:      storage,
		appConfig:    appConfig,
		program:      program,
		autoYes:      autoYes,
		state:        stateDefault,
		appState:     appState,
	}
	h.list = ui.NewList(&h.spinner, autoYes)
	h.sidebar = ui.NewSidebar()
	h.setFocus(1) // Start with instance list focused

	// Load saved instances
	instances, err := storage.LoadInstances()
	if err != nil {
		fmt.Printf("Failed to load instances: %v\n", err)
		os.Exit(1)
	}

	// Add loaded instances to the list
	for _, instance := range instances {
		// Call the finalizer immediately.
		h.list.AddInstance(instance)()
		if autoYes {
			instance.AutoYes = true
		}
	}

	// Load topics
	topics, err := storage.LoadTopics()
	if err != nil {
		log.ErrorLog.Printf("Failed to load topics: %v", err)
		topics = []*session.Topic{}
	}
	h.topics = topics
	h.updateSidebarItems()

	return h
}

// updateHandleWindowSizeEvent sets the sizes of the components.
// The components will try to render inside their bounds.
func (m *home) updateHandleWindowSizeEvent(msg tea.WindowSizeMsg) {
	// Three-column layout: sidebar (15%), instance list (20%), preview (remaining)
	sidebarWidth := int(float32(msg.Width) * 0.15)
	if sidebarWidth < 16 {
		sidebarWidth = 16
	}
	listWidth := int(float32(msg.Width) * 0.20)
	tabsWidth := msg.Width - sidebarWidth - listWidth

	// Menu takes 10% of height, list and window take 90%
	contentHeight := int(float32(msg.Height) * 0.9)
	menuHeight := msg.Height - contentHeight - 1     // minus 1 for error box
	m.errBox.SetSize(int(float32(msg.Width)*0.9), 1) // error box takes 1 row

	m.tabbedWindow.SetSize(tabsWidth, contentHeight)
	m.list.SetSize(listWidth, contentHeight)
	m.sidebar.SetSize(sidebarWidth, contentHeight)

	// Store for mouse hit-testing
	m.sidebarWidth = sidebarWidth
	m.listWidth = listWidth
	m.contentHeight = contentHeight

	if m.textInputOverlay != nil {
		m.textInputOverlay.SetSize(int(float32(msg.Width)*0.6), int(float32(msg.Height)*0.4))
	}
	if m.textOverlay != nil {
		m.textOverlay.SetWidth(int(float32(msg.Width) * 0.6))
	}

	previewWidth, previewHeight := m.tabbedWindow.GetPreviewSize()
	if err := m.list.SetSessionPreviewSize(previewWidth, previewHeight); err != nil {
		log.ErrorLog.Print(err)
	}
	m.menu.SetSize(msg.Width, menuHeight)
}

func (m *home) Init() tea.Cmd {
	// Upon starting, we want to start the spinner. Whenever we get a spinner.TickMsg, we
	// update the spinner, which sends a new spinner.TickMsg. I think this lasts forever lol.
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			time.Sleep(100 * time.Millisecond)
			return previewTickMsg{}
		},
		tickUpdateMetadataCmd,
	)
}

func (m *home) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case hideErrMsg:
		m.errBox.Clear()
	case previewTickMsg:
		cmd := m.instanceChanged()
		return m, tea.Batch(
			cmd,
			func() tea.Msg {
				time.Sleep(100 * time.Millisecond)
				return previewTickMsg{}
			},
		)
	case keyupMsg:
		m.menu.ClearKeydown()
		return m, nil
	case tickUpdateMetadataMessage:
		for _, instance := range m.list.GetInstances() {
			if !instance.Started() || instance.Paused() {
				continue
			}
			updated, prompt := instance.HasUpdated()
			if updated {
				instance.SetStatus(session.Running)
			} else {
				if prompt {
					instance.TapEnter()
				} else {
					instance.SetStatus(session.Ready)
				}
			}
			if err := instance.UpdateDiffStats(); err != nil {
				log.WarningLog.Printf("could not update diff stats: %v", err)
			}
		}
		return m, tickUpdateMetadataCmd
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		m.updateHandleWindowSizeEvent(msg)
		return m, nil
	case error:
		// Handle errors from confirmation actions
		return m, m.handleError(msg)
	case instanceChangedMsg:
		// Handle instance changed after confirmation action
		m.updateSidebarItems()
		return m, m.instanceChanged()
	case instanceStartedMsg:
		if msg.err != nil {
			m.list.Kill()
			m.updateSidebarItems()
			return m, m.handleError(msg.err)
		}
		// Instance started successfully — save and finalize
		if err := m.storage.SaveInstances(m.list.GetInstances()); err != nil {
			return m, m.handleError(err)
		}
		m.updateSidebarItems()
		if m.newInstanceFinalizer != nil {
			m.newInstanceFinalizer()
		}
		if m.autoYes {
			msg.instance.AutoYes = true
		}
		return m, tea.Batch(tea.WindowSize(), m.instanceChanged())
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *home) handleQuit() (tea.Model, tea.Cmd) {
	if err := m.storage.SaveInstances(m.list.GetInstances()); err != nil {
		return m, m.handleError(err)
	}
	if err := m.storage.SaveTopics(m.topics); err != nil {
		return m, m.handleError(err)
	}
	return m, tea.Quit
}

func (m *home) handleMenuHighlighting(msg tea.KeyMsg) (cmd tea.Cmd, returnEarly bool) {
	// Handle menu highlighting when you press a button. We intercept it here and immediately return to
	// update the ui while re-sending the keypress. Then, on the next call to this, we actually handle the keypress.
	if m.keySent {
		m.keySent = false
		return nil, false
	}
	if m.state == statePrompt || m.state == stateHelp || m.state == stateConfirm || m.state == stateNewTopic || m.state == stateNewTopicConfirm || m.state == stateSearch || m.state == stateMoveTo || m.state == stateContextMenu {
		return nil, false
	}
	// If it's in the global keymap, we should try to highlight it.
	name, ok := keys.GlobalKeyStringsMap[msg.String()]
	if !ok {
		return nil, false
	}

	if m.list.GetSelectedInstance() != nil && m.list.GetSelectedInstance().Paused() && name == keys.KeyEnter {
		return nil, false
	}
	if name == keys.KeyShiftDown || name == keys.KeyShiftUp {
		return nil, false
	}

	// Skip the menu highlighting if the key is not in the map or we are using the shift up and down keys.
	// TODO: cleanup: when you press enter on stateNew, we use keys.KeySubmitName. We should unify the keymap.
	if name == keys.KeyEnter && m.state == stateNew {
		name = keys.KeySubmitName
	}
	m.keySent = true
	return tea.Batch(
		func() tea.Msg { return msg },
		m.keydownCallback(name)), true
}

// handleMouse processes mouse events for click and scroll interactions.
func (m *home) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}

	// Handle scroll wheel (existing behavior)
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		selected := m.list.GetSelectedInstance()
		if selected != nil && selected.Status != session.Paused {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.tabbedWindow.ScrollUp()
			case tea.MouseButtonWheelDown:
				m.tabbedWindow.ScrollDown()
			}
		}
		return m, nil
	}

	// Don't handle clicks in overlay states (except context menu dismissal)
	if m.state == stateContextMenu && msg.Button == tea.MouseButtonLeft {
		m.contextMenu = nil
		m.state = stateDefault
		return m, nil
	}
	if m.state != stateDefault {
		return m, nil
	}

	x, y := msg.X, msg.Y

	// Account for PaddingTop(1) on columns
	contentY := y - 1

	// Right-click: show context menu
	if msg.Button == tea.MouseButtonRight {
		return m.handleRightClick(x, y, contentY)
	}

	// Only handle left clicks from here
	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	// Determine which column was clicked
	if x < m.sidebarWidth {
		// Click in sidebar
		m.setFocus(0)

		// Search bar is at rows 0-2 in the sidebar content (border takes 3 rows)
		if contentY >= 0 && contentY <= 2 {
			m.sidebar.ActivateSearch()
			m.state = stateSearch
			return m, nil
		}

		// Sidebar items start after search bar (row 0) + border (2 rows) + blank line (1 row) = row 4
		itemRow := contentY - 4
		if itemRow >= 0 {
			m.sidebar.ClickItem(itemRow)
			m.filterInstancesByTopic()
			return m, m.instanceChanged()
		}
	} else if x < m.sidebarWidth+m.listWidth {
		// Click in instance list
		m.setFocus(1)

		// Instance list items: each takes ~4 rows (title padding + title + desc + gap)
		// First 2 rows are the header ("Instances" title + blank lines)
		listY := contentY - 4
		if listY >= 0 {
			// Each instance item is approximately 4 rows (padding + title + branch + gap)
			itemIdx := listY / 4
			if itemIdx < m.list.NumInstances() {
				m.list.SetSelectedInstance(itemIdx)
				return m, m.instanceChanged()
			}
		}
	} else {
		// Click in preview/diff area — just switch focus to instance list
		m.setFocus(1)
	}

	return m, nil
}

// executeContextAction performs the action selected from a context menu.
func (m *home) executeContextAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "delete_topic":
		selectedID := m.sidebar.GetSelectedID()
		// Remove all instances in this topic first
		for _, inst := range m.list.GetInstances() {
			if inst.TopicName == selectedID {
				inst.TopicName = ""
			}
		}
		// Remove the topic
		for i, t := range m.topics {
			if t.Name == selectedID {
				t.Cleanup()
				m.topics = append(m.topics[:i], m.topics[i+1:]...)
				break
			}
		}
		m.updateSidebarItems()
		m.storage.SaveInstances(m.list.GetInstances())
		m.storage.SaveTopics(m.topics)
		return m, tea.WindowSize()

	case "kill_instance":
		selected := m.list.GetSelectedInstance()
		if selected != nil {
			m.list.Kill()
			m.storage.SaveInstances(m.list.GetInstances())
			m.updateSidebarItems()
		}
		return m, tea.Batch(tea.WindowSize(), m.instanceChanged())

	case "open_instance":
		selected := m.list.GetSelectedInstance()
		if selected == nil || !selected.Started() || selected.Paused() || !selected.TmuxAlive() {
			return m, nil
		}
		return m, func() tea.Msg {
			ch, err := m.list.Attach()
			if err != nil {
				return err
			}
			<-ch
			return instanceChangedMsg{}
		}

	case "pause_instance":
		selected := m.list.GetSelectedInstance()
		if selected != nil && selected.Status != session.Paused {
			if err := selected.Pause(); err != nil {
				return m, m.handleError(err)
			}
			m.storage.SaveInstances(m.list.GetInstances())
		}
		return m, tea.Batch(tea.WindowSize(), m.instanceChanged())

	case "resume_instance":
		selected := m.list.GetSelectedInstance()
		if selected != nil && selected.Status == session.Paused {
			if err := selected.Resume(); err != nil {
				return m, m.handleError(err)
			}
			m.storage.SaveInstances(m.list.GetInstances())
		}
		return m, tea.Batch(tea.WindowSize(), m.instanceChanged())

	case "move_instance":
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		m.state = stateMoveTo
		m.pickerOverlay = overlay.NewPickerOverlay("Move to topic", m.getMovableTopicNames())
		return m, nil

	case "push_instance":
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		// Trigger the existing push flow
		return m, func() tea.Msg {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
		}

	case "push_topic":
		// Push the topic's branch — find first running instance in topic to push via
		selectedID := m.sidebar.GetSelectedID()
		for _, inst := range m.list.GetInstances() {
			if inst.TopicName == selectedID && inst.Started() {
				m.list.SetSelectedInstance(0) // select it
				return m, func() tea.Msg {
					return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
				}
			}
		}
		return m, nil
	}

	return m, nil
}

// handleRightClick builds and shows a context menu based on what was right-clicked.
func (m *home) handleRightClick(x, y, contentY int) (tea.Model, tea.Cmd) {
	if x < m.sidebarWidth {
		// Right-click in sidebar
		itemRow := contentY - 4
		if itemRow >= 0 {
			m.sidebar.ClickItem(itemRow)
			m.filterInstancesByTopic()
		}
		selectedID := m.sidebar.GetSelectedID()
		if selectedID == ui.SidebarAll || selectedID == ui.SidebarUngrouped {
			// No context menu for All/Ungrouped
			return m, nil
		}
		// Find the topic
		var topic *session.Topic
		for _, t := range m.topics {
			if t.Name == selectedID {
				topic = t
				break
			}
		}
		if topic == nil {
			return m, nil
		}
		items := []overlay.ContextMenuItem{
			{Label: "Delete topic", Action: "delete_topic"},
			{Label: "Rename topic", Action: "rename_topic", Disabled: true},
		}
		if topic.SharedWorktree {
			items = append(items, overlay.ContextMenuItem{Label: "Push branch", Action: "push_topic"})
		}
		m.contextMenu = overlay.NewContextMenu(x, y, items)
		m.state = stateContextMenu
		return m, nil
	} else if x < m.sidebarWidth+m.listWidth {
		// Right-click in instance list — select the item first
		listY := contentY - 4
		if listY >= 0 {
			itemIdx := listY / 4
			if itemIdx < m.list.NumInstances() {
				m.list.SetSelectedInstance(itemIdx)
			}
		}
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		items := []overlay.ContextMenuItem{
			{Label: "Open", Action: "open_instance"},
			{Label: "Kill", Action: "kill_instance"},
		}
		if selected.Status == session.Paused {
			items = append(items, overlay.ContextMenuItem{Label: "Resume", Action: "resume_instance"})
		} else {
			items = append(items, overlay.ContextMenuItem{Label: "Pause", Action: "pause_instance"})
		}
		items = append(items, overlay.ContextMenuItem{Label: "Move to topic", Action: "move_instance"})
		items = append(items, overlay.ContextMenuItem{Label: "Push branch", Action: "push_instance"})
		m.contextMenu = overlay.NewContextMenu(x, y, items)
		m.state = stateContextMenu
		return m, nil
	}
	return m, nil
}

func (m *home) handleKeyPress(msg tea.KeyMsg) (mod tea.Model, cmd tea.Cmd) {
	cmd, returnEarly := m.handleMenuHighlighting(msg)
	if returnEarly {
		return m, cmd
	}

	if m.state == stateContextMenu {
		if m.contextMenu == nil {
			m.state = stateDefault
			return m, nil
		}
		action, closed := m.contextMenu.HandleKeyPress(msg)
		if closed {
			m.contextMenu = nil
			m.state = stateDefault
			if action != "" {
				return m.executeContextAction(action)
			}
			return m, nil
		}
		return m, nil
	}

	if m.state == stateHelp {
		return m.handleHelpState(msg)
	}

	if m.state == stateNew {
		// Handle quit commands first. Don't handle q because the user might want to type that.
		if msg.String() == "ctrl+c" {
			m.state = stateDefault
			m.promptAfterName = false
			m.list.Kill()
			return m, tea.Sequence(
				tea.WindowSize(),
				func() tea.Msg {
					m.menu.SetState(ui.StateDefault)
					return nil
				},
			)
		}

		instance := m.list.GetInstances()[m.list.TotalInstances()-1]
		switch msg.Type {
		// Start the instance (enable previews etc) and go back to the main menu state.
		case tea.KeyEnter:
			if len(instance.Title) == 0 {
				return m, m.handleError(fmt.Errorf("title cannot be empty"))
			}

			// Set loading status and transition to default state immediately
			instance.SetStatus(session.Loading)
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)

			// Handle prompt-after-name flow
			if m.promptAfterName {
				m.state = statePrompt
				m.menu.SetState(ui.StatePrompt)
				m.textInputOverlay = overlay.NewTextInputOverlay("Enter prompt", "")
				m.textInputOverlay.SetSize(50, 5)
				m.promptAfterName = false
			}

			// Find topic for shared worktree check
			var topic *session.Topic
			for _, t := range m.topics {
				if t.Name == instance.TopicName {
					topic = t
					break
				}
			}

			// Start instance asynchronously
			startCmd := func() tea.Msg {
				var startErr error
				if topic != nil && topic.SharedWorktree && topic.Started() {
					startErr = instance.StartInSharedWorktree(topic.GetGitWorktree(), topic.Branch)
				} else {
					startErr = instance.Start(true)
				}
				return instanceStartedMsg{instance: instance, err: startErr}
			}

			return m, tea.Batch(tea.WindowSize(), startCmd)
		case tea.KeyRunes:
			if runewidth.StringWidth(instance.Title) >= 32 {
				return m, m.handleError(fmt.Errorf("title cannot be longer than 32 characters"))
			}
			if err := instance.SetTitle(instance.Title + string(msg.Runes)); err != nil {
				return m, m.handleError(err)
			}
		case tea.KeyBackspace:
			runes := []rune(instance.Title)
			if len(runes) == 0 {
				return m, nil
			}
			if err := instance.SetTitle(string(runes[:len(runes)-1])); err != nil {
				return m, m.handleError(err)
			}
		case tea.KeySpace:
			if err := instance.SetTitle(instance.Title + " "); err != nil {
				return m, m.handleError(err)
			}
		case tea.KeyEsc:
			m.list.Kill()
			m.state = stateDefault
			m.instanceChanged()

			return m, tea.Sequence(
				tea.WindowSize(),
				func() tea.Msg {
					m.menu.SetState(ui.StateDefault)
					return nil
				},
			)
		default:
		}
		return m, nil
	} else if m.state == statePrompt {
		// Use the new TextInputOverlay component to handle all key events
		shouldClose := m.textInputOverlay.HandleKeyPress(msg)

		// Check if the form was submitted or canceled
		if shouldClose {
			selected := m.list.GetSelectedInstance()
			// TODO: this should never happen since we set the instance in the previous state.
			if selected == nil {
				return m, nil
			}
			if m.textInputOverlay.IsSubmitted() {
				if err := selected.SendPrompt(m.textInputOverlay.GetValue()); err != nil {
					// TODO: we probably end up in a bad state here.
					return m, m.handleError(err)
				}
			}

			// Close the overlay and reset state
			m.textInputOverlay = nil
			m.state = stateDefault
			return m, tea.Sequence(
				tea.WindowSize(),
				func() tea.Msg {
					m.menu.SetState(ui.StateDefault)
					m.showHelpScreen(helpStart(selected), nil)
					return nil
				},
			)
		}

		return m, nil
	}

	// Handle confirmation state
	if m.state == stateConfirm {
		shouldClose := m.confirmationOverlay.HandleKeyPress(msg)
		if shouldClose {
			m.state = stateDefault
			m.confirmationOverlay = nil
			return m, nil
		}
		return m, nil
	}

	// Handle new topic creation state
	if m.state == stateNewTopic {
		shouldClose := m.textInputOverlay.HandleKeyPress(msg)
		if shouldClose {
			if m.textInputOverlay.IsSubmitted() {
				m.pendingTopicName = m.textInputOverlay.GetValue()
				if m.pendingTopicName == "" {
					m.state = stateDefault
					m.menu.SetState(ui.StateDefault)
					m.textInputOverlay = nil
					return m, m.handleError(fmt.Errorf("topic name cannot be empty"))
				}
				// Show shared worktree confirmation
				m.textInputOverlay = nil
				m.confirmationOverlay = overlay.NewConfirmationOverlay(
					fmt.Sprintf("Create shared worktree for topic '%s'?\nAll instances will share one branch and directory.", m.pendingTopicName),
				)
				m.confirmationOverlay.SetWidth(60)
				m.state = stateNewTopicConfirm
				return m, nil
			}
			// Cancelled
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			m.pendingTopicName = ""
			m.textInputOverlay = nil
			return m, tea.WindowSize()
		}
		return m, nil
	}

	// Handle new topic shared worktree confirmation state
	if m.state == stateNewTopicConfirm {
		if m.confirmationOverlay == nil {
			m.state = stateDefault
			return m, nil
		}
		shouldClose := m.confirmationOverlay.HandleKeyPress(msg)
		if !shouldClose {
			return m, nil // No decision yet
		}

		// Determine if confirmed (y) or cancelled (n/esc) based on which key was pressed
		shared := msg.String() == m.confirmationOverlay.ConfirmKey
		topic := session.NewTopic(session.TopicOptions{
			Name:           m.pendingTopicName,
			SharedWorktree: shared,
			Path:           ".",
		})
		if err := topic.Setup(); err != nil {
			m.pendingTopicName = ""
			m.confirmationOverlay = nil
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			return m, m.handleError(err)
		}
		m.topics = append(m.topics, topic)
		m.updateSidebarItems()
		if err := m.storage.SaveTopics(m.topics); err != nil {
			return m, m.handleError(err)
		}
		m.pendingTopicName = ""
		m.confirmationOverlay = nil
		m.state = stateDefault
		m.menu.SetState(ui.StateDefault)
		return m, tea.WindowSize()
	}

	// Handle move-to-topic state (picker overlay)
	if m.state == stateMoveTo {
		shouldClose := m.pickerOverlay.HandleKeyPress(msg)
		if shouldClose {
			selected := m.list.GetSelectedInstance()
			if selected != nil && m.pickerOverlay.IsSubmitted() {
				picked := m.pickerOverlay.Value()
				if picked == "(Ungrouped)" {
					selected.TopicName = ""
				} else {
					selected.TopicName = picked
				}
				m.updateSidebarItems()
				if err := m.storage.SaveInstances(m.list.GetInstances()); err != nil {
					m.state = stateDefault
					m.menu.SetState(ui.StateDefault)
					m.pickerOverlay = nil
					return m, m.handleError(err)
				}
			}
			m.state = stateDefault
			m.menu.SetState(ui.StateDefault)
			m.pickerOverlay = nil
			return m, tea.WindowSize()
		}
		return m, nil
	}

	// Handle search state — allows typing to filter AND arrow keys to navigate topics
	if m.state == stateSearch {
		switch {
		case msg.String() == "esc":
			m.sidebar.DeactivateSearch()
			m.sidebar.UpdateMatchCounts(nil, 0)
			m.state = stateDefault
			m.filterInstancesByTopic()
			return m, nil
		case msg.String() == "enter":
			m.sidebar.DeactivateSearch()
			m.sidebar.UpdateMatchCounts(nil, 0)
			m.state = stateDefault
			return m, nil
		case msg.String() == "up":
			m.sidebar.Up()
			m.filterSearchWithTopic()
			return m, m.instanceChanged()
		case msg.String() == "down":
			m.sidebar.Down()
			m.filterSearchWithTopic()
			return m, m.instanceChanged()
		case msg.Type == tea.KeyBackspace:
			q := m.sidebar.GetSearchQuery()
			if len(q) > 0 {
				runes := []rune(q)
				m.sidebar.SetSearchQuery(string(runes[:len(runes)-1]))
			}
			m.filterBySearch()
			return m, nil
		case msg.Type == tea.KeySpace:
			m.sidebar.SetSearchQuery(m.sidebar.GetSearchQuery() + " ")
			m.filterBySearch()
			return m, nil
		case msg.Type == tea.KeyRunes:
			m.sidebar.SetSearchQuery(m.sidebar.GetSearchQuery() + string(msg.Runes))
			m.filterBySearch()
			return m, nil
		}
		return m, nil
	}

	// Exit scrolling mode when ESC is pressed and preview pane is in scrolling mode
	// Check if Escape key was pressed and we're not in the diff tab (meaning we're in preview tab)
	// Always check for escape key first to ensure it doesn't get intercepted elsewhere
	if msg.Type == tea.KeyEsc {
		// If in preview tab and in scroll mode, exit scroll mode
		if !m.tabbedWindow.IsInDiffTab() && m.tabbedWindow.IsPreviewInScrollMode() {
			// Use the selected instance from the list
			selected := m.list.GetSelectedInstance()
			err := m.tabbedWindow.ResetPreviewToNormalMode(selected)
			if err != nil {
				return m, m.handleError(err)
			}
			return m, m.instanceChanged()
		}
	}

	// Handle quit commands first
	if msg.String() == "ctrl+c" || msg.String() == "q" {
		return m.handleQuit()
	}

	name, ok := keys.GlobalKeyStringsMap[msg.String()]
	if !ok {
		return m, nil
	}

	switch name {
	case keys.KeyHelp:
		return m.showHelpScreen(helpTypeGeneral{}, nil)
	case keys.KeyPrompt:
		if m.list.TotalInstances() >= GlobalInstanceLimit {
			return m, m.handleError(
				fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
		}
		topicName := ""
		selectedID := m.sidebar.GetSelectedID()
		if selectedID != ui.SidebarAll && selectedID != ui.SidebarUngrouped {
			topicName = selectedID
		}
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:     "",
			Path:      ".",
			Program:   m.program,
			TopicName: topicName,
		})
		if err != nil {
			return m, m.handleError(err)
		}

		m.newInstanceFinalizer = m.list.AddInstance(instance)
		m.list.SetSelectedInstance(m.list.NumInstances() - 1)
		m.state = stateNew
		m.menu.SetState(ui.StateNewInstance)
		m.promptAfterName = true

		return m, nil
	case keys.KeyNew:
		if m.list.TotalInstances() >= GlobalInstanceLimit {
			return m, m.handleError(
				fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
		}
		topicName := ""
		selectedID := m.sidebar.GetSelectedID()
		if selectedID != ui.SidebarAll && selectedID != ui.SidebarUngrouped {
			topicName = selectedID
		}
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:     "",
			Path:      ".",
			Program:   m.program,
			TopicName: topicName,
		})
		if err != nil {
			return m, m.handleError(err)
		}

		m.newInstanceFinalizer = m.list.AddInstance(instance)
		m.list.SetSelectedInstance(m.list.NumInstances() - 1)
		m.state = stateNew
		m.menu.SetState(ui.StateNewInstance)

		return m, nil
	case keys.KeyNewSkipPermissions:
		if m.list.TotalInstances() >= GlobalInstanceLimit {
			return m, m.handleError(
				fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
		}
		topicName := ""
		selectedID := m.sidebar.GetSelectedID()
		if selectedID != ui.SidebarAll && selectedID != ui.SidebarUngrouped {
			topicName = selectedID
		}
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:           "",
			Path:            ".",
			Program:         m.program,
			SkipPermissions: true,
			TopicName:       topicName,
		})
		if err != nil {
			return m, m.handleError(err)
		}

		m.newInstanceFinalizer = m.list.AddInstance(instance)
		m.list.SetSelectedInstance(m.list.NumInstances() - 1)
		m.state = stateNew
		m.menu.SetState(ui.StateNewInstance)

		return m, nil
	case keys.KeyUp:
		if m.focusedPanel == 0 {
			m.sidebar.Up()
			m.filterInstancesByTopic()
		} else {
			m.list.Up()
		}
		return m, m.instanceChanged()
	case keys.KeyDown:
		if m.focusedPanel == 0 {
			m.sidebar.Down()
			m.filterInstancesByTopic()
		} else {
			m.list.Down()
		}
		return m, m.instanceChanged()
	case keys.KeyShiftUp:
		m.tabbedWindow.ScrollUp()
		return m, m.instanceChanged()
	case keys.KeyShiftDown:
		m.tabbedWindow.ScrollDown()
		return m, m.instanceChanged()
	case keys.KeyTab:
		m.tabbedWindow.Toggle()
		m.menu.SetInDiffTab(m.tabbedWindow.IsInDiffTab())
		return m, m.instanceChanged()
	case keys.KeyKill:
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}

		// Create the kill action as a tea.Cmd
		killAction := func() tea.Msg {
			// Get worktree and check if branch is checked out
			worktree, err := selected.GetGitWorktree()
			if err != nil {
				return err
			}

			checkedOut, err := worktree.IsBranchCheckedOut()
			if err != nil {
				return err
			}

			if checkedOut {
				return fmt.Errorf("instance %s is currently checked out", selected.Title)
			}

			// Delete from storage first
			if err := m.storage.DeleteInstance(selected.Title); err != nil {
				return err
			}

			// Then kill the instance
			m.list.Kill()
			return instanceChangedMsg{}
		}

		// Show confirmation modal
		message := fmt.Sprintf("[!] Kill session '%s'?", selected.Title)
		return m, m.confirmAction(message, killAction)
	case keys.KeySubmit:
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}

		// Create the push action as a tea.Cmd
		pushAction := func() tea.Msg {
			// Default commit message with timestamp
			commitMsg := fmt.Sprintf("[claudesquad] update from '%s' on %s", selected.Title, time.Now().Format(time.RFC822))
			worktree, err := selected.GetGitWorktree()
			if err != nil {
				return err
			}
			if err = worktree.PushChanges(commitMsg, true); err != nil {
				return err
			}
			return nil
		}

		// Show confirmation modal
		message := fmt.Sprintf("[!] Push changes from session '%s'?", selected.Title)
		return m, m.confirmAction(message, pushAction)
	case keys.KeyCheckout:
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}

		// Show help screen before pausing
		m.showHelpScreen(helpTypeInstanceCheckout{}, func() {
			if err := selected.Pause(); err != nil {
				m.handleError(err)
			}
			m.instanceChanged()
		})
		return m, nil
	case keys.KeyResume:
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		if err := selected.Resume(); err != nil {
			return m, m.handleError(err)
		}
		return m, tea.WindowSize()
	case keys.KeyEnter:
		if m.list.NumInstances() == 0 {
			return m, nil
		}
		selected := m.list.GetSelectedInstance()
		if selected == nil || selected.Paused() || !selected.TmuxAlive() {
			return m, nil
		}
		// Show help screen before attaching
		m.showHelpScreen(helpTypeInstanceAttach{}, func() {
			ch, err := m.list.Attach()
			if err != nil {
				m.handleError(err)
				return
			}
			<-ch
			m.state = stateDefault
		})
		return m, nil
	case keys.KeyLeft:
		m.setFocus(0)
		return m, nil
	case keys.KeyRight:
		m.setFocus(1)
		return m, nil
	case keys.KeyNewTopic:
		m.state = stateNewTopic
		m.textInputOverlay = overlay.NewTextInputOverlay("Topic name", "")
		m.textInputOverlay.SetSize(50, 3)
		return m, nil
	case keys.KeyMoveTo:
		selected := m.list.GetSelectedInstance()
		if selected == nil {
			return m, nil
		}
		// Can't move shared-worktree instances (they're tied to their topic's worktree)
		if selected.TopicName != "" {
			for _, t := range m.topics {
				if t.Name == selected.TopicName && t.SharedWorktree {
					return m, m.handleError(fmt.Errorf("cannot move instances in shared-worktree topics"))
				}
			}
		}
		m.state = stateMoveTo
		m.pickerOverlay = overlay.NewPickerOverlay("Move to topic", m.getMovableTopicNames())
		return m, nil
	case keys.KeySearch:
		m.sidebar.ActivateSearch()
		m.sidebar.SelectFirst() // Reset to "All" when starting search
		m.state = stateSearch
		m.setFocus(0)
		m.list.SetFilter("") // Show all instances
		return m, nil
	default:
		return m, nil
	}
}

func (m *home) updateSidebarItems() {
	topicNames := make([]string, len(m.topics))
	countByTopic := make(map[string]int)
	sharedTopics := make(map[string]bool)
	ungroupedCount := 0

	for i, t := range m.topics {
		topicNames[i] = t.Name
		if t.SharedWorktree {
			sharedTopics[t.Name] = true
		}
	}

	for _, inst := range m.list.GetInstances() {
		if inst.TopicName == "" {
			ungroupedCount++
		} else {
			countByTopic[inst.TopicName]++
		}
	}

	m.sidebar.SetItems(topicNames, countByTopic, ungroupedCount, sharedTopics)
}

// getMovableTopicNames returns topic names that a non-shared instance can be moved to.
func (m *home) getMovableTopicNames() []string {
	names := []string{"(Ungrouped)"}
	for _, t := range m.topics {
		names = append(names, t.Name)
	}
	return names
}

// setFocus updates which panel has focus and syncs the focused state to sidebar and list.
func (m *home) setFocus(panel int) {
	m.focusedPanel = panel
	m.sidebar.SetFocused(panel == 0)
	m.list.SetFocused(panel == 1)
}

func (m *home) filterInstancesByTopic() {
	selectedID := m.sidebar.GetSelectedID()
	switch selectedID {
	case ui.SidebarAll:
		m.list.SetFilter("")
	case ui.SidebarUngrouped:
		m.list.SetFilter(ui.SidebarUngrouped)
	default:
		m.list.SetFilter(selectedID)
	}
}

// filterSearchWithTopic applies the search query scoped to the currently selected topic.
func (m *home) filterSearchWithTopic() {
	query := strings.ToLower(m.sidebar.GetSearchQuery())
	selectedID := m.sidebar.GetSelectedID()
	topicFilter := ""
	switch selectedID {
	case ui.SidebarAll:
		topicFilter = ""
	case ui.SidebarUngrouped:
		topicFilter = ui.SidebarUngrouped
	default:
		topicFilter = selectedID
	}
	m.list.SetSearchFilterWithTopic(query, topicFilter)
}

func (m *home) filterBySearch() {
	query := strings.ToLower(m.sidebar.GetSearchQuery())
	if query == "" {
		m.sidebar.UpdateMatchCounts(nil, 0)
		m.filterInstancesByTopic()
		return
	}
	m.list.SetSearchFilter(query)

	// Calculate match counts per topic for sidebar dimming
	matchesByTopic := make(map[string]int)
	totalMatches := 0
	for _, inst := range m.list.GetInstances() {
		if strings.Contains(strings.ToLower(inst.Title), query) ||
			strings.Contains(strings.ToLower(inst.TopicName), query) {
			matchesByTopic[inst.TopicName]++
			totalMatches++
		}
	}
	m.sidebar.UpdateMatchCounts(matchesByTopic, totalMatches)
}

// instanceChanged updates the preview pane, menu, and diff pane based on the selected instance. It returns an error
// Cmd if there was any error.
func (m *home) instanceChanged() tea.Cmd {
	// selected may be nil
	selected := m.list.GetSelectedInstance()

	m.tabbedWindow.UpdateDiff(selected)
	m.tabbedWindow.SetInstance(selected)
	// Update menu with current instance
	m.menu.SetInstance(selected)

	// If there's no selected instance, we don't need to update the preview.
	if err := m.tabbedWindow.UpdatePreview(selected); err != nil {
		return m.handleError(err)
	}
	return nil
}

type keyupMsg struct{}

// keydownCallback clears the menu option highlighting after 500ms.
func (m *home) keydownCallback(name keys.KeyName) tea.Cmd {
	m.menu.Keydown(name)
	return func() tea.Msg {
		select {
		case <-m.ctx.Done():
		case <-time.After(500 * time.Millisecond):
		}

		return keyupMsg{}
	}
}

// hideErrMsg implements tea.Msg and clears the error text from the screen.
type hideErrMsg struct{}

// previewTickMsg implements tea.Msg and triggers a preview update
type previewTickMsg struct{}

type tickUpdateMetadataMessage struct{}

type instanceChangedMsg struct{}

// instanceStartedMsg is sent when an async instance startup completes.
type instanceStartedMsg struct {
	instance *session.Instance
	err      error
}

// tickUpdateMetadataCmd is the callback to update the metadata of the instances every 500ms. Note that we iterate
// overall the instances and capture their output. It's a pretty expensive operation. Let's do it 2x a second only.
var tickUpdateMetadataCmd = func() tea.Msg {
	time.Sleep(500 * time.Millisecond)
	return tickUpdateMetadataMessage{}
}

// handleError handles all errors which get bubbled up to the app. sets the error message. We return a callback tea.Cmd that returns a hideErrMsg message
// which clears the error message after 3 seconds.
func (m *home) handleError(err error) tea.Cmd {
	log.ErrorLog.Printf("%v", err)
	m.errBox.SetError(err)
	return func() tea.Msg {
		select {
		case <-m.ctx.Done():
		case <-time.After(3 * time.Second):
		}

		return hideErrMsg{}
	}
}

// confirmAction shows a confirmation modal and stores the action to execute on confirm
func (m *home) confirmAction(message string, action tea.Cmd) tea.Cmd {
	m.state = stateConfirm

	// Create and show the confirmation overlay using ConfirmationOverlay
	m.confirmationOverlay = overlay.NewConfirmationOverlay(message)
	// Set a fixed width for consistent appearance
	m.confirmationOverlay.SetWidth(50)

	// Set callbacks for confirmation and cancellation
	m.confirmationOverlay.OnConfirm = func() {
		m.state = stateDefault
		// Execute the action if it exists
		if action != nil {
			_ = action()
		}
		m.updateSidebarItems()
	}

	m.confirmationOverlay.OnCancel = func() {
		m.state = stateDefault
	}

	return nil
}

func (m *home) View() string {
	// All columns use identical padding and height for uniform alignment.
	colStyle := lipgloss.NewStyle().PaddingTop(1).Height(m.contentHeight + 1)
	sidebarView := colStyle.Render(m.sidebar.String())
	listWithPadding := colStyle.Render(m.list.String())
	previewWithPadding := colStyle.Render(m.tabbedWindow.String())
	listAndPreview := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, listWithPadding, previewWithPadding)

	mainView := lipgloss.JoinVertical(
		lipgloss.Left,
		listAndPreview,
		m.menu.String(),
		m.errBox.String(),
	)

	if m.state == stateMoveTo && m.pickerOverlay != nil {
		return overlay.PlaceOverlay(0, 0, m.pickerOverlay.Render(), mainView, true, true)
	} else if m.state == stateNewTopic && m.textInputOverlay != nil {
		return overlay.PlaceOverlay(0, 0, m.textInputOverlay.Render(), mainView, true, true)
	} else if m.state == statePrompt {
		if m.textInputOverlay == nil {
			log.ErrorLog.Printf("text input overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, m.textInputOverlay.Render(), mainView, true, true)
	} else if m.state == stateHelp {
		if m.textOverlay == nil {
			log.ErrorLog.Printf("text overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, m.textOverlay.Render(), mainView, true, true)
	} else if m.state == stateConfirm || m.state == stateNewTopicConfirm {
		if m.confirmationOverlay == nil {
			log.ErrorLog.Printf("confirmation overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, m.confirmationOverlay.Render(), mainView, true, true)
	} else if m.state == stateContextMenu && m.contextMenu != nil {
		cx, cy := m.contextMenu.GetPosition()
		return overlay.PlaceOverlay(cx, cy, m.contextMenu.Render(), mainView, true, false)
	}

	return mainView
}
