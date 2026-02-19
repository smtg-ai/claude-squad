package ui

import (
	"fmt"
	"hivemind/session"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

var (
	AdditionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))
	DeletionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444"))
	HunkStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#0ea5e9"))

	fileItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F0A868"))
	fileItemSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#F0A868")).
				Foreground(lipgloss.Color("#1a1a1a")).
				Bold(true)
	fileItemDimStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
	filePanelBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#d0d0d0", Dark: "#333333"})
	filePanelBorderFocusedStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color("#F0A868"))
	diffHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F0A868")).
			Bold(true)
	diffHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#aaaaaa", Dark: "#555555"})
)

// fileChunk holds a single file's parsed diff data.
type fileChunk struct {
	path    string
	added   int
	removed int
	diff    string
}

type DiffPane struct {
	viewport viewport.Model
	width    int
	height   int

	files        []fileChunk
	totalAdded   int
	totalRemoved int
	fullDiff     string

	// selectedFile: -1 = all files, 0..N = specific file
	selectedFile int

	// sidebarWidth is computed from file names
	sidebarWidth int
}

func NewDiffPane() *DiffPane {
	return &DiffPane{
		viewport:     viewport.New(0, 0),
		selectedFile: 0,
	}
}

func (d *DiffPane) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.computeSidebarWidth()
	d.updateViewportWidth()
	d.viewport.Height = height
	d.rebuildViewport()
}

func (d *DiffPane) updateViewportWidth() {
	diffWidth := d.width - d.sidebarWidth - 1
	if diffWidth < 10 {
		diffWidth = 10
	}
	d.viewport.Width = diffWidth
}

func (d *DiffPane) computeSidebarWidth() {
	// Compute the inner content width needed, then add border frame.
	borderFrame := filePanelBorderStyle.GetHorizontalFrameSize() // typically 2 for left+right border
	innerMin := 18
	innerMax := innerMin
	for _, f := range d.files {
		base := filepath.Base(f.path)
		statsW := len(fmt.Sprintf(" +%d -%d", f.added, f.removed))
		w := runewidth.StringWidth(base) + statsW + 4
		if w > innerMax {
			innerMax = w
		}
	}
	// Cap at 35% of total width (including border)
	limit := d.width*35/100 - borderFrame
	if limit < innerMin {
		limit = innerMin
	}
	if innerMax > limit {
		innerMax = limit
	}
	// sidebarWidth is the total outer width (content + border)
	d.sidebarWidth = innerMax + borderFrame
}

func (d *DiffPane) SetDiff(instance *session.Instance) {
	if instance == nil || !instance.Started() {
		d.files = nil
		d.fullDiff = ""
		return
	}

	stats := instance.GetDiffStats()
	if stats == nil || stats.Error != nil || stats.IsEmpty() {
		d.files = nil
		d.fullDiff = ""
		if stats != nil && stats.Error != nil {
			d.fullDiff = fmt.Sprintf("Error: %v", stats.Error)
		}
		return
	}

	d.totalAdded = stats.Added
	d.totalRemoved = stats.Removed
	d.files = parseFileChunks(stats.Content)
	d.fullDiff = colorizeDiff(stats.Content)

	if d.selectedFile >= len(d.files) {
		d.selectedFile = len(d.files) - 1
	}
	if d.selectedFile < -1 {
		d.selectedFile = -1
	}

	d.computeSidebarWidth()
	d.updateViewportWidth()
	d.rebuildViewport()
}

func (d *DiffPane) rebuildViewport() {
	if len(d.files) == 0 {
		return
	}
	var diff string
	if d.selectedFile < 0 {
		diff = d.fullDiff
	} else if d.selectedFile < len(d.files) {
		diff = colorizeDiff(d.files[d.selectedFile].diff)
	}
	d.viewport.SetContent(diff)
}

func (d *DiffPane) String() string {
	if len(d.files) == 0 {
		msg := "No changes"
		if d.fullDiff != "" {
			msg = d.fullDiff
		}
		return lipgloss.Place(d.width, d.height, lipgloss.Center, lipgloss.Center, msg)
	}

	sidebar := d.renderSidebar()
	diffContent := d.viewport.View()

	// Join sidebar and diff horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " ", diffContent)
}

// renderSidebar builds the left-hand file list panel.
func (d *DiffPane) renderSidebar() string {
	borderFrame := filePanelBorderStyle.GetHorizontalFrameSize()
	innerWidth := d.sidebarWidth - borderFrame

	var b strings.Builder

	// Header
	additions := AdditionStyle.Render(fmt.Sprintf("+%d", d.totalAdded))
	deletions := DeletionStyle.Render(fmt.Sprintf("-%d", d.totalRemoved))
	headerText := fmt.Sprintf("%s %s", additions, deletions)
	b.WriteString(headerText)
	b.WriteString("\n")

	// "All" entry
	allLabel := "\uf0ce All"
	if d.selectedFile == -1 {
		b.WriteString(fileItemSelectedStyle.Width(innerWidth).Render(" " + allLabel))
	} else {
		b.WriteString(fileItemStyle.Render(" " + allLabel))
	}
	b.WriteString("\n")

	// File entries
	for i, f := range d.files {
		base := filepath.Base(f.path)
		dir := filepath.Dir(f.path)
		isSelected := i == d.selectedFile

		// Stats suffix
		statsStr := ""
		if f.added > 0 {
			statsStr += fmt.Sprintf("+%d", f.added)
		}
		if f.removed > 0 {
			if statsStr != "" {
				statsStr += " "
			}
			statsStr += fmt.Sprintf("-%d", f.removed)
		}

		if isSelected {
			// Truncate filename to fit
			maxName := innerWidth - runewidth.StringWidth(statsStr) - 3
			name := base
			if maxName > 3 && runewidth.StringWidth(name) > maxName {
				name = runewidth.Truncate(name, maxName, "…")
			}
			line := fmt.Sprintf(" %s %s", name, statsStr)
			b.WriteString(fileItemSelectedStyle.Width(innerWidth).Render(line))
		} else {
			// Show dir/ dimmed, filename in accent color
			maxName := innerWidth - runewidth.StringWidth(statsStr) - 3
			var nameDisplay string
			if dir != "." {
				dirPrefix := dir + "/"
				remaining := maxName - runewidth.StringWidth(dirPrefix)
				if remaining < 4 {
					// Not enough room for dir, just show filename
					name := base
					if maxName > 3 && runewidth.StringWidth(name) > maxName {
						name = runewidth.Truncate(name, maxName, "…")
					}
					nameDisplay = fileItemStyle.Render(name)
				} else {
					name := base
					if runewidth.StringWidth(name) > remaining {
						name = runewidth.Truncate(name, remaining, "…")
					}
					nameDisplay = fileItemDimStyle.Render(dirPrefix) + fileItemStyle.Render(name)
				}
			} else {
				name := base
				if maxName > 3 && runewidth.StringWidth(name) > maxName {
					name = runewidth.Truncate(name, maxName, "…")
				}
				nameDisplay = fileItemStyle.Render(name)
			}

			// Colored stats
			coloredStats := ""
			if f.added > 0 {
				coloredStats += AdditionStyle.Render(fmt.Sprintf("+%d", f.added))
			}
			if f.removed > 0 {
				if coloredStats != "" {
					coloredStats += " "
				}
				coloredStats += DeletionStyle.Render(fmt.Sprintf("-%d", f.removed))
			}

			b.WriteString(" " + nameDisplay + " " + coloredStats)
		}
		b.WriteString("\n")
	}

	// Fill remaining height with empty lines so the border stretches
	lines := 2 + len(d.files)             // header + all entry + file entries
	for i := lines; i < d.height-3; i++ { // -3 for border + hint
		b.WriteString("\n")
	}

	// Hint at the bottom
	b.WriteString(diffHintStyle.Render("shift+↑↓"))

	content := b.String()
	vertFrame := filePanelBorderStyle.GetVerticalFrameSize()
	innerHeight := d.height - vertFrame
	if innerHeight < 1 {
		innerHeight = 1
	}
	return filePanelBorderStyle.Width(innerWidth).Height(innerHeight).Render(content)
}

func (d *DiffPane) FileUp() {
	if len(d.files) == 0 {
		return
	}
	d.selectedFile--
	if d.selectedFile < -1 {
		d.selectedFile = len(d.files) - 1
	}
	d.rebuildViewport()
	d.viewport.GotoTop()
}

func (d *DiffPane) FileDown() {
	if len(d.files) == 0 {
		return
	}
	d.selectedFile++
	if d.selectedFile >= len(d.files) {
		d.selectedFile = -1
	}
	d.rebuildViewport()
	d.viewport.GotoTop()
}

func (d *DiffPane) ScrollUp() {
	d.viewport.LineUp(3)
}

func (d *DiffPane) ScrollDown() {
	d.viewport.LineDown(3)
}

func (d *DiffPane) HasFiles() bool {
	return len(d.files) > 0
}

// parseFileChunks splits a unified diff into per-file chunks with stats.
func parseFileChunks(content string) []fileChunk {
	var chunks []fileChunk
	var current *fileChunk
	var currentLines strings.Builder

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if current != nil {
				current.diff = currentLines.String()
				currentLines.Reset()
			}
			parts := strings.SplitN(line, " b/", 2)
			path := ""
			if len(parts) == 2 {
				path = parts[1]
			}
			chunks = append(chunks, fileChunk{path: path})
			current = &chunks[len(chunks)-1]
			currentLines.WriteString(line + "\n")
		} else if current != nil {
			currentLines.WriteString(line + "\n")
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				current.added++
			} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				current.removed++
			}
		}
	}
	if current != nil {
		current.diff = currentLines.String()
	}
	return chunks
}

func colorizeDiff(diff string) string {
	var coloredOutput strings.Builder
	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if len(line) > 0 {
			if strings.HasPrefix(line, "@@") {
				coloredOutput.WriteString(HunkStyle.Render(line) + "\n")
			} else if line[0] == '+' && (len(line) == 1 || line[1] != '+') {
				coloredOutput.WriteString(AdditionStyle.Render(line) + "\n")
			} else if line[0] == '-' && (len(line) == 1 || line[1] != '-') {
				coloredOutput.WriteString(DeletionStyle.Render(line) + "\n")
			} else {
				coloredOutput.WriteString(line + "\n")
			}
		} else {
			coloredOutput.WriteString("\n")
		}
	}
	return coloredOutput.String()
}
