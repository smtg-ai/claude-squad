package overlay

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var contextMenuStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#F0A868")).
	Padding(0, 1)

var contextItemStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#dddddd")).
	Padding(0, 1)

var contextSelectedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#1a1a1a")).
	Background(lipgloss.Color("#7EC8D8")).
	Padding(0, 1)

var contextDisabledStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#555555")).
	Padding(0, 1)

var contextSearchStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#555555")).
	Padding(0, 1).
	MarginBottom(1)

var contextSearchPlaceholderStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#666666"))

var contextHintStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#666666")).
	MarginTop(1)

var contextNumberStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#F0A868"))

// ContextMenuItem represents a single menu option.
type ContextMenuItem struct {
	Label    string
	Action   string // identifier returned when selected
	Disabled bool
}

// ContextMenu displays a floating context menu with search and numbered shortcuts.
type ContextMenu struct {
	items       []ContextMenuItem
	filtered    []filteredItem
	selectedIdx int
	x, y        int // screen position
	width       int
	searchQuery string
}

// filteredItem tracks the original index for number shortcuts.
type filteredItem struct {
	item    ContextMenuItem
	origIdx int // 1-based number for display and hotkey
}

// NewContextMenu creates a context menu at the given screen position.
func NewContextMenu(x, y int, items []ContextMenuItem) *ContextMenu {
	c := &ContextMenu{
		items: items,
		x:     x,
		y:     y,
	}
	c.applyFilter()
	c.calculateWidth()
	return c
}

func (c *ContextMenu) calculateWidth() {
	maxWidth := 0
	for i, item := range c.items {
		label := fmt.Sprintf("%d %s", i+1, item.Label)
		if len(label) > maxWidth {
			maxWidth = len(label)
		}
	}
	placeholder := "\uf002 Type to filter..."
	if len(placeholder) > maxWidth {
		maxWidth = len(placeholder)
	}
	c.width = maxWidth + 4 // padding
}

func (c *ContextMenu) applyFilter() {
	c.filtered = nil
	query := strings.ToLower(c.searchQuery)
	for i, item := range c.items {
		if query == "" || strings.Contains(strings.ToLower(item.Label), query) {
			c.filtered = append(c.filtered, filteredItem{
				item:    item,
				origIdx: i + 1,
			})
		}
	}
	if c.selectedIdx >= len(c.filtered) {
		c.selectedIdx = len(c.filtered) - 1
	}
	if c.selectedIdx < 0 {
		c.selectedIdx = 0
	}
	c.skipToNonDisabled(1)
}

func (c *ContextMenu) skipToNonDisabled(direction int) {
	if len(c.filtered) == 0 {
		return
	}
	start := c.selectedIdx
	for c.filtered[c.selectedIdx].item.Disabled {
		c.selectedIdx += direction
		if c.selectedIdx >= len(c.filtered) {
			c.selectedIdx = 0
		}
		if c.selectedIdx < 0 {
			c.selectedIdx = len(c.filtered) - 1
		}
		if c.selectedIdx == start {
			break
		}
	}
}

// HandleKeyPress processes key events. Returns the selected action string, or "" if no selection.
// Returns ("", false) if menu stays open, (action, true) if an item was selected, ("", true) if dismissed.
func (c *ContextMenu) HandleKeyPress(msg tea.KeyMsg) (string, bool) {
	switch msg.String() {
	case "esc":
		return "", true
	case " ":
		if c.searchQuery == "" {
			return "", true
		}
		c.searchQuery += " "
		c.applyFilter()
	case "enter":
		if c.selectedIdx < len(c.filtered) && !c.filtered[c.selectedIdx].item.Disabled {
			return c.filtered[c.selectedIdx].item.Action, true
		}
		return "", false
	case "up", "k":
		for i := c.selectedIdx - 1; i >= 0; i-- {
			if !c.filtered[i].item.Disabled {
				c.selectedIdx = i
				break
			}
		}
	case "down", "j":
		for i := c.selectedIdx + 1; i < len(c.filtered); i++ {
			if !c.filtered[i].item.Disabled {
				c.selectedIdx = i
				break
			}
		}
	case "backspace":
		if len(c.searchQuery) > 0 {
			runes := []rune(c.searchQuery)
			c.searchQuery = string(runes[:len(runes)-1])
			c.applyFilter()
		}
	default:
		if msg.Type == tea.KeyRunes {
			r := msg.Runes[0]
			// Number shortcut (1-9) when search is empty
			if r >= '1' && r <= '9' && c.searchQuery == "" {
				num := int(r - '0')
				for i, fi := range c.filtered {
					if fi.origIdx == num && !fi.item.Disabled {
						c.selectedIdx = i
						return fi.item.Action, true
					}
				}
				return "", false
			}
			c.searchQuery += string(msg.Runes)
			c.applyFilter()
		}
	}
	return "", false
}

// Render returns the styled menu string.
func (c *ContextMenu) Render() string {
	var b strings.Builder

	// Search bar
	innerWidth := c.width
	if innerWidth < 10 {
		innerWidth = 10
	}
	searchText := c.searchQuery
	if searchText == "" {
		searchText = contextSearchPlaceholderStyle.Render("\uf002 Type to filter...")
	}
	b.WriteString(contextSearchStyle.Width(innerWidth).Render(searchText))
	b.WriteString("\n")

	// Items with numbers
	if len(c.filtered) == 0 {
		b.WriteString(contextDisabledStyle.Width(c.width).Render("No matches"))
	} else {
		for i, fi := range c.filtered {
			numPrefix := contextNumberStyle.Render(fmt.Sprintf("%d", fi.origIdx))
			label := fmt.Sprintf(" %s", fi.item.Label)

			var line string
			if fi.item.Disabled {
				line = contextDisabledStyle.Width(c.width).Render(
					fmt.Sprintf("%d %s", fi.origIdx, fi.item.Label))
			} else if i == c.selectedIdx {
				line = contextSelectedStyle.Width(c.width).Render(
					fmt.Sprintf("%d %s", fi.origIdx, fi.item.Label))
			} else {
				line = contextItemStyle.Render(numPrefix + label)
			}
			b.WriteString(line)
			if i < len(c.filtered)-1 {
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(contextHintStyle.Render("↑↓ nav • space close"))

	return contextMenuStyle.Render(b.String())
}

// GetPosition returns the screen coordinates for overlay placement.
func (c *ContextMenu) GetPosition() (int, int) {
	return c.x, c.y
}
