package ui

import (
	"hivemind/log"
	"hivemind/session"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const readyIcon = "● "
const pausedIcon = "\uf04c "

var readyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#51bd73", Dark: "#51bd73"})

var notifyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#F0A868"))

var addedLinesStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#51bd73", Dark: "#51bd73"})

var removedLinesStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#de613e"))

var pausedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"})

var titleStyle = lipgloss.NewStyle().
	Padding(1, 1, 0, 1).
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})

var listDescStyle = lipgloss.NewStyle().
	Padding(0, 1, 1, 1).
	Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"})

var evenRowTitleStyle = lipgloss.NewStyle().
	Padding(1, 1, 0, 1).
	Background(lipgloss.AdaptiveColor{Light: "#f5f5f5", Dark: "#1e1e1e"}).
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})

var evenRowDescStyle = lipgloss.NewStyle().
	Padding(0, 1, 1, 1).
	Background(lipgloss.AdaptiveColor{Light: "#f5f5f5", Dark: "#1e1e1e"}).
	Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"})

var selectedTitleStyle = lipgloss.NewStyle().
	Padding(1, 1, 0, 1).
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#1a1a1a"})

var selectedDescStyle = lipgloss.NewStyle().
	Padding(0, 1, 1, 1).
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#1a1a1a"})

// Active (unfocused) styles — muted version of selected
var activeTitleStyle = lipgloss.NewStyle().
	Padding(1, 1, 0, 1).
	Background(lipgloss.AdaptiveColor{Light: "#b0b0b0", Dark: "#666666"}).
	Foreground(lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#1a1a1a"})

var activeDescStyle = lipgloss.NewStyle().
	Padding(0, 1, 1, 1).
	Background(lipgloss.AdaptiveColor{Light: "#b0b0b0", Dark: "#666666"}).
	Foreground(lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#1a1a1a"})

var mainTitle = lipgloss.NewStyle().
	Background(lipgloss.Color("216")).
	Foreground(lipgloss.Color("230"))

var autoYesStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.Color("#1a1a1a"))

var resourceStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#777777"})

var activityStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#aaaaaa", Dark: "#666666"})

// Status filter tab styles
var activeFilterTab = lipgloss.NewStyle().
	Background(lipgloss.Color("216")).
	Foreground(lipgloss.Color("230")).
	Padding(0, 1)

var inactiveFilterTab = lipgloss.NewStyle().
	Background(lipgloss.AdaptiveColor{Light: "#b0b0b0", Dark: "#444444"}).
	Foreground(lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#999999"}).
	Padding(0, 1)

// StatusFilter determines which instances are shown based on their status.
type StatusFilter int

const (
	StatusFilterAll    StatusFilter = iota // Show all instances
	StatusFilterActive                     // Show only non-paused instances
)

type List struct {
	items         []*session.Instance
	selectedIdx   int
	height, width int
	renderer      *InstanceRenderer
	autoyes       bool
	focused       bool

	// map of repo name to number of instances using it. Used to display the repo name only if there are
	// multiple repos in play.
	repos map[string]int

	filter       string       // topic name filter (empty = show all)
	statusFilter StatusFilter // status filter (All or Active)
	allItems     []*session.Instance
}

func NewList(spinner *spinner.Model, autoYes bool) *List {
	return &List{
		items:    []*session.Instance{},
		renderer: &InstanceRenderer{spinner: spinner},
		repos:    make(map[string]int),
		autoyes:  autoYes,
		focused:  true,
	}
}

func (l *List) SetFocused(focused bool) {
	l.focused = focused
}

// SetStatusFilter sets the status filter and rebuilds the filtered items.
func (l *List) SetStatusFilter(filter StatusFilter) {
	l.statusFilter = filter
	l.rebuildFilteredItems()
}

// GetStatusFilter returns the current status filter.
func (l *List) GetStatusFilter() StatusFilter {
	return l.statusFilter
}

// GetSelectedIdx returns the index of the currently selected item in the filtered list.
func (l *List) GetSelectedIdx() int {
	return l.selectedIdx
}

// allTabText and activeTabText are the rendered tab labels with hotkey indicators.
const allTabText = "1 All"
const activeTabText = "2 Active"

// HandleTabClick checks if a click at the given local coordinates (relative to the
// list's top-left corner) hits a filter tab. Returns the filter and true if a tab was
// clicked, or false if the click was outside the tab area.
func (l *List) HandleTabClick(localX, localY int) (StatusFilter, bool) {
	// The list String() starts with 2 newlines, then the tab row, then 2 more
	// newlines. Accept clicks on rows 1-3 to cover the tab area generously,
	// since the exact row depends on how lipgloss.Place renders the output.
	if localY < 1 || localY > 3 {
		return 0, false
	}

	// Tab widths include Padding(0,1) so 1 char padding on each side.
	allWidth := len(allTabText) + 2  // "1 All" + 2 padding = 7
	activeWidth := len(activeTabText) + 2 // "2 Active" + 2 padding = 10

	if localX >= 0 && localX < allWidth {
		return StatusFilterAll, true
	} else if localX >= allWidth && localX < allWidth+activeWidth {
		return StatusFilterActive, true
	}
	return 0, false
}

// SetSize sets the height and width of the list.
func (l *List) SetSize(width, height int) {
	l.width = width
	l.height = height
	l.renderer.setWidth(width)
}

// SetSessionPreviewSize sets the height and width for the tmux sessions. This makes the stdout line have the correct
// width and height.
func (l *List) SetSessionPreviewSize(width, height int) (err error) {
	for i, item := range l.allItems {
		if !item.Started() || item.Paused() {
			continue
		}

		if innerErr := item.SetPreviewSize(width, height); innerErr != nil {
			err = errors.Join(
				err, fmt.Errorf("could not set preview size for instance %d: %v", i, innerErr))
		}
	}
	return
}

func (l *List) NumInstances() int {
	return len(l.items)
}

// InstanceRenderer handles rendering of session.Instance objects
type InstanceRenderer struct {
	spinner *spinner.Model
	width   int
}

func (r *InstanceRenderer) setWidth(width int) {
	r.width = AdjustPreviewWidth(width)
}

const branchIcon = "\uf126"

func (r *InstanceRenderer) Render(i *session.Instance, selected bool, focused bool, hasMultipleRepos bool, rowIndex int) string {
	prefix := " "
	titleS := selectedTitleStyle
	descS := selectedDescStyle
	if selected && !focused {
		// Active but unfocused — muted highlight
		titleS = activeTitleStyle
		descS = activeDescStyle
	} else if !selected {
		if rowIndex%2 == 1 {
			titleS = evenRowTitleStyle
			descS = evenRowDescStyle
		} else {
			titleS = titleStyle
			descS = listDescStyle
		}
	}

	// add spinner next to title if it's running
	var join string
	switch i.Status {
	case session.Running, session.Loading:
		join = fmt.Sprintf("%s ", r.spinner.View())
	case session.Ready:
		if i.Notified {
			t := (math.Sin(float64(time.Now().UnixMilli())/300.0) + 1.0) / 2.0
			cr := lerpByte(0x51, 0xF0, t)
			cg := lerpByte(0xBD, 0xA8, t)
			cb := lerpByte(0x73, 0x68, t)
			pulseStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", cr, cg, cb)))
			join = pulseStyle.Render(readyIcon)
		} else {
			join = readyStyle.Render(readyIcon)
		}
	case session.Paused:
		join = pausedStyle.Render(pausedIcon)
	default:
	}

	// Cut the title if it's too long
	titleText := i.Title
	widthAvail := r.width - 3 - runewidth.StringWidth(prefix) - 1
	if widthAvail > 0 && runewidth.StringWidth(titleText) > widthAvail {
		titleText = runewidth.Truncate(titleText, widthAvail-3, "...")
	}

	// Add skip-permissions indicator
	skipPermsIndicator := ""
	if i.SkipPermissions {
		skipPermsIndicator = " \uf132"
	}

	titleContent := fmt.Sprintf("%s %s%s", prefix, titleText, skipPermsIndicator)
	// Build title line: content + spaces + status icon, all fitting within r.width
	titleContentWidth := runewidth.StringWidth(titleContent)
	joinWidth := runewidth.StringWidth(join)
	titlePad := r.width - titleContentWidth - joinWidth - 2 // 2 for left/right padding in style
	if titlePad < 1 {
		titlePad = 1
	}
	titleLine := titleContent + strings.Repeat(" ", titlePad) + join
	title := titleS.Width(r.width).Render(titleLine)

	stat := i.GetDiffStats()

	var diff string
	var addedDiff, removedDiff string
	if stat == nil || stat.Error != nil || stat.IsEmpty() {
		// Don't show diff stats if there's an error or if they don't exist
		addedDiff = ""
		removedDiff = ""
		diff = ""
	} else {
		addedDiff = fmt.Sprintf("+%d", stat.Added)
		removedDiff = fmt.Sprintf("-%d ", stat.Removed)
		diff = lipgloss.JoinHorizontal(
			lipgloss.Center,
			addedLinesStyle.Background(descS.GetBackground()).Render(addedDiff),
			lipgloss.Style{}.Background(descS.GetBackground()).Foreground(descS.GetForeground()).Render(","),
			removedLinesStyle.Background(descS.GetBackground()).Render(removedDiff),
		)
	}

	remainingWidth := r.width
	remainingWidth -= runewidth.StringWidth(prefix)
	remainingWidth -= runewidth.StringWidth(branchIcon)

	diffWidth := runewidth.StringWidth(addedDiff) + runewidth.StringWidth(removedDiff)
	if diffWidth > 0 {
		diffWidth += 1
	}

	// Use fixed width for diff stats to avoid layout issues
	remainingWidth -= diffWidth

	branch := i.Branch
	if i.Started() && hasMultipleRepos {
		repoName, err := i.RepoName()
		if err != nil {
			log.ErrorLog.Printf("could not get repo name in instance renderer: %v", err)
		} else {
			branch += fmt.Sprintf(" (%s)", repoName)
		}
	}
	// Don't show branch if there's no space for it. Or show ellipsis if it's too long.
	branchWidth := runewidth.StringWidth(branch)
	if remainingWidth < 0 {
		branch = ""
	} else if remainingWidth < branchWidth {
		if remainingWidth < 3 {
			branch = ""
		} else {
			// We know the remainingWidth is at least 4 and branch is longer than that, so this is safe.
			branch = runewidth.Truncate(branch, remainingWidth-3, "...")
		}
	}
	remainingWidth -= runewidth.StringWidth(branch)

	// Build activity indicator for running instances.
	var activityText string
	if i.Status == session.Running && i.LastActivity != nil {
		act := i.LastActivity
		if act.Detail != "" {
			activityText = fmt.Sprintf(" \u00b7 %s %s", act.Action, act.Detail)
		} else {
			activityText = fmt.Sprintf(" \u00b7 %s", act.Action)
		}
		activityWidth := runewidth.StringWidth(activityText)
		// Only show if there is enough room (at least the separator + a few chars).
		if activityWidth > remainingWidth-1 {
			// Truncate or drop if it doesn't fit.
			avail := remainingWidth - 1 // leave at least 1 space before diff
			if avail > 5 {
				activityText = " " + runewidth.Truncate(activityText[1:], avail-1, "...")
			} else {
				activityText = ""
			}
		}
		remainingWidth -= runewidth.StringWidth(activityText)
	}

	// Add spaces to fill the remaining width.
	spaces := ""
	if remainingWidth > 0 {
		spaces = strings.Repeat(" ", remainingWidth)
	}

	// Render the activity text in a muted style.
	var renderedActivity string
	if activityText != "" {
		renderedActivity = activityStyle.Background(descS.GetBackground()).Render(activityText)
	}

	branchLine := fmt.Sprintf("%s %s-%s%s%s%s", strings.Repeat(" ", len(prefix)), branchIcon, branch, renderedActivity, spaces, diff)

	// Build resource usage line for non-paused instances (third line)
	var resourceLine string
	if i.Status != session.Paused && i.MemMB > 0 {
		cpuText := fmt.Sprintf("\U000f0d46 %.0f%%", i.CPUPercent)
		memText := fmt.Sprintf("\uefc5 %.0fM", i.MemMB)
		resourceContent := fmt.Sprintf("%s %s  %s", strings.Repeat(" ", len(prefix)), cpuText, memText)
		resourcePad := r.width - runewidth.StringWidth(resourceContent)
		if resourcePad < 0 {
			resourcePad = 0
		}
		resourceLine = resourceStyle.Render(resourceContent) + strings.Repeat(" ", resourcePad)
	}

	// join title, branch, and optionally resource line
	lines := []string{
		title,
		descS.Width(r.width).Render(branchLine),
	}
	if resourceLine != "" {
		lines = append(lines, descS.Width(r.width).Render(resourceLine))
	}
	text := lipgloss.JoinVertical(lipgloss.Left, lines...)

	return text
}

func (l *List) String() string {
	const autoYesText = " auto-yes "

	// Write the title.
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("\n")

	// Write filter tabs
	titleWidth := AdjustPreviewWidth(l.width) + 2

	allTab := inactiveFilterTab
	activeTab := inactiveFilterTab
	if l.statusFilter == StatusFilterAll {
		allTab = activeFilterTab
	} else {
		activeTab = activeFilterTab
	}

	tabs := lipgloss.JoinHorizontal(lipgloss.Bottom,
		allTab.Render(allTabText),
		activeTab.Render(activeTabText),
	)

	if !l.autoyes {
		b.WriteString(lipgloss.Place(
			titleWidth, 1, lipgloss.Left, lipgloss.Bottom, tabs))
	} else {
		title := lipgloss.Place(
			titleWidth/2, 1, lipgloss.Left, lipgloss.Bottom, tabs)
		autoYes := lipgloss.Place(
			titleWidth-(titleWidth/2), 1, lipgloss.Right, lipgloss.Bottom, autoYesStyle.Render(autoYesText))
		b.WriteString(lipgloss.JoinHorizontal(
			lipgloss.Top, title, autoYes))
	}

	b.WriteString("\n")
	b.WriteString("\n")

	// Render the list.
	for i, item := range l.items {
		b.WriteString(l.renderer.Render(item, i == l.selectedIdx, l.focused, len(l.repos) > 1, i))
		if i != len(l.items)-1 {
			b.WriteString("\n\n")
		}
	}
	return lipgloss.Place(l.width, l.height, lipgloss.Left, lipgloss.Top, b.String())
}

// itemHeight returns the rendered row count for an instance entry.
// Title style has Padding(1,0) top, desc style has Padding(0,1) bottom.
// 2-line item (title+branch) = 4 rows; 3-line (with resource) = 6 rows.
func (l *List) itemHeight(idx int) int {
	inst := l.items[idx]
	base := 4 // title (1 pad top + 1 content) + branch (1 content + 1 pad bottom)
	if inst.Status != session.Paused && inst.MemMB > 0 {
		base += 2 // resource line (1 content + 1 pad bottom)
	}
	return base
}

// GetItemAtRow maps a row offset (relative to the first item) to an item index.
// Returns -1 if the row doesn't correspond to any item.
func (l *List) GetItemAtRow(row int) int {
	currentRow := 0
	for i := range l.items {
		h := l.itemHeight(i)
		if row >= currentRow && row < currentRow+h {
			return i
		}
		currentRow += h + 1 // +1 for the blank line gap between items
	}
	return -1
}

// Down selects the next item in the list.
func (l *List) Down() {
	if len(l.items) == 0 {
		return
	}
	if l.selectedIdx < len(l.items)-1 {
		l.selectedIdx++
	}
}

// Kill removes and kills the currently selected instance.
func (l *List) Kill() {
	if len(l.items) == 0 {
		return
	}
	targetInstance := l.items[l.selectedIdx]

	// Kill the tmux session
	if err := targetInstance.Kill(); err != nil {
		log.ErrorLog.Printf("could not kill instance: %v", err)
	}

	// If you delete the last one in the list, select the previous one.
	if l.selectedIdx == len(l.items)-1 {
		defer l.Up()
	}

	// Unregister the reponame.
	repoName, err := targetInstance.RepoName()
	if err != nil {
		log.ErrorLog.Printf("could not get repo name: %v", err)
	} else {
		l.rmRepo(repoName)
	}

	// Remove from both items and allItems
	l.items = append(l.items[:l.selectedIdx], l.items[l.selectedIdx+1:]...)
	for i, inst := range l.allItems {
		if inst == targetInstance {
			l.allItems = append(l.allItems[:i], l.allItems[i+1:]...)
			break
		}
	}
}

// KillInstancesByTopic kills and removes all instances belonging to the given topic.
func (l *List) KillInstancesByTopic(topicName string) {
	var remaining []*session.Instance
	for _, inst := range l.allItems {
		if inst.TopicName == topicName {
			if err := inst.Kill(); err != nil {
				log.ErrorLog.Printf("could not kill instance %s: %v", inst.Title, err)
			}
			repoName, err := inst.RepoName()
			if err == nil {
				l.rmRepo(repoName)
			}
		} else {
			remaining = append(remaining, inst)
		}
	}
	l.allItems = remaining
	l.rebuildFilteredItems()
}

func (l *List) Attach() (chan struct{}, error) {
	targetInstance := l.items[l.selectedIdx]
	return targetInstance.Attach()
}

// Up selects the prev item in the list.
func (l *List) Up() {
	if len(l.items) == 0 {
		return
	}
	if l.selectedIdx > 0 {
		l.selectedIdx--
	}
}

func (l *List) addRepo(repo string) {
	if _, ok := l.repos[repo]; !ok {
		l.repos[repo] = 0
	}
	l.repos[repo]++
}

func (l *List) rmRepo(repo string) {
	if _, ok := l.repos[repo]; !ok {
		log.ErrorLog.Printf("repo %s not found", repo)
		return
	}
	l.repos[repo]--
	if l.repos[repo] == 0 {
		delete(l.repos, repo)
	}
}

// AddInstance adds a new instance to the list. It returns a finalizer function that should be called when the instance
// is started. If the instance was restored from storage or is paused, you can call the finalizer immediately.
// When creating a new one and entering the name, you want to call the finalizer once the name is done.
func (l *List) AddInstance(instance *session.Instance) (finalize func()) {
	l.allItems = append(l.allItems, instance)
	l.rebuildFilteredItems()
	// The finalizer registers the repo name once the instance is started.
	return func() {
		repoName, err := instance.RepoName()
		if err != nil {
			log.ErrorLog.Printf("could not get repo name: %v", err)
			return
		}

		l.addRepo(repoName)
	}
}

// GetSelectedInstance returns the currently selected instance
func (l *List) GetSelectedInstance() *session.Instance {
	if len(l.items) == 0 {
		return nil
	}
	return l.items[l.selectedIdx]
}

// SetSelectedInstance sets the selected index. Noop if the index is out of bounds.
func (l *List) SetSelectedInstance(idx int) {
	if idx >= len(l.items) {
		return
	}
	l.selectedIdx = idx
}

// GetInstances returns all instances (unfiltered) for persistence and metadata updates.
func (l *List) GetInstances() []*session.Instance {
	return l.allItems
}

// TotalInstances returns the total number of instances regardless of filter.
func (l *List) TotalInstances() int {
	return len(l.allItems)
}

// SetFilter filters the displayed instances by topic name.
// Empty string shows all. SidebarUngrouped shows only ungrouped instances.
func (l *List) SetFilter(topicFilter string) {
	l.filter = topicFilter
	l.rebuildFilteredItems()
}

// SetSearchFilter filters instances by search query across all topics.
// SetSearchFilter filters instances by search query across all topics.
func (l *List) SetSearchFilter(query string) {
	l.SetSearchFilterWithTopic(query, "")
}

// SetSearchFilterWithTopic filters instances by search query, optionally scoped to a topic.
// topicFilter: "" = all topics, "__ungrouped__" = ungrouped only, otherwise = specific topic.
func (l *List) SetSearchFilterWithTopic(query string, topicFilter string) {
	l.filter = ""
	filtered := make([]*session.Instance, 0)
	for _, inst := range l.allItems {
		// Check status filter
		if l.statusFilter == StatusFilterActive && inst.Paused() {
			continue
		}
		// Check topic filter
		if topicFilter != "" {
			if topicFilter == "__ungrouped__" && inst.TopicName != "" {
				continue
			} else if topicFilter != "__ungrouped__" && inst.TopicName != topicFilter {
				continue
			}
		}
		// Then check search query
		if query == "" ||
			strings.Contains(strings.ToLower(inst.Title), query) ||
			strings.Contains(strings.ToLower(inst.TopicName), query) {
			filtered = append(filtered, inst)
		}
	}
	l.items = filtered
	if l.selectedIdx >= len(l.items) {
		l.selectedIdx = len(l.items) - 1
	}
	if l.selectedIdx < 0 {
		l.selectedIdx = 0
	}
}

func (l *List) rebuildFilteredItems() {
	// First apply topic filter
	var topicFiltered []*session.Instance
	if l.filter == "" {
		topicFiltered = l.allItems
	} else if l.filter == SidebarUngrouped {
		topicFiltered = make([]*session.Instance, 0)
		for _, inst := range l.allItems {
			if inst.TopicName == "" {
				topicFiltered = append(topicFiltered, inst)
			}
		}
	} else {
		topicFiltered = make([]*session.Instance, 0)
		for _, inst := range l.allItems {
			if inst.TopicName == l.filter {
				topicFiltered = append(topicFiltered, inst)
			}
		}
	}

	// Then apply status filter
	if l.statusFilter == StatusFilterActive {
		filtered := make([]*session.Instance, 0)
		for _, inst := range topicFiltered {
			if !inst.Paused() {
				filtered = append(filtered, inst)
			}
		}
		l.items = filtered
	} else {
		l.items = topicFiltered
	}

	if l.selectedIdx >= len(l.items) {
		l.selectedIdx = len(l.items) - 1
	}
	if l.selectedIdx < 0 {
		l.selectedIdx = 0
	}
}
