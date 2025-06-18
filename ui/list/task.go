package list

import (
	"fmt"
	"strings"

	"claude-squad/instance/task"

	"github.com/charmbracelet/lipgloss"
)

type InstanceStatus int

const (
	InstanceRunning InstanceStatus = iota
	InstanceReady
	InstancePaused
	InstanceLoading
)

// InstanceRenderData contains the data needed to render an instance
type InstanceRenderData struct {
	Title     string
	Branch    string
	Status    InstanceStatus
	DiffStats *DiffStats
	IsStarted bool
	RepoName  string
}

type DiffStats struct {
	Added   int
	Removed int
	Error   error
}

func (d *DiffStats) IsEmpty() bool {
	return d.Added == 0 && d.Removed == 0
}

func (l *List) RenderTask(t *task.Task, idx int, selected bool) string {
	prefix := fmt.Sprintf("%s %d. ", "", idx)
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
	switch t.Status {
	case task.Running:
		join = fmt.Sprintf("%s ", l.spinner.View())
	case task.Ready:
		join = readyStyle.Render(readyIcon)
	case task.Paused:
		join = pausedStyle.Render(pausedIcon)
	default:
	}

	// Cut the title if it's too long
	titleText := t.Title
	listWidth := int(float64(l.width) * 0.4) // List takes about 40% of width
	widthAvail := listWidth - 3 - len(prefix) - 1
	if widthAvail > 0 && widthAvail < len(titleText) && len(titleText) >= widthAvail-3 {
		titleText = titleText[:widthAvail-3] + "..."
	}
	title := titleS.Render(lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.Place(listWidth-3, 1, lipgloss.Left, lipgloss.Center, fmt.Sprintf("%s %s", prefix, titleText)),
		" ",
		join,
	))

	stat := t.DiffStats

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

	remainingWidth := listWidth
	remainingWidth -= len(prefix)
	remainingWidth -= len(branchIcon)

	diffWidth := len(addedDiff) + len(removedDiff)
	if diffWidth > 0 {
		diffWidth += 1
	}

	// Use fixed width for diff stats to avoid layout issues
	remainingWidth -= diffWidth

	branch := t.Branch
	if t.IsRunning() {
		repoName, _ := t.RepoName()
		if repoName != "" {
			branch += fmt.Sprintf(" (%s)", repoName)
		}
	}
	// Don't show branch if there's no space for it. Or show ellipsis if it's too long.
	if remainingWidth < 0 {
		branch = ""
	} else if remainingWidth < len(branch) {
		if remainingWidth < 3 {
			branch = ""
		} else {
			// We know the remainingWidth is at least 4 and branch is longer than that, so this is safe.
			branch = branch[:remainingWidth-3] + "..."
		}
	}
	remainingWidth -= len(branch)

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
