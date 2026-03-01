package ui

import (
	"claude-squad/cmd/cmd_test"
	"claude-squad/log"
	"claude-squad/session"
	"claude-squad/session/tmux"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newMockTmuxSession creates a mock tmux session backed by MockCmdExec.
// The returned session will report as existing and support capture-pane commands.
func newMockTmuxSession(t *testing.T, name string, cmdExec cmd_test.MockCmdExec) *tmux.TmuxSession {
	t.Helper()
	ptyFactory := &MockPtyFactory{
		t:       t,
		cmdExec: cmdExec,
	}
	return tmux.NewTmuxSessionWithDeps(name, "bash", ptyFactory, cmdExec)
}

// mockCmdExec returns a MockCmdExec that simulates a working tmux session.
// captureContent is returned for capture-pane commands.
func mockCmdExec(captureContent string, sessionExists bool) cmd_test.MockCmdExec {
	return cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			cmdStr := cmd.String()
			if strings.Contains(cmdStr, "has-session") {
				if sessionExists {
					return nil
				}
				return fmt.Errorf("session does not exist")
			}
			if strings.Contains(cmdStr, "new-session") {
				return nil
			}
			if strings.Contains(cmdStr, "kill-session") {
				return nil
			}
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			cmdStr := cmd.String()
			if strings.Contains(cmdStr, "capture-pane") {
				return []byte(captureContent), nil
			}
			return []byte(""), nil
		},
	}
}

// makeStartedInstance creates a minimal instance that reports as started with the given title.
func makeStartedInstance(t *testing.T, title string) *session.Instance {
	t.Helper()
	workdir := t.TempDir()
	setupGitRepo(t, workdir)

	random := time.Now().UnixNano() % 10000000
	sessionName := fmt.Sprintf("test-terminal-%s-%d-%d", title, time.Now().UnixNano(), random)

	sessionCreated := false
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			cmdStr := cmd.String()
			if strings.Contains(cmdStr, "has-session") {
				if sessionCreated {
					return nil
				}
				return fmt.Errorf("session does not exist")
			}
			if strings.Contains(cmdStr, "new-session") {
				sessionCreated = true
				return nil
			}
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte(""), nil
		},
	}

	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   sessionName,
		Path:    workdir,
		Program: "bash",
		AutoYes: false,
	})
	require.NoError(t, err)

	ptyFactory := &MockPtyFactory{
		t:       t,
		cmdExec: cmdExec,
	}
	tmuxSession := tmux.NewTmuxSessionWithDeps(sessionName, "bash", ptyFactory, cmdExec)
	instance.SetTmuxSession(tmuxSession)

	err = instance.Start(true)
	require.NoError(t, err)

	return instance
}

// injectSession injects a mock tmux session into the TerminalPane's sessions map.
func injectSession(tp *TerminalPane, title string, ts *tmux.TmuxSession, worktreePath string) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.sessions[title] = &terminalSession{
		tmuxSession:  ts,
		worktreePath: worktreePath,
	}
	tp.currentTitle = title
}

func TestTerminalUpdateContent(t *testing.T) {
	log.Initialize(false)
	defer log.Close()

	expectedContent := "$ whoami\nuser\n$ ls\nfile1.txt  file2.txt"

	cmdExec := mockCmdExec(expectedContent, true)

	instance := makeStartedInstance(t, "update-content")
	defer func() { _ = instance.Kill() }()

	tp := NewTerminalPane()
	tp.SetSize(80, 30)

	// Inject a mock session that returns expectedContent on capture-pane
	ts := newMockTmuxSession(t, "mock-update", cmdExec)
	// Start the session so DoesSessionExist returns true
	injectSession(tp, instance.Title, ts, t.TempDir())

	// UpdateContent should set fallback=false and capture content
	err := tp.UpdateContent(instance)
	require.NoError(t, err)

	tp.mu.Lock()
	require.False(t, tp.fallback, "should not be in fallback mode after successful content update")
	require.Equal(t, expectedContent, tp.content, "content should match captured pane output")
	tp.mu.Unlock()

	// Verify String() output contains the content
	rendered := tp.String()
	require.Contains(t, rendered, "whoami", "rendered output should contain captured content")
}

func TestTerminalFallbackStates(t *testing.T) {
	log.Initialize(false)
	defer log.Close()

	tp := NewTerminalPane()
	tp.SetSize(80, 30)

	t.Run("nil instance", func(t *testing.T) {
		err := tp.UpdateContent(nil)
		require.NoError(t, err)

		tp.mu.Lock()
		require.True(t, tp.fallback, "should be in fallback mode for nil instance")
		require.Contains(t, tp.fallbackText, "Select an instance", "fallback text should prompt to select instance")
		require.Empty(t, tp.content, "content should be empty in fallback mode")
		tp.mu.Unlock()
	})

	t.Run("paused instance", func(t *testing.T) {
		// Create an instance without starting it, then set status to Paused.
		// UpdateContent checks Paused status before Started(), so no need to start.
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "paused-inst",
			Path:    t.TempDir(),
			Program: "bash",
		})
		require.NoError(t, err)
		instance.SetStatus(session.Paused)

		err = tp.UpdateContent(instance)
		require.NoError(t, err)

		tp.mu.Lock()
		require.True(t, tp.fallback, "should be in fallback mode for paused instance")
		require.Contains(t, tp.fallbackText, "paused", "fallback text should mention paused")
		tp.mu.Unlock()
	})

	t.Run("not started instance", func(t *testing.T) {
		// Create an instance that hasn't been started
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   "not-started",
			Path:    t.TempDir(),
			Program: "bash",
		})
		require.NoError(t, err)

		err = tp.UpdateContent(instance)
		require.NoError(t, err)

		tp.mu.Lock()
		require.True(t, tp.fallback, "should be in fallback mode for not-started instance")
		require.Contains(t, tp.fallbackText, "not started", "fallback text should indicate not started")
		tp.mu.Unlock()
	})
}

func TestTerminalSessionCaching(t *testing.T) {
	log.Initialize(false)
	defer log.Close()

	tp := NewTerminalPane()
	tp.SetSize(80, 30)

	content1 := "session-1-content"
	cmdExec1 := mockCmdExec(content1, true)
	ts1 := newMockTmuxSession(t, "cache-test-1", cmdExec1)

	content2 := "session-2-content"
	cmdExec2 := mockCmdExec(content2, true)
	ts2 := newMockTmuxSession(t, "cache-test-2", cmdExec2)

	instance1 := makeStartedInstance(t, "cache1")
	defer func() { _ = instance1.Kill() }()
	instance2 := makeStartedInstance(t, "cache2")
	defer func() { _ = instance2.Kill() }()

	// Inject two separate sessions
	injectSession(tp, instance1.Title, ts1, t.TempDir())

	tp.mu.Lock()
	tp.sessions[instance2.Title] = &terminalSession{
		tmuxSession:  ts2,
		worktreePath: t.TempDir(),
	}
	tp.mu.Unlock()

	// Switch to instance1 and capture
	tp.mu.Lock()
	tp.currentTitle = instance1.Title
	tp.mu.Unlock()

	err := tp.UpdateContent(instance1)
	require.NoError(t, err)
	tp.mu.Lock()
	require.Equal(t, content1, tp.content)
	tp.mu.Unlock()

	// Switch to instance2 and capture
	tp.mu.Lock()
	tp.currentTitle = instance2.Title
	tp.mu.Unlock()

	err = tp.UpdateContent(instance2)
	require.NoError(t, err)
	tp.mu.Lock()
	require.Equal(t, content2, tp.content)
	tp.mu.Unlock()

	// Switch back to instance1 — session should still exist (cached)
	tp.mu.Lock()
	tp.currentTitle = instance1.Title
	tp.mu.Unlock()

	err = tp.UpdateContent(instance1)
	require.NoError(t, err)
	tp.mu.Lock()
	require.Equal(t, content1, tp.content, "should get cached session content when switching back")
	// Verify both sessions are still in the map
	require.Len(t, tp.sessions, 2, "both sessions should be cached")
	tp.mu.Unlock()
}

func TestTerminalScrolling(t *testing.T) {
	log.Initialize(false)
	defer log.Close()

	// Create content with many lines for scrolling
	const numLines = 100
	lines := make([]string, numLines)
	for i := range numLines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	fullContent := strings.Join(lines, "\n")

	cmdExec := mockCmdExec(fullContent, true)
	instance := makeStartedInstance(t, "scroll")
	defer func() { _ = instance.Kill() }()

	tp := NewTerminalPane()
	tp.SetSize(80, 30)

	ts := newMockTmuxSession(t, "scroll-test", cmdExec)
	injectSession(tp, instance.Title, ts, t.TempDir())

	// Initially not scrolling
	require.False(t, tp.IsScrolling(), "should not be scrolling initially")

	// ScrollUp should enter scroll mode
	err := tp.ScrollUp()
	require.NoError(t, err)
	require.True(t, tp.IsScrolling(), "should be in scroll mode after ScrollUp")

	// Viewport should contain the content
	viewContent := tp.viewport.View()
	require.NotEmpty(t, viewContent, "viewport should have content in scroll mode")

	// ScrollDown should continue in scroll mode
	err = tp.ScrollDown()
	require.NoError(t, err)
	require.True(t, tp.IsScrolling(), "should still be in scroll mode after ScrollDown")

	// ResetToNormalMode should exit scroll mode
	tp.ResetToNormalMode()
	require.False(t, tp.IsScrolling(), "should not be scrolling after ResetToNormalMode")

	// Viewport content should be cleared
	tp.mu.Lock()
	require.False(t, tp.isScrolling, "isScrolling should be false")
	tp.mu.Unlock()
}

func TestTerminalCloseForInstance(t *testing.T) {
	log.Initialize(false)
	defer log.Close()

	tp := NewTerminalPane()
	tp.SetSize(80, 30)

	content := "some content"
	cmdExec := mockCmdExec(content, true)

	instance1 := makeStartedInstance(t, "close1")
	defer func() { _ = instance1.Kill() }()
	instance2 := makeStartedInstance(t, "close2")
	defer func() { _ = instance2.Kill() }()

	ts1 := newMockTmuxSession(t, "close-test-1", cmdExec)
	ts2 := newMockTmuxSession(t, "close-test-2", cmdExec)

	injectSession(tp, instance1.Title, ts1, t.TempDir())
	tp.mu.Lock()
	tp.sessions[instance2.Title] = &terminalSession{
		tmuxSession:  ts2,
		worktreePath: t.TempDir(),
	}
	tp.mu.Unlock()

	// Verify both sessions exist
	tp.mu.Lock()
	require.Len(t, tp.sessions, 2)
	tp.mu.Unlock()

	// Close instance1's session
	tp.CloseForInstance(instance1.Title)

	// Only instance2 should remain
	tp.mu.Lock()
	require.Len(t, tp.sessions, 1, "should have only 1 session after closing instance1")
	_, exists := tp.sessions[instance1.Title]
	require.False(t, exists, "instance1 session should be removed")
	_, exists = tp.sessions[instance2.Title]
	require.True(t, exists, "instance2 session should still exist")
	require.Empty(t, tp.currentTitle, "currentTitle should be cleared when closing current instance")
	tp.mu.Unlock()

	// Closing a non-existent instance should not panic
	tp.CloseForInstance("non-existent")

	tp.mu.Lock()
	require.Len(t, tp.sessions, 1, "non-existent close should not affect existing sessions")
	tp.mu.Unlock()
}
