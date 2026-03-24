package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitCmd runs a git command in a directory and returns stdout, failing the test on error.
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %s (%v)", args, dir, out, err)
	}
	return strings.TrimSpace(string(out))
}

// headSHA returns the HEAD commit SHA for the repo.
func headSHA(t *testing.T, dir string) string {
	t.Helper()
	return gitCmd(t, dir, "rev-parse", "HEAD")
}

// branchExists checks if a local branch exists in the repo.
func branchExists(t *testing.T, repo, branch string) bool {
	t.Helper()
	cmd := exec.Command("git", "show-ref", "--verify", "refs/heads/"+branch)
	cmd.Dir = repo
	return cmd.Run() == nil
}

// worktreeDir returns the path to a worktree inside a temp dir, creating the parent.
func worktreeDir(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	return dir
}

func TestSetupNewWorktreeFromHEAD(t *testing.T) {
	repo := createTestRepo(t)
	wtPath := worktreeDir(t, "wt-new")

	gw := &GitWorktree{
		repoPath:     repo,
		worktreePath: wtPath,
		sessionName:  "test-session",
		branchName:   "cs-test-session",
	}

	if err := gw.Setup(); err != nil {
		t.Fatalf("Setup() error: %v", err)
	}
	defer gw.Cleanup()

	// Worktree directory should exist
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree directory not created: %v", err)
	}

	// Branch should exist
	if !branchExists(t, repo, "cs-test-session") {
		t.Error("expected branch 'cs-test-session' to exist")
	}

	// baseCommitSHA should be set to HEAD
	repoHead := headSHA(t, repo)
	if gw.baseCommitSHA != repoHead {
		t.Errorf("baseCommitSHA = %q, want %q", gw.baseCommitSHA, repoHead)
	}

	// Worktree should be on the new branch
	wtBranch := gitCmd(t, wtPath, "branch", "--show-current")
	if wtBranch != "cs-test-session" {
		t.Errorf("worktree branch = %q, want 'cs-test-session'", wtBranch)
	}
}

func TestSetupFromExistingBranch(t *testing.T) {
	repo := createTestRepo(t)
	// Create a branch with a commit ahead of main
	gitCmd(t, repo, "checkout", "-b", "feature-x")
	gitCmd(t, repo, "commit", "--allow-empty", "-m", "feature commit")
	featureSHA := headSHA(t, repo)
	gitCmd(t, repo, "checkout", "-") // back to main/master

	wtPath := worktreeDir(t, "wt-existing")

	gw := &GitWorktree{
		repoPath:         repo,
		worktreePath:     wtPath,
		sessionName:      "test-existing",
		branchName:       "feature-x",
		isExistingBranch: true,
	}

	if err := gw.Setup(); err != nil {
		t.Fatalf("Setup() error: %v", err)
	}
	defer gw.Cleanup()

	// Worktree directory should exist
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree directory not created: %v", err)
	}

	// Worktree should be on feature-x
	wtBranch := gitCmd(t, wtPath, "branch", "--show-current")
	if wtBranch != "feature-x" {
		t.Errorf("worktree branch = %q, want 'feature-x'", wtBranch)
	}

	// Worktree HEAD should match the feature branch commit
	wtHead := headSHA(t, wtPath)
	if wtHead != featureSHA {
		t.Errorf("worktree HEAD = %q, want %q (feature-x tip)", wtHead, featureSHA)
	}
}

func TestSetupFromRef(t *testing.T) {
	// Create a repo with two commits so we can use the first as a ref
	repo := createTestRepo(t)
	firstCommit := headSHA(t, repo)
	gitCmd(t, repo, "commit", "--allow-empty", "-m", "second commit")
	secondCommit := headSHA(t, repo)

	if firstCommit == secondCommit {
		t.Fatal("expected different commits")
	}

	wtPath := worktreeDir(t, "wt-ref")

	gw := &GitWorktree{
		repoPath:     repo,
		worktreePath: wtPath,
		sessionName:  "test-ref",
		branchName:   "cs-test-ref",
		baseRef:      firstCommit, // base off the first commit, not HEAD
	}

	if err := gw.Setup(); err != nil {
		t.Fatalf("Setup() error: %v", err)
	}
	defer gw.Cleanup()

	// Worktree directory should exist
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree directory not created: %v", err)
	}

	// Branch should exist
	if !branchExists(t, repo, "cs-test-ref") {
		t.Error("expected branch 'cs-test-ref' to exist")
	}

	// baseCommitSHA should be the first commit, not HEAD
	if gw.baseCommitSHA != firstCommit {
		t.Errorf("baseCommitSHA = %q, want %q", gw.baseCommitSHA, firstCommit)
	}

	// Worktree HEAD should be at the first commit
	wtHead := headSHA(t, wtPath)
	if wtHead != firstCommit {
		t.Errorf("worktree HEAD = %q, want %q (first commit)", wtHead, firstCommit)
	}

	// Worktree should be on the new branch
	wtBranch := gitCmd(t, wtPath, "branch", "--show-current")
	if wtBranch != "cs-test-ref" {
		t.Errorf("worktree branch = %q, want 'cs-test-ref'", wtBranch)
	}
}

func TestSetupFromRefWithOriginBranch(t *testing.T) {
	// Simulate the real use case: create from origin/main
	remote := createTestRepo(t)
	gitCmd(t, remote, "commit", "--allow-empty", "-m", "remote commit")
	remoteSHA := headSHA(t, remote)

	// Clone to get a local repo with an origin
	local := t.TempDir()
	cmd := exec.Command("git", "clone", remote, local)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %s (%v)", out, err)
	}

	// Add a local commit so HEAD diverges from origin
	gitCmd(t, local, "commit", "--allow-empty", "-m", "local commit")
	localSHA := headSHA(t, local)
	if localSHA == remoteSHA {
		t.Fatal("expected local to diverge from remote")
	}

	// Determine the default branch name
	defaultBranch := GetDefaultBranch(local)
	baseRef := "origin/" + defaultBranch

	wtPath := worktreeDir(t, "wt-origin")

	gw := &GitWorktree{
		repoPath:     local,
		worktreePath: wtPath,
		sessionName:  "test-origin",
		branchName:   "cs-test-origin",
		baseRef:      baseRef,
	}

	if err := gw.Setup(); err != nil {
		t.Fatalf("Setup() error: %v", err)
	}
	defer gw.Cleanup()

	// Worktree HEAD should match the remote commit, NOT the local HEAD
	wtHead := headSHA(t, wtPath)
	if wtHead != remoteSHA {
		t.Errorf("worktree HEAD = %q, want %q (origin/%s)", wtHead, remoteSHA, defaultBranch)
	}
	if wtHead == localSHA {
		t.Error("worktree HEAD should NOT match local HEAD — should be based on origin")
	}
}

func TestCleanupRemovesWorktreeAndBranch(t *testing.T) {
	repo := createTestRepo(t)
	wtPath := worktreeDir(t, "wt-cleanup")

	gw := &GitWorktree{
		repoPath:     repo,
		worktreePath: wtPath,
		sessionName:  "test-cleanup",
		branchName:   "cs-test-cleanup",
	}

	if err := gw.Setup(); err != nil {
		t.Fatalf("Setup() error: %v", err)
	}

	// Verify setup worked
	if !branchExists(t, repo, "cs-test-cleanup") {
		t.Fatal("branch should exist after setup")
	}

	// Cleanup
	if err := gw.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error: %v", err)
	}

	// Worktree directory should be gone
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed after cleanup")
	}

	// Branch should be gone (not an existing branch)
	if branchExists(t, repo, "cs-test-cleanup") {
		t.Error("branch should be removed after cleanup")
	}
}

func TestCleanupPreservesExistingBranch(t *testing.T) {
	repo := createTestRepo(t)
	gitCmd(t, repo, "checkout", "-b", "keep-me")
	gitCmd(t, repo, "checkout", "-")

	wtPath := worktreeDir(t, "wt-preserve")

	gw := &GitWorktree{
		repoPath:         repo,
		worktreePath:     wtPath,
		sessionName:      "test-preserve",
		branchName:       "keep-me",
		isExistingBranch: true,
	}

	if err := gw.Setup(); err != nil {
		t.Fatalf("Setup() error: %v", err)
	}

	if err := gw.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error: %v", err)
	}

	// Branch should still exist because isExistingBranch was true
	if !branchExists(t, repo, "keep-me") {
		t.Error("existing branch should be preserved after cleanup")
	}
}
