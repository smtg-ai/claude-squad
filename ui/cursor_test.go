package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOverlayCursor(t *testing.T) {
	tests := []struct {
		name    string
		content string
		x, y    int
		want    string
	}{
		{
			name:    "cursor mid line",
			content: "hello",
			x:       1,
			y:       0,
			want:    "h\x1b[7me\x1b[27mllo",
		},
		{
			name:    "cursor at end of line",
			content: "hi",
			x:       2,
			y:       0,
			want:    "hi\x1b[7m \x1b[27m",
		},
		{
			name:    "cursor beyond end pads with spaces",
			content: "hi",
			x:       4,
			y:       0,
			want:    "hi  \x1b[7m \x1b[27m",
		},
		{
			name:    "ansi sequences are skipped",
			content: "\x1b[31mred\x1b[0m",
			x:       0,
			y:       0,
			want:    "\x1b[31m\x1b[7mr\x1b[27med\x1b[0m",
		},
		{
			name:    "cursor on second line",
			content: "one\ntwo",
			x:       0,
			y:       1,
			want:    "one\n\x1b[7mt\x1b[27mwo",
		},
		{
			name:    "cursor row beyond content appends lines",
			content: "one",
			x:       0,
			y:       2,
			want:    "one\n\n\x1b[7m \x1b[27m",
		},
		{
			name:    "wide runes count double",
			content: "日本",
			x:       2,
			y:       0,
			want:    "日\x1b[7m本\x1b[27m",
		},
		{
			name:    "osc hyperlink with ST terminator is zero width",
			content: "\x1b]8;;file:///very/long/path/that/must/not/count\x1b\\dir\x1b]8;;\x1b\\ x",
			x:       4,
			y:       0,
			want:    "\x1b]8;;file:///very/long/path/that/must/not/count\x1b\\dir\x1b]8;;\x1b\\ \x1b[7mx\x1b[27m",
		},
		{
			name:    "osc with BEL terminator is zero width",
			content: "\x1b]0;window title\x07ab",
			x:       1,
			y:       0,
			want:    "\x1b]0;window title\x07a\x1b[7mb\x1b[27m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, OverlayCursor(tt.content, tt.x, tt.y))
		})
	}
}

func TestOverlayRangeInLine(t *testing.T) {
	assert.Equal(t, "h\x1b[7mel\x1b[27mlo", overlayRangeInLine("hello", 1, 3))
	assert.Equal(t, "he\x1b[7mllo\x1b[27m", overlayRangeInLine("hello", 2, -1))
	assert.Equal(t, "\x1b[31m\x1b[7mre\x1b[27md\x1b[0m", overlayRangeInLine("\x1b[31mred\x1b[0m", 0, 2))
	assert.Equal(t, "hello", overlayRangeInLine("hello", 3, 3))
}

func TestPlainColumns(t *testing.T) {
	assert.Equal(t, "ell", plainColumns("hello", 1, 4))
	assert.Equal(t, "red", plainColumns("\x1b[31mred\x1b[0m", 0, -1))
	assert.Equal(t, "llo", plainColumns("he\x1b[7mllo\x1b[27m", 2, -1))
}

func TestPreviewSelectionFinish(t *testing.T) {
	p := NewPreviewPane()
	p.previewState = previewState{text: "first line\nsecond line\nthird line"}

	p.SelectionStart(0, 6)
	p.SelectionDrag(2, 4)
	got := p.SelectionFinish()
	assert.Equal(t, "line\nsecond line\nthird", got)

	// Plain click without drag copies nothing.
	p.SelectionStart(1, 3)
	assert.Equal(t, "", p.SelectionFinish())

	// Reverse drag (bottom-up) normalizes.
	p.SelectionStart(1, 6)
	p.SelectionDrag(0, 6)
	assert.Equal(t, "line\nsecond", p.SelectionFinish())
}

func TestPreviewSelectionDedentsCommonGutter(t *testing.T) {
	p := NewPreviewPane()
	p.previewState = previewState{text: "  Title: something\n  body line\n\n    indented code\n  more body"}

	// Selection starts mid-line on 'T' (col 2), ends at the last line.
	p.SelectionStart(0, 2)
	p.SelectionDrag(4, 10)
	got := p.SelectionFinish()
	assert.Equal(t, "Title: something\nbody line\n\n  indented code\nmore body", got)

	// Content without a common gutter stays untouched.
	p.previewState = previewState{text: "alpha\n  beta"}
	p.SelectionStart(0, 0)
	p.SelectionDrag(1, 5)
	assert.Equal(t, "alpha\n  beta", p.SelectionFinish())
}
