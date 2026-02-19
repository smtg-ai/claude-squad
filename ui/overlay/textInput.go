package overlay

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TextInputOverlay represents a text input overlay with state management.
type TextInputOverlay struct {
	textarea      textarea.Model
	Title         string
	FocusIndex    int // 0 for text input, 1 for enter button
	Submitted     bool
	Canceled      bool
	OnSubmit      func()
	width, height int
}

// NewTextInputOverlay creates a new text input overlay with the given title and initial value.
func NewTextInputOverlay(title string, initialValue string) *TextInputOverlay {
	ti := textarea.New()
	ti.SetValue(initialValue)
	ti.Focus()
	ti.ShowLineNumbers = false
	ti.Prompt = ""
	ti.FocusedStyle.CursorLine = lipgloss.NewStyle()

	// Ensure no character limit
	ti.CharLimit = 0
	// Ensure no maximum height limit
	ti.MaxHeight = 0

	return &TextInputOverlay{
		textarea:   ti,
		Title:      title,
		FocusIndex: 0,
		Submitted:  false,
		Canceled:   false,
	}
}

func (t *TextInputOverlay) SetSize(width, height int) {
	t.textarea.SetHeight(height)
	t.width = width
	t.height = height
}

// HandleKeyPress processes a key press and updates the state accordingly.
// Returns true if the overlay should be closed.
func (t *TextInputOverlay) HandleKeyPress(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyTab, tea.KeyShiftTab:
		t.FocusIndex = (t.FocusIndex + 1) % 2
		if t.FocusIndex == 0 {
			t.textarea.Focus()
		} else {
			t.textarea.Blur()
		}
		return false
	case tea.KeyEsc:
		t.Canceled = true
		return true
	case tea.KeyEnter:
		// Enter always submits
		t.Submitted = true
		if t.OnSubmit != nil {
			t.OnSubmit()
		}
		return true
	default:
		if t.FocusIndex == 0 {
			t.textarea, _ = t.textarea.Update(msg)
		}
		return false
	}
}

// GetValue returns the current value of the text input.
func (t *TextInputOverlay) GetValue() string {
	return t.textarea.Value()
}

// IsSubmitted returns whether the form was submitted.
func (t *TextInputOverlay) IsSubmitted() bool {
	return t.Submitted
}

// Render renders the text input overlay.
func (t *TextInputOverlay) Render() string {
	// Create styles
	style := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#F0A868")).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F0A868")).
		Bold(true).
		MarginBottom(1)

	buttonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))

	focusedButtonStyle := buttonStyle
	focusedButtonStyle = focusedButtonStyle.
		Background(lipgloss.Color("216")).
		Foreground(lipgloss.Color("0"))

	// Set textarea width to fit within the overlay
	w := t.width
	if w < 40 {
		w = 40
	}
	t.textarea.SetWidth(w - 6) // Account for padding and borders

	// Build the view
	content := titleStyle.Render(t.Title) + "\n"
	content += t.textarea.View() + "\n\n"

	// Render enter button with appropriate style
	enterButton := " Enter "
	if t.FocusIndex == 1 {
		enterButton = focusedButtonStyle.Render(enterButton)
	} else {
		enterButton = buttonStyle.Render(enterButton)
	}
	content += enterButton

	return style.Render(content)
}
