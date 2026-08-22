package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// specialKeySequences maps bubbletea's special (negative) key types to the
// ANSI escape sequences a terminal would send for them.
var specialKeySequences = map[tea.KeyType]string{
	tea.KeyUp:             "\x1b[A",
	tea.KeyDown:           "\x1b[B",
	tea.KeyRight:          "\x1b[C",
	tea.KeyLeft:           "\x1b[D",
	tea.KeyShiftTab:       "\x1b[Z",
	tea.KeyHome:           "\x1b[H",
	tea.KeyEnd:            "\x1b[F",
	tea.KeyPgUp:           "\x1b[5~",
	tea.KeyPgDown:         "\x1b[6~",
	tea.KeyDelete:         "\x1b[3~",
	tea.KeyInsert:         "\x1b[2~",
	tea.KeyCtrlUp:         "\x1b[1;5A",
	tea.KeyCtrlDown:       "\x1b[1;5B",
	tea.KeyCtrlRight:      "\x1b[1;5C",
	tea.KeyCtrlLeft:       "\x1b[1;5D",
	tea.KeyShiftUp:        "\x1b[1;2A",
	tea.KeyShiftDown:      "\x1b[1;2B",
	tea.KeyShiftRight:     "\x1b[1;2C",
	tea.KeyShiftLeft:      "\x1b[1;2D",
	tea.KeyCtrlShiftUp:    "\x1b[1;6A",
	tea.KeyCtrlShiftDown:  "\x1b[1;6B",
	tea.KeyCtrlShiftRight: "\x1b[1;6C",
	tea.KeyCtrlShiftLeft:  "\x1b[1;6D",
	tea.KeyCtrlHome:       "\x1b[1;5H",
	tea.KeyCtrlEnd:        "\x1b[1;5F",
	tea.KeyShiftHome:      "\x1b[1;2H",
	tea.KeyShiftEnd:       "\x1b[1;2F",
	tea.KeyCtrlPgUp:       "\x1b[5;5~",
	tea.KeyCtrlPgDown:     "\x1b[6;5~",
	tea.KeyF1:             "\x1bOP",
	tea.KeyF2:             "\x1bOQ",
	tea.KeyF3:             "\x1bOR",
	tea.KeyF4:             "\x1bOS",
	tea.KeyF5:             "\x1b[15~",
	tea.KeyF6:             "\x1b[17~",
	tea.KeyF7:             "\x1b[18~",
	tea.KeyF8:             "\x1b[19~",
	tea.KeyF9:             "\x1b[20~",
	tea.KeyF10:            "\x1b[21~",
	tea.KeyF11:            "\x1b[23~",
	tea.KeyF12:            "\x1b[24~",
}

// keyMsgToBytes translates a bubbletea key message into the byte sequence a
// terminal would have sent for that key, so it can be replayed into the
// session's PTY. Returns nil for keys that have no terminal representation.
func keyMsgToBytes(msg tea.KeyMsg) []byte {
	var out []byte
	if msg.Alt {
		out = append(out, 0x1b)
	}

	switch {
	case msg.Type == tea.KeyRunes:
		if msg.Paste {
			// Preserve bracketed paste so multi-line pastes don't submit per line.
			out = append(out, []byte("\x1b[200~")...)
			out = append(out, []byte(string(msg.Runes))...)
			out = append(out, []byte("\x1b[201~")...)
			return out
		}
		return append(out, []byte(string(msg.Runes))...)
	case msg.Type == tea.KeySpace:
		return append(out, ' ')
	case msg.Type >= 0 && msg.Type <= 0x1f, msg.Type == 0x7f:
		// Control characters map 1:1 (enter, tab, esc, backspace, ctrl+a..z).
		return append(out, byte(msg.Type))
	default:
		if seq, ok := specialKeySequences[msg.Type]; ok {
			return append(out, []byte(seq)...)
		}
	}
	return nil
}
