package overlay

import (
	"claude-squad/project"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProjectHistoryOverlay represents a project history selection overlay
type ProjectHistoryOverlay struct {
	projectManager *project.ProjectManager
	submitted      bool
	canceled       bool
	selectedPath   string
	width, height  int

	// Grid navigation for 0-9 quick select
	projectPaths  []string // All recent project paths
	filteredPaths []string // Filtered list for search
	selectedIndex int      // Selected index in filtered list

	// Search functionality
	searchMode  bool
	searchQuery string
}

// NewProjectHistoryOverlay creates a new project history overlay
func NewProjectHistoryOverlay(projectManager *project.ProjectManager) *ProjectHistoryOverlay {
	if projectManager == nil {
		return nil
	}

	// Get recent project paths
	recentPaths := projectManager.GetRecentProjectPaths()

	overlay := &ProjectHistoryOverlay{
		projectManager: projectManager,
		submitted:      false,
		canceled:       false,
		selectedPath:   "",
		projectPaths:   recentPaths,
		filteredPaths:  recentPaths, // Initially show all
		selectedIndex:  0,
		searchMode:     false,
		searchQuery:    "",
	}

	return overlay
}

// SetSize sets the dimensions for the overlay
func (p *ProjectHistoryOverlay) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// Init initializes the overlay
func (p *ProjectHistoryOverlay) Init() tea.Cmd {
	return nil
}

// View renders the overlay
func (p *ProjectHistoryOverlay) View() string {
	return p.Render()
}

// HandleKeyPress processes key presses according to Sally's UX spec
func (p *ProjectHistoryOverlay) HandleKeyPress(msg tea.KeyMsg) bool {
	// Handle search mode
	if p.searchMode {
		switch msg.Type {
		case tea.KeyEsc:
			// Exit search mode and clear filter
			p.searchMode = false
			p.searchQuery = ""
			p.filteredPaths = p.projectPaths
			p.selectedIndex = 0
			return false
		case tea.KeyEnter:
			// Exit search mode but keep filter
			p.searchMode = false
			return false
		case tea.KeyBackspace:
			if len(p.searchQuery) > 0 {
				p.searchQuery = p.searchQuery[:len(p.searchQuery)-1]
				p.filterProjects()
			}
			return false
		case tea.KeyRunes:
			p.searchQuery += string(msg.Runes)
			p.filterProjects()
			return false
		default:
			return false
		}
	}

	// Normal navigation mode
	switch msg.String() {
	case "esc", "q":
		p.canceled = true
		return true
	case "/":
		// Enter search mode (UX spec: typing starts search)
		p.searchMode = true
		return false
	case "c":
		// Clear history - keep last 10 (UX spec)
		if err := p.projectManager.ClearProjectHistory(10); err == nil {
			// Refresh the display
			p.projectPaths = p.projectManager.GetRecentProjectPaths()
			p.filteredPaths = p.projectPaths
			p.selectedIndex = 0
		}
		return false
	case "n":
		// New manual project - close this overlay and trigger add project
		p.selectedPath = "NEW_MANUAL"
		p.submitted = true
		return true
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Quick select 0-9 (UX spec)
		index := int(msg.String()[0] - '0')
		if index < len(p.filteredPaths) {
			p.selectedIndex = index
			p.selectedPath = p.filteredPaths[index]
			p.submitted = true
			return true
		}
		return false
	case "up", "k":
		p.moveUp()
		return false
	case "down", "j":
		p.moveDown()
		return false
	case " ", "enter":
		// Select current project
		if p.selectedIndex >= 0 && p.selectedIndex < len(p.filteredPaths) {
			p.selectedPath = p.filteredPaths[p.selectedIndex]
			p.submitted = true
			return true
		}
		return false
	default:
		// Start search on any typing (UX spec: search-first)
		if msg.Type == tea.KeyRunes {
			p.searchMode = true
			p.searchQuery = string(msg.Runes)
			p.filterProjects()
		}
		return false
	}
}

// filterProjects filters the project list based on search query
func (p *ProjectHistoryOverlay) filterProjects() {
	if p.searchQuery == "" {
		p.filteredPaths = p.projectPaths
	} else {
		p.filteredPaths = p.projectManager.FilterProjectPaths(p.searchQuery)
	}
	// Reset selection to first item
	p.selectedIndex = 0
}

// moveUp moves selection up
func (p *ProjectHistoryOverlay) moveUp() {
	if p.selectedIndex > 0 {
		p.selectedIndex--
	}
}

// moveDown moves selection down
func (p *ProjectHistoryOverlay) moveDown() {
	if p.selectedIndex < len(p.filteredPaths)-1 {
		p.selectedIndex++
	}
}

// IsSubmitted returns whether the overlay was submitted
func (p *ProjectHistoryOverlay) IsSubmitted() bool {
	return p.submitted
}

// IsCanceled returns whether the overlay was canceled
func (p *ProjectHistoryOverlay) IsCanceled() bool {
	return p.canceled
}

// GetSelectedPath returns the selected project path
func (p *ProjectHistoryOverlay) GetSelectedPath() string {
	return p.selectedPath
}

// Render renders the project history overlay
func (p *ProjectHistoryOverlay) Render() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	var content string

	if len(p.projectPaths) == 0 {
		content = "No recent projects found.\n\nStart by adding a project with 'n' for new manual entry."
		helpText := "\nPress 'n' for new project or 'q' to cancel"
		helpStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
		content += helpStyle.Render(helpText)
	} else {
		// Header with count
		totalCount := len(p.projectPaths)
		headerText := fmt.Sprintf("Recent Projects (%d total):", totalCount)
		content = headerText + "\n"

		// Show search info if in search mode or filter is active
		if p.searchMode {
			searchText := fmt.Sprintf("> %s_", p.searchQuery)
			searchStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("33"))
			content += searchStyle.Render(searchText) + "\n"
		} else if p.searchQuery != "" {
			filterText := fmt.Sprintf("> %s [Showing %d of %d]", p.searchQuery, len(p.filteredPaths), totalCount)
			filterStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))
			content += filterStyle.Render(filterText) + "\n"
		} else {
			// Show input prompt when not searching
			promptText := "> _"
			promptStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))
			content += promptStyle.Render(promptText) + "\n"
		}

		content += "\n" + p.renderProjectList()

		// Dynamic help text based on mode
		var helpText string
		if p.searchMode {
			helpText = "\nSearch: (esc)clear filter, (enter)keep filter"
		} else {
			helpText = "\n[0-9] select • [/] search • [c]clear • [n]ew • [q]uit"
		}
		helpStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
		content += helpStyle.Render(helpText)
	}

	return style.Render(content)
}

// renderProjectList renders the list of projects with quick select numbers
func (p *ProjectHistoryOverlay) renderProjectList() string {
	if len(p.filteredPaths) == 0 {
		return "No projects match your search"
	}

	var lines []string

	// Show first 10 with numbers for quick select (UX spec: 0-9)
	displayCount := len(p.filteredPaths)
	if displayCount > 10 {
		displayCount = 10
	}

	for i := 0; i < displayCount; i++ {
		path := p.filteredPaths[i]

		// Extract directory name for cleaner display
		dirName := filepath.Base(path)
		displayText := fmt.Sprintf("%d. %-20s %s", i, dirName, path)

		// Highlight selected item
		if i == p.selectedIndex {
			selectedStyle := lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("15"))
			displayText = selectedStyle.Render(displayText)
		}

		lines = append(lines, displayText)
	}

	// Show count if more than 10
	if len(p.filteredPaths) > 10 {
		moreText := fmt.Sprintf("... and %d more (refine search to see all)", len(p.filteredPaths)-10)
		moreStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
		lines = append(lines, "", moreStyle.Render(moreText))
	}

	return strings.Join(lines, "\n")
}
