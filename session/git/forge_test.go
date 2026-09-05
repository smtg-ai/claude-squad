package git

import (
	"path/filepath"
	"testing"
)

func TestDetectForge_GitHub(t *testing.T) {
	repoPath := filepath.Join(t.TempDir(), "repo")
	mustRunGit(t, "", "init", repoPath)
	mustRunGit(t, repoPath, "remote", "add", "origin", "git@github.com:armarquez/claude-squad.git")

	if got := detectForge(repoPath); got != forgeGitHub {
		t.Errorf("detectForge() = %q, want %q", got, forgeGitHub)
	}
}

func TestDetectForge_Forgejo(t *testing.T) {
	repoPath := filepath.Join(t.TempDir(), "repo")
	mustRunGit(t, "", "init", repoPath)
	mustRunGit(t, repoPath, "remote", "add", "origin", "https://git.mqz.casa/boogey/claude-squad.git")

	if got := detectForge(repoPath); got != forgeForgejo {
		t.Errorf("detectForge() = %q, want %q", got, forgeForgejo)
	}
}

func TestDetectForge_NoRemote(t *testing.T) {
	repoPath := filepath.Join(t.TempDir(), "repo")
	mustRunGit(t, "", "init", repoPath)

	if got := detectForge(repoPath); got != forgeGitHub {
		t.Errorf("detectForge() = %q, want %q (safe default)", got, forgeGitHub)
	}
}
