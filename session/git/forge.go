package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// forgeKind identifies which git-hosting CLI backend to use for operations that need
// one (pushing/syncing a branch, opening it in a browser).
type forgeKind string

const (
	forgeGitHub  forgeKind = "github"
	forgeForgejo forgeKind = "forgejo"
)

// detectForge inspects the origin remote URL to decide which CLI backend to use.
// Defaults to forgeGitHub on any detection failure or unrecognized host, preserving
// existing behavior for anyone not using Forgejo.
func detectForge(repoPath string) forgeKind {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return forgeGitHub
	}
	if strings.Contains(string(output), "github.com") {
		return forgeGitHub
	}
	return forgeForgejo
}

// checkTeaCLI checks if the Forgejo/Gitea CLI (tea) is installed and has at least one
// configured login. Mirrors checkGHCLI in util.go.
func checkTeaCLI() error {
	if _, err := exec.LookPath("tea"); err != nil {
		return fmt.Errorf("Forgejo/Gitea CLI (tea) is not installed. Please install it first")
	}

	output, err := exec.Command("tea", "logins", "list", "-o", "simple").Output()
	if err != nil || len(strings.TrimSpace(string(output))) == 0 {
		return fmt.Errorf("tea CLI has no configured logins. Please run 'tea login add' first")
	}

	return nil
}

// pushViaGit pushes the branch to origin with a plain `git push -u`. This is the
// Forgejo push path (Forgejo has no fork/sync-aware CLI equivalent to `gh repo sync`),
// and matches the same fallback PushChanges already uses if `gh repo sync` fails.
func pushViaGit(worktreePath, branchName string) error {
	cmd := exec.Command("git", "push", "-u", "origin", branchName)
	cmd.Dir = worktreePath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push branch: %s (%w)", output, err)
	}
	return nil
}

// openBranchURLForgejo opens the repository in the browser via `tea open`, which infers
// the repo from the worktree's local git remote. Unlike `gh browse --branch`, tea has no
// flag to target a specific branch — this opens the repo's default view instead.
func openBranchURLForgejo(worktreePath string) error {
	cmd := exec.Command("tea", "open")
	cmd.Dir = worktreePath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open repo URL: %w", err)
	}
	return nil
}
