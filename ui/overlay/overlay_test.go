package overlay

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// hyperlink wraps text in an OSC 8 hyperlink, the way Claude Code renders links.
func hyperlink(url, text string) string {
	return "\x1b]8;id=abc;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

// TestPlaceOverlayCentersOverHyperlinks guards against measuring OSC 8
// hyperlink URLs as visible text, which inflated the background width and
// pushed the overlay off-screen.
func TestPlaceOverlayCentersOverHyperlinks(t *testing.T) {
	const bgWidth, bgHeight = 40, 7

	linked := "Sources: " + hyperlink("https://example.com/a/very/long/path/that/is/not/visible", "example") + ", " +
		hyperlink("https://example.org/another/long/invisible/url", "other")
	linked += strings.Repeat(" ", bgWidth-ansi.StringWidth(linked))

	bgLines := make([]string, bgHeight)
	for i := range bgLines {
		bgLines[i] = strings.Repeat(".", bgWidth)
	}
	bgLines[3] = "\x1b[94m" + linked + "\x1b[0m"
	fg := "+----+\n|  X |\n+----+"

	out := PlaceOverlay(0, 0, fg, strings.Join(bgLines, "\n"), false, true)
	outLines := strings.Split(out, "\n")

	if len(outLines) != bgHeight {
		t.Fatalf("got %d lines, want %d", len(outLines), bgHeight)
	}
	for i, line := range outLines {
		if w := ansi.StringWidth(line); w != bgWidth {
			t.Errorf("line %d has width %d, want %d: %q", i, w, bgWidth, line)
		}
	}

	// fg is 6x3, so centered on 40x7 it starts at column 17, row 2.
	wantCol := (bgWidth - 6) / 2
	for row, want := range []string{"+----+", "|  X |", "+----+"} {
		plain := ansi.Strip(outLines[2+row])
		if got := strings.Index(plain, want); got != wantCol {
			t.Errorf("row %d: overlay at column %d, want %d: %q", 2+row, got, wantCol, plain)
		}
	}
}

// TestPlaceOverlayKeepsHyperlinkEscapesIntact checks that cutting a background
// line never splits an escape sequence, which would make the terminal swallow
// the rest of the frame.
func TestPlaceOverlayKeepsHyperlinkEscapesIntact(t *testing.T) {
	bg := hyperlink("https://example.com/xyz", strings.Repeat("L", 20)) + strings.Repeat(" ", 10)
	out := PlaceOverlay(5, 0, "FG", bg, false, false)

	if got, want := ansi.Strip(out), "LLLLLFGLLLLLLLLLLLLL          "; got != want {
		t.Errorf("visible text = %q, want %q", got, want)
	}
	if strings.Count(out, "\x1b]8;") != strings.Count(out, "\x1b\\") {
		t.Errorf("unbalanced OSC 8 sequences in %q", out)
	}
}
