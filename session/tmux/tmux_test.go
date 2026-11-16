package tmux

import (
	cmd2 "claude-squad/cmd"
	"claude-squad/log"
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

type executingNewSessionPtyFactory struct {
	t *testing.T
}

func (pt executingNewSessionPtyFactory) Start(cmd *exec.Cmd) (*os.File, error) {
	if strings.Contains(cmd.String(), "new-session") {
		if err := cmd.Run(); err != nil {
			return nil, err
		}
	}
	return os.CreateTemp(pt.t.TempDir(), "pty-")
}

func (executingNewSessionPtyFactory) Close() {}

func TestSanitizeName(t *testing.T) {
	session := NewTmuxSession("asdf", "program")
	require.Equal(t, TmuxPrefix+"asdf", session.sanitizedName)

	session = NewTmuxSession("a sd f . . asdf", "program")
	require.Equal(t, TmuxPrefix+"asdf__asdf", session.sanitizedName)
}

func TestStartTmuxSession(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)

	created := false
	var outputCommands []string
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			if strings.Contains(cmd.String(), "has-session") && !created {
				created = true
				return fmt.Errorf("session already exists")
			}
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			outputCommands = append(outputCommands, cmd2.ToString(cmd))
			if strings.Contains(cmd.String(), "show-options") && strings.Contains(cmd.String(), "update-environment") {
				return []byte("DISPLAY TMUX TMUX_TMPDIR"), nil
			}
			return []byte("output"), nil
		},
	}

	t.Setenv("CPA_API_KEY", "test-cpa-key")
	t.Setenv("TMUX_TMPDIR", "test-tmux-tmp")
	workdir := t.TempDir()
	session := newTmuxSession("test-session", "claude", ptyFactory, cmdExec)

	err := session.Start(workdir)
	require.NoError(t, err)
	require.Len(t, ptyFactory.cmds, 2)
	createCmd := ptyFactory.cmds[0]
	require.Contains(t, createCmd.Args, ";")
	require.Contains(t, createCmd.Args, "new-session")
	require.NotContains(t, cmd2.ToString(createCmd), "test-cpa-key")
	require.Equal(t, "tmux -L claudesquad attach-session -t claudesquad_test-session",
		cmd2.ToString(ptyFactory.cmds[1]))
	require.Contains(t, outputCommands, "tmux -L claudesquad start-server ; show-options -gqv update-environment")
	updateEnvironment := strings.Fields(createCmd.Args[6])
	require.Contains(t, updateEnvironment, "DISPLAY")
	require.Contains(t, updateEnvironment, "CPA_API_KEY")
	require.Contains(t, updateEnvironment, "TMUX_TMPDIR")
	for _, name := range []string{"PWD", "TERM", "TMUX", "TMUX_PANE"} {
		require.NotContains(t, updateEnvironment, name)
	}

	require.Equal(t, 2, len(ptyFactory.files))

	// File should be closed.
	_, err = ptyFactory.files[0].Stat()
	require.Error(t, err)
	// File should be open
	_, err = ptyFactory.files[1].Stat()
	require.NoError(t, err)
}

func TestStartFailsWhenUpdateEnvironmentCannotBeRead(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(*exec.Cmd) error {
			return fmt.Errorf("session not found")
		},
		OutputFunc: func(*exec.Cmd) ([]byte, error) {
			return nil, fmt.Errorf("read failed")
		},
	}
	session := newTmuxSession("test-session", "claude", ptyFactory, cmdExec)

	err := session.Start(t.TempDir())

	require.ErrorContains(t, err, "error reading tmux update-environment: read failed")
	require.Empty(t, ptyFactory.cmds)
}

func TestStartRemovesEnvironmentUnsetSincePreviousRun(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}

	t.Setenv("TMUX_TMPDIR", t.TempDir())
	t.Setenv("CLAUDE_SQUAD_FIRST_ONLY", "old")
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-L", TmuxSocketName, "kill-server").Run()
	})

	ptyFactory := executingNewSessionPtyFactory{t: t}
	first := newTmuxSession("first", "sleep 30", ptyFactory, cmd2.MakeExecutor())
	require.NoError(t, first.Start(t.TempDir()))
	t.Cleanup(func() {
		_ = first.ptmx.Close()
	})

	require.NoError(t, os.Unsetenv("CLAUDE_SQUAD_FIRST_ONLY"))
	second := newTmuxSession("second", "sleep 30", ptyFactory, cmd2.MakeExecutor())
	require.NoError(t, second.Start(t.TempDir()))
	t.Cleanup(func() {
		_ = second.ptmx.Close()
	})

	output, err := exec.Command(
		"tmux", "-L", TmuxSocketName,
		"show-environment", "-t", second.sanitizedName,
		"CLAUDE_SQUAD_FIRST_ONLY",
	).CombinedOutput()
	require.NoError(t, err, string(output))
	require.Equal(t, "-CLAUDE_SQUAD_FIRST_ONLY\n", string(output))
}

func TestCleanupSessionsUsesDedicatedSocket(t *testing.T) {
	log.Initialize(false)
	defer log.Close()

	var outputCommands []string
	var runCommands []string

	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			runCommands = append(runCommands, cmd2.ToString(cmd))
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			outputCommands = append(outputCommands, cmd2.ToString(cmd))
			return []byte("claudesquad_one: 1 windows (created Mon Jan 01 00:00:00 2024)\nother: 1 windows"), nil
		},
	}

	err := CleanupSessions(cmdExec)
	require.NoError(t, err)
	require.Equal(t, []string{"tmux -L claudesquad ls"}, outputCommands)
	require.Equal(t, []string{"tmux -L claudesquad kill-session -t claudesquad_one"}, runCommands)
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
