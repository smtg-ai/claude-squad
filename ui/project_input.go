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
				Width(60)
	
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
	ti.Width = 50
	
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

// SetSize sets the dimensions for proper centering
func (p *ProjectInputOverlay) SetSize(width, height int) {
	p.width = width
	p.height = height
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
	
	// Wrap in styled container
	overlay := projectInputStyle.Render(content.String())
	
	// Center the overlay
	if p.width > 0 && p.height > 0 {
		return CenterOverlay(overlay, p.width, p.height)
	}
	
	return overlay
}

// CenterOverlay centers content within the given dimensions
func CenterOverlay(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	contentHeight := len(lines)
	contentWidth := 0
	
	// Find the widest line
	for _, line := range lines {
		if lineWidth := lipgloss.Width(line); lineWidth > contentWidth {
			contentWidth = lineWidth
		}
	}
	
	// Calculate vertical offset
	verticalOffset := (height - contentHeight) / 2
	if verticalOffset < 0 {
		verticalOffset = 0
	}
	
	// Calculate horizontal offset
	horizontalOffset := (width - contentWidth) / 2
	if horizontalOffset < 0 {
		horizontalOffset = 0
	}
	
	// Add vertical padding
	var result strings.Builder
	for i := 0; i < verticalOffset; i++ {
		result.WriteString("\n")
	}
	
	// Add content with horizontal padding
	for _, line := range lines {
		for j := 0; j < horizontalOffset; j++ {
			result.WriteString(" ")
		}
		result.WriteString(line)
		result.WriteString("\n")
	}
	
	return result.String()
}