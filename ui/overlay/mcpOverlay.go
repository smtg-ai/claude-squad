package overlay

import (
	"claude-squad/config"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MCPItem represents an MCP server configuration item in the list
type MCPItem struct {
	name   string
	config config.MCPServerConfig
}

func (i MCPItem) FilterValue() string { return i.name }
func (i MCPItem) Title() string       { return i.name }
func (i MCPItem) Description() string {
	desc := fmt.Sprintf("Command: %s", i.config.Command)
	if len(i.config.Args) > 0 {
		desc += fmt.Sprintf(" %s", strings.Join(i.config.Args, " "))
	}
	return desc
}

// MCPOverlayMode represents the current mode of the overlay
type MCPOverlayMode int

const (
	MCPModeList MCPOverlayMode = iota
	MCPModeAdd
	MCPModeEdit
	MCPModeDelete
)

// MCPOverlay represents an MCP management overlay
type MCPOverlay struct {
	config        *config.Config
	list          list.Model
	mode          MCPOverlayMode
	nameInput     textinput.Model
	commandInput  textinput.Model
	argsInput     textinput.Model
	envInput      textinput.Model
	focusIndex    int
	editingName   string
	submitted     bool
	canceled      bool
	width, height int
}

// NewMCPOverlay creates a new MCP management overlay
func NewMCPOverlay() *MCPOverlay {
	cfg := config.LoadConfig()

	// Create list items from current MCP servers
	items := make([]list.Item, 0, len(cfg.MCPServers))
	for name, mcpConfig := range cfg.MCPServers {
		items = append(items, MCPItem{name: name, config: mcpConfig})
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "MCP Servers"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)

	// Create text inputs for adding/editing
	nameInput := textinput.New()
	nameInput.Placeholder = "MCP server name..."
	nameInput.CharLimit = 50

	commandInput := textinput.New()
	commandInput.Placeholder = "Command (e.g., npx @modelcontextprotocol/server-github)"
	commandInput.CharLimit = 200

	argsInput := textinput.New()
	argsInput.Placeholder = "Arguments (space-separated, optional)"
	argsInput.CharLimit = 500

	envInput := textinput.New()
	envInput.Placeholder = "Environment variables (KEY=VALUE, space-separated, optional)"
	envInput.CharLimit = 500

	return &MCPOverlay{
		config:       cfg,
		list:         l,
		mode:         MCPModeList,
		nameInput:    nameInput,
		commandInput: commandInput,
		argsInput:    argsInput,
		envInput:     envInput,
		focusIndex:   0,
	}
}

func (m *MCPOverlay) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetWidth(width - 4)
	m.list.SetHeight(height - 8)

	inputWidth := width - 10
	m.nameInput.Width = inputWidth
	m.commandInput.Width = inputWidth
	m.argsInput.Width = inputWidth
	m.envInput.Width = inputWidth
}

// Init initializes the MCP overlay model
func (m *MCPOverlay) Init() tea.Cmd {
	return nil
}

// View renders the overlay
func (m *MCPOverlay) View() string {
	return m.Render()
}

// HandleKeyPress processes key presses based on current mode
func (m *MCPOverlay) HandleKeyPress(msg tea.KeyMsg) bool {
	switch m.mode {
	case MCPModeList:
		return m.handleListKeys(msg)
	case MCPModeAdd, MCPModeEdit:
		return m.handleFormKeys(msg)
	case MCPModeDelete:
		return m.handleDeleteKeys(msg)
	}
	return false
}

func (m *MCPOverlay) handleListKeys(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc", "q":
		m.canceled = true
		return true
	case "a":
		m.mode = MCPModeAdd
		m.clearInputs()
		m.focusIndex = 0
		m.nameInput.Focus()
		return false
	case "e":
		if len(m.list.Items()) > 0 {
			item := m.list.SelectedItem().(MCPItem)
			m.mode = MCPModeEdit
			m.editingName = item.name
			m.loadItemIntoInputs(item)
			m.focusIndex = 0
			m.nameInput.Focus()
		}
		return false
	case "d":
		if len(m.list.Items()) > 0 {
			m.mode = MCPModeDelete
		}
		return false
	default:
		m.list, _ = m.list.Update(msg)
		return false
	}
}

func (m *MCPOverlay) handleFormKeys(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc":
		m.mode = MCPModeList
		m.refocusList()
		return false
	case "tab":
		m.nextInput()
		return false
	case "shift+tab":
		m.prevInput()
		return false
	case "enter":
		if m.focusIndex == 4 { // Submit button focused
			m.submitForm()
			return false
		}
		fallthrough
	default:
		m.updateCurrentInput(msg)
		return false
	}
}

func (m *MCPOverlay) handleDeleteKeys(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc", "n":
		m.mode = MCPModeList
		return false
	case "y":
		m.deleteSelectedItem()
		m.mode = MCPModeList
		return false
	}
	return false
}

func (m *MCPOverlay) nextInput() {
	m.blurCurrentInput()
	m.focusIndex = (m.focusIndex + 1) % 5 // 4 inputs + submit button
	m.focusCurrentInput()
}

func (m *MCPOverlay) prevInput() {
	m.blurCurrentInput()
	m.focusIndex = (m.focusIndex - 1 + 5) % 5
	m.focusCurrentInput()
}

func (m *MCPOverlay) focusCurrentInput() {
	switch m.focusIndex {
	case 0:
		m.nameInput.Focus()
	case 1:
		m.commandInput.Focus()
	case 2:
		m.argsInput.Focus()
	case 3:
		m.envInput.Focus()
	}
}

func (m *MCPOverlay) blurCurrentInput() {
	m.nameInput.Blur()
	m.commandInput.Blur()
	m.argsInput.Blur()
	m.envInput.Blur()
}

func (m *MCPOverlay) updateCurrentInput(msg tea.KeyMsg) {
	switch m.focusIndex {
	case 0:
		m.nameInput, _ = m.nameInput.Update(msg)
	case 1:
		m.commandInput, _ = m.commandInput.Update(msg)
	case 2:
		m.argsInput, _ = m.argsInput.Update(msg)
	case 3:
		m.envInput, _ = m.envInput.Update(msg)
	}
}

func (m *MCPOverlay) clearInputs() {
	m.nameInput.SetValue("")
	m.commandInput.SetValue("")
	m.argsInput.SetValue("")
	m.envInput.SetValue("")
	m.editingName = ""
}

func (m *MCPOverlay) loadItemIntoInputs(item MCPItem) {
	m.nameInput.SetValue(item.name)
	m.commandInput.SetValue(item.config.Command)
	m.argsInput.SetValue(strings.Join(item.config.Args, " "))

	var envPairs []string
	for k, v := range item.config.Env {
		envPairs = append(envPairs, fmt.Sprintf("%s=%s", k, v))
	}
	m.envInput.SetValue(strings.Join(envPairs, " "))
}

func (m *MCPOverlay) submitForm() {
	name := strings.TrimSpace(m.nameInput.Value())
	command := strings.TrimSpace(m.commandInput.Value())

	if name == "" || command == "" {
		return // Validation failed
	}

	// Parse arguments
	var args []string
	if argsStr := strings.TrimSpace(m.argsInput.Value()); argsStr != "" {
		args = strings.Fields(argsStr)
	}

	// Parse environment variables
	env := make(map[string]string)
	if envStr := strings.TrimSpace(m.envInput.Value()); envStr != "" {
		envPairs := strings.Fields(envStr)
		for _, pair := range envPairs {
			if parts := strings.SplitN(pair, "=", 2); len(parts) == 2 {
				env[parts[0]] = parts[1]
			}
		}
	}

	// Create MCP server config
	mcpConfig := config.MCPServerConfig{
		Command: command,
		Args:    args,
		Env:     env,
	}

	// Remove old entry if editing
	if m.mode == MCPModeEdit && m.editingName != "" && m.editingName != name {
		delete(m.config.MCPServers, m.editingName)
	}

	// Add/update the configuration
	m.config.MCPServers[name] = mcpConfig

	// Save configuration
	if err := config.SaveConfig(m.config); err != nil {
		// Handle error (could show in overlay)
		return
	}

	// Update list
	m.refreshList()
	m.mode = MCPModeList
	m.refocusList()
}

func (m *MCPOverlay) deleteSelectedItem() {
	if len(m.list.Items()) == 0 {
		return
	}

	item := m.list.SelectedItem().(MCPItem)
	delete(m.config.MCPServers, item.name)

	// Save configuration
	config.SaveConfig(m.config)

	// Update list
	m.refreshList()
}

func (m *MCPOverlay) refreshList() {
	items := make([]list.Item, 0, len(m.config.MCPServers))
	for name, mcpConfig := range m.config.MCPServers {
		items = append(items, MCPItem{name: name, config: mcpConfig})
	}
	m.list.SetItems(items)
}

func (m *MCPOverlay) refocusList() {
	// Focus back on the list
	m.blurCurrentInput()
}

// IsSubmitted returns whether the overlay was submitted
func (m *MCPOverlay) IsSubmitted() bool {
	return m.submitted
}

// IsCanceled returns whether the overlay was canceled
func (m *MCPOverlay) IsCanceled() bool {
	return m.canceled
}

// Render renders the MCP overlay
func (m *MCPOverlay) Render() string {
	switch m.mode {
	case MCPModeList:
		return m.renderList()
	case MCPModeAdd:
		return m.renderForm("Add MCP Server")
	case MCPModeEdit:
		return m.renderForm("Edit MCP Server")
	case MCPModeDelete:
		return m.renderDeleteConfirmation()
	}
	return ""
}

func (m *MCPOverlay) renderList() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	helpText := "Commands: (a)dd, (e)dit, (d)elete, (q)uit/esc"
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	content := m.list.View() + "\n" + helpStyle.Render(helpText)
	return style.Render(content)
}

func (m *MCPOverlay) renderForm(title string) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("62")).
		Bold(true).
		MarginBottom(1)

	buttonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7")).
		Padding(0, 1)

	focusedButtonStyle := buttonStyle.Copy().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("0"))

	content := titleStyle.Render(title) + "\n\n"
	content += "Name:\n" + m.nameInput.View() + "\n\n"
	content += "Command:\n" + m.commandInput.View() + "\n\n"
	content += "Arguments:\n" + m.argsInput.View() + "\n\n"
	content += "Environment:\n" + m.envInput.View() + "\n\n"

	// Submit button
	submitButton := " Submit "
	if m.focusIndex == 4 {
		submitButton = focusedButtonStyle.Render(submitButton)
	} else {
		submitButton = buttonStyle.Render(submitButton)
	}
	content += submitButton

	helpText := "\nTab/Shift+Tab: Navigate, Enter: Submit, Esc: Cancel"
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)
	content += helpStyle.Render(helpText)

	return style.Render(content)
}

func (m *MCPOverlay) renderDeleteConfirmation() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true).
		MarginBottom(1)

	if len(m.list.Items()) == 0 {
		return style.Render("No MCP servers to delete")
	}

	item := m.list.SelectedItem().(MCPItem)
	content := titleStyle.Render("Delete MCP Server") + "\n\n"
	content += fmt.Sprintf("Are you sure you want to delete '%s'?\n\n", item.name)
	content += "Press 'y' to confirm, 'n' or 'esc' to cancel"

	return style.Render(content)
}
