package ui

import (
	"claude-squad/session"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var terminalPaneStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})

type TerminalPane struct {
	width  int
	height int

	terminalState terminalState
}

type terminalState struct {
	// fallback is true if the terminal pane is displaying fallback text
	fallback bool
	// text is the text displayed in the terminal pane
	text string
}

func NewTerminalPane() *TerminalPane {
	return &TerminalPane{}
}

func (t *TerminalPane) SetSize(width, maxHeight int) {
	t.width = width
	t.height = maxHeight
}

// setFallbackState sets the terminal state with fallback text and a message
func (t *TerminalPane) setFallbackState(message string) {
	t.terminalState = terminalState{
		fallback: true,
		text:     lipgloss.JoinVertical(lipgloss.Center, FallBackText, "", message),
	}
}

// UpdateContent updates the terminal pane content with the actual terminal output
func (t *TerminalPane) UpdateContent(instance *session.Instance) error {
	switch {
	case instance == nil:
		t.setFallbackState("No instance selected.")
		return nil
	case instance.Status == session.Paused:
		t.setFallbackState("Session is paused. Press 'r' to resume.")
		return nil
	}

	// Get terminal content from the instance
	content, err := instance.GetTerminalContent()
	if err != nil {
		t.setFallbackState("Terminal not available yet...")
		return nil
	}

	// If content is empty, show a welcome message
	if strings.TrimSpace(content) == "" {
		content = "Terminal ready. This is a separate shell in the worktree directory.\n\n" +
			"Note: This is a read-only view. To interact with the terminal, press 'a' to attach to the session.\n"
	}

	t.terminalState = terminalState{
		fallback: false,
		text:     content,
	}
	return nil
}

// String returns the terminal pane content as a string.
func (t *TerminalPane) String() string {
	if t.width == 0 || t.height == 0 {
		return strings.Repeat("\n", t.height)
	}

	if t.terminalState.fallback {
		// Calculate available height for fallback text
		availableHeight := t.height - 3 - 4 // 2 for borders, 1 for margin, 1 for padding

		// Count the number of lines in the fallback text
		fallbackLines := len(strings.Split(t.terminalState.text, "\n"))

		// Calculate padding needed above and below to center the content
		totalPadding := availableHeight - fallbackLines
		topPadding := 0
		bottomPadding := 0
		if totalPadding > 0 {
			topPadding = totalPadding / 2
			bottomPadding = totalPadding - topPadding // accounts for odd numbers
		}

		// Build the centered content
		var lines []string
		if topPadding > 0 {
			lines = append(lines, strings.Repeat("\n", topPadding))
		}
		lines = append(lines, t.terminalState.text)
		if bottomPadding > 0 {
			lines = append(lines, strings.Repeat("\n", bottomPadding))
		}

		// Center both vertically and horizontally
		return terminalPaneStyle.
			Width(t.width).
			Align(lipgloss.Center).
			Render(strings.Join(lines, ""))
	}

	// Calculate available height accounting for border and margin
	availableHeight := t.height - 1 //  1 for ellipsis

	lines := strings.Split(t.terminalState.text, "\n")

	// Truncate if we have more lines than available height
	if availableHeight > 0 {
		if len(lines) > availableHeight {
			lines = lines[:availableHeight]
			lines = append(lines, "...")
		} else {
			// Pad with empty lines to fill available height
			padding := availableHeight - len(lines)
			lines = append(lines, make([]string, padding)...)
		}
	}

	content := strings.Join(lines, "\n")
	rendered := terminalPaneStyle.Width(t.width).Render(content)
	return rendered
}