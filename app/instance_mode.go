package app

import (
	"claude-squad/keys"
	"claude-squad/log"
	"claude-squad/session"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// instanceMode implements ModeStrategy for instance mode.
type instanceMode struct{}

func (im *instanceMode) Render(h *home) string {
	listWithPadding := lipgloss.NewStyle().PaddingTop(1).Render(h.list.String())
	previewWithPadding := lipgloss.NewStyle().PaddingTop(1).Render(h.tabbedWindow.String())
	listAndPreview := lipgloss.JoinHorizontal(lipgloss.Top, listWithPadding, previewWithPadding)

	mainView := lipgloss.JoinVertical(
		lipgloss.Center,
		listAndPreview,
		h.menu.String(),
		h.errBox.String(),
	)

	if h.state == statePrompt {
		if h.textInputOverlay == nil {
			log.ErrorLog.Printf("text input overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, h.textInputOverlay.Render(), mainView, true, true)
	} else if h.state == stateHelp {
		if h.textOverlay == nil {
			log.ErrorLog.Printf("text overlay is nil")
		}
		return overlay.PlaceOverlay(0, 0, h.textOverlay.Render(), mainView, true, true)
	}

	return mainView
}

func (im *instanceMode) Update(h *home, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case hideErrMsg:
		h.errBox.Clear()
	case previewTickMsg:
		cmd := im.instanceChanged(h)
		return h, tea.Batch(
			cmd,
			func() tea.Msg {
				time.Sleep(100 * time.Millisecond)
				return previewTickMsg{}
			},
		)
	case keyupMsg:
		h.menu.ClearKeydown()
		return h, nil
	case tickUpdateMetadataMessage:
		for _, instance := range h.list.GetInstances() {
			if !instance.Started() || instance.Paused() {
				continue
			}
			updated, prompt := instance.HasUpdated()
			if updated {
				instance.SetStatus(session.Running)
			} else {
				if prompt {
					instance.TapEnter()
				} else {
					instance.SetStatus(session.Ready)
				}
			}
			if err := instance.UpdateDiffStats(); err != nil {
				log.WarningLog.Printf("could not update diff stats: %v", err)
			}
		}
		return h, tickUpdateMetadataCmd
	case tea.MouseMsg:
		// Handle mouse wheel scrolling in the diff view
		if h.tabbedWindow.IsInDiffTab() {
			if msg.Action == tea.MouseActionPress {
				switch msg.Button {
				case tea.MouseButtonWheelUp:
					h.tabbedWindow.ScrollUp()
					return h, im.instanceChanged(h)
				case tea.MouseButtonWheelDown:
					h.tabbedWindow.ScrollDown()
					return h, im.instanceChanged(h)
				}
			}
		}
		return h, nil
	case tea.KeyMsg:
		return im.handleKeyPress(h, msg)
	case tea.WindowSizeMsg:
		h.updateHandleWindowSizeEvent(msg)
		return h, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		h.spinner, cmd = h.spinner.Update(msg)
		return h, cmd
	}
	return h, nil
}

func (im *instanceMode) handleKeyPress(h *home, msg tea.KeyMsg) (mod tea.Model, cmd tea.Cmd) {
	cmd, returnEarly := h.handleMenuHighlighting(msg)
	if returnEarly {
		return h, cmd
	}

	if h.state == stateHelp {
		return h.handleHelpState(msg)
	}

	if h.state == stateNew {
		// Handle quit commands first. Don't handle q because the user might want to type that.
		if msg.String() == "ctrl+c" {
			h.state = stateDefault
			h.promptAfterName = false
			h.list.Kill()
			return h, tea.Sequence(
				tea.WindowSize(),
				func() tea.Msg {
					h.menu.SetState(ui.StateDefault)
					return nil
				},
			)
		}

		instance := h.list.GetInstances()[h.list.NumInstances()-1]
		switch msg.Type {
		case tea.KeyEnter:
			if len(instance.Title) == 0 {
				return h, h.handleError(fmt.Errorf("title cannot be empty"))
			}

			if err := instance.Start(true); err != nil {
				h.list.Kill()
				h.state = stateDefault
				return h, h.handleError(err)
			}
			// Save after adding new instance
			if err := h.storage.SaveInstances(h.list.GetInstances()); err != nil {
				return h, h.handleError(err)
			}
			// Instance added successfully, call the finalizer.
			h.newInstanceFinalizer()
			if h.autoYes {
				instance.AutoYes = true
			}

			h.newInstanceFinalizer()
			h.state = stateDefault
			if h.promptAfterName {
				h.state = statePrompt
				h.menu.SetState(ui.StatePrompt)
				// Initialize the text input overlay
				h.textInputOverlay = overlay.NewTextInputOverlay("Enter prompt", "")
				h.promptAfterName = false
			} else {
				h.menu.SetState(ui.StateDefault)
				h.showHelpScreen(helpTypeInstanceStart, nil)
			}

			return h, tea.Batch(tea.WindowSize(), im.instanceChanged(h))
		case tea.KeyRunes:
			if len(instance.Title) >= 32 {
				return h, h.handleError(fmt.Errorf("title cannot be longer than 32 characters"))
			}
			if err := instance.SetTitle(instance.Title + string(msg.Runes)); err != nil {
				return h, h.handleError(err)
			}
		case tea.KeyBackspace:
			if len(instance.Title) == 0 {
				return h, nil
			}
			if err := instance.SetTitle(instance.Title[:len(instance.Title)-1]); err != nil {
				return h, h.handleError(err)
			}
		case tea.KeySpace:
			if err := instance.SetTitle(instance.Title + " "); err != nil {
				return h, h.handleError(err)
			}
		case tea.KeyEsc:
			h.list.Kill()
			h.state = stateDefault
			im.instanceChanged(h)

			return h, tea.Sequence(
				tea.WindowSize(),
				func() tea.Msg {
					h.menu.SetState(ui.StateDefault)
					return nil
				},
			)
		default:
		}
		return h, nil
	} else if h.state == statePrompt {
		// Use the new TextInputOverlay component to handle all key events
		shouldClose := h.textInputOverlay.HandleKeyPress(msg)

		// Check if the form was submitted or canceled
		if shouldClose {
			if h.textInputOverlay.IsSubmitted() {
				// Form was submitted, process the input
				selected := h.list.GetSelectedInstance()
				if selected == nil {
					return h, nil
				}
				if err := selected.SendPrompt(h.textInputOverlay.GetValue()); err != nil {
					return h, h.handleError(err)
				}
			}

			// Close the overlay and reset state
			h.textInputOverlay = nil
			h.state = stateDefault
			return h, tea.Sequence(
				tea.WindowSize(),
				func() tea.Msg {
					h.menu.SetState(ui.StateDefault)
					h.showHelpScreen(helpTypeInstanceStart, nil)
					return nil
				},
			)
		}

		return h, nil
	}

	// Handle quit commands first
	if msg.String() == "ctrl+c" || msg.String() == "q" {
		return h.handleQuit()
	}

	name, ok := getKeyMapForMode(h.mode)[msg.String()]
	if !ok {
		return h, nil
	}

	switch name {
	case keys.KeyHelp:
		return h.showHelpScreen(helpTypeGeneral, nil)
	case keys.KeyPrompt:
		if h.list.NumInstances() >= GlobalInstanceLimit {
			return h, h.handleError(
				fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
		}
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "",
			Path:    ".",
			Program: h.program,
		})
		if err != nil {
			return h, h.handleError(err)
		}

		h.newInstanceFinalizer = h.list.AddInstance(instance)
		h.list.SetSelectedInstance(h.list.NumInstances() - 1)
		h.state = stateNew
		h.menu.SetState(ui.StateNewInstance)
		h.promptAfterName = true

		return h, nil
	case keys.KeyNew:
		if h.list.NumInstances() >= GlobalInstanceLimit {
			return h, h.handleError(
				fmt.Errorf("you can't create more than %d instances", GlobalInstanceLimit))
		}
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "",
			Path:    ".",
			Program: h.program,
		})
		if err != nil {
			return h, h.handleError(err)
		}

		h.newInstanceFinalizer = h.list.AddInstance(instance)
		h.list.SetSelectedInstance(h.list.NumInstances() - 1)
		h.state = stateNew
		h.menu.SetState(ui.StateNewInstance)

		return h, nil
	case keys.KeyMode:
		// Switch to orchestrator mode
		h.mode = modeOrchestrate
		h.modeStrategy = &orchestratorMode{}
		return h, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				h.menu.SetState(ui.StateDefault)
				return nil
			},
		)
	case keys.KeyUp:
		h.list.Up()
		return h, im.instanceChanged(h)
	case keys.KeyDown:
		h.list.Down()
		return h, im.instanceChanged(h)
	case keys.KeyShiftUp:
		if h.tabbedWindow.IsInDiffTab() {
			h.tabbedWindow.ScrollUp()
		}
		return h, im.instanceChanged(h)
	case keys.KeyShiftDown:
		if h.tabbedWindow.IsInDiffTab() {
			h.tabbedWindow.ScrollDown()
		}
		return h, im.instanceChanged(h)
	case keys.KeyTab:
		h.tabbedWindow.Toggle()
		h.menu.SetInDiffTab(h.tabbedWindow.IsInDiffTab())
		return h, im.instanceChanged(h)
	case keys.KeyKill:
		selected := h.list.GetSelectedInstance()
		if selected == nil {
			return h, nil
		}

		worktree, err := selected.GetGitWorktree()
		if err != nil {
			return h, h.handleError(err)
		}

		checkedOut, err := worktree.IsBranchCheckedOut()
		if err != nil {
			return h, h.handleError(err)
		}

		if checkedOut {
			return h, h.handleError(fmt.Errorf("instance %s is currently checked out", selected.Title))
		}

		// Delete from storage first
		if err := h.storage.DeleteInstance(selected.Title); err != nil {
			return h, h.handleError(err)
		}

		// Then kill the instance
		h.list.Kill()
		return h, im.instanceChanged(h)
	case keys.KeySubmit:
		selected := h.list.GetSelectedInstance()
		if selected == nil {
			return h, nil
		}

		// Default commit message with timestamp
		commitMsg := fmt.Sprintf("[claudesquad] update from '%s' on %s", selected.Title, time.Now().Format(time.RFC822))
		worktree, err := selected.GetGitWorktree()
		if err != nil {
			return h, h.handleError(err)
		}
		if err = worktree.PushChanges(commitMsg, true); err != nil {
			return h, h.handleError(err)
		}

		return h, nil
	case keys.KeyCheckout:
		selected := h.list.GetSelectedInstance()
		if selected == nil {
			return h, nil
		}

		// Show help screen before pausing
		h.showHelpScreen(helpTypeInstanceCheckout, func() {
			if err := selected.Pause(); err != nil {
				h.handleError(err)
			}
			im.instanceChanged(h)
		})
		return h, nil
	case keys.KeyResume:
		selected := h.list.GetSelectedInstance()
		if selected == nil {
			return h, nil
		}
		if err := selected.Resume(); err != nil {
			return h, h.handleError(err)
		}
		return h, tea.WindowSize()
	case keys.KeyEnter:
		if h.list.NumInstances() == 0 {
			return h, nil
		}
		selected := h.list.GetSelectedInstance()
		if selected == nil || selected.Paused() || !selected.TmuxAlive() {
			return h, nil
		}
		// Show help screen before attaching
		h.showHelpScreen(helpTypeInstanceAttach, func() {
			ch, err := h.list.Attach()
			if err != nil {
				h.handleError(err)
				return
			}
			<-ch
			h.state = stateDefault
		})
		return h, nil
	default:
		return h, nil
	}
}

func (im *instanceMode) instanceChanged(h *home) tea.Cmd {
	// selected may be nil
	selected := h.list.GetSelectedInstance()

	h.tabbedWindow.UpdateDiff(selected)
	// Update menu with current instance
	h.menu.SetInstance(selected)

	// If there's no selected instance, we don't need to update the preview.
	if err := h.tabbedWindow.UpdatePreview(selected); err != nil {
		return h.handleError(err)
	}
	return nil
}
