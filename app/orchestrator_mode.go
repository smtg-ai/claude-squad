package app

import (
	"claude-squad/keys"
	"claude-squad/orchestrator"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// orchestratorMode implements ModeStrategy for orchestrator mode.
type orchestratorMode struct {
	prompt           string
	plan             []orchestrator.Task
	result           string
	step             int // 0: prompt, 1: plan, 2: running, 3: done
	textInputOverlay *overlay.TextInputOverlay
}

func (om *orchestratorMode) HandleQuit(h *home) (tea.Model, tea.Cmd) {
	return h, tea.Quit
}

func (om *orchestratorMode) Render(h *home) string {
	mainView := lipgloss.NewStyle().Padding(2, 4).Bold(true).Render("[Orchestrator Mode]\n\n" +
		"Goal: " + om.prompt + "\n")

	switch om.step {
	case 0: // Prompt
		if om.textInputOverlay != nil {
			return overlay.PlaceOverlay(0, 0, om.textInputOverlay.Render(), mainView, true, true)
		}
		return mainView
	case 1: // Plan
		planText := "Orchestration Plan:\n\n"
		for i, task := range om.plan {
			planText += fmt.Sprintf("Task %d: %s\nPrompt: %s\n\n", i+1, task.Name, task.Prompt)
		}
		planText += "\nPress 'y' to proceed, 'n' to cancel."
		return overlay.PlaceOverlay(0, 0, planText, mainView, true, true)
	case 2: // Running
		return overlay.PlaceOverlay(0, 0, "Running orchestration...", mainView, true, true)
	case 3: // Done
		return overlay.PlaceOverlay(0, 0, "Orchestration complete!\n\n"+om.result, mainView, true, true)
	default:
		return mainView
	}
}

func (om *orchestratorMode) Update(h *home, msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// Handle switching back to instance mode
		name, ok := getKeyMapForMode(h.mode)[keyMsg.String()]
		if ok && name == keys.KeyMode {
			h.mode = modeInstance
			h.modeStrategy = &instanceMode{}
			return h, tea.Sequence(
				tea.WindowSize(),
				func() tea.Msg {
					h.menu.SetState(ui.StateDefault)
					return nil
				},
			)
		}

		// Orchestrator prompt/plan flow
		switch om.step {
		case 0: // Prompt
			if om.textInputOverlay == nil {
				om.textInputOverlay = overlay.NewTextInputOverlay("Enter orchestration goal", "")
				return h, nil
			}
			shouldClose := om.textInputOverlay.HandleKeyPress(keyMsg)
			if shouldClose {
				if om.textInputOverlay.IsSubmitted() {
					om.prompt = om.textInputOverlay.GetValue()
					// Create orchestrator and divide prompt into tasks
					orch := orchestrator.NewOrchestrator(om.prompt, h.autoYes)
					orch.SetProgram(h.program)
					om.plan = orch.DividePrompt()
					om.step = 1 // Show plan
					om.textInputOverlay = nil
				} else {
					om.textInputOverlay = nil
					om.prompt = ""
				}
				return h, tea.WindowSize()
			}
			return h, nil
		case 1: // Plan
			if keyMsg.String() == "y" || keyMsg.String() == "Y" {
				// Confirmed, run orchestrator
				om.step = 2
				return h, func() tea.Msg {
					// Run orchestrator in background (simulate long task)
					orch := orchestrator.NewOrchestrator(om.prompt, h.autoYes)
					orch.SetProgram(h.program)
					orch.Plan = om.plan
					result, err := orch.Run(".")
					if err != nil {
						om.result = "Error: " + err.Error()
					} else {
						om.result = "Final merged changes:\n" + result
					}
					om.step = 3
					return nil
				}
			}
			if keyMsg.String() == "n" || keyMsg.String() == "N" {
				// Cancel
				om.step = 0
				om.prompt = ""
				om.plan = nil
				return h, tea.WindowSize()
			}
			return h, nil
		case 2, 3:
			// Ignore key input during running/done
			return h, nil
		}
	}
	return h, nil
}
