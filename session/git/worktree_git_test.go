package git

import (
	"os/exec"
	"testing"
)

// createTestRepo creates a temporary git repo with an initial commit and returns its path.
func createTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %s (%v)", args, out, err)
		}
	}
	return dir
}

func TestGetDefaultBranch(t *testing.T) {
	repo := createTestRepo(t)

	// Should fall back to current branch (no origin)
	branch := GetDefaultBranch(repo)
	if branch == "" {
		t.Fatal("expected non-empty branch name")
	}
	// Should be "main" or "master" depending on git config
	if branch != "main" && branch != "master" {
		t.Fatalf("unexpected default branch: %s", branch)
	}
}

func TestGetDefaultBranchWithOrigin(t *testing.T) {
	// Create a "remote" repo
	remote := createTestRepo(t)
	// Create local clone
	local := t.TempDir()
	cmd := exec.Command("git", "clone", remote, local)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %s (%v)", out, err)
	}

	branch := GetDefaultBranch(local)
	if branch == "" {
		t.Fatal("expected non-empty branch name")
	}
}

func TestGetDefaultBranchNonGitDir(t *testing.T) {
	dir := t.TempDir()
	branch := GetDefaultBranch(dir)
	if branch != "main" {
		t.Fatalf("expected 'main' fallback for non-git dir, got: %s", branch)
	}
}

func TestGetCurrentBranch(t *testing.T) {
	repo := createTestRepo(t)
	branch, err := GetCurrentBranch(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch == "" {
		t.Fatal("expected non-empty branch name")
	}
}
