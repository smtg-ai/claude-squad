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

func TestSanitizeName(t *testing.T) {
	session := NewTmuxSession("asdf", "program")
	require.Equal(t, TmuxPrefix+"asdf", session.sanitizedName)

	session = NewTmuxSession("a sd f . . asdf", "program")
	require.Equal(t, TmuxPrefix+"asdf__asdf", session.sanitizedName)
}

func TestStartTmuxSession(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)

	created := false
	var runCommands []string
	var outputCommands []string
	var updateEnvironmentValue string
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			runCommands = append(runCommands, cmd2.ToString(cmd))
			if strings.Contains(cmd.String(), "set-option") && strings.Contains(cmd.String(), "update-environment") {
				updateEnvironmentValue = cmd.Args[len(cmd.Args)-1]
			}
			if strings.Contains(cmd.String(), "has-session") && !created {
				created = true
				return fmt.Errorf("session already exists")
			}
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			outputCommands = append(outputCommands, cmd2.ToString(cmd))
			if strings.Contains(cmd.String(), "show-options") && strings.Contains(cmd.String(), "update-environment") {
				return []byte("DISPLAY SSH_AUTH_SOCK"), nil
			}
			return []byte("output"), nil
		},
	}

	t.Setenv("CPA_API_KEY", "test-cpa-key")
	workdir := t.TempDir()
	session := newTmuxSession("test-session", "claude", ptyFactory, cmdExec)

	err := session.Start(workdir)
	require.NoError(t, err)
	require.Equal(t, 2, len(ptyFactory.cmds))
	require.Equal(t, fmt.Sprintf("tmux -L claudesquad new-session -d -s claudesquad_test-session -c %s claude", workdir),
		cmd2.ToString(ptyFactory.cmds[0]))
	require.Equal(t, "tmux -L claudesquad attach-session -t claudesquad_test-session",
		cmd2.ToString(ptyFactory.cmds[1]))
	require.Contains(t, runCommands, "tmux -L claudesquad start-server")
	require.Contains(t, outputCommands, "tmux -L claudesquad show-options -gqv update-environment")
	require.Contains(t, updateEnvironmentValue, "CPA_API_KEY")
	require.NotContains(t, updateEnvironmentValue, "TMUX")

	require.Equal(t, 2, len(ptyFactory.files))

	// File should be closed.
	_, err = ptyFactory.files[0].Stat()
	require.Error(t, err)
	// File should be open
	_, err = ptyFactory.files[1].Stat()
	require.NoError(t, err)
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
