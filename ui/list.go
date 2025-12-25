package ui

import (
	"claude-squad/log"
	"claude-squad/session"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
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

var mainTitle = lipgloss.NewStyle().
	Background(lipgloss.Color("62")).
	Foreground(lipgloss.Color("230"))

var autoYesStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.Color("#1a1a1a"))

type List struct {
	items          []*session.Instance
	selectedIdx    int
	height, width  int
	renderer       *InstanceRenderer
	autoyes        bool
	verticalLayout bool
	scrollOffset   int // Track scroll position for vertical scrolling

	// map of repo name to number of instances using it. Used to display the repo name only if there are
	// multiple repos in play.
	repos map[string]int
}

func NewList(spinner *spinner.Model, autoYes bool) *List {
	return &List{
		items:          []*session.Instance{},
		renderer:       &InstanceRenderer{spinner: spinner},
		repos:          make(map[string]int),
		autoyes:        autoYes,
		verticalLayout: false,
		scrollOffset:   0,
	}
}

// SetSize sets the height and width of the list.
func (l *List) SetSize(width, height int) {
	l.width = width
	l.height = height
	l.renderer.setWidth(width, l.verticalLayout)
}

// SetVerticalLayout enables or disables vertical layout mode
func (l *List) SetVerticalLayout(vertical bool) {
	l.verticalLayout = vertical
	// Update renderer width when layout changes
	l.renderer.setWidth(l.width, l.verticalLayout)
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

func (r *InstanceRenderer) setWidth(width int, verticalLayout bool) {
	if verticalLayout {
		r.width = width
	} else {
		r.width = AdjustPreviewWidth(width)
	}
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
	case session.Running:
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
	const titleText = " Instances "
	const autoYesText = " auto-yes "

	// Write the title.
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("\n")

	// Write title line
	// add padding of 2 because the border on list items adds some extra characters
	var titleWidth int
	if l.verticalLayout {
		titleWidth = l.width + 2
	} else {
		titleWidth = AdjustPreviewWidth(l.width) + 2
	}

	if !l.autoyes {
		b.WriteString(lipgloss.Place(
			titleWidth, 1, lipgloss.Left, lipgloss.Bottom, mainTitle.Render(titleText)))
	} else {
		title := lipgloss.Place(
			titleWidth/2, 1, lipgloss.Left, lipgloss.Bottom, mainTitle.Render(titleText))

		// In vertical layout (small screen), reduce the auto-yes box width to prevent overflow
		var autoYesWidth int
		var autoYesAlign lipgloss.Position
		if l.verticalLayout {
			// For small screens, use a smaller width and center alignment to avoid overflow
			autoYesWidth = titleWidth/2 - 4 // Leave some padding to prevent overflow
			autoYesAlign = lipgloss.Center
		} else {
			// For full layout, use the original logic
			autoYesWidth = titleWidth - (titleWidth / 2)
			autoYesAlign = lipgloss.Right
		}

		autoYes := lipgloss.Place(
			autoYesWidth, 1, autoYesAlign, lipgloss.Bottom, autoYesStyle.Render(autoYesText))
		b.WriteString(lipgloss.JoinHorizontal(
			lipgloss.Top, title, autoYes))
	}

	b.WriteString("\n")
	b.WriteString("\n")

	// Render the list.
	if l.verticalLayout {
		// Two-column layout for vertical mode
		l.renderTwoColumns(&b)

		// Add scroll indicators if needed
		l.addScrollIndicators(&b)
	} else {
		// Single column layout for horizontal mode
		for i, item := range l.items {
			b.WriteString(l.renderer.Render(item, i+1, i == l.selectedIdx, len(l.repos) > 1))
			if i != len(l.items)-1 {
				b.WriteString("\n\n")
			}
		}
	}
	return lipgloss.Place(l.width, l.height, lipgloss.Left, lipgloss.Top, b.String())
}

// renderTwoColumns renders the list items in a two-column layout for vertical mode
func (l *List) renderTwoColumns(b *strings.Builder) {
	if len(l.items) == 0 {
		return
	}

	// Calculate column width (half the total width minus some padding)
	columnWidth := (l.width - 4) / 2 // Leave 4 characters for spacing between columns

	// Temporarily adjust renderer width for two-column layout
	originalWidth := l.renderer.width
	l.renderer.width = columnWidth

	// Calculate how many rows can fit in the available height
	availableHeight := l.height - 6 // Reserve space for title and padding
	linesPerRow := 3                // Approximate lines per row
	visibleRows := availableHeight / linesPerRow
	if visibleRows < 1 {
		visibleRows = 1
	}

	// Calculate start and end indices based on scroll offset
	startRow := l.scrollOffset
	endRow := startRow + visibleRows
	totalRows := (len(l.items) + 1) / 2

	if endRow > totalRows {
		endRow = totalRows
	}

	// Group items into rows of 2, but only render visible rows
	for row := startRow; row < endRow; row++ {
		i := row * 2 // Convert row to item index
		leftItem := l.items[i]
		leftRendered := l.renderer.Render(leftItem, i+1, i == l.selectedIdx, len(l.repos) > 1)

		var rightRendered string
		if i+1 < len(l.items) {
			rightItem := l.items[i+1]
			rightRendered = l.renderer.Render(rightItem, i+2, i+1 == l.selectedIdx, len(l.repos) > 1)
		} else {
			// If odd number of items, fill right column with empty space
			rightRendered = strings.Repeat(" ", columnWidth)
		}

		// Split rendered items into lines for proper alignment
		leftLines := strings.Split(leftRendered, "\n")
		rightLines := strings.Split(rightRendered, "\n")

		// Ensure both columns have the same number of lines
		maxLines := len(leftLines)
		if len(rightLines) > maxLines {
			maxLines = len(rightLines)
		}

		// Pad shorter column with empty lines
		for len(leftLines) < maxLines {
			leftLines = append(leftLines, strings.Repeat(" ", columnWidth))
		}
		for len(rightLines) < maxLines {
			rightLines = append(rightLines, strings.Repeat(" ", columnWidth))
		}

		// Join columns horizontally line by line
		for j := 0; j < maxLines; j++ {
			// For left column, ensure proper width handling
			leftLine := leftLines[j]
			// Use lipgloss.Place to ensure proper width and handle ANSI sequences
			leftColumn := lipgloss.Place(columnWidth, 1, lipgloss.Left, lipgloss.Top, leftLine)

			b.WriteString(leftColumn)
			b.WriteString("  ") // 2 spaces between columns
			b.WriteString(rightLines[j])
			b.WriteString("\n")
		}

		// Add spacing between rows (except for the last visible row)
		if row < endRow-1 {
			b.WriteString("\n")
		}
	}

	// Restore original renderer width
	l.renderer.width = originalWidth
}

// addScrollIndicators adds visual indicators when there are more items to scroll
func (l *List) addScrollIndicators(b *strings.Builder) {
	if len(l.items) == 0 {
		return
	}

	totalRows := (len(l.items) + 1) / 2
	availableHeight := l.height - 6
	linesPerRow := 3
	visibleRows := availableHeight / linesPerRow
	if visibleRows < 1 {
		visibleRows = 1
	}

	// Check if we need scroll indicators
	hasMoreAbove := l.scrollOffset > 0
	hasMoreBelow := l.scrollOffset+visibleRows < totalRows

	if hasMoreAbove || hasMoreBelow {
		b.WriteString("\n")

		// Create scroll indicator line
		indicator := ""
		if hasMoreAbove {
			indicator += "↑ "
		} else {
			indicator += "  "
		}

		indicator += fmt.Sprintf("(%d/%d rows)", l.scrollOffset+1, totalRows)

		if hasMoreBelow {
			indicator += " ↓"
		}

		// Center the indicator
		indicatorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"}).
			Italic(true)

		centeredIndicator := lipgloss.Place(l.width, 1, lipgloss.Center, lipgloss.Center, indicatorStyle.Render(indicator))
		b.WriteString(centeredIndicator)
	}
}

// ensureSelectedVisible adjusts scroll offset to ensure selected item is visible
func (l *List) ensureSelectedVisible() {
	if len(l.items) == 0 {
		return
	}

	if l.verticalLayout {
		// In two-column layout, calculate which row the selected item is in
		selectedRow := l.selectedIdx / 2

		// Calculate how many rows can fit in the available height
		// Each item takes approximately 2-3 lines (title + branch info) + spacing
		// Reserve space for title (4 lines) and some padding
		availableHeight := l.height - 6 // Reserve space for title and padding
		linesPerRow := 3                // Approximate lines per row (title + branch + spacing)
		visibleRows := availableHeight / linesPerRow
		if visibleRows < 1 {
			visibleRows = 1
		}

		// Adjust scroll offset to keep selected row visible
		if selectedRow < l.scrollOffset {
			// Selected item is above visible area, scroll up
			l.scrollOffset = selectedRow
		} else if selectedRow >= l.scrollOffset+visibleRows {
			// Selected item is below visible area, scroll down
			l.scrollOffset = selectedRow - visibleRows + 1
		}

		// Ensure scroll offset doesn't go negative
		if l.scrollOffset < 0 {
			l.scrollOffset = 0
		}

		// Ensure we don't scroll past the last items
		totalRows := (len(l.items) + 1) / 2 // Round up for odd number of items
		maxScrollOffset := totalRows - visibleRows
		if maxScrollOffset < 0 {
			maxScrollOffset = 0
		}
		if l.scrollOffset > maxScrollOffset {
			l.scrollOffset = maxScrollOffset
		}
	} else {
		// Single column layout - implement similar logic if needed
		// For now, keep existing behavior
		l.scrollOffset = 0
	}
}

// Down selects the next item in the list.
func (l *List) Down() {
	if len(l.items) == 0 {
		return
	}

	if l.verticalLayout {
		// In two-column layout, down moves to the item 2 positions ahead (next row)
		if l.selectedIdx+2 < len(l.items) {
			l.selectedIdx += 2
		} else {
			// If we can't move down 2, try to move to the last item
			l.selectedIdx = len(l.items) - 1
		}
	} else {
		// Single column layout - normal behavior
		if l.selectedIdx < len(l.items)-1 {
			l.selectedIdx++
		}
	}

	l.ensureSelectedVisible()
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

	// Ensure scroll position is still valid after deletion
	l.ensureSelectedVisible()
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

	if l.verticalLayout {
		// In two-column layout, up moves to the item 2 positions back (previous row)
		if l.selectedIdx-2 >= 0 {
			l.selectedIdx -= 2
		} else {
			// If we can't move up 2, move to the first item
			l.selectedIdx = 0
		}
	} else {
		// Single column layout - normal behavior
		if l.selectedIdx > 0 {
			l.selectedIdx--
		}
	}

	l.ensureSelectedVisible()
}

// Left moves to the left column in two-column layout (only in vertical mode)
func (l *List) Left() {
	if len(l.items) == 0 || !l.verticalLayout {
		return
	}

	// If we're in the right column (odd index), move to left column
	if l.selectedIdx%2 == 1 {
		l.selectedIdx--
	}

	l.ensureSelectedVisible()
}

// Right moves to the right column in two-column layout (only in vertical mode)
func (l *List) Right() {
	if len(l.items) == 0 || !l.verticalLayout {
		return
	}

	// If we're in the left column (even index) and there's a right item, move to right column
	if l.selectedIdx%2 == 0 && l.selectedIdx+1 < len(l.items) {
		l.selectedIdx++
	}

	l.ensureSelectedVisible()
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
	l.ensureSelectedVisible()
}

// GetInstances returns all instances in the list
func (l *List) GetInstances() []*session.Instance {
	return l.items
}
