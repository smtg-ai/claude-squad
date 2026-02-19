package overlay

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var pickerBorderStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7D56F4")).
	Padding(1, 2)

var pickerTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#7D56F4")).
	MarginBottom(1)

var pickerSearchStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#555555")).
	Padding(0, 1).
	MarginBottom(1)

var pickerSearchActiveStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7D56F4")).
	Padding(0, 1).
	MarginBottom(1)

var pickerItemStyle = lipgloss.NewStyle().
	Padding(0, 1).
	Foreground(lipgloss.Color("#dddddd"))

var pickerSelectedItemStyle = lipgloss.NewStyle().
	Padding(0, 1).
	Background(lipgloss.Color("#7D56F4")).
	Foreground(lipgloss.Color("#ffffff"))

var pickerHintStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#666666")).
	MarginTop(1)

// PickerOverlay shows a searchable list of options for selection.
type PickerOverlay struct {
	title       string
	allItems    []string
	filtered    []string
	selectedIdx int
	searchQuery string
	width       int
	submitted   bool
	cancelled   bool
}

// NewPickerOverlay creates a picker with a title and list of items.
func NewPickerOverlay(title string, items []string) *PickerOverlay {
	filtered := make([]string, len(items))
	copy(filtered, items)
	return &PickerOverlay{
		title:    title,
		allItems: items,
		filtered: filtered,
		width:    40,
	}
}

func (p *PickerOverlay) SetWidth(w int) {
	p.width = w
}

// HandleKeyPress processes input. Returns true when the overlay should close.
func (p *PickerOverlay) HandleKeyPress(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc":
		p.cancelled = true
		return true
	case "enter":
		p.submitted = true
		return true
	case "up", "shift+tab":
		if p.selectedIdx > 0 {
			p.selectedIdx--
		}
	case "down", "tab":
		if p.selectedIdx < len(p.filtered)-1 {
			p.selectedIdx++
		}
	case "backspace":
		if len(p.searchQuery) > 0 {
			runes := []rune(p.searchQuery)
			p.searchQuery = string(runes[:len(runes)-1])
			p.applyFilter()
		}
	default:
		if msg.Type == tea.KeyRunes {
			p.searchQuery += string(msg.Runes)
			p.applyFilter()
		}
	}
	return false
}

func (p *PickerOverlay) applyFilter() {
	if p.searchQuery == "" {
		p.filtered = make([]string, len(p.allItems))
		copy(p.filtered, p.allItems)
	} else {
		query := strings.ToLower(p.searchQuery)
		p.filtered = nil
		for _, item := range p.allItems {
			if strings.Contains(strings.ToLower(item), query) {
				p.filtered = append(p.filtered, item)
			}
		}
	}
	if p.selectedIdx >= len(p.filtered) {
		p.selectedIdx = len(p.filtered) - 1
	}
	if p.selectedIdx < 0 {
		p.selectedIdx = 0
	}
}

// Value returns the selected item, or empty string if cancelled or nothing selected.
func (p *PickerOverlay) Value() string {
	if p.cancelled || len(p.filtered) == 0 {
		return ""
	}
	return p.filtered[p.selectedIdx]
}

// IsSubmitted returns true if the user pressed Enter.
func (p *PickerOverlay) IsSubmitted() bool {
	return p.submitted
}

// Render draws the picker overlay.
func (p *PickerOverlay) Render() string {
	var b strings.Builder

	// Title
	b.WriteString(pickerTitleStyle.Render(p.title))
	b.WriteString("\n")

	// Search bar
	innerWidth := p.width - 8 // borders + padding
	if innerWidth < 10 {
		innerWidth = 10
	}
	searchText := p.searchQuery
	if searchText == "" {
		searchText = "Type to filter..."
	}
	b.WriteString(pickerSearchActiveStyle.Width(innerWidth).Render(searchText))
	b.WriteString("\n")

	// Items
	if len(p.filtered) == 0 {
		b.WriteString(pickerHintStyle.Render("  No matching topics"))
		b.WriteString("\n")
	} else {
		for i, item := range p.filtered {
			if i == p.selectedIdx {
				b.WriteString(pickerSelectedItemStyle.Width(innerWidth).Render("▸ " + item))
			} else {
				b.WriteString(pickerItemStyle.Width(innerWidth).Render("  " + item))
			}
			b.WriteString("\n")
		}
	}

	// Hint
	b.WriteString(pickerHintStyle.Render("↑↓ navigate • enter select • esc cancel"))

	return pickerBorderStyle.Width(p.width).Render(b.String())
}

func (p *PickerOverlay) SetSize(width, height int) {
	p.width = width
}
