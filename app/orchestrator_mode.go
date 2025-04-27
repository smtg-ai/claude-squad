package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// orchestratorMode implements ModeStrategy for orchestrator mode.
type orchestratorMode struct{}

func (om *orchestratorMode) Render(h *home) string {
	return lipgloss.NewStyle().Padding(2, 4).Bold(true).Render(
		"[Orchestrator Mode]\n\n" +
			"This is the orchestrator dashboard.\n" +
			"(TODO: Show orchestrator plan, tasks, worker instances, etc.)",
	)
}

func (om *orchestratorMode) Update(h *home, msg tea.Msg) (tea.Model, tea.Cmd) {
	// TODO: Add orchestrator-mode-specific update logic here
	return h, nil
}
