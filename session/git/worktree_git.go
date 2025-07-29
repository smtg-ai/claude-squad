package git

import (
	"claude-squad/log"
	"fmt"
	"os/exec"
	"runtime"
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

	// If we have a cached URL and just want to open it, do that directly
	if g.githubURL != "" && open && !g.hasUnpushedCommits() {
		return g.OpenBranchURL()
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

	// Check if this is the first push (branch doesn't exist on remote)
	isFirstPush := false
	checkCmd := exec.Command("git", "ls-remote", "--heads", "origin", g.branchName)
	checkCmd.Dir = g.worktreePath
	checkOutput, _ := checkCmd.Output()
	if len(checkOutput) == 0 {
		isFirstPush = true
	}

	// Push the branch to remote
	gitPushCmd := exec.Command("git", "push", "-u", "origin", g.branchName)
	gitPushCmd.Dir = g.worktreePath
	pushOutput, err := gitPushCmd.CombinedOutput()
	if err != nil {
		log.ErrorLog.Print(err)
		return fmt.Errorf("failed to push branch: %s (%w)", pushOutput, err)
	}

	// Parse the PR creation URL from git push output if this is the first push
	if isFirstPush && g.githubURL == "" {
		if prURL := g.parsePRCreationURL(string(pushOutput)); prURL != "" {
			g.githubURL = prURL
			log.InfoLog.Printf("Captured PR creation URL: %s", prURL)
		} else {
			// Log the output to help debug if URL parsing fails
			log.InfoLog.Printf("Could not find PR URL in git push output: %s", string(pushOutput))
		}
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

// hasUnpushedCommits checks if there are commits that haven't been pushed to remote
func (g *GitWorktree) hasUnpushedCommits() bool {
	// Check if we have commits ahead of origin
	output, err := g.runGitCommand(g.worktreePath, "rev-list", "--count", fmt.Sprintf("origin/%s..HEAD", g.branchName))
	if err != nil {
		// If the command fails (e.g., branch doesn't exist on remote), assume we need to push
		return true
	}
	count := strings.TrimSpace(output)
	return count != "0"
}

// IsBranchCheckedOut checks if the instance branch is currently checked out
func (g *GitWorktree) IsBranchCheckedOut() (bool, error) {
	output, err := g.runGitCommand(g.repoPath, "branch", "--show-current")
	if err != nil {
		return false, fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)) == g.branchName, nil
}

// parsePRCreationURL extracts the PR creation URL from git push output
func (g *GitWorktree) parsePRCreationURL(output string) string {
	// Git push output typically contains a line like:
	// remote: Create a pull request for 'branch-name' on GitHub by visiting:
	// remote:      https://github.com/owner/repo/pull/new/branch-name
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if strings.Contains(line, "Create a pull request") && i+1 < len(lines) {
			// The URL is typically on the next line
			urlLine := strings.TrimSpace(lines[i+1])
			// Remove "remote:" prefix if present
			urlLine = strings.TrimPrefix(urlLine, "remote:")
			urlLine = strings.TrimSpace(urlLine)
			if strings.HasPrefix(urlLine, "http") {
				return urlLine
			}
		}
	}
	
	// If we can't find the PR URL in output, return empty
	return ""
}

// OpenBranchURL opens the branch URL in the default browser
func (g *GitWorktree) OpenBranchURL() error {
	urlToOpen := g.githubURL
	
	// If we don't have a cached PR URL, fall back to gh browse
	if urlToOpen == "" {
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

	// Open the cached URL
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", urlToOpen)
	case "linux":
		cmd = exec.Command("xdg-open", urlToOpen)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", urlToOpen)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open URL: %w", err)
	}
	return nil
}
