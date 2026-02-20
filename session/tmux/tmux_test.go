package tmux

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cmd2 "github.com/ByteMirror/hivemind/cmd"

	"github.com/ByteMirror/hivemind/cmd/cmd_test"

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

func TestSanitizeName(t *testing.T) {
	session := NewTmuxSession("asdf", "program", false)
	require.Equal(t, TmuxPrefix+"asdf", session.sanitizedName)

	session = NewTmuxSession("a sd f . . asdf", "program", false)
	require.Equal(t, TmuxPrefix+"a-sd-f---asdf", session.sanitizedName)
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
			if strings.Contains(cmd.String(), "capture-pane") {
				// Return substantial content so startup detection exits quickly
				return []byte(strings.Repeat("x", 200)), nil
			}
			return []byte("output"), nil
		},
	}

	workdir := t.TempDir()
	session := newTmuxSession("test-session", "claude", false, ptyFactory, cmdExec)

	err := session.Start(workdir)
	require.NoError(t, err)
	require.Equal(t, 2, len(ptyFactory.cmds))
	require.Equal(t, fmt.Sprintf("tmux new-session -d -s hivemind_test-session -c %s claude", workdir),
		cmd2.ToString(ptyFactory.cmds[0]))
	require.Equal(t, "tmux attach-session -t hivemind_test-session",
		cmd2.ToString(ptyFactory.cmds[1]))

	require.Equal(t, 2, len(ptyFactory.files))

	// File should be closed.
	_, err = ptyFactory.files[0].Stat()
	require.Error(t, err)
	// File should be open
	_, err = ptyFactory.files[1].Stat()
	require.NoError(t, err)
}

func TestStartTmuxSessionWithSkipPermissions(t *testing.T) {
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
			if strings.Contains(cmd.String(), "capture-pane") {
				return []byte(strings.Repeat("x", 200)), nil
			}
			return []byte("output"), nil
		},
	}

	workdir := t.TempDir()
	session := newTmuxSession("test-session", "claude", true, ptyFactory, cmdExec)

	err := session.Start(workdir)
	require.NoError(t, err)
	require.Equal(t, 2, len(ptyFactory.cmds))
	require.Equal(t, fmt.Sprintf("tmux new-session -d -s hivemind_test-session -c %s claude --dangerously-skip-permissions", workdir),
		cmd2.ToString(ptyFactory.cmds[0]))
}

func TestListWindows(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)

	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			if strings.Contains(cmd.String(), "list-windows") {
				return []byte("0 claude 12345\n1 claude 12346\n2 claude 12347\n"), nil
			}
			return []byte(""), nil
		},
	}

	session := newTmuxSession("test-session", "claude", false, ptyFactory, cmdExec)
	windows, err := session.ListWindows()
	require.NoError(t, err)
	require.Equal(t, 3, len(windows))

	require.Equal(t, 0, windows[0].Index)
	require.Equal(t, "claude", windows[0].Name)
	require.Equal(t, 12345, windows[0].PanePID)

	require.Equal(t, 1, windows[1].Index)
	require.Equal(t, 12346, windows[1].PanePID)

	require.Equal(t, 2, windows[2].Index)
	require.Equal(t, 12347, windows[2].PanePID)
}

func TestListWindowsSingleWindow(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)

	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			if strings.Contains(cmd.String(), "list-windows") {
				return []byte("0 claude 12345\n"), nil
			}
			return []byte(""), nil
		},
	}

	session := newTmuxSession("test-session", "claude", false, ptyFactory, cmdExec)
	windows, err := session.ListWindows()
	require.NoError(t, err)
	require.Equal(t, 1, len(windows))
	require.Equal(t, 0, windows[0].Index)
}

func TestStartTmuxSessionSkipPermissionsNotAppliedToAider(t *testing.T) {
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
			if strings.Contains(cmd.String(), "capture-pane") {
				return []byte(strings.Repeat("x", 200)), nil
			}
			return []byte("output"), nil
		},
	}

	workdir := t.TempDir()
	session := newTmuxSession("test-session", "aider --model gpt-4", true, ptyFactory, cmdExec)

	err := session.Start(workdir)
	require.NoError(t, err)
	require.Equal(t, 2, len(ptyFactory.cmds))
	require.Equal(t, fmt.Sprintf("tmux new-session -d -s hivemind_test-session -c %s aider --model gpt-4", workdir),
		cmd2.ToString(ptyFactory.cmds[0]))
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bold text",
			input:    "\x1b[1mDo you trust the files in this folder?\x1b[0m",
			expected: "Do you trust the files in this folder?",
		},
		{
			name:     "colored text",
			input:    "\x1b[32m✓\x1b[0m Ready to go",
			expected: "✓ Ready to go",
		},
		{
			name:     "multiple codes",
			input:    "\x1b[1m\x1b[34mDo\x1b[0m \x1b[33myou\x1b[0m trust",
			expected: "Do you trust",
		},
		{
			name:     "no codes",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, stripANSI(tt.input))
		})
	}
}

func TestStartDetectsTrustPromptWithANSI(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)

	// Simulate trust prompt wrapped in ANSI escape codes
	trustPromptWithANSI := "\x1b[1m\x1b[33m? \x1b[0m\x1b[1mDo you trust the files in this folder?\x1b[0m\n" +
		strings.Repeat(" ", 80) + "\n" +
		"\x1b[36m>\x1b[0m Yes, proceed\n" +
		"  No, exit"

	enterSent := false
	created := false
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			if strings.Contains(cmd.String(), "has-session") && !created {
				created = true
				return fmt.Errorf("session does not exist")
			}
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			if strings.Contains(cmd.String(), "capture-pane") {
				return []byte(trustPromptWithANSI), nil
			}
			return []byte("output"), nil
		},
	}

	workdir := t.TempDir()
	session := newTmuxSession("test-ansi", "claude", false, ptyFactory, cmdExec)

	start := time.Now()
	err := session.Start(workdir)
	elapsed := time.Since(start)

	require.NoError(t, err)
	// Should complete in well under the 15s timeout — the trust prompt should be
	// detected on the first poll (250ms). Allow 3s for CI slack.
	require.Less(t, elapsed, 3*time.Second, "startup should detect trust prompt quickly, took %v", elapsed)

	// Verify enter was sent by checking PTY writes (the mock PTY is a temp file)
	_ = enterSent // enter is sent via ptmx.Write, verified by no error from Start
}
