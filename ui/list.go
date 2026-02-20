package ui

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ByteMirror/hivemind/log"
	"github.com/ByteMirror/hivemind/session"

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
	repoFilter   string       // repo path filter (empty = show all repos)
	statusFilter StatusFilter // status filter (All or Active)
	sortMode     SortMode     // how instances are sorted
	allItems     []*session.Instance

	// expanded tracks which instances have their sub-agent tree expanded (by title).
	expanded map[string]bool
	// childExpanded tracks which parent instances have their brain-spawned children visible.
	childExpanded map[string]bool
}

func NewList(spinner *spinner.Model, autoYes bool) *List {
	return &List{
		items:    []*session.Instance{},
		renderer: &InstanceRenderer{spinner: spinner},
		repos:    make(map[string]int),
		autoyes:  autoYes,
		focused:  true,
		expanded:      make(map[string]bool),
		childExpanded: make(map[string]bool),
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
	// The list String() starts with a blank line, then the tab row.
	// Accept clicks on rows 1-2 to cover the tab area.
	log.InfoLog.Printf("HandleTabClick: localX=%d localY=%d", localX, localY)
	if localY < 1 || localY > 2 {
		log.InfoLog.Printf("HandleTabClick: Y out of range (need 1-2, got %d)", localY)
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

// KillInstanceByTitle kills and removes a single instance by its title.
func (l *List) KillInstanceByTitle(title string) {
	for i, inst := range l.allItems {
		if inst.Title == title {
			if err := inst.Kill(); err != nil {
				log.ErrorLog.Printf("could not kill instance %s: %v", inst.Title, err)
			}
			repoName, err := inst.RepoName()
			if err == nil {
				l.rmRepo(repoName)
			}
			l.allItems = append(l.allItems[:i], l.allItems[i+1:]...)
			l.rebuildFilteredItems()
			return
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

// SelectInstanceByRef finds an instance by pointer in the filtered list and selects it.
func (l *List) SelectInstanceByRef(instance *session.Instance) {
	for i, item := range l.items {
		if item == instance {
			l.selectedIdx = i
			return
		}
	}
}

// ToggleExpanded toggles the sub-agent tree for the currently selected instance.
// It first tries tmux sub-agents, then brain-spawned children.
// Returns true if the toggle was meaningful.
func (l *List) ToggleExpanded() bool {
	inst := l.GetSelectedInstance()
	if inst == nil {
		return false
	}
	// Try tmux sub-agents first.
	if inst.SubAgentCount > 0 {
		l.expanded[inst.Title] = !l.expanded[inst.Title]
		return true
	}
	// Then try brain-spawned children.
	return l.ToggleChildExpanded()
}

// ToggleChildExpanded toggles whether brain-spawned children of the selected instance are shown.
func (l *List) ToggleChildExpanded() bool {
	inst := l.GetSelectedInstance()
	if inst == nil {
		return false
	}
	hasChildren := false
	for _, item := range l.allItems {
		if item.ParentTitle == inst.Title {
			hasChildren = true
			break
		}
	}
	if !hasChildren {
		return false
	}
	l.childExpanded[inst.Title] = !l.childExpanded[inst.Title]
	l.rebuildFilteredItems()
	return true
}

// IsExpanded returns whether the given instance title has its sub-agent tree expanded.
func (l *List) IsExpanded(title string) bool {
	return l.expanded[title]
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
	l.repoFilter = ""
	l.rebuildFilteredItems()
}

// SetFilterByRepoAndTopic filters instances by both topic and repo path.
func (l *List) SetFilterByRepoAndTopic(topicFilter, repoPath string) {
	l.filter = topicFilter
	l.repoFilter = repoPath
	l.rebuildFilteredItems()
}

// SetSearchFilter filters instances by search query across all topics.
func (l *List) SetSearchFilter(query string) {
	l.SetSearchFilterWithTopic(query, "")
}

// SetSearchFilterWithTopic filters instances by search query, optionally scoped to a topic.
// topicFilter: "" = all topics, "__ungrouped__" = ungrouped only, otherwise = specific topic.
func (l *List) SetSearchFilterWithTopic(query string, topicFilter string) {
	l.SetSearchFilterWithTopicAndRepo(query, topicFilter, "")
}

// SetSearchFilterWithTopicAndRepo filters instances by search query, topic, and optionally repo.
func (l *List) SetSearchFilterWithTopicAndRepo(query string, topicFilter string, repoPath string) {
	l.filter = ""
	l.repoFilter = ""
	filtered := make([]*session.Instance, 0)
	for _, inst := range l.allItems {
		// Check status filter
		if l.statusFilter == StatusFilterActive && inst.Paused() {
			continue
		}
		// Check repo filter
		if repoPath != "" && l.instanceRepoPath(inst) != repoPath {
			continue
		}
		// Check topic filter
		if topicFilter != "" {
			if IsUngroupedID(topicFilter) && inst.TopicName != "" {
				continue
			} else if !IsUngroupedID(topicFilter) && inst.TopicName != topicFilter {
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

// instanceRepoPath returns the repo path for an instance, falling back to inst.Path.
func (l *List) instanceRepoPath(inst *session.Instance) string {
	rp := inst.GetRepoPath()
	if rp == "" {
		return inst.Path
	}
	return rp
}

// Clear removes all instances from the list.
func (l *List) Clear() {
	l.allItems = nil
	l.items = nil
	l.selectedIdx = 0
	l.filter = ""
	l.repoFilter = ""
}

func (l *List) rebuildFilteredItems() {
	// First apply topic + repo filter.
	// Always copy to avoid aliasing l.allItems — sorting l.items must not reorder l.allItems.
	var topicFiltered []*session.Instance
	if l.filter == "" {
		topicFiltered = make([]*session.Instance, 0, len(l.allItems))
		for _, inst := range l.allItems {
			if l.repoFilter != "" && l.instanceRepoPath(inst) != l.repoFilter {
				continue
			}
			topicFiltered = append(topicFiltered, inst)
		}
	} else if IsUngroupedID(l.filter) {
		topicFiltered = make([]*session.Instance, 0)
		repoPath := UngroupedRepoPath(l.filter)
		for _, inst := range l.allItems {
			if inst.TopicName != "" {
				continue
			}
			if repoPath != "" && l.instanceRepoPath(inst) != repoPath {
				continue
			}
			if l.repoFilter != "" && l.instanceRepoPath(inst) != l.repoFilter {
				continue
			}
			topicFiltered = append(topicFiltered, inst)
		}
	} else {
		topicFiltered = make([]*session.Instance, 0)
		for _, inst := range l.allItems {
			if inst.TopicName != l.filter {
				continue
			}
			if l.repoFilter != "" && l.instanceRepoPath(inst) != l.repoFilter {
				continue
			}
			topicFiltered = append(topicFiltered, inst)
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

	// Group brain-spawned children under their parents.
	l.items = l.groupChildrenUnderParents(l.items)

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

// groupChildrenUnderParents reorders items so that child instances (ParentTitle != "")
// appear directly after their parent, but only when the parent's children are expanded.
// Children whose parent is not expanded are hidden from the list.
//
// BrainChildCount is derived from allItems (the unfiltered master list) so the expand
// indicator is correct regardless of which topic filter is active. When a parent is
// expanded, its children are pulled from allItems too, ensuring they appear even if
// they wouldn't normally pass the current topic filter.
func (l *List) groupChildrenUnderParents(items []*session.Instance) []*session.Instance {
	// Build a lookup of ALL children from the unfiltered master list.
	allChildrenOf := make(map[string][]*session.Instance)
	for _, inst := range l.allItems {
		if inst.ParentTitle != "" {
			allChildrenOf[inst.ParentTitle] = append(allChildrenOf[inst.ParentTitle], inst)
		}
	}

	// Separate the filtered items into parents and children.
	var parents []*session.Instance
	filteredChildOf := make(map[string]bool) // children already in the filtered set
	for _, inst := range items {
		if inst.ParentTitle == "" {
			parents = append(parents, inst)
		} else {
			filteredChildOf[inst.Title] = true
		}
	}

	// Set BrainChildCount from allItems so the count is always accurate.
	for _, p := range parents {
		p.BrainChildCount = len(allChildrenOf[p.Title])
	}

	// Rebuild: parent followed by its children (if expanded).
	// Pull children from allItems so they appear even when filtered out by topic.
	var result []*session.Instance
	seen := make(map[string]bool) // track children added via expansion
	for _, p := range parents {
		result = append(result, p)
		if l.childExpanded[p.Title] {
			for _, child := range allChildrenOf[p.Title] {
				result = append(result, child)
				seen[child.Title] = true
			}
		}
	}

	// Orphan children (parent not in current filtered view) that were in the
	// filtered set but not yet added — append at the end.
	parentSeen := make(map[string]bool)
	for _, p := range parents {
		parentSeen[p.Title] = true
	}
	for parentTitle, children := range allChildrenOf {
		if parentSeen[parentTitle] {
			continue // parent is visible, children handled above
		}
		for _, child := range children {
			if filteredChildOf[child.Title] && !seen[child.Title] {
				result = append(result, child)
			}
		}
	}

	return result
}
