package app

import (
	"context"
	"fmt"
	"github.com/ByteMirror/hivemind/config"
	"github.com/ByteMirror/hivemind/log"
	"github.com/ByteMirror/hivemind/session"
	"github.com/ByteMirror/hivemind/ui"
	"github.com/ByteMirror/hivemind/ui/overlay"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

const GlobalInstanceLimit = 10

// Run is the main entrypoint into the application.
func Run(ctx context.Context, program string, autoYes bool) error {
	zone.NewGlobal()
	p := tea.NewProgram(
		newHome(ctx, program, autoYes),
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(), // Full mouse tracking for hover + scroll + click
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
	// statePRTitle is the state when the user is entering a PR title.
	statePRTitle
	// statePRBody is the state when the user is editing the PR body/description.
	statePRBody
	// stateRenameInstance is the state when the user is renaming an instance.
	stateRenameInstance
	// stateRenameTopic is the state when the user is renaming a topic.
	stateRenameTopic
	// stateSendPrompt is the state when the user is sending a prompt via text overlay.
	stateSendPrompt
	// stateFocusAgent is the state when the user is typing directly into the agent pane.
	stateFocusAgent
	// stateContextMenu is the state when a right-click context menu is shown.
	stateContextMenu
	// stateRepoSwitch is the state when the user is switching repos via picker.
	stateRepoSwitch
)

type home struct {
	ctx context.Context

	// -- Storage and Configuration --

	program string
	autoYes bool

	// activeRepoPath is the currently active repository path for filtering and new instances
	activeRepoPath string

	// storage is the interface for saving/loading data to/from the app's state
	storage *session.Storage
	// appConfig stores persistent application configuration
	appConfig *config.Config
	// appState stores persistent application state like seen help screens
	appState config.AppState

	// -- State --

	// allInstances stores every instance across all repos (master list)
	allInstances []*session.Instance

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
	// toastManager manages toast notifications
	toastManager *overlay.ToastManager
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
	// topics is the list of topics for the active repo
	topics []*session.Topic
	// allTopics stores every topic across all repos (master list)
	allTopics []*session.Topic
	// focusedPanel tracks which panel has keyboard focus (0=sidebar, 1=instance list)
	focusedPanel int
	// pendingTopicName stores the topic name during the two-step creation flow
	pendingTopicName string
	// pendingPRTitle stores the PR title during the two-step PR creation flow
	pendingPRTitle string
	// pendingPRToastID stores the toast ID for the in-progress PR creation
	pendingPRToastID string

	// contextMenu is the right-click context menu overlay
	contextMenu *overlay.ContextMenu
	// pickerOverlay is the topic picker overlay for move-to-topic
	pickerOverlay *overlay.PickerOverlay

	// Layout dimensions for mouse hit-testing
	sidebarWidth  int
	listWidth     int
	contentHeight int

	// embeddedTerminal is the VT emulator for focus mode (nil when not in focus mode)
	embeddedTerminal *session.EmbeddedTerminal

	// repoPickerMap maps picker display text to full repo path
	repoPickerMap map[string]string
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

	activeRepoPath, err := filepath.Abs(".")
	if err != nil {
		fmt.Printf("Failed to get current directory: %v\n", err)
		os.Exit(1)
	}

	h := &home{
		ctx:            ctx,
		spinner:        spinner.New(spinner.WithSpinner(spinner.Dot)),
		menu:           ui.NewMenu(),
		tabbedWindow:   ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane(), ui.NewGitPane()),
		storage:        storage,
		appConfig:      appConfig,
		program:        program,
		autoYes:        autoYes,
		state:          stateDefault,
		appState:       appState,
		activeRepoPath: activeRepoPath,
	}
	h.list = ui.NewList(&h.spinner, autoYes)
	h.toastManager = overlay.NewToastManager(&h.spinner)
	h.sidebar = ui.NewSidebar()
	h.sidebar.SetRepoName(filepath.Base(activeRepoPath))
	h.setFocus(1) // Start with instance list focused

	// Load saved instances
	instances, err := storage.LoadInstances()
	if err != nil {
		fmt.Printf("Failed to load instances: %v\n", err)
		os.Exit(1)
	}

	h.allInstances = instances

	// Add instances matching active repo to the list
	for _, instance := range instances {
		repoPath := instance.GetRepoPath()
		if repoPath == "" || repoPath == h.activeRepoPath {
			h.list.AddInstance(instance)()
			if autoYes {
				instance.AutoYes = true
			}
		}
	}

	// Load topics
	topics, err := storage.LoadTopics()
	if err != nil {
		log.ErrorLog.Printf("Failed to load topics: %v", err)
		topics = []*session.Topic{}
	}
	// Migrate legacy topics that used "." as their path
	for _, t := range topics {
		if t.Path == "" || t.Path == "." {
			t.Path = activeRepoPath
		}
	}
	h.allTopics = topics
	h.topics = h.filterTopicsByRepo(topics, activeRepoPath)
	h.updateSidebarItems()

	// Persist the active repo so it appears in the picker even if it has no instances
	if state, ok := h.appState.(*config.State); ok {
		state.AddRecentRepo(activeRepoPath)
	}

	return h
}

// updateHandleWindowSizeEvent sets the sizes of the components.
// The components will try to render inside their bounds.
func (m *home) updateHandleWindowSizeEvent(msg tea.WindowSizeMsg) {
	// Three-column layout: sidebar (15%), instance list (20%), preview (remaining)
	sidebarWidth := int(float32(msg.Width) * 0.18)
	if sidebarWidth < 20 {
		sidebarWidth = 20
	}
	listWidth := int(float32(msg.Width) * 0.20)
	tabsWidth := msg.Width - sidebarWidth - listWidth

	// Menu takes 10% of height, list and window take 90%
	contentHeight := int(float32(msg.Height) * 0.9)
	menuHeight := msg.Height - contentHeight
	m.toastManager.SetSize(msg.Width, msg.Height)

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
		m.toastTickCmd(),
	)
}

func (m *home) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case overlay.ToastTickMsg:
		m.toastManager.Tick()
		if m.toastManager.HasActiveToasts() {
			return m, m.toastTickCmd()
		}
		return m, nil
	case prCreatedMsg:
		m.toastManager.Resolve(m.pendingPRToastID, overlay.ToastSuccess, "PR created!")
		m.pendingPRToastID = ""
		return m, m.toastTickCmd()
	case prErrorMsg:
		log.ErrorLog.Printf("%v", msg.err)
		m.toastManager.Resolve(msg.id, overlay.ToastError, msg.err.Error())
		m.pendingPRToastID = ""
		return m, m.toastTickCmd()
	case previewTickMsg:
		cmd := m.instanceChanged()
		return m, tea.Batch(
			cmd,
			func() tea.Msg {
				time.Sleep(100 * time.Millisecond)
				return previewTickMsg{}
			},
		)
	case focusPreviewTickMsg:
		if m.state != stateFocusAgent || m.embeddedTerminal == nil {
			return m, nil
		}
		if content, changed := m.embeddedTerminal.Render(); changed {
			m.tabbedWindow.SetPreviewContent(content)
		}
		// Capture reference for the command goroutine — safe even if
		// exitFocusMode() nils m.embeddedTerminal before the command fires.
		term := m.embeddedTerminal
		return m, func() tea.Msg {
			// Block until new content is rendered or 50ms elapses.
			// This replaces the fixed 16ms sleep with event-driven wakeup,
			// cutting worst-case display latency from ~24ms to ~1-3ms.
			term.WaitForRender(50 * time.Millisecond)
			return focusPreviewTickMsg{}
		}
	case gitTabTickMsg:
		if !m.tabbedWindow.IsInGitTab() {
			return m, nil
		}
		gitPane := m.tabbedWindow.GetGitPane()
		if !gitPane.IsRunning() {
			return m, nil
		}
		// Only trigger re-render when content changed to avoid flicker
		content, changed := gitPane.Render()
		if changed {
			m.tabbedWindow.SetGitContent(content)
		}
		return m, func() tea.Msg {
			time.Sleep(33 * time.Millisecond)
			return gitTabTickMsg{}
		}
	case keyupMsg:
		m.menu.ClearKeydown()
		return m, nil
	case tickUpdateMetadataMessage:
		for _, instance := range m.list.GetInstances() {
			if !instance.Started() || instance.Paused() {
				instance.LastActivity = nil
				continue
			}
			updated, prompt := instance.HasUpdated()
			if updated {
				instance.SetStatus(session.Running)
				// Parse activity from pane content.
				if content, err := instance.GetPaneContent(); err == nil && content != "" {
					instance.LastActivity = session.ParseActivity(content, instance.Program)
				}
			} else {
				if prompt {
					instance.PromptDetected = true
					instance.TapEnter()
				} else {
					instance.SetStatus(session.Ready)
				}
				// Clear activity when instance is no longer running.
				if instance.Status != session.Running {
					instance.LastActivity = nil
				}
			}
			if err := instance.UpdateDiffStats(); err != nil {
				log.WarningLog.Printf("could not update diff stats: %v", err)
			}
			instance.UpdateResourceUsage()
		}
		m.updateSidebarItems()
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
		// Instance started successfully — add to master list, save and finalize
		m.allInstances = append(m.allInstances, msg.instance)
		if err := m.saveAllInstances(); err != nil {
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
	case folderPickedMsg:
		m.state = stateDefault
		m.pickerOverlay = nil
		if msg.err != nil {
			return m, m.handleError(msg.err)
		}
		if msg.path != "" {
			m.activeRepoPath = msg.path
			m.sidebar.SetRepoName(filepath.Base(msg.path))
			if state, ok := m.appState.(*config.State); ok {
				state.AddRecentRepo(msg.path)
			}
			m.rebuildInstanceList()
		}
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *home) handleQuit() (tea.Model, tea.Cmd) {
	m.killGitTab()
	if err := m.saveAllInstances(); err != nil {
		return m, m.handleError(err)
	}
	if err := m.saveAllTopics(); err != nil {
		return m, m.handleError(err)
	}
	return m, tea.Quit
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
	)

	var result string
	switch {
	case m.state == stateSendPrompt && m.textInputOverlay != nil:
		result = overlay.PlaceOverlay(0, 0, m.textInputOverlay.Render(), mainView, true, true)
	case m.state == statePRTitle && m.textInputOverlay != nil:
		result = overlay.PlaceOverlay(0, 0, m.textInputOverlay.Render(), mainView, true, true)
	case m.state == statePRBody && m.textInputOverlay != nil:
		result = overlay.PlaceOverlay(0, 0, m.textInputOverlay.Render(), mainView, true, true)
	case m.state == stateRenameInstance && m.textInputOverlay != nil:
		result = overlay.PlaceOverlay(0, 0, m.textInputOverlay.Render(), mainView, true, true)
	case m.state == stateRenameTopic && m.textInputOverlay != nil:
		result = overlay.PlaceOverlay(0, 0, m.textInputOverlay.Render(), mainView, true, true)
	case m.state == stateMoveTo && m.pickerOverlay != nil:
		result = overlay.PlaceOverlay(0, 0, m.pickerOverlay.Render(), mainView, true, true)
	case m.state == stateRepoSwitch && m.pickerOverlay != nil:
		// Position near the repo button at the bottom of the sidebar
		pickerX := 1
		pickerY := m.contentHeight - 10 // above the bottom menu, near the repo indicator
		if pickerY < 2 {
			pickerY = 2
		}
		result = overlay.PlaceOverlay(pickerX, pickerY, m.pickerOverlay.Render(), mainView, true, false)
	case m.state == stateNewTopic && m.textInputOverlay != nil:
		result = overlay.PlaceOverlay(0, 0, m.textInputOverlay.Render(), mainView, true, true)
	case m.state == statePrompt:
		if m.textInputOverlay == nil {
			log.ErrorLog.Printf("text input overlay is nil")
		}
		result = overlay.PlaceOverlay(0, 0, m.textInputOverlay.Render(), mainView, true, true)
	case m.state == stateHelp:
		if m.textOverlay == nil {
			log.ErrorLog.Printf("text overlay is nil")
		}
		result = overlay.PlaceOverlay(0, 0, m.textOverlay.Render(), mainView, true, true)
	case m.state == stateConfirm || m.state == stateNewTopicConfirm:
		if m.confirmationOverlay == nil {
			log.ErrorLog.Printf("confirmation overlay is nil")
		}
		result = overlay.PlaceOverlay(0, 0, m.confirmationOverlay.Render(), mainView, true, true)
	case m.state == stateContextMenu && m.contextMenu != nil:
		cx, cy := m.contextMenu.GetPosition()
		result = overlay.PlaceOverlay(cx, cy, m.contextMenu.Render(), mainView, true, false)
	default:
		result = mainView
	}

	if toastView := m.toastManager.View(); toastView != "" {
		x, y := m.toastManager.GetPosition()
		result = overlay.PlaceOverlay(x, y, toastView, result, false, false)
	}

	return zone.Scan(result)
}

// prCreatedMsg is sent when async PR creation succeeds.
type prCreatedMsg struct{}

// prErrorMsg is sent when async PR creation fails.
type prErrorMsg struct {
	id  string
	err error
}

// previewTickMsg implements tea.Msg and triggers a preview update
type previewTickMsg struct{}

type tickUpdateMetadataMessage struct{}

// focusPreviewTickMsg is a fast ticker (30fps) for focus mode preview refresh only.
type focusPreviewTickMsg struct{}

// gitTabTickMsg is a 30fps ticker for refreshing the git tab's lazygit rendering.
type gitTabTickMsg struct{}

type instanceChangedMsg struct{}

// instanceStartedMsg is sent when an async instance startup completes.
type instanceStartedMsg struct {
	instance *session.Instance
	err      error
}

type keyupMsg struct{}

// tickUpdateMetadataCmd is the callback to update the metadata of the instances every 500ms. Note that we iterate
// overall the instances and capture their output. It's a pretty expensive operation. Let's do it 2x a second only.
var tickUpdateMetadataCmd = func() tea.Msg {
	time.Sleep(500 * time.Millisecond)
	return tickUpdateMetadataMessage{}
}

func (m *home) toastTickCmd() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(50 * time.Millisecond)
		return overlay.ToastTickMsg{}
	}
}
