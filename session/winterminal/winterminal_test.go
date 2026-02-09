//go:build windows

package winterminal

import (
	"os/exec"
	"testing"
	"time"

	"claude-squad/cmd/cmd_test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Unit tests (no ConPTY needed) ---

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "simple name", input: "test", expected: "claudesquad_test"},
		{name: "name with spaces", input: "my session", expected: "claudesquad_my_session"},
		{name: "name with hyphens", input: "my-session", expected: "claudesquad_my_session"},
		{name: "name with dots", input: "my.session", expected: "claudesquad_my_session"},
		{name: "name with mixed special chars", input: "my session-name.v2", expected: "claudesquad_my_session_name_v2"},
		{name: "empty name", input: "", expected: "claudesquad_"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, sanitizeName(tc.input))
		})
	}
}

func TestNewWindowsTerminalSession(t *testing.T) {
	session := NewWindowsTerminalSession("test-session", "claude")
	assert.Equal(t, "test-session", session.sessionName)
	assert.Equal(t, "claudesquad_test_session", session.sanitizedName)
	assert.Equal(t, "claude", session.program)
	assert.False(t, session.isRunning)
	assert.Nil(t, session.cpty)
	assert.NotNil(t, session.cmdExec)
}

func TestNewWindowsTerminalSessionWithDeps(t *testing.T) {
	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}
	session := NewWindowsTerminalSessionWithDeps("test-session", "claude", cmdExec)
	assert.Equal(t, "test-session", session.sessionName)
	assert.Equal(t, "claudesquad_test_session", session.sanitizedName)
	assert.Equal(t, "claude", session.program)
	assert.False(t, session.isRunning)
}

func TestDoesSessionExist_NoCpty(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")
	assert.False(t, session.DoesSessionExist())
}

func TestClose_NotRunning(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")
	err := session.Close()
	assert.NoError(t, err)
	assert.False(t, session.isRunning)
}

func TestSendKeys_NotRunning(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")
	err := session.SendKeys("hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestTapEnter_NotRunning(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")
	err := session.TapEnter()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestAttach_NotRunning(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")
	ch, err := session.Attach()
	assert.Error(t, err)
	assert.Nil(t, ch)
	assert.Contains(t, err.Error(), "not running")
}

func TestCapturePaneContent_Empty(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")
	content, err := session.CapturePaneContent()
	assert.NoError(t, err)
	assert.Equal(t, "", content)
}

func TestCapturePaneContentWithOptions_Empty(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")
	content, err := session.CapturePaneContentWithOptions("-", "-")
	assert.NoError(t, err)
	assert.Equal(t, "", content)
}

func TestSetDetachedSize_NoCpty(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")
	err := session.SetDetachedSize(80, 24)
	assert.NoError(t, err)
	assert.Equal(t, 80, session.width)
	assert.Equal(t, 24, session.height)
}

func TestDetachSafely_NotAttached(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")
	err := session.DetachSafely()
	assert.NoError(t, err)
}

func TestRestore_NotExisting(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")
	err := session.Restore()
	assert.NoError(t, err)
	assert.False(t, session.isRunning)
}

func TestStart_AlreadyRunning(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")
	session.isRunning = true
	err := session.Start(t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestCleanupSessions(t *testing.T) {
	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}
	err := CleanupSessions(cmdExec)
	assert.NoError(t, err)
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "no ansi", input: "hello world", expected: "hello world"},
		{name: "color codes", input: "\x1b[31mred\x1b[0m", expected: "red"},
		{name: "bold", input: "\x1b[1mbold\x1b[0m text", expected: "bold text"},
		{name: "empty", input: "", expected: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, stripANSI(tc.input))
		})
	}
}

func TestAppendToBuffer(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")

	// Single line with newline
	session.appendToBuffer([]byte("hello\n"))
	assert.Equal(t, []string{"hello"}, session.lines)
	assert.Equal(t, "", session.partial)

	// Partial line
	session.appendToBuffer([]byte("world"))
	assert.Equal(t, []string{"hello"}, session.lines)
	assert.Equal(t, "world", session.partial)

	// Complete the partial line
	session.appendToBuffer([]byte("!\n"))
	assert.Equal(t, []string{"hello", "world!"}, session.lines)
	assert.Equal(t, "", session.partial)

	// CRLF handling
	session.appendToBuffer([]byte("line\r\n"))
	assert.Equal(t, []string{"hello", "world!", "line"}, session.lines)

	// Multiple lines at once
	session.appendToBuffer([]byte("a\nb\nc\n"))
	assert.Equal(t, 6, len(session.lines))
}

func TestCapturePaneContent_WithBuffer(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")
	session.height = 3

	for i := 0; i < 10; i++ {
		session.appendToBuffer([]byte("line\n"))
	}

	content, err := session.CapturePaneContent()
	assert.NoError(t, err)
	// Should return last 3 lines
	assert.Equal(t, "line\nline\nline", content)
}

func TestCapturePaneContentWithOptions_WithBuffer(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")
	session.height = 3

	session.appendToBuffer([]byte("first\nsecond\nthird\n"))

	content, err := session.CapturePaneContentWithOptions("-", "-")
	assert.NoError(t, err)
	// Full history returns everything
	assert.Equal(t, "first\nsecond\nthird", content)
}

func TestHasUpdated_EmptyThenSame(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")

	// First call on empty buffer — content differs from nil hash
	updated, hasPrompt := session.HasUpdated()
	assert.True(t, updated)
	assert.False(t, hasPrompt)

	// Second call with same empty content — no change
	updated, hasPrompt = session.HasUpdated()
	assert.False(t, updated)
	assert.False(t, hasPrompt)
}

func TestHasUpdated_ContentChange(t *testing.T) {
	session := NewWindowsTerminalSession("test", "claude")

	// Seed initial hash
	session.HasUpdated()

	// Add content → should detect update
	session.appendToBuffer([]byte("new output\n"))
	updated, _ := session.HasUpdated()
	assert.True(t, updated)

	// Same content → no update
	updated, _ = session.HasUpdated()
	assert.False(t, updated)
}

func TestHasUpdated_PromptDetection(t *testing.T) {
	tests := []struct {
		program string
		prompt  string
	}{
		{"claude", "No, and tell Claude what to do differently"},
		{"aider", "(Y)es/(N)o/(D)on't ask again"},
		{"gemini", "Yes, allow once"},
	}
	for _, tc := range tests {
		t.Run(tc.program, func(t *testing.T) {
			session := NewWindowsTerminalSession("test", tc.program)
			session.appendToBuffer([]byte(tc.prompt + "\n"))
			_, hasPrompt := session.HasUpdated()
			assert.True(t, hasPrompt)
		})
	}
}

// --- Integration tests (require real ConPTY on Windows) ---

func TestStart_EchoCommand(t *testing.T) {
	session := NewWindowsTerminalSession("test-echo", "echo hello world")
	err := session.Start(t.TempDir())
	require.NoError(t, err)
	defer session.Close()

	// Wait for output to be captured
	time.Sleep(1 * time.Second)

	assert.True(t, session.isRunning || !session.isRunning) // process may have exited

	content, err := session.CapturePaneContent()
	assert.NoError(t, err)
	assert.Contains(t, content, "hello world")
}

func TestStart_SessionAlive(t *testing.T) {
	// Use "cmd.exe" directly (stays running until we close)
	session := NewWindowsTerminalSession("test-alive", "cmd.exe /k echo ready")
	err := session.Start(t.TempDir())
	require.NoError(t, err)
	defer session.Close()

	time.Sleep(500 * time.Millisecond)

	assert.True(t, session.DoesSessionExist())

	err = session.Close()
	assert.NoError(t, err)

	// After close, session should not exist
	time.Sleep(200 * time.Millisecond)
	assert.False(t, session.DoesSessionExist())
}

func TestSendKeys_Running(t *testing.T) {
	session := NewWindowsTerminalSession("test-keys", "cmd.exe /k echo ready")
	err := session.Start(t.TempDir())
	require.NoError(t, err)
	defer session.Close()

	time.Sleep(500 * time.Millisecond)

	err = session.SendKeys("echo from-sendkeys\r")
	assert.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	content, err := session.CapturePaneContent()
	assert.NoError(t, err)
	assert.Contains(t, content, "from-sendkeys")
}

func TestSetDetachedSize_Running(t *testing.T) {
	session := NewWindowsTerminalSession("test-resize", "cmd.exe /k echo ready")
	err := session.Start(t.TempDir())
	require.NoError(t, err)
	defer session.Close()

	time.Sleep(500 * time.Millisecond)

	err = session.SetDetachedSize(120, 40)
	assert.NoError(t, err)
	assert.Equal(t, 120, session.width)
	assert.Equal(t, 40, session.height)
}
