package model

import (
	instanceInterfaces "claude-squad/instance/interfaces"
	"claude-squad/instance/task"
	"claude-squad/instance/task/git"
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
const globalInstanceLimit = 10

// Controller manages instances
type Controller struct {
	// newInstanceFinalizer is called when the state is stateNew and then you press enter.
	// It registers the new instance in the list after the instance has been started.
	newInstanceFinalizer func()
	// promptAfterName tracks if we should enter prompt mode after naming
	promptAfterName bool

	// instances is the list of instances being managed - this is the source of truth
	instances []instanceInterfaces.Instance

	// # UI Components

	// list displays the list of instances - observes changes to instances
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
	c := &Controller{
		list:         ui.NewList(spinner, autoYes),
		tabbedWindow: ui.NewTabbedWindow(ui.NewPreviewPane(), ui.NewDiffPane()),
	}
	// Initialize with empty instances
	c.notifyInstancesChanged()
	return c
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
		return overlay.PlaceOverlay(0, 0, c.textInputOverlay.Render(), mainView, true, true)
	} else if model.state == tuiStateHelp {
		return overlay.PlaceOverlay(0, 0, c.textOverlay.Render(), mainView, true, true)
	} else if model.state == tuiStateConfirm {
		return overlay.PlaceOverlay(0, 0, c.confirmationOverlay.Render(), mainView, true, true)
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
	for _, instance := range c.instances {
		taskInstance := instance.(*task.Task)
		if !taskInstance.Started() || taskInstance.Paused() {
			continue
		}
		updated, prompt := taskInstance.HasUpdated()
		if updated {
			taskInstance.SetStatus(task.Running)
		} else {
			if prompt {
				taskInstance.TapEnter()
			} else {
				taskInstance.SetStatus(task.Ready)
			}
		}
		if err := taskInstance.UpdateDiffStats(); err != nil {
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
	if model.state == tuiStatePrompt {
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
		// Handle regular prompt for selected instance
		selected := c.list.GetSelectedInstance()
		if selected != nil {
			taskInstance := selected.(*task.Task)
			if err := taskInstance.SendPrompt(c.textInputOverlay.GetValue()); err != nil {
				return model, model.handleError(err)
			}
		}
	}

	// Close the overlay and reset state
	c.textInputOverlay = nil
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
		return model.HandleHelpState(msg, c.textOverlay)
	}

	if model.state == tuiStateNew {
		return c.handleNewInstanceState(model, msg)
	}

	// Handle confirmation state
	if model.state == tuiStateConfirm {
		if c.confirmationOverlay != nil {
			shouldClose := c.confirmationOverlay.HandleKeyPress(msg)
			if shouldClose {
				model.state = tuiStateDefault
				c.confirmationOverlay = nil
				return model, nil
			}
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

// convertToUIStatus converts task.Status to ui.InstanceStatus
func convertToUIStatus(status task.Status) ui.InstanceStatus {
	switch status {
	case task.Running:
		return ui.InstanceRunning
	case task.Ready:
		return ui.InstanceReady
	case task.Paused:
		return ui.InstancePaused
	case task.Loading:
		return ui.InstanceLoading
	default:
		return ui.InstanceReady
	}
}

// convertDiffStats converts git.DiffStats to ui.DiffStats
func convertDiffStats(stats *git.DiffStats) *ui.DiffStats {
	if stats == nil {
		return nil
	}
	return &ui.DiffStats{
		Added:   stats.Added,
		Removed: stats.Removed,
		Error:   stats.Error,
	}
}

// notifyInstancesChanged converts instances and notifies the UI list
func (c *Controller) notifyInstancesChanged() {
	renderData := make([]ui.InstanceRenderData, len(c.instances))
	repos := make(map[string]int)

	for i, instance := range c.instances {
		taskInstance := instance.(*task.Task)

		// Convert to rendering data
		renderData[i] = ui.InstanceRenderData{
			Title:     taskInstance.Title,
			Branch:    taskInstance.Branch,
			Status:    convertToUIStatus(taskInstance.Status),
			DiffStats: convertDiffStats(taskInstance.GetDiffStats()),
			IsStarted: taskInstance.Started(),
		}

		// Add repo name if instance is started
		if taskInstance.Started() {
			if repoName, err := taskInstance.RepoName(); err == nil {
				renderData[i].RepoName = repoName
				repos[repoName]++
			}
		}
	}

	c.list.OnInstancesChanged(c.instances)
	c.list.SetRenderData(renderData, repos)
}

// addInstance adds an instance to the instances slice and notifies observers
func (c *Controller) addInstance(instance instanceInterfaces.Instance) {
	c.instances = append(c.instances, instance)
	c.notifyInstancesChanged()
}

// removeInstance removes an instance from the instances slice and notifies observers
func (c *Controller) removeInstance(title string) {
	for i, inst := range c.instances {
		if inst.(*task.Task).Title == title {
			c.instances = append(c.instances[:i], c.instances[i+1:]...)
			c.notifyInstancesChanged()
			break
		}
	}
}
