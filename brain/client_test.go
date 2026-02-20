package brain

import (
	"path/filepath"
	"testing"
)

func TestClientPing(t *testing.T) {
	s := startTestServer(t)
	c := NewClient(s.SocketPath())

	if err := c.Ping(); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestClientPingNoServer(t *testing.T) {
	c := NewClient(filepath.Join(t.TempDir(), "nonexistent.sock"))
	if err := c.Ping(); err == nil {
		t.Fatal("expected error when server not running")
	}
}

func TestClientUpdateAndGetBrain(t *testing.T) {
	s := startTestServer(t)
	c := NewClient(s.SocketPath())

	result, err := c.UpdateStatus("/repo", "agent-1", "implement auth", []string{"auth.go"})
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if len(result.Conflicts) != 0 {
		t.Errorf("expected no conflicts, got %v", result.Conflicts)
	}

	state, err := c.GetBrain("/repo", "agent-1")
	if err != nil {
		t.Fatalf("GetBrain: %v", err)
	}
	if len(state.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(state.Agents))
	}
	if state.Agents["agent-1"].Feature != "implement auth" {
		t.Errorf("Feature = %q, want %q", state.Agents["agent-1"].Feature, "implement auth")
	}
}

func TestClientSendMessage(t *testing.T) {
	s := startTestServer(t)
	c := NewClient(s.SocketPath())

	if err := c.SendMessage("/repo", "agent-1", "agent-2", "heads up"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	state, err := c.GetBrain("/repo", "agent-2")
	if err != nil {
		t.Fatalf("GetBrain: %v", err)
	}
	if len(state.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(state.Messages))
	}
}

func TestClientRemoveAgent(t *testing.T) {
	s := startTestServer(t)
	c := NewClient(s.SocketPath())

	c.UpdateStatus("/repo", "agent-1", "work", nil)
	if err := c.RemoveAgent("/repo", "agent-1"); err != nil {
		t.Fatalf("RemoveAgent: %v", err)
	}

	state, err := c.GetBrain("/repo", "agent-1")
	if err != nil {
		t.Fatalf("GetBrain: %v", err)
	}
	if len(state.Agents) != 0 {
		t.Errorf("expected 0 agents after removal, got %d", len(state.Agents))
	}
}

func TestClientConflictDetection(t *testing.T) {
	s := startTestServer(t)
	c := NewClient(s.SocketPath())

	c.UpdateStatus("/repo", "agent-1", "auth", []string{"auth.go"})
	result, err := c.UpdateStatus("/repo", "agent-2", "auth fix", []string{"auth.go"})
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if len(result.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(result.Conflicts))
	}
}
