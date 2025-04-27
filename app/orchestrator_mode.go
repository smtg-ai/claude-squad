package app

import (
	"claude-squad/keys"
	"claude-squad/ui"

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
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		name, ok := getKeyMapForMode(h.mode)[keyMsg.String()]
		if ok && name == keys.KeyMode {
			// Switch to instance mode
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
	}
	return h, nil
}
