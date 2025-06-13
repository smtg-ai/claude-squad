package model

import (
	"claude-squad/instance"
	instanceInterfaces "claude-squad/instance/interfaces"
	"claude-squad/instance/task"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// LoadExistingInstances loads instances from storage into the list
func (c *Controller) LoadExistingInstances(storage *instance.Storage[instanceInterfaces.Instance]) error {
	instances, err := storage.LoadInstances()
	if err != nil {
		return err
	}

	for _, instance := range instances {
		finalizer := c.list.AddInstance(instance.(*task.Task))
		finalizer() // Call finalizer immediately since instance is already started
	}

	c.instances = instances

	return nil
}

// handleNewTask creates a new task
func (c *Controller) handleNewTask(model *Model, promptAfter bool) (tea.Model, tea.Cmd) {
	// Check if we've hit the instance limit
	if c.list.NumInstances() >= globalInstanceLimit {
		return model, model.handleError(fmt.Errorf("maximum number of instances (%d) reached", globalInstanceLimit))
	}

	c.promptAfterName = promptAfter
	model.state = tuiStateNew
	model.menu.SetState(ui.StatePrompt)

	// Create a new task immediately with default name
	options := task.TaskOptions{
		Program: model.program,
		Title:   "",
		Path:    ".",
	}
	newTask, err := task.NewTask(options)
	if err != nil {
		return model, model.handleError(err)
	}
	c.newInstanceFinalizer = c.list.AddInstance(newTask)

	return model, tea.WindowSize()
}

// handleNewInstanceState handles the state when a new instance is being created
func (c *Controller) handleNewInstanceState(model *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle quit commands first. Don't handle q because the user might want to type that.
	if msg.String() == "ctrl+c" {
		model.state = tuiStateDefault
		c.promptAfterName = false
		c.list.Kill()
		return model, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				model.menu.SetState(ui.StateDefault)
				return nil
			},
		)
	}

	// Handle escape to cancel instance creation
	if msg.String() == "esc" {
		// Revert the new instance
		if c.newInstanceFinalizer != nil {
			c.newInstanceFinalizer()
			c.newInstanceFinalizer = nil
		}
		filtered := c.instances[:0]
		for _, inst := range c.instances {
			if inst.IsRunning() {
				filtered = append(filtered, inst)
			}
		}
		c.instances = filtered
		model.state = tuiStateDefault
		model.menu.SetState(ui.StateDefault)
		return model, tea.WindowSize()
	}

	// Handle enter to finalize instance
	if msg.String() == "enter" {
		selected := c.list.GetSelectedInstance()
		if selected == nil {
			return model, nil
		}
		return c.finalizeNewInstance(model, selected)
	}

	// Handle backspace
	if msg.String() == "backspace" {
		selected := c.list.GetSelectedInstance()
		if selected != nil && len(selected.Title) > 0 {
			selected.Title = selected.Title[:len(selected.Title)-1]
		}
		return model, nil
	}

	// Handle regular character input
	if len(msg.String()) == 1 && msg.String() >= " " && msg.String() <= "~" {
		selected := c.list.GetSelectedInstance()
		if selected != nil {
			// Clear default name on first character
			if selected.Title == "New Instance" {
				selected.Title = ""
			}
			selected.Title += msg.String()
		}
		return model, nil
	}

	// Show help screen for unhandled keys
	model.ShowHelpScreen(helpTypeInstanceStart, c.list.GetSelectedInstance(), nil, nil)
	return model, nil
}

// finalizeNewInstance finalizes the creation of a new instance
func (c *Controller) finalizeNewInstance(model *Model, instance *task.Task) (tea.Model, tea.Cmd) {
	// Reset state
	model.state = tuiStateDefault
	model.menu.SetState(ui.StateDefault)

	// Start the instance with firstTimeSetup=true
	err := instance.Start(true)
	if err != nil {
		// If there's an error, delete the instance from the list and revert
		c.list.Kill()
		if c.newInstanceFinalizer != nil {
			c.newInstanceFinalizer = nil
		}
		return model, model.handleError(err)
	}

	// Call the finalizer to indicate we're done with the instance
	if c.newInstanceFinalizer != nil {
		c.newInstanceFinalizer()
		c.newInstanceFinalizer = nil
	}

	// Add the successfully started instance to our instances slice for saving
	c.instances = append(c.instances, instance)

	// If we should prompt after creating the instance, do so
	if c.promptAfterName {
		c.textInputOverlay = overlay.NewTextInputOverlay("Enter a prompt for the new instance", "")
		c.textInputOverlay.SetSize(80, 20)
		// Set up callbacks
		c.textInputOverlay.SetOnSubmit(func() {
			prompt := c.textInputOverlay.GetValue()
			model.state = tuiStateDefault
			model.menu.SetState(ui.StateDefault)
			c.textInputOverlay = nil
			// Send the prompt to the instance
			err := instance.SendPrompt(prompt)
			if err != nil {
				model.handleError(err)
			}
		})
		model.state = tuiStatePrompt
		model.menu.SetState(ui.StatePrompt)
	}

	return model, tea.WindowSize()
}

// handleKillInstance kills the selected instance
func (c *Controller) handleKillInstance(model *Model) (tea.Model, tea.Cmd) {
	selected := c.list.GetSelectedInstance()
	if selected == nil {
		return model, nil
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
		if err := model.storage.DeleteInstance(selected.Title); err != nil {
			return err
		}

		// Then kill the instance from the list
		c.list.Kill()

		// Also remove from our instances slice
		for i, inst := range c.instances {
			if inst.(*task.Task).Title == selected.Title {
				c.instances = append(c.instances[:i], c.instances[i+1:]...)
				break
			}
		}

		return instanceChangedMsg{}
	}

	// Show confirmation modal
	message := fmt.Sprintf("[!] Kill session '%s'?", selected.Title)
	return model, model.confirmAction(message, killAction)
}

// handleSubmitChanges submits changes to the selected instance
func (c *Controller) handleSubmitChanges(model *Model) (tea.Model, tea.Cmd) {
	selected := c.list.GetSelectedInstance()
	if selected == nil || selected.Paused() {
		return model, nil
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
	return model, model.confirmAction(message, pushAction)
}

// handleCheckoutInstance checks out the selected instance
func (c *Controller) handleCheckoutInstance(model *Model) (tea.Model, tea.Cmd) {
	selected := c.list.GetSelectedInstance()
	if selected == nil {
		return model, nil
	}

	// Show help screen before pausing
	model.ShowHelpScreen(helpTypeInstanceCheckout, selected, nil, func() {
		if err := selected.Pause(); err != nil {
			model.handleError(err)
		}
		c.instanceChanged(model)
	})
	return model, nil
}

// handleResumeInstance resumes the selected instance
func (c *Controller) handleResumeInstance(model *Model) (tea.Model, tea.Cmd) {
	selected := c.list.GetSelectedInstance()
	if selected == nil {
		return model, nil
	}
	if err := selected.Resume(); err != nil {
		return model, model.handleError(err)
	}
	return model, tea.WindowSize()
}

// handleAttachInstance attaches to the selected instance
func (c *Controller) handleAttachInstance(model *Model) (tea.Model, tea.Cmd) {
	if c.list.NumInstances() == 0 {
		return model, nil
	}
	selected := c.list.GetSelectedInstance()
	if selected == nil || selected.Paused() || !selected.TmuxAlive() {
		return model, nil
	}
	// Show help screen before attaching
	model.ShowHelpScreen(helpTypeInstanceAttach, selected, nil, func() {
		ch, err := c.list.Attach()
		if err != nil {
			model.handleError(err)
			return
		}
		<-ch
		model.state = (tuiStateDefault)
	})
	return model, nil
}

// instanceChanged updates the UI when the selected instance changes
func (c *Controller) instanceChanged(model *Model) tea.Cmd {
	// selected may be nil
	selected := c.list.GetSelectedInstance()

	c.tabbedWindow.UpdateDiff(selected)
	// Update menu with current instance
	model.menu.SetInstance(selected)

	// If there's no selected instance, we don't need to update the preview.
	if err := c.tabbedWindow.UpdatePreview(selected); err != nil {
		return model.handleError(err)
	}
	return nil
}
