package tmux

import (
	cmd2 "claude-squad/cmd"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"claude-squad/cmd/cmd_test"

	"github.com/stretchr/testify/require"
)

type MockPtyFactory struct {
	t *testing.T

	// Array of commands and the corresponding file handles representing PTYs.
	cmds  []*exec.Cmd
	files []*os.File
}

func (pt *MockPtyFactory) Start(cmd *exec.Cmd) (*os.File, error) {
	filePath := filepath.Join(pt.t.TempDir(), fmt.Sprintf("pty-%s-%d", pt.t.Name(), rand.Int31()))
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err == nil {
		pt.cmds = append(pt.cmds, cmd)
		pt.files = append(pt.files, f)
	}
	return f, err
}

func (pt *MockPtyFactory) Close() {}

func NewMockPtyFactory(t *testing.T) *MockPtyFactory {
	return &MockPtyFactory{
		t: t,
	}
}

func TestScanForDetach(t *testing.T) {
	modifyOtherKeysCtrlQ := []byte{0x1b, '[', '2', '7', ';', '5', ';', '1', '1', '3', '~'}
	modifyOtherKeysCtrlShiftQ := []byte{0x1b, '[', '2', '7', ';', '6', ';', '8', '1', '~'}
	// Sequences that look like the detach key but are not, plus a mouse-report
	// burst of the kind that routinely arrives co-buffered with a keystroke.
	ctrlP := []byte("\x1b[27;5;112~")
	shiftQ := []byte("\x1b[27;2;81~")
	truncated := []byte("\x1b[27;5;11")
	mouseBurst := []byte("\x1b[<64;10;5M")

	cases := []struct {
		name        string
		buf         []byte
		wantForward int
		wantDetach  bool
	}{
		{"empty buffer", []byte{}, 0, false},
		{"single non-ctrl-q byte", []byte{'a'}, 1, false},
		{"multi-byte without ctrl-q", []byte("hello"), 5, false},
		{"single ctrl-q byte", []byte{0x11}, 0, true},
		{"ctrl-q at start with trailing bytes", []byte{0x11, 'a', 'b'}, 0, true},
		{"ctrl-q in the middle", []byte{'a', 0x11, 'b'}, 1, true},
		{"ctrl-q after mouse-tracking escape", []byte{0x1b, '[', 'M', ' ', '!', '!', 0x11}, 6, true},
		{"modifyOtherKeys ctrl-q alone", modifyOtherKeysCtrlQ, 0, true},
		{"modifyOtherKeys ctrl-q preceded by other bytes", append([]byte{'a', 'b'}, modifyOtherKeysCtrlQ...), 2, true},
		{"modifyOtherKeys ctrl-shift-q", modifyOtherKeysCtrlShiftQ, 0, true},
		{"unrelated CSI sequence is not ctrl-q", []byte{0x1b, '[', 'A'}, 3, false},
		{"modifyOtherKeys ctrl-p is not ctrl-q", ctrlP, len(ctrlP), false},
		{"modifyOtherKeys shift-q without ctrl is not ctrl-q", shiftQ, len(shiftQ), false},
		{"truncated modifyOtherKeys sequence is not ctrl-q", truncated, len(truncated), false},
		{
			"ctrl-q after an SGR mouse burst",
			append(append([]byte{}, mouseBurst...), 0x11),
			len(mouseBurst),
			true,
		},
		{
			"earliest encoding wins when both forms are present",
			append(append([]byte{}, modifyOtherKeysCtrlQ...), 'a', 0x11),
			0,
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotForward, gotDetach := scanForDetach(tc.buf)
			require.Equal(t, tc.wantForward, gotForward)
			require.Equal(t, tc.wantDetach, gotDetach)
		})
	}
}

func TestSanitizeName(t *testing.T) {
	session := NewTmuxSession("asdf", "program")
	require.Equal(t, TmuxPrefix+"asdf", session.sanitizedName)

	session = NewTmuxSession("a sd f . . asdf", "program")
	require.Equal(t, TmuxPrefix+"asdf__asdf", session.sanitizedName)
}

func TestStartTmuxSession(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)

	created := false
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			if strings.Contains(cmd.String(), "has-session") && !created {
				created = true
				return fmt.Errorf("session already exists")
			}
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("output"), nil
		},
	}

	workdir := t.TempDir()
	session := newTmuxSession("test-session", "claude", ptyFactory, cmdExec)

	err := session.Start(workdir)
	require.NoError(t, err)
	require.Equal(t, 2, len(ptyFactory.cmds))
	require.Equal(t, fmt.Sprintf("tmux new-session -d -s claudesquad_test-session -c %s claude", workdir),
		cmd2.ToString(ptyFactory.cmds[0]))
	require.Equal(t, "tmux attach-session -t claudesquad_test-session",
		cmd2.ToString(ptyFactory.cmds[1]))

	require.Equal(t, 2, len(ptyFactory.files))

	// File should be closed.
	_, err = ptyFactory.files[0].Stat()
	require.Error(t, err)
	// File should be open
	_, err = ptyFactory.files[1].Stat()
	require.NoError(t, err)
}

// A tmux server that has gone away (reboot, crash, `tmux kill-server`) takes every session
// with it. attach-session against a missing session still forks successfully, so Restore
// has to check for the session itself or it reports success while attached to nothing.
func TestRestoreReturnsErrSessionNotFoundWhenSessionIsGone(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			if strings.Contains(cmd.String(), "has-session") {
				return fmt.Errorf("can't find session")
			}
			return nil
		},
	}

	session := NewTmuxSessionWithDeps("gone", "program", ptyFactory, cmdExec)
	err := session.Restore()

	require.ErrorIs(t, err, ErrSessionNotFound)
	require.Empty(t, ptyFactory.cmds, "should not have opened a PTY for a session that does not exist")
}

func TestRestoreAttachesWhenSessionExists(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error { return nil },
	}

	session := NewTmuxSessionWithDeps("alive", "program", ptyFactory, cmdExec)
	require.NoError(t, session.Restore())
	require.Len(t, ptyFactory.cmds, 1)
	require.Contains(t, ptyFactory.cmds[0].String(), "attach-session")
}
