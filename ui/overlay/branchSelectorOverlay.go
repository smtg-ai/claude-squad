package overlay

import (
	"claude-squad/session/git"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type BranchSelectorOverlay struct {
	branches       []git.BranchInfo
	filteredBranches []git.BranchInfo
	cursor         int
	selected       bool
	selectedBranch string
	filter         textinput.Model
	width          int
	height         int
}

func NewBranchSelectorOverlay(branches []git.BranchInfo) *BranchSelectorOverlay {
	ti := textinput.New()
	ti.Placeholder = "Filter branches..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50

	b := &BranchSelectorOverlay{
		branches:         branches,
		filteredBranches: branches,
		filter:          ti,
		width:           80,
		height:          20,
	}
	return b
}

func (b *BranchSelectorOverlay) Init() tea.Cmd {
	return textinput.Blink
}

func (b *BranchSelectorOverlay) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			b.selected = true
			b.selectedBranch = ""
			return b, nil
		case "enter":
			if len(b.filteredBranches) > 0 {
				b.selected = true
				b.selectedBranch = b.filteredBranches[b.cursor].Name
			}
			return b, nil
		case "up", "ctrl+p":
			if b.cursor > 0 {
				b.cursor--
			}
		case "down", "ctrl+n":
			if b.cursor < len(b.filteredBranches)-1 {
				b.cursor++
			}
		default:
			// Update filter input
			prevFilter := b.filter.Value()
			b.filter, cmd = b.filter.Update(msg)
			
			// If filter changed, update filtered branches
			if b.filter.Value() != prevFilter {
				b.updateFilteredBranches()
			}
			return b, cmd
		}
	case tea.WindowSizeMsg:
		b.width = msg.Width
		b.height = msg.Height
	}

	return b, nil
}

func (b *BranchSelectorOverlay) updateFilteredBranches() {
	filter := strings.ToLower(b.filter.Value())
	if filter == "" {
		b.filteredBranches = b.branches
	} else {
		b.filteredBranches = make([]git.BranchInfo, 0)
		for _, branch := range b.branches {
			if strings.Contains(strings.ToLower(branch.Name), filter) ||
				strings.Contains(strings.ToLower(branch.CommitMessage), filter) {
				b.filteredBranches = append(b.filteredBranches, branch)
			}
		}
	}
	
	// Reset cursor if it's out of bounds
	if b.cursor >= len(b.filteredBranches) {
		b.cursor = max(0, len(b.filteredBranches)-1)
	}
}

func (b *BranchSelectorOverlay) View() string {
	if b.selected {
		return ""
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4")).
		MarginBottom(1)

	listStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2).
		Width(b.width - 4).
		Height(b.height - 6)

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7D56F4")).
		Foreground(lipgloss.Color("#FAFAFA"))

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA"))

	mutedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666"))

	// Build the view
	var s strings.Builder

	// Title
	s.WriteString(titleStyle.Render("Select a Remote Branch"))
	s.WriteString("\n\n")

	// Filter input
	s.WriteString(b.filter.View())
	s.WriteString("\n\n")

	// Branch list
	maxVisible := b.height - 10 // Account for title, filter, borders, padding
	if maxVisible < 1 {
		maxVisible = 5
	}

	// Calculate visible range
	startIdx := 0
	endIdx := len(b.filteredBranches)
	
	if len(b.filteredBranches) > maxVisible {
		// Scroll to keep cursor visible
		if b.cursor >= maxVisible {
			startIdx = b.cursor - maxVisible + 1
		}
		endIdx = min(startIdx + maxVisible, len(b.filteredBranches))
	}

	// Display branches
	var branchList strings.Builder
	for i := startIdx; i < endIdx; i++ {
		branch := b.filteredBranches[i]
		
		// Format branch line
		timeAgo := formatTimeAgo(branch.CommitTime)
		branchLine := fmt.Sprintf("%-30s %s", 
			truncateString(branch.Name, 30),
			mutedStyle.Render(fmt.Sprintf("%s • %s", timeAgo, truncateString(branch.CommitMessage, 40))))

		if i == b.cursor {
			branchList.WriteString(selectedStyle.Render("> " + branchLine))
		} else {
			branchList.WriteString(normalStyle.Render("  " + branchLine))
		}
		
		if i < endIdx-1 {
			branchList.WriteString("\n")
		}
	}

	// Show scroll indicators
	if startIdx > 0 {
		branchList.WriteString("\n" + mutedStyle.Render("↑ more above"))
	}
	if endIdx < len(b.filteredBranches) {
		branchList.WriteString("\n" + mutedStyle.Render("↓ more below"))
	}

	s.WriteString(listStyle.Render(branchList.String()))
	
	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		MarginTop(1)
	s.WriteString("\n")
	s.WriteString(helpStyle.Render("↑/↓ navigate • enter select • esc cancel"))

	return s.String()
}

func (b *BranchSelectorOverlay) IsSelected() bool {
	return b.selected
}

func (b *BranchSelectorOverlay) SelectedBranch() string {
	return b.selectedBranch
}

func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)
	
	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if duration < 30*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else if duration < 365*24*time.Hour {
		months := int(duration.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
	
	years := int(duration.Hours() / 24 / 365)
	if years == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", years)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}