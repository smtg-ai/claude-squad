package ui

import (
	"claude-squad/session"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
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

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

// visibleTitles returns the instance titles which the list actually rendered.
func visibleTitles(t *testing.T, l *List, titles []string) []string {
	t.Helper()
	out := ansiRe.ReplaceAllString(l.String(), "")
	var visible []string
	for i, title := range titles {
		// The renderer trims a space off the prefix once the index reaches two digits.
		prefix := fmt.Sprintf(" %d. ", i+1)
		if i+1 >= 10 {
			prefix = prefix[:len(prefix)-1]
		}
		if strings.Contains(out, fmt.Sprintf("%s %s", prefix, title)) {
			visible = append(visible, title)
		}
	}
	return visible
}

func TestStringFitsInHeight(t *testing.T) {
	titles := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	l := newTestList(titles...)

	// A short terminal (e.g. after increasing the font size) can't fit every instance.
	for _, height := range []int{4, 8, 10, 15, 20, 30, 60} {
		l.SetSize(40, height)
		require.LessOrEqual(t, lipgloss.Height(l.String()), height, "height %d", height)
	}
}

func TestStringKeepsFirstInstanceVisible(t *testing.T) {
	titles := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	l := newTestList(titles...)
	l.SetSize(40, 14)
	l.SetSelectedInstance(0)

	visible := visibleTitles(t, l, titles)
	require.NotEmpty(t, visible)
	require.Equal(t, "a", visible[0])
}

func TestStringScrollsToSelectedInstance(t *testing.T) {
	titles := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	l := newTestList(titles...)
	l.SetSize(40, 14)

	// Walking down the list keeps the selected instance on screen.
	for i := range titles {
		l.SetSelectedInstance(i)
		require.Contains(t, visibleTitles(t, l, titles), titles[i], "selected %s", titles[i])
		require.LessOrEqual(t, lipgloss.Height(l.String()), 14)
	}

	// The last instance is selected, so the list is scrolled to the bottom.
	visible := visibleTitles(t, l, titles)
	require.Equal(t, "h", visible[len(visible)-1])

	// Walking back up scrolls back to the top.
	for i := len(titles) - 1; i >= 0; i-- {
		l.SetSelectedInstance(i)
		require.Contains(t, visibleTitles(t, l, titles), titles[i], "selected %s", titles[i])
	}
	require.Equal(t, "a", visibleTitles(t, l, titles)[0])
}

func TestStringTinyHeight(t *testing.T) {
	titles := []string{"a", "b", "c"}
	l := newTestList(titles...)
	l.SetSelectedInstance(2)

	// Not even the title plus one instance fits in these. Nothing useful can be shown,
	// but the list must still stay inside the height it was given.
	for _, height := range []int{1, 2, 3, 5} {
		l.SetSize(40, height)
		require.Equal(t, height, lipgloss.Height(l.String()), "height %d", height)
	}
}

func TestStringUnboundedHeightRendersEverything(t *testing.T) {
	titles := []string{"a", "b", "c"}
	l := newTestList(titles...)
	l.SetSize(40, 0)

	require.Equal(t, titles, visibleTitles(t, l, titles))
}

// titleLines returns the line numbers on which instance titles were rendered.
func titleLines(t *testing.T, l *List, titles []string) []int {
	t.Helper()
	out := ansiRe.ReplaceAllString(l.String(), "")
	var lines []int
	for i, line := range strings.Split(out, "\n") {
		for j, title := range titles {
			prefix := fmt.Sprintf(" %d. ", j+1)
			if j+1 >= 10 {
				prefix = prefix[:len(prefix)-1]
			}
			if strings.Contains(line, fmt.Sprintf("%s %s", prefix, title)) {
				lines = append(lines, i)
			}
		}
	}
	return lines
}

func TestRenderCompactHalvesTheHeight(t *testing.T) {
	l := newTestList("a")
	l.renderer.setWidth(40)

	roomy := l.renderer.Render(l.items[0], 1, false, false, false)
	compact := l.renderer.Render(l.items[0], 1, false, false, true)

	// The roomy layout keeps a blank line above and below each instance.
	require.Equal(t, 4, lipgloss.Height(roomy))
	require.Equal(t, 2, lipgloss.Height(compact))
}

func TestStringCompactsToFitEveryInstance(t *testing.T) {
	titles := []string{"alpha", "bravo", "charlie", "delta", "echo",
		"foxtrot", "golf", "hotel", "india", "juliett"}
	l := newTestList(titles...)
	// What a 40 row terminal leaves for the list. The roomy layout needs 53 rows for
	// ten instances, so the list falls back to the compact one rather than hiding any.
	l.SetSize(40, 34)

	require.Equal(t, titles, visibleTitles(t, l, titles))
	require.LessOrEqual(t, lipgloss.Height(l.String()), 34)
	lines := titleLines(t, l, titles)
	require.Equal(t, 3, lines[1]-lines[0], "compact instances are 3 rows apart")
}

func TestStringKeepsRoomyLayoutWhenEverythingFits(t *testing.T) {
	titles := []string{"alpha", "bravo", "charlie", "delta", "echo",
		"foxtrot", "golf", "hotel", "india", "juliett"}
	l := newTestList(titles...)
	l.SetSize(40, 60)

	require.Equal(t, titles, visibleTitles(t, l, titles))
	lines := titleLines(t, l, titles)
	require.Equal(t, 5, lines[1]-lines[0], "roomy instances are 5 rows apart")
}
