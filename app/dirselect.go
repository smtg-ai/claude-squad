package app

import (
	"claude-squad/session"
	"claude-squad/session/git"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// validateRepoDir expands and validates a user-entered directory, returning
// its absolute path. The directory must exist and be inside a git repository.
func validateRepoDir(input string) (string, error) {
	abs, err := filepath.Abs(overlay.ExpandUserPath(input))
	if err != nil {
		return "", fmt.Errorf("invalid path %q: %w", input, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("directory does not exist: %s", abs)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}
	if _, err := git.FindGitRepoRoot(abs); err != nil {
		return "", fmt.Errorf("not a git repository: %s", abs)
	}
	return abs, nil
}

// newDirOverlay opens the directory selection overlay for a new session.
// promptAfterName carries whether the prompt overlay should follow naming (N vs n).
func (m *home) newDirOverlay(promptAfterName bool) {
	m.state = stateNewDir
	m.menu.SetState(ui.StateNewInstance)
	m.promptAfterName = promptAfterName
	m.textInputOverlay = overlay.NewPathInputOverlay("Repository directory (empty = current repo)", "")
}

// cancelDirOverlay closes the directory overlay without creating an instance.
func (m *home) cancelDirOverlay() tea.Cmd {
	m.textInputOverlay = nil
	m.promptAfterName = false
	m.state = stateDefault
	return tea.Sequence(
		tea.WindowSize(),
		func() tea.Msg {
			m.menu.SetState(ui.StateDefault)
			return nil
		},
	)
}

// handleDirSubmit validates the submitted directory and, if valid, creates the
// new (unstarted) instance there and moves on to the naming state. An invalid
// directory keeps the overlay open with the error displayed.
func (m *home) handleDirSubmit(value string) (tea.Model, tea.Cmd) {
	path := "."
	if value != "" {
		validated, err := validateRepoDir(value)
		if err != nil {
			m.textInputOverlay.ResetSubmitted()
			m.textInputOverlay.SetError(err.Error())
			return m, nil
		}
		path = validated
	}

	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "",
		Path:    path,
		Program: m.program,
	})
	if err != nil {
		return m, tea.Batch(m.cancelDirOverlay(), m.handleError(err))
	}

	m.textInputOverlay = nil
	m.newInstanceFinalizer = m.list.AddInstance(instance)
	m.list.SetSelectedInstance(m.list.NumInstances() - 1)
	m.state = stateNew

	if m.promptAfterName {
		// Start a background fetch so branches are up to date by the time the picker opens
		repoDir := instance.Path
		return m, func() tea.Msg {
			git.FetchBranches(repoDir)
			return nil
		}
	}
	return m, nil
}
