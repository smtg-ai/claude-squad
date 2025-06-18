package overlay

import (
	"claude-squad/config"
	"claude-squad/session"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
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
	desc := fmt.Sprintf("Command: %s", i.config.Command)
	if len(i.config.Args) > 0 {
		desc += fmt.Sprintf(" %s", strings.Join(i.config.Args, " "))
	}
	if i.assigned {
		desc += " (assigned to this worktree)"
	}
	return desc
}

// MCPOverlay represents a worktree-specific MCP assignment overlay
type MCPOverlay struct {
	config            *config.Config
	list              list.Model
	instance          *session.Instance
	worktreePath      string
	submitted         bool
	canceled          bool
	width, height     int
	assignments       map[string]bool // MCP name -> assigned status
	originalAssignments map[string]bool // Original assignments to detect changes
	assignmentsChanged bool            // Flag to track if assignments have changed
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

	// Create list items from available MCP servers
	items := make([]list.Item, 0, len(cfg.MCPServers))
	for name, mcpConfig := range cfg.MCPServers {
		assigned := assignments[name]
		items = append(items, MCPAssignmentItem{
			name:     name,
			config:   mcpConfig,
			assigned: assigned,
		})
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	
	title := "Select MCPs for Current Worktree"
	if worktreePath != "" {
		title = fmt.Sprintf("Select MCPs for: %s", worktreePath)
	}
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)

	return &MCPOverlay{
		config:              cfg,
		list:                l,
		instance:            instance,
		worktreePath:        worktreePath,
		assignments:         assignments,
		originalAssignments: originalAssignments,
		assignmentsChanged:  false,
	}
}

func (m *MCPOverlay) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetWidth(width - 4)
	m.list.SetHeight(height - 8)
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
	switch msg.String() {
	case "esc", "q":
		m.canceled = true
		return true
	case " ", "enter":
		// Toggle assignment for selected item
		if len(m.list.Items()) > 0 {
			selectedItem := m.list.SelectedItem().(MCPAssignmentItem)
			m.toggleAssignment(selectedItem.name)
			m.refreshList()
		}
		return false
	case "s":
		// Save assignments
		m.saveAssignments()
		m.submitted = true
		return true
	default:
		m.list, _ = m.list.Update(msg)
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

// refreshList updates the list items with current assignment status
func (m *MCPOverlay) refreshList() {
	items := make([]list.Item, 0, len(m.config.MCPServers))
	for name, mcpConfig := range m.config.MCPServers {
		assigned := m.assignments[name]
		items = append(items, MCPAssignmentItem{
			name:     name,
			config:   mcpConfig,
			assigned: assigned,
		})
	}
	selectedIndex := m.list.Index()
	m.list.SetItems(items)
	if selectedIndex < len(items) {
		m.list.Select(selectedIndex)
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
		content = m.list.View()
		
		// Show assignment summary
		assignedCount := 0
		for _, assigned := range m.assignments {
			if assigned {
				assignedCount++
			}
		}
		
		summaryText := fmt.Sprintf("\nAssigned MCPs: %d of %d", assignedCount, len(m.config.MCPServers))
		summaryStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("33")).
			MarginTop(1)
		content += summaryStyle.Render(summaryText)
		
		helpText := "\nCommands: (space)toggle, (s)ave and exit, (q)uit/esc"
		helpStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
		content += helpStyle.Render(helpText)
	}

	return style.Render(content)
}