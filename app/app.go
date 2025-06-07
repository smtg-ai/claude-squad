package app

import (
	"claude-squad/app/model"
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

// Run is the main entrypoint into the application.
func Run(ctx context.Context, program string, autoYes bool) error {
	m := model.NewModel(ctx, program, autoYes)

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // Mouse scroll
	)
	_, err := p.Run()
	return err
}
