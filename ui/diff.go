package ui

import (
	"claude-squad/session"
	"claude-squad/session/git"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var (
	AdditionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))
	DeletionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444"))
	HunkStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#0ea5e9"))
)

type DiffMode int

const (
	DiffModeAll DiffMode = iota
	DiffModeLastCommit
)

type DiffPane struct {
	viewport      viewport.Model
	diff          string
	stats         string
	width         int
	height        int
	filePositions []int // Line numbers where each file starts
	mode          DiffMode
	instance      *session.Instance
	commitOffset  int // Offset from HEAD when viewing commits (0 = HEAD, 1 = HEAD~1, etc.)
}

func NewDiffPane() *DiffPane {
	return &DiffPane{
		viewport: viewport.New(0, 0),
		mode:     DiffModeAll,
	}
}

func (d *DiffPane) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.viewport.Width = width
	d.viewport.Height = height
	// Update viewport content if diff exists
	if d.diff != "" || d.stats != "" {
		d.viewport.SetContent(lipgloss.JoinVertical(lipgloss.Left, d.stats, d.diff))
	}
}

func (d *DiffPane) SetDiff(instance *session.Instance) {
	d.instance = instance
	d.refreshDiff()
}

func (d *DiffPane) refreshDiff() {
	centeredFallbackMessage := lipgloss.Place(
		d.width,
		d.height,
		lipgloss.Center,
		lipgloss.Center,
		"No changes",
	)

	if d.instance == nil || !d.instance.Started() {
		d.viewport.SetContent(centeredFallbackMessage)
		return
	}

	var stats *git.DiffStats
	var modeLabel string
	
	switch d.mode {
	case DiffModeAll:
		stats = d.instance.GetDiffStats()
		modeLabel = "[All Changes] "
	case DiffModeLastCommit:
		// Show commit diff based on offset
		stats = d.instance.GetCommitDiffAtOffset(d.commitOffset)
		if d.commitOffset == -1 && stats != nil && stats.IsUncommitted {
			modeLabel = "[Uncommitted Changes] "
		} else {
			// Determine the actual offset for commit info
			// When commitOffset is -1 but we're showing HEAD (no uncommitted changes), use offset 0
			actualOffset := d.commitOffset
			if d.commitOffset == -1 && stats != nil && !stats.IsUncommitted {
				actualOffset = 0
			}
			
			if hash, msg, err := d.instance.GetCommitInfo(actualOffset); err == nil {
				// Truncate message if too long
				if len(msg) > 40 {
					msg = msg[:37] + "..."
				}
				if actualOffset == 0 {
					modeLabel = fmt.Sprintf("[HEAD: %s] ", msg)
				} else {
					modeLabel = fmt.Sprintf("[%s: %s] ", hash, msg)
				}
			} else {
				if actualOffset == 0 {
					modeLabel = "[Last Commit] "
				} else {
					modeLabel = fmt.Sprintf("[HEAD~%d] ", actualOffset)
				}
			}
		}
	}
	if stats == nil {
		// Show loading message if worktree is not ready
		centeredMessage := lipgloss.Place(
			d.width,
			d.height,
			lipgloss.Center,
			lipgloss.Center,
			"Setting up worktree...",
		)
		d.viewport.SetContent(centeredMessage)
		return
	}

	if stats.Error != nil {
		// Show error message
		centeredMessage := lipgloss.Place(
			d.width,
			d.height,
			lipgloss.Center,
			lipgloss.Center,
			fmt.Sprintf("Error: %v", stats.Error),
		)
		d.viewport.SetContent(centeredMessage)
		return
	}

	if stats.IsEmpty() {
		d.stats = ""
		d.diff = ""
		d.viewport.SetContent(centeredFallbackMessage)
	} else {
		additions := AdditionStyle.Render(fmt.Sprintf("%d additions(+)", stats.Added))
		deletions := DeletionStyle.Render(fmt.Sprintf("%d deletions(-)", stats.Removed))
		d.stats = lipgloss.JoinHorizontal(lipgloss.Center, modeLabel, additions, " ", deletions)
		d.diff = colorizeDiff(stats.Content)
		content := lipgloss.JoinVertical(lipgloss.Left, d.stats, d.diff)
		d.viewport.SetContent(content)
		
		// Parse file positions after setting content
		d.parseFilePositions(content)
	}
}

func (d *DiffPane) String() string {
	return d.viewport.View()
}

// ScrollUp scrolls the viewport up
func (d *DiffPane) ScrollUp() {
	d.viewport.LineUp(1)
}

// ScrollDown scrolls the viewport down
func (d *DiffPane) ScrollDown() {
	d.viewport.LineDown(1)
}

// ScrollToTop scrolls the viewport to the top
func (d *DiffPane) ScrollToTop() {
	d.viewport.GotoTop()
}

// ScrollToBottom scrolls the viewport to the bottom
func (d *DiffPane) ScrollToBottom() {
	d.viewport.GotoBottom()
}

// PageUp scrolls the viewport up by one page
func (d *DiffPane) PageUp() {
	d.viewport.ViewUp()
}

// PageDown scrolls the viewport down by one page
func (d *DiffPane) PageDown() {
	d.viewport.ViewDown()
}

// parseFilePositions identifies line numbers where each file starts in the diff
func (d *DiffPane) parseFilePositions(content string) {
	d.filePositions = []int{}
	lines := strings.Split(content, "\n")
	
	for i, line := range lines {
		// Look for diff headers that indicate a new file
		if strings.HasPrefix(line, "diff --git") {
			d.filePositions = append(d.filePositions, i)
		}
	}
}

// JumpToNextFile jumps to the next file in the diff
func (d *DiffPane) JumpToNextFile() {
	if len(d.filePositions) == 0 {
		return
	}
	
	currentOffset := d.viewport.YOffset
	
	// Find the next file position after the current offset
	for _, pos := range d.filePositions {
		if pos > currentOffset {
			d.viewport.SetYOffset(pos)
			return
		}
	}
	
	// If no next file, stay at current position
}

// JumpToPrevFile jumps to the previous file in the diff
func (d *DiffPane) JumpToPrevFile() {
	if len(d.filePositions) == 0 {
		return
	}
	
	currentOffset := d.viewport.YOffset
	
	// Find the previous file position before the current offset
	for i := len(d.filePositions) - 1; i >= 0; i-- {
		if d.filePositions[i] < currentOffset {
			d.viewport.SetYOffset(d.filePositions[i])
			return
		}
	}
	
	// If no previous file, jump to the first file
	if currentOffset > d.filePositions[0] {
		d.viewport.SetYOffset(d.filePositions[0])
	}
}

// SetDiffMode changes the diff display mode
func (d *DiffPane) SetDiffMode(mode DiffMode) {
	if d.mode != mode {
		d.mode = mode
		if mode == DiffModeLastCommit {
			d.commitOffset = -1 // Start with uncommitted changes
		} else {
			d.commitOffset = 0
		}
		d.refreshDiff()
	}
}

// GetDiffMode returns the current diff mode
func (d *DiffPane) GetDiffMode() DiffMode {
	return d.mode
}

// NavigateToPrevCommit moves to the previous (older) commit
func (d *DiffPane) NavigateToPrevCommit() {
	if d.mode == DiffModeLastCommit {
		d.commitOffset++
		d.refreshDiff()
	}
}

// NavigateToNextCommit moves to the next (newer) commit
func (d *DiffPane) NavigateToNextCommit() {
	if d.mode == DiffModeLastCommit {
		// Check if we're trying to go to uncommitted changes (-1)
		if d.commitOffset == 0 {
			// Only allow going to -1 if there are uncommitted changes
			stats := d.instance.GetCommitDiffAtOffset(-1)
			if stats != nil && stats.IsUncommitted && !stats.IsEmpty() {
				d.commitOffset = -1
				d.refreshDiff()
			}
			// Otherwise stay at 0 (HEAD)
		} else if d.commitOffset > 0 {
			// Normal navigation to newer commits
			d.commitOffset--
			d.refreshDiff()
		}
	}
}

func colorizeDiff(diff string) string {
	var coloredOutput strings.Builder

	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if len(line) > 0 {
			if strings.HasPrefix(line, "@@") {
				// Color hunk headers cyan
				coloredOutput.WriteString(HunkStyle.Render(line) + "\n")
			} else if line[0] == '+' && (len(line) == 1 || line[1] != '+') {
				// Color added lines green, excluding metadata like '+++'
				coloredOutput.WriteString(AdditionStyle.Render(line) + "\n")
			} else if line[0] == '-' && (len(line) == 1 || line[1] != '-') {
				// Color removed lines red, excluding metadata like '---'
				coloredOutput.WriteString(DeletionStyle.Render(line) + "\n")
			} else {
				// Print metadata and unchanged lines without color
				coloredOutput.WriteString(line + "\n")
			}
		} else {
			// Preserve empty lines
			coloredOutput.WriteString("\n")
		}
	}

	return coloredOutput.String()
}
