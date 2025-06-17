package ui

import (
	"claude-squad/session"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var (
	consolePaneStyle = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})
)

type ConsolePane struct {
	viewport viewport.Model
	width    int
	height   int

	previewState      consolePreviewState
	instancePositions map[*session.Instance]viewport.Model // Track viewport state per instance
	instanceContent   map[*session.Instance]string         // Track content hash per instance to detect changes
	activeInstance    *session.Instance                    // Track which instance is currently active
}

type consolePreviewState struct {
	// fallback is true if the console pane is displaying fallback text
	fallback bool
	// text is the text displayed in the console pane
	text string
}

func NewConsolePane() *ConsolePane {
	return &ConsolePane{
		viewport:          viewport.New(0, 0),
		instancePositions: make(map[*session.Instance]viewport.Model),
		instanceContent:   make(map[*session.Instance]string),
	}
}

func (c *ConsolePane) SetSize(width, height int) {
	c.width = width
	c.height = height
	c.viewport.Width = width
	c.viewport.Height = height
}

// setFallbackState sets the console state with fallback text and a message
func (c *ConsolePane) setFallbackState(message string) {
	content := lipgloss.Place(
		c.width,
		c.height,
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, FallBackText, "", message),
	)
	c.previewState = consolePreviewState{
		fallback: true,
		text:     content,
	}
	c.viewport.SetContent(content)
}

// setFallbackStateWithPrompt sets fallback state with enhanced prompt context
func (c *ConsolePane) setFallbackStateWithPrompt(message string, instance *session.Instance) {
	var content string

	if instance != nil && instance.Started() {
		// Show prompt preview
		prompt := buildEnhancedPrompt(instance)
		content = lipgloss.Place(
			c.width,
			c.height,
			lipgloss.Center,
			lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Center,
				FallBackText,
				"",
				message,
				"",
				"Preview prompt:",
				prompt,
			),
		)
	} else {
		content = lipgloss.Place(
			c.width,
			c.height,
			lipgloss.Center,
			lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Center, FallBackText, "", message),
		)
	}

	c.previewState = consolePreviewState{
		fallback: true,
		text:     content,
	}
	c.viewport.SetContent(content)
}

// UpdateContent updates the console pane content with the instance's console preview
func (c *ConsolePane) UpdateContent(instance *session.Instance) error {
	switch {
	case instance == nil:
		c.setFallbackState("No agents running yet. Spin up a new instance with 'n' to get started!")
		return nil
	case instance.Status == session.Paused:
		c.setFallbackStateWithPrompt("Session is paused. Press 'r' to resume.", instance)
		return nil
	case !instance.Started():
		c.setFallbackState("Please enter a name for the instance.")
		return nil
	}

	content, err := instance.ConsolePreview()
	if err != nil {
		return err
	}

	if len(content) == 0 {
		c.setFallbackStateWithPrompt("Console session ready. Press Enter to attach.", instance)
		return nil
	}

	// Enhance the content with better prompts
	enhancedContent := enhanceContent(content, instance)

	// Check if this is new content or instance switch
	oldContent, contentExists := c.instanceContent[instance]
	isNewContent := !contentExists || oldContent != enhancedContent
	isInstanceSwitch := c.activeInstance != instance

	// Get or create viewport for this instance
	instanceViewport, exists := c.instancePositions[instance]
	if !exists {
		// Create new viewport for this instance
		instanceViewport = viewport.New(c.width, c.height)
		instanceViewport.SetContent(enhancedContent)
		instanceViewport.GotoBottom() // Position at bottom for new instances
		c.instancePositions[instance] = instanceViewport
	} else {
		// Check if user is currently at the bottom before updating content
		wasAtBottom := instanceViewport.AtBottom()

		// Update content
		instanceViewport.SetContent(enhancedContent)

		// Auto-scroll behavior similar to PreviewPane
		if isInstanceSwitch {
			instanceViewport.GotoBottom()
		} else if isNewContent && wasAtBottom {
			instanceViewport.GotoBottom()
		}

		c.instancePositions[instance] = instanceViewport
	}

	// Update content tracking
	c.instanceContent[instance] = enhancedContent

	// Update the main viewport to be a reference to this instance's viewport
	c.viewport = c.instancePositions[instance]
	c.activeInstance = instance

	c.previewState = consolePreviewState{
		fallback: false,
		text:     enhancedContent,
	}

	return nil
}

// ScrollUp scrolls the viewport up
func (c *ConsolePane) ScrollUp() {
	c.viewport.LineUp(1)
	// Update the map with the modified viewport
	c.syncViewportToMap()
}

// ScrollDown scrolls the viewport down
func (c *ConsolePane) ScrollDown() {
	c.viewport.LineDown(1)
	// Update the map with the modified viewport
	c.syncViewportToMap()
}

// FastScrollUp scrolls the viewport up by 10 lines
func (c *ConsolePane) FastScrollUp() {
	c.viewport.LineUp(10)
	// Update the map with the modified viewport
	c.syncViewportToMap()
}

// FastScrollDown scrolls the viewport down by 10 lines
func (c *ConsolePane) FastScrollDown() {
	c.viewport.LineDown(10)
	// Update the map with the modified viewport
	c.syncViewportToMap()
}

// syncViewportToMap updates the instance map with current viewport state
func (c *ConsolePane) syncViewportToMap() {
	if c.activeInstance != nil {
		c.instancePositions[c.activeInstance] = c.viewport
	}
}

func (c *ConsolePane) String() string {
	return c.viewport.View()
}
