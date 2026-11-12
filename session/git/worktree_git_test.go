package git

import (
	"claude-squad/log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	log.Initialize(false)
	code := m.Run()
	log.Close()
	os.Exit(code)
}

func TestPushChanges_CustomRemote(t *testing.T) {
	root := t.TempDir()

	remotePath := filepath.Join(root, "fork.git")
	mustRunGit(t, "", "init", "--bare", remotePath)

	repoPath := filepath.Join(root, "repo")
	mustRunGit(t, "", "init", repoPath)
	mustRunGit(t, repoPath, "config", "user.name", "Test User")
	mustRunGit(t, repoPath, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	mustRunGit(t, repoPath, "add", "README.md")
	mustRunGit(t, repoPath, "commit", "-m", "initial")
	mustRunGit(t, repoPath, "remote", "add", "upstream", remotePath)

	branchName := "Ticket/1/Foo"
	mustRunGit(t, repoPath, "checkout", "-b", branchName)

	// Leave uncommitted work so PushChanges has to commit it first.
	if err := os.WriteFile(filepath.Join(repoPath, "change.txt"), []byte("change\n"), 0644); err != nil {
		t.Fatalf("write change: %v", err)
	}

	g := &GitWorktree{
		repoPath:     repoPath,
		worktreePath: repoPath,
		branchName:   branchName,
	}

	if err := g.PushChanges("test commit", false, "upstream"); err != nil {
		t.Fatalf("PushChanges() error = %v", err)
	}

	refs := mustRunGit(t, repoPath, "ls-remote", "upstream")
	if !strings.Contains(refs, "refs/heads/"+branchName) {
		t.Fatalf("branch %q not found on remote, ls-remote output:\n%s", branchName, refs)
	}

	if dirty, err := g.IsDirty(); err != nil {
		t.Fatalf("IsDirty() error = %v", err)
	} else if dirty {
		t.Fatalf("worktree still dirty after PushChanges")
	}

	upstreamBranch := strings.TrimSpace(mustRunGit(t, repoPath, "rev-parse", "--abbrev-ref", branchName+"@{upstream}"))
	if upstreamBranch != "upstream/"+branchName {
		t.Fatalf("upstream tracking = %q, want %q", upstreamBranch, "upstream/"+branchName)
	}
}
