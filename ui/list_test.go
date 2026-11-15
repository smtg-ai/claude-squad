package ui

import (
	"claude-squad/session"
	"claude-squad/session/ci"
	"claude-squad/session/git"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"
)

func newTestList(titles ...string) *List {
	s := spinner.New()
	l := NewList(&s, false)
	for _, t := range titles {
		inst, _ := session.NewInstance(session.InstanceOptions{
			Title:   t,
			Path:    ".",
			Program: "echo",
		})
		l.AddInstance(inst)
	}
	return l
}

func TestMoveUp(t *testing.T) {
	l := newTestList("a", "b", "c")
	l.SetSelectedInstance(1) // select "b"

	moved := l.MoveUp()
	require.True(t, moved)
	require.Equal(t, 0, l.selectedIdx)
	require.Equal(t, "b", l.items[0].Title)
	require.Equal(t, "a", l.items[1].Title)
	require.Equal(t, "c", l.items[2].Title)
}

func TestMoveUp_AtTop(t *testing.T) {
	l := newTestList("a", "b", "c")
	l.SetSelectedInstance(0)

	moved := l.MoveUp()
	require.False(t, moved)
	require.Equal(t, 0, l.selectedIdx)
	require.Equal(t, "a", l.items[0].Title)
}

func TestMoveDown(t *testing.T) {
	l := newTestList("a", "b", "c")
	l.SetSelectedInstance(1) // select "b"

	moved := l.MoveDown()
	require.True(t, moved)
	require.Equal(t, 2, l.selectedIdx)
	require.Equal(t, "a", l.items[0].Title)
	require.Equal(t, "c", l.items[1].Title)
	require.Equal(t, "b", l.items[2].Title)
}

func TestMoveDown_AtBottom(t *testing.T) {
	l := newTestList("a", "b", "c")
	l.SetSelectedInstance(2)

	moved := l.MoveDown()
	require.False(t, moved)
	require.Equal(t, 2, l.selectedIdx)
	require.Equal(t, "c", l.items[2].Title)
}

func TestMoveWithSingleItem(t *testing.T) {
	l := newTestList("only")
	l.SetSelectedInstance(0)

	require.False(t, l.MoveUp())
	require.False(t, l.MoveDown())
}

// newTestRenderer builds a renderer with a known width plus one instance whose
// CI verdict the caller controls.
func newTestRenderer(t *testing.T, width int, branch string, status ci.Status) (*InstanceRenderer, *session.Instance) {
	t.Helper()
	s := spinner.New()
	r := &InstanceRenderer{spinner: &s}
	r.setWidth(width)

	inst, err := session.NewInstance(session.InstanceOptions{Title: "session", Path: ".", Program: "echo"})
	require.NoError(t, err)
	inst.Branch = branch
	inst.SetCIStatus(status)
	return r, inst
}

// branchLineOf returns the rendered line carrying the branch name.
func branchLineOf(t *testing.T, out string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, branchIcon) {
			return line
		}
	}
	t.Fatalf("no branch line in rendered output %q", out)
	return ""
}

func TestRenderCIBadge(t *testing.T) {
	tests := []struct {
		name   string
		status ci.Status
		want   string
	}{
		{
			name:   "failure shows the glyph and PR number",
			status: ci.Status{State: ci.StateFailure, PRNumber: 4420, Failed: 1},
			want:   ciFailureIcon + " #4420",
		},
		{
			name:   "success shows the glyph and PR number",
			status: ci.Status{State: ci.StateSuccess, PRNumber: 12, Passed: 7},
			want:   ciSuccessIcon + " #12",
		},
		{
			name:   "still running shows the pending glyph",
			status: ci.Status{State: ci.StatePending, PRNumber: 99, Pending: 3},
			want:   ciPendingIcon + " #99",
		},
		{
			name:   "a branch with no pull request says so",
			status: ci.Status{State: ci.StateNoPR},
			want:   ciNoPRIcon + " no PR",
		},
		{
			// Not the PR number: nothing more is going to happen to it, and a merged
			// PR is all-green, so a bare tick would read as "open and passing".
			name:   "a merged pull request says merged, not passed",
			status: ci.Status{State: ci.StateMerged, PRNumber: 4418},
			want:   ciMergedIcon + " merged",
		},
		{
			name:   "a closed pull request says closed",
			status: ci.Status{State: ci.StateClosed, PRNumber: 4418},
			want:   ciClosedIcon + " closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, inst := newTestRenderer(t, 80, "feature", tt.status)
			require.Contains(t, branchLineOf(t, r.Render(inst, 1, false, false)), tt.want)
		})
	}
}

// StateUnknown covers three cases that must all look identical: the feature is
// disabled, gh is unavailable, and the first lookup has not come back yet. None
// of them may draw a badge, or the list would claim a verdict it does not have.
func TestRenderNoBadgeWithoutAVerdict(t *testing.T) {
	r, inst := newTestRenderer(t, 80, "feature", ci.Status{})
	line := branchLineOf(t, r.Render(inst, 1, false, false))

	for _, icon := range []string{ciSuccessIcon, ciFailureIcon, ciPendingIcon, ciMergedIcon, ciClosedIcon, ciNoPRIcon} {
		require.NotContains(t, line, icon)
	}
}

// The badge is reserved out of the line's width budget before the branch name is
// truncated, so adding one must not change the rendered width at all -- an
// overlong branch has to give way instead. Regression guard for the width
// accounting in Render.
//
// Compared against the no-badge baseline rather than against r.width, because
// the branch line already exceeds r.width by 2 upstream: listDescStyle carries
// Padding(0, 1, 1, 1) and remainingWidth never subtracts it. That is not this
// feature's bug to encode as correct, so the assertion is "the badge costs
// nothing", which is what this change is responsible for.
func TestRenderCIBadgeDoesNotChangeLineWidth(t *testing.T) {
	const longBranch = "a-very-long-branch-name-that-cannot-possibly-fit-on-one-line"

	badged := []ci.Status{
		{State: ci.StateNoPR},
		{State: ci.StateMerged, PRNumber: 4418},
		{State: ci.StateClosed, PRNumber: 4418},
		{State: ci.StateFailure, PRNumber: 4420},
		{State: ci.StatePending, PRNumber: 7},
		{State: ci.StateSuccess, PRNumber: 123456},
	}

	for _, width := range []int{20, 30, 40, 60, 80, 120} {
		base, baseInst := newTestRenderer(t, width, longBranch, ci.Status{})
		baseWidth := lipgloss.Width(branchLineOf(t, base.Render(baseInst, 1, false, false)))

		for _, status := range badged {
			r, inst := newTestRenderer(t, width, longBranch, status)
			line := branchLineOf(t, r.Render(inst, 1, false, false))
			require.Equal(t, baseWidth, lipgloss.Width(line),
				"width %d, state %v: badge changed the line width", width, status.State)
			require.Contains(t, line, "-"+runewidth.Truncate(longBranch, 3, ""),
				"width %d, state %v: branch name vanished entirely", width, status.State)
		}
	}
}

// A bare space sitting after an already-styled span renders with the terminal's
// own background rather than the row's, which showed up as a black block in the
// middle of the highlighted row. Guard the whole class rather than the one space:
// on a selected row, no reset may be immediately followed by an unstyled space.
func TestRenderSelectedRowHasNoUnstyledGaps(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	for _, status := range []ci.Status{
		{State: ci.StateNoPR},
		{State: ci.StateMerged, PRNumber: 4418},
		{State: ci.StateClosed, PRNumber: 4418},
		{State: ci.StatePending, PRNumber: 4420},
		{State: ci.StateFailure, PRNumber: 4420},
		{State: ci.StateSuccess, PRNumber: 4420},
	} {
		r, inst := newTestRenderer(t, 80, "task-5636-vis-6", status)
		inst.SetDiffStats(&git.DiffStats{Added: 9997, Removed: 192})

		line := branchLineOf(t, r.Render(inst, 1, true, false))
		require.NotContains(t, line, "\x1b[0m ",
			"state %v: a style reset is followed by an unstyled space, which renders as a gap in the row highlight",
			status.State)
	}
}
