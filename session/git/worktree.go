package git

import (
	"claude-squad/config"
	"claude-squad/log"
	"claude-squad/session/vcs"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// GitWorktree manages git worktree operations for a session
type GitWorktree struct {
	RepoPath      string
	WorktreePath  string
	SessionName   string
	BranchName    string
	BaseCommitSHA string
}

func NewGitWorktreeFromStorage(repoPath string, worktreePath string, sessionName string, branchName string, baseCommitSHA string) *GitWorktree {
	return &GitWorktree{
		RepoPath:      repoPath,
		WorktreePath:  worktreePath,
		SessionName:   sessionName,
		BranchName:    branchName,
		BaseCommitSHA: baseCommitSHA,
	}
}

// NewGitWorktree creates a new GitWorktree instance
func NewGitWorktree(repoPath string, sessionName string) (*GitWorktree, string, error) {
	cfg := config.LoadConfig()
	sanitizedName := vcs.SanitizeBranchName(sessionName)
	branchName := fmt.Sprintf("%s%s", cfg.BranchPrefix, sanitizedName)

	worktreePath := filepath.Join(repoPath, ".claude-squad", "worktrees", sanitizedName)

	tree := &GitWorktree{
		RepoPath:     repoPath,
		WorktreePath: worktreePath,
		SessionName:  sessionName,
		BranchName:   branchName,
	}

	return tree, branchName, nil
}

// GetWorktreePath returns the path to the worktree
func (g *GitWorktree) GetWorktreePath() string {
	return g.WorktreePath
}

// GetBranchName returns the name of the branch associated with this worktree
func (g *GitWorktree) GetBranchName() string {
	return g.BranchName
}

// GetRepoPath returns the path to the repository
func (g *GitWorktree) GetRepoPath() string {
	return g.RepoPath
}

// GetRepoName returns the name of the repository (last part of the repoPath).
func (g *GitWorktree) GetRepoName() string {
	return filepath.Base(g.RepoPath)
}

// GetBaseCommitSHA returns the base commit SHA for the worktree
func (g *GitWorktree) GetBaseCommitSHA() string {
	return g.BaseCommitSHA
}

// Setup creates a new worktree for the session
func (g *GitWorktree) Setup() error {
	// Check if branch exists first
	repo, err := git.PlainOpen(g.RepoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	branchRef := plumbing.NewBranchReferenceName(g.BranchName)
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
	worktreesDir := filepath.Join(g.RepoPath, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return fmt.Errorf("failed to create worktrees directory: %w", err)
	}

	// Clean up any existing worktree first
	_, _ = g.runGitCommand(g.RepoPath, "worktree", "remove", "-f", g.WorktreePath) // Ignore error if worktree doesn't exist

	// Create a new worktree from the existing branch
	if _, err := g.runGitCommand(g.RepoPath, "worktree", "add", g.WorktreePath, g.BranchName); err != nil {
		return fmt.Errorf("failed to create worktree from branch %s: %w", g.BranchName, err)
	}

	return nil
}

// SetupNewWorktree creates a new worktree from HEAD
func (g *GitWorktree) SetupNewWorktree() error {
	// Ensure worktrees directory exists
	worktreesDir := filepath.Join(g.RepoPath, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return fmt.Errorf("failed to create worktrees directory: %w", err)
	}

	// Clean up any existing worktree first
	_, _ = g.runGitCommand(g.RepoPath, "worktree", "remove", "-f", g.WorktreePath) // Ignore error if worktree doesn't exist

	// Open the repository
	repo, err := git.PlainOpen(g.RepoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// Clean up any existing branch or reference
	if err := g.cleanupExistingBranch(repo); err != nil {
		return fmt.Errorf("failed to cleanup existing branch: %w", err)
	}

	output, err := g.runGitCommand(g.RepoPath, "rev-parse", "HEAD")
	if err != nil {
		if strings.Contains(err.Error(), "fatal: ambiguous argument 'HEAD'") ||
			strings.Contains(err.Error(), "fatal: not a valid object name") ||
			strings.Contains(err.Error(), "fatal: HEAD: not a valid object name") {
			return fmt.Errorf("this appears to be a brand new repository: please create an initial commit before creating an instance")
		}
		return fmt.Errorf("failed to get HEAD commit hash: %w", err)
	}
	headCommit := strings.TrimSpace(string(output))
	g.BaseCommitSHA = headCommit

	// Create a new worktree from the HEAD commit
	// Otherwise, we'll inherit uncommitted changes from the previous worktree.
	// This way, we can start the worktree with a clean slate.
	// TODO: we might want to give an option to use main/master instead of the current branch.
	if _, err := g.runGitCommand(g.RepoPath, "worktree", "add", "-b", g.BranchName, g.WorktreePath, headCommit); err != nil {
		return fmt.Errorf("failed to create worktree from commit %s: %w", headCommit, err)
	}

	return nil
}

// Cleanup removes the worktree and associated branch
func (g *GitWorktree) Cleanup() error {
	var errs []error

	// Check if worktree path exists before attempting removal
	if _, err := os.Stat(g.WorktreePath); err == nil {
		// Remove the worktree using git command
		if _, err := g.runGitCommand(g.RepoPath, "worktree", "remove", "-f", g.WorktreePath); err != nil {
			errs = append(errs, err)
		}
	} else if !os.IsNotExist(err) {
		// Only append error if it's not a "not exists" error
		errs = append(errs, fmt.Errorf("failed to check worktree path: %w", err))
	}

	// Open the repository for branch cleanup
	repo, err := git.PlainOpen(g.RepoPath)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to open repository for cleanup: %w", err))
		return vcs.CombineErrors(errs)
	}

	branchRef := plumbing.NewBranchReferenceName(g.BranchName)

	// Check if branch exists before attempting removal
	if _, err := repo.Reference(branchRef, false); err == nil {
		if err := repo.Storer.RemoveReference(branchRef); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove branch %s: %w", g.BranchName, err))
		}
	} else if err != plumbing.ErrReferenceNotFound {
		errs = append(errs, fmt.Errorf("error checking branch %s existence: %w", g.BranchName, err))
	}

	// Prune the worktree to clean up any remaining references
	if err := g.Prune(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return vcs.CombineErrors(errs)
	}

	return nil
}

// Remove removes the worktree but keeps the branch
func (g *GitWorktree) Remove() error {
	// Remove the worktree using git command
	if _, err := g.runGitCommand(g.RepoPath, "worktree", "remove", "-f", g.WorktreePath); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	return nil
}

// Prune removes all working tree administrative files and directories
func (g *GitWorktree) Prune() error {
	if _, err := g.runGitCommand(g.RepoPath, "worktree", "prune"); err != nil {
		return fmt.Errorf("failed to prune worktrees: %w", err)
	}
	return nil
}


// cleanupExistingBranch removes the branch if it exists
func (g *GitWorktree) cleanupExistingBranch(repo *git.Repository) error {
	branchRef := plumbing.NewBranchReferenceName(g.BranchName)
	if _, err := repo.Reference(branchRef, false); err == nil {
		// Branch exists, delete it
		if err := repo.Storer.RemoveReference(branchRef); err != nil {
			return fmt.Errorf("failed to remove existing branch %s: %w", g.BranchName, err)
		}
	} else if err != plumbing.ErrReferenceNotFound {
		return fmt.Errorf("error checking branch %s existence: %w", g.BranchName, err)
	}
	return nil
}

// runGitCommand executes a git command and returns its output
func (g *GitWorktree) runGitCommand(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("git command failed: %s\n%s", err, output)
	}
	return output, nil
}

// GitCleanupWorktrees removes all worktrees created by the application
func GitCleanupWorktrees() error {
	// Get all worktrees
	output, err := runGitCommand(".", "worktree", "list", "--porcelain")
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	worktrees, err := parseWorktreeOutput(string(output))
	if err != nil {
		return fmt.Errorf("failed to parse worktree output: %w", err)
	}

	var errs []error
	for _, wt := range worktrees {
		if strings.Contains(wt.Path, ".claude-squad/worktrees") {
			log.InfoLog.Printf("Cleaning up worktree: %s", wt.Path)
			if _, err := runGitCommand(".", "worktree", "remove", "-f", wt.Path); err != nil {
				errs = append(errs, fmt.Errorf("failed to remove worktree %s: %w", wt.Path, err))
			}
		}
	}

	if len(errs) > 0 {
		return vcs.CombineErrors(errs)
	}

	return nil
}

type worktreeInfo struct {
	Path   string
	Branch string
}

func parseWorktreeOutput(output string) ([]worktreeInfo, error) {
	var worktrees []worktreeInfo
	lines := strings.Split(output, "\n")
	var currentWT worktreeInfo

	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			if currentWT.Path != "" {
				worktrees = append(worktrees, currentWT)
			}
			currentWT = worktreeInfo{Path: strings.TrimPrefix(line, "worktree ")}
		} else if strings.HasPrefix(line, "branch ") {
			currentWT.Branch = strings.TrimPrefix(line, "branch ")}
	}
	if currentWT.Path != "" {
		worktrees = append(worktrees, currentWT)
	}

	return worktrees, nil
}

// runGitCommand is a standalone git command runner
func runGitCommand(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("git command failed: %s\n%s", err, output)
	}
	return output, nil
}