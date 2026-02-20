package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ByteMirror/hivemind/log"
	"github.com/ByteMirror/hivemind/session"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// renderOpt carries optional flags for instance rendering.
type renderOpt struct {
	expanded bool
	isChild  bool
	isLast   bool // last child in the group
}

// InstanceRenderer handles rendering of session.Instance objects
type InstanceRenderer struct {
	spinner *spinner.Model
	width   int
}

func (r *InstanceRenderer) setWidth(width int) {
	r.width = width
}

func (r *InstanceRenderer) Render(i *session.Instance, selected bool, focused bool, hasMultipleRepos bool, rowIndex int, opts ...renderOpt) string {
	var expanded, isChild, isLast bool
	for _, o := range opts {
		if o.expanded {
			expanded = true
		}
		if o.isChild {
			isChild = true
		}
		if o.isLast {
			isLast = true
		}
	}
	prefix := " "
	titleS := selectedTitleStyle
	descS := selectedDescStyle

	if isChild {
		if isLast {
			prefix = " └─"
		} else {
			prefix = " ├─"
		}
		if selected {
			titleS = childSelectedTitleStyle
			descS = childSelectedDescStyle
		} else if !focused && selected {
			titleS = activeTitleStyle
			descS = activeDescStyle
		} else {
			titleS = childTitleStyle
			descS = childDescStyle
		}
	} else if selected && !focused {
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

	// Add skip-permissions and auto-accept indicators
	skipPermsIndicator := ""
	if i.SkipPermissions {
		skipPermsIndicator = " \uf132"
	}
	if i.AutoYes {
		skipPermsIndicator += " \uf00c"
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

	// For Loading status, show a progress bar instead of normal branch/diff info
	if i.Status == session.Loading {
		stage := i.LoadingStage
		total := i.LoadingTotal
		if total == 0 {
			total = 7
		}

		barWidth := r.width - len(prefix) - 3 // prefix + padding
		if barWidth < 10 {
			barWidth = 10
		}
		if barWidth > 30 {
			barWidth = 30
		}
		filled := (stage * barWidth) / total
		if filled > barWidth {
			filled = barWidth
		}
		bar := GradientBar(barWidth, filled, "#F0A868", "#7EC8D8")

		stepText := i.LoadingMessage
		if stepText == "" {
			stepText = "Starting..."
		}
		// Truncate step text if needed
		maxStepWidth := r.width - len(prefix) - barWidth - 4
		if maxStepWidth > 0 && runewidth.StringWidth(stepText) > maxStepWidth {
			stepText = runewidth.Truncate(stepText, maxStepWidth, "...")
		}

		loadingLine := fmt.Sprintf("%s %s %s", strings.Repeat(" ", len(prefix)), bar, loadingStepStyle.Render(stepText))

		lines := []string{
			title,
			descS.Width(r.width).Render(loadingLine),
		}
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

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
		diffWidth += 1 // account for comma separator
	}

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

	branchLine := fmt.Sprintf("%s %s-%s%s%s", strings.Repeat(" ", len(prefix)), branchIcon, branch, renderedActivity, spaces)

	// Build third line: resource usage (left slot) + role icon (middle) + diff stats (right slot).
	// Each segment gets the row background explicitly so ANSI resets between
	// styled spans don't create black gaps on highlighted rows.
	var thirdLine string
	hasResource := i.Status != session.Paused && i.MemMB > 0
	hasRole := i.Role != ""
	if hasResource || diff != "" || hasRole {
		bg := descS.GetBackground()
		bgStyle := lipgloss.NewStyle().Background(bg)

		// Left slot: resource usage — measure from raw text only.
		var leftRendered string
		var leftWidth int
		if hasResource {
			cpuText := fmt.Sprintf("\U000f0d46 %.0f%%", i.CPUPercent)
			memText := fmt.Sprintf("\uefc5 %.0fM", i.MemMB)
			resourceRaw := fmt.Sprintf("%s %s  %s", strings.Repeat(" ", len(prefix)), cpuText, memText)
			leftWidth = runewidth.StringWidth(resourceRaw)
			leftRendered = resourceStyle.Background(bg).Render(resourceRaw)

			// Sub-agent count indicator
			if i.SubAgentCount > 0 {
				subIcon := fmt.Sprintf("  \u2e0b %d", i.SubAgentCount) // ⸋ N
				leftWidth += runewidth.StringWidth(subIcon)
				leftRendered += subAgentCountStyle.Background(bg).Render(subIcon)
			}
		}

		// Brain-spawned child count indicator on parent rows.
		if i.BrainChildCount > 0 {
			expandHint := "+"
			if expanded {
				expandHint = "-"
			}
			childText := fmt.Sprintf("  %s\uf0c0 %d", expandHint, i.BrainChildCount)
			leftWidth += runewidth.StringWidth(childText)
			leftRendered += subAgentCountStyle.Background(bg).Render(childText)
		}

		// Role icon segment.
		var roleRendered string
		var roleWidth int
		if hasRole {
			icon, ok := roleIcons[i.Role]
			if !ok {
				icon = roleIconFallback
			}
			roleText := fmt.Sprintf("  %s %s", icon, i.Role)
			roleWidth = runewidth.StringWidth(roleText)
			roleRendered = roleIconStyle.Background(bg).Render(roleText)
		}

		// Content width accounts for descS horizontal padding (1 left + 1 right).
		// Without this, the content overflows the content area and wraps.
		contentWidth := r.width - 2

		// If left + role + right exceed the content width, truncate the left slot.
		if leftWidth+roleWidth+diffWidth > contentWidth && leftWidth > 0 {
			maxLeft := contentWidth - roleWidth - diffWidth
			if maxLeft < 4 {
				maxLeft = 4
			}
			// Rebuild resource text truncated to maxLeft.
			cpuText := fmt.Sprintf("\U000f0d46 %.0f%%", i.CPUPercent)
			memText := fmt.Sprintf("\uefc5 %.0fM", i.MemMB)
			resourceRaw := fmt.Sprintf("%s %s  %s", strings.Repeat(" ", len(prefix)), cpuText, memText)
			resourceRaw = runewidth.Truncate(resourceRaw, maxLeft, "...")
			leftWidth = runewidth.StringWidth(resourceRaw)
			leftRendered = resourceStyle.Background(bg).Render(resourceRaw)
		}

		// Right slot: diff stats (already styled with bg).
		// Gap fills remaining space with the row's background.
		gap := contentWidth - leftWidth - roleWidth - diffWidth
		if gap < 0 {
			gap = 0
		}

		thirdLine = leftRendered + roleRendered + bgStyle.Render(strings.Repeat(" ", gap)) + diff
	}

	// join title, branch, and optionally third line
	lines := []string{
		title,
		descS.Width(r.width).Render(branchLine),
	}
	if thirdLine != "" {
		lines = append(lines, descS.Width(r.width).Render(thirdLine))
	}

	// Show expanded sub-agent rows when toggled
	if expanded && i.SubAgentCount > 0 {
		for idx, sa := range i.SubAgents {
			resource := fmt.Sprintf("%.0f%% CPU, %.0fMB", sa.CPU, sa.MemMB)
			saLine := fmt.Sprintf("     └ Sub-agent %d: %s  (%s)", idx+1, sa.Activity, resource)
			lines = append(lines, subAgentRowStyle.Width(r.width).Render(saLine))
		}
	}

	text := lipgloss.JoinVertical(lipgloss.Left, lines...)

	return text
}

func (l *List) String() string {
	const autoYesText = " auto-yes "

	var b strings.Builder
	b.WriteString("\n")

	// Write filter tabs
	titleWidth := l.width

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

	sortLabel := sortDropdownStyle.Render("3 \uf0dc " + sortModeLabels[l.sortMode])

	if !l.autoyes {
		left := tabs
		right := sortLabel
		gap := titleWidth - runewidth.StringWidth(left) - runewidth.StringWidth(right)
		if gap < 1 {
			gap = 1
		}
		b.WriteString(left + strings.Repeat(" ", gap) + right)
	} else {
		left := tabs + " " + sortLabel
		autoYes := autoYesStyle.Render(autoYesText)
		gap := titleWidth - runewidth.StringWidth(left) - runewidth.StringWidth(autoYes)
		if gap < 1 {
			gap = 1
		}
		b.WriteString(left + strings.Repeat(" ", gap) + autoYes)
	}

	b.WriteString("\n")
	b.WriteString("\n")

	// Render the list.
	for i, item := range l.items {
		var opts []renderOpt
		if l.expanded[item.Title] || l.childExpanded[item.Title] {
			opts = append(opts, renderOpt{expanded: true})
		}
		if item.ParentTitle != "" {
			isLastChild := (i == len(l.items)-1) || l.items[i+1].ParentTitle != item.ParentTitle
			opts = append(opts, renderOpt{isChild: true, isLast: isLastChild})
		}
		b.WriteString(l.renderer.Render(item, i == l.selectedIdx, l.focused, len(l.repos) > 1, i, opts...))
		if i != len(l.items)-1 {
			b.WriteString("\n\n")
		}
	}
	return lipgloss.Place(l.width, l.height, lipgloss.Left, lipgloss.Top, b.String())
}

// itemHeight returns the rendered row count for an instance entry.
// Title style has Padding(1,0) top, desc style has Padding(0,1) bottom.
// 2-line item (title+branch) = 4 rows; 3-line (with resource) = 6 rows.
// When expanded, each sub-agent adds 1 row.
func (l *List) itemHeight(idx int) int {
	inst := l.items[idx]
	base := 4 // title (1 pad top + 1 content) + branch (1 content + 1 pad bottom)
	hasResource := inst.Status != session.Paused && inst.MemMB > 0
	stat := inst.GetDiffStats()
	hasDiff := stat != nil && stat.Error == nil && !stat.IsEmpty()
	hasRole := inst.Role != ""
	if hasResource || hasDiff || hasRole {
		base += 2 // third line (1 content + 1 pad bottom)
	}
	if l.expanded[inst.Title] && inst.SubAgentCount > 0 {
		base += inst.SubAgentCount // one row per sub-agent
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
