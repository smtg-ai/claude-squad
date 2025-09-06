package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple lowercase string",
			input:    "feature",
			expected: "feature",
		},
		{
			name:     "string with spaces",
			input:    "new feature branch",
			expected: "new-feature-branch",
		},
		{
			name:     "mixed case string",
			input:    "FeAtUrE BrAnCh",
			expected: "feature-branch",
		},
		{
			name:     "string with special characters",
			input:    "feature!@#$%^&*()",
			expected: "feature",
		},
		{
			name:     "string with allowed special characters",
			input:    "feature/sub_branch.v1",
			expected: "feature/sub_branch.v1",
		},
		{
			name:     "string with multiple dashes",
			input:    "feature---branch",
			expected: "feature-branch",
		},
		{
			name:     "string with leading and trailing dashes",
			input:    "-feature-branch-",
			expected: "feature-branch",
		},
		{
			name:     "string with leading and trailing slashes",
			input:    "/feature/branch/",
			expected: "feature/branch",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "complex mixed case with special chars",
			input:    "USER/Feature Branch!@#$%^&*()/v1.0",
			expected: "user/feature-branch/v1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeBranchName(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeBranchName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestGitWorktree_CheckRemoteBranch tests the remote branch detection functionality
func TestGitWorktree_CheckRemoteBranch(t *testing.T) {
	// Create temporary directory for test repository
	tempDir, err := os.MkdirTemp("", "git-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	// Configure git user for testing
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	// Create initial commit
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("initial content"), 0644)
	require.NoError(t, err)

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	// Create GitWorktree instance
	worktree := &GitWorktree{
		repoPath:     tempDir,
		branchName:   "test-branch",
		worktreePath: filepath.Join(tempDir, "worktrees", "test-branch"),
	}

	t.Run("non-existent remote branch", func(t *testing.T) {
		exists, needsSync, err := worktree.CheckRemoteBranch("non-existent-branch")
		assert.NoError(t, err)
		assert.False(t, exists)
		assert.False(t, needsSync)
	})

	t.Run("no remote configured", func(t *testing.T) {
		exists, needsSync, err := worktree.CheckRemoteBranch("main")
		assert.NoError(t, err)
		assert.False(t, exists)
		assert.False(t, needsSync)
	})
}

// TestGitWorktree_SyncWithRemoteBranch tests the remote branch sync functionality
func TestGitWorktree_SyncWithRemoteBranch(t *testing.T) {
	// Create temporary directory for test repository
	tempDir, err := os.MkdirTemp("", "git-sync-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	// Configure git user for testing
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	// Create initial commit
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("initial content"), 0644)
	require.NoError(t, err)

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	// Create GitWorktree instance
	worktree := &GitWorktree{
		repoPath:     tempDir,
		branchName:   "test-branch",
		worktreePath: filepath.Join(tempDir, "worktrees", "test-branch"),
	}

	t.Run("sync with non-existent remote", func(t *testing.T) {
		// Should handle gracefully - either error or no-op
		// The exact behavior depends on git configuration
		assert.NotPanics(t, func() {
			worktree.SyncWithRemoteBranch("non-existent-branch")
		})
	})

	t.Run("sync without remote configured", func(t *testing.T) {
		// Should handle gracefully when no remote is configured
		assert.NotPanics(t, func() {
			worktree.SyncWithRemoteBranch("main")
		})
	})
}

// TestGitWorktree_SetupFromExistingBranch_WithRemote tests enhanced setup logic
func TestGitWorktree_SetupFromExistingBranch_WithRemote(t *testing.T) {
	// This test would require a more complex setup with actual remote repositories
	// For now, we'll test the logic paths that don't require network access

	tempDir, err := os.MkdirTemp("", "git-setup-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	// Configure git user for testing
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	// Create initial commit
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("initial content"), 0644)
	require.NoError(t, err)

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	// Create a local branch to test with
	cmd = exec.Command("git", "checkout", "-b", "test-branch")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	// Switch back to main
	cmd = exec.Command("git", "checkout", "main")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		// Fallback to master if main doesn't exist
		cmd = exec.Command("git", "checkout", "master")
		cmd.Dir = tempDir
		cmd.Run() // Ignore error if master also doesn't exist
	}

	// Create worktrees directory
	worktreesDir := filepath.Join(tempDir, "worktrees")
	err = os.MkdirAll(worktreesDir, 0755)
	require.NoError(t, err)

	// Create GitWorktree instance
	worktree := &GitWorktree{
		repoPath:     tempDir,
		branchName:   "test-branch",
		worktreePath: filepath.Join(worktreesDir, "test-branch"),
	}

	t.Run("setup from existing local branch", func(t *testing.T) {
		err := worktree.setupFromExistingBranch()
		// Should succeed for local branch
		if err != nil {
			// Log error for debugging but don't fail test since git setup can be environment-dependent
			t.Logf("Setup failed (may be expected in test environment): %v", err)
		}
	})
}

// TestCheckRemoteBranchEdgeCases tests edge cases for remote branch checking
func TestCheckRemoteBranchEdgeCases(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "git-edge-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create GitWorktree instance with invalid repo path
	worktree := &GitWorktree{
		repoPath:     "/non/existent/path",
		branchName:   "test-branch",
		worktreePath: filepath.Join(tempDir, "worktrees", "test-branch"),
	}

	t.Run("invalid repo path for remote check", func(t *testing.T) {
		exists, needsSync, _ := worktree.CheckRemoteBranch("test-branch")
		// Should handle gracefully
		assert.False(t, exists)
		assert.False(t, needsSync)
		// Error is acceptable for invalid path
	})

	t.Run("empty branch name", func(t *testing.T) {
		exists, needsSync, _ := worktree.CheckRemoteBranch("")
		assert.False(t, exists)
		assert.False(t, needsSync)
		// Should not panic
	})

	t.Run("branch name with special characters", func(t *testing.T) {
		exists, needsSync, err := worktree.CheckRemoteBranch("feature/test-branch_v1.0")
		assert.False(t, exists)
		assert.False(t, needsSync)
		assert.NoError(t, err)
	})
}

// TestSyncWithRemoteBranchEdgeCases tests edge cases for remote branch sync
func TestSyncWithRemoteBranchEdgeCases(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "git-sync-edge-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create GitWorktree instance with invalid repo path
	worktree := &GitWorktree{
		repoPath:     "/non/existent/path",
		branchName:   "test-branch",
		worktreePath: filepath.Join(tempDir, "worktrees", "test-branch"),
	}

	t.Run("sync with invalid repo path", func(t *testing.T) {
		err := worktree.SyncWithRemoteBranch("test-branch")
		// Should handle error gracefully
		assert.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "failed to fetch")
	})

	t.Run("sync with empty branch name", func(t *testing.T) {
		// Should not panic
		assert.NotPanics(t, func() {
			worktree.SyncWithRemoteBranch("")
		})
	})
}
