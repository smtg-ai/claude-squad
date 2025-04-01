package overlay

import "github.com/charmbracelet/lipgloss"

// TextInputOverlay represents a text input overlay with state management.
type HelpOverlay struct {
	width int
}

func (ho *HelpOverlay) SetSize(width int) {
	ho.width = width
}

// Render renders the text input overlay.
func (ho *HelpOverlay) Render() string {
	// Create styles
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("62")).
		Bold(true).
		MarginBottom(1)

	subTitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("62")).
		Underline(true).
		MarginBottom(1)

	// Build the view
	content := titleStyle.Render("Help") + "\n"
	content += subTitleStyle.Render("Context Switching")
	content += `
↵/o:     context switch into an instance 
ctrl-q:  context switch out of an instance`
	content += "\n\n"
	content += subTitleStyle.Render("Checkout/Resume")
	content += `
c:       pauses an instance so the branch can be checked out
r:       resumes an instance so it can continue working on its branch`
	content += "\n\n"

	content += lipgloss.Place(ho.width, 1, lipgloss.Center, lipgloss.Top, "(press 'ctrl+q' to close)")

	return style.Render(content)
}
