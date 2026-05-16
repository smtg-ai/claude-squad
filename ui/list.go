package ui

import (
	"claude-squad/log"
	"claude-squad/session"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

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

var selectedDescLineStyle = lipgloss.NewStyle().
	Padding(0, 1, 0, 1).
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"})

var mainTitle = lipgloss.NewStyle().
	Background(lipgloss.Color("62")).
	Foreground(lipgloss.Color("230"))

var autoYesStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.Color("#1a1a1a"))

// matchHighlightStyle 搜索匹配子串的高亮样式
var matchHighlightStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#5B4A8A")).
	Foreground(lipgloss.Color("#FFFFFF"))

// HighlightMatch 在文本中高亮匹配搜索词的子串
func HighlightMatch(text, query string) string {
	if query == "" {
		return text
	}
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)

	var b strings.Builder
	lastIdx := 0
	for {
		idx := strings.Index(lowerText[lastIdx:], lowerQuery)
		if idx == -1 {
			b.WriteString(text[lastIdx:])
			break
		}
		absIdx := lastIdx + idx
		b.WriteString(text[lastIdx:absIdx])
		b.WriteString(matchHighlightStyle.Render(text[absIdx : absIdx+len(query)]))
		lastIdx = absIdx + len(query)
	}
	return b.String()
}

type List struct {
	items         []*session.Instance
	selectedIdx   int
	height, width int
	renderer      *InstanceRenderer
	autoyes       bool
	// descInputActive is true when the user is in stateDescription, typing a description.
	// When true, the ☰ line is rendered even if Description is empty, with a blinking cursor.
	descInputActive bool

	// map of repo name to number of instances using it. Used to display the repo name only if there are
	// multiple repos in play.
	repos map[string]int

	searchInput   textinput.Model
	searchFocused bool
	filteredItems []*session.Instance
}

func NewList(spinner *spinner.Model, autoYes bool) *List {
	si := textinput.New()
	si.Placeholder = "/ Search..."
	si.CharLimit = 64
	si.Width = 20

	return &List{
		items:         []*session.Instance{},
		renderer:      &InstanceRenderer{spinner: spinner},
		repos:         make(map[string]int),
		autoyes:       autoYes,
		searchInput:   si,
		filteredItems: nil, // nil 表示未过滤，显示全部
	}
}

// SetSize sets the height and width of the list.
func (l *List) SetSize(width, height int) {
	l.width = width
	l.height = height
	l.renderer.setWidth(width)
	l.searchInput.Width = AdjustPreviewWidth(width) - 4
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
	return len(l.visibleItems())
}

// InstanceRenderer handles rendering of session.Instance objects
type InstanceRenderer struct {
	spinner         *spinner.Model
	width           int
	descInputActive bool
	searchQuery     string
}

func (r *InstanceRenderer) setWidth(width int) {
	r.width = AdjustPreviewWidth(width)
}

// SetDescInputActive sets whether the description input is active.
// When active, the ☰ line is rendered even for empty descriptions, with a cursor.
func (l *List) SetDescInputActive(active bool) {
	l.descInputActive = active
	l.renderer.descInputActive = active
}

// SetSearchQueryOnRenderer 设置渲染器的搜索词（用于高亮）
func (l *List) SetSearchQueryOnRenderer(query string) {
	l.renderer.searchQuery = query
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
	descLineS := listDescStyle
	if selected {
		descLineS = selectedDescLineStyle
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
	if r.searchQuery != "" {
		titleText = HighlightMatch(titleText, r.searchQuery)
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

	if r.searchQuery != "" {
		branch = HighlightMatch(branch, r.searchQuery)
	}

	// Add spaces to fill the remaining width.
	spaces := ""
	if remainingWidth > 0 {
		spaces = strings.Repeat(" ", remainingWidth)
	}

	branchLine := fmt.Sprintf("%s %s-%s%s%s", strings.Repeat(" ", len(prefix)), branchIcon, branch, spaces, diff)

	// Description 行：有描述内容或正在输入描述时显示
	showDescLine := i.Description != "" || r.descInputActive
	if showDescLine {
		descPrefix := strings.Repeat(" ", len(prefix))
		descText := i.Description
		descWidthAvail := r.width - 3 - runewidth.StringWidth(prefix) - runewidth.StringWidth("☰ ")
		// 为光标预留 1 个字符宽度
		if r.descInputActive {
			descWidthAvail -= 1
		}
		if descWidthAvail > 0 && runewidth.StringWidth(descText) > descWidthAvail {
			descText = runewidth.Truncate(descText, descWidthAvail-1, "…")
		}
		if r.searchQuery != "" {
			descText = HighlightMatch(descText, r.searchQuery)
		}
		// 闪烁光标：利用 spinner 的当前帧判断是否显示
		cursor := ""
		if r.descInputActive {
			// spinner.View() 每 tick 返回不同内容，用其长度交替显示光标
			if len(r.spinner.View()) > 0 {
				cursor = "|"
			}
		}
		descLine := fmt.Sprintf("%s ☰ %s%s", descPrefix, descText, cursor)
		// 填充空格使背景色覆盖到右侧（与 branch 行一致）
		// descLine 格式: "%s ☰ %s%s" = descPrefix + 空格 + ☰ + 空格 + descText + cursor
		descContentWidth := runewidth.StringWidth(descPrefix) + 1 + runewidth.StringWidth("☰ ") + runewidth.StringWidth(descText) + runewidth.StringWidth(cursor)
		descRemaining := r.width - descContentWidth
		if descRemaining > 0 {
			descLine += strings.Repeat(" ", descRemaining)
		}
		text := lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			descLineS.Render(descLine),
			descS.Render(branchLine),
		)
		return text
	}

	// join title and subtitle
	text := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		descS.Render(branchLine),
	)

	return text
}

func (l *List) String() string {
	const titleText = " Instances "
	const autoYesText = " auto-yes "

	// Write the title.
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("\n")

	// Write title line
	// add padding of 2 because the border on list items adds some extra characters
	titleWidth := AdjustPreviewWidth(l.width) + 2
	if !l.autoyes {
		b.WriteString(lipgloss.Place(
			titleWidth, 1, lipgloss.Left, lipgloss.Bottom, mainTitle.Render(titleText)))
	} else {
		title := lipgloss.Place(
			titleWidth/2, 1, lipgloss.Left, lipgloss.Bottom, mainTitle.Render(titleText))
		autoYes := lipgloss.Place(
			titleWidth-(titleWidth/2), 1, lipgloss.Right, lipgloss.Bottom, autoYesStyle.Render(autoYesText))
		b.WriteString(lipgloss.JoinHorizontal(
			lipgloss.Top, title, autoYes))
	}

	b.WriteString("\n")
	b.WriteString("\n")

	// 渲染搜索框
	searchStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(AdjustPreviewWidth(l.width))
	if l.searchFocused {
		searchStyle = searchStyle.Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("99"))
	} else {
		searchStyle = searchStyle.Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("3C3C3C"))
	}
	b.WriteString(searchStyle.Render(l.searchInput.View()))
	b.WriteString("\n")

	// 渲染列表项
	items := l.visibleItems()
	if len(items) == 0 && l.filteredItems != nil {
		noMatchStyle := lipgloss.NewStyle().
			Padding(1, 1).
			Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"})
		b.WriteString(noMatchStyle.Render("No matching instances"))
	} else {
		for i, item := range items {
			b.WriteString(l.renderer.Render(item, i+1, i == l.selectedIdx, len(l.repos) > 1))
			if i != len(items)-1 {
				b.WriteString("\n\n")
			}
		}
	}
	return lipgloss.Place(l.width, l.height, lipgloss.Left, lipgloss.Top, b.String())
}

// Down selects the next item in the list.
func (l *List) Down() {
	items := l.visibleItems()
	if len(items) == 0 {
		return
	}
	if l.selectedIdx < len(items)-1 {
		l.selectedIdx++
	}
}

// Kill selects the next item in the list.
func (l *List) Kill() {
	items := l.visibleItems()
	if len(items) == 0 {
		return
	}
	targetInstance := items[l.selectedIdx]

	if err := targetInstance.Kill(); err != nil {
		log.ErrorLog.Printf("could not kill instance: %v", err)
	}

	// 在完整列表中找到并删除
	realIdx := -1
	for i, item := range l.items {
		if item == targetInstance {
			realIdx = i
			break
		}
	}
	if realIdx == -1 {
		return
	}

	repoName, err := targetInstance.RepoName()
	if err != nil {
		log.ErrorLog.Printf("could not get repo name: %v", err)
	} else {
		l.rmRepo(repoName)
	}

	l.items = append(l.items[:realIdx], l.items[realIdx+1:]...)

	// 确保 selectedIdx 在合法范围内
	if l.selectedIdx >= len(l.items) {
		l.selectedIdx = len(l.items) - 1
	}
	if l.selectedIdx < 0 {
		l.selectedIdx = 0
	}

	l.updateFilteredItems()
}

func (l *List) Attach() (chan struct{}, error) {
	items := l.visibleItems()
	if len(items) == 0 {
		return nil, fmt.Errorf("no instances to attach")
	}
	targetInstance := items[l.selectedIdx]
	return targetInstance.Attach()
}

// Up selects the prev item in the list.
func (l *List) Up() {
	items := l.visibleItems()
	if len(items) == 0 {
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
	l.items = append(l.items, instance)
	l.updateFilteredItems()
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
	items := l.visibleItems()
	if len(items) == 0 {
		return nil
	}
	if l.selectedIdx >= len(items) {
		return nil
	}
	return items[l.selectedIdx]
}

// SetSelectedInstance sets the selected index. Noop if the index is out of bounds.
func (l *List) SetSelectedInstance(idx int) {
	if idx >= len(l.visibleItems()) {
		return
	}
	l.selectedIdx = idx
}

// SelectInstance finds and selects the given instance in the list.
func (l *List) SelectInstance(target *session.Instance) {
	for i, inst := range l.visibleItems() {
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

// SetSearchFocused 设置搜索框的聚焦状态
func (l *List) SetSearchFocused(focused bool) {
	l.searchFocused = focused
	if focused {
		l.searchInput.Focus()
	} else {
		l.searchInput.Blur()
	}
}

// IsSearchFocused 返回搜索框是否聚焦
func (l *List) IsSearchFocused() bool {
	return l.searchFocused
}

// SearchInput 返回搜索框的指针，用于处理键盘事件
func (l *List) SearchInput() *textinput.Model {
	return &l.searchInput
}

// SetSearchQuery 设置搜索词并更新过滤结果
func (l *List) SetSearchQuery(query string) {
	l.searchInput.SetValue(query)
	l.updateFilteredItems()
}

// SearchQuery 返回当前搜索词
func (l *List) SearchQuery() string {
	return l.searchInput.Value()
}

// ClearSearch 清空搜索词并取消聚焦
func (l *List) ClearSearch() {
	l.searchInput.SetValue("")
	l.searchInput.Blur()
	l.searchFocused = false
	l.filteredItems = nil
}

// updateFilteredItems 根据当前搜索词过滤实例列表
func (l *List) updateFilteredItems() {
	query := l.searchInput.Value()
	if query == "" {
		l.filteredItems = nil
		l.SetSearchQueryOnRenderer("")
		return
	}

	// 保存原始搜索词用于高亮
	l.SetSearchQueryOnRenderer(query)

	// 保存当前选中实例（从 visibleItems 获取，而非 l.items）
	var currentSelected *session.Instance
	if vis := l.visibleItems(); l.selectedIdx < len(vis) {
		currentSelected = vis[l.selectedIdx]
	}

	queryLower := strings.ToLower(query)
	l.filteredItems = []*session.Instance{}
	for _, item := range l.items {
		if strings.Contains(strings.ToLower(item.Title), queryLower) ||
			strings.Contains(strings.ToLower(item.Description), queryLower) ||
			strings.Contains(strings.ToLower(item.Branch), queryLower) {
			l.filteredItems = append(l.filteredItems, item)
		}
	}

	// 如果当前选中项仍在过滤结果中，保持选中
	if currentSelected != nil && len(l.filteredItems) > 0 {
		found := false
		for _, fi := range l.filteredItems {
			if fi == currentSelected {
				found = true
				break
			}
		}
		if !found {
			l.selectedIdx = 0
		}
	} else if len(l.filteredItems) > 0 {
		l.selectedIdx = 0
	}
}

// UpdateFilter 重新计算过滤结果（在搜索输入变化后调用）
func (l *List) UpdateFilter() {
	l.updateFilteredItems()
}

// NumFilteredInstances 返回当前过滤后的实例数量
func (l *List) NumFilteredInstances() int {
	if l.filteredItems == nil {
		return len(l.items)
	}
	return len(l.filteredItems)
}

// GetFilteredInstance 返回过滤后指定索引的实例
func (l *List) GetFilteredInstance(idx int) *session.Instance {
	if l.filteredItems == nil {
		if idx >= len(l.items) {
			return nil
		}
		return l.items[idx]
	}
	if idx >= len(l.filteredItems) {
		return nil
	}
	return l.filteredItems[idx]
}

// visibleItems 返回当前可见的实例列表（过滤后或全部）
func (l *List) visibleItems() []*session.Instance {
	if l.filteredItems == nil {
		return l.items
	}
	return l.filteredItems
}
