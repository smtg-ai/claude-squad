package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSetupFromExistingBranch_RemovesOrphanedDirectory(t *testing.T) {
	tempHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	defer func() {
		_ = os.Setenv("HOME", originalHome)
	}()

	repoPath := filepath.Join(t.TempDir(), "repo")
	mustRunGit(t, "", "init", repoPath)
	mustRunGit(t, repoPath, "config", "user.name", "Test User")
	mustRunGit(t, repoPath, "config", "user.email", "test@example.com")

	readmePath := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(readmePath, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	mustRunGit(t, repoPath, "add", "README.md")
	mustRunGit(t, repoPath, "commit", "-m", "initial")
	mustRunGit(t, repoPath, "branch", "feature/test")

	worktreePath := filepath.Join(tempHome, ".claude-squad", "worktrees", "feature-test")
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("mkdir orphaned worktree: %v", err)
	}

	junkPath := filepath.Join(worktreePath, "orphan.txt")
	if err := os.WriteFile(junkPath, []byte("orphaned\n"), 0644); err != nil {
		t.Fatalf("write orphan marker: %v", err)
	}

	g := &GitWorktree{
		repoPath:         repoPath,
		worktreePath:     worktreePath,
		branchName:       "feature/test",
		isExistingBranch: true,
	}

	if err := g.Setup(); err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	if _, err := os.Stat(junkPath); !os.IsNotExist(err) {
		t.Fatalf("orphan marker still exists after Setup, err = %v", err)
	}

	if valid, err := g.IsValidWorktree(); err != nil {
		t.Fatalf("IsValidWorktree() error = %v", err)
	} else if !valid {
		t.Fatal("expected Setup() to recreate a valid worktree")
	}

	currentBranch := mustRunGit(t, worktreePath, "branch", "--show-current")
	if currentBranch != "feature/test\n" {
		t.Fatalf("current branch = %q, want %q", currentBranch, "feature/test\n")
	}
}

func TestCountWorktreesWithUncommittedChanges(t *testing.T) {
	tempHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	defer func() {
		_ = os.Setenv("HOME", originalHome)
	}()

	worktreesDir := filepath.Join(tempHome, ".claude-squad", "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		t.Fatalf("mkdir worktrees dir: %v", err)
	}

	// A clean worktree: initialized, committed, no pending changes.
	cleanPath := filepath.Join(worktreesDir, "clean")
	mustRunGit(t, "", "init", cleanPath)
	mustRunGit(t, cleanPath, "config", "user.name", "Test User")
	mustRunGit(t, cleanPath, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(cleanPath, "README.md"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	mustRunGit(t, cleanPath, "add", "README.md")
	mustRunGit(t, cleanPath, "commit", "-m", "initial")

	// A dirty worktree: has an uncommitted change.
	dirtyPath := filepath.Join(worktreesDir, "dirty")
	mustRunGit(t, "", "init", dirtyPath)
	mustRunGit(t, dirtyPath, "config", "user.name", "Test User")
	mustRunGit(t, dirtyPath, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dirtyPath, "README.md"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	mustRunGit(t, dirtyPath, "add", "README.md")
	mustRunGit(t, dirtyPath, "commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(dirtyPath, "README.md"), []byte("modified\n"), 0644); err != nil {
		t.Fatalf("modify README: %v", err)
	}

	count, err := CountWorktreesWithUncommittedChanges()
	if err != nil {
		t.Fatalf("CountWorktreesWithUncommittedChanges() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

func TestCountWorktreesWithUncommittedChanges_NoWorktreesDir(t *testing.T) {
	tempHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	defer func() {
		_ = os.Setenv("HOME", originalHome)
	}()

	count, err := CountWorktreesWithUncommittedChanges()
	if err != nil {
		t.Fatalf("CountWorktreesWithUncommittedChanges() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
}

func mustRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmdArgs := args
	if dir != "" {
		cmdArgs = append([]string{"-C", dir}, args...)
	}

	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}
