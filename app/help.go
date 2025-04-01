package app

import (
	"claude-squad/session"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type helpType int

// Make a help state type enum
const (
	helpTypeGeneral helpType = iota
	helpTypeInstanceStart
	helpTypeInstanceAttach
	helpTypeInstanceCheckout
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(lipgloss.Color("#7D56F4"))
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#36CFC9"))
	keyStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFCC00"))
	descStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
)

func (h helpType) ToContent(instance *session.Instance) string {
	switch h {
	case helpTypeGeneral:
		content := lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Claude Squad"),
			"",
			"A terminal UI that manages multiple Claude Code (and other local agents) in separate workspaces.",
			"",
			headerStyle.Render("Managing:"),
			keyStyle.Render("n")+descStyle.Render("     - Create a new session"),
			keyStyle.Render("N")+descStyle.Render("     - Create a new session with a prompt"),
			keyStyle.Render("D")+descStyle.Render("     - Kill (delete) the selected session"),
			keyStyle.Render("↑/j, ↓/k")+descStyle.Render(" - Navigate between sessions"),
			keyStyle.Render("↵/o")+descStyle.Render("   - Attach to the selected session"),
			keyStyle.Render("ctrl-q")+descStyle.Render(" - Detach from session"),
			"",
			headerStyle.Render("Other:"),
			keyStyle.Render("tab")+descStyle.Render("   - Switch between preview and diff tabs"),
			keyStyle.Render("shift-↓/↑")+descStyle.Render(" - Scroll in diff view"),
			keyStyle.Render("q")+descStyle.Render("     - Quit the application"),
			"",
			headerStyle.Render("Handoff:"),
			keyStyle.Render("p")+descStyle.Render("     - Commit and push branch to github"),
			keyStyle.Render("c")+descStyle.Render("     - Checkout: commit changes and pause session"),
			keyStyle.Render("r")+descStyle.Render("     - Resume a paused session"),
		)
		return content

	case helpTypeInstanceStart:
		content := lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Instance Created"),
			"",
			descStyle.Render("New session created:"),
			descStyle.Render(fmt.Sprintf("• Git branch: %s (isolated worktree)", lipgloss.NewStyle().Bold(true).Render(instance.Branch))),
			descStyle.Render(fmt.Sprintf("• %s running in background tmux session", lipgloss.NewStyle().Bold(true).Render(instance.Program))),
			"",
			headerStyle.Render("Managing:"),
			keyStyle.Render("↵/o")+descStyle.Render("   - Attach to the session to interact with it directly"),
			keyStyle.Render("tab")+descStyle.Render("   - Switch preview panes to view session diff"),
			keyStyle.Render("D")+descStyle.Render("     - Kill (delete) the selected session"),
			"",
			headerStyle.Render("Handoff:"),
			keyStyle.Render("c")+descStyle.Render("     - Checkout this instance's branch"),
			keyStyle.Render("p")+descStyle.Render("     - Push branch to GitHub to create a PR"),
		)
		return content

	case helpTypeInstanceAttach:
		content := lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Attaching to Instance"),
			"",
			descStyle.Render("To detach from a session, press ")+keyStyle.Render("ctrl-q"),
		)
		return content

	case helpTypeInstanceCheckout:
		content := lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Checkout Instance"),
			"",
			"Changes will be committed and pushed to GitHub. The branch name has been copied to your clipboard for you to checkout.",
			"",
			"Feel free to make changes to the branch and commit them. When resuming, the session will continue from where you left off.",
			"",
			headerStyle.Render("Commands:"),
			keyStyle.Render("c")+descStyle.Render(" - Checkout: commit changes and pause session"),
			keyStyle.Render("r")+descStyle.Render(" - Resume a paused session"),
		)
		return content
	}
	return ""
}

// showHelpScreen displays the help screen overlay
func (m *home) showHelpScreen(helpType helpType, onDismiss func()) (tea.Model, tea.Cmd) {
	content := helpType.ToContent(m.list.GetSelectedInstance())

	m.textOverlay = overlay.NewTextOverlay(content)
	m.textOverlay.OnDismiss = onDismiss
	m.state = stateHelp
	return m, nil
}

// handleHelpState handles key events when in help state
func (m *home) handleHelpState(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Any key press will close the help overlay
	shouldClose := m.textOverlay.HandleKeyPress(msg)
	if shouldClose {
		m.state = stateDefault
		return m, tea.Sequence(
			tea.WindowSize(),
			func() tea.Msg {
				m.menu.SetState(ui.StateDefault)
				return nil
			},
		)
	}

	return m, nil
}
