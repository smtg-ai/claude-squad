package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEditorCommand(t *testing.T) {
	// An explicit editor_command wins.
	c := &Config{EditorCommand: "code -w"}
	assert.Equal(t, "code -w", c.GetEditorCommand())

	// Whitespace-only is treated as unset and falls back to $EDITOR.
	t.Setenv("EDITOR", "vim")
	c = &Config{EditorCommand: "   "}
	assert.Equal(t, "vim", c.GetEditorCommand())

	// Unset config with no $EDITOR yields empty (caller reports no editor).
	t.Setenv("EDITOR", "")
	c = &Config{}
	assert.Equal(t, "", c.GetEditorCommand())
}
