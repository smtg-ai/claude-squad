package overlay

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TextOverlay represents a text screen overlay
type TextOverlay struct {
	// Whether the overlay has been dismissed
	Dismissed bool
	// Callback function to be called when the overlay is dismissed
	OnDismiss func()
	// Content to display in the overlay
	content string
	// Whether this is a confirmation modal
	isConfirmation bool

	width int
}

// NewTextOverlay creates a new text screen overlay with the given title and content
func NewTextOverlay(content string) *TextOverlay {
	return &TextOverlay{
		Dismissed: false,
		content:   content,
	}
}

// HandleKeyPress processes a key press and updates the state
// Returns true if the overlay should be closed
func (t *TextOverlay) HandleKeyPress(msg tea.KeyMsg) bool {
	// Close on any key
	t.Dismissed = true
	// Call the OnDismiss callback if it exists
	if t.OnDismiss != nil {
		t.OnDismiss()
	}
	return true
}

// Render renders the text overlay
func (t *TextOverlay) Render(opts ...WhitespaceOption) string {
	// Create styles
	borderColor := lipgloss.Color("62") // Default color
	if t.isConfirmation {
		borderColor = lipgloss.Color("#de613e") // Red color for confirmations
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(t.width)

	// Apply the border style and return
	return style.Render(t.content)
}

func (t *TextOverlay) SetWidth(width int) {
	t.width = width
}

// SetIsConfirmation sets whether this is a confirmation modal
func (t *TextOverlay) SetIsConfirmation(isConfirmation bool) {
	t.isConfirmation = isConfirmation
}
