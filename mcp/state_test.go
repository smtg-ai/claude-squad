package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadInstances(t *testing.T) {
	tests := []struct {
		name        string
		stateJSON   string // if empty, no file is created
		createFile  bool
		wantCount   int
		wantNil     bool
		wantErr     bool
		checkFields func(t *testing.T, instances []InstanceInfo)
	}{
		{
			name:       "valid state file with 2 instances",
			createFile: true,
			stateJSON: `{
				"help_screens_seen": 0,
				"instances": [
					{
						"title": "auth-refactor",
						"path": "/repo/backend",
						"branch": "user/auth-refactor",
						"status": 0,
						"program": "claude",
						"topic_name": "auth",
						"worktree": {
							"repo_path": "/repo/backend",
							"worktree_path": "/home/.hivemind/worktrees/auth",
							"session_name": "auth-refactor",
							"branch_name": "user/auth-refactor",
							"base_commit_sha": "abc123"
						},
						"diff_stats": { "added": 10, "removed": 5 }
					},
					{
						"title": "fix-tests",
						"path": "/repo/frontend",
						"branch": "user/fix-tests",
						"status": 1,
						"program": "aider",
						"worktree": {
							"repo_path": "/repo/frontend",
							"worktree_path": "/home/.hivemind/worktrees/tests",
							"session_name": "fix-tests",
							"branch_name": "user/fix-tests",
							"base_commit_sha": "def456"
						},
						"diff_stats": { "added": 3, "removed": 1, "content": "diff --git ..." }
					}
				]
			}`,
			wantCount: 2,
			wantNil:   false,
			wantErr:   false,
			checkFields: func(t *testing.T, instances []InstanceInfo) {
				t.Helper()
				inst0 := instances[0]
				if inst0.Title != "auth-refactor" {
					t.Errorf("instance[0].Title = %q, want %q", inst0.Title, "auth-refactor")
				}
				if inst0.Path != "/repo/backend" {
					t.Errorf("instance[0].Path = %q, want %q", inst0.Path, "/repo/backend")
				}
				if inst0.Branch != "user/auth-refactor" {
					t.Errorf("instance[0].Branch = %q, want %q", inst0.Branch, "user/auth-refactor")
				}
				if inst0.Status != StatusRunning {
					t.Errorf("instance[0].Status = %d, want %d (StatusRunning)", inst0.Status, StatusRunning)
				}
				if inst0.Program != "claude" {
					t.Errorf("instance[0].Program = %q, want %q", inst0.Program, "claude")
				}
				if inst0.TopicName != "auth" {
					t.Errorf("instance[0].TopicName = %q, want %q", inst0.TopicName, "auth")
				}
				if inst0.Worktree.RepoPath != "/repo/backend" {
					t.Errorf("instance[0].Worktree.RepoPath = %q, want %q", inst0.Worktree.RepoPath, "/repo/backend")
				}
				if inst0.Worktree.BaseCommitSHA != "abc123" {
					t.Errorf("instance[0].Worktree.BaseCommitSHA = %q, want %q", inst0.Worktree.BaseCommitSHA, "abc123")
				}
				if inst0.DiffStats.Added != 10 || inst0.DiffStats.Removed != 5 {
					t.Errorf("instance[0].DiffStats = {%d, %d}, want {10, 5}", inst0.DiffStats.Added, inst0.DiffStats.Removed)
				}

				inst1 := instances[1]
				if inst1.Title != "fix-tests" {
					t.Errorf("instance[1].Title = %q, want %q", inst1.Title, "fix-tests")
				}
				if inst1.Status != StatusReady {
					t.Errorf("instance[1].Status = %d, want %d (StatusReady)", inst1.Status, StatusReady)
				}
				if inst1.Program != "aider" {
					t.Errorf("instance[1].Program = %q, want %q", inst1.Program, "aider")
				}
				if inst1.DiffStats.Content != "diff --git ..." {
					t.Errorf("instance[1].DiffStats.Content = %q, want %q", inst1.DiffStats.Content, "diff --git ...")
				}
			},
		},
		{
			name:       "empty instances array",
			createFile: true,
			stateJSON:  `{"instances": []}`,
			wantCount:  0,
			wantNil:    false,
			wantErr:    false,
		},
		{
			name:       "missing state file",
			createFile: false,
			wantNil:    true,
			wantErr:    false,
		},
		{
			name:       "malformed JSON",
			createFile: true,
			stateJSON:  `{"instances": [not valid json}`,
			wantErr:    true,
		},
		{
			name:       "state file with null instances",
			createFile: true,
			stateJSON:  `{"instances": null}`,
			wantNil:    true,
			wantErr:    false,
		},
		{
			name:       "status value running (0)",
			createFile: true,
			stateJSON:  `{"instances": [{"title": "t", "path": "/p", "branch": "b", "status": 0, "program": "claude", "worktree": {}, "diff_stats": {}}]}`,
			wantCount:  1,
			wantNil:    false,
			wantErr:    false,
			checkFields: func(t *testing.T, instances []InstanceInfo) {
				t.Helper()
				if instances[0].Status != StatusRunning {
					t.Errorf("Status = %d, want %d (StatusRunning)", instances[0].Status, StatusRunning)
				}
				if instances[0].Status.String() != "running" {
					t.Errorf("Status.String() = %q, want %q", instances[0].Status.String(), "running")
				}
			},
		},
		{
			name:       "status value ready (1)",
			createFile: true,
			stateJSON:  `{"instances": [{"title": "t", "path": "/p", "branch": "b", "status": 1, "program": "claude", "worktree": {}, "diff_stats": {}}]}`,
			wantCount:  1,
			wantNil:    false,
			wantErr:    false,
			checkFields: func(t *testing.T, instances []InstanceInfo) {
				t.Helper()
				if instances[0].Status != StatusReady {
					t.Errorf("Status = %d, want %d (StatusReady)", instances[0].Status, StatusReady)
				}
				if instances[0].Status.String() != "ready" {
					t.Errorf("Status.String() = %q, want %q", instances[0].Status.String(), "ready")
				}
			},
		},
		{
			name:       "status value loading (2)",
			createFile: true,
			stateJSON:  `{"instances": [{"title": "t", "path": "/p", "branch": "b", "status": 2, "program": "claude", "worktree": {}, "diff_stats": {}}]}`,
			wantCount:  1,
			wantNil:    false,
			wantErr:    false,
			checkFields: func(t *testing.T, instances []InstanceInfo) {
				t.Helper()
				if instances[0].Status != StatusLoading {
					t.Errorf("Status = %d, want %d (StatusLoading)", instances[0].Status, StatusLoading)
				}
				if instances[0].Status.String() != "loading" {
					t.Errorf("Status.String() = %q, want %q", instances[0].Status.String(), "loading")
				}
			},
		},
		{
			name:       "status value paused (3)",
			createFile: true,
			stateJSON:  `{"instances": [{"title": "t", "path": "/p", "branch": "b", "status": 3, "program": "claude", "worktree": {}, "diff_stats": {}}]}`,
			wantCount:  1,
			wantNil:    false,
			wantErr:    false,
			checkFields: func(t *testing.T, instances []InstanceInfo) {
				t.Helper()
				if instances[0].Status != StatusPaused {
					t.Errorf("Status = %d, want %d (StatusPaused)", instances[0].Status, StatusPaused)
				}
				if instances[0].Status.String() != "paused" {
					t.Errorf("Status.String() = %q, want %q", instances[0].Status.String(), "paused")
				}
			},
		},
		{
			name:       "unknown status value",
			createFile: true,
			stateJSON:  `{"instances": [{"title": "t", "path": "/p", "branch": "b", "status": 99, "program": "claude", "worktree": {}, "diff_stats": {}}]}`,
			wantCount:  1,
			wantNil:    false,
			wantErr:    false,
			checkFields: func(t *testing.T, instances []InstanceInfo) {
				t.Helper()
				if instances[0].Status.String() != "unknown" {
					t.Errorf("Status.String() = %q, want %q", instances[0].Status.String(), "unknown")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.createFile {
				statePath := filepath.Join(tmpDir, "state.json")
				if err := os.WriteFile(statePath, []byte(tt.stateJSON), 0600); err != nil {
					t.Fatalf("failed to write test state file: %v", err)
				}
			}

			reader := NewStateReader(tmpDir)
			instances, err := reader.ReadInstances()

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantNil {
				if instances != nil {
					t.Fatalf("expected nil, got %v", instances)
				}
				return
			}

			if instances == nil {
				t.Fatal("expected non-nil instances, got nil")
			}
			if len(instances) != tt.wantCount {
				t.Fatalf("len(instances) = %d, want %d", len(instances), tt.wantCount)
			}

			if tt.checkFields != nil {
				tt.checkFields(t, instances)
			}
		})
	}
}
