package model

import (
	"claude-squad/instance/orchestrator"
	"claude-squad/ui"
	"claude-squad/ui/overlay"

	tea "github.com/charmbracelet/bubbletea"
)

// generateOrchestratorPlan generates a plan from the user's prompt and shows it for approval
func (c *Controller) generateOrchestratorPlan(model *Model, prompt string) (tea.Model, tea.Cmd) {
	return model, func() tea.Msg {
		orch := orchestrator.NewOrchestrator(model.program, prompt)
		c.instances = append(c.instances, orch)

		orch.ForumulatePlan()

		return tea.WindowSize()
	}
}

// handleOrchestratorPlanApproval handles when user approves the orchestrator plan
func (c *Controller) handleOrchestratorPlanApproval(model *Model) (tea.Model, tea.Cmd) {
	// For testing purposes, just show a success message
	return model, func() tea.Msg {
		// Show success message
		successMessage := "Plan Approved\n\nOrchestration plan has been approved. For testing purposes, no workers will be created."
		c.textOverlay = overlay.NewTextOverlay(successMessage)

		model.state = (tuiStateHelp) // Show the text overlay
		return tea.WindowSize()
	}
}

// handleOrchestratorPlanKeyPress handles key presses when showing orchestrator plan for approval
func (c *Controller) handleOrchestratorPlanKeyPress(model *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// User approved the plan
		// c.orchestratorPlan = ""
		return c.handleOrchestratorPlanApproval(model)
	case "esc", "q":
		// User cancelled the plan
		// c.orchestratorPlan = ""
		c.textOverlay = nil
		model.state = (tuiStateDefault)
		return model, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				model.menu.SetState(ui.StateDefault)
				return nil
			},
		)
	default:
		// Any other key shows help about the plan approval
		return model, nil
	}
}

// handleNewOrchestrator creates a new orchestrator
func (c *Controller) handleNewOrchestrator(model *Model) (tea.Model, tea.Cmd) {
	// Create an orchestrator instance - similar to KeyPrompt but for orchestration
	model.state = (tuiStatePrompt)
	model.menu.SetState(ui.StatePrompt)
	// Initialize the text input overlay for orchestrator goal
	c.textInputOverlay = overlay.NewTextInputOverlay("Enter orchestration goal", "")
	// Set proper size for the overlay (should match other overlays)
	c.textInputOverlay.SetSize(80, 20)
	// Set up callbacks
	c.textInputOverlay.SetOnSubmit(func() {
		prompt := c.textInputOverlay.GetValue()
		model.state = (tuiStateDefault)
		c.textInputOverlay = nil
		// Generate orchestrator plan with the prompt
		c.generateOrchestratorPlan(model, prompt)
	})
	c.orchestratorState = orchestratorStatePrompt
	return model, nil
}
