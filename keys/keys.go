package keys

import (
	"github.com/charmbracelet/bubbles/key"
)

type KeyName int

const (
	KeyUp KeyName = iota
	KeyDown
	KeyEnter
	KeyNew
	KeyKill
	KeyQuit
	KeyReview
	KeyPush
	KeySubmit

	KeyTab        // Tab is a special keybinding for switching between panes.
	KeySubmitName // SubmitName is a special keybinding for submitting the name of a new instance.

	KeyCheckout
	KeyResume
	KeyPrompt // New key for entering a prompt
	KeyHelp   // Key for showing help screen

	// Scroll keybindings (work in both preview and diff panels)
	KeyShiftUp
	KeyShiftDown
	KeyCtrlShiftUp    // Fast scroll up (10 lines)
	KeyCtrlShiftDown  // Fast scroll down (10 lines)
	KeyAddProject     // Key for adding a new project
	KeyMCPManage      // Key for MCP management
	KeyProjectHistory // Key for project history selection
)

// GlobalKeyStringsMap is a global, immutable map string to keybinding.
var GlobalKeyStringsMap = map[string]KeyName{
	"up":              KeyUp,
	"k":               KeyUp,
	"down":            KeyDown,
	"j":               KeyDown,
	"shift+up":        KeyShiftUp,
	"shift+down":      KeyShiftDown,
	"ctrl+shift+up":   KeyCtrlShiftUp,
	"ctrl+shift+down": KeyCtrlShiftDown,
	"N":               KeyPrompt,
	"enter":           KeyEnter,
	"o":               KeyEnter,
	"n":               KeyNew,
	"D":               KeyKill,
	"q":               KeyQuit,
	"tab":             KeyTab,
	"c":               KeyCheckout,
	"r":               KeyResume,
	"p":               KeySubmit,
	"P":               KeyAddProject,
	"m":               KeyMCPManage,
	"R":               KeyProjectHistory,
	"?":               KeyHelp,
}

// GlobalkeyBindings is a global, immutable map of KeyName tot keybinding.
var GlobalkeyBindings = map[KeyName]key.Binding{
	KeyUp: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	KeyDown: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	KeyShiftUp: key.NewBinding(
		key.WithKeys("shift+up"),
		key.WithHelp("shift+↑", "scroll up (+ctrl for fast)"),
	),
	KeyShiftDown: key.NewBinding(
		key.WithKeys("shift+down"),
		key.WithHelp("shift+↓", "scroll down (+ctrl for fast)"),
	),
	KeyCtrlShiftUp: key.NewBinding(
		key.WithKeys("ctrl+shift+up"),
		key.WithHelp("ctrl+shift+↑", "fast scroll up"),
	),
	KeyCtrlShiftDown: key.NewBinding(
		key.WithKeys("ctrl+shift+down"),
		key.WithHelp("ctrl+shift+↓", "fast scroll down"),
	),
	KeyEnter: key.NewBinding(
		key.WithKeys("enter", "o"),
		key.WithHelp("↵/o", "open"),
	),
	KeyNew: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "new"),
	),
	KeyKill: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "kill"),
	),
	KeyHelp: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	KeyQuit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),
	KeySubmit: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "push branch"),
	),
	KeyPrompt: key.NewBinding(
		key.WithKeys("N"),
		key.WithHelp("N", "new with prompt"),
	),
	KeyCheckout: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "checkout"),
	),
	KeyTab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch tab"),
	),
	KeyResume: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "resume"),
	),
	KeyAddProject: key.NewBinding(
		key.WithKeys("P"),
		key.WithHelp("P", "add project"),
	),
	KeyMCPManage: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "manage MCP servers"),
	),
	KeyProjectHistory: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "recent projects"),
	),

	// -- Special keybindings --

	KeySubmitName: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "submit name"),
	),
}
