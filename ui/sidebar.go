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
	MatchCount      int    // search match count (-1 = not searching)
	SharedWorktree  bool   // true if this topic has a shared worktree
	AutoYes         bool   // true if this topic has auto-accept enabled
	HasRunning      bool   // true if this topic has running instances
	HasNotification bool   // true if this topic has recently-finished instances
	RepoPath        string // repo path this item belongs to (for multi-repo disambiguation)
}

// Sidebar is the left-most panel showing topics and search.
type Sidebar struct {
	items         []SidebarItem
	selectedIdx   int
	height, width int
	focused       bool

	searchActive bool
	searchQuery  string

	repoName    string // current repo name shown at bottom
	repoHovered bool   // true when mouse is hovering over the repo button

	// Repo button screen-relative bounds (set during render).
	// Coordinates are relative to the sidebar's top-left (0,0).
	repoBtnTop, repoBtnBot int
	repoBtnLeft, repoBtnRight int
	repoBtnVisible bool
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

// SetRepoHovered sets whether the mouse is hovering over the repo button.
func (s *Sidebar) SetRepoHovered(hovered bool) {
	s.repoHovered = hovered
}

// IsRepoBtnHit tests whether screen coordinates (x, y) fall within the repo button.
// x and y are absolute screen coordinates; the sidebar is at column 0, row screenTop.
func (s *Sidebar) IsRepoBtnHit(x, y, screenTop int) bool {
	if !s.repoBtnVisible {
		return false
	}
	absTop := screenTop + s.repoBtnTop
	absBot := screenTop + s.repoBtnBot
	return x >= s.repoBtnLeft && x <= s.repoBtnRight && y >= absTop && y <= absBot
}

// TopicStatus holds status flags for a topic's instances.
type TopicStatus struct {
	HasRunning      bool
	HasNotification bool
}

// SetItems updates the sidebar items from the current topics.
// sharedTopics maps topic name → whether it has a shared worktree.
// topicStatuses maps topic name → running/notification status.
func (s *Sidebar) SetItems(topicNames []string, instanceCountByTopic map[string]int, ungroupedCount int, sharedTopics map[string]bool, autoYesTopics map[string]bool, topicStatuses map[string]TopicStatus) {
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

	for _, name := range topicNames {
		count := instanceCountByTopic[name]
		st := topicStatuses[name]
		items = append(items, SidebarItem{
			Name: name, ID: name, Count: count,
			SharedWorktree: sharedTopics[name],
			AutoYes:        autoYesTopics[name],
			HasRunning:     st.HasRunning, HasNotification: st.HasNotification,
		})
	}

	if ungroupedCount > 0 {
		ungroupedSt := topicStatuses[""]
		items = append(items, SidebarItem{
			Name: "Ungrouped", ID: SidebarUngrouped, Count: ungroupedCount,
			HasRunning: ungroupedSt.HasRunning, HasNotification: ungroupedSt.HasNotification,
		})
	}

	s.applyItems(items)
}

// applyItems sets the sidebar items, preserving search match counts and clamping selection.
func (s *Sidebar) applyItems(items []SidebarItem) {
	if s.searchActive {
		oldCounts := make(map[string]int, len(s.items))
		for _, item := range s.items {
			oldCounts[item.ID] = item.MatchCount
		}
		for i := range items {
			if mc, ok := oldCounts[items[i].ID]; ok {
				items[i].MatchCount = mc
			}
		}
	}

	s.items = items
	if s.selectedIdx >= len(items) {
		s.selectedIdx = len(items) - 1
	}
	if s.selectedIdx < 0 {
		s.selectedIdx = 0
	}
}

// RepoGroup holds all sidebar data for a single repo in multi-repo view.
type RepoGroup struct {
	RepoPath       string
	RepoName       string
	TopicNames     []string
	CountByTopic   map[string]int
	UngroupedCount int
	SharedTopics   map[string]bool
	AutoYesTopics  map[string]bool
	TopicStatuses  map[string]TopicStatus
}

// SetGroupedItems builds sidebar items with repo section headers for multi-repo view.
func (s *Sidebar) SetGroupedItems(groups []RepoGroup) {
	totalCount := 0
	anyRunning := false
	anyNotification := false
	for _, g := range groups {
		totalCount += g.UngroupedCount
		for _, c := range g.CountByTopic {
			totalCount += c
		}
		for _, st := range g.TopicStatuses {
			if st.HasRunning {
				anyRunning = true
			}
			if st.HasNotification {
				anyNotification = true
			}
		}
	}

	items := []SidebarItem{
		{Name: "All", ID: SidebarAll, Count: totalCount, HasRunning: anyRunning, HasNotification: anyNotification},
	}

	for _, g := range groups {
		// Section header for this repo
		items = append(items, SidebarItem{Name: g.RepoName, IsSection: true})

		for _, name := range g.TopicNames {
			count := g.CountByTopic[name]
			st := g.TopicStatuses[name]
			items = append(items, SidebarItem{
				Name: name, ID: name, Count: count,
				SharedWorktree: g.SharedTopics[name],
				AutoYes:        g.AutoYesTopics[name],
				HasRunning:     st.HasRunning, HasNotification: st.HasNotification,
				RepoPath: g.RepoPath,
			})
		}

		if g.UngroupedCount > 0 {
			ungroupedSt := g.TopicStatuses[""]
			ungroupedID := SidebarUngrouped + ":" + g.RepoPath
			items = append(items, SidebarItem{
				Name: "Ungrouped", ID: ungroupedID, Count: g.UngroupedCount,
				HasRunning: ungroupedSt.HasRunning, HasNotification: ungroupedSt.HasNotification,
				RepoPath: g.RepoPath,
			})
		}
	}

	s.applyItems(items)
}

// GetSelectedRepoPath returns the RepoPath of the currently selected sidebar item.
func (s *Sidebar) GetSelectedRepoPath() string {
	if len(s.items) == 0 || s.selectedIdx >= len(s.items) {
		return ""
	}
	return s.items[s.selectedIdx].RepoPath
}

// SetRepoNames sets the repo name(s) displayed at the bottom of the sidebar.
func (s *Sidebar) SetRepoNames(names []string) {
	if len(names) == 1 {
		s.repoName = names[0]
	} else {
		s.repoName = fmt.Sprintf("%d repos", len(names))
	}
}

// IsUngroupedID checks if an ID represents an ungrouped item (single or per-repo).
func IsUngroupedID(id string) bool {
	return id == SidebarUngrouped || strings.HasPrefix(id, SidebarUngrouped+":")
}

// UngroupedRepoPath extracts the repo path from a per-repo ungrouped ID.
// Returns empty string for the plain SidebarUngrouped ID.
func UngroupedRepoPath(id string) string {
	if strings.HasPrefix(id, SidebarUngrouped+":") {
		return strings.TrimPrefix(id, SidebarUngrouped+":")
	}
	return ""
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
		switch {
		case s.items[i].ID == SidebarAll:
			s.items[i].MatchCount = totalMatches
		case IsUngroupedID(s.items[i].ID):
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

// SelectLast selects the last non-section item.
func (s *Sidebar) SelectLast() {
	for i := len(s.items) - 1; i >= 0; i-- {
		if !s.items[i].IsSection {
			s.selectedIdx = i
			return
		}
	}
}

func (s *Sidebar) ActivateSearch()         { s.searchActive = true; s.searchQuery = "" }
func (s *Sidebar) DeactivateSearch()       { s.searchActive = false; s.searchQuery = "" }
func (s *Sidebar) IsSearchActive() bool    { return s.searchActive }
func (s *Sidebar) GetSearchQuery() string  { return s.searchQuery }
func (s *Sidebar) SetSearchQuery(q string) { s.searchQuery = q }

func (s *Sidebar) String() string {
	borderStyle := sidebarBorderStyle
	if s.focused {
		borderStyle = borderStyle.BorderForeground(lipgloss.Color("#F0A868"))
	} else {
		borderStyle = borderStyle.BorderForeground(lipgloss.AdaptiveColor{Light: "#d0d0d0", Dark: "#333333"})
	}

	// innerWidth is the lipgloss Width param for the border (includes padding, excludes border).
	// Drawable content area inside border+padding = innerWidth - 2.
	innerWidth := s.width - 2
	if innerWidth < 8 {
		innerWidth = 8
	}
	contentWidth := innerWidth - 2 // actual drawable area after padding

	var b strings.Builder

	// lipgloss Width includes padding but excludes border; subtract border(2) only
	searchWidth := contentWidth - 2
	if searchWidth < 4 {
		searchWidth = 4
	}
	if s.searchActive {
		// Blinking cursor: visible half the time (~500ms cycle)
		cursor := "▎"
		if time.Now().UnixMilli()/500%2 == 0 {
			cursor = " "
		}
		searchText := "\uf002 " + s.searchQuery + cursor
		b.WriteString(searchActiveBarStyle.Width(searchWidth).Render(searchText))
	} else {
		b.WriteString(searchBarStyle.Width(searchWidth).Render("\uf002 search"))
	}
	b.WriteString("\n\n")

	// Items fill the content area; their own Padding(0,1) handles text inset
	itemWidth := contentWidth
	if itemWidth < 4 {
		itemWidth = 4
	}
	for i, item := range s.items {
		// During search, hide section headers and topics with 0 matches
		if s.searchActive && s.searchQuery != "" {
			if item.IsSection {
				continue
			}
			if item.ID != SidebarAll && !IsUngroupedID(item.ID) && item.MatchCount == 0 {
				continue
			}
			if IsUngroupedID(item.ID) && item.MatchCount == 0 {
				continue
			}
		}

		if item.IsSection {
			b.WriteString(sectionHeaderStyle.Render("── " + item.Name + " ──"))
			b.WriteString("\n")
			continue
		}

		// Fixed-slot layout: [prefix 1ch] [name+count flexible] [icons fixed right]
		// Content area = itemWidth - 2 (Padding(0,1) in item styles)
		contentWidth := itemWidth - 2

		// Build count string for the icon slot
		displayCount := item.Count
		if s.searchActive && item.MatchCount >= 0 {
			displayCount = item.MatchCount
		}
		countStr := ""
		if displayCount > 0 {
			countStr = fmt.Sprintf("%d", displayCount)
		}

		// Build trailing icons and measure their fixed width
		trailingWidth := 0
		if countStr != "" {
			trailingWidth += 1 + runewidth.StringWidth(countStr) // " N"
		}
		if item.SharedWorktree {
			trailingWidth += 2 // " \ue727"
		}
		if item.AutoYes {
			trailingWidth += 2 // " \uf00c"
		}
		if item.HasNotification || item.HasRunning {
			trailingWidth += 2 // " ●"
		}

		// Truncate name to fit: contentWidth - prefix(1) - trailing
		nameText := item.Name
		maxNameWidth := contentWidth - 1 - trailingWidth
		if maxNameWidth < 3 {
			maxNameWidth = 3
		}
		if runewidth.StringWidth(nameText) > maxNameWidth {
			nameText = runewidth.Truncate(nameText, maxNameWidth-1, "…")
		}

		// Left part: prefix + name
		prefix := " "
		if i == s.selectedIdx {
			prefix = "▸"
		}
		leftPart := prefix + nameText
		leftWidth := runewidth.StringWidth(leftPart)

		// Pad between left and right to push icons to the right edge
		gap := contentWidth - leftWidth - trailingWidth
		if gap < 0 {
			gap = 0
		}
		paddedLeft := leftPart + strings.Repeat(" ", gap)

		// Style the trailing icons (count + shared worktree + status dot).
		// The count is rendered unstyled (inherits parent colors) so it
		// doesn't break the highlight background on selected items.
		var styledTrailing string
		if countStr != "" {
			styledTrailing += " " + countStr
		}
		if item.SharedWorktree {
			styledTrailing += " \ue727"
		}
		if item.AutoYes {
			styledTrailing += " \uf00c"
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

		line := paddedLeft + styledTrailing
		if i == s.selectedIdx && s.focused {
			b.WriteString(selectedTopicStyle.Width(itemWidth).Render(line))
		} else if i == s.selectedIdx && !s.focused {
			b.WriteString(activeTopicStyle.Width(itemWidth).Render(line))
		} else {
			b.WriteString(topicItemStyle.Width(itemWidth).Render(line))
		}
		b.WriteString("\n")
	}

	// Build repo indicator as a clickable dropdown button at the bottom.
	var repoSection string
	if s.repoName != "" {
		btnWidth := contentWidth - 2 // lipgloss Width includes padding, excludes border(2)
		if btnWidth < 4 {
			btnWidth = 4
		}

		borderColor := lipgloss.AdaptiveColor{Light: "#c0c0c0", Dark: "#555555"}
		textColor := lipgloss.AdaptiveColor{Light: "#555555", Dark: "#aaaaaa"}
		if s.repoHovered {
			borderColor = lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"}
			textColor = lipgloss.AdaptiveColor{Light: "#000000", Dark: "#ffffff"}
		}

		// Truncate repo name to fit: btnWidth - padding(2) - arrow
		arrowStr := " ▾"
		contentWidth := btnWidth - 2 // subtract padding
		maxNameLen := contentWidth - runewidth.StringWidth(arrowStr)
		displayName := s.repoName
		if runewidth.StringWidth(displayName) > maxNameLen {
			displayName = runewidth.Truncate(displayName, maxNameLen-1, "…")
		}

		btnStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Foreground(textColor).
			Width(btnWidth).
			Padding(0, 1)

		repoSection = btnStyle.Render(displayName + arrowStr)
	}

	topContent := b.String()

	// borderHeight is the lipgloss Height param (includes padding, excludes border).
	// Content lines available = borderHeight - bottom_padding (top padding is 0).
	borderHeight := s.height - 2
	if borderHeight < 4 {
		borderHeight = 4
	}
	contentLines := borderHeight // no vertical padding (top and bottom are 0)

	s.repoBtnVisible = false
	innerContent := topContent
	if repoSection != "" {
		topLines := strings.Count(topContent, "\n") + 1
		repoLines := strings.Count(repoSection, "\n") + 1
		gap := contentLines - topLines - repoLines + 1
		if gap < 1 {
			gap = 1
		}
		innerContent = topContent + strings.Repeat("\n", gap) + repoSection

		// Compute button bounds relative to sidebar top-left using bottom-relative
		// positioning (more reliable than counting content lines from the top).
		// Sidebar layout: row 0 = top border, rows 1..borderHeight = content,
		// row borderHeight+1 = bottom border. Button is the last repoLines of content.
		s.repoBtnBot = borderHeight                  // last content row
		s.repoBtnTop = borderHeight - repoLines + 1  // first button row
		s.repoBtnLeft = 2                             // border(1) + left padding(1)
		s.repoBtnRight = 2 + contentWidth - 1         // button fills contentWidth
		s.repoBtnVisible = true
	}

	bordered := borderStyle.Width(innerWidth).Height(borderHeight).Render(innerContent)
	return lipgloss.Place(s.width, s.height, lipgloss.Left, lipgloss.Top, bordered)
}
