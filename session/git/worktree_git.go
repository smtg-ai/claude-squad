package git

import (
	"claude-squad/log"
	"fmt"
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

		// Create commit
		if _, err := g.runGitCommand(g.worktreePath, "commit", "-m", commitMessage, "--no-verify"); err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to commit changes: %w", err)
		}
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

	// Try to create a pull request if environment is available
	if err := g.createPullRequestIfPossible(); err != nil {
		// Just log the error but don't fail the push operation
		log.ErrorLog.Printf("failed to create pull request: %v", err)
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
	return strings.TrimSpace(string(output)) == g.branchName, nil
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

// createPullRequestIfPossible attempts to create a pull request to main branch
// Returns nil if successful or if PR already exists, error only for unexpected failures
func (g *GitWorktree) createPullRequestIfPossible() error {
	// Check if GitHub CLI is available
	if err := checkGHCLI(); err != nil {
		// Skip PR creation if gh is not available
		return nil
	}

	// Check if current branch is main/master (can't create PR from main to main)
	if g.branchName == "main" || g.branchName == "master" {
		return nil
	}

	// Check if we're in a GitHub repository
	checkCmd := exec.Command("gh", "repo", "view")
	checkCmd.Dir = g.worktreePath
	if err := checkCmd.Run(); err != nil {
		// Not a GitHub repo or not authenticated, skip PR creation
		return nil
	}

	// Get the last commit message to use as PR title
	titleCmd := exec.Command("git", "log", "-1", "--pretty=format:%s")
	titleCmd.Dir = g.worktreePath
	titleOutput, err := titleCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get last commit message: %w", err)
	}
	prTitle := strings.TrimSpace(string(titleOutput))

	// Create pull request with --fill flag for non-interactive mode
	// Target main branch as base
	prCmd := exec.Command("gh", "pr", "create",
		"--base", "main",
		"--head", g.branchName,
		"--title", prTitle,
		"--body", fmt.Sprintf("Pull request created automatically by Claude Squad\n\nBranch: %s", g.branchName),
		"--fill")
	prCmd.Dir = g.worktreePath

	output, err := prCmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		// Check if PR already exists - this is not an error
		if strings.Contains(outputStr, "already exists") ||
			strings.Contains(outputStr, "pull request for branch") {
			log.InfoLog.Printf("Pull request already exists for branch %s", g.branchName)
			return nil
		}
		return fmt.Errorf("failed to create pull request: %s", outputStr)
	}

	// Extract PR URL from output and log it
	prURL := strings.TrimSpace(string(output))
	log.InfoLog.Printf("Successfully created pull request: %s", prURL)

	return nil
}
