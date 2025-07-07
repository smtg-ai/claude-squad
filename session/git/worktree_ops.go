package git

import (
	"claude-squad/config"
	"claude-squad/log"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Setup creates a new worktree for the session
func (g *GitWorktree) Setup() error {
	// Check if branch exists first
	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	branchRef := plumbing.NewBranchReferenceName(g.branchName)
	if _, err := repo.Reference(branchRef, false); err == nil {
		// Branch exists, use SetupFromExistingBranch
		return g.SetupFromExistingBranch()
	}

	// Branch doesn't exist, create new worktree from HEAD
	return g.SetupNewWorktree()
}

// SetupFromExistingBranch creates a worktree from an existing branch
func (g *GitWorktree) SetupFromExistingBranch() error {
	// Ensure worktrees directory exists
	worktreesDir := filepath.Join(g.repoPath, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return fmt.Errorf("failed to create worktrees directory: %w", err)
	}

	// Clean up any existing worktree first
	_, _ = g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath) // Ignore error if worktree doesn't exist

	// Create a new worktree from the existing branch
	if _, err := g.runGitCommand(g.repoPath, "worktree", "add", g.worktreePath, g.branchName); err != nil {
		return fmt.Errorf("failed to create worktree from branch %s: %w", g.branchName, err)
	}

	// Run worktree setup if configured
	if err := g.runWorktreeSetup(); err != nil {
		log.WarningLog.Printf("failed to run worktree setup: %v", err)
		// Don't fail the worktree setup if setup fails
	}

	return nil
}

// SetupNewWorktree creates a new worktree from HEAD
func (g *GitWorktree) SetupNewWorktree() error {
	// Ensure worktrees directory exists
	worktreesDir := filepath.Join(g.repoPath, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return fmt.Errorf("failed to create worktrees directory: %w", err)
	}

	// Clean up any existing worktree first
	_, _ = g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath) // Ignore error if worktree doesn't exist

	// Open the repository
	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// Clean up any existing branch or reference
	if err := g.cleanupExistingBranch(repo); err != nil {
		return fmt.Errorf("failed to cleanup existing branch: %w", err)
	}

	output, err := g.runGitCommand(g.repoPath, "rev-parse", "HEAD")
	if err != nil {
		if strings.Contains(err.Error(), "fatal: ambiguous argument 'HEAD'") ||
			strings.Contains(err.Error(), "fatal: not a valid object name") ||
			strings.Contains(err.Error(), "fatal: HEAD: not a valid object name") {
			return fmt.Errorf("this appears to be a brand new repository: please create an initial commit before creating an instance")
		}
		return fmt.Errorf("failed to get HEAD commit hash: %w", err)
	}
	headCommit := strings.TrimSpace(string(output))
	g.baseCommitSHA = headCommit

	// Create a new worktree from the HEAD commit
	// Otherwise, we'll inherit uncommitted changes from the previous worktree.
	// This way, we can start the worktree with a clean slate.
	// TODO: we might want to give an option to use main/master instead of the current branch.
	if _, err := g.runGitCommand(g.repoPath, "worktree", "add", "-b", g.branchName, g.worktreePath, headCommit); err != nil {
		return fmt.Errorf("failed to create worktree from commit %s: %w", headCommit, err)
	}

	// Run worktree setup if configured
	if err := g.runWorktreeSetup(); err != nil {
		log.WarningLog.Printf("failed to run worktree setup: %v", err)
		// Don't fail the worktree setup if setup fails
	}

	return nil
}

// runWorktreeSetup runs the configured setup steps for new worktrees
func (g *GitWorktree) runWorktreeSetup() error {
	cfg := config.LoadConfig()
	if cfg.WorktreeSetup == nil {
		return nil // No setup configured
	}

	// Copy gitignored files first
	if err := g.copyGitIgnoredFiles(cfg.WorktreeSetup); err != nil {
		return fmt.Errorf("failed to copy gitignored files: %w", err)
	}

	// Run setup commands
	if err := g.runSetupCommands(cfg.WorktreeSetup); err != nil {
		return fmt.Errorf("failed to run setup commands: %w", err)
	}

	return nil
}

// copyGitIgnoredFiles copies gitignored files matching the configured patterns into the worktree
func (g *GitWorktree) copyGitIgnoredFiles(setup *config.WorktreeSetup) error {
	if len(setup.CopyIgnored) == 0 {
		return nil // Nothing to copy
	}

	for _, pattern := range setup.CopyIgnored {
		// Validate that the pattern doesn't start with /
		if strings.HasPrefix(pattern, "/") {
			log.WarningLog.Printf("skipping absolute path pattern: %s", pattern)
			continue
		}

		// Find matching files in the repository
		matches, err := filepath.Glob(filepath.Join(g.repoPath, pattern))
		if err != nil {
			log.WarningLog.Printf("invalid glob pattern %s: %v", pattern, err)
			continue
		}

		for _, srcPath := range matches {
			// Get relative path from repo root
			relPath, err := filepath.Rel(g.repoPath, srcPath)
			if err != nil {
				log.WarningLog.Printf("failed to get relative path for %s: %v", srcPath, err)
				continue
			}

			// Skip if the file is not gitignored
			if !g.isGitIgnored(relPath) {
				continue
			}

			// Determine destination path
			dstPath := filepath.Join(g.worktreePath, relPath)

			// Create destination directory if needed
			dstDir := filepath.Dir(dstPath)
			if err := os.MkdirAll(dstDir, 0755); err != nil {
				log.WarningLog.Printf("failed to create directory %s: %v", dstDir, err)
				continue
			}

			// Copy the file
			if err := g.copyFile(srcPath, dstPath); err != nil {
				log.WarningLog.Printf("failed to copy %s to %s: %v", srcPath, dstPath, err)
				continue
			}

			log.InfoLog.Printf("copied gitignored file: %s", relPath)
		}
	}

	return nil
}

// isGitIgnored checks if a file is ignored by git
func (g *GitWorktree) isGitIgnored(relPath string) bool {
	// Use git check-ignore to determine if file is ignored
	cmd := exec.Command("git", "-C", g.repoPath, "check-ignore", relPath)
	err := cmd.Run()
	// If the command exits with status 0, the file is ignored
	return err == nil
}

// copyFile copies a file from src to dst
func (g *GitWorktree) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// Skip directories
	if srcInfo.IsDir() {
		return nil
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// runSetupCommands runs the configured commands in the worktree
func (g *GitWorktree) runSetupCommands(setup *config.WorktreeSetup) error {
	if len(setup.Run) == 0 {
		return nil // No commands to run
	}

	for _, command := range setup.Run {
		log.InfoLog.Printf("running setup command: %s", command)
		
		// Run command in the worktree directory
		cmd := exec.Command("sh", "-c", command)
		cmd.Dir = g.worktreePath
		
		// Capture output instead of sending to console
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.ErrorLog.Printf("command failed: %s\nOutput: %s", command, string(output))
			return fmt.Errorf("failed to run command %q: %w", command, err)
		}
		
		// Log output for debugging if needed
		if len(output) > 0 {
			log.InfoLog.Printf("command output: %s", string(output))
		}
	}

	return nil
}

// Cleanup removes the worktree and associated branch
func (g *GitWorktree) Cleanup() error {
	var errs []error

	// Check if worktree path exists before attempting removal
	if _, err := os.Stat(g.worktreePath); err == nil {
		// Remove the worktree using git command
		if _, err := g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath); err != nil {
			errs = append(errs, err)
		}
	} else if !os.IsNotExist(err) {
		// Only append error if it's not a "not exists" error
		errs = append(errs, fmt.Errorf("failed to check worktree path: %w", err))
	}

	// Open the repository for branch cleanup
	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to open repository for cleanup: %w", err))
		return g.combineErrors(errs)
	}

	branchRef := plumbing.NewBranchReferenceName(g.branchName)

	// Check if branch exists before attempting removal
	if _, err := repo.Reference(branchRef, false); err == nil {
		if err := repo.Storer.RemoveReference(branchRef); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove branch %s: %w", g.branchName, err))
		}
	} else if err != plumbing.ErrReferenceNotFound {
		errs = append(errs, fmt.Errorf("error checking branch %s existence: %w", g.branchName, err))
	}

	// Prune the worktree to clean up any remaining references
	if err := g.Prune(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return g.combineErrors(errs)
	}

	return nil
}

// Remove removes the worktree but keeps the branch
func (g *GitWorktree) Remove() error {
	// Remove the worktree using git command
	if _, err := g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	return nil
}

// Prune removes all working tree administrative files and directories
func (g *GitWorktree) Prune() error {
	if _, err := g.runGitCommand(g.repoPath, "worktree", "prune"); err != nil {
		return fmt.Errorf("failed to prune worktrees: %w", err)
	}
	return nil
}

// CleanupWorktrees removes all worktrees and their associated branches
func CleanupWorktrees() error {
	worktreesDir, err := getWorktreeDirectory()
	if err != nil {
		return fmt.Errorf("failed to get worktree directory: %w", err)
	}

	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return fmt.Errorf("failed to read worktree directory: %w", err)
	}

	// Get a list of all branches associated with worktrees
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Parse the output to extract branch names
	worktreeBranches := make(map[string]string)
	currentWorktree := ""
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			currentWorktree = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			branchPath := strings.TrimPrefix(line, "branch ")
			// Extract branch name from refs/heads/branch-name
			branchName := strings.TrimPrefix(branchPath, "refs/heads/")
			if currentWorktree != "" {
				worktreeBranches[currentWorktree] = branchName
			}
		}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			worktreePath := filepath.Join(worktreesDir, entry.Name())

			// Delete the branch associated with this worktree if found
			for path, branch := range worktreeBranches {
				if strings.Contains(path, entry.Name()) {
					// Delete the branch
					deleteCmd := exec.Command("git", "branch", "-D", branch)
					if err := deleteCmd.Run(); err != nil {
						// Log the error but continue with other worktrees
						log.ErrorLog.Printf("failed to delete branch %s: %v", branch, err)
					}
					break
				}
			}

			// Remove the worktree directory
			os.RemoveAll(worktreePath)
		}
	}

	// You have to prune the cleaned up worktrees.
	cmd = exec.Command("git", "worktree", "prune")
	_, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to prune worktrees: %w", err)
	}

	return nil
}
