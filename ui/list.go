package ui

import (
	"errors"
	"fmt"
	"github.com/ByteMirror/hivemind/log"
	"github.com/ByteMirror/hivemind/session"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
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
	sortMode     SortMode     // how instances are sorted
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

// CycleSortMode advances to the next sort mode and rebuilds.
func (l *List) CycleSortMode() {
	l.sortMode = (l.sortMode + 1) % 4
	l.rebuildFilteredItems()
}

// GetSortMode returns the current sort mode.
func (l *List) GetSortMode() SortMode {
	return l.sortMode
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
	allWidth := len(allTabText) + 2       // "1 All" + 2 padding = 7
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

// Clear removes all instances from the list.
func (l *List) Clear() {
	l.allItems = nil
	l.items = nil
	l.selectedIdx = 0
	l.filter = ""
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

	// Apply sort
	l.sortItems()

	if l.selectedIdx >= len(l.items) {
		l.selectedIdx = len(l.items) - 1
	}
	if l.selectedIdx < 0 {
		l.selectedIdx = 0
	}
}

func (l *List) sortItems() {
	switch l.sortMode {
	case SortNewest:
		sort.SliceStable(l.items, func(i, j int) bool {
			return l.items[i].UpdatedAt.After(l.items[j].UpdatedAt)
		})
	case SortOldest:
		sort.SliceStable(l.items, func(i, j int) bool {
			return l.items[i].CreatedAt.Before(l.items[j].CreatedAt)
		})
	case SortName:
		sort.SliceStable(l.items, func(i, j int) bool {
			return strings.ToLower(l.items[i].Title) < strings.ToLower(l.items[j].Title)
		})
	case SortStatus:
		sort.SliceStable(l.items, func(i, j int) bool {
			return l.items[i].Status < l.items[j].Status
		})
	}
}
