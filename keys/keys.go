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

	// Diff keybindings
	KeyShiftUp
	KeyShiftDown
)

// GlobalKeyStringsMap is a configurable map string to keybinding.
var GlobalKeyStringsMap = map[string]KeyName{
	"up":         KeyUp,
	"k":          KeyUp,
	"down":       KeyDown,
	"j":          KeyDown,
	"shift+up":   KeyShiftUp,
	"shift+down": KeyShiftDown,
	"N":          KeyPrompt,
	"enter":      KeyEnter,
	"o":          KeyEnter,
	"n":          KeyNew,
	"D":          KeyKill,
	"q":          KeyQuit,
	"tab":        KeyTab,
	"c":          KeyCheckout,
	"r":          KeyResume,
	"p":          KeySubmit,
	"?":          KeyHelp,
}

// GlobalkeyBindings is a configurable map of KeyName to keybinding.
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
		key.WithHelp("shift+↑", "scroll"),
	),
	KeyShiftDown: key.NewBinding(
		key.WithKeys("shift+down"),
		key.WithHelp("shift+↓", "scroll"),
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

	// -- Special keybindings --

	KeySubmitName: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "submit name"),
	),
}

// UpdateKeyMappings updates the global key mappings based on user configuration
func UpdateKeyMappings(userMappings map[string][]string) {
	if userMappings == nil {
		return
	}

	// Start with default mappings
	GlobalKeyStringsMap = map[string]KeyName{
		"up":         KeyUp,
		"k":          KeyUp,
		"down":       KeyDown,
		"j":          KeyDown,
		"shift+up":   KeyShiftUp,
		"shift+down": KeyShiftDown,
		"N":          KeyPrompt,
		"enter":      KeyEnter,
		"o":          KeyEnter,
		"n":          KeyNew,
		"D":          KeyKill,
		"q":          KeyQuit,
		"tab":        KeyTab,
		"c":          KeyCheckout,
		"r":          KeyResume,
		"p":          KeySubmit,
		"?":          KeyHelp,
	}

	// Override with user-configured mappings
	for action, keys := range userMappings {
		var keyName KeyName
		var defaultKeys []string
		switch action {
		case "up":
			keyName = KeyUp
			defaultKeys = []string{"up", "k"}
		case "down":
			keyName = KeyDown
			defaultKeys = []string{"down", "j"}
		case "shift+up":
			keyName = KeyShiftUp
			defaultKeys = []string{"shift+up"}
		case "shift+down":
			keyName = KeyShiftDown
			defaultKeys = []string{"shift+down"}
		case "enter":
			keyName = KeyEnter
			defaultKeys = []string{"enter", "o"}
		case "new":
			keyName = KeyNew
			defaultKeys = []string{"n"}
		case "kill":
			keyName = KeyKill
			defaultKeys = []string{"D"}
		case "quit":
			keyName = KeyQuit
			defaultKeys = []string{"q"}
		case "tab":
			keyName = KeyTab
			defaultKeys = []string{"tab"}
		case "checkout":
			keyName = KeyCheckout
			defaultKeys = []string{"c"}
		case "resume":
			keyName = KeyResume
			defaultKeys = []string{"r"}
		case "submit":
			keyName = KeySubmit
			defaultKeys = []string{"p"}
		case "prompt":
			keyName = KeyPrompt
			defaultKeys = []string{"N"}
		case "help":
			keyName = KeyHelp
			defaultKeys = []string{"?"}
		default:
			continue // Skip unknown actions
		}

		// Clear default keys for this action
		for _, k := range defaultKeys {
			delete(GlobalKeyStringsMap, k)
		}

		// Map all configured keys to this action
		for _, k := range keys {
			GlobalKeyStringsMap[k] = keyName
		}
	}

	// Update key bindings with new mappings
	updateKeyBindings(userMappings)
}

// updateKeyBindings recreates the key bindings with user-configured keys
func updateKeyBindings(userMappings map[string][]string) {
	if keys, ok := userMappings["up"]; ok {
		GlobalkeyBindings[KeyUp] = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(getHelpKey(keys), "up"),
		)
	}

	if keys, ok := userMappings["down"]; ok {
		GlobalkeyBindings[KeyDown] = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(getHelpKey(keys), "down"),
		)
	}

	if keys, ok := userMappings["shift+up"]; ok {
		GlobalkeyBindings[KeyShiftUp] = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(getHelpKey(keys), "scroll"),
		)
	}

	if keys, ok := userMappings["shift+down"]; ok {
		GlobalkeyBindings[KeyShiftDown] = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(getHelpKey(keys), "scroll"),
		)
	}

	if keys, ok := userMappings["enter"]; ok {
		GlobalkeyBindings[KeyEnter] = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(getHelpKey(keys), "open"),
		)
	}

	if keys, ok := userMappings["new"]; ok {
		GlobalkeyBindings[KeyNew] = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(getHelpKey(keys), "new"),
		)
	}

	if keys, ok := userMappings["kill"]; ok {
		GlobalkeyBindings[KeyKill] = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(getHelpKey(keys), "kill"),
		)
	}

	if keys, ok := userMappings["quit"]; ok {
		GlobalkeyBindings[KeyQuit] = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(getHelpKey(keys), "quit"),
		)
	}

	if keys, ok := userMappings["tab"]; ok {
		GlobalkeyBindings[KeyTab] = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(getHelpKey(keys), "switch tab"),
		)
	}

	if keys, ok := userMappings["checkout"]; ok {
		GlobalkeyBindings[KeyCheckout] = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(getHelpKey(keys), "checkout"),
		)
	}

	if keys, ok := userMappings["resume"]; ok {
		GlobalkeyBindings[KeyResume] = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(getHelpKey(keys), "resume"),
		)
	}

	if keys, ok := userMappings["submit"]; ok {
		GlobalkeyBindings[KeySubmit] = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(getHelpKey(keys), "push branch"),
		)
	}

	if keys, ok := userMappings["prompt"]; ok {
		GlobalkeyBindings[KeyPrompt] = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(getHelpKey(keys), "new with prompt"),
		)
	}

	if keys, ok := userMappings["help"]; ok {
		GlobalkeyBindings[KeyHelp] = key.NewBinding(
			key.WithKeys(keys...),
			key.WithHelp(getHelpKey(keys), "help"),
		)
	}
}

// getHelpKey formats the help text for key combinations
func getHelpKey(keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	if len(keys) == 1 {
		return keys[0]
	}
	// Join multiple keys with "/"
	result := ""
	for i, k := range keys {
		if i > 0 {
			result += "/"
		}
		result += k
	}
	return result
}
