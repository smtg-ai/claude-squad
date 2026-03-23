package tmux

import (
	"os/exec"
	"testing"
)

func TestConnectPTY(t *testing.T) {
	// Create a real tmux session for testing
	sessionName := "test_connectpty"
	tmuxName := toClaudeSquadTmuxName(sessionName)

	// Clean up any leftover session
	_ = exec.Command("tmux", "kill-session", "-t", tmuxName).Run()

	ts := NewTmuxSession(sessionName, "bash")
	tmpDir := t.TempDir()
	if err := ts.Start(tmpDir); err != nil {
		t.Fatalf("failed to start tmux session: %v", err)
	}
	defer ts.Close()

	// ConnectPTY should return a valid file
	ptmx, err := ts.ConnectPTY()
	if err != nil {
		t.Fatalf("ConnectPTY failed: %v", err)
	}
	if ptmx == nil {
		t.Fatal("ConnectPTY returned nil file")
	}

	// Should be able to write to the PTY
	_, err = ptmx.Write([]byte("echo hello\n"))
	if err != nil {
		t.Fatalf("failed to write to PTY: %v", err)
	}

	// DisconnectPTY should clean up without error
	if err := ts.DisconnectPTY(ptmx); err != nil {
		t.Fatalf("DisconnectPTY failed: %v", err)
	}

	// Session should still exist after disconnect
	if !ts.DoesSessionExist() {
		t.Fatal("tmux session should still exist after DisconnectPTY")
	}
}
