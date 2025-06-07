package model

import (
	instanceInterfaces "claude-squad/instance/interfaces"
	"claude-squad/instance/task"
	"claude-squad/keys"
	"claude-squad/log"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Global instance limit
const GlobalInstanceLimit = 10

type orchestratorState int

const (
	// orchestratorStateDefault is the default state for orchestrator
	orchestratorStateDefault orchestratorState = iota
	// orchestratorStatePrompt is the state when the user is entering a prompt for orchestrator
	orchestratorStatePrompt
	// orchestratorStatePlan is the state when the orchestrator plan is being displayed
	orchestratorStatePlan
)

// Controller manages instances and orchestrators
type Controller struct {
	// newInstanceFinalizer is called when the state is stateNew and then you press enter.
	// It registers the new instance in the list after the instance has been started.
	newInstanceFinalizer func()
	// promptAfterName tracks if we should enter prompt mode after naming
	promptAfterName bool
	// orchestratorState is the state of the orchestrator
	orchestratorState orchestratorState

	// instances is the list of instances being managed
	instances []instanceInterfaces.Instance

	// # UI Components

	// list displays the list of instances
	list *ui.List
	// tabbedWindow displays the tabbed window with preview and diff panes
	tabbedWindow *ui.TabbedWindow
	// textInputOverlay handles text input with state
	textInputOverlay *overlay.TextInputOverlay
	// textOverlay displays text information
	textOverlay *overlay.TextOverlay
	// confirmationOverlay displays confirmation modals
	confirmationOverlay *overlay.ConfirmationOverlay
}

// NewController creates a new controller
func NewController(spinner *spinner.Model, autoYes bool) *Controller {
	return &Controller{
		list:         ui.NewList(spinner, autoYes),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane()),
	}
}

// Render returns the rendered UI
func (c *Controller) Render(model *Model) string {
	listWithPadding := lipgloss.NewStyle().PaddingTop(1).Render(c.list.String())
	previewWithPadding := lipgloss.NewStyle().PaddingTop(1).Render(c.tabbedWindow.String())
	listAndPreview := lipgloss.JoinHorizontal(lipgloss.Top, listWithPadding, previewWithPadding)

	mainView := lipgloss.JoinVertical(
		lipgloss.Center,
		listAndPreview,
		model.menu.String(),
		model.errBox.String(),
	)

	if model.state == tuiStatePrompt {
		if c.textInputOverlay == nil {
			log.ErrorLog.Printf("text input overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, c.textInputOverlay.Render(), mainView, true, true)
	} else if model.state == tuiStateHelp {
		if c.textOverlay == nil {
			log.ErrorLog.Printf("text overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, c.textOverlay.Render(), mainView, true, true)
	}

	return mainView
}

// Update updates the controller state
func (c *Controller) Update(model *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case hideErrMsg:
		model.errBox.Clear()
	case previewTickMsg:
		cmd := c.instanceChanged(model)
		return model, tea.Batch(
			cmd,
			func() tea.Msg {
				time.Sleep(100 * time.Millisecond)
				return previewTickMsg{}
			},
		)
	case keyupMsg:
		model.menu.ClearKeydown()
		return model, nil
	case tickUpdateMetadataMessage:
		return model, c.handleMetadataUpdate()
	case tea.MouseMsg:
		return c.handleMouseEvent(model, msg)
	case tea.KeyMsg:
		return c.handleKeyEvent(model, msg)
	case tea.WindowSizeMsg:
		model.UpdateHandleWindowSizeEvent(msg)
		return model, nil
	case instanceChangedMsg:
		return model, c.instanceChanged(model)
	case spinner.TickMsg:
		spinner := model.spinner
		var cmd tea.Cmd
		_, cmd = spinner.Update(msg)
		return model, cmd
	}
	return model, nil
}

// handleMetadataUpdate updates the metadata for all instances
func (c *Controller) handleMetadataUpdate() tea.Cmd {
	for _, instance := range c.list.GetInstances() {
		if !instance.Started() || instance.Paused() {
			continue
		}
		updated, prompt := instance.HasUpdated()
		if updated {
			instance.SetStatus(task.Running)
		} else {
			if prompt {
				instance.TapEnter()
			} else {
				instance.SetStatus(task.Ready)
			}
		}
		if err := instance.UpdateDiffStats(); err != nil {
			log.WarningLog.Printf("could not update diff stats: %v", err)
		}
	}
	return tickUpdateMetadataCmd
}

// handleMouseEvent handles mouse events
func (c *Controller) handleMouseEvent(model *Model, msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Handle mouse wheel scrolling in the diff view
	if c.tabbedWindow.IsInDiffTab() {
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				c.tabbedWindow.ScrollUp()
				return model, c.instanceChanged(model)
			case tea.MouseButtonWheelDown:
				c.tabbedWindow.ScrollDown()
				return model, c.instanceChanged(model)
			default:
				break
			}
		}
	}
	return model, nil
}

// handleKeyEvent handles key events
func (c *Controller) handleKeyEvent(model *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle prompt state key events
	if model.state == tuiStatePrompt && c.textInputOverlay != nil {
		return c.handlePromptKeyEvent(model, msg)
	}

	// Handle other key events
	return c.handleKeyPress(model, msg)
}

// handlePromptKeyEvent handles prompt key events
func (c *Controller) handlePromptKeyEvent(model *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	shouldClose := c.textInputOverlay.HandleKeyPress(msg)
	if !shouldClose {
		return model, nil
	}

	if c.textInputOverlay.IsSubmitted() {
		if c.orchestratorState == orchestratorStatePrompt {
			// Handle orchestrator prompt - generate plan first
			prompt := c.textInputOverlay.GetValue()
			c.textInputOverlay = nil
			c.orchestratorState = orchestratorStatePrompt
			return c.generateOrchestratorPlan(model, prompt)
		} else {
			// Handle regular prompt for selected instance
			selected := c.list.GetSelectedInstance()
			if selected != nil {
				if err := selected.SendPrompt(c.textInputOverlay.GetValue()); err != nil {
					return model, model.handleError(err)
				}
			}
		}
	}

	// Close the overlay and reset state
	c.textInputOverlay = nil
	// c.isOrchestratorPrompt = false
	model.state = tuiStateDefault
	return model, tea.Sequence(
		tea.WindowSize(),
		func() tea.Msg {
			model.menu.SetState(ui.StateDefault)
			model.ShowHelpScreen(helpTypeInstanceStart, nil, nil, nil)
			return nil
		},
	)
}

// handleKeyPress handles key presses
func (c *Controller) handleKeyPress(model *Model, msg tea.KeyMsg) (mod tea.Model, cmd tea.Cmd) {
	cmd, returnEarly := model.HandleMenuHighlighting(msg)
	if returnEarly {
		return model, cmd
	}

	if model.state == tuiStateHelp {
		// // Check if we're showing an orchestrator plan for approval
		// if c.orchestratorPlan != "" && c.textOverlay != nil {
		// 	return c.handleOrchestratorPlanKeyPress(model, msg)
		// }
		return model.HandleHelpState(msg, c.textOverlay)
	}

	if model.state == tuiStateNew {
		return c.handleNewInstanceState(model, msg)
	}

	// Handle confirmation state
	if model.state == tuiStateConfirm {
		shouldClose := c.confirmationOverlay.HandleKeyPress(msg)
		if shouldClose {
			model.state = tuiStateDefault
			c.confirmationOverlay = nil
			return model, nil
		}
		return model, nil
	}

	// Handle quit commands first
	if msg.String() == "ctrl+c" || msg.String() == "q" {
		return model.HandleQuit()
	}

	name, ok := keys.InstanceModeKeyMap[msg.String()]
	if !ok {
		return model, nil
	}

	switch name {
	case keys.KeyHelp:
		return model, tea.Cmd(func() tea.Msg {
			model.ShowHelpScreen(helpTypeGeneral, nil, nil, nil)
			return nil
		})
	case keys.KeyPrompt, keys.KeyNew:
		return c.handleNewTask(model, name == keys.KeyPrompt)
	case keys.KeyOrchestrator:
		return c.handleNewOrchestrator(model)
	case keys.KeyUp:
		c.list.Up()
		return model, c.instanceChanged(model)
	case keys.KeyDown:
		c.list.Down()
		return model, c.instanceChanged(model)
	case keys.KeyShiftUp:
		if c.tabbedWindow.IsInDiffTab() {
			c.tabbedWindow.ScrollUp()
		}
		return model, c.instanceChanged(model)
	case keys.KeyShiftDown:
		if c.tabbedWindow.IsInDiffTab() {
			c.tabbedWindow.ScrollDown()
		}
		return model, c.instanceChanged(model)
	case keys.KeyTab:
		c.tabbedWindow.Toggle()
		model.menu.SetInDiffTab(c.tabbedWindow.IsInDiffTab())
		return model, c.instanceChanged(model)
	case keys.KeyKill:
		return c.handleKillInstance(model)
	case keys.KeySubmit:
		return c.handleSubmitChanges(model)
	case keys.KeyCheckout:
		return c.handleCheckoutInstance(model)
	case keys.KeyResume:
		return c.handleResumeInstance(model)
	case keys.KeyEnter:
		return c.handleAttachInstance(model)
	default:
		return model, nil
	}
}

// HandleQuit handles the quit action
func (c *Controller) HandleQuit(model *Model) {
	if err := model.storage.SaveInstances(c.instances); err != nil {
		model.handleError(err)
	}
}
