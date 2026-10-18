package session

import (
	"claude-squad/cmd/cmd_test"
	"claude-squad/log"
	"claude-squad/session/tmux"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	log.Initialize(false)
	defer log.Close()
	os.Exit(m.Run())
}

// nullPtyFactory hands back a throwaway file instead of a real PTY.
type nullPtyFactory struct {
	t     *testing.T
	calls int
}

func (p *nullPtyFactory) Start(cmd *exec.Cmd) (*os.File, error) {
	p.calls++
	return os.OpenFile(filepath.Join(p.t.TempDir(), "pty"), os.O_CREATE|os.O_RDWR, 0644)
}

func (p *nullPtyFactory) Close() {}

// When the tmux server dies between runs, every session goes with it while the worktree
// and branch survive on disk. Restoring such an instance must park it as Paused so the
// user can resume it. Returning an error instead is not an option: LoadInstances aborts on
// the first failure, so one dead session would hide every other instance.
// See https://github.com/smtg-ai/claude-squad/issues/216.
func TestStartPausesInstanceWhenTmuxSessionNoLongerExists(t *testing.T) {
	ptyFactory := &nullPtyFactory{t: t}
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			if strings.Contains(cmd.String(), "has-session") {
				return fmt.Errorf("can't find session")
			}
			return nil
		},
	}

	instance, err := NewInstance(InstanceOptions{Title: "revived", Path: t.TempDir(), Program: "claude"})
	require.NoError(t, err)
	instance.SetTmuxSession(tmux.NewTmuxSessionWithDeps("revived", "claude", ptyFactory, cmdExec))

	require.NoError(t, instance.Start(false), "a dead tmux session is recoverable, not a startup failure")
	require.Equal(t, Paused, instance.Status)
	require.True(t, instance.Started())
	require.Zero(t, ptyFactory.calls, "should not attach to a session that does not exist")
}

// The happy path is unchanged: an instance whose session survived comes back Running.
func TestStartRestoresInstanceWhenTmuxSessionSurvives(t *testing.T) {
	ptyFactory := &nullPtyFactory{t: t}
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error { return nil },
	}

	instance, err := NewInstance(InstanceOptions{Title: "alive", Path: t.TempDir(), Program: "claude"})
	require.NoError(t, err)
	instance.SetTmuxSession(tmux.NewTmuxSessionWithDeps("alive", "claude", ptyFactory, cmdExec))

	require.NoError(t, instance.Start(false))
	require.Equal(t, Running, instance.Status)
	require.Equal(t, 1, ptyFactory.calls)
}

// pipePtyFactory hands SendPrompt a pipe so the test can act as the CLI consuming the
// keystrokes that would otherwise go to a real PTY.
type pipePtyFactory struct {
	writer *os.File
}

func (p *pipePtyFactory) Start(*exec.Cmd) (*os.File, error) { return p.writer, nil }

func (p *pipePtyFactory) Close() {}

// A CLI that is still processing pasted text discards a carriage return that arrives
// mid-processing. That is the reported Claude Code failure: the prompt text lands in the
// input box but Enter is dropped during startup. The test acts as such a CLI and requires
// the prompt to be submitted exactly once.
func TestSendPromptWaitsForInputProcessing(t *testing.T) {
	const (
		prompt          = "fix the failing test"
		processingDelay = 300 * time.Millisecond
	)

	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})

	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte("output"), nil },
	}
	instance, err := NewInstance(InstanceOptions{Title: "prompt", Path: t.TempDir(), Program: "claude"})
	require.NoError(t, err)
	instance.SetTmuxSession(tmux.NewTmuxSessionWithDeps("prompt", "claude", &pipePtyFactory{writer: writer}, cmdExec))
	require.NoError(t, instance.Start(false))

	type terminalState struct {
		submitted []string
		dropped   int
		pending   string
		err       error
	}

	ready := make(chan struct{})
	results := make(chan terminalState, 1)
	go func() {
		close(ready)
		_ = reader.SetReadDeadline(time.Now().Add(10 * time.Second))

		var (
			pending   []byte
			lastText  time.Time
			submitted []string
			dropped   int
		)
		buf := make([]byte, 256)
		for {
			n, readErr := reader.Read(buf)
			for _, b := range buf[:n] {
				if b != '\r' {
					pending = append(pending, b)
					lastText = time.Now()
					continue
				}
				if !lastText.IsZero() && time.Since(lastText) < processingDelay {
					dropped++
					continue
				}
				if len(pending) > 0 {
					submitted = append(submitted, string(pending))
					pending = nil
				}
			}
			if readErr != nil {
				state := terminalState{submitted: submitted, dropped: dropped, pending: string(pending)}
				if !errors.Is(readErr, io.EOF) {
					state.err = readErr
				}
				results <- state
				return
			}
		}
	}()

	<-ready
	require.NoError(t, instance.SendPrompt(prompt))
	require.NoError(t, writer.Close(), "closing the write end should signal EOF to the reader")

	select {
	case state := <-results:
		require.NoError(t, state.err)
		require.Empty(t, state.pending, "prompt text was never acted on")
		require.Zero(t, state.dropped, "the carriage return was dropped while the CLI was still processing")
		require.Equal(t, []string{prompt}, state.submitted, "the prompt must be submitted exactly once")
	case <-time.After(5 * time.Second):
		t.Fatal("reader did not observe EOF")
	}
}
