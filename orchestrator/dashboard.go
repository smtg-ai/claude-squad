package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)

	statusStyle = map[string]lipgloss.Style{
		StatusPending: lipgloss.NewStyle().
			Foreground(lipgloss.Color("yellow")),
		StatusRunning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("cyan")).
			Bold(true),
		StatusCompleted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("green")),
		StatusFailed: lipgloss.NewStyle().
			Foreground(lipgloss.Color("red")),
	}

	metricStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// DashboardModel represents the TUI dashboard for monitoring agent execution
type DashboardModel struct {
	client   *Client
	table    table.Model
	width    int
	height   int
	lastSync time.Time
	err      error
}

// NewDashboard creates a new dashboard model
func NewDashboard(client *Client) DashboardModel {
	columns := []table.Column{
		{Title: "Metric", Width: 30},
		{Title: "Value", Width: 20},
		{Title: "Details", Width: 50},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(false),
		table.WithHeight(15),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return DashboardModel{
		client: client,
		table:  t,
	}
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m DashboardModel) Init() tea.Cmd {
	return tickCmd()
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		// Refresh analytics
		if err := m.refreshAnalytics(); err != nil {
			m.err = err
		}
		m.lastSync = time.Now()
		return m, tickCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *DashboardModel) refreshAnalytics() error {
	analytics, err := m.client.GetAnalytics()
	if err != nil {
		return err
	}

	rows := []table.Row{
		{"Total Tasks", fmt.Sprintf("%d", analytics.TotalTasks), "All tasks in the system"},
		{"Running Tasks", m.coloredStatus(StatusRunning, analytics.RunningCount),
			fmt.Sprintf("Currently executing (%d/%d slots)", analytics.RunningCount, analytics.MaxConcurrent)},
		{"Available Slots", m.coloredMetric(analytics.AvailableSlots),
			fmt.Sprintf("Ready to execute %d more tasks", analytics.AvailableSlots)},
		{"Pending Tasks", m.coloredStatus(StatusPending, analytics.StatusCounts[StatusPending]),
			"Waiting for dependencies or slots"},
		{"Completed Tasks", m.coloredStatus(StatusCompleted, analytics.StatusCounts[StatusCompleted]),
			"Successfully finished"},
		{"Failed Tasks", m.coloredStatus(StatusFailed, analytics.StatusCounts[StatusFailed]),
			"Execution errors"},
		{"Max Concurrency", fmt.Sprintf("%d", analytics.MaxConcurrent),
			"Maximum parallel agents"},
		{"Utilization", m.utilizationBar(analytics),
			fmt.Sprintf("%.1f%% capacity used", m.calculateUtilization(analytics))},
	}

	m.table.SetRows(rows)
	return nil
}

func (m *DashboardModel) coloredStatus(status string, count int) string {
	style, ok := statusStyle[status]
	if !ok {
		return fmt.Sprintf("%d", count)
	}
	return style.Render(fmt.Sprintf("%d", count))
}

func (m *DashboardModel) coloredMetric(value int) string {
	return metricStyle.Render(fmt.Sprintf("%d", value))
}

func (m *DashboardModel) calculateUtilization(analytics *Analytics) float64 {
	if analytics.MaxConcurrent == 0 {
		return 0
	}
	return float64(analytics.RunningCount) / float64(analytics.MaxConcurrent) * 100
}

func (m *DashboardModel) utilizationBar(analytics *Analytics) string {
	if analytics.MaxConcurrent == 0 {
		return ""
	}

	utilization := m.calculateUtilization(analytics)
	barWidth := 30
	filledWidth := int(float64(barWidth) * utilization / 100)

	filled := strings.Repeat("█", filledWidth)
	empty := strings.Repeat("░", barWidth-filledWidth)

	var barStyle lipgloss.Style
	if utilization >= 90 {
		barStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("red"))
	} else if utilization >= 70 {
		barStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("yellow"))
	} else {
		barStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("green"))
	}

	return barStyle.Render(filled + empty)
}

func (m DashboardModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("🤖 Oxigraph Agent Orchestrator Dashboard"))
	b.WriteString("\n\n")

	// Error display
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("red")).
			Bold(true)
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
	}

	// Table
	b.WriteString(baseStyle.Render(m.table.View()))
	b.WriteString("\n\n")

	// Footer
	footer := labelStyle.Render(fmt.Sprintf(
		"Last sync: %s | Press 'q' to quit",
		m.lastSync.Format("15:04:05"),
	))
	b.WriteString(footer)

	return b.String()
}

// RunDashboard starts the TUI dashboard
func RunDashboard(ctx context.Context, orchestratorURL string) error {
	client := NewClient(orchestratorURL)

	if err := client.Health(); err != nil {
		return fmt.Errorf("orchestrator service not healthy: %w", err)
	}

	m := NewDashboard(client)

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run dashboard: %w", err)
	}

	return nil
}
