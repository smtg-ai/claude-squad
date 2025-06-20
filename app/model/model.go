package model

import (
	"claude-squad/log"
	"context"
	"encoding/json"
	"time"

	"claude-squad/config"
	"claude-squad/instance"
	instanceInterfaces "claude-squad/instance/interfaces"
	"claude-squad/instance/task"
	"claude-squad/instance/types"
	"claude-squad/keys"
	"claude-squad/registry"
	"claude-squad/ui"
	"claude-squad/ui/overlay"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type tuiState int

const (
	// tuiStateDefault is the default state
	tuiStateDefault tuiState = iota
	// tuiStateNew is the state when the user is creating a new instance.
	tuiStateNew
	// tuiStatePrompt is the state when the user is entering a prompt.
	tuiStatePrompt
	// tuiStateHelp is the state when a help screen is displayed.
	tuiStateHelp
	// tuiStateConfirm is the state when a confirmation overlay is displayed.
	tuiStateConfirm
)

// Model represents the application model
type Model struct {
	ctx context.Context

	// # State

	// state is the current discrete state of the app
	state tuiState
	// program is the program to use for instances
	program string
	// autoYes is whether to automatically approve actions
	autoYes bool
	// keySent is used to manage underlining menu items
	keySent bool
	// pendingAction stores an action to execute after confirmation
	pendingAction tea.Cmd

	// # UI components

	// menu displays the bottom menu
	menu *ui.Menu
	// errBox displays error messages
	errBox *ui.ErrBox
	// global spinner instance. we plumb this down to where it's needed
	spinner spinner.Model

	// # Storage and Configuration

	// storage is the interface for saving/loading data to/from the app's state
	storage *instance.Storage[instanceInterfaces.Instance]
	// appConfig stores persistent application configuration
	appConfig *config.Config
	// appState stores persistent application state like seen help screens
	appState config.AppState

	// Controller will be injected after creation to avoid circular dependency
	controller *Controller
}

// NewModel creates a new model
func NewModel(ctx context.Context, program string, autoYes bool) *Model {
	appConfig := config.LoadConfig()
	appState := config.LoadState()

	// Create serialization functions for Instance interface
	toData := func(i instanceInterfaces.Instance) ([]byte, error) {
		return registry.MarshalInstanceWithType(i)
	}

	fromData := func(data []byte) (instanceInterfaces.Instance, error) {
		// Unmarshal the instance with type information from the registry
		var tagged types.TaggedInstance
		if err := json.Unmarshal(data, &tagged); err != nil {
			return nil, err
		}
		return registry.UnmarshalInstanceWithType(tagged)
	}

	getTitle := func(i instanceInterfaces.Instance) string {
		if task, ok := i.(*task.Task); ok {
			return task.Title
		}
		return i.StatusText()
	}

	storage := instance.NewStorage(appState, toData, fromData, getTitle)

	spinner := spinner.New(spinner.WithSpinner(spinner.MiniDot))

	controller := NewController(&spinner, autoYes)

	h := &Model{
		ctx:        ctx,
		spinner:    spinner,
		menu:       ui.NewMenu(),
		errBox:     ui.NewErrBox(),
		storage:    storage,
		appConfig:  appConfig,
		program:    program,
		autoYes:    autoYes,
		state:      tuiStateDefault,
		appState:   appState,
		controller: controller,
	}

	// Load existing instances from storage
	if err := controller.LoadExistingInstances(storage); err != nil {
		// Log error but don't fail initialization
		// The app should still start even if loading instances fails
		log.ErrorLog.Printf("Failed to load existing instances: %v", err)
	}

	return h
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	// Upon starting, we want to start the spinner. Whenever we get a spinner.TickMsg, we
	// update the spinner, which sends a new spinner.TickMsg. I think this lasts forever lol.
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			time.Sleep(100 * time.Millisecond)
			return previewTickMsg{}
		},
		tickUpdateMetadataCmd,
		// Immediately update UI components with loaded instances
		func() tea.Msg {
			return instanceChangedMsg{}
		},
	)
}

// Update updates the model state
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.controller != nil {
		return m.controller.Update(m, msg)
	}
	return m, nil
}

// handleQuit handles quit events
func (m *Model) handleQuit() (tea.Model, tea.Cmd) {
	if m.controller != nil {
		m.controller.HandleQuit(m)
	}
	return m, tea.Quit
}

// View renders the UI using the controller
func (m *Model) View() string {
	return m.controller.Render(m)
}

// updateHandleWindowSizeEvent sets the sizes of the components.
// The components will try to render inside their bounds.
func (m *Model) updateHandleWindowSizeEvent(msg tea.WindowSizeMsg) {
	// Menu takes 10% of height, list and window take 90%
	contentHeight := int(float32(msg.Height) * 0.9)
	menuHeight := msg.Height - contentHeight - 1     // minus 1 for error box
	m.errBox.SetSize(int(float32(msg.Width)*0.9), 1) // error box takes 1 row

	m.menu.SetSize(msg.Width, menuHeight)

	// Set sizes for instance mode components
	if m.controller != nil {
		// Account for 8 units of horizontal padding (4 left + 4 right)
		availableWidth := msg.Width - 8

		// Split the content width between list and preview
		// List takes ~20% of width, preview takes ~80%
		listWidth := int(float32(availableWidth) * 0.3)
		previewWidth := availableWidth - listWidth

		m.controller.list.SetSize(listWidth, contentHeight)
		m.controller.tabbedWindow.SetSize(previewWidth, contentHeight)
	}
}

func (m *Model) handleMenuHighlighting(msg tea.KeyMsg) (cmd tea.Cmd, returnEarly bool) {
	// Handle menu highlighting when you press a button. We intercept it here and immediately return to
	// update the ui while re-sending the keypress. Then, on the next call to this, we actually handle the keypress.
	if m.keySent {
		m.keySent = false
		return nil, false
	}

	if m.state == tuiStatePrompt || m.state == tuiStateHelp {
		return nil, false
	}

	// If it's in the instance mode keymap, we should try to highlight it.
	name, ok := keys.InstanceModeKeyMap[msg.String()]
	if !ok {
		return nil, false
	}

	m.keySent = true
	return tea.Batch(
		func() tea.Msg { return msg },
		m.keydownCallback(name)), true
}

// ShowHelpScreen shows a help screen by calling the internal showHelpScreen method
func (m *Model) ShowHelpScreen(helpType helpType, instance interface{}, data interface{}, callback func()) {
	var taskPtr *task.Task
	var overlayPtr *overlay.TextOverlay

	if instance != nil {
		if t, ok := instance.(*task.Task); ok {
			taskPtr = t
		}
	}
	if data != nil {
		if o, ok := data.(*overlay.TextOverlay); ok {
			overlayPtr = o
		}
	}

	m.showHelpScreen(helpType, taskPtr, overlayPtr, callback)
}

// HandleMenuHighlighting handles menu highlighting by calling the internal method
func (m *Model) HandleMenuHighlighting(msg tea.KeyMsg) (tea.Cmd, bool) {
	return m.handleMenuHighlighting(msg)
}

// UpdateHandleWindowSizeEvent handles window size events
func (m *Model) UpdateHandleWindowSizeEvent(msg tea.WindowSizeMsg) {
	m.updateHandleWindowSizeEvent(msg)
}

// HandleQuit handles quit events
func (m *Model) HandleQuit() (tea.Model, tea.Cmd) {
	return m.handleQuit()
}

// HandleHelpState handles help state events
func (m *Model) HandleHelpState(msg tea.KeyMsg, textOverlay interface{}) (tea.Model, tea.Cmd) {
	if overlay, ok := textOverlay.(*overlay.TextOverlay); ok {
		return m.handleHelpState(msg, overlay)
	}
	return m, nil
}

// KeydownCallback handles keydown callbacks
func (m *Model) KeydownCallback(name string) tea.Cmd {
	if keyName, ok := keys.InstanceModeKeyMap[name]; ok {
		return m.keydownCallback(keyName)
	}
	return nil
}

// confirmAction shows a confirmation modal and stores the action to execute on confirm
func (m *Model) confirmAction(message string, action tea.Cmd) tea.Cmd {
	m.state = tuiStateConfirm
	m.pendingAction = action

	// Create and show the confirmation overlay using ConfirmationOverlay
	m.controller.confirmationOverlay = overlay.NewConfirmationOverlay(message)
	// Set a fixed width for consistent appearance
	m.controller.confirmationOverlay.SetWidth(50)

	// Set callbacks for confirmation and cancellation
	m.controller.confirmationOverlay.OnConfirm = func() {
		m.state = tuiStateDefault
		// Execute the pending action if it exists
		if m.pendingAction != nil {
			if msg := m.pendingAction(); msg != nil {
				// Handle the returned message directly
				switch msg := msg.(type) {
				case error:
					m.handleError(msg)
				case instanceChangedMsg:
					// Force a UI refresh by calling instanceChanged
					if cmd := m.controller.instanceChanged(m); cmd != nil {
						cmd()
					}
				}
			}
			m.pendingAction = nil
		}
	}

	m.controller.confirmationOverlay.OnCancel = func() {
		m.state = tuiStateDefault
		m.pendingAction = nil
	}

	return nil
}
