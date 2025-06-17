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

	previewState      previewState
	instancePositions map[string]viewport.Model // Track viewport state per instance
	instanceContent   map[string]string         // Track content hash per instance to detect changes
	activeInstance    string                    // Track which instance is currently active
}

type previewState struct {
	// fallback is true if the preview pane is displaying fallback text
	fallback bool
	// text is the text displayed in the preview pane
	text string
}

func NewPreviewPane() *PreviewPane {
	return &PreviewPane{
		viewport:          viewport.New(0, 0),
		instancePositions: make(map[string]viewport.Model),
		instanceContent:   make(map[string]string),
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

	// Check if this is new content or instance switch
	oldContent, contentExists := p.instanceContent[instance.Title]
	isNewContent := !contentExists || oldContent != content
	isInstanceSwitch := p.activeInstance != instance.Title

	// Get or create viewport for this instance
	instanceViewport, exists := p.instancePositions[instance.Title]
	if !exists {
		// Create new viewport for this instance
		instanceViewport = viewport.New(p.width, p.height)
		instanceViewport.SetContent(content)
		instanceViewport.GotoBottom() // Position at bottom for new instances
		p.instancePositions[instance.Title] = instanceViewport
	} else {
		// Check if user is currently at the bottom before updating content
		wasAtBottom := instanceViewport.AtBottom()
		
		// Update content
		instanceViewport.SetContent(content)
		
		// Auto-scroll behavior:
		// 1. Always go to bottom when switching to a different instance (show latest activity)
		// 2. Only auto-scroll for new content if user was already at the bottom (terminal behavior)
		if isInstanceSwitch {
			instanceViewport.GotoBottom()
		} else if isNewContent && wasAtBottom {
			// Only auto-scroll if user was already viewing the latest content
			instanceViewport.GotoBottom()
		}
		// If user was scrolled up and there's new content, preserve their position
		
		p.instancePositions[instance.Title] = instanceViewport
	}

	// Update content tracking
	p.instanceContent[instance.Title] = content

	// Update the main viewport to be a reference to this instance's viewport
	p.viewport = p.instancePositions[instance.Title]
	p.activeInstance = instance.Title
	
	p.previewState = previewState{
		fallback: false,
		text:     content,
	}
	
	return nil
}

func (p *PreviewPane) String() string {
	return p.viewport.View()
}

// ScrollUp scrolls the viewport up
func (p *PreviewPane) ScrollUp() {
	p.viewport.LineUp(1)
	// Update the map with the modified viewport
	p.syncViewportToMap()
}

// ScrollDown scrolls the viewport down
func (p *PreviewPane) ScrollDown() {
	p.viewport.LineDown(1)
	// Update the map with the modified viewport
	p.syncViewportToMap()
}

// syncViewportToMap updates the instance map with current viewport state
func (p *PreviewPane) syncViewportToMap() {
	if p.activeInstance != "" {
		p.instancePositions[p.activeInstance] = p.viewport
	}
}
