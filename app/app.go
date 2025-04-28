package app

import (
	"claude-squad/config"
	"claude-squad/keys"
	"claude-squad/log"
	"claude-squad/session"
	"claude-squad/ui"
	"context"
	"fmt"
	"os"
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

type state int

const (
	stateDefault state = iota
	// stateNew is the state when the user is creating a new instance.
	stateNew
	// statePrompt is the state when the user is entering a prompt.
	statePrompt
	// stateHelp is the state when a help screen is displayed.
	stateHelp
)

type mode int

const (
	modeInstance mode = iota
	modeOrchestrate
)

// ModeStrategy defines the interface for app modes.
type ModeStrategy interface {
	Render(*home) string
	Update(*home, tea.Msg) (tea.Model, tea.Cmd)
	HandleQuit(*home) (tea.Model, tea.Cmd)
}

type home struct {
	ctx context.Context

	mode         mode
	modeStrategy ModeStrategy

	program string
	autoYes bool

	// ui components
	menu   *ui.Menu
	errBox *ui.ErrBox
	// global spinner instance. we plumb this down to where it's needed
	spinner spinner.Model

	// storage is the interface for saving/loading data to/from the app's state
	storage *session.Storage
	// appConfig stores persistent application configuration
	appConfig *config.Config
	// appState stores persistent application state like seen help screens
	appState config.AppState

	// state
	state state
	// keySent is used to manage underlining menu items
	keySent bool
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
		ctx:       ctx,
		spinner:   spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		menu:      ui.NewMenu(),
		errBox:    ui.NewErrBox(),
		storage:   storage,
		appConfig: appConfig,
		program:   program,
		autoYes:   autoYes,
		state:     stateDefault,
		appState:  appState,
	}

	switch h.mode {
	case modeOrchestrate:
		h.modeStrategy = &orchestratorMode{}
	case modeInstance:
		fallthrough
	default:
		h.modeStrategy = newInstanceMode(&h.spinner, h.autoYes)
	}

	return h
}

// Renderer defines a function that renders the UI for a given home instance.
type Renderer func(*home) string

// -- Mode-specific renderers moved to instance_mode.go and orchestrator_mode.go --
// (instanceRenderer, orchestratorRenderer)

// View renders the UI based on the current mode using the appropriate renderer.
func (m *home) View() string {
	return m.modeStrategy.Render(m)
}

// updateHandleWindowSizeEvent sets the sizes of the components.
// The components will try to render inside their bounds.
func (m *home) updateHandleWindowSizeEvent(msg tea.WindowSizeMsg) {
	// Menu takes 10% of height, list and window take 90%
	contentHeight := int(float32(msg.Height) * 0.9)
	menuHeight := msg.Height - contentHeight - 1     // minus 1 for error box
	m.errBox.SetSize(int(float32(msg.Width)*0.9), 1) // error box takes 1 row

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
	return m.modeStrategy.Update(m, msg)
}

func (m *home) handleQuit() (tea.Model, tea.Cmd) {
	m.modeStrategy.HandleQuit(m)
	return m, tea.Quit
}

func (m *home) handleMenuHighlighting(msg tea.KeyMsg) (cmd tea.Cmd, returnEarly bool) {
	// Handle menu highlighting when you press a button. We intercept it here and immediately return to
	// update the ui while re-sending the keypress. Then, on the next call to this, we actually handle the keypress.
	if m.keySent {
		m.keySent = false
		return nil, false
	}
	if m.state == statePrompt || m.state == stateHelp {
		return nil, false
	}
	// If it's in the global keymap, we should try to highlight it.
	name, ok := getKeyMapForMode(m.mode)[msg.String()]
	if !ok {
		return nil, false
	}

	m.keySent = true
	return tea.Batch(
		func() tea.Msg { return msg },
		m.keydownCallback(name)), true
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

// getKeyMapForMode returns the keymap for the current mode
func getKeyMapForMode(mode mode) map[string]keys.KeyName {
	switch mode {
	case modeOrchestrate:
		return keys.OrchestratorModeKeyMap
	case modeInstance:
		fallthrough
	default:
		return keys.InstanceModeKeyMap
	}
}
