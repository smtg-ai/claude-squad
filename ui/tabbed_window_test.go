package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newCellTestWindow(previewWidth, previewHeight int) *TabbedWindow {
	w := &TabbedWindow{preview: NewPreviewPane()}
	w.preview.SetSize(previewWidth, previewHeight)
	// activeTab defaults to PreviewTab (0).
	return w
}

func TestPreviewCellAt(t *testing.T) {
	w := newCellTestWindow(20, 5) // content top row is previewContentTop (4)

	// Top-left content cell: x=1 (past the left border), y=previewContentTop.
	row, col, ok := w.PreviewCellAt(1, previewContentTop)
	assert.True(t, ok)
	assert.Equal(t, 0, row)
	assert.Equal(t, 0, col)

	// Bottom-right content cell.
	row, col, ok = w.PreviewCellAt(20, previewContentTop+4)
	assert.True(t, ok)
	assert.Equal(t, 4, row)
	assert.Equal(t, 19, col)

	// Out of bounds: on the border, above the content, past the width/height.
	for _, tc := range []struct{ x, y int }{
		{0, previewContentTop},     // left border column
		{1, previewContentTop - 1}, // tab row above content
		{21, previewContentTop},    // past the right edge
		{1, previewContentTop + 5}, // past the bottom edge
	} {
		_, _, ok := w.PreviewCellAt(tc.x, tc.y)
		assert.Falsef(t, ok, "expected out-of-bounds at (%d,%d)", tc.x, tc.y)
	}

	// Not the preview tab: never maps.
	w.activeTab = DiffTab
	_, _, ok = w.PreviewCellAt(1, previewContentTop)
	assert.False(t, ok)
}

func TestPreviewCellClamped(t *testing.T) {
	w := newCellTestWindow(20, 5)

	// Above/left of the pane clamps to the first cell.
	row, col := w.PreviewCellClamped(0, 0)
	assert.Equal(t, 0, row)
	assert.Equal(t, 0, col)

	// Far below/right clamps to the last cell.
	row, col = w.PreviewCellClamped(1000, 1000)
	assert.Equal(t, 4, row)
	assert.Equal(t, 19, col)
}

func TestFocusTogglesCursorAndBorder(t *testing.T) {
	w := &TabbedWindow{preview: NewPreviewPane()}

	assert.False(t, w.focused)
	assert.False(t, w.preview.showCursor)

	w.SetFocused(true)
	w.SetShowCursor(true)
	assert.True(t, w.focused)
	assert.True(t, w.preview.showCursor)

	w.SetFocused(false)
	w.SetShowCursor(false)
	assert.False(t, w.focused)
	assert.False(t, w.preview.showCursor)
}
