package git

import (
	"fmt"
	"github.com/ByteMirror/hivemind/log"
	"os/exec"
	"strings"
)

// runGitCommand executes a git command and returns any error
func (g *GitWorktree) runGitCommand(path string, args ...string) (string, error) {
	baseArgs := []string{"-C", path}
	cmd := exec.Command("git", append(baseArgs, args...)...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git command failed: %s (%w)", output, err)
	}

	return string(output), nil
}

// PushChanges commits and pushes changes in the worktree to the remote branch
func (g *GitWorktree) PushChanges(commitMessage string, open bool) error {
	if err := checkGHCLI(); err != nil {
		return err
	}

	if err := g.CommitChanges(commitMessage); err != nil {
		return err
	}

	// First push the branch to remote to ensure it exists
	pushCmd := exec.Command("gh", "repo", "sync", "--source", "-b", g.branchName)
	pushCmd.Dir = g.worktreePath
	if err := pushCmd.Run(); err != nil {
		// If sync fails, try creating the branch on remote first
		gitPushCmd := exec.Command("git", "push", "-u", "origin", g.branchName)
		gitPushCmd.Dir = g.worktreePath
		if pushOutput, pushErr := gitPushCmd.CombinedOutput(); pushErr != nil {
			log.ErrorLog.Print(pushErr)
			return fmt.Errorf("failed to push branch: %s (%w)", pushOutput, pushErr)
		}
	}

	// Now sync with remote
	syncCmd := exec.Command("gh", "repo", "sync", "-b", g.branchName)
	syncCmd.Dir = g.worktreePath
	if output, err := syncCmd.CombinedOutput(); err != nil {
		log.ErrorLog.Print(err)
		return fmt.Errorf("failed to sync changes: %s (%w)", output, err)
	}

	// Open the branch in the browser
	if open {
		if err := g.OpenBranchURL(); err != nil {
			// Just log the error but don't fail the push operation
			log.ErrorLog.Printf("failed to open branch URL: %v", err)
		}
	}

	return nil
}

// GeneratePRBody assembles a markdown PR description from the branch's
// changed files, commit history, and diff stats.
func (g *GitWorktree) GeneratePRBody() (string, error) {
	base := g.GetBaseCommitSHA()
	if base == "" {
		return "", fmt.Errorf("no base commit SHA available")
	}

	var sections []string

	// Changed files
	files, err := g.runGitCommand(g.worktreePath, "diff", "--name-only", base)
	if err == nil && strings.TrimSpace(files) != "" {
		sections = append(sections, "## Changes\n\n"+strings.TrimSpace(files))
	}

	// Commit messages on the branch
	commits, err := g.runGitCommand(g.worktreePath, "log", "--oneline", base+"..HEAD")
	if err == nil && strings.TrimSpace(commits) != "" {
		sections = append(sections, "## Commits\n\n"+strings.TrimSpace(commits))
	}

	// Diff stats summary
	stats, err := g.runGitCommand(g.worktreePath, "diff", "--stat", base)
	if err == nil && strings.TrimSpace(stats) != "" {
		sections = append(sections, "## Stats\n\n"+strings.TrimSpace(stats))
	}

	if len(sections) == 0 {
		return "", nil
	}

	return strings.Join(sections, "\n\n"), nil
}

// CreatePR pushes changes and creates a pull request on GitHub.
func (g *GitWorktree) CreatePR(title, body, commitMsg string) error {
	// Push changes first (without opening browser)
	if err := g.PushChanges(commitMsg, false); err != nil {
		return fmt.Errorf("failed to push changes: %w", err)
	}

	// Create the pull request
	prCmd := exec.Command("gh", "pr", "create", "--title", title, "--body", body, "--head", g.branchName)
	prCmd.Dir = g.worktreePath
	if output, err := prCmd.CombinedOutput(); err != nil {
		// If PR already exists, just open it
		if strings.Contains(string(output), "already exists") {
			viewCmd := exec.Command("gh", "pr", "view", "--web", g.branchName)
			viewCmd.Dir = g.worktreePath
			_ = viewCmd.Run()
			return nil
		}
		return fmt.Errorf("failed to create PR: %s (%w)", output, err)
	}

	// Open the PR in browser
	viewCmd := exec.Command("gh", "pr", "view", "--web", g.branchName)
	viewCmd.Dir = g.worktreePath
	_ = viewCmd.Run()

	return nil
}

// CommitChanges commits changes locally without pushing to remote
func (g *GitWorktree) CommitChanges(commitMessage string) error {
	// Check if there are any changes to commit
	isDirty, err := g.IsDirty()
	if err != nil {
		return fmt.Errorf("failed to check for changes: %w", err)
	}

	if isDirty {
		// Stage all changes
		if _, err := g.runGitCommand(g.worktreePath, "add", "."); err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to stage changes: %w", err)
		}

		// Create commit (local only)
		if _, err := g.runGitCommand(g.worktreePath, "commit", "-m", commitMessage, "--no-verify"); err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to commit changes: %w", err)
		}
	}

	return nil
}

// IsDirty checks if the worktree has uncommitted changes
func (g *GitWorktree) IsDirty() (bool, error) {
	output, err := g.runGitCommand(g.worktreePath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("failed to check worktree status: %w", err)
	}
	return len(output) > 0, nil
}

// IsBranchCheckedOut checks if the instance branch is currently checked out
func (g *GitWorktree) IsBranchCheckedOut() (bool, error) {
	output, err := g.runGitCommand(g.repoPath, "branch", "--show-current")
	if err != nil {
		return false, fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(output) == g.branchName, nil
}

// OpenBranchURL opens the branch URL in the default browser
func (g *GitWorktree) OpenBranchURL() error {
	// Check if GitHub CLI is available
	if err := checkGHCLI(); err != nil {
		return err
	}

	cmd := exec.Command("gh", "browse", "--branch", g.branchName)
	cmd.Dir = g.worktreePath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open branch URL: %w", err)
	}
	return nil
}
