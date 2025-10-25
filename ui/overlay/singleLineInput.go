package overlay

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SingleLineInputOverlay represents a single-line text input overlay with state management.
type SingleLineInputOverlay struct {
	textinput     textinput.Model
	Title         string
	Submitted     bool
	Canceled      bool
	OnSubmit      func()
	width, height int
}

// NewSingleLineInputOverlay creates a new single-line input overlay with the given title and initial value.
func NewSingleLineInputOverlay(title string, placeholder string, initialValue string) *SingleLineInputOverlay {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(initialValue)
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 60

	return &SingleLineInputOverlay{
		textinput: ti,
		Title:     title,
		Submitted: false,
		Canceled:  false,
	}
}

func (s *SingleLineInputOverlay) SetSize(width, height int) {
	s.width = width
	s.height = height
	// Set input width to fit within the overlay
	s.textinput.Width = width - 10 // Account for padding and borders
}

// Init initializes the single-line input overlay model
func (s *SingleLineInputOverlay) Init() tea.Cmd {
	return textinput.Blink
}

// View renders the model's view
func (s *SingleLineInputOverlay) View() string {
	return s.Render()
}

// HandleKeyPress processes a key press and updates the state accordingly.
// Returns true if the overlay should be closed.
func (s *SingleLineInputOverlay) HandleKeyPress(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyEsc:
		s.Canceled = true
		return true
	case tea.KeyEnter:
		// Submit on Enter
		s.Submitted = true
		if s.OnSubmit != nil {
			s.OnSubmit()
		}
		return true
	default:
		s.textinput, _ = s.textinput.Update(msg)
		return false
	}
}

// GetValue returns the current value of the text input.
func (s *SingleLineInputOverlay) GetValue() string {
	return s.textinput.Value()
}

// IsSubmitted returns whether the form was submitted.
func (s *SingleLineInputOverlay) IsSubmitted() bool {
	return s.Submitted
}

// IsCanceled returns whether the form was canceled.
func (s *SingleLineInputOverlay) IsCanceled() bool {
	return s.Canceled
}

// SetOnSubmit sets a callback function for form submission.
func (s *SingleLineInputOverlay) SetOnSubmit(onSubmit func()) {
	s.OnSubmit = onSubmit
}

// Render renders the single-line input overlay.
func (s *SingleLineInputOverlay) Render() string {
	// Create styles
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("62")).
		Bold(true).
		MarginBottom(1)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		MarginTop(1)

	// Build the view
	content := titleStyle.Render(s.Title) + "\n"
	content += s.textinput.View() + "\n"
	content += helpStyle.Render("(Enter to submit, Esc to cancel)")

	return style.Render(content)
}
