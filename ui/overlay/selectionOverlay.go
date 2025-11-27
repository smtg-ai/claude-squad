package overlay

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SelectionOverlay represents a selection dialog overlay
type SelectionOverlay struct {
	// Title of the selection dialog
	title string
	// Options to choose from
	options []string
	// Currently selected index
	selectedIndex int
	// Width of the overlay
	width int
	// Callback function to be called when the user confirms selection
	OnSelect func(index int, value string)
	// Callback function to be called when the user cancels
	OnCancel func()
	// Custom styling options
	borderColor lipgloss.Color
}

// NewSelectionOverlay creates a new selection dialog overlay
func NewSelectionOverlay(title string, options []string) *SelectionOverlay {
	return &SelectionOverlay{
		title:         title,
		options:       options,
		selectedIndex: 0,
		width:         50,
		borderColor:   lipgloss.Color("#7dc4e4"), // Blue color for selection
	}
}

// HandleKeyPress processes a key press and updates the state
// Returns true if the overlay should be closed
func (s *SelectionOverlay) HandleKeyPress(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "up", "k":
		if s.selectedIndex > 0 {
			s.selectedIndex--
		}
		return false
	case "down", "j":
		if s.selectedIndex < len(s.options)-1 {
			s.selectedIndex++
		}
		return false
	case "enter":
		if s.OnSelect != nil && len(s.options) > 0 {
			s.OnSelect(s.selectedIndex, s.options[s.selectedIndex])
		}
		return true
	case "esc":
		s.selectedIndex = -1 // Mark as cancelled
		if s.OnCancel != nil {
			s.OnCancel()
		}
		return true
	default:
		// Check for number keys 1-9
		if len(msg.String()) == 1 && msg.String()[0] >= '1' && msg.String()[0] <= '9' {
			index := int(msg.String()[0] - '1')
			if index < len(s.options) {
				s.selectedIndex = index
				if s.OnSelect != nil {
					s.OnSelect(s.selectedIndex, s.options[s.selectedIndex])
				}
				return true
			}
		}
		return false
	}
}

// Render renders the selection overlay
func (s *SelectionOverlay) Render(opts ...WhitespaceOption) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.borderColor).
		Padding(1, 2).
		Width(s.width)

	// Build the content
	content := lipgloss.NewStyle().Bold(true).Render(s.title) + "\n\n"

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a6da95")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#cad3f5"))

	for i, option := range s.options {
		prefix := "  "
		optionStyle := normalStyle
		if i == s.selectedIndex {
			prefix = "> "
			optionStyle = selectedStyle
		}
		// Show number shortcut
		number := lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("[%d] ", i+1))
		content += prefix + number + optionStyle.Render(option) + "\n"
	}

	content += "\n" + lipgloss.NewStyle().Faint(true).Render("↑/↓ to move, Enter to select, Esc to cancel")

	return style.Render(content)
}

// SetWidth sets the width of the selection overlay
func (s *SelectionOverlay) SetWidth(width int) {
	s.width = width
}

// GetSelectedIndex returns the currently selected index
func (s *SelectionOverlay) GetSelectedIndex() int {
	return s.selectedIndex
}

// GetSelectedValue returns the currently selected value
func (s *SelectionOverlay) GetSelectedValue() string {
	if s.selectedIndex >= 0 && s.selectedIndex < len(s.options) {
		return s.options[s.selectedIndex]
	}
	return ""
}
