package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

var sidebarTitleStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("216")).
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
	BorderForeground(lipgloss.Color("216")).
	Padding(0, 1)

var searchActiveBarStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7EC8D8")).
	Padding(0, 1)

const (
	SidebarAll       = "__all__"
	SidebarUngrouped = "__ungrouped__"
)

// dimmedTopicStyle is for topics with no matching instances during search
var dimmedTopicStyle = lipgloss.NewStyle().
	Padding(0, 1).
	Foreground(lipgloss.AdaptiveColor{Light: "#c0c0c0", Dark: "#444444"})

var sidebarRunningStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#51bd73"))

var sidebarReadyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#51bd73"))

var sidebarNotifyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#F0A868"))

// SidebarItem represents a selectable item in the sidebar.
type SidebarItem struct {
	Name            string
	ID              string
	IsSection       bool
	Count           int
	MatchCount      int  // search match count (-1 = not searching)
	SharedWorktree  bool // true if this topic has a shared worktree
	HasRunning      bool // true if this topic has running instances
	HasNotification bool // true if this topic has recently-finished instances
}

// Sidebar is the left-most panel showing topics and search.
type Sidebar struct {
	items         []SidebarItem
	selectedIdx   int
	height, width int
	focused       bool

	searchActive bool
	searchQuery  string

	repoName string // current repo name shown at bottom
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

// SetRepoName sets the current repo name displayed at the bottom of the sidebar.
func (s *Sidebar) SetRepoName(name string) {
	s.repoName = name
}

// TopicStatus holds status flags for a topic's instances.
type TopicStatus struct {
	HasRunning      bool
	HasNotification bool
}

// SetItems updates the sidebar items from the current topics.
// sharedTopics maps topic name → whether it has a shared worktree.
// topicStatuses maps topic name → running/notification status.
func (s *Sidebar) SetItems(topicNames []string, instanceCountByTopic map[string]int, ungroupedCount int, sharedTopics map[string]bool, topicStatuses map[string]TopicStatus) {
	totalCount := ungroupedCount
	for _, c := range instanceCountByTopic {
		totalCount += c
	}

	// Aggregate statuses for "All"
	anyRunning := false
	anyNotification := false
	for _, st := range topicStatuses {
		if st.HasRunning {
			anyRunning = true
		}
		if st.HasNotification {
			anyNotification = true
		}
	}

	items := []SidebarItem{
		{Name: "All", ID: SidebarAll, Count: totalCount, HasRunning: anyRunning, HasNotification: anyNotification},
	}

	if len(topicNames) > 0 {
		items = append(items, SidebarItem{Name: "Topics", IsSection: true})
		for _, name := range topicNames {
			count := instanceCountByTopic[name]
			st := topicStatuses[name]
			items = append(items, SidebarItem{
				Name: name, ID: name, Count: count,
				SharedWorktree: sharedTopics[name],
				HasRunning: st.HasRunning, HasNotification: st.HasNotification,
			})
		}
	}

	if ungroupedCount > 0 {
		ungroupedSt := topicStatuses[""]
		items = append(items, SidebarItem{Name: "Ungrouped", IsSection: true})
		items = append(items, SidebarItem{
			Name: "Ungrouped", ID: SidebarUngrouped, Count: ungroupedCount,
			HasRunning: ungroupedSt.HasRunning, HasNotification: ungroupedSt.HasNotification,
		})
	}

	s.items = items
	if s.selectedIdx >= len(items) {
		s.selectedIdx = len(items) - 1
	}
	if s.selectedIdx < 0 {
		s.selectedIdx = 0
	}
}

// GetSelectedIdx returns the index of the currently selected item in the sidebar.
func (s *Sidebar) GetSelectedIdx() int {
	return s.selectedIdx
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

// UpdateMatchCounts sets the search match counts for each topic item.
// Pass nil to clear search highlighting.
func (s *Sidebar) UpdateMatchCounts(matchesByTopic map[string]int, totalMatches int) {
	for i := range s.items {
		if s.items[i].IsSection {
			continue
		}
		if matchesByTopic == nil {
			s.items[i].MatchCount = -1 // not searching
			continue
		}
		switch s.items[i].ID {
		case SidebarAll:
			s.items[i].MatchCount = totalMatches
		case SidebarUngrouped:
			s.items[i].MatchCount = matchesByTopic[""]
		default:
			s.items[i].MatchCount = matchesByTopic[s.items[i].ID]
		}
	}
}

// SelectFirst selects the first non-section item (typically "All").
func (s *Sidebar) SelectFirst() {
	for i, item := range s.items {
		if !item.IsSection {
			s.selectedIdx = i
			return
		}
	}
}

func (s *Sidebar) ActivateSearch()        { s.searchActive = true; s.searchQuery = "" }
func (s *Sidebar) DeactivateSearch()      { s.searchActive = false; s.searchQuery = "" }
func (s *Sidebar) IsSearchActive() bool   { return s.searchActive }
func (s *Sidebar) GetSearchQuery() string { return s.searchQuery }
func (s *Sidebar) SetSearchQuery(q string) { s.searchQuery = q }

func (s *Sidebar) String() string {
	borderStyle := sidebarBorderStyle
	if s.focused {
		borderStyle = borderStyle.BorderForeground(lipgloss.Color("#F0A868"))
	} else {
		borderStyle = borderStyle.BorderForeground(lipgloss.AdaptiveColor{Light: "#d0d0d0", Dark: "#333333"})
	}

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
		b.WriteString(searchBarStyle.Width(searchWidth).Render("\uf002 search"))
	}
	b.WriteString("\n\n")

	// Items
	itemWidth := innerWidth - 2 // item padding
	if itemWidth < 4 {
		itemWidth = 4
	}
	for i, item := range s.items {
		// During search, hide section headers and topics with 0 matches
		if s.searchActive && s.searchQuery != "" {
			if item.IsSection {
				continue // hide section headers during search
			}
			if item.ID != SidebarAll && item.MatchCount == 0 {
				continue // hide topics with no matches
			}
		}

		if item.IsSection {
			b.WriteString(sectionHeaderStyle.Render("── " + item.Name + " ──"))
			b.WriteString("\n")
			continue
		}

		// Build trailing indicators: shared worktree icon + status dot
		var trailingIcons string
		trailingWidth := 0

		if item.SharedWorktree {
			trailingIcons += " \ue727"
			trailingWidth += 2
		}

		if item.HasNotification {
			trailingIcons += " ●"
			trailingWidth += 2
		} else if item.HasRunning {
			trailingIcons += " ●"
			trailingWidth += 2
		}

		// Build count suffix
		displayCount := item.Count
		if s.searchActive && item.MatchCount >= 0 {
			displayCount = item.MatchCount
		}
		countSuffix := ""
		if displayCount > 0 {
			countSuffix = fmt.Sprintf(" (%d)", displayCount)
		}

		// Truncate topic name to fit: itemWidth - prefix(1) - countSuffix - trailing
		nameText := item.Name
		maxNameWidth := itemWidth - 1 - runewidth.StringWidth(countSuffix) - trailingWidth
		if maxNameWidth < 3 {
			maxNameWidth = 3
		}
		if runewidth.StringWidth(nameText) > maxNameWidth {
			nameText = runewidth.Truncate(nameText, maxNameWidth-1, "…")
		}

		// Assemble plain text (no styled parts yet)
		plainText := nameText + countSuffix

		// Style the trailing icons
		var styledTrailing string
		if item.SharedWorktree {
			styledTrailing += " \ue727"
		}
		if item.HasNotification {
			if time.Now().UnixMilli()/500%2 == 0 {
				styledTrailing += " " + sidebarReadyStyle.Render("●")
			} else {
				styledTrailing += " " + sidebarNotifyStyle.Render("●")
			}
		} else if item.HasRunning {
			styledTrailing += " " + sidebarRunningStyle.Render("●")
		}

		if i == s.selectedIdx && s.focused {
			b.WriteString(selectedTopicStyle.Width(itemWidth).MaxWidth(itemWidth).Render("▸" + plainText + styledTrailing))
		} else if i == s.selectedIdx && !s.focused {
			b.WriteString(activeTopicStyle.Width(itemWidth).MaxWidth(itemWidth).Render("▸" + plainText + styledTrailing))
		} else {
			b.WriteString(topicItemStyle.Width(itemWidth).MaxWidth(itemWidth).Render(" " + plainText + styledTrailing))
		}
		b.WriteString("\n")
	}

	// Build repo indicator for the bottom
	var repoSection string
	if s.repoName != "" {
		repoSection = sectionHeaderStyle.Render("── Repo ──") + "\n" +
			topicItemStyle.Width(itemWidth).MaxWidth(itemWidth).Render("\uf1d3 " + s.repoName)
	}

	topContent := b.String()

	// Wrap content in the subtle rounded border — use full available height
	borderHeight := s.height - 2 // account for top border + bottom border
	if borderHeight < 4 {
		borderHeight = 4
	}

	innerContent := topContent
	if repoSection != "" {
		topLines := strings.Count(topContent, "\n") + 1
		repoLines := strings.Count(repoSection, "\n") + 1
		gap := borderHeight - topLines - repoLines
		if gap < 1 {
			gap = 1
		}
		innerContent = topContent + strings.Repeat("\n", gap) + repoSection
	}

	bordered := borderStyle.Width(innerWidth).Height(borderHeight).Render(innerContent)
	return lipgloss.Place(s.width, s.height, lipgloss.Left, lipgloss.Top, bordered)
}
