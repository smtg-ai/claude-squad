package git

import (
	"claude-squad/log"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Setup creates a new worktree for the session
func (g *GitWorktree) Setup() error {
	// Ensure worktrees directory exists early (can be done in parallel with branch check)
	worktreesDir, err := getWorktreeDirectory()
	if err != nil {
		return fmt.Errorf("failed to get worktree directory: %w", err)
	}

	// Create directory and check branch existence in parallel
	errChan := make(chan error, 2)
	var branchExists bool

	// Goroutine for directory creation
	go func() {
		errChan <- os.MkdirAll(worktreesDir, 0755)
	}()

	// Goroutine for branch check
	go func() {
		repo, err := git.PlainOpen(g.repoPath)
		if err != nil {
			errChan <- fmt.Errorf("failed to open repository: %w", err)
			return
		}

		branchRef := plumbing.NewBranchReferenceName(g.branchName)
		if _, err := repo.Reference(branchRef, false); err == nil {
			branchExists = true
		}
		errChan <- nil
	}()

	// Wait for both operations
	for i := 0; i < 2; i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}

	if branchExists {
		return g.setupFromExistingBranch()
	}
	return g.setupNewWorktree()
}

// setupFromExistingBranch creates a worktree from an existing branch
func (g *GitWorktree) setupFromExistingBranch() error {
	// Directory already created in Setup(), skip duplicate creation

	// Clean up any existing worktree first
	_, _ = g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath) // Ignore error if worktree doesn't exist

	// Check if we need to track a remote branch
	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	localBranchRef := plumbing.NewBranchReferenceName(g.branchName)
	_, localErr := repo.Reference(localBranchRef, false)

	remoteBranchRef := plumbing.NewRemoteReferenceName("origin", g.branchName)
	_, remoteErr := repo.Reference(remoteBranchRef, false)

	if localErr != nil && remoteErr == nil {
		// Local branch doesn't exist but remote does, create tracking branch
		if _, err := g.runGitCommand(g.repoPath, "worktree", "add", "-b", g.branchName, g.worktreePath, "origin/"+g.branchName); err != nil {
			return fmt.Errorf("failed to create worktree from remote branch %s: %w", g.branchName, err)
		}
		
		// Explicitly set upstream tracking for the newly created branch
		if _, err := g.runGitCommand(g.worktreePath, "branch", "--set-upstream-to=origin/"+g.branchName, g.branchName); err != nil {
			// Log warning but don't fail - branch is still functional
			fmt.Printf("Warning: failed to set upstream tracking for branch %s: %v\n", g.branchName, err)
		}
	} else {
		// Create a new worktree from the existing local branch
		if _, err := g.runGitCommand(g.repoPath, "worktree", "add", g.worktreePath, g.branchName); err != nil {
			return fmt.Errorf("failed to create worktree from branch %s: %w", g.branchName, err)
		}
		
		// Check if this local branch should track a remote branch
		if remoteErr == nil {
			// Remote branch exists, ensure tracking is set up
			if _, err := g.runGitCommand(g.worktreePath, "branch", "--set-upstream-to=origin/"+g.branchName, g.branchName); err != nil {
				// Log warning but don't fail
				fmt.Printf("Warning: failed to set upstream tracking for branch %s: %v\n", g.branchName, err)
			}
		}
	}

	return nil
}

// setupNewWorktree creates a new worktree from HEAD
func (g *GitWorktree) setupNewWorktree() error {
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
			_ = os.RemoveAll(worktreePath)
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

// CheckRemoteBranchStatic checks if a branch exists on remote without requiring a worktree instance
func CheckRemoteBranchStatic(repoPath, branchName string) (exists bool, needsSync bool, err error) {
	cmd := exec.Command("git", "ls-remote", "origin", "refs/heads/"+branchName)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		// If command fails, assume no remote
		return false, false, nil
	}

	// If output is empty, remote branch doesn't exist
	if strings.TrimSpace(string(output)) == "" {
		return false, false, nil
	}

	exists = true

	// Check if we need to sync (compare local vs remote)
	// First, check if local branch exists
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return exists, false, fmt.Errorf("failed to open repository: %w", err)
	}

	localBranchRef := plumbing.NewBranchReferenceName(branchName)
	localRef, err := repo.Reference(localBranchRef, false)
	if err != nil {
		// Local branch doesn't exist, so we need to sync to get remote
		return exists, true, nil
	}

	// Fetch the remote branch reference
	remoteBranchRef := plumbing.NewRemoteReferenceName("origin", branchName)
	remoteRef, err := repo.Reference(remoteBranchRef, false)
	if err != nil {
		// Remote ref not found locally, need to fetch
		return exists, true, nil
	}

	// Compare hashes
	needsSync = localRef.Hash() != remoteRef.Hash()
	return exists, needsSync, nil
}

// CheckRemoteBranch checks if a branch exists on the remote and returns comparison info
func (g *GitWorktree) CheckRemoteBranch(branchName string) (exists bool, needsSync bool, err error) {
	// Check if remote branch exists
	output, err := g.runGitCommand(g.repoPath, "ls-remote", "origin", "refs/heads/"+branchName)
	if err != nil {
		// If command fails, assume no remote
		return false, false, nil
	}

	// If output is empty, remote branch doesn't exist
	if strings.TrimSpace(string(output)) == "" {
		return false, false, nil
	}

	exists = true

	// Check if we need to sync (compare local vs remote)
	// First, check if local branch exists
	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		return exists, false, fmt.Errorf("failed to open repository: %w", err)
	}

	localBranchRef := plumbing.NewBranchReferenceName(branchName)
	localRef, err := repo.Reference(localBranchRef, false)
	if err != nil {
		// Local branch doesn't exist, so we need to sync to get remote
		return exists, true, nil
	}

	// Fetch the remote branch reference
	remoteBranchRef := plumbing.NewRemoteReferenceName("origin", branchName)
	remoteRef, err := repo.Reference(remoteBranchRef, false)
	if err != nil {
		// Remote ref not found locally, need to fetch
		return exists, true, nil
	}

	// Compare hashes
	needsSync = localRef.Hash() != remoteRef.Hash()
	return exists, needsSync, nil
}

// SyncWithRemoteBranchStatic fetches a remote branch without requiring a worktree instance
func SyncWithRemoteBranchStatic(repoPath, branchName string) error {
	cmd := exec.Command("git", "fetch", "origin", branchName)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch remote branch %s: %w", branchName, err)
	}
	return nil
}

// SyncWithRemoteBranch fetches and syncs with the remote branch
func (g *GitWorktree) SyncWithRemoteBranch(branchName string) error {
	// Fetch the specific branch from remote
	if _, err := g.runGitCommand(g.repoPath, "fetch", "origin", branchName); err != nil {
		return fmt.Errorf("failed to fetch remote branch %s: %w", branchName, err)
	}

	return nil
}
