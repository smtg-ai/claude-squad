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
	if c.list.NumInstances() >= GlobalInstanceLimit {
		return model, model.handleError(fmt.Errorf("maximum number of instances (%d) reached", GlobalInstanceLimit))
	}

	c.promptAfterName = promptAfter
	model.state = tuiStatePrompt
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

	if c.textInputOverlay != nil && model.state == tuiStatePrompt {
		// Handle text input overlay
		switch msg.Type {
		case tea.KeyEnter:
			name := c.textInputOverlay.GetValue()
			c.textInputOverlay = nil
			model.state = tuiStateNew

			// Create a new task with TaskOptions
			options := task.TaskOptions{
				Program: model.program,
				Title:   name,
			}
			newTask, err := task.NewTask(options)
			if err != nil {
				return model, model.handleError(err)
			}
			c.newInstanceFinalizer = c.list.AddInstance(newTask)
			return model, tea.WindowSize()
		case tea.KeyEsc:
			c.textInputOverlay = nil
			model.state = tuiStateDefault
			return model, tea.WindowSize()
		default:
			// Let the text input overlay handle the key
			if c.textInputOverlay.HandleKeyPress(msg) {
				// If HandleKeyPress returns true, the overlay should be closed
				if c.textInputOverlay.IsSubmitted() {
					name := c.textInputOverlay.GetValue()
					c.textInputOverlay = nil
					model.state = tuiStateNew

					// Create a new task with TaskOptions
					options := task.TaskOptions{
						Program: model.program,
						Title:   name,
					}
					newTask, err := task.NewTask(options)
					if err != nil {
						return model, model.handleError(err)
					}
					c.newInstanceFinalizer = c.list.AddInstance(newTask)
					return model, tea.WindowSize()
				} else if c.textInputOverlay.IsCanceled() {
					c.textInputOverlay = nil
					model.state = (tuiStateDefault)
					return model, tea.WindowSize()
				}
			}
			return model, nil
		}
	}

	// If we're in the new instance state
	if model.state == tuiStateNew {
		switch msg.String() {
		case "enter":
			selected := c.list.GetSelectedInstance()
			if selected == nil {
				return model, nil
			}
			return c.finalizeNewInstance(model, selected)
		case "esc", "q":
			// Revert the new instance
			if c.newInstanceFinalizer != nil {
				c.newInstanceFinalizer()
				c.newInstanceFinalizer = nil
			}
			model.state = (tuiStateDefault)
			return model, tea.WindowSize()
		default:
			// Show help screen
			model.ShowHelpScreen(helpTypeInstanceStart, c.list.GetSelectedInstance(), nil, nil)
			return model, nil
		}
	}

	return model, nil
}

// finalizeNewInstance finalizes the creation of a new instance
func (c *Controller) finalizeNewInstance(model *Model, instance *task.Task) (tea.Model, tea.Cmd) {
	// Reset state
	model.state = (tuiStateDefault)

	// Start the instance with firstTimeSetup=true
	err := instance.Start(true)
	if err != nil {
		// If there's an error, revert the new instance
		if c.newInstanceFinalizer != nil {
			c.newInstanceFinalizer()
			c.newInstanceFinalizer = nil
		}
		return model, model.handleError(err)
	}

	// Call the finalizer to indicate we're done with the instance
	if c.newInstanceFinalizer != nil {
		c.newInstanceFinalizer()
		c.newInstanceFinalizer = nil
	}

	// If we should prompt after creating the instance, do so
	if c.promptAfterName {
		c.textInputOverlay = overlay.NewTextInputOverlay("Enter a prompt for the new instance", "")
		c.textInputOverlay.SetSize(80, 20)
		// Set up callbacks
		c.textInputOverlay.SetOnSubmit(func() {
			prompt := c.textInputOverlay.GetValue()
			model.state = (tuiStateDefault)
			c.textInputOverlay = nil
			// Send the prompt to the instance
			err := instance.SendPrompt(prompt)
			if err != nil {
				model.handleError(err)
			}
		})
		model.state = (tuiStatePrompt)
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

		// Then kill the instance
		c.list.Kill()
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
