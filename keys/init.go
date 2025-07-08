//go:build !pro

package keys

import "github.com/charmbracelet/bubbles/key"

// init registers some global key bindings.
func init() {
	GlobalkeyBindings[KeyPrev] = key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	)
	GlobalkeyBindings[KeyNext] = key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	)
	GlobalKeyStringsMap["up"] = KeyPrev
	GlobalKeyStringsMap["k"] = KeyPrev
	GlobalKeyStringsMap["down"] = KeyNext
	GlobalKeyStringsMap["j"] = KeyNext
}
