package git

import (
	"claude-squad/log"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
	worktreesDir, err := getWorktreeDirectory()
	if err != nil {
		return fmt.Errorf("failed to get worktree directory: %w", err)
	}
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return fmt.Errorf("failed to create worktrees directory: %w", err)
	}

	// Clean up any existing worktree first
	if _, err := g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath); err != nil {
		// Log the error but continue - worktree might not exist
		log.InfoLog.Printf("Warning: failed to remove existing worktree %s: %v", g.worktreePath, err)
	}

	// Create a new worktree from the existing branch
	if _, err := g.runGitCommand(g.repoPath, "worktree", "add", g.worktreePath, g.branchName); err != nil {
		return fmt.Errorf("failed to create worktree from branch %s: %w", g.branchName, err)
	}

	return nil
}

// SetupNewWorktree creates a new worktree from HEAD
func (g *GitWorktree) SetupNewWorktree() error {
	// Ensure worktrees directory exists
	worktreesDir, err := getWorktreeDirectory()
	if err != nil {
		return fmt.Errorf("failed to get worktree directory: %w", err)
	}
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return fmt.Errorf("failed to create worktrees directory: %w", err)
	}

	// Clean up any existing worktree first
	if _, err := g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath); err != nil {
		// Log the error but continue - worktree might not exist
		log.InfoLog.Printf("Warning: failed to remove existing worktree %s: %v", g.worktreePath, err)
	}

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

	return nil
}

// Cleanup removes the worktree and associated branch
func (g *GitWorktree) Cleanup() error {
	// Use write lock to prevent concurrent operations during cleanup
	g.opMu.Lock()
	defer g.opMu.Unlock()

	return g.cleanupWithRetry(3, 100*time.Millisecond)
}

// cleanupWithRetry performs cleanup with retry logic for robust cleanup
func (g *GitWorktree) cleanupWithRetry(maxRetries int, retryDelay time.Duration) error {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Progressive delay: 100ms, 200ms, 300ms, etc.
			time.Sleep(retryDelay * time.Duration(attempt))
			log.InfoLog.Printf("Retrying cleanup for branch %s (attempt %d/%d)", g.branchName, attempt+1, maxRetries)
		}

		err := g.performAtomicCleanup()
		if err == nil {
			// Success - verify cleanup was complete
			if verifyErr := g.verifyCleanupComplete(); verifyErr != nil {
				log.InfoLog.Printf("Cleanup verification failed for branch %s: %v", g.branchName, verifyErr)
				lastErr = verifyErr

				// If verification fails due to branch recreation test, it might be a timing issue
				// Give it a bit more time on the next retry
				if strings.Contains(verifyErr.Error(), "cannot be immediately recreated") && attempt < maxRetries-1 {
					time.Sleep(50 * time.Millisecond)
				}
				continue
			}
			log.InfoLog.Printf("Successfully cleaned up worktree and branch %s", g.branchName)
			return nil
		}

		lastErr = err
		log.InfoLog.Printf("Cleanup attempt %d failed for branch %s: %v", attempt+1, g.branchName, err)

		// If it's just a "not found" error, that's actually success - the resource is already gone
		if g.isAlreadyCleanedUpError(err) {
			log.InfoLog.Printf("Branch %s appears to be already cleaned up", g.branchName)
			return nil
		}
	}

	return fmt.Errorf("cleanup failed after %d attempts: %w", maxRetries, lastErr)
}

// performAtomicCleanup performs a single atomic cleanup operation
func (g *GitWorktree) performAtomicCleanup() error {
	var errs []error

	// Step 1: Remove all worktrees associated with this branch
	if err := g.removeWorktrees(); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove worktrees: %w", err))
	}

	// Step 2: Force remove branch using both git library and command-line
	if err := g.removeBranchRobust(); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove branch: %w", err))
	}

	// Step 3: Prune to clean up any remaining references
	if err := g.Prune(); err != nil {
		errs = append(errs, fmt.Errorf("failed to prune worktrees: %w", err))
	}

	if len(errs) > 0 {
		return g.combineErrors(errs)
	}

	return nil
}

// removeWorktrees removes all worktrees associated with this branch
func (g *GitWorktree) removeWorktrees() error {
	var errs []error

	// Get list of all worktrees and find ones associated with our branch
	output, err := g.runGitCommand(g.repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		// If listing fails, try removing the stored path as fallback
		if _, statErr := os.Stat(g.worktreePath); statErr == nil {
			if _, removeErr := g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath); removeErr != nil {
				// Only report error if it's not a "not a working tree" error
				if !strings.Contains(removeErr.Error(), "not a working tree") {
					errs = append(errs, fmt.Errorf("failed to remove worktree %s: %w", g.worktreePath, removeErr))
				}
			}
		}
		return g.combineErrors(errs)
	}

	// Parse worktree list to find all worktrees with our branch
	lines := strings.Split(string(output), "\n")
	var worktreesToRemove []string

	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "worktree ") {
			worktreePath := strings.TrimPrefix(lines[i], "worktree ")
			// Check subsequent lines for branch reference
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				if strings.Contains(lines[j], g.branchName) {
					worktreesToRemove = append(worktreesToRemove, worktreePath)
					break
				}
			}
		}
	}

	// Remove all found worktrees
	for _, worktreePath := range worktreesToRemove {
		if _, err := g.runGitCommand(g.repoPath, "worktree", "remove", "-f", worktreePath); err != nil {
			// Only report error if it's not a "not a working tree" error
			if !strings.Contains(err.Error(), "not a working tree") {
				errs = append(errs, fmt.Errorf("failed to remove worktree %s: %w", worktreePath, err))
			}
		}
	}

	return g.combineErrors(errs)
}

// removeBranchRobust removes the branch using multiple methods for robustness
func (g *GitWorktree) removeBranchRobust() error {
	var errs []error

	// Method 1: Use go-git library
	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to open repository for branch cleanup: %w", err))
	} else {
		branchRef := plumbing.NewBranchReferenceName(g.branchName)

		// Check if branch exists and remove it
		if _, err := repo.Reference(branchRef, false); err == nil {
			if err := repo.Storer.RemoveReference(branchRef); err != nil {
				errs = append(errs, fmt.Errorf("failed to remove branch reference %s using go-git: %w", g.branchName, err))
			}
		} else if err != plumbing.ErrReferenceNotFound {
			errs = append(errs, fmt.Errorf("error checking branch %s existence: %w", g.branchName, err))
		}

		// Also clean up using the more thorough method
		if cleanupErr := g.cleanupExistingBranch(repo); cleanupErr != nil {
			errs = append(errs, fmt.Errorf("failed to cleanup existing branch: %w", cleanupErr))
		}
	}

	// Method 2: Use git command-line for force deletion (more robust)
	if _, err := g.runGitCommand(g.repoPath, "branch", "-D", g.branchName); err != nil {
		// Only add to errors if it's not "branch not found"
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "does not exist") {
			errs = append(errs, fmt.Errorf("failed to force delete branch %s using git command: %w", g.branchName, err))
		}
	}

	return g.combineErrors(errs)
}

// verifyCleanupComplete verifies that the cleanup was successful
func (g *GitWorktree) verifyCleanupComplete() error {
	var errs []error

	// Verify branch is gone
	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to open repository for verification: %w", err))
	} else {
		branchRef := plumbing.NewBranchReferenceName(g.branchName)
		if _, err := repo.Reference(branchRef, false); err == nil {
			errs = append(errs, fmt.Errorf("branch %s still exists after cleanup", g.branchName))
		} else if err != plumbing.ErrReferenceNotFound {
			errs = append(errs, fmt.Errorf("error verifying branch %s removal: %w", g.branchName, err))
		}
	}

	// Verify no worktrees are associated with this branch
	if output, err := g.runGitCommand(g.repoPath, "worktree", "list", "--porcelain"); err == nil {
		if strings.Contains(string(output), g.branchName) {
			errs = append(errs, fmt.Errorf("worktrees still associated with branch %s after cleanup", g.branchName))
		}
	} else {
		// If we can't list worktrees, just log a warning
		log.InfoLog.Printf("Warning: could not verify worktree cleanup for branch %s: %v", g.branchName, err)
	}

	// Test that a new worktree with the same branch name can be created immediately
	// This is the ultimate test to ensure no race condition exists
	if err := g.testBranchRecreatability(); err != nil {
		errs = append(errs, fmt.Errorf("branch %s cannot be immediately recreated: %w", g.branchName, err))
	}

	return g.combineErrors(errs)
}

// testBranchRecreatability tests if a branch with the same name can be created immediately
// This is the ultimate test to ensure cleanup was complete and no race conditions exist
func (g *GitWorktree) testBranchRecreatability() error {
	// Try to create a test branch with the same name to verify it's completely cleaned up
	if _, err := g.runGitCommand(g.repoPath, "branch", g.branchName, "HEAD"); err != nil {
		// If branch creation fails, the cleanup was incomplete
		return fmt.Errorf("cannot create branch %s: %w", g.branchName, err)
	}

	// Clean up the test branch immediately
	if _, err := g.runGitCommand(g.repoPath, "branch", "-D", g.branchName); err != nil {
		// Log warning but don't fail verification - the main goal was to test createability
		log.InfoLog.Printf("Warning: failed to clean up test branch %s: %v", g.branchName, err)
	}

	return nil
}

// isAlreadyCleanedUpError checks if an error indicates the resources are already cleaned up
func (g *GitWorktree) isAlreadyCleanedUpError(err error) bool {
	if err == nil {
		return true
	}

	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "does not exist") ||
		strings.Contains(errStr, "no such file") ||
		strings.Contains(errStr, "already deleted") ||
		strings.Contains(errStr, "reference not found")
}

// combineErrors combines multiple errors into a single error
func (g *GitWorktree) combineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}

	errMsg := "multiple errors occurred:"
	for _, err := range errs {
		errMsg += "\n  - " + err.Error()
	}
	return fmt.Errorf("%s", errMsg)
}

// Remove removes the worktree but keeps the branch
func (g *GitWorktree) Remove() error {
	// Use write lock to prevent concurrent operations during removal
	g.opMu.Lock()
	defer g.opMu.Unlock()

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
