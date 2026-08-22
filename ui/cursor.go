package ui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// OverlayCursor renders the cursor cell at (x, y) into captured pane content by
// wrapping it in reverse video. x and y are 0-based screen coordinates as
// reported by tmux; the content must be captured without joining wrapped lines
// so that line indices match screen rows.
func OverlayCursor(content string, x, y int) string {
	lines := strings.Split(content, "\n")
	for len(lines) <= y {
		lines = append(lines, "")
	}
	lines[y] = overlayCursorInLine(lines[y], x)
	return strings.Join(lines, "\n")
}

// skipEscape returns the index just past the escape sequence starting at i
// (runes[i] must be ESC). Handles CSI (ESC [ ... final byte), string sequences
// like OSC/DCS/APC (terminated by BEL or ST), and two-byte escapes.
func skipEscape(runes []rune, i int) int {
	j := i + 1
	if j >= len(runes) {
		return j
	}
	switch runes[j] {
	case '[':
		j++
		for j < len(runes) && (runes[j] < 0x40 || runes[j] > 0x7e) {
			j++
		}
		if j < len(runes) {
			j++
		}
	case ']', 'P', '^', '_':
		// String sequence (e.g. OSC 8 hyperlinks): runs until BEL or ST (ESC \).
		j++
		for j < len(runes) {
			if runes[j] == 0x07 {
				j++
				break
			}
			if runes[j] == 0x1b && j+1 < len(runes) && runes[j+1] == '\\' {
				j += 2
				break
			}
			j++
		}
	default:
		j++
	}
	return j
}

// overlayCursorInLine wraps the rune at visible column col in reverse video,
// skipping ANSI escape sequences. If the line is shorter than col, it is padded
// and a reversed space is appended.
func overlayCursorInLine(line string, col int) string {
	var b strings.Builder
	runes := []rune(line)
	visible := 0
	i := 0
	for i < len(runes) {
		r := runes[i]
		if r == 0x1b {
			j := skipEscape(runes, i)
			b.WriteString(string(runes[i:j]))
			i = j
			continue
		}
		if visible == col {
			b.WriteString("\x1b[7m")
			b.WriteRune(r)
			b.WriteString("\x1b[27m")
			visible += runewidth.RuneWidth(r)
			i++
			continue
		}
		b.WriteRune(r)
		visible += runewidth.RuneWidth(r)
		i++
	}
	if visible <= col {
		b.WriteString(strings.Repeat(" ", col-visible))
		b.WriteString("\x1b[7m \x1b[27m")
	}
	return b.String()
}

// overlayRangeInLine wraps the visible columns [from, to) in reverse video,
// skipping ANSI escape sequences. to < 0 means to end of line.
func overlayRangeInLine(line string, from, to int) string {
	if to >= 0 && to <= from {
		return line
	}
	var b strings.Builder
	runes := []rune(line)
	visible := 0
	i := 0
	inSelection := false
	for i < len(runes) {
		r := runes[i]
		if r == 0x1b {
			j := skipEscape(runes, i)
			b.WriteString(string(runes[i:j]))
			i = j
			continue
		}
		if !inSelection && visible >= from && (to < 0 || visible < to) {
			b.WriteString("\x1b[7m")
			inSelection = true
		}
		if inSelection && to >= 0 && visible >= to {
			b.WriteString("\x1b[27m")
			inSelection = false
		}
		b.WriteRune(r)
		visible += runewidth.RuneWidth(r)
		i++
	}
	if inSelection {
		b.WriteString("\x1b[27m")
	}
	return b.String()
}

// plainColumns returns the plain text (escape sequences stripped) of the
// visible columns [from, to) of line. to < 0 means to end of line.
func plainColumns(line string, from, to int) string {
	var b strings.Builder
	runes := []rune(line)
	visible := 0
	i := 0
	for i < len(runes) {
		r := runes[i]
		if r == 0x1b {
			i = skipEscape(runes, i)
			continue
		}
		if visible >= from && (to < 0 || visible < to) {
			b.WriteRune(r)
		}
		visible += runewidth.RuneWidth(r)
		i++
	}
	return b.String()
}
