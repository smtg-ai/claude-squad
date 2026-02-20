package ui

import (
	"strings"

	"github.com/ByteMirror/hivemind/keys"

	"github.com/ByteMirror/hivemind/session"

	"github.com/charmbracelet/lipgloss"
)

var keyStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
	Light: "#655F5F",
	Dark:  "#7F7A7A",
})

var descStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
	Light: "#7A7474",
	Dark:  "#9C9494",
})

var sepStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
	Light: "#DDDADA",
	Dark:  "#3C3C3C",
})

var actionGroupStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("216"))

var separator = " • "
var verticalSeparator = " │ "

var menuStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#7EC8D8"))

// MenuState represents different states the menu can be in
type MenuState int

const (
	StateDefault MenuState = iota
	StateEmpty
	StateNewInstance
	StatePrompt
)

// menuGroup is a logical group of hotkeys rendered together, separated from
// other groups by a vertical bar.
type menuGroup struct {
	keys     []keys.KeyName
	isAction bool // action groups get a distinct highlight color
}

// menuRow is one horizontal line in the footer, composed of one or more groups.
type menuRow []menuGroup

type Menu struct {
	rows          []menuRow
	height, width int
	state         MenuState
	instance      *session.Instance
	isInDiffTab   bool

	// keyDown is the key which is pressed. The default is -1.
	keyDown keys.KeyName
}

func NewMenu() *Menu {
	m := &Menu{
		state:       StateEmpty,
		isInDiffTab: false,
		keyDown:     -1,
	}
	m.updateOptions()
	return m
}

func (m *Menu) Keydown(name keys.KeyName) {
	m.keyDown = name
}

func (m *Menu) ClearKeydown() {
	m.keyDown = -1
}

// SetState updates the menu state and options accordingly
func (m *Menu) SetState(state MenuState) {
	m.state = state
	m.updateOptions()
}

// SetInstance updates the current instance and refreshes menu options
func (m *Menu) SetInstance(instance *session.Instance) {
	m.instance = instance
	if m.state != StateNewInstance && m.state != StatePrompt {
		if m.instance != nil {
			m.state = StateDefault
		} else {
			m.state = StateEmpty
		}
	}
	m.updateOptions()
}

// SetInDiffTab updates whether we're currently in the diff tab
func (m *Menu) SetInDiffTab(inDiffTab bool) {
	m.isInDiffTab = inDiffTab
	m.updateOptions()
}

func (m *Menu) updateOptions() {
	switch m.state {
	case StateEmpty:
		m.rows = []menuRow{
			// Row 1: primary actions
			{
				menuGroup{keys: []keys.KeyName{keys.KeyNew, keys.KeyPrompt}, isAction: true},
			},
			// Row 2: system
			{
				menuGroup{keys: []keys.KeyName{keys.KeySearch, keys.KeySpace, keys.KeyRepoSwitch}},
				menuGroup{keys: []keys.KeyName{keys.KeyHelp, keys.KeyQuit}},
			},
		}
	case StateDefault:
		if m.instance != nil {
			m.buildInstanceRows()
		} else {
			m.rows = []menuRow{
				{
					menuGroup{keys: []keys.KeyName{keys.KeyNew, keys.KeyPrompt}, isAction: true},
				},
				{
					menuGroup{keys: []keys.KeyName{keys.KeySearch, keys.KeySpace, keys.KeyRepoSwitch}},
					menuGroup{keys: []keys.KeyName{keys.KeyHelp, keys.KeyQuit}},
				},
			}
		}
	case StateNewInstance, StatePrompt:
		m.rows = []menuRow{
			{menuGroup{keys: []keys.KeyName{keys.KeySubmitName}}},
		}
	}
}

func (m *Menu) buildInstanceRows() {
	// Row 1: Sessions + Actions (the things you do)
	sessionGroup := menuGroup{keys: []keys.KeyName{keys.KeyNew, keys.KeyKill, keys.KeyAutoYes}}

	actionKeys := []keys.KeyName{keys.KeyEnter, keys.KeySendPrompt, keys.KeySpace}
	actionGroup := menuGroup{keys: actionKeys, isAction: true}

	gitKeys := []keys.KeyName{keys.KeySubmit, keys.KeyCreatePR}
	if m.instance.Status == session.Paused {
		gitKeys = append(gitKeys, keys.KeyResume)
	} else {
		gitKeys = append(gitKeys, keys.KeyCheckout)
	}
	gitGroup := menuGroup{keys: gitKeys, isAction: true}

	// Row 2: Navigation + System
	navKeys := []keys.KeyName{keys.KeyShiftLeft, keys.KeyShiftUp, keys.KeySearch, keys.KeyRepoSwitch}
	navGroup := menuGroup{keys: navKeys}

	sysGroup := menuGroup{keys: []keys.KeyName{keys.KeyKillAllInTopic, keys.KeyHelp, keys.KeyQuit}}

	m.rows = []menuRow{
		{sessionGroup, actionGroup, gitGroup},
		{navGroup, sysGroup},
	}
}

// SetSize sets the width of the window. The menu will be centered horizontally within this width.
func (m *Menu) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// renderRow renders a single row of grouped hotkeys into a styled string.
func (m *Menu) renderRow(row menuRow) string {
	var s strings.Builder

	for gi, group := range row {
		for ki, k := range group.keys {
			binding := keys.GlobalkeyBindings[k]

			localActionStyle := actionGroupStyle
			localKeyStyle := keyStyle
			localDescStyle := descStyle
			if m.keyDown == k {
				localActionStyle = localActionStyle.Underline(true)
				localKeyStyle = localKeyStyle.Underline(true)
				localDescStyle = localDescStyle.Underline(true)
			}

			if group.isAction {
				s.WriteString(localActionStyle.Render(binding.Help().Key))
				s.WriteString(" ")
				s.WriteString(localActionStyle.Render(binding.Help().Desc))
			} else {
				s.WriteString(localKeyStyle.Render(binding.Help().Key))
				s.WriteString(" ")
				s.WriteString(localDescStyle.Render(binding.Help().Desc))
			}

			// Separator within a group
			if ki < len(group.keys)-1 {
				s.WriteString(sepStyle.Render(separator))
			}
		}

		// Separator between groups
		if gi < len(row)-1 {
			s.WriteString(sepStyle.Render(verticalSeparator))
		}
	}

	return s.String()
}

func (m *Menu) String() string {
	var renderedRows []string
	for _, row := range m.rows {
		renderedRows = append(renderedRows, menuStyle.Render(m.renderRow(row)))
	}

	joined := lipgloss.JoinVertical(lipgloss.Center, renderedRows...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, joined)
}
