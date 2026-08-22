package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestKeyMsgToBytes(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyMsg
		want []byte
	}{
		{
			name: "plain runes",
			msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")},
			want: []byte("hello"),
		},
		{
			name: "umlaut runes",
			msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("grüezi")},
			want: []byte("grüezi"),
		},
		{
			name: "space",
			msg:  tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")},
			want: []byte(" "),
		},
		{
			name: "enter",
			msg:  tea.KeyMsg{Type: tea.KeyEnter},
			want: []byte{0x0d},
		},
		{
			name: "escape",
			msg:  tea.KeyMsg{Type: tea.KeyEsc},
			want: []byte{0x1b},
		},
		{
			name: "backspace",
			msg:  tea.KeyMsg{Type: tea.KeyBackspace},
			want: []byte{0x7f},
		},
		{
			name: "ctrl+c",
			msg:  tea.KeyMsg{Type: tea.KeyCtrlC},
			want: []byte{0x03},
		},
		{
			name: "tab",
			msg:  tea.KeyMsg{Type: tea.KeyTab},
			want: []byte{0x09},
		},
		{
			name: "arrow up",
			msg:  tea.KeyMsg{Type: tea.KeyUp},
			want: []byte("\x1b[A"),
		},
		{
			name: "shift+tab",
			msg:  tea.KeyMsg{Type: tea.KeyShiftTab},
			want: []byte("\x1b[Z"),
		},
		{
			name: "alt+rune prefixes escape",
			msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b"), Alt: true},
			want: []byte("\x1bb"),
		},
		{
			name: "paste is bracketed",
			msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("line1\nline2"), Paste: true},
			want: []byte("\x1b[200~line1\nline2\x1b[201~"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, keyMsgToBytes(tt.msg))
		})
	}
}
