package hooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunWorktreeSetupHook_NoHook(t *testing.T) {
	ctx := WorktreeContext{
		RepoPath:     "/tmp/repo",
		WorktreePath: "/tmp/worktree",
		BranchName:   "test-branch",
		SessionName:  "test-session",
	}

	err := RunWorktreeSetupHook("", "continue", ctx)
	if err != nil {
		t.Errorf("Expected nil error for empty hook, got: %v", err)
	}
}

func TestRunWorktreeSetupHook_SimpleCommand(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hooks-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ctx := WorktreeContext{
		RepoPath:     tmpDir,
		WorktreePath: tmpDir,
		BranchName:   "test-branch",
		SessionName:  "test-session",
	}

	err = RunWorktreeSetupHook("echo hello", "continue", ctx)
	if err != nil {
		t.Errorf("Expected nil error for successful hook, got: %v", err)
	}
}

func TestRunWorktreeSetupHook_EnvironmentVariables(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hooks-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	markerFile := filepath.Join(tmpDir, "env-check.txt")
	hookCmd := "echo $CS_REPO_PATH $CS_WORKTREE_PATH $CS_BRANCH $CS_SESSION > " + markerFile

	ctx := WorktreeContext{
		RepoPath:     "/path/to/repo",
		WorktreePath: tmpDir,
		BranchName:   "my-branch",
		SessionName:  "my-session",
	}

	err = RunWorktreeSetupHook(hookCmd, "continue", ctx)
	if err != nil {
		t.Errorf("Expected nil error, got: %v", err)
	}

	content, err := os.ReadFile(markerFile)
	if err != nil {
		t.Fatalf("Failed to read marker file: %v", err)
	}

	expected := "/path/to/repo " + tmpDir + " my-branch my-session\n"
	if string(content) != expected {
		t.Errorf("Expected env vars %q, got %q", expected, string(content))
	}
}

func TestRunWorktreeSetupHook_FailModeContinue(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hooks-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ctx := WorktreeContext{
		RepoPath:     tmpDir,
		WorktreePath: tmpDir,
		BranchName:   "test-branch",
		SessionName:  "test-session",
	}

	err = RunWorktreeSetupHook("exit 1", "continue", ctx)
	if err != nil {
		t.Errorf("Expected nil error in continue mode, got: %v", err)
	}
}

func TestRunWorktreeSetupHook_FailModeFail(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hooks-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ctx := WorktreeContext{
		RepoPath:     tmpDir,
		WorktreePath: tmpDir,
		BranchName:   "test-branch",
		SessionName:  "test-session",
	}

	err = RunWorktreeSetupHook("exit 1", "fail", ctx)
	if err == nil {
		t.Error("Expected error in fail mode, got nil")
	}
}
