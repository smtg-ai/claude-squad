package brain

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestManagerGetBrainEmpty(t *testing.T) {
	m := NewManager()
	state := m.GetBrain("/repo", "agent-1")
	if len(state.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(state.Agents))
	}
	if len(state.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(state.Messages))
	}
}

func TestManagerUpdateStatus(t *testing.T) {
	m := NewManager()

	result := m.UpdateStatus("/repo", "agent-1", "implement auth", []string{"auth.go", "middleware.go"})
	if len(result.Conflicts) != 0 {
		t.Errorf("expected no conflicts, got %v", result.Conflicts)
	}

	state := m.GetBrain("/repo", "agent-1")
	if len(state.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(state.Agents))
	}
	agent := state.Agents["agent-1"]
	if agent.Feature != "implement auth" {
		t.Errorf("Feature = %q, want %q", agent.Feature, "implement auth")
	}
	if len(agent.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(agent.Files))
	}
}

func TestManagerConflictDetection(t *testing.T) {
	m := NewManager()

	m.UpdateStatus("/repo", "agent-1", "auth feature", []string{"auth.go", "config.go"})
	result := m.UpdateStatus("/repo", "agent-2", "auth refactor", []string{"auth.go"})

	if len(result.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d: %v", len(result.Conflicts), result.Conflicts)
	}
	if result.Conflicts[0] == "" {
		t.Error("conflict string should not be empty")
	}
}

func TestManagerSendMessage(t *testing.T) {
	m := NewManager()

	m.SendMessage("/repo", "agent-1", "agent-2", "hello")
	m.SendMessage("/repo", "agent-1", "", "broadcast")

	// agent-2 should see: directed message + broadcast
	state := m.GetBrain("/repo", "agent-2")
	if len(state.Messages) != 2 {
		t.Fatalf("expected 2 messages for agent-2, got %d", len(state.Messages))
	}

	// agent-3 should see: only broadcast
	state = m.GetBrain("/repo", "agent-3")
	if len(state.Messages) != 1 {
		t.Fatalf("expected 1 message for agent-3, got %d", len(state.Messages))
	}
	if state.Messages[0].Content != "broadcast" {
		t.Errorf("expected broadcast message, got %q", state.Messages[0].Content)
	}
}

func TestManagerMessageCap(t *testing.T) {
	m := NewManager()

	for i := 0; i < 60; i++ {
		m.SendMessage("/repo", "agent-1", "", fmt.Sprintf("msg-%d", i))
	}

	state := m.GetBrain("/repo", "agent-1")
	if len(state.Messages) != maxMessages {
		t.Fatalf("expected %d messages, got %d", maxMessages, len(state.Messages))
	}
	// Oldest should be msg-10 (60 - 50 = 10)
	if state.Messages[0].Content != "msg-10" {
		t.Errorf("first message = %q, want %q", state.Messages[0].Content, "msg-10")
	}
}

func TestManagerRemoveAgent(t *testing.T) {
	m := NewManager()

	m.UpdateStatus("/repo", "agent-1", "work", nil)
	m.UpdateStatus("/repo", "agent-2", "work", nil)

	m.RemoveAgent("/repo", "agent-1")

	state := m.GetBrain("/repo", "agent-2")
	if len(state.Agents) != 1 {
		t.Fatalf("expected 1 agent after removal, got %d", len(state.Agents))
	}
	if _, ok := state.Agents["agent-2"]; !ok {
		t.Error("expected agent-2 to remain")
	}
}

func TestManagerPrunesStaleAgents(t *testing.T) {
	m := NewManager()

	// Insert an agent with a stale timestamp directly.
	rs := m.getOrCreateRepo("/repo")
	rs.agents["stale"] = &AgentStatus{
		Feature:   "old work",
		UpdatedAt: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
	}
	rs.agents["fresh"] = &AgentStatus{
		Feature:   "new work",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	state := m.GetBrain("/repo", "fresh")
	if len(state.Agents) != 1 {
		t.Fatalf("expected 1 agent after pruning, got %d", len(state.Agents))
	}
	if _, ok := state.Agents["fresh"]; !ok {
		t.Error("expected 'fresh' agent to survive pruning")
	}
}

func TestManagerConcurrentAccess(t *testing.T) {
	m := NewManager()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			agentID := fmt.Sprintf("agent-%d", id)
			m.UpdateStatus("/repo", agentID, "feature", []string{"file.go"})
			m.GetBrain("/repo", agentID)
			m.SendMessage("/repo", agentID, "", "hello")
		}(i)
	}
	wg.Wait()

	state := m.GetBrain("/repo", "agent-0")
	if len(state.Agents) != 20 {
		t.Errorf("expected 20 agents, got %d", len(state.Agents))
	}
}

func TestManagerRepoIsolation(t *testing.T) {
	m := NewManager()

	m.UpdateStatus("/repo-a", "agent-1", "work-a", nil)
	m.UpdateStatus("/repo-b", "agent-2", "work-b", nil)

	stateA := m.GetBrain("/repo-a", "agent-1")
	stateB := m.GetBrain("/repo-b", "agent-2")

	if len(stateA.Agents) != 1 {
		t.Errorf("repo-a: expected 1 agent, got %d", len(stateA.Agents))
	}
	if len(stateB.Agents) != 1 {
		t.Errorf("repo-b: expected 1 agent, got %d", len(stateB.Agents))
	}
	if _, ok := stateA.Agents["agent-2"]; ok {
		t.Error("repo-a should not contain agent-2")
	}
}
