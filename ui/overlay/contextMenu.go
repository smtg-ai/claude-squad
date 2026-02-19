package overlay

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var contextMenuStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7D56F4")).
	Background(lipgloss.Color("#1a1a2e")).
	Padding(0, 1)

var contextItemStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#dddddd")).
	Padding(0, 1)

var contextSelectedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#1a1a1a")).
	Background(lipgloss.Color("#7D56F4")).
	Padding(0, 1)

var contextDisabledStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#555555")).
	Padding(0, 1)

// ContextMenuItem represents a single menu option.
type ContextMenuItem struct {
	Label    string
	Action   string // identifier returned when selected
	Disabled bool
}

// ContextMenu displays a floating right-click menu.
type ContextMenu struct {
	items       []ContextMenuItem
	selectedIdx int
	x, y        int // screen position
	width       int
}

// NewContextMenu creates a context menu at the given screen position.
func NewContextMenu(x, y int, items []ContextMenuItem) *ContextMenu {
	// Calculate width from longest label
	maxWidth := 0
	for _, item := range items {
		if len(item.Label) > maxWidth {
			maxWidth = len(item.Label)
		}
	}

	// Find first non-disabled item
	selectedIdx := 0
	for i, item := range items {
		if !item.Disabled {
			selectedIdx = i
			break
		}
	}

	return &ContextMenu{
		items:       items,
		selectedIdx: selectedIdx,
		x:           x,
		y:           y,
		width:       maxWidth + 4, // padding
	}
}

// HandleKeyPress processes key events. Returns the selected action string, or "" if no selection.
// Returns ("", false) if menu stays open, (action, true) if an item was selected, ("", true) if dismissed.
func (c *ContextMenu) HandleKeyPress(msg tea.KeyMsg) (string, bool) {
	switch msg.String() {
	case "esc", "q":
		return "", true
	case "enter":
		if c.selectedIdx < len(c.items) && !c.items[c.selectedIdx].Disabled {
			return c.items[c.selectedIdx].Action, true
		}
		return "", false
	case "up", "k":
		for i := c.selectedIdx - 1; i >= 0; i-- {
			if !c.items[i].Disabled {
				c.selectedIdx = i
				break
			}
		}
	case "down", "j":
		for i := c.selectedIdx + 1; i < len(c.items); i++ {
			if !c.items[i].Disabled {
				c.selectedIdx = i
				break
			}
		}
	}
	return "", false
}

// Render returns the styled menu string.
func (c *ContextMenu) Render() string {
	var b strings.Builder
	for i, item := range c.items {
		var line string
		if item.Disabled {
			line = contextDisabledStyle.Width(c.width).Render(item.Label)
		} else if i == c.selectedIdx {
			line = contextSelectedStyle.Width(c.width).Render(item.Label)
		} else {
			line = contextItemStyle.Width(c.width).Render(item.Label)
		}
		b.WriteString(line)
		if i < len(c.items)-1 {
			b.WriteString("\n")
		}
	}
	return contextMenuStyle.Render(b.String())
}

// GetPosition returns the screen coordinates for overlay placement.
func (c *ContextMenu) GetPosition() (int, int) {
	return c.x, c.y
}
