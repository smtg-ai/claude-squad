package mcp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ByteMirror/hivemind/brain"

	gomcp "github.com/mark3labs/mcp-go/mcp"
)

// resultText extracts the text string from a CallToolResult.
// It assumes the result contains exactly one TextContent item.
func resultText(t *testing.T, result *gomcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := gomcp.AsTextContent(result.Content[0])
	if !ok {
		t.Fatalf("result content[0] is not TextContent: %T", result.Content[0])
	}
	return tc.Text
}

func TestHandleListInstances(t *testing.T) {
	tests := []struct {
		name      string
		stateJSON string
		wantErr   bool   // tool-level isError flag
		contains  string // substring to look for in result text
		checkJSON func(t *testing.T, text string)
	}{
		{
			name: "returns instance data as JSON",
			stateJSON: `{
				"instances": [
					{
						"title": "my-task",
						"path": "/repo",
						"branch": "user/my-task",
						"status": 0,
						"program": "claude",
						"topic_name": "refactor",
						"worktree": {
							"repo_path": "/repo",
							"worktree_path": "/home/.hivemind/worktrees/my-task",
							"session_name": "my-task",
							"branch_name": "user/my-task",
							"base_commit_sha": "abc123"
						},
						"diff_stats": { "added": 15, "removed": 3 }
					}
				]
			}`,
			checkJSON: func(t *testing.T, text string) {
				t.Helper()
				var views []instanceView
				if err := json.Unmarshal([]byte(text), &views); err != nil {
					t.Fatalf("failed to parse JSON response: %v", err)
				}
				if len(views) != 1 {
					t.Fatalf("len(views) = %d, want 1", len(views))
				}
				v := views[0]
				if v.Title != "my-task" {
					t.Errorf("Title = %q, want %q", v.Title, "my-task")
				}
				if v.Branch != "user/my-task" {
					t.Errorf("Branch = %q, want %q", v.Branch, "user/my-task")
				}
				if v.Status != "running" {
					t.Errorf("Status = %q, want %q", v.Status, "running")
				}
				if v.Program != "claude" {
					t.Errorf("Program = %q, want %q", v.Program, "claude")
				}
				if v.TopicName != "refactor" {
					t.Errorf("TopicName = %q, want %q", v.TopicName, "refactor")
				}
				if v.Path != "/repo" {
					t.Errorf("Path = %q, want %q", v.Path, "/repo")
				}
				if v.DiffStats.Added != 15 {
					t.Errorf("DiffStats.Added = %d, want 15", v.DiffStats.Added)
				}
				if v.DiffStats.Removed != 3 {
					t.Errorf("DiffStats.Removed = %d, want 3", v.DiffStats.Removed)
				}
			},
		},
		{
			name:      "no instances returns message",
			stateJSON: `{"instances": []}`,
			contains:  "No Hivemind instances found for this repository.",
		},
		{
			name:      "missing state file returns message",
			stateJSON: "", // no file created
			contains:  "No Hivemind instances found for this repository.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.stateJSON != "" {
				statePath := filepath.Join(tmpDir, "state.json")
				if err := os.WriteFile(statePath, []byte(tt.stateJSON), 0600); err != nil {
					t.Fatalf("failed to write state file: %v", err)
				}
			}

			reader := NewStateReader(tmpDir)
			handler := handleListInstances(reader, "/repo")

			req := gomcp.CallToolRequest{}
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}

			if tt.wantErr && !result.IsError {
				t.Fatal("expected IsError=true, got false")
			}

			text := resultText(t, result)

			if tt.contains != "" {
				if !strings.Contains(text, tt.contains) {
					t.Errorf("result text %q does not contain %q", text, tt.contains)
				}
			}

			if tt.checkJSON != nil {
				tt.checkJSON(t, text)
			}
		})
	}
}

func TestHandleUpdateStatus(t *testing.T) {
	t.Run("sets agent status", func(t *testing.T) {
		tmpDir := t.TempDir()
		handler := handleUpdateStatus(NewFileBrainClient(tmpDir), "/test-repo", "agent-1")

		req := gomcp.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{
			"feature": "implement auth",
			"files":   "auth.go, middleware.go",
		}

		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		text := resultText(t, result)
		if !strings.Contains(text, "No conflicts") {
			t.Errorf("expected 'No conflicts', got: %s", text)
		}

		// Verify brain.json was written
		brain, err := readBrain(tmpDir, "/test-repo")
		if err != nil {
			t.Fatalf("failed to read brain: %v", err)
		}
		agent, ok := brain.Agents["agent-1"]
		if !ok {
			t.Fatal("agent-1 not found in brain")
		}
		if agent.Feature != "implement auth" {
			t.Errorf("Feature = %q, want %q", agent.Feature, "implement auth")
		}
		if len(agent.Files) != 2 {
			t.Fatalf("len(Files) = %d, want 2", len(agent.Files))
		}
		if agent.Files[0] != "auth.go" {
			t.Errorf("Files[0] = %q, want %q", agent.Files[0], "auth.go")
		}
		if agent.Files[1] != "middleware.go" {
			t.Errorf("Files[1] = %q, want %q", agent.Files[1], "middleware.go")
		}
	})

	t.Run("detects file conflicts", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Pre-populate brain with another agent working on auth.go
		brain := &brainFile{
			Agents: map[string]*agentStatus{
				"agent-2": {
					Feature:   "fix auth bug",
					Files:     []string{"auth.go"},
					UpdatedAt: time.Now().UTC().Format(time.RFC3339),
				},
			},
		}
		if err := writeBrain(tmpDir, "/test-repo", brain); err != nil {
			t.Fatal(err)
		}

		handler := handleUpdateStatus(NewFileBrainClient(tmpDir), "/test-repo", "agent-1")
		req := gomcp.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{
			"feature": "implement auth",
			"files":   "auth.go",
		}

		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		text := resultText(t, result)
		if !strings.Contains(text, "Conflicts detected") {
			t.Errorf("expected conflict warning, got: %s", text)
		}
		if !strings.Contains(text, "agent-2") {
			t.Errorf("expected agent-2 in conflict warning, got: %s", text)
		}
	})

	t.Run("missing feature returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		handler := handleUpdateStatus(NewFileBrainClient(tmpDir), "/test-repo", "agent-1")

		req := gomcp.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{}

		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected IsError=true for missing feature")
		}
	})

	t.Run("works without files parameter", func(t *testing.T) {
		tmpDir := t.TempDir()
		handler := handleUpdateStatus(NewFileBrainClient(tmpDir), "/test-repo", "agent-1")

		req := gomcp.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{
			"feature": "research task",
		}

		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		if result.IsError {
			t.Fatal("unexpected error for status without files")
		}

		brain, err := readBrain(tmpDir, "/test-repo")
		if err != nil {
			t.Fatal(err)
		}
		if len(brain.Agents["agent-1"].Files) != 0 {
			t.Errorf("expected 0 files, got %d", len(brain.Agents["agent-1"].Files))
		}
	})
}

func TestHandleGetBrain(t *testing.T) {
	t.Run("returns empty brain when no file exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		handler := handleGetBrain(NewFileBrainClient(tmpDir), "/test-repo", "agent-1")

		req := gomcp.CallToolRequest{}
		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}

		text := resultText(t, result)
		var view brain.BrainState
		if err := json.Unmarshal([]byte(text), &view); err != nil {
			t.Fatalf("failed to parse brain view: %v", err)
		}
		if len(view.Agents) != 0 {
			t.Errorf("expected 0 agents, got %d", len(view.Agents))
		}
	})

	t.Run("returns agents and filtered messages", func(t *testing.T) {
		tmpDir := t.TempDir()

		bf := &brainFile{
			Agents: map[string]*agentStatus{
				"agent-1": {
					Feature:   "auth",
					Files:     []string{"auth.go"},
					UpdatedAt: time.Now().UTC().Format(time.RFC3339),
				},
				"agent-2": {
					Feature:   "tests",
					Files:     []string{"auth_test.go"},
					UpdatedAt: time.Now().UTC().Format(time.RFC3339),
				},
			},
			Messages: []brainMessage{
				{From: "agent-2", To: "agent-1", Content: "I'll handle tests", Timestamp: time.Now().UTC().Format(time.RFC3339)},
				{From: "agent-1", To: "agent-2", Content: "Sounds good", Timestamp: time.Now().UTC().Format(time.RFC3339)},
				{From: "agent-2", To: "", Content: "Broadcast to all", Timestamp: time.Now().UTC().Format(time.RFC3339)},
			},
		}
		if err := writeBrain(tmpDir, "/test-repo", bf); err != nil {
			t.Fatal(err)
		}

		handler := handleGetBrain(NewFileBrainClient(tmpDir), "/test-repo", "agent-1")
		req := gomcp.CallToolRequest{}
		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}

		text := resultText(t, result)
		var view brain.BrainState
		if err := json.Unmarshal([]byte(text), &view); err != nil {
			t.Fatalf("failed to parse brain view: %v", err)
		}

		if len(view.Agents) != 2 {
			t.Errorf("expected 2 agents, got %d", len(view.Agents))
		}

		// agent-1 should see: message to agent-1 + broadcast, NOT message to agent-2
		if len(view.Messages) != 2 {
			t.Fatalf("expected 2 messages for agent-1, got %d", len(view.Messages))
		}
		if view.Messages[0].Content != "I'll handle tests" {
			t.Errorf("Messages[0].Content = %q, want %q", view.Messages[0].Content, "I'll handle tests")
		}
		if view.Messages[1].Content != "Broadcast to all" {
			t.Errorf("Messages[1].Content = %q, want %q", view.Messages[1].Content, "Broadcast to all")
		}
	})

	t.Run("prunes stale agents", func(t *testing.T) {
		tmpDir := t.TempDir()

		bf := &brainFile{
			Agents: map[string]*agentStatus{
				"fresh": {
					Feature:   "active work",
					UpdatedAt: time.Now().UTC().Format(time.RFC3339),
				},
				"stale": {
					Feature:   "old work",
					UpdatedAt: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
				},
			},
		}
		if err := writeBrain(tmpDir, "/test-repo", bf); err != nil {
			t.Fatal(err)
		}

		handler := handleGetBrain(NewFileBrainClient(tmpDir), "/test-repo", "fresh")
		req := gomcp.CallToolRequest{}
		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}

		text := resultText(t, result)
		var view brain.BrainState
		if err := json.Unmarshal([]byte(text), &view); err != nil {
			t.Fatalf("failed to parse brain view: %v", err)
		}

		if len(view.Agents) != 1 {
			t.Fatalf("expected 1 agent after pruning, got %d", len(view.Agents))
		}
		if _, ok := view.Agents["fresh"]; !ok {
			t.Error("expected 'fresh' agent to survive pruning")
		}
	})
}

func TestHandleSendMessage(t *testing.T) {
	t.Run("sends directed message", func(t *testing.T) {
		tmpDir := t.TempDir()
		handler := handleSendMessage(NewFileBrainClient(tmpDir), "/test-repo", "agent-1")

		req := gomcp.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{
			"to":      "agent-2",
			"message": "Don't touch auth.go, I'm refactoring it",
		}

		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		text := resultText(t, result)
		if !strings.Contains(text, "agent-2") {
			t.Errorf("expected confirmation mentioning agent-2, got: %s", text)
		}

		brain, err := readBrain(tmpDir, "/test-repo")
		if err != nil {
			t.Fatal(err)
		}
		if len(brain.Messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(brain.Messages))
		}
		if brain.Messages[0].From != "agent-1" {
			t.Errorf("From = %q, want %q", brain.Messages[0].From, "agent-1")
		}
		if brain.Messages[0].To != "agent-2" {
			t.Errorf("To = %q, want %q", brain.Messages[0].To, "agent-2")
		}
	})

	t.Run("broadcasts when to is empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		handler := handleSendMessage(NewFileBrainClient(tmpDir), "/test-repo", "agent-1")

		req := gomcp.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{
			"message": "Config format changed, update your parsers",
		}

		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		text := resultText(t, result)
		if !strings.Contains(text, "all agents") {
			t.Errorf("expected confirmation mentioning 'all agents', got: %s", text)
		}

		brain, err := readBrain(tmpDir, "/test-repo")
		if err != nil {
			t.Fatal(err)
		}
		if brain.Messages[0].To != "" {
			t.Errorf("To = %q, want empty for broadcast", brain.Messages[0].To)
		}
	})

	t.Run("missing message returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		handler := handleSendMessage(NewFileBrainClient(tmpDir), "/test-repo", "agent-1")

		req := gomcp.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{}

		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected IsError=true for missing message")
		}
	})

	t.Run("caps messages at 50", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Pre-populate with 49 messages
		brain := &brainFile{Agents: make(map[string]*agentStatus)}
		for i := 0; i < 49; i++ {
			brain.Messages = append(brain.Messages, brainMessage{
				From: "old", To: "", Content: "old msg", Timestamp: time.Now().UTC().Format(time.RFC3339),
			})
		}
		if err := writeBrain(tmpDir, "/test-repo", brain); err != nil {
			t.Fatal(err)
		}

		// Send 2 more messages (total 51, should be capped to 50)
		handler := handleSendMessage(NewFileBrainClient(tmpDir), "/test-repo", "agent-1")
		for _, msg := range []string{"msg-50", "msg-51"} {
			req := gomcp.CallToolRequest{}
			req.Params.Arguments = map[string]interface{}{"message": msg}
			if _, err := handler(context.Background(), req); err != nil {
				t.Fatal(err)
			}
		}

		brain, err := readBrain(tmpDir, "/test-repo")
		if err != nil {
			t.Fatal(err)
		}
		if len(brain.Messages) != 50 {
			t.Fatalf("expected 50 messages after cap, got %d", len(brain.Messages))
		}
		// The last message should be "msg-51"
		if brain.Messages[49].Content != "msg-51" {
			t.Errorf("last message = %q, want %q", brain.Messages[49].Content, "msg-51")
		}
	})
}

// setupGitWorktreeForTest creates a temporary git repo with a branch and state.json
// pointing at it. Returns (hivemindDir, worktreePath, baseCommitSHA).
func setupGitWorktreeForTest(t *testing.T, instanceID string) (string, string, string) {
	t.Helper()

	repoDir := t.TempDir()

	// Initialize git repo and make a base commit.
	cmds := [][]string{
		{"git", "-C", repoDir, "init"},
		{"git", "-C", repoDir, "config", "user.email", "test@test.com"},
		{"git", "-C", repoDir, "config", "user.name", "Test"},
	}
	for _, c := range cmds {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("cmd %v failed: %s (%v)", c, out, err)
		}
	}

	// Create initial file and commit.
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][]string{
		{"git", "-C", repoDir, "add", "."},
		{"git", "-C", repoDir, "commit", "-m", "initial"},
	} {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("cmd %v failed: %s (%v)", c, out, err)
		}
	}

	// Get base commit SHA.
	out, err := exec.Command("git", "-C", repoDir, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse failed: %s (%v)", out, err)
	}
	baseSHA := strings.TrimSpace(string(out))

	// Create branch and add a change.
	branch := "test-branch"
	for _, c := range [][]string{
		{"git", "-C", repoDir, "checkout", "-b", branch},
	} {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("cmd %v failed: %s (%v)", c, out, err)
		}
	}

	if err := os.WriteFile(filepath.Join(repoDir, "new.go"), []byte("package main\n// new file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][]string{
		{"git", "-C", repoDir, "add", "."},
		{"git", "-C", repoDir, "commit", "-m", "add new file"},
	} {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("cmd %v failed: %s (%v)", c, out, err)
		}
	}

	// Write state.json with instance pointing at this repo as its worktree.
	hivemindDir := t.TempDir()
	stateJSON := `{
		"instances": [
			{
				"title": "` + instanceID + `",
				"path": "` + repoDir + `",
				"branch": "` + branch + `",
				"status": 0,
				"program": "claude",
				"worktree": {
					"repo_path": "` + repoDir + `",
					"worktree_path": "` + repoDir + `",
					"session_name": "` + instanceID + `",
					"branch_name": "` + branch + `",
					"base_commit_sha": "` + baseSHA + `"
				},
				"diff_stats": { "added": 2, "removed": 0 }
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(hivemindDir, "state.json"), []byte(stateJSON), 0600); err != nil {
		t.Fatal(err)
	}

	return hivemindDir, repoDir, baseSHA
}

func TestHandleGetMySessionSummary(t *testing.T) {
	t.Run("returns summary with git data", func(t *testing.T) {
		hivemindDir, _, _ := setupGitWorktreeForTest(t, "agent-1")
		reader := NewStateReader(hivemindDir)
		handler := handleGetMySessionSummary(reader, "agent-1")

		req := gomcp.CallToolRequest{}
		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}

		text := resultText(t, result)

		var summary struct {
			Title        string `json:"title"`
			Branch       string `json:"branch"`
			Status       string `json:"status"`
			ChangedFiles string `json:"changed_files"`
			Commits      string `json:"commits"`
		}
		if err := json.Unmarshal([]byte(text), &summary); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if summary.Title != "agent-1" {
			t.Errorf("Title = %q, want %q", summary.Title, "agent-1")
		}
		if summary.Branch != "test-branch" {
			t.Errorf("Branch = %q, want %q", summary.Branch, "test-branch")
		}
		if !strings.Contains(summary.ChangedFiles, "new.go") {
			t.Errorf("ChangedFiles %q should contain 'new.go'", summary.ChangedFiles)
		}
		if !strings.Contains(summary.Commits, "add new file") {
			t.Errorf("Commits %q should contain 'add new file'", summary.Commits)
		}
	})

	t.Run("unknown instance returns error", func(t *testing.T) {
		hivemindDir, _, _ := setupGitWorktreeForTest(t, "agent-1")
		reader := NewStateReader(hivemindDir)
		handler := handleGetMySessionSummary(reader, "nonexistent")

		req := gomcp.CallToolRequest{}
		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected IsError=true for unknown instance")
		}
	})

	t.Run("empty instanceID returns error", func(t *testing.T) {
		hivemindDir, _, _ := setupGitWorktreeForTest(t, "agent-1")
		reader := NewStateReader(hivemindDir)
		handler := handleGetMySessionSummary(reader, "")

		req := gomcp.CallToolRequest{}
		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected IsError=true for empty instanceID")
		}
		text := resultText(t, result)
		if !strings.Contains(text, "HIVEMIND_INSTANCE_ID") {
			t.Errorf("error text %q should mention HIVEMIND_INSTANCE_ID", text)
		}
	})
}

func TestHandleGetMyDiff(t *testing.T) {
	t.Run("returns diff output", func(t *testing.T) {
		hivemindDir, _, _ := setupGitWorktreeForTest(t, "agent-1")
		reader := NewStateReader(hivemindDir)
		handler := handleGetMyDiff(reader, "agent-1")

		req := gomcp.CallToolRequest{}
		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}

		text := resultText(t, result)
		if !strings.Contains(text, "new.go") {
			t.Errorf("diff %q should mention 'new.go'", text)
		}
	})

	t.Run("unknown instance returns error", func(t *testing.T) {
		hivemindDir, _, _ := setupGitWorktreeForTest(t, "agent-1")
		reader := NewStateReader(hivemindDir)
		handler := handleGetMyDiff(reader, "nonexistent")

		req := gomcp.CallToolRequest{}
		result, err := handler(context.Background(), req)
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected IsError=true for unknown instance")
		}
	})
}
