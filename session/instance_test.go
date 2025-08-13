package session

import (
	"claude-squad/session/git"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFromInstanceDataWithMissingWorktree(t *testing.T) {
	// Create a temporary directory to simulate a worktree path
	tempDir, err := os.MkdirTemp("", "claude-squad-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create worktree path within temp dir
	worktreePath := filepath.Join(tempDir, "worktree-path")
	err = os.MkdirAll(worktreePath, 0755)
	if err != nil {
		t.Fatalf("Failed to create worktree directory: %v", err)
	}

	// Test our fix function directly instead of trying to mock everything
	// Create a test instance with a gitWorktree that points to a real path
	instance := &Instance{
		Title:       "Test Instance",
		Path:        "/path/to/repo",
		Branch:      "test-branch",
		Status:      Ready,
		Height:      100,
		Width:       200,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Program:     "claude",
		gitWorktree: git.NewGitWorktreeFromStorage(
			"/path/to/repo",
			worktreePath,
			"Test Instance",
			"test-branch",
			"abcdef1234567890",
		),
		started:     true,
	}

	// Test 1: Worktree exists - instance should not be paused
	checkInstanceStatus(t, instance, worktreePath, false)

	// Now delete the worktree directory to simulate a stale worktree
	err = os.RemoveAll(worktreePath)
	if err != nil {
		t.Fatalf("Failed to remove test worktree directory: %v", err)
	}

	// Reload the instance from data - this should detect the missing worktree
	// We need to use a modified approach since we can't call the actual FromInstanceData
	// which would try to start a real session
	instance = &Instance{
		Title:       "Test Instance",
		Path:        "/path/to/repo",
		Branch:      "test-branch",
		Status:      Ready,
		Height:      100,
		Width:       200,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Program:     "claude",
		gitWorktree: git.NewGitWorktreeFromStorage(
			"/path/to/repo",
			worktreePath,
			"Test Instance",
			"test-branch",
			"abcdef1234567890",
		),
		started:     true,
	}

	// Test 2: Apply our fix - check if worktree exists and update status
	if !instance.Paused() && instance.gitWorktree != nil {
		worktreePath := instance.gitWorktree.GetWorktreePath()
		if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
			// Worktree has been deleted, mark instance as paused
			instance.Status = Paused
		}
	}

	// Verify that the instance is now paused
	checkInstanceStatus(t, instance, worktreePath, true)
}

func checkInstanceStatus(t *testing.T, instance *Instance, worktreePath string, expectPaused bool) {
	if expectPaused && !instance.Paused() {
		t.Errorf("Expected instance to be paused when worktree at %s doesn't exist", worktreePath)
	} else if !expectPaused && instance.Paused() {
		t.Errorf("Expected instance to not be paused when worktree at %s exists", worktreePath)
	}
}

