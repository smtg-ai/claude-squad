package ui

import (
	"claude-squad/session"
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var previewPaneStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})

type PreviewPane struct {
	viewport viewport.Model
	width    int
	height   int

	previewState    previewState
	initialPosition bool // Track if we've positioned to bottom initially
}

type previewState struct {
	// fallback is true if the preview pane is displaying fallback text
	fallback bool
	// text is the text displayed in the preview pane
	text string
}

func NewPreviewPane() *PreviewPane {
	return &PreviewPane{
		viewport: viewport.New(0, 0),
	}
}

func (p *PreviewPane) SetSize(width, maxHeight int) {
	p.width = width
	p.height = maxHeight
	p.viewport.Width = width
	p.viewport.Height = maxHeight
}

// setFallbackState sets the preview state with fallback text and a message
func (p *PreviewPane) setFallbackState(message string) {
	content := lipgloss.Place(
		p.width,
		p.height,
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, FallBackText, "", message),
	)
	p.previewState = previewState{
		fallback: true,
		text:     content,
	}
	p.viewport.SetContent(content)
	// For fallback states, center the content (no need for GotoBottom)
}

// Updates the preview pane content with the tmux pane content
func (p *PreviewPane) UpdateContent(instance *session.Instance) error {
	switch {
	case instance == nil:
		p.setFallbackState("No agents running yet. Spin up a new instance with 'n' to get started!")
		return nil
	case instance.Status == session.Paused:
		p.setFallbackState(lipgloss.JoinVertical(lipgloss.Center,
			"Session is paused. Press 'r' to resume.",
			"",
			lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{
					Light: "#FFD700",
					Dark:  "#FFD700",
				}).
				Render(fmt.Sprintf(
					"The instance can be checked out at '%s' (copied to your clipboard)",
					instance.Branch,
				)),
		))
		return nil
	}

	content, err := instance.Preview()
	if err != nil {
		return err
	}

	if len(content) == 0 && !instance.Started() {
		p.setFallbackState("Please enter a name for the instance.")
		return nil
	}

	p.previewState = previewState{
		fallback: false,
		text:     content,
	}
	p.viewport.SetContent(content)
	
	// Only position at bottom on first load, not on every update
	if !p.initialPosition {
		p.viewport.GotoBottom()
		p.initialPosition = true
	}
	
	return nil
}

func (p *PreviewPane) String() string {
	return p.viewport.View()
}

// ScrollUp scrolls the viewport up
func (p *PreviewPane) ScrollUp() {
	p.viewport.LineUp(1)
}

// ScrollDown scrolls the viewport down
func (p *PreviewPane) ScrollDown() {
	p.viewport.LineDown(1)
}
