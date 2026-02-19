package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var sidebarTitleStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("62")).
	Foreground(lipgloss.Color("230"))

// sidebarBorderStyle wraps the entire sidebar content in a subtle rounded border
var sidebarBorderStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.AdaptiveColor{Light: "#d0d0d0", Dark: "#3a3a3a"}).
	Padding(0, 1)

var topicItemStyle = lipgloss.NewStyle().
	Padding(0, 1).
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})

// selectedTopicStyle — focused: white bg (dark) / black bg (light)
var selectedTopicStyle = lipgloss.NewStyle().
	Padding(0, 1).
	Background(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#ffffff"}).
	Foreground(lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#1a1a1a"})

// activeTopicStyle — unfocused: 50% transparent version (muted)
var activeTopicStyle = lipgloss.NewStyle().
	Padding(0, 1).
	Background(lipgloss.AdaptiveColor{Light: "#b0b0b0", Dark: "#666666"}).
	Foreground(lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#1a1a1a"})

var sectionHeaderStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#666666"}).
	Padding(0, 1)

var searchBarStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("62")).
	Padding(0, 1)

var searchActiveBarStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("205")).
	Padding(0, 1)

const (
	SidebarAll       = "__all__"
	SidebarUngrouped = "__ungrouped__"
)

// SidebarItem represents a selectable item in the sidebar.
type SidebarItem struct {
	Name      string
	ID        string
	IsSection bool
	Count     int
}

// Sidebar is the left-most panel showing topics and search.
type Sidebar struct {
	items         []SidebarItem
	selectedIdx   int
	height, width int
	focused       bool

	searchActive bool
	searchQuery  string
}

func NewSidebar() *Sidebar {
	return &Sidebar{
		items: []SidebarItem{
			{Name: "All", ID: SidebarAll},
		},
		selectedIdx: 0,
		focused:     true,
	}
}

func (s *Sidebar) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s *Sidebar) SetFocused(focused bool) {
	s.focused = focused
}

func (s *Sidebar) IsFocused() bool {
	return s.focused
}

// SetItems updates the sidebar items from the current topics.
func (s *Sidebar) SetItems(topicNames []string, instanceCountByTopic map[string]int, ungroupedCount int) {
	totalCount := ungroupedCount
	for _, c := range instanceCountByTopic {
		totalCount += c
	}

	items := []SidebarItem{
		{Name: "All", ID: SidebarAll, Count: totalCount},
	}

	if len(topicNames) > 0 {
		items = append(items, SidebarItem{Name: "Topics", IsSection: true})
		for _, name := range topicNames {
			count := instanceCountByTopic[name]
			items = append(items, SidebarItem{Name: name, ID: name, Count: count})
		}
	}

	if ungroupedCount > 0 {
		items = append(items, SidebarItem{Name: "Ungrouped", IsSection: true})
		items = append(items, SidebarItem{Name: "Ungrouped", ID: SidebarUngrouped, Count: ungroupedCount})
	}

	s.items = items
	if s.selectedIdx >= len(items) {
		s.selectedIdx = len(items) - 1
	}
	if s.selectedIdx < 0 {
		s.selectedIdx = 0
	}
}

func (s *Sidebar) GetSelectedID() string {
	if len(s.items) == 0 {
		return SidebarAll
	}
	return s.items[s.selectedIdx].ID
}

func (s *Sidebar) Up() {
	for i := s.selectedIdx - 1; i >= 0; i-- {
		if !s.items[i].IsSection {
			s.selectedIdx = i
			return
		}
	}
}

func (s *Sidebar) Down() {
	for i := s.selectedIdx + 1; i < len(s.items); i++ {
		if !s.items[i].IsSection {
			s.selectedIdx = i
			return
		}
	}
}

// ClickItem selects a sidebar item by its rendered row offset (0-indexed from the first item).
// Section headers count as a row but are skipped for selection.
func (s *Sidebar) ClickItem(row int) {
	currentRow := 0
	for i, item := range s.items {
		if currentRow == row {
			if !item.IsSection {
				s.selectedIdx = i
			}
			return
		}
		currentRow++
	}
}

func (s *Sidebar) ActivateSearch()        { s.searchActive = true; s.searchQuery = "" }
func (s *Sidebar) DeactivateSearch()      { s.searchActive = false; s.searchQuery = "" }
func (s *Sidebar) IsSearchActive() bool   { return s.searchActive }
func (s *Sidebar) GetSearchQuery() string { return s.searchQuery }
func (s *Sidebar) SetSearchQuery(q string) { s.searchQuery = q }

func (s *Sidebar) String() string {
	// Inner width accounts for border (2) + border padding (2)
	innerWidth := s.width - 6
	if innerWidth < 8 {
		innerWidth = 8
	}

	var b strings.Builder

	// Search bar
	searchWidth := innerWidth - 4 // search bar has its own border+padding
	if searchWidth < 4 {
		searchWidth = 4
	}
	if s.searchActive {
		searchText := s.searchQuery
		if searchText == "" {
			searchText = " "
		}
		b.WriteString(searchActiveBarStyle.Width(searchWidth).Render(searchText))
	} else {
		b.WriteString(searchBarStyle.Width(searchWidth).Render("/ search"))
	}
	b.WriteString("\n\n")

	// Items
	itemWidth := innerWidth - 2 // item padding
	if itemWidth < 4 {
		itemWidth = 4
	}
	for i, item := range s.items {
		if item.IsSection {
			b.WriteString(sectionHeaderStyle.Render("── " + item.Name + " ──"))
			b.WriteString("\n")
			continue
		}

		display := item.Name
		if item.Count > 0 {
			display = fmt.Sprintf("%s (%d)", display, item.Count)
		}

		if i == s.selectedIdx && s.focused {
			b.WriteString(selectedTopicStyle.Width(itemWidth).Render("▸ " + display))
		} else if i == s.selectedIdx && !s.focused {
			b.WriteString(activeTopicStyle.Width(itemWidth).Render("▸ " + display))
		} else {
			b.WriteString(topicItemStyle.Width(itemWidth).Render("  " + display))
		}
		b.WriteString("\n")
	}

	// Wrap content in the subtle rounded border
	bordered := sidebarBorderStyle.Width(innerWidth).Render(b.String())
	return lipgloss.Place(s.width, s.height, lipgloss.Left, lipgloss.Top, bordered)
}
