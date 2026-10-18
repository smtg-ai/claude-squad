package tmux

import (
	cmd2 "claude-squad/cmd"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"claude-squad/cmd/cmd_test"
	"claude-squad/log"

	"github.com/stretchr/testify/require"
)

// TestMain runs before all tests to set up the test environment.
func TestMain(m *testing.M) {
	log.Initialize(false)
	defer log.Close()

	os.Exit(m.Run())
}

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

func TestWaitForCliReadyKnownCliIgnoresStableContent(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			// Static startup screen of a known CLI, without a ready prompt.
			return []byte("Claude Code is starting...\nLoading plugins"), nil
		},
	}

	session := newTmuxSession("test-session", "claude", ptyFactory, cmdExec)

	require.False(t, session.WaitForCliReady(1500*time.Millisecond))
}

func TestWaitForCliReadyKnownCliDetectsReadyPrompt(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("Claude Code\n❯ "), nil
		},
	}

	session := newTmuxSession("test-session", "claude", ptyFactory, cmdExec)

	require.True(t, session.WaitForCliReady(2*time.Second))
}

func TestWaitForCliReadyUnknownCliFallsBackToStableContent(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("my-cli is starting...\nLoading"), nil
		},
	}

	session := newTmuxSession("test-session", "my-cli", ptyFactory, cmdExec)

	require.True(t, session.WaitForCliReady(3*time.Second))
}

// Claude Code with SessionStart hooks paints its prompt glyph while hooks are still
// writing to the pane, so the first prompt-shaped frame is not a usable input surface.
// Readiness has to wait until the pane has stopped changing.
func TestWaitForCliReadyRequiresSettledPrompt(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)
	var (
		mu           sync.Mutex
		captures     int
		firstSettled time.Time
	)
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			mu.Lock()
			defer mu.Unlock()
			captures++
			if captures <= 5 {
				return []byte(fmt.Sprintf("SessionStart hook %d\n❯ ", captures)), nil
			}
			if firstSettled.IsZero() {
				firstSettled = time.Now()
			}
			return []byte("Claude Code\n❯ "), nil
		},
	}

	session := newTmuxSession("test-session", "claude", ptyFactory, cmdExec)

	require.True(t, session.WaitForCliReady(5*time.Second))

	mu.Lock()
	settledAt, total := firstSettled, captures
	mu.Unlock()
	require.False(t, settledAt.IsZero(), "never observed the settled prompt")
	require.GreaterOrEqual(t, time.Since(settledAt), time.Second,
		"reported ready before the prompt had been unchanged for a full second")
	require.GreaterOrEqual(t, total, 6, "should have waited past the startup frames")
}

// A capture error is not a stable pane: the stability window must start over, and
// success must require a whole fresh second of unchanged content.
func TestWaitForCliReadyResetsStability(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)
	var (
		mu        sync.Mutex
		calls     int
		resumedAt time.Time
	)
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			mu.Lock()
			defer mu.Unlock()
			calls++
			if calls == 4 {
				return nil, fmt.Errorf("pane capture failed")
			}
			if calls > 4 && resumedAt.IsZero() {
				resumedAt = time.Now()
			}
			return []byte("Claude Code\n❯ "), nil
		},
	}

	session := newTmuxSession("test-session", "claude", ptyFactory, cmdExec)

	require.True(t, session.WaitForCliReady(5*time.Second))

	mu.Lock()
	resumed, total := resumedAt, calls
	mu.Unlock()
	require.False(t, resumed.IsZero(), "never captured the pane again after the error")
	require.GreaterOrEqual(t, time.Since(resumed), time.Second,
		"stability window was not reset by the capture error")
	require.GreaterOrEqual(t, total, 5)
}

// Blank panes are not a ready input surface, no matter how long they stay blank.
func TestWaitForCliReadyNeverAcceptsBlankOutput(t *testing.T) {
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("  \n\t\n"), nil
		},
	}

	session := newTmuxSession("test-session", "claude", NewMockPtyFactory(t), cmdExec)

	require.False(t, session.WaitForCliReady(1500*time.Millisecond))
}

// Continuously changing output never settles, so it must not be mistaken for readiness.
func TestWaitForCliReadyNeverAcceptsChangingOutput(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)
	var mu sync.Mutex
	frames := 0
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			mu.Lock()
			defer mu.Unlock()
			frames++
			return []byte(fmt.Sprintf("SessionStart hook %d\n❯ ", frames)), nil
		},
	}

	session := newTmuxSession("test-session", "claude", ptyFactory, cmdExec)

	require.False(t, session.WaitForCliReady(1500*time.Millisecond))
}

// The stability gate must never outlive the caller's deadline.
func TestWaitForCliReadyCannotExceedDeadline(t *testing.T) {
	for _, timeout := range []time.Duration{0, -time.Millisecond, 500 * time.Millisecond} {
		t.Run(timeout.String(), func(t *testing.T) {
			cmdExec := cmd_test.MockCmdExec{
				RunFunc: func(cmd *exec.Cmd) error { return nil },
				OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
					return []byte("Claude Code\n❯ "), nil
				},
			}

			session := newTmuxSession("test-session", "claude", NewMockPtyFactory(t), cmdExec)

			require.False(t, session.WaitForCliReady(timeout))
		})
	}
}

// IsCliReady stays an instantaneous predicate over one capture; it must not grow the
// stability requirement that belongs to WaitForCliReady.
func TestIsCliReadyKnownPrompts(t *testing.T) {
	tests := []struct {
		name    string
		program string
		content string
		want    bool
	}{
		{"claude prompt glyph", "claude", "Claude Code\n❯ ", true},
		{"claude help hint", "claude", "Claude Code\n/help", true},
		{"claude startup screen", "claude", "Claude Code is starting...\nLoading plugins", false},
		{"aider prompt", "aider", "aider v0.50.0\n>", true},
		{"gemini prompt", "gemini", "Gemini CLI\n>", true},
		{"codex prompt", "codex", "codex\n>", true},
		{"unknown cli prompt", "my-cli", "my-cli\n>", false},
		{"blank pane", "claude", "  \n\t\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdExec := cmd_test.MockCmdExec{
				RunFunc: func(cmd *exec.Cmd) error { return nil },
				OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
					return []byte(tt.content), nil
				},
			}

			session := newTmuxSession("test-session", tt.program, NewMockPtyFactory(t), cmdExec)

			require.Equal(t, tt.want, session.IsCliReady())
		})
	}
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
