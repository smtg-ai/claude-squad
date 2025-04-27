package app

import (
	"claude-squad/config"
	"claude-squad/keys"
	"claude-squad/log"
	"claude-squad/orchestrator"
	"claude-squad/session"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
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
}

type home struct {
	ctx context.Context

	mode         mode
	modeStrategy ModeStrategy

	program string
	autoYes bool

	// ui components
	list         *ui.List
	menu         *ui.Menu
	tabbedWindow *ui.TabbedWindow
	errBox       *ui.ErrBox
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
	// newInstanceFinalizer is called when the state is stateNew and then you press enter.
	// It registers the new instance in the list after the instance has been started.
	newInstanceFinalizer func()

	// promptAfterName tracks if we should enter prompt mode after naming
	promptAfterName bool

	// textInputOverlay is the component for handling text input with state
	textInputOverlay *overlay.TextInputOverlay

	// textOverlay is the component for displaying text information
	textOverlay *overlay.TextOverlay

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

	switch h.mode {
	case modeOrchestrate:
		h.modeStrategy = &orchestratorMode{}
	case modeInstance:
		fallthrough
	default:
		h.modeStrategy = &instanceMode{}
	}

	h.list = ui.NewList(&h.spinner, autoYes)

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
	// List takes 30% of width, preview takes 70%
	listWidth := int(float32(msg.Width) * 0.3)
	tabsWidth := msg.Width - listWidth

	// Menu takes 10% of height, list and window take 90%
	contentHeight := int(float32(msg.Height) * 0.9)
	menuHeight := msg.Height - contentHeight - 1     // minus 1 for error box
	m.errBox.SetSize(int(float32(msg.Width)*0.9), 1) // error box takes 1 row

	m.tabbedWindow.SetSize(tabsWidth, contentHeight)
	m.list.SetSize(listWidth, contentHeight)

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
	// Delegate event handling to the current mode's strategy
	return m.modeStrategy.Update(m, msg)
}

func (m *home) handleQuit() (tea.Model, tea.Cmd) {
	if err := m.storage.SaveInstances(m.list.GetInstances()); err != nil {
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
	if m.state == statePrompt || m.state == stateHelp {
		return nil, false
	}
	// If it's in the global keymap, we should try to highlight it.
	name, ok := getKeyMapForMode(m.mode)[msg.String()]
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

func (m *home) instanceChanged() tea.Cmd {
	// selected may be nil
	selected := m.list.GetSelectedInstance()

	m.tabbedWindow.UpdateDiff(selected)
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

func RunOrchestrator(ctx context.Context, program string, autoYes bool, prompt string, basePath string) error {
	fmt.Printf("Starting orchestrator with prompt: %s\n", prompt)

	// Create new orchestrator
	orch := orchestrator.NewOrchestrator(prompt, autoYes)

	// Set the program
	orch.SetProgram(program)

	// Format the orchestrator prompt
	orchestratorPrompt := fmt.Sprintf(`You are a project orchestrator. Your goal is to implement: %s

Break this goal down into manageable tasks that can be assigned to worker instances. I'll help you develop a plan, then you can create and manage worker instances to implement specific tasks.

You have these capabilities:
1. You can analyze the codebase to understand its structure.
2. You can create worker instances to implement specific tasks.
3. You can monitor worker progress and integrate their outputs.

To create a worker instance, say: CREATE_WORKER: <task_name> | <initial_prompt>
Example: CREATE_WORKER: implement-login | Implement a login form with email/password fields...

Workers will send you notifications when they need help or complete tasks.

Let's start by analyzing this goal and identifying the key components we need to build.`, prompt)

	orch.Prompt = orchestratorPrompt

	// Run the orchestrator
	if autoYes {
		// In auto mode, just run the orchestrator
		result, err := orch.Run(basePath)
		if err != nil {
			return fmt.Errorf("orchestrator failed: %w", err)
		}

		fmt.Printf("\nOrchestration completed successfully\n")
		fmt.Printf("Final merged changes:\n%s\n", result)
		return nil
	} else {
		// If not in auto mode, first divide the prompt into tasks
		tasks := orch.DividePrompt()

		// Display the plan to the user and let them modify it
		fmt.Printf("Orchestration plan:\n\n")
		for i, task := range tasks {
			fmt.Printf("Task %d: %s\n", i+1, task.Name)
			fmt.Printf("Prompt: %s\n\n", task.Prompt)
		}

		// TODO: Add interactive editing of the plan
		fmt.Printf("\nDo you want to proceed with this plan? (y/n): ")
		var response string
		fmt.Scanln(&response)

		if response != "y" && response != "Y" {
			return fmt.Errorf("orchestration cancelled by user")
		}

		// Set the plan
		orch.Plan = tasks

		// Run the orchestration
		result, err := orch.Run(basePath)
		if err != nil {
			return fmt.Errorf("orchestrator failed: %w", err)
		}

		fmt.Printf("\nOrchestration completed successfully\n")
		fmt.Printf("Final merged changes:\n%s\n", result)
		return nil
	}
}
