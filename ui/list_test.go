package ui

import (
	"claude-squad/log"
	"claude-squad/session"
	"os"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	log.Initialize(false)
	defer log.Close()
	os.Exit(m.Run())
}

// newTestList creates a list with n mock items and a given height.
// Each item renders to 4 lines (title padding + title + desc + desc padding).
func newTestList(n int, height int) *List {
	s := spinner.New()
	l := NewList(&s, false)
	l.SetSize(60, height)

	for i := 0; i < n; i++ {
		inst := &session.Instance{Title: "test"}
		l.items = append(l.items, inst)
	}
	return l
}

// visibleItemCount renders the list and counts how many items appear.
func visibleItemCount(l *List) int {
	rendered := renderItems(l)
	if len(rendered) == 0 {
		return 0
	}

	headerLines := 4
	availableLines := l.height - headerLines
	if availableLines < 1 {
		return 0
	}

	count := 0
	linesUsed := 0
	for i := l.scrollOffset; i < len(rendered); i++ {
		needed := rendered[i].lines
		if i > l.scrollOffset {
			needed += 1
		}
		if linesUsed+needed > availableLines && i > l.scrollOffset {
			break
		}
		linesUsed += needed
		count++
	}
	return count
}

// renderItems mirrors the rendering logic to get item line counts.
func renderItems(l *List) []listRenderedItem {
	rendered := make([]listRenderedItem, len(l.items))
	for i, item := range l.items {
		text := l.renderer.Render(item, i+1, i == l.selectedIdx, len(l.repos) > 1)
		lineCount := len(splitLines(text))
		rendered[i] = listRenderedItem{text: text, lines: lineCount}
	}
	return rendered
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

func TestScrollDownAndUp(t *testing.T) {
	// Height 30 should fit about 5 items (4 lines each + 1 separator).
	l := newTestList(10, 30)

	// Initially at top.
	assert.Equal(t, 0, l.scrollOffset)
	assert.Equal(t, 0, l.selectedIdx)

	// Navigate down past visible area.
	for i := 0; i < 9; i++ {
		l.Down()
	}
	assert.Equal(t, 9, l.selectedIdx)

	// Scroll offset should have moved to keep selection visible.
	_ = l.String() // triggers adjustScrollOffset
	assert.Greater(t, l.scrollOffset, 0, "scroll offset should increase when navigating down")

	// Navigate back to top.
	for i := 0; i < 9; i++ {
		l.Up()
	}
	assert.Equal(t, 0, l.selectedIdx)
	_ = l.String()
	assert.Equal(t, 0, l.scrollOffset, "scroll offset should return to 0 when at top")
}

func TestScrollConsistentItemCount(t *testing.T) {
	// Use enough items and height so scrolling is needed.
	l := newTestList(20, 50)

	// Scroll to bottom.
	for i := 0; i < 19; i++ {
		l.Down()
	}
	_ = l.String()
	countAtBottom := visibleItemCount(l)

	// Scroll back to top.
	for i := 0; i < 19; i++ {
		l.Up()
	}
	_ = l.String()
	countAtTop := visibleItemCount(l)

	assert.Equal(t, countAtTop, countAtBottom,
		"visible item count should be the same at top and bottom")
}

func TestScrollAfterKillLastItem(t *testing.T) {
	l := newTestList(15, 50)

	// Scroll to the last item.
	for i := 0; i < 14; i++ {
		l.Down()
	}
	_ = l.String()
	assert.Equal(t, 14, l.selectedIdx)
	offsetBefore := l.scrollOffset

	// Kill the last item — should not leave empty space at bottom.
	// We can't call Kill() directly (it calls instance.Kill()), so simulate it.
	l.items = append(l.items[:l.selectedIdx], l.items[l.selectedIdx+1:]...)
	l.Up()
	if l.scrollOffset >= len(l.items) {
		l.scrollOffset = len(l.items) - 1
	}
	l.clampScrollOffset()

	_ = l.String()
	assert.Equal(t, 13, l.selectedIdx)
	assert.LessOrEqual(t, l.scrollOffset, offsetBefore,
		"scroll offset should decrease after deleting the last item")

	// Verify no excessive empty space: visible items should reach the end of the list.
	rendered := renderItems(l)
	headerLines := 4
	availableLines := l.height - headerLines
	linesUsed := 0
	lastVisible := l.scrollOffset
	for i := l.scrollOffset; i < len(rendered); i++ {
		needed := rendered[i].lines
		if i > l.scrollOffset {
			needed += 1
		}
		if linesUsed+needed > availableLines && i > l.scrollOffset {
			break
		}
		lastVisible = i
		linesUsed += needed
	}
	assert.Equal(t, len(l.items)-1, lastVisible,
		"last item should be visible after deleting from bottom")
}

func TestScrollOffsetBounds(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		l := newTestList(0, 50)
		l.Down()
		l.Up()
		assert.Equal(t, 0, l.scrollOffset)
		assert.Equal(t, 0, l.selectedIdx)
	})

	t.Run("items fit without scrolling", func(t *testing.T) {
		// 3 items in a large area — no scroll needed.
		l := newTestList(3, 80)
		l.Down()
		l.Down()
		_ = l.String()
		assert.Equal(t, 0, l.scrollOffset, "should not scroll when all items fit")
	})

	t.Run("scroll offset clamps after multiple deletions", func(t *testing.T) {
		l := newTestList(10, 30)
		// Navigate to the end.
		for i := 0; i < 9; i++ {
			l.Down()
		}
		_ = l.String()

		// Delete items from the end.
		for len(l.items) > 3 {
			l.items = l.items[:len(l.items)-1]
			if l.selectedIdx >= len(l.items) {
				l.selectedIdx = len(l.items) - 1
			}
			if l.scrollOffset >= len(l.items) {
				l.scrollOffset = len(l.items) - 1
			}
			l.clampScrollOffset()
		}
		_ = l.String()

		assert.Equal(t, 0, l.scrollOffset,
			"scroll offset should be 0 when remaining items fit on screen")
	})
}
