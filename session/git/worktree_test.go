package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupNewWorktreeTest points HOME at a temp dir containing the given config.json
// and returns a fresh git repository path.
func setupNewWorktreeTest(t *testing.T, configJSON string) (home string, repoPath string) {
	t.Helper()

	home = t.TempDir()
	originalHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("HOME", originalHome)
	})

	configDir := filepath.Join(home, ".claude-squad")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(configJSON), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	repoPath = filepath.Join(t.TempDir(), "repo")
	mustRunGit(t, "", "init", repoPath)
	return home, repoPath
}

func TestNewGitWorktree_BranchNaming(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		sessionName string
		wantBranch  string
		wantDirName string
	}{
		{
			name:        "default lowercases branch",
			configJSON:  `{"branch_prefix": "Ticket/"}`,
			sessionName: "19235/FeatureName",
			wantBranch:  "ticket/19235/featurename",
			wantDirName: "ticket/19235/featurename",
		},
		{
			name:        "preserve case keeps branch case but not directory case",
			configJSON:  `{"branch_prefix": "Ticket/", "preserve_branch_case": true}`,
			sessionName: "19235/FeatureName",
			wantBranch:  "Ticket/19235/FeatureName",
			wantDirName: "ticket/19235/featurename",
		},
		{
			name:        "empty prefix uses session name as full branch name",
			configJSON:  `{"branch_prefix": "", "preserve_branch_case": true}`,
			sessionName: "Ticket/19235/FeatureName",
			wantBranch:  "Ticket/19235/FeatureName",
			wantDirName: "ticket/19235/featurename",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home, repoPath := setupNewWorktreeTest(t, tt.configJSON)

			tree, branch, err := NewGitWorktree(repoPath, tt.sessionName)
			if err != nil {
				t.Fatalf("NewGitWorktree() error = %v", err)
			}

			if branch != tt.wantBranch {
				t.Errorf("branch = %q, want %q", branch, tt.wantBranch)
			}
			if tree.GetBranchName() != tt.wantBranch {
				t.Errorf("GetBranchName() = %q, want %q", tree.GetBranchName(), tt.wantBranch)
			}

			worktreeDir := filepath.Join(home, ".claude-squad", "worktrees")
			rel, err := filepath.Rel(worktreeDir, tree.GetWorktreePath())
			if err != nil {
				t.Fatalf("worktree path %q not under %q: %v", tree.GetWorktreePath(), worktreeDir, err)
			}
			if !strings.HasPrefix(filepath.ToSlash(rel), tt.wantDirName+"_") {
				t.Errorf("worktree path = %q, want directory %q_<timestamp>", rel, tt.wantDirName)
			}
		})
	}
}
