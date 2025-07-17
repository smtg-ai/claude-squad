package overlay

import (
	"fmt"

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
	Title         string
	Submitted     bool
	Canceled      bool
	selectedBranch string
	OnSubmit      func(string)
	width, height int
	fixedWidth    int // Calculated fixed width based on content
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
		Title:         title,
		Submitted:     false,
		Canceled:      false,
		selectedBranch: defaultBranch,
	}
	
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
		// Submit the currently selected branch
		b.Submitted = true
		if b.OnSubmit != nil {
			selectedBranch := b.GetSelectedBranch()
			b.OnSubmit(selectedBranch)
		}
		return true
	default:
		// Forward key events to the list
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
	content += b.list.View() + "\n"

	// Current selection info with fixed width
	selectedBranch := b.GetSelectedBranch()
	selectionInfo := fmt.Sprintf("Selected: %s", selectedBranch)
	content += infoStyle.Render(selectionInfo) + "\n"
	
	// Instructions with fixed width
	instructions := "Press Enter to select • Press Esc to cancel"
	content += infoStyle.Render(instructions)

	return style.Render(content)
}