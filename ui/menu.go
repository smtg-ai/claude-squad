package ui

import (
	"claude-squad/keys"
	"strings"

	"claude-squad/session"

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

var actionGroupStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))

var separator = " • "
var verticalSeparator = " │ "

var menuStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("205"))

// MenuState represents different states the menu can be in
type MenuState int

const (
	StateDefault MenuState = iota
	StateEmpty
	StateNewInstance
	StatePrompt
)

type Menu struct {
	options       []keys.KeyName
	height, width int
	state         MenuState
	instance      *session.Instance
	activeTab     int

	// keyDown is the key which is pressed. The default is -1.
	keyDown keys.KeyName
}

var defaultMenuOptions = []keys.KeyName{
	keys.KeyNew, keys.KeyPrompt,
	keys.KeyAddWorkspace, keys.KeySwitchWorkspace,
	keys.KeyHelp, keys.KeyQuit,
}
var newInstanceMenuOptions = []keys.KeyName{keys.KeySubmitName}
var promptMenuOptions = []keys.KeyName{keys.KeySubmitName}

// menuGroup tags each menu option with the visual group it belongs to so
// String() can insert a vertical separator on group transitions without
// hardcoded index ranges. Order of declaration is the visual order.
type menuGroup int

const (
	menuGroupInstance menuGroup = iota // n / D
	menuGroupAction                    // enter / submit / resume / checkout / shift-arrow
	menuGroupWorkspace                 // A / W / V / z
	menuGroupSystem                    // tab / ? / q
)

// keyMenuGroup is the canonical group for each KeyName. New keys must be added
// here for them to render with the correct separator placement.
var keyMenuGroup = map[keys.KeyName]menuGroup{
	keys.KeyNew:               menuGroupInstance,
	keys.KeyPrompt:             menuGroupInstance,
	keys.KeyKill:               menuGroupInstance,
	keys.KeyEnter:              menuGroupAction,
	keys.KeySubmit:             menuGroupAction,
	keys.KeyResume:             menuGroupAction,
	keys.KeyCheckout:           menuGroupAction,
	keys.KeyShiftUp:            menuGroupAction,
	keys.KeyShiftDown:          menuGroupAction,
	keys.KeyAddWorkspace:       menuGroupWorkspace,
	keys.KeySwitchWorkspace:    menuGroupWorkspace,
	keys.KeyViewFilter:         menuGroupWorkspace,
	keys.KeyCollapseWorkspace:  menuGroupWorkspace,
	keys.KeyTab:                menuGroupSystem,
	keys.KeyHelp:               menuGroupSystem,
	keys.KeyQuit:               menuGroupSystem,
	keys.KeySubmitName:         menuGroupAction,
}

func NewMenu() *Menu {
	return &Menu{
		options:   defaultMenuOptions,
		state:     StateEmpty,
		activeTab: 0,
		keyDown:   -1,
	}
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
	// Only change the state if we're not in a special state (NewInstance or Prompt)
	if m.state != StateNewInstance && m.state != StatePrompt {
		if m.instance != nil {
			m.state = StateDefault
		} else {
			m.state = StateEmpty
		}
	}
	m.updateOptions()
}

// SetActiveTab updates the currently active tab
func (m *Menu) SetActiveTab(tab int) {
	m.activeTab = tab
	m.updateOptions()
}

// updateOptions updates the menu options based on current state and instance
func (m *Menu) updateOptions() {
	switch m.state {
	case StateEmpty:
		m.options = defaultMenuOptions
	case StateDefault:
		if m.instance != nil {
			// When there is an instance, show that instance's options
			m.addInstanceOptions()
		} else {
			// When there is no instance, show the empty state
			m.options = defaultMenuOptions
		}
	case StateNewInstance:
		m.options = newInstanceMenuOptions
	case StatePrompt:
		m.options = promptMenuOptions
	}
}

func (m *Menu) addInstanceOptions() {
	// Loading instances only get minimal options
	if m.instance != nil && m.instance.Status == session.Loading {
		m.options = []keys.KeyName{keys.KeyNew, keys.KeyHelp, keys.KeyQuit}
		return
	}

	// Instance management group
	options := []keys.KeyName{keys.KeyNew, keys.KeyKill}

	// Action group
	actionGroup := []keys.KeyName{keys.KeyEnter, keys.KeySubmit}
	if m.instance.Status == session.Paused {
		actionGroup = append(actionGroup, keys.KeyResume)
	} else {
		actionGroup = append(actionGroup, keys.KeyCheckout)
	}

	// Navigation group (when in diff tab)
	if m.activeTab == DiffTab || m.activeTab == TerminalTab {
		actionGroup = append(actionGroup, keys.KeyShiftUp)
	}

	// Workspace group: workspace-management keys, surfaced in every state so
	// users discover them without opening the full help screen.
	workspaceGroup := []keys.KeyName{keys.KeyAddWorkspace, keys.KeySwitchWorkspace}

	// System group
	systemGroup := []keys.KeyName{keys.KeyTab, keys.KeyHelp, keys.KeyQuit}

	// Combine all groups in visual order.
	options = append(options, actionGroup...)
	options = append(options, workspaceGroup...)
	options = append(options, systemGroup...)

	m.options = options
}

// SetSize sets the width of the window. The menu will be centered horizontally within this width.
func (m *Menu) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *Menu) String() string {
	var s strings.Builder

	for i, k := range m.options {
		binding := keys.GlobalkeyBindings[k]

		var (
			localActionStyle = actionGroupStyle
			localKeyStyle    = keyStyle
			localDescStyle   = descStyle
		)
		if m.keyDown == k {
			localActionStyle = localActionStyle.Underline(true)
			localKeyStyle = localKeyStyle.Underline(true)
			localDescStyle = localDescStyle.Underline(true)
		}

		// The action group (enter / push / resume / etc.) is the "primary"
		// group and gets emphasized styling.
		if keyMenuGroup[k] == menuGroupAction {
			s.WriteString(localActionStyle.Render(binding.Help().Key))
			s.WriteString(" ")
			s.WriteString(localActionStyle.Render(binding.Help().Desc))
		} else {
			s.WriteString(localKeyStyle.Render(binding.Help().Key))
			s.WriteString(" ")
			s.WriteString(localDescStyle.Render(binding.Help().Desc))
		}

		// Group-transition separators (vertical bar) vs. intra-group separators (bullet).
		if i != len(m.options)-1 {
			nextGroupDiffers := keyMenuGroup[k] != keyMenuGroup[m.options[i+1]]
			if nextGroupDiffers {
				s.WriteString(sepStyle.Render(verticalSeparator))
			} else {
				s.WriteString(sepStyle.Render(separator))
			}
		}
	}

	centeredMenuText := menuStyle.Render(s.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, centeredMenuText)
}
