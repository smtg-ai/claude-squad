package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
			contains:  "No Hivemind instances found.",
		},
		{
			name:      "missing state file returns message",
			stateJSON: "", // no file created
			contains:  "No Hivemind instances found.",
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
			handler := handleListInstances(reader)

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

func TestHandleCheckFileActivity(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]interface{}
		wantErr   bool   // tool-level isError flag
		contains  string // substring to look for in result text
		checkJSON func(t *testing.T, text string)
	}{
		{
			name: "returns checked files in response",
			args: map[string]interface{}{
				"files": "auth.go, main.go",
			},
			checkJSON: func(t *testing.T, text string) {
				t.Helper()
				var result struct {
					FilesChecked []string `json:"files_checked"`
					Conflicts    []string `json:"conflicts"`
					Message      string   `json:"message"`
				}
				if err := json.Unmarshal([]byte(text), &result); err != nil {
					t.Fatalf("failed to parse JSON response: %v", err)
				}
				if len(result.FilesChecked) != 2 {
					t.Fatalf("len(FilesChecked) = %d, want 2", len(result.FilesChecked))
				}
				if result.FilesChecked[0] != "auth.go" {
					t.Errorf("FilesChecked[0] = %q, want %q", result.FilesChecked[0], "auth.go")
				}
				if result.FilesChecked[1] != "main.go" {
					t.Errorf("FilesChecked[1] = %q, want %q", result.FilesChecked[1], "main.go")
				}
				if len(result.Conflicts) != 0 {
					t.Errorf("len(Conflicts) = %d, want 0", len(result.Conflicts))
				}
				if result.Message == "" {
					t.Error("Message is empty, expected non-empty")
				}
			},
		},
		{
			name:     "missing files param returns error",
			args:     map[string]interface{}{},
			wantErr:  true,
			contains: "missing required parameter: files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handleCheckFileActivity()

			req := gomcp.CallToolRequest{}
			req.Params.Arguments = tt.args

			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}

			text := resultText(t, result)

			if tt.wantErr {
				if !result.IsError {
					t.Fatalf("expected IsError=true, got false; text: %s", text)
				}
			}

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

func TestHandleGetSharedContext(t *testing.T) {
	tests := []struct {
		name        string
		contextJSON string // if empty, no file is created
		createFile  bool
		wantErr     bool
		contains    string
		checkJSON   func(t *testing.T, text string)
	}{
		{
			name:       "no shared context file returns empty array",
			createFile: false,
			contains:   "[]",
		},
		{
			name:       "shared context with entries",
			createFile: true,
			contextJSON: `[
				{
					"instance_id": "agent-1",
					"type": "discovery",
					"content": "Found rate limiter in middleware.go",
					"timestamp": "2026-02-20T10:00:00Z"
				},
				{
					"instance_id": "agent-2",
					"type": "decision",
					"content": "Using repository pattern for data access"
				}
			]`,
			checkJSON: func(t *testing.T, text string) {
				t.Helper()
				var entries []sharedContextEntry
				if err := json.Unmarshal([]byte(text), &entries); err != nil {
					t.Fatalf("failed to parse JSON response: %v", err)
				}
				if len(entries) != 2 {
					t.Fatalf("len(entries) = %d, want 2", len(entries))
				}
				if entries[0].InstanceID != "agent-1" {
					t.Errorf("entries[0].InstanceID = %q, want %q", entries[0].InstanceID, "agent-1")
				}
				if entries[0].Type != "discovery" {
					t.Errorf("entries[0].Type = %q, want %q", entries[0].Type, "discovery")
				}
				if entries[0].Content != "Found rate limiter in middleware.go" {
					t.Errorf("entries[0].Content = %q, want %q", entries[0].Content, "Found rate limiter in middleware.go")
				}
				if entries[0].Timestamp != "2026-02-20T10:00:00Z" {
					t.Errorf("entries[0].Timestamp = %q, want %q", entries[0].Timestamp, "2026-02-20T10:00:00Z")
				}
				if entries[1].InstanceID != "agent-2" {
					t.Errorf("entries[1].InstanceID = %q, want %q", entries[1].InstanceID, "agent-2")
				}
				if entries[1].Timestamp != "" {
					t.Errorf("entries[1].Timestamp = %q, want empty", entries[1].Timestamp)
				}
			},
		},
		{
			name:        "empty shared context array",
			createFile:  true,
			contextJSON: `[]`,
			contains:    "[]",
		},
		{
			name:        "malformed shared context",
			createFile:  true,
			contextJSON: `{not valid json`,
			wantErr:     true,
			contains:    "failed to parse shared context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.createFile {
				contextPath := filepath.Join(tmpDir, "shared_context.json")
				if err := os.WriteFile(contextPath, []byte(tt.contextJSON), 0600); err != nil {
					t.Fatalf("failed to write shared context file: %v", err)
				}
			}

			handler := handleGetSharedContext(tmpDir)

			req := gomcp.CallToolRequest{}
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}

			text := resultText(t, result)

			if tt.wantErr {
				if !result.IsError {
					t.Fatalf("expected IsError=true, got false; text: %s", text)
				}
			}

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
