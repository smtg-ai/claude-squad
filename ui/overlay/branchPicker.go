package overlay

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const newBranchOption = "New branch (from HEAD)"

// BranchPicker is an embeddable component for selecting a branch.
// It does not hold the full branch list — results are provided asynchronously
// via SetResults after each debounced search.
type BranchPicker struct {
	results       []string // current search results (from git)
	filter        string   // current filter text
	filterVersion uint64   // incremented on each filter change
	cursor        int      // index into visibleItems()
	focused       bool
	width         int
	showNewBranch bool // whether to show the "New branch" option
}

// NewBranchPicker creates a new empty branch picker.
func NewBranchPicker() *BranchPicker {
	return &BranchPicker{
		showNewBranch: true,
	}
}

// SetWidth sets the width of the branch picker.
func (bp *BranchPicker) SetWidth(w int) {
	bp.width = w
}

// Focus gives the branch picker focus.
func (bp *BranchPicker) Focus() {
	bp.focused = true
}

// Blur removes focus from the branch picker.
func (bp *BranchPicker) Blur() {
	bp.focused = false
}

// IsFocused returns whether the branch picker is focused.
func (bp *BranchPicker) IsFocused() bool {
	return bp.focused
}

// GetFilter returns the current filter text.
func (bp *BranchPicker) GetFilter() string {
	return bp.filter
}

// GetFilterVersion returns a monotonically increasing version that changes on every filter edit.
func (bp *BranchPicker) GetFilterVersion() uint64 {
	return bp.filterVersion
}

// HandleKeyPress processes a key event. Returns (consumed, filterChanged).
func (bp *BranchPicker) HandleKeyPress(msg tea.KeyMsg) (consumed bool, filterChanged bool) {
	switch msg.Type {
	case tea.KeyUp:
		if bp.cursor > 0 {
			bp.cursor--
		}
		return true, false
	case tea.KeyDown:
		items := bp.visibleItems()
		if bp.cursor < len(items)-1 {
			bp.cursor++
		}
		return true, false
	case tea.KeyBackspace:
		if len(bp.filter) > 0 {
			runes := []rune(bp.filter)
			bp.filter = string(runes[:len(runes)-1])
			bp.filterVersion++
			return true, true
		}
		return true, false
	case tea.KeyRunes:
		bp.filter += string(msg.Runes)
		bp.filterVersion++
		return true, true
	case tea.KeySpace:
		bp.filter += " "
		bp.filterVersion++
		return true, true
	}
	return false, false
}

// SetResults updates the branch list with search results.
// version must match filterVersion for the results to be accepted (prevents stale updates).
func (bp *BranchPicker) SetResults(branches []string, version uint64) {
	if version != bp.filterVersion {
		return // stale results
	}
	bp.results = branches

	// Hide "New branch" when filter exactly matches a branch name
	bp.showNewBranch = true
	if bp.filter != "" {
		lower := strings.ToLower(bp.filter)
		for _, b := range branches {
			if strings.ToLower(b) == lower {
				bp.showNewBranch = false
				break
			}
		}
	}

	// Clamp cursor
	items := bp.visibleItems()
	if bp.cursor >= len(items) {
		if len(items) > 0 {
			bp.cursor = len(items) - 1
		} else {
			bp.cursor = 0
		}
	}
}

// visibleItems returns the list of items to display.
func (bp *BranchPicker) visibleItems() []string {
	var items []string
	if bp.showNewBranch {
		items = append(items, newBranchOption)
	}
	items = append(items, bp.results...)
	return items
}

// GetSelectedBranch returns the selected branch name, or empty string for "New branch".
func (bp *BranchPicker) GetSelectedBranch() string {
	items := bp.visibleItems()
	if bp.cursor < 0 || bp.cursor >= len(items) {
		return ""
	}
	selected := items[bp.cursor]
	if selected == newBranchOption {
		return ""
	}
	return selected
}

// Render renders the branch picker.
func (bp *BranchPicker) Render() string {
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("62")).
		Bold(true)

	filterStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("0"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	var s strings.Builder
	s.WriteString(labelStyle.Render("Branch"))
	if bp.focused {
		cursor := bp.filter + "█"
		s.WriteString(filterStyle.Render(" (filter: " + cursor + ")"))
	} else if bp.filter != "" {
		s.WriteString(dimStyle.Render(" (filter: " + bp.filter + ")"))
	}
	s.WriteString("\n")

	items := bp.visibleItems()
	if len(items) == 0 {
		s.WriteString(dimStyle.Render("  No matching branches"))
		return s.String()
	}

	// Show max 5 visible items, windowed around cursor
	maxVisible := 5
	start := 0
	if bp.cursor >= maxVisible {
		start = bp.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(items) {
		end = len(items)
	}

	for i := start; i < end; i++ {
		prefix := "  "
		label := items[i]
		if i == bp.cursor && bp.focused {
			prefix = "> "
			s.WriteString(selectedStyle.Render(prefix + label))
		} else if i == bp.cursor {
			s.WriteString(prefix + label)
		} else {
			s.WriteString(dimStyle.Render(prefix + label))
		}
		if i < end-1 {
			s.WriteString("\n")
		}
	}

	return s.String()
}
