package ui

import (
	"claude-squad/config"
	"claude-squad/log"
	"claude-squad/session"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

var workspaceHeaderStyle = lipgloss.NewStyle().
	Padding(0, 1).
	Bold(true).
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})

var workspaceTabActiveStyle = lipgloss.NewStyle().
	Padding(0, 1).
	Bold(true).
	Background(lipgloss.Color("62")).
	Foreground(lipgloss.Color("230"))

var workspaceTabInactiveStyle = lipgloss.NewStyle().
	Padding(0, 1).
	Foreground(lipgloss.AdaptiveColor{Light: "#7A7474", Dark: "#9C9494"})

var countSummaryStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#7A7474", Dark: "#9C9494"})

var emptyHintStyle = lipgloss.NewStyle().
	Padding(2, 2).
	Foreground(lipgloss.AdaptiveColor{Light: "#7A7474", Dark: "#9C9494"})

const readyIcon = "● "
const pausedIcon = "⏸ "

var readyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#51bd73", Dark: "#51bd73"})

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

var selectedTitleStyle = lipgloss.NewStyle().
	Padding(1, 1, 0, 1).
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#1a1a1a"})

var selectedDescStyle = lipgloss.NewStyle().
	Padding(0, 1, 1, 1).
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#1a1a1a"})

var mainTitle = lipgloss.NewStyle().
	Background(lipgloss.Color("62")).
	Foreground(lipgloss.Color("230"))

var autoYesStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.Color("#1a1a1a"))

type List struct {
	items         []*session.Instance
	selectedIdx   int
	height, width int
	renderer      *InstanceRenderer
	autoyes       bool

	// activeWorkspaceName / activeWorkspaceID: where new sessions land (driven by W key).
	activeWorkspaceName string
	activeWorkspaceID   string

	// viewFilter is the workspace id to filter the visible list to. Empty = show all.
	viewFilter string

	// collapsedWorkspaces tracks which workspace groups are folded in the list.
	collapsedWorkspaces map[string]bool

	// map of repo name to number of instances using it. Used to display the repo name only if there are
	// multiple repos in play.
	repos map[string]int
}

// SetActiveWorkspace sets the display name shown in the title bar to indicate
// which workspace new sessions will be created in.
func (l *List) SetActiveWorkspace(name string) {
	l.activeWorkspaceName = name
}

// SetActiveWorkspaceID records the id of the workspace new sessions land in.
// Used to detect the "active" tab in the workspace tab bar.
func (l *List) SetActiveWorkspaceID(id string) {
	l.activeWorkspaceID = id
}

// SetViewFilter restricts the rendered list to instances belonging to the given workspace.
// Empty string means show all.
func (l *List) SetViewFilter(workspaceID string) {
	l.viewFilter = workspaceID
	// Snap selection to a visible item if the current one became hidden.
	l.ensureSelectionVisible()
}

// GetViewFilter returns the current view-filter workspace id, or "" if showing all.
func (l *List) GetViewFilter() string {
	return l.viewFilter
}

// ToggleCollapseCurrent folds/unfolds the workspace group containing the current selection.
// If folding hides the selection, selection jumps to the next visible item.
func (l *List) ToggleCollapseCurrent() {
	sel := l.GetSelectedInstance()
	if sel == nil {
		return
	}
	if l.collapsedWorkspaces == nil {
		l.collapsedWorkspaces = map[string]bool{}
	}
	id := sel.WorkspaceID
	l.collapsedWorkspaces[id] = !l.collapsedWorkspaces[id]
	l.ensureSelectionVisible()
}

// VisibleInstanceCount returns the number of instances matching the current view filter
// (ignoring collapse — collapsed items are "hidden" visually but still counted).
func (l *List) VisibleInstanceCount() int {
	if l.viewFilter == "" {
		return len(l.items)
	}
	n := 0
	for _, inst := range l.items {
		if inst.WorkspaceID == l.viewFilter {
			n++
		}
	}
	return n
}

// WorkspaceCount returns the number of distinct workspaces present in the list.
func (l *List) WorkspaceCount() int {
	seen := map[string]struct{}{}
	for _, inst := range l.items {
		seen[inst.WorkspaceID] = struct{}{}
	}
	return len(seen)
}

// isItemVisible reports whether the given instance is currently rendered
// (i.e. passes the view filter and is not inside a collapsed group).
func (l *List) isItemVisible(inst *session.Instance) bool {
	if l.viewFilter != "" && inst.WorkspaceID != l.viewFilter {
		return false
	}
	if l.collapsedWorkspaces[inst.WorkspaceID] {
		return false
	}
	return true
}

// ensureSelectionVisible nudges the selection forward (then backward) to the
// nearest visible item if the current selection has been hidden by filter/collapse.
func (l *List) ensureSelectionVisible() {
	if len(l.items) == 0 {
		return
	}
	if l.selectedIdx < len(l.items) && l.isItemVisible(l.items[l.selectedIdx]) {
		return
	}
	for i := l.selectedIdx + 1; i < len(l.items); i++ {
		if l.isItemVisible(l.items[i]) {
			l.selectedIdx = i
			return
		}
	}
	for i := l.selectedIdx - 1; i >= 0; i-- {
		if l.isItemVisible(l.items[i]) {
			l.selectedIdx = i
			return
		}
	}
}

func NewList(spinner *spinner.Model, autoYes bool) *List {
	return &List{
		items:               []*session.Instance{},
		renderer:            &InstanceRenderer{spinner: spinner},
		repos:               make(map[string]int),
		collapsedWorkspaces: map[string]bool{},
		autoyes:             autoYes,
	}
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
	for i, item := range l.items {
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

// ɹ and ɻ are other options.
const branchIcon = "Ꮧ"

func (r *InstanceRenderer) Render(i *session.Instance, idx int, selected bool, hasMultipleRepos bool) string {
	prefix := fmt.Sprintf(" %d. ", idx)
	if idx >= 10 {
		prefix = prefix[:len(prefix)-1]
	}
	titleS := selectedTitleStyle
	descS := selectedDescStyle
	if !selected {
		titleS = titleStyle
		descS = listDescStyle
	}

	// add spinner next to title if it's running
	var join string
	switch i.Status {
	case session.Running, session.Loading:
		join = fmt.Sprintf("%s ", r.spinner.View())
	case session.Ready:
		join = readyStyle.Render(readyIcon)
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
	title := titleS.Render(lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.Place(r.width-3, 1, lipgloss.Left, lipgloss.Center, fmt.Sprintf("%s %s", prefix, titleText)),
		" ",
		join,
	))

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
	remainingWidth -= 2 // for the literal " " and "-" in the branchLine format string

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

	// Add spaces to fill the remaining width.
	spaces := ""
	if remainingWidth > 0 {
		spaces = strings.Repeat(" ", remainingWidth)
	}

	branchLine := fmt.Sprintf("%s %s-%s%s%s", strings.Repeat(" ", len(prefix)), branchIcon, branch, spaces, diff)

	// join title and subtitle
	text := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		descS.Render(branchLine),
	)

	return text
}

func (l *List) String() string {
	header := l.renderHeader()
	headerLines := strings.Split(header, "\n")

	if len(l.items) == 0 {
		body := l.renderEmptyState()
		return lipgloss.Place(l.width, l.height, lipgloss.Left, lipgloss.Top,
			strings.Join(append(headerLines, strings.Split(body, "\n")...), "\n"))
	}

	bodyLines, selectedLine := l.renderBody()

	// Reserve every header line; the body scrolls inside the remainder so the
	// list pane never overflows its allotted height (which would otherwise push
	// the preview pane and menu below the visible terminal area).
	available := l.height - len(headerLines)
	if available < 1 {
		available = 1
	}
	if len(bodyLines) > available {
		bodyLines = scrollClamp(bodyLines, selectedLine, available)
	}

	final := append(headerLines, bodyLines...)
	return lipgloss.Place(l.width, l.height, lipgloss.Left, lipgloss.Top, strings.Join(final, "\n"))
}

// renderHeader builds the always-visible part of the list pane: the title bar,
// the workspace tab strip (which may itself wrap), and the count summary.
func (l *List) renderHeader() string {
	titleText := " Instances "
	if l.activeWorkspaceName != "" {
		titleText = fmt.Sprintf(" Instances · new → %s ", l.activeWorkspaceName)
	}
	const autoYesText = " auto-yes "

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("\n")

	titleWidth := AdjustPreviewWidth(l.width) + 2
	if !l.autoyes {
		b.WriteString(lipgloss.Place(
			titleWidth, 1, lipgloss.Left, lipgloss.Bottom, mainTitle.Render(titleText)))
	} else {
		title := lipgloss.Place(
			titleWidth/2, 1, lipgloss.Left, lipgloss.Bottom, mainTitle.Render(titleText))
		autoYes := lipgloss.Place(
			titleWidth-(titleWidth/2), 1, lipgloss.Right, lipgloss.Bottom, autoYesStyle.Render(autoYesText))
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, title, autoYes))
	}

	b.WriteString("\n")
	b.WriteString("\n")

	groups := l.groupedItems()
	tabs := l.workspaceTabDescriptors(groups)
	if len(tabs) >= 1 {
		b.WriteString(l.renderWorkspaceTabs(tabs))
		b.WriteString("\n")
		b.WriteString(countSummaryStyle.Render(l.renderCountSummary(tabs)))
		b.WriteString("\n")
	}
	return b.String()
}

// renderEmptyState produces the body for an empty list — workspace-aware so
// users see what to do next instead of a bare "No sessions yet."
func (l *List) renderEmptyState() string {
	if l.activeWorkspaceName != "" {
		return emptyHintStyle.Render(fmt.Sprintf(
			"No sessions in %q yet.\n\n  n   create a new session here\n  A   add another workspace\n  W   switch which workspace new sessions land in\n  ?   show help",
			l.activeWorkspaceName,
		))
	}
	return emptyHintStyle.Render(
		"No sessions yet.\n\n  A   add a workspace (existing or new directory)\n  ?   show help",
	)
}

// renderBody returns the body lines (group headers + instance items) and the
// line index of the currently-selected instance, used as the scroll anchor.
// Returns -1 for the anchor when the selection is outside any visible group.
func (l *List) renderBody() ([]string, int) {
	groups := l.groupedItems()
	hasMultipleRepos := len(l.repos) > 1
	var lines []string
	selectedLine := -1
	visibleIdx := 0
	first := true

	for _, g := range groups {
		if l.viewFilter != "" && g.id != l.viewFilter {
			continue
		}
		if !first {
			lines = append(lines, "")
		}
		first = false

		collapsed := l.collapsedWorkspaces[g.id]
		marker := "▾"
		if collapsed {
			marker = "▸"
		}
		lines = append(lines, workspaceHeaderStyle.Render(fmt.Sprintf("%s %s (%d)", marker, g.label, len(g.items))))
		if collapsed {
			continue
		}
		for j, item := range g.items {
			visibleIdx++
			origIdx := l.indexInItems(item)
			itemStartLine := len(lines)
			// Item separator: blank line between item rows so the rendered
			// title/branch pairs visually breathe.
			lines = append(lines, "")
			rendered := l.renderer.Render(item, visibleIdx, origIdx == l.selectedIdx, hasMultipleRepos)
			for _, rl := range strings.Split(rendered, "\n") {
				lines = append(lines, rl)
			}
			if origIdx == l.selectedIdx {
				selectedLine = itemStartLine
			}
			_ = j
		}
	}
	return lines, selectedLine
}

// scrollClamp returns the slice of body lines that should be visible given an
// available height. The window is positioned so the selected item is roughly
// centered, then clamped to [0, len(lines)-available].
func scrollClamp(lines []string, selectedLine, available int) []string {
	if available <= 0 || len(lines) <= available {
		return lines
	}
	if selectedLine < 0 {
		return lines[:available]
	}
	half := available / 2
	offset := selectedLine - half
	if offset < 0 {
		offset = 0
	}
	if offset+available > len(lines) {
		offset = len(lines) - available
	}
	return lines[offset : offset+available]
}

// tabDescriptor is one entry in the workspace tab bar.
type tabDescriptor struct {
	id    string
	label string
}

// workspaceTabDescriptors returns the workspace tabs to render: the union of
// workspaces present in the registry and any orphan WorkspaceIDs found in
// l.items (so we still show a tab for instances whose workspace was deleted).
// Order: registry order first, then orphan ids in their first-appearance order.
func (l *List) workspaceTabDescriptors(groups []instanceGroup) []tabDescriptor {
	reg := config.LoadWorkspaceRegistry()
	seen := map[string]struct{}{}
	out := make([]tabDescriptor, 0, len(reg.Workspaces)+len(groups))
	for _, w := range reg.Workspaces {
		seen[w.ID] = struct{}{}
		out = append(out, tabDescriptor{id: w.ID, label: w.DisplayName})
	}
	for _, g := range groups {
		if _, ok := seen[g.id]; ok {
			continue
		}
		out = append(out, tabDescriptor{id: g.id, label: g.label})
	}
	return out
}

// renderWorkspaceTabs renders a "[All] [foo] [bar]" tab strip, wrapping to a
// new row when the cumulative width of tabs would overflow the list pane.
// Without wrapping a long tab list pushes the rest of the UI off-screen.
func (l *List) renderWorkspaceTabs(tabs []tabDescriptor) string {
	cells := []string{l.tabRender("All", l.viewFilter == "")}
	for _, t := range tabs {
		cells = append(cells, l.tabRender(t.label, l.viewFilter == t.id))
	}

	// Effective width of the list pane after the renderer's adjustment. Leave
	// a one-char margin so wrapping kicks in slightly early; truncation in
	// edge cases (a single tab wider than the pane) is preferred over
	// overflow.
	maxWidth := AdjustPreviewWidth(l.width) - 1
	if maxWidth < 1 {
		maxWidth = 1
	}

	var rows []string
	var rowCells []string
	rowWidth := 0
	for _, cell := range cells {
		cellW := lipgloss.Width(cell)
		if len(rowCells) > 0 && rowWidth+cellW > maxWidth {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rowCells...))
			rowCells = nil
			rowWidth = 0
		}
		rowCells = append(rowCells, cell)
		rowWidth += cellW
	}
	if len(rowCells) > 0 {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rowCells...))
	}
	return strings.Join(rows, "\n")
}

func (l *List) tabRender(label string, active bool) string {
	if active {
		return workspaceTabActiveStyle.Render(label)
	}
	return workspaceTabInactiveStyle.Render(label)
}

// renderCountSummary builds the "5 sessions · 3 workspaces" status line beneath
// the tab bar (or "showing 2/5 sessions" when a view filter is active). The
// workspace count comes from the tab bar so empty (registered, no sessions)
// workspaces are reflected.
func (l *List) renderCountSummary(tabs []tabDescriptor) string {
	total := len(l.items)
	wsCount := len(tabs)
	if l.viewFilter == "" {
		return fmt.Sprintf(" %d %s · %d %s",
			total, plural(total, "session"),
			wsCount, plural(wsCount, "workspace"))
	}
	visible := l.VisibleInstanceCount()
	return fmt.Sprintf(" showing %d of %d %s in 1 of %d workspaces",
		visible, total, plural(total, "session"), wsCount)
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// indexInItems returns the index of inst within l.items, or -1 if not found.
// Used by String() to map a (group, item) pair back to the canonical index used by selectedIdx.
func (l *List) indexInItems(inst *session.Instance) int {
	for i, it := range l.items {
		if it == inst {
			return i
		}
	}
	return -1
}

type instanceGroup struct {
	id    string
	label string
	items []*session.Instance
}

// groupedItems partitions l.items by workspace id (preserving the existing
// item order within each group). Groups are returned in the same order their
// first instance appears in l.items, so adding/selecting doesn't reshuffle.
func (l *List) groupedItems() []instanceGroup {
	if len(l.items) == 0 {
		return nil
	}
	reg := config.LoadWorkspaceRegistry()
	labels := make(map[string]string)
	order := []string{}
	groups := map[string][]*session.Instance{}
	for _, inst := range l.items {
		key := inst.WorkspaceID
		if _, ok := groups[key]; !ok {
			order = append(order, key)
			labels[key] = workspaceLabel(reg, key)
		}
		groups[key] = append(groups[key], inst)
	}
	out := make([]instanceGroup, 0, len(order))
	for _, k := range order {
		out = append(out, instanceGroup{id: k, label: labels[k], items: groups[k]})
	}
	// Stable secondary sort: groups with a known workspace before unknown.
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].label == unknownWorkspaceLabel) != (out[j].label == unknownWorkspaceLabel) {
			return out[j].label == unknownWorkspaceLabel
		}
		return false
	})
	return out
}

const unknownWorkspaceLabel = "(unknown workspace)"

func workspaceLabel(reg *config.WorkspaceRegistry, id string) string {
	if id == "" {
		return unknownWorkspaceLabel
	}
	if ws := reg.Get(id); ws != nil {
		return ws.DisplayName
	}
	return unknownWorkspaceLabel
}

// Down selects the next visible item in the list (skipping items hidden by the
// current view filter or by a collapsed workspace group).
func (l *List) Down() {
	if len(l.items) == 0 {
		return
	}
	for i := l.selectedIdx + 1; i < len(l.items); i++ {
		if l.isItemVisible(l.items[i]) {
			l.selectedIdx = i
			return
		}
	}
}

// Kill selects the next item in the list.
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

	// Since there's items after this, the selectedIdx can stay the same.
	l.items = append(l.items[:l.selectedIdx], l.items[l.selectedIdx+1:]...)
	l.ensureSelectionVisible()
}

func (l *List) Attach() (chan struct{}, error) {
	targetInstance := l.items[l.selectedIdx]
	return targetInstance.Attach()
}

// Up selects the prev visible item in the list (skipping items hidden by filter or collapse).
func (l *List) Up() {
	if len(l.items) == 0 {
		return
	}
	for i := l.selectedIdx - 1; i >= 0; i-- {
		if l.isItemVisible(l.items[i]) {
			l.selectedIdx = i
			return
		}
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
	l.items = append(l.items, instance)
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

// SelectInstance finds and selects the given instance in the list.
func (l *List) SelectInstance(target *session.Instance) {
	for i, inst := range l.items {
		if inst == target {
			l.SetSelectedInstance(i)
			return
		}
	}
}

// GetInstances returns all instances in the list
func (l *List) GetInstances() []*session.Instance {
	return l.items
}
