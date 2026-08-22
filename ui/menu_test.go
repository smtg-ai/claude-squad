package ui

import (
	"claude-squad/session"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMenuFocusStateShowsHint(t *testing.T) {
	m := NewMenu()
	m.SetSize(120, 1)
	m.SetState(StateFocus)

	out := m.String()
	assert.Contains(t, out, "back to menu")
	assert.Contains(t, out, "keys go to the session")
	// The normal action keys must not leak into the focus hint.
	assert.NotContains(t, out, "push branch")
}

func TestMenuDefaultShowsFocusKey(t *testing.T) {
	m := NewMenu()
	m.SetSize(240, 1)
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "menu-keys",
		Path:    ".",
		Program: "bash",
	})
	require.NoError(t, err)
	m.SetInstance(inst) // drives the default (non-empty) option layout

	out := stripANSI(m.String())
	assert.Contains(t, out, "f focus")
}

// stripANSI removes escape sequences so assertions match on visible text.
func stripANSI(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); {
		if runes[i] == 0x1b {
			i = skipEscape(runes, i)
			continue
		}
		b.WriteRune(runes[i])
		i++
	}
	return b.String()
}
