package ui

import "strings"

// paneSelection is a squad-side mouse selection over a pane's captured
// content: the terminal cannot restrict native selection to one pane, but
// squad receives mouse coordinates and knows its layout, so it selects within
// the pane content itself and copies on release. Coordinates are content
// cells (row, column) of the owning pane.
type paneSelection struct {
	active             bool
	startRow, startCol int
	endRow, endCol     int
}

// Start begins a selection at the given content cell.
func (s *paneSelection) Start(row, col int) {
	s.active = true
	s.startRow, s.startCol = row, col
	s.endRow, s.endCol = row, col
}

// Drag extends the selection to the given content cell.
func (s *paneSelection) Drag(row, col int) {
	if !s.active {
		return
	}
	s.endRow, s.endCol = row, col
}

// Active reports whether a drag selection is in progress.
func (s *paneSelection) Active() bool {
	return s.active
}

// Cancel drops an in-progress selection without copying. Used when a stray
// selection would otherwise keep the pane frozen, e.g. after a mouse release
// that never reached the app.
func (s *paneSelection) Cancel() {
	s.active = false
}

// bounds returns the normalized selection (start before end).
func (s *paneSelection) bounds() (r1, c1, r2, c2 int) {
	r1, c1, r2, c2 = s.startRow, s.startCol, s.endRow, s.endCol
	if r2 < r1 || (r2 == r1 && c2 < c1) {
		r1, c1, r2, c2 = r2, c2, r1, c1
	}
	return
}

// FinishText ends the selection and returns the selected plain text from
// content (escape sequences stripped, trailing spaces trimmed). Empty for a
// plain click without drag.
func (s *paneSelection) FinishText(content string) string {
	if !s.active {
		return ""
	}
	s.active = false
	r1, c1, r2, c2 := s.bounds()
	if r1 == r2 && c1 == c2 {
		return ""
	}

	lines := strings.Split(content, "\n")
	var out []string
	for r := r1; r <= r2 && r < len(lines); r++ {
		from, to := 0, -1
		if r == r1 {
			from = c1
		}
		if r == r2 {
			to = c2 + 1
		}
		out = append(out, strings.TrimRight(plainColumns(lines[r], from, to), " "))
	}
	return strings.Join(dedent(out, c1 > 0), "\n")
}

// Apply renders the in-progress selection as reverse video.
func (s *paneSelection) Apply(lines []string) []string {
	if !s.active {
		return lines
	}
	r1, c1, r2, c2 := s.bounds()
	for r := r1; r <= r2 && r < len(lines); r++ {
		from, to := 0, -1
		if r == r1 {
			from = c1
		}
		if r == r2 {
			to = c2 + 1
		}
		lines[r] = overlayRangeInLine(lines[r], from, to)
	}
	return lines
}

// dedent strips the common leading-space prefix from the fully selected lines.
// Agents like Claude Code render a two-space gutter that would otherwise stick
// to every copied line; relative indentation (code blocks) is preserved.
// skipFirst marks a first line that starts mid-column.
func dedent(lines []string, skipFirst bool) []string {
	common := -1
	for idx, line := range lines {
		if idx == 0 && skipFirst {
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if common == -1 || indent < common {
			common = indent
		}
	}
	if common <= 0 {
		return lines
	}
	for idx := range lines {
		if idx == 0 && skipFirst {
			continue
		}
		if len(lines[idx]) >= common {
			lines[idx] = lines[idx][common:]
		}
	}
	return lines
}
