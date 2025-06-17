package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	projectInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")).
				Padding(1, 2).
				Width(70).   // Increased width for better visual presence
				MaxWidth(80) // Maximum width constraint for very wide terminals

	projectInputTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Bold(true).
				Margin(0, 0, 1, 0)

	projectInputHelpStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("243")).
				Margin(1, 0, 0, 0)

	projectInputErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Margin(1, 0, 0, 0)
)

// ProjectInputOverlay handles project path input
type ProjectInputOverlay struct {
	textInput textinput.Model
	error     string
	visible   bool
	width     int
	height    int
}

// NewProjectInputOverlay creates a new project input overlay
func NewProjectInputOverlay() *ProjectInputOverlay {
	ti := textinput.New()
	ti.Placeholder = "Enter absolute project path (e.g., /path/to/project)"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 60 // Match the dialog width better

	return &ProjectInputOverlay{
		textInput: ti,
		visible:   false,
	}
}

// Show displays the project input overlay
func (p *ProjectInputOverlay) Show() {
	p.visible = true
	p.textInput.Focus()
	p.textInput.SetValue("")
	p.error = ""
}

// Hide hides the project input overlay
func (p *ProjectInputOverlay) Hide() {
	p.visible = false
	p.textInput.Blur()
	p.error = ""
}

// IsVisible returns whether the overlay is currently visible
func (p *ProjectInputOverlay) IsVisible() bool {
	return p.visible
}

// GetValue returns the current input value
func (p *ProjectInputOverlay) GetValue() string {
	return p.textInput.Value()
}

// SetError sets an error message to display
func (p *ProjectInputOverlay) SetError(err string) {
	p.error = err
}

// ClearError clears any displayed error
func (p *ProjectInputOverlay) ClearError() {
	p.error = ""
}

// ValidatePath validates the entered path
func (p *ProjectInputOverlay) ValidatePath() error {
	path := strings.TrimSpace(p.textInput.Value())

	if path == "" {
		return fmt.Errorf("project path cannot be empty")
	}

	// Check if path is absolute
	if !filepath.IsAbs(path) {
		return fmt.Errorf("project path must be absolute (start with /)")
	}

	// Clean the path
	cleanPath := filepath.Clean(path)
	if cleanPath != path {
		// Update the input with the cleaned path
		p.textInput.SetValue(cleanPath)
	}

	return nil
}

// SetSize sets the dimensions for proper centering and responsive sizing
func (p *ProjectInputOverlay) SetSize(width, height int) {
	p.width = width
	p.height = height

	// Responsive sizing: adjust dialog width based on terminal size
	// For small terminals, use a smaller dialog; for large terminals, keep reasonable max
	dialogWidth := 70
	inputWidth := 60

	if width < 90 {
		// Small terminal: reduce dialog size
		dialogWidth = int(float32(width) * 0.8)
		if dialogWidth < 50 {
			dialogWidth = 50 // Minimum usable width
		}
		inputWidth = dialogWidth - 10 // Account for padding and borders
		if inputWidth < 40 {
			inputWidth = 40 // Minimum input width
		}
	} else if width > 120 {
		// Large terminal: cap at reasonable size for readability
		dialogWidth = 80
		inputWidth = 70
	}

	// Update styles with responsive dimensions
	projectInputStyle = projectInputStyle.Width(dialogWidth)
	p.textInput.Width = inputWidth
}

// Update handles input events
func (p *ProjectInputOverlay) Update(msg tea.Msg) (*ProjectInputOverlay, tea.Cmd) {
	if !p.visible {
		return p, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			// Validate path before accepting
			if err := p.ValidatePath(); err != nil {
				p.SetError(err.Error())
				return p, nil
			}
			// Path is valid, but don't hide here - let parent handle the submission
			return p, nil
		case tea.KeyEsc, tea.KeyCtrlC:
			p.Hide()
			return p, nil
		}
	}

	// Clear error when user starts typing
	if p.error != "" {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyRunes || msg.Type == tea.KeyBackspace || msg.Type == tea.KeyDelete {
				p.ClearError()
			}
		}
	}

	var cmd tea.Cmd
	p.textInput, cmd = p.textInput.Update(msg)
	return p, cmd
}

// View renders the project input overlay
func (p *ProjectInputOverlay) View() string {
	if !p.visible {
		return ""
	}

	var content strings.Builder

	// Title
	content.WriteString(projectInputTitleStyle.Render("Add Project"))
	content.WriteString("\n")

	// Input field
	content.WriteString(p.textInput.View())
	content.WriteString("\n")

	// Error message if present
	if p.error != "" {
		content.WriteString(projectInputErrorStyle.Render("Error: " + p.error))
		content.WriteString("\n")
	}

	// Help text
	helpText := "Enter absolute path • Press Enter to add • Press Esc to cancel"
	content.WriteString(projectInputHelpStyle.Render(helpText))

	// Wrap in styled container - this returns the content ready for overlay placement
	return projectInputStyle.Render(content.String())
}
