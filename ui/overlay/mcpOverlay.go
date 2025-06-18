package overlay

import (
	"claude-squad/config"
	"claude-squad/session"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MCPAssignmentItem represents an MCP server with assignment status
type MCPAssignmentItem struct {
	name     string
	config   config.MCPServerConfig
	assigned bool
}

func (i MCPAssignmentItem) FilterValue() string { return i.name }
func (i MCPAssignmentItem) Title() string {
	title := i.name
	if i.assigned {
		title = "✓ " + title
	} else {
		title = "  " + title
	}
	return title
}
func (i MCPAssignmentItem) Description() string {
	// Compact view - no description for cleaner UI
	return ""
}

// MCPOverlay represents a worktree-specific MCP assignment overlay
type MCPOverlay struct {
	config              *config.Config
	instance            *session.Instance
	worktreePath        string
	submitted           bool
	canceled            bool
	width, height       int
	assignments         map[string]bool // MCP name -> assigned status
	originalAssignments map[string]bool // Original assignments to detect changes
	assignmentsChanged  bool            // Flag to track if assignments have changed

	// Grid navigation
	mcpNames       []string // Sorted list of MCP names
	filteredNames  []string // Filtered list for search
	selectedRow    int
	selectedCol    int
	columns        int
	itemsPerColumn int

	// Search functionality
	searchMode  bool
	searchQuery string
}

// NewMCPOverlay creates a new MCP assignment overlay for a specific instance
func NewMCPOverlay(instance *session.Instance) *MCPOverlay {
	cfg := config.LoadConfig()

	var worktreePath string
	if instance != nil && instance.Started() {
		if worktree, err := instance.GetGitWorktree(); err == nil {
			worktreePath = worktree.GetWorktreePath()
		}
	}

	// Get currently assigned MCPs for this worktree
	assignedMCPs := cfg.GetWorktreeMCPs(worktreePath)
	assignments := make(map[string]bool)
	originalAssignments := make(map[string]bool)
	for _, mcpName := range assignedMCPs {
		assignments[mcpName] = true
		originalAssignments[mcpName] = true
	}

	// Create sorted list of MCP names for consistent ordering
	var mcpNames []string
	for name := range cfg.MCPServers {
		mcpNames = append(mcpNames, name)
	}
	sort.Strings(mcpNames)

	overlay := &MCPOverlay{
		config:              cfg,
		instance:            instance,
		worktreePath:        worktreePath,
		assignments:         assignments,
		originalAssignments: originalAssignments,
		assignmentsChanged:  false,
		mcpNames:            mcpNames,
		filteredNames:       mcpNames, // Initially show all
		selectedRow:         0,
		selectedCol:         0,
		columns:             3,
		searchMode:          false,
		searchQuery:         "",
	}

	// Calculate items per column based on total items and columns
	totalItems := len(mcpNames)
	overlay.itemsPerColumn = (totalItems + overlay.columns - 1) / overlay.columns

	return overlay
}

func (m *MCPOverlay) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Init initializes the MCP overlay model
func (m *MCPOverlay) Init() tea.Cmd {
	return nil
}

// View renders the overlay
func (m *MCPOverlay) View() string {
	return m.Render()
}

// HandleKeyPress processes key presses
func (m *MCPOverlay) HandleKeyPress(msg tea.KeyMsg) bool {
	// Handle search mode
	if m.searchMode {
		switch msg.Type {
		case tea.KeyEsc:
			// Exit search mode and clear filter
			m.searchMode = false
			m.searchQuery = ""
			m.filteredNames = m.mcpNames
			m.selectedRow = 0
			m.selectedCol = 0
			return false
		case tea.KeyEnter:
			// Exit search mode but keep filter
			m.searchMode = false
			return false
		case tea.KeyBackspace:
			if len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				m.filterMCPs()
			}
			return false
		case tea.KeyRunes:
			m.searchQuery += string(msg.Runes)
			m.filterMCPs()
			return false
		default:
			return false
		}
	}

	// Normal navigation mode
	switch msg.String() {
	case "esc", "q":
		m.canceled = true
		return true
	case "/":
		// Enter search mode
		m.searchMode = true
		return false
	case " ", "enter":
		// Toggle assignment for selected item
		if m.getSelectedMCPName() != "" {
			m.toggleAssignment(m.getSelectedMCPName())
		}
		return false
	case "s":
		// Save assignments
		m.saveAssignments()
		m.submitted = true
		return true
	case "up", "k":
		m.moveUp()
		return false
	case "down", "j":
		m.moveDown()
		return false
	case "left", "h":
		m.moveLeft()
		return false
	case "right", "l":
		m.moveRight()
		return false
	default:
		return false
	}
}

// toggleAssignment toggles the assignment status of an MCP
func (m *MCPOverlay) toggleAssignment(mcpName string) {
	m.assignments[mcpName] = !m.assignments[mcpName]
	m.checkForChanges()
}

// checkForChanges compares current assignments with original assignments to detect changes
func (m *MCPOverlay) checkForChanges() {
	// Check if the number of assigned MCPs has changed
	originalCount := 0
	currentCount := 0

	for _, assigned := range m.originalAssignments {
		if assigned {
			originalCount++
		}
	}

	for _, assigned := range m.assignments {
		if assigned {
			currentCount++
		}
	}

	// If counts differ, assignments have definitely changed
	if originalCount != currentCount {
		m.assignmentsChanged = true
		return
	}

	// If counts are the same, check if the specific assignments match
	for mcpName, originalAssigned := range m.originalAssignments {
		currentAssigned := m.assignments[mcpName]
		if originalAssigned != currentAssigned {
			m.assignmentsChanged = true
			return
		}
	}

	// Also check for new MCPs that weren't in original assignments
	for mcpName, currentAssigned := range m.assignments {
		_, existedBefore := m.originalAssignments[mcpName]
		if !existedBefore && currentAssigned {
			m.assignmentsChanged = true
			return
		}
	}

	m.assignmentsChanged = false
}

// filterMCPs filters the MCP list based on search query
func (m *MCPOverlay) filterMCPs() {
	if m.searchQuery == "" {
		m.filteredNames = m.mcpNames
	} else {
		var filtered []string
		query := strings.ToLower(m.searchQuery)
		for _, name := range m.mcpNames {
			if strings.Contains(strings.ToLower(name), query) {
				filtered = append(filtered, name)
			}
		}
		m.filteredNames = filtered
	}
	// Reset selection to first item
	m.selectedRow = 0
	m.selectedCol = 0
}

// getSelectedMCPName returns the currently selected MCP name
func (m *MCPOverlay) getSelectedMCPName() string {
	// Convert grid position to alphabetical index
	itemsPerColumn := (len(m.filteredNames) + m.columns - 1) / m.columns
	index := m.selectedCol*itemsPerColumn + m.selectedRow
	if index >= 0 && index < len(m.filteredNames) {
		return m.filteredNames[index]
	}
	return ""
}

// moveUp moves selection up in the grid
func (m *MCPOverlay) moveUp() {
	if m.selectedRow > 0 {
		m.selectedRow--
	}
}

// moveDown moves selection down in the grid
func (m *MCPOverlay) moveDown() {
	itemsPerColumn := (len(m.filteredNames) + m.columns - 1) / m.columns
	if m.selectedRow < itemsPerColumn-1 {
		newIndex := m.selectedCol*itemsPerColumn + (m.selectedRow + 1)
		if newIndex < len(m.filteredNames) {
			m.selectedRow++
		}
	}
}

// moveLeft moves selection left in the grid
func (m *MCPOverlay) moveLeft() {
	if m.selectedCol > 0 {
		m.selectedCol--
	}
}

// moveRight moves selection right in the grid
func (m *MCPOverlay) moveRight() {
	if m.selectedCol < m.columns-1 {
		itemsPerColumn := (len(m.filteredNames) + m.columns - 1) / m.columns
		newIndex := (m.selectedCol+1)*itemsPerColumn + m.selectedRow
		if newIndex < len(m.filteredNames) {
			m.selectedCol++
		}
	}
}

// saveAssignments saves the current assignments to the config
func (m *MCPOverlay) saveAssignments() {
	if m.worktreePath == "" {
		return // Cannot save without worktree path
	}

	// Build list of assigned MCPs
	var assignedMCPs []string
	for mcpName, assigned := range m.assignments {
		if assigned {
			assignedMCPs = append(assignedMCPs, mcpName)
		}
	}

	// Update config
	m.config.SetWorktreeMCPs(m.worktreePath, assignedMCPs)
	config.SaveConfig(m.config)
}

// IsSubmitted returns whether the overlay was submitted
func (m *MCPOverlay) IsSubmitted() bool {
	return m.submitted
}

// IsCanceled returns whether the overlay was canceled
func (m *MCPOverlay) IsCanceled() bool {
	return m.canceled
}

// AssignmentsChanged returns whether the MCP assignments have changed from original
func (m *MCPOverlay) AssignmentsChanged() bool {
	return m.assignmentsChanged
}

// GetInstance returns the instance associated with this overlay
func (m *MCPOverlay) GetInstance() *session.Instance {
	return m.instance
}

// Render renders the MCP assignment overlay
func (m *MCPOverlay) Render() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	var content string

	if len(m.config.MCPServers) == 0 {
		content = "No MCP servers configured.\n\nMCP servers should be configured in your Claude configuration.\nThis overlay allows you to assign existing MCPs to worktrees."
		helpText := "\nPress 'q' or 'esc' to cancel"
		helpStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
		content += helpStyle.Render(helpText)
	} else {
		// Create header with shorter worktree path
		headerText := "Select MCPs"
		if m.worktreePath != "" {
			// Extract just the worktree name (last part of path)
			parts := strings.Split(m.worktreePath, "/")
			worktreeName := parts[len(parts)-1]
			headerText = fmt.Sprintf("Select MCPs: %s", worktreeName)
		}

		content = headerText + "\n"

		// Show search info if in search mode or filter is active
		if m.searchMode {
			searchText := fmt.Sprintf("Search: %s_", m.searchQuery)
			searchStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("33"))
			content += searchStyle.Render(searchText) + "\n"
		} else if m.searchQuery != "" {
			filterText := fmt.Sprintf("Filter: %s [Showing %d of %d]", m.searchQuery, len(m.filteredNames), len(m.mcpNames))
			filterStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))
			content += filterStyle.Render(filterText) + "\n"
		}

		content += "\n" + m.renderGrid()

		// Show assignment summary
		assignedCount := 0
		for _, assigned := range m.assignments {
			if assigned {
				assignedCount++
			}
		}

		summaryText := fmt.Sprintf("\nAssigned MCPs: %d of %d", assignedCount, len(m.mcpNames))
		summaryStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("33")).
			MarginTop(1)
		content += summaryStyle.Render(summaryText)

		// Dynamic help text based on mode
		var helpText string
		if m.searchMode {
			helpText = "\nSearch: (esc)clear filter, (enter)keep filter"
		} else {
			helpText = "\nCommands: (/)search, (space)toggle, (s)ave, (q)uit/esc"
		}
		helpStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
		content += helpStyle.Render(helpText)
	}

	return style.Render(content)
}

// renderGrid renders the 3-column grid of MCPs
func (m *MCPOverlay) renderGrid() string {
	if len(m.filteredNames) == 0 {
		return "No MCPs match your search"
	}

	var rows []string
	columnWidth := 35 // ~35 characters per column for full names

	// Calculate number of rows needed - fill columns first for proper alphabetical order
	itemsPerColumn := (len(m.filteredNames) + m.columns - 1) / m.columns
	numRows := itemsPerColumn

	for row := 0; row < numRows; row++ {
		var columns []string

		for col := 0; col < m.columns; col++ {
			// For proper alphabetical order: fill columns first
			index := col*itemsPerColumn + row
			if index >= len(m.filteredNames) {
				// Empty cell
				columns = append(columns, strings.Repeat(" ", columnWidth))
				continue
			}

			mcpName := m.filteredNames[index]
			assigned := m.assignments[mcpName]

			// Check if this is the selected cell
			selected := (row == m.selectedRow && col == m.selectedCol)

			// Format the cell
			var cellContent string
			if assigned {
				cellContent = "✓ " + mcpName
			} else {
				cellContent = "  " + mcpName
			}

			// Truncate if too long
			if len(cellContent) > columnWidth {
				cellContent = cellContent[:columnWidth-3] + "..."
			}

			// Pad to column width
			cellContent += strings.Repeat(" ", columnWidth-len(cellContent))

			// Apply selection styling
			if selected {
				cellStyle := lipgloss.NewStyle().
					Background(lipgloss.Color("62")).
					Foreground(lipgloss.Color("15"))
				cellContent = cellStyle.Render(cellContent)
			}

			columns = append(columns, cellContent)
		}

		rows = append(rows, strings.Join(columns, ""))
	}

	return strings.Join(rows, "\n")
}
