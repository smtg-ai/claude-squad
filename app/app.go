package app

import (
	"claude-squad/config"
	"claude-squad/instance"
	"claude-squad/instance/interfaces"
	"claude-squad/instance/types"
	"claude-squad/keys"
	"claude-squad/registry"
	"claude-squad/ui"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
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

type tuiState int

const (
	tuiStateDefault tuiState = iota
	// tuiStateNew is the state when the user is creating a new instance.
	tuiStateNew
	// tuiStatePrompt is the state when the user is entering a prompt.
	tuiStatePrompt
	// tuiStateHelp is the state when a help screen is displayed.
	tuiStateHelp
)

type home struct {
	ctx context.Context

	// # State

	// state is the current discrete state of the app
	state tuiState
	// controller manages instances and orchestrators
	controller *controller
	// program is the program to use for instances and orchestrators
	program string
	// autoYes is whether to automatically approve actions
	autoYes bool
	// keySent is used to manage underlining menu items
	keySent bool

	// # UI components

	// menu is the menu UI component
	menu *ui.Menu
	// errBox is the error box UI component
	errBox *ui.ErrBox
	// spinner is the global spinner instance. We plumb this down to where it's needed
	spinner spinner.Model

	// # Storage and Configuration

	// storage is the interface for saving/loading data to/from the app's state
	storage *instance.Storage[interfaces.Instance]
	// appConfig stores persistent application configuration
	appConfig *config.Config
	// appState stores persistent application state like seen help screens
	appState config.AppState
}

func newHome(ctx context.Context, program string, autoYes bool) *home {
	appConfig := config.LoadConfig()
	appState := config.LoadState()

	// Create serialization functions for Instance interface
	toData := func(i interfaces.Instance) ([]byte, error) {
		return registry.MarshalInstanceWithType(i)
	}

	fromData := func(data []byte) (interfaces.Instance, error) {
		// Unmarshal the instance with type information from the registry
		var tagged types.TaggedInstance
		if err := json.Unmarshal(data, &tagged); err != nil {
			return nil, err
		}
		return registry.UnmarshalInstanceWithType(tagged)
	}

	getTitle := func(i interfaces.Instance) string {
		return i.StatusText()
	}

	storage := instance.NewStorage(appState, toData, fromData, getTitle)

	h := &home{
		ctx:       ctx,
		spinner:   spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		menu:      ui.NewMenu(),
		errBox:    ui.NewErrBox(),
		storage:   storage,
		appConfig: appConfig,
		program:   program,
		autoYes:   autoYes,
		state:     tuiStateDefault,
		appState:  appState,
	}

	controller := newController(&h.spinner, h.autoYes)
	if err := controller.LoadExistingInstances(h); err != nil {
		fmt.Printf("Warning: Failed to load existing instances: %v\n", err)
	}
	h.controller = controller

	return h
}

// View renders the UI using the instance mode directly.
func (m *home) View() string {
	return m.controller.Render(m)
}

// updateHandleWindowSizeEvent sets the sizes of the components.
// The components will try to render inside their bounds.
func (m *home) updateHandleWindowSizeEvent(msg tea.WindowSizeMsg) {
	// Menu takes 10% of height, list and window take 90%
	contentHeight := int(float32(msg.Height) * 0.9)
	menuHeight := msg.Height - contentHeight - 1     // minus 1 for error box
	m.errBox.SetSize(int(float32(msg.Width)*0.9), 1) // error box takes 1 row

	m.menu.SetSize(msg.Width, menuHeight)

	// Set sizes for instance mode components
	// Split the content width between list and preview
	// List takes ~40% of width, preview takes ~60%
	listWidth := int(float32(msg.Width) * 0.4)
	previewWidth := msg.Width - listWidth

	m.controller.list.SetSize(listWidth, contentHeight)
	m.controller.tabbedWindow.SetSize(previewWidth, contentHeight)
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
	return m.controller.Update(m, msg)
}

func (m *home) handleQuit() (tea.Model, tea.Cmd) {
	m.controller.HandleQuit(m)
	return m, tea.Quit
}

func (m *home) handleMenuHighlighting(msg tea.KeyMsg) (cmd tea.Cmd, returnEarly bool) {
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
