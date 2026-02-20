package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ByteMirror/hivemind/brain"
	"github.com/ByteMirror/hivemind/config"
	"github.com/ByteMirror/hivemind/log"
	"github.com/ByteMirror/hivemind/session"
	"github.com/ByteMirror/hivemind/ui"
	"github.com/ByteMirror/hivemind/ui/overlay"

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
	// stateNewTopicRepo is the state when the user picks a repo for the new topic (multi-repo).
	stateNewTopicRepo
)

type home struct {
	ctx context.Context

	// -- Storage and Configuration --

	program string
	autoYes bool

	// primaryRepoPath is the launch directory, never changes, used for new ungrouped instances
	primaryRepoPath string
	// activeRepoPaths contains visible repos; first element is always the primary
	activeRepoPaths []string

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
	// pendingInstance is the instance currently being named in stateNew.
	// Stored as a direct reference to avoid position-based lookups that break when
	// the list is sorted.
	pendingInstance *session.Instance

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
	// pendingTopicRepoPath stores the repo path during multi-repo topic creation
	pendingTopicRepoPath string
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
	listWidth     int // full allocation including gaps
	columnGap     int // gap on each side of the instance list
	contentHeight int

	// embeddedTerminal is the VT emulator for focus mode (nil when not in focus mode)
	embeddedTerminal *session.EmbeddedTerminal
	// previewFetching is true when an async tmux capture-pane is in progress
	previewFetching bool
	// previewGeneration increments on every navigation/instance change.
	// Async fetch results are discarded if their generation doesn't match.
	previewGeneration uint64
	// metadataFetching is true when the background metadata goroutine is running
	metadataFetching bool

	// navPosition tracks the current navigation stop for Shift+Arrow traversal.
	// 0=sidebar, 1=instances, 2=agent tab, 3=terminal tab, 4=diff tab, 5=git tab
	navPosition int

	// repoPickerMap maps picker display text to full repo path
	repoPickerMap map[string]string

	// brainServer is the IPC server for coordinating brain state between MCP agents
	brainServer *brain.Server
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

	primaryRepoPath, err := filepath.Abs(".")
	if err != nil {
		fmt.Printf("Failed to get current directory: %v\n", err)
		os.Exit(1)
	}

	h := &home{
		ctx:             ctx,
		spinner:         spinner.New(spinner.WithSpinner(spinner.Dot)),
		menu:            ui.NewMenu(),
		tabbedWindow:    ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewTerminalPane(), ui.NewDiffPane(), ui.NewGitPane()),
		storage:         storage,
		appConfig:       appConfig,
		program:         program,
		autoYes:         autoYes,
		state:           stateDefault,
		appState:        appState,
		primaryRepoPath: primaryRepoPath,
		activeRepoPaths: []string{primaryRepoPath},
	}
	h.list = ui.NewList(&h.spinner, autoYes)
	h.toastManager = overlay.NewToastManager(&h.spinner)
	h.sidebar = ui.NewSidebar()
	h.sidebar.SetRepoNames([]string{filepath.Base(primaryRepoPath)})
	h.setFocus(1) // Start with instance list focused

	// Load saved instances
	instances, err := storage.LoadInstances()
	if err != nil {
		fmt.Printf("Failed to load instances: %v\n", err)
		os.Exit(1)
	}

	h.allInstances = instances

	// Add instances matching active repos to the list
	for _, instance := range instances {
		if h.instanceMatchesActiveRepos(instance) {
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
			t.Path = primaryRepoPath
		}
	}
	h.allTopics = topics
	h.topics = h.filterTopicsByActiveRepos()
	h.updateSidebarItems()

	// Persist the active repo so it appears in the picker even if it has no instances
	if state, ok := h.appState.(*config.State); ok {
		state.AddRecentRepo(primaryRepoPath)
	}

	// Start brain IPC server for multi-agent coordination
	configDir, err := config.GetConfigDir()
	if err == nil {
		socketPath := filepath.Join(configDir, "hivemind.sock")
		h.brainServer = brain.NewServer(socketPath)
		if err := h.brainServer.Start(); err != nil {
			log.WarningLog.Printf("failed to start brain server: %v", err)
			h.brainServer = nil
		}
	}

	return h
}

// instanceMatchesActiveRepos checks if an instance belongs to any of the active repos.
func (m *home) instanceMatchesActiveRepos(inst *session.Instance) bool {
	repoPath := inst.GetRepoPath()
	if repoPath == "" {
		// Legacy instances without a repo path match the primary repo
		return m.activeRepoSet()[m.primaryRepoPath]
	}
	return m.activeRepoSet()[repoPath]
}

// activeRepoSet returns an O(1) lookup set of active repo paths.
func (m *home) activeRepoSet() map[string]bool {
	set := make(map[string]bool, len(m.activeRepoPaths))
	for _, rp := range m.activeRepoPaths {
		set[rp] = true
	}
	return set
}

// isMultiRepoView returns true when more than one repo is visible.
func (m *home) isMultiRepoView() bool {
	return len(m.activeRepoPaths) > 1
}

// updateHandleWindowSizeEvent sets the sizes of the components.
// The components will try to render inside their bounds.
func (m *home) updateHandleWindowSizeEvent(msg tea.WindowSizeMsg) {
	// Three-column layout: sidebar (19%), instance list (20% with 1-char gaps), preview (remaining).
	columnGap := 1 // gap on each side of the instance list
	sidebarWidth := int(float32(msg.Width) * 0.19)
	if sidebarWidth < 20 {
		sidebarWidth = 20
	}
	listAlloc := int(float32(msg.Width) * 0.20) // full allocation including gaps
	listWidth := listAlloc - 2*columnGap         // actual list content width
	tabsWidth := msg.Width - sidebarWidth - listAlloc

	// Menu takes 10% of height, list and window take 90%
	contentHeight := int(float32(msg.Height) * 0.9)
	menuHeight := msg.Height - contentHeight
	m.toastManager.SetSize(msg.Width, msg.Height)

	m.tabbedWindow.SetSize(tabsWidth, contentHeight)
	m.list.SetSize(listWidth, contentHeight)
	m.sidebar.SetSize(sidebarWidth, contentHeight)

	// Store for mouse hit-testing
	m.sidebarWidth = sidebarWidth
	m.listWidth = listAlloc // full allocation including gaps
	m.columnGap = columnGap
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
	cmds := []tea.Cmd{
		m.spinner.Tick,
		func() tea.Msg {
			time.Sleep(100 * time.Millisecond)
			return previewTickMsg{}
		},
		tickUpdateMetadataCmd,
		m.toastTickCmd(),
	}

	// Start polling the brain server's action channel for Tier 3 requests.
	if m.brainServer != nil {
		cmds = append(cmds, m.pollBrainActions())
	}

	return tea.Batch(cmds...)
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
		selected := m.list.GetSelectedInstance()
		// Diff uses cached stats — synchronous and fast
		m.tabbedWindow.UpdateDiff(selected)
		// Preview: cheap states handled synchronously, running instances fetched async
		previewCmd := m.asyncUpdatePreview(selected)
		return m, tea.Batch(previewCmd, func() tea.Msg {
			time.Sleep(100 * time.Millisecond)
			return previewTickMsg{}
		})
	case previewFetchedMsg:
		m.previewFetching = false
		// Discard result if user navigated since this fetch started
		if msg.generation != m.previewGeneration {
			return m, nil
		}
		if msg.err != nil {
			return m, m.handleError(msg.err)
		}
		m.tabbedWindow.ApplyPreviewContent(msg.content)
		m.tabbedWindow.ClearContentStale()
		return m, nil
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
			gitPane.WaitForRender(50 * time.Millisecond)
			return gitTabTickMsg{}
		}
	case terminalTabTickMsg:
		if !m.tabbedWindow.IsInTerminalTab() {
			return m, nil
		}
		termPane := m.tabbedWindow.GetTerminalPane()
		if !termPane.IsAttached() {
			return m, nil
		}
		content, changed := termPane.Render()
		if changed {
			m.tabbedWindow.SetTerminalContent(content)
		}
		return m, func() tea.Msg {
			termPane.WaitForRender(50 * time.Millisecond)
			return terminalTabTickMsg{}
		}
	case keyupMsg:
		m.menu.ClearKeydown()
		return m, nil
	case brainActionMsg:
		return m.handleBrainAction(msg.action)
	case tickUpdateMetadataMessage:
		if m.metadataFetching {
			return m, nil // previous tick still running, skip
		}
		m.metadataFetching = true
		instances := m.list.GetInstances()
		brainSrv := m.brainServer
		return m, func() tea.Msg {
			for _, instance := range instances {
				if !instance.Started() || instance.Paused() || instance.IsTmuxDead() || instance.Status == session.Loading {
					instance.LastActivity = nil
					continue
				}
				prevStatus := instance.Status
				updated, prompt := instance.HasUpdated()

				// Re-check: status may have changed to Paused/Loading while we were
				// doing I/O above (e.g. Pause() called concurrently). Don't overwrite.
				if instance.Paused() || instance.Status == session.Loading {
					continue
				}

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
				// Push event when status transitions between Running and Ready.
				pushStatusEvent(brainSrv, instance, prevStatus)
				if err := instance.UpdateDiffStats(); err != nil {
					log.WarningLog.Printf("could not update diff stats: %v", err)
				}
				instance.UpdateResourceUsage()
			}
			return metadataFetchedMsg{}
		}
	case metadataFetchedMsg:
		m.metadataFetching = false
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
	case brainInstanceStartedMsg:
		// Brain-spawned instance started — add to master list with its own finalizer.
		m.allInstances = append(m.allInstances, msg.instance)
		if err := m.saveAllInstances(); err != nil {
			return m, m.handleError(err)
		}
		if msg.finalizer != nil {
			msg.finalizer()
		}
		if m.autoYes {
			msg.instance.AutoYes = true
		}
		m.updateSidebarItems()
		return m, tea.Batch(tea.WindowSize(), m.instanceChanged())
	case brainInstanceFailedMsg:
		// Brain-spawned instance failed to start — remove from list.
		m.list.KillInstanceByTitle(msg.title)
		m.updateSidebarItems()
		return m, m.instanceChanged()
	case instanceResumedMsg:
		if msg.err != nil {
			if msg.wasDead {
				// Restart failed — revert to Running (tmuxDead is still set)
				msg.instance.SetStatus(session.Running)
			} else {
				// Resume failed — revert to Paused status
				msg.instance.SetStatus(session.Paused)
			}
			m.updateSidebarItems()
			return m, m.handleError(msg.err)
		}
		if err := m.saveAllInstances(); err != nil {
			return m, m.handleError(err)
		}
		m.updateSidebarItems()
		return m, tea.Batch(tea.WindowSize(), m.instanceChanged())
	case folderPickedMsg:
		m.state = stateDefault
		m.pickerOverlay = nil
		if msg.err != nil {
			return m, m.handleError(msg.err)
		}
		if msg.path != "" {
			m.addActiveRepo(msg.path)
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
	if m.brainServer != nil {
		m.brainServer.Stop()
	}
	m.killGitTab()
	m.tabbedWindow.GetTerminalPane().Kill()
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
	listColStyle := colStyle.PaddingLeft(m.columnGap).PaddingRight(m.columnGap)
	listWithPadding := listColStyle.Render(m.list.String())
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
	case m.state == stateNewTopicRepo && m.pickerOverlay != nil:
		result = overlay.PlaceOverlay(0, 0, m.pickerOverlay.Render(), mainView, true, true)
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

// previewFetchedMsg carries the result of an async tmux capture-pane call.
type previewFetchedMsg struct {
	content    string
	err        error
	generation uint64
}

type tickUpdateMetadataMessage struct{}

// metadataFetchedMsg signals that the background metadata goroutine completed.
type metadataFetchedMsg struct{}

// focusPreviewTickMsg is a fast ticker (30fps) for focus mode preview refresh only.
type focusPreviewTickMsg struct{}

// gitTabTickMsg is a 30fps ticker for refreshing the git tab's lazygit rendering.
type gitTabTickMsg struct{}

// terminalTabTickMsg is a 30fps ticker for refreshing the terminal tab rendering.
type terminalTabTickMsg struct{}

type instanceChangedMsg struct{}

// instanceStartedMsg is sent when an async instance startup completes.
type instanceStartedMsg struct {
	instance *session.Instance
	err      error
}

// instanceResumedMsg is sent when an async instance resume or restart completes.
type instanceResumedMsg struct {
	instance *session.Instance
	err      error
	// wasDead is true if this was a restart of a dead instance (not a resume from paused)
	wasDead bool
}

type keyupMsg struct{}

// brainActionMsg wraps a brain action request received from an agent via the brain server.
type brainActionMsg struct {
	action brain.ActionRequest
}

// brainInstanceStartedMsg is sent when a brain-spawned instance finishes starting.
// Unlike instanceStartedMsg, it carries its own finalizer so multiple concurrent
// brain spawns don't overwrite each other's finalizer via m.newInstanceFinalizer.
type brainInstanceStartedMsg struct {
	instance  *session.Instance
	finalizer func()
}

// brainInstanceFailedMsg is sent when a brain-spawned instance fails to start.
type brainInstanceFailedMsg struct {
	title string
}

// tickUpdateMetadataCmd is the callback to update the metadata of the instances every 500ms. Note that we iterate
// overall the instances and capture their output. It's a pretty expensive operation. Let's do it 2x a second only.
var tickUpdateMetadataCmd = func() tea.Msg {
	time.Sleep(500 * time.Millisecond)
	return tickUpdateMetadataMessage{}
}

// pushStatusEvent emits an EventInstanceStatusChanged when the instance transitions
// between Running and Ready. No-op if brainSrv is nil or the transition is not relevant.
func pushStatusEvent(brainSrv *brain.Server, instance *session.Instance, prevStatus session.Status) {
	if brainSrv == nil || instance.Status == prevStatus {
		return
	}
	isRelevant := (instance.Status == session.Ready && prevStatus == session.Running) ||
		(instance.Status == session.Running && prevStatus == session.Ready)
	if !isRelevant {
		return
	}

	var statusStr string
	switch instance.Status {
	case session.Ready:
		statusStr = "ready"
	case session.Running:
		statusStr = "running"
	default:
		statusStr = "unknown"
	}

	brainSrv.PushEvent(brain.Event{
		Type:     brain.EventInstanceStatusChanged,
		Source:   instance.Title,
		RepoPath: instance.GetRepoPath(),
		Data: map[string]any{
			"status":       statusStr,
			"parent_title": instance.ParentTitle,
		},
	})
}

// asyncUpdatePreview handles preview content fetching. Cheap instance states
// (nil, Loading, Paused, TmuxDead) are handled synchronously on the main thread.
// Running instances spawn a background goroutine for the expensive tmux capture-pane
// call so it doesn't block the Bubble Tea event loop.
func (m *home) asyncUpdatePreview(instance *session.Instance) tea.Cmd {
	if m.tabbedWindow.GetActiveTab() != ui.PreviewTab {
		// Not on preview tab — diff was already updated synchronously
		m.tabbedWindow.ClearContentStale()
		return nil
	}

	// Only running instances need the expensive tmux capture-pane subprocess.
	// All other states are handled synchronously since they're cheap (no I/O).
	needsAsync := instance != nil && instance.Started() &&
		instance.Status != session.Loading && instance.Status != session.Paused &&
		!instance.IsTmuxDead() && !m.tabbedWindow.IsPreviewInScrollMode()

	if !needsAsync {
		m.tabbedWindow.UpdatePreview(instance)
		m.tabbedWindow.ClearContentStale()
		return nil
	}

	// Skip if a fetch for this same generation is already in-flight
	if m.previewFetching {
		return nil
	}
	m.previewFetching = true
	gen := m.previewGeneration

	return func() tea.Msg {
		content, err := instance.Preview()
		return previewFetchedMsg{content: content, err: err, generation: gen}
	}
}

func (m *home) toastTickCmd() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(50 * time.Millisecond)
		return overlay.ToastTickMsg{}
	}
}
