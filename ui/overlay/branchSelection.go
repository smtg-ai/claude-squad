package overlay

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type branchItem struct {
	name string
}

func (i branchItem) FilterValue() string { return i.name }
func (i branchItem) Title() string       { return i.name }
func (i branchItem) Description() string { return "" }

// BranchSelectionOverlay represents a branch selection overlay with state management.
type BranchSelectionOverlay struct {
	list          list.Model
	branches      []string
	allBranches   []string // Keep original list for filtering
	Title         string
	Submitted     bool
	Canceled      bool
	selectedBranch string
	OnSubmit      func(string)
	width, height int
	fixedWidth    int // Calculated fixed width based on content
	filterText    string // Current filter text
}

// NewBranchSelectionOverlay creates a new branch selection overlay.
func NewBranchSelectionOverlay(title string, branches []string, defaultBranch string) *BranchSelectionOverlay {
	items := make([]list.Item, len(branches))
	for i, branch := range branches {
		items[i] = branchItem{name: branch}
	}

	// Create list model
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select Parent Branch"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	// Find and select default branch
	for i, branch := range branches {
		if branch == defaultBranch {
			l.Select(i)
			break
		}
	}

	overlay := &BranchSelectionOverlay{
		list:          l,
		branches:      branches,
		allBranches:   make([]string, len(branches)), // Store original list
		Title:         title,
		Submitted:     false,
		Canceled:      false,
		selectedBranch: defaultBranch,
		filterText:    "",
	}
	
	// Copy branches to allBranches
	copy(overlay.allBranches, branches)
	
	// Calculate fixed width based on content
	overlay.fixedWidth = overlay.calculateFixedWidth()
	
	return overlay
}

func (b *BranchSelectionOverlay) SetSize(width, height int) {
	b.width = width
	b.height = height
	
	// Use fixed width for the list, but respect the provided height
	listHeight := height - 6 // Leave room for title and submit button
	listWidth := b.fixedWidth - 4 // Account for border and padding
	if listWidth < 20 {
		listWidth = 20 // Minimum width
	}
	b.list.SetSize(listWidth, listHeight)
}

// calculateFixedWidth determines the optimal fixed width for the overlay
func (b *BranchSelectionOverlay) calculateFixedWidth() int {
	maxWidth := len(b.Title) // Start with title width
	
	// Check all branch names to find the longest
	for _, branch := range b.branches {
		if len(branch) > maxWidth {
			maxWidth = len(branch)
		}
	}
	
	// Check the selection info text (longest possible branch name + "Selected: ")
	longestBranch := ""
	for _, branch := range b.branches {
		if len(branch) > len(longestBranch) {
			longestBranch = branch
		}
	}
	selectionInfoWidth := len("Selected: " + longestBranch)
	if selectionInfoWidth > maxWidth {
		maxWidth = selectionInfoWidth
	}
	
	// Check instructions width
	instructionsWidth := len("Press Enter to select • Press Esc to cancel")
	if instructionsWidth > maxWidth {
		maxWidth = instructionsWidth
	}
	
	// Add padding for borders and spacing (border + padding + margin)
	return maxWidth + 8
}

// filterBranches filters the branch list based on the current filter text
func (b *BranchSelectionOverlay) filterBranches() {
	if b.filterText == "" {
		// No filter, show all branches
		b.branches = make([]string, len(b.allBranches))
		copy(b.branches, b.allBranches)
	} else {
		// Filter branches that start with the filter text
		var filtered []string
		for _, branch := range b.allBranches {
			if strings.HasPrefix(strings.ToLower(branch), strings.ToLower(b.filterText)) {
				filtered = append(filtered, branch)
			}
		}
		b.branches = filtered
	}
	
	// Update the list with filtered branches
	items := make([]list.Item, len(b.branches))
	for i, branch := range b.branches {
		items[i] = branchItem{name: branch}
	}
	b.list.SetItems(items)
	
	// Reset selection to first item if available
	if len(b.branches) > 0 {
		b.list.Select(0)
		b.selectedBranch = b.branches[0]
	} else {
		b.selectedBranch = ""
	}
}

// Init initializes the branch selection overlay model
func (b *BranchSelectionOverlay) Init() tea.Cmd {
	return nil
}

// View renders the model's view
func (b *BranchSelectionOverlay) View() string {
	return b.Render()
}

// HandleKeyPress processes a key press and updates the state accordingly.
// Returns true if the overlay should be closed.
func (b *BranchSelectionOverlay) HandleKeyPress(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyEsc:
		b.Canceled = true
		return true
	case tea.KeyEnter:
		// Submit the currently selected branch (only if we have branches)
		if len(b.branches) > 0 {
			b.Submitted = true
			if b.OnSubmit != nil {
				selectedBranch := b.GetSelectedBranch()
				b.OnSubmit(selectedBranch)
			}
			return true
		}
		return false
	case tea.KeyBackspace:
		// Handle backspace in filter
		if len(b.filterText) > 0 {
			b.filterText = b.filterText[:len(b.filterText)-1]
			b.filterBranches()
		}
		return false
	case tea.KeyUp, tea.KeyDown:
		// Handle navigation in the list
		var cmd tea.Cmd
		b.list, cmd = b.list.Update(msg)
		// Update selected branch when list selection changes
		if selectedItem := b.list.SelectedItem(); selectedItem != nil {
			if item, ok := selectedItem.(branchItem); ok {
				b.selectedBranch = item.name
			}
		}
		_ = cmd
		return false
	default:
		// Handle character input for filtering
		if msg.Type == tea.KeyRunes {
			for _, r := range msg.Runes {
				// Only add printable characters to filter
				if r >= 32 && r < 127 {
					b.filterText += string(r)
					b.filterBranches()
				}
			}
		}
		return false
	}
}

// GetSelectedBranch returns the currently selected branch.
func (b *BranchSelectionOverlay) GetSelectedBranch() string {
	return b.selectedBranch
}

// IsSubmitted returns whether the form was submitted.
func (b *BranchSelectionOverlay) IsSubmitted() bool {
	return b.Submitted
}

// IsCanceled returns whether the form was canceled.
func (b *BranchSelectionOverlay) IsCanceled() bool {
	return b.Canceled
}

// SetOnSubmit sets a callback function for form submission.
func (b *BranchSelectionOverlay) SetOnSubmit(onSubmit func(string)) {
	b.OnSubmit = onSubmit
}

// Render renders the branch selection overlay.
func (b *BranchSelectionOverlay) Render() string {
	// Create styles with fixed width
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(b.fixedWidth)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("62")).
		Bold(true).
		MarginBottom(1).
		Width(b.fixedWidth - 4) // Account for border and padding

	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		MarginTop(1).
		Width(b.fixedWidth - 4) // Account for border and padding

	// Build the view with fixed-width content
	content := titleStyle.Render(b.Title) + "\n"
	
	// Show filter text if user is typing
	if b.filterText != "" {
		filterStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("yellow")).
			Bold(true).
			Width(b.fixedWidth - 4)
		filterInfo := fmt.Sprintf("Filter: %s", b.filterText)
		content += filterStyle.Render(filterInfo) + "\n"
	}
	
	content += b.list.View() + "\n"

	// Current selection info with fixed width
	selectedBranch := b.GetSelectedBranch()
	if selectedBranch != "" {
		selectionInfo := fmt.Sprintf("Selected: %s", selectedBranch)
		content += infoStyle.Render(selectionInfo) + "\n"
	} else if len(b.branches) == 0 {
		noMatchInfo := "No matching branches"
		content += infoStyle.Render(noMatchInfo) + "\n"
	}
	
	// Instructions with fixed width
	instructions := "Type to filter • Enter to select • Esc to cancel"
	content += infoStyle.Render(instructions)

	return style.Render(content)
}