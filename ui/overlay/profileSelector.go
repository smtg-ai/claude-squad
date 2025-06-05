package overlay

import (
	"claude-squad/config"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	teav1 "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const listHeight = 14

var (
	profileTitleStyle        = lipgloss.NewStyle().MarginLeft(2)
	profileItemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	profileSelectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	profilePaginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	profileHelpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	profileQuitTextStyle     = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

type ProfileItem struct {
	name    string
	profile config.Profile
}

func (i ProfileItem) Title() string       { return i.name }
func (i ProfileItem) Description() string { return i.profile.Command }
func (i ProfileItem) FilterValue() string { return i.name }

type ProfileSelector struct {
	list            list.Model
	choice          string
	selectedProfile config.Profile
	quitting        bool
	submitted       bool
	width, height   int
	hasProfiles     bool
}

func NewProfileSelector(profiles map[string]config.Profile) *ProfileSelector {
	hasProfiles := len(profiles) > 0

	items := make([]list.Item, 0, len(profiles))
	for name, profile := range profiles {
		items = append(items, ProfileItem{
			name:    name,
			profile: profile,
		})
	}

	const defaultWidth = 20

	l := list.New(items, list.NewDefaultDelegate(), defaultWidth, listHeight)
	l.Title = "Choose AI Assistant Profile"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = profileTitleStyle
	l.Styles.PaginationStyle = profilePaginationStyle
	l.Styles.HelpStyle = profileHelpStyle

	return &ProfileSelector{
		list:        l,
		hasProfiles: hasProfiles,
	}
}

func (m *ProfileSelector) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *ProfileSelector) Init() tea.Cmd {
	return nil
}

func (m *ProfileSelector) Update(msg tea.Msg) (*ProfileSelector, tea.Cmd) {
	if !m.hasProfiles {
		switch msg.(type) {
		case tea.KeyPressMsg:
			m.quitting = true
			return m, nil
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyPressMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(ProfileItem)
			if ok {
				m.choice = i.name
				m.selectedProfile = i.profile
				m.submitted = true
			}
			return m, nil

		case "esc":
			m.quitting = true
			return m, nil
		}
	}

	var cmdv1 teav1.Cmd
	m.list, cmdv1 = m.list.Update(msg)
	if cmdv1 != nil {
		return m, func() tea.Msg { return cmdv1() }
	}
	return m, nil
}

func (m *ProfileSelector) View() string {
	return m.Render()
}

func (m *ProfileSelector) Render() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(m.width)

	var content string
	if !m.hasProfiles {
		content += "No AI assistant profiles have been configured.\n"
		content += "Please add profiles to your config file first.\n\n"
		content += "Press any key to cancel..."
	} else if m.quitting {
		content = "Selection cancelled."
	} else {
		content += m.list.View()
	}

	return style.Render(content)
}

func (m *ProfileSelector) IsSubmitted() bool {
	return m.submitted
}

func (m *ProfileSelector) IsQuitting() bool {
	return m.quitting
}

func (m *ProfileSelector) GetSelectedProfile() config.Profile {
	return m.selectedProfile
}

func (m *ProfileSelector) GetSelectedName() string {
	return m.choice
}
