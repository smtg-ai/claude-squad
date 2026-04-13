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

func TestSanitizeName(t *testing.T) {
	session := NewTmuxSession("asdf", "program")
	require.Equal(t, TmuxPrefix+"asdf", session.sanitizedName)

	session = NewTmuxSession("a sd f . . asdf", "program")
	require.Equal(t, TmuxPrefix+"asdf__asdf", session.sanitizedName)
}

func TestListStandaloneAgents(t *testing.T) {
	tests := []struct {
		name           string
		paneOutput     string
		paneErr        error
		psOutput       string
		psErr          error
		lsofOutput     string
		lsofErr        error
		expectedAgents []StandaloneAgentInfo
		expectErr      bool
	}{
		{
			name:       "finds standalone claude process",
			paneOutput: "/dev/ttys001\n/dev/ttys002\n",
			psOutput: "  PID TTY      COMMAND\n" +
				"48158 ttys025  claude --effort high\n" +
				"  100 ttys001  bash\n",
			lsofOutput: "node    48158 user  cwd  DIR  1,18  896  13090311 /Users/user/workspace/project\n",
			expectedAgents: []StandaloneAgentInfo{
				{
					PID:     "48158",
					Command: "claude --effort high",
					WorkDir: "/Users/user/workspace/project",
					TTY:     "ttys025",
					Program: "claude",
				},
			},
		},
		{
			name:       "finds standalone codex process via node wrapper",
			paneOutput: "/dev/ttys001\n",
			psOutput: "  PID TTY      COMMAND\n" +
				"48158 ttys025  node /Users/user/n/bin/codex\n",
			lsofOutput: "node    48158 user  cwd  DIR  1,18  896  13090311 /Users/user/workspace/rpjava\n",
			expectedAgents: []StandaloneAgentInfo{
				{
					PID:     "48158",
					Command: "node /Users/user/n/bin/codex",
					WorkDir: "/Users/user/workspace/rpjava",
					TTY:     "ttys025",
					Program: "codex",
				},
			},
		},
		{
			name:       "skips process inside tmux pane",
			paneOutput: "/dev/ttys025\n",
			psOutput: "  PID TTY      COMMAND\n" +
				"48158 ttys025  claude\n",
			expectedAgents: nil,
		},
		{
			name:       "skips background process with ?? tty",
			paneOutput: "",
			psOutput: "  PID TTY      COMMAND\n" +
				"48158 ??       claude\n",
			expectedAgents: nil,
		},
		{
			name:       "skips claudesquad process",
			paneOutput: "",
			psOutput: "  PID TTY      COMMAND\n" +
				"48158 ttys025  claudesquad --foo\n",
			expectedAgents: nil,
		},
		{
			name:       "skips claude-squad process",
			paneOutput: "",
			psOutput: "  PID TTY      COMMAND\n" +
				"48158 ttys025  claude-squad run\n",
			expectedAgents: nil,
		},
		{
			name:    "ps command fails",
			psErr:   fmt.Errorf("ps failed"),
			psOutput: "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdExec := cmd_test.MockCmdExec{
				RunFunc: func(cmd *exec.Cmd) error {
					return nil
				},
				OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
					cmdStr := cmd.String()
					if strings.Contains(cmdStr, "list-panes") {
						if tt.paneErr != nil {
							return nil, tt.paneErr
						}
						return []byte(tt.paneOutput), nil
					}
					if strings.Contains(cmdStr, "ps") {
						if tt.psErr != nil {
							return nil, tt.psErr
						}
						return []byte(tt.psOutput), nil
					}
					if strings.Contains(cmdStr, "lsof") {
						if tt.lsofErr != nil {
							return nil, tt.lsofErr
						}
						return []byte(tt.lsofOutput), nil
					}
					return nil, fmt.Errorf("unexpected command: %s", cmdStr)
				},
			}

			agents, err := ListStandaloneAgents(cmdExec)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, len(tt.expectedAgents), len(agents))
			for i, expected := range tt.expectedAgents {
				require.Equal(t, expected.PID, agents[i].PID)
				require.Equal(t, expected.Command, agents[i].Command)
				require.Equal(t, expected.WorkDir, agents[i].WorkDir)
				require.Equal(t, expected.TTY, agents[i].TTY)
				require.Equal(t, expected.Program, agents[i].Program)
			}
		})
	}
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
