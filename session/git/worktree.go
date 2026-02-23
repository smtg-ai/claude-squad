package git

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/ByteMirror/hivemind/config"
	"github.com/ByteMirror/hivemind/log"
)

func getWorktreeDirectory() (string, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, "worktrees"), nil
}

// GitWorktree manages git worktree operations for a session
type GitWorktree struct {
	// Path to the repository
	repoPath string
	// Path to the worktree
	worktreePath string
	// Name of the session
	sessionName string
	// Branch name for the worktree
	branchName string
	// Base commit hash for the worktree
	baseCommitSHA string
	// skipGitHooks controls whether --no-verify is passed to git commit
	skipGitHooks bool
	// managedBranch is true when hivemind created the branch and should delete it on cleanup.
	// false means the branch existed before and must be preserved.
	managedBranch bool
}

func NewGitWorktreeFromStorage(repoPath string, worktreePath string, sessionName string, branchName string, baseCommitSHA string, managedBranch bool) *GitWorktree {
	cfg := config.LoadConfig()
	return &GitWorktree{
		repoPath:      repoPath,
		worktreePath:  worktreePath,
		sessionName:   sessionName,
		branchName:    branchName,
		baseCommitSHA: baseCommitSHA,
		skipGitHooks:  cfg.ShouldSkipGitHooks(),
		managedBranch: managedBranch,
	}
}

// NewGitWorktree creates a new GitWorktree instance
func NewGitWorktree(repoPath string, sessionName string) (tree *GitWorktree, branchname string, err error) {
	cfg := config.LoadConfig()
	branchName := fmt.Sprintf("%s%s", cfg.BranchPrefix, sessionName)
	// Sanitize the final branch name to handle invalid characters from any source
	// (e.g., backslashes from Windows domain usernames like DOMAIN\user)
	branchName = sanitizeBranchName(branchName)

	// Convert repoPath to absolute path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		log.ErrorLog.Printf("git worktree path abs error, falling back to repoPath %s: %s", repoPath, err)
		// If we can't get absolute path, use original path as fallback
		absPath = repoPath
	}

	repoPath, err = FindGitRepoRoot(absPath)
	if err != nil {
		return nil, "", err
	}

	worktreeDir, err := getWorktreeDirectory()
	if err != nil {
		return nil, "", err
	}

	// Use sanitized branch name for the worktree directory name
	worktreePath := filepath.Join(worktreeDir, branchName)
	worktreePath = worktreePath + "_" + fmt.Sprintf("%x", time.Now().UnixNano())

	return &GitWorktree{
		repoPath:      repoPath,
		sessionName:   sessionName,
		branchName:    branchName,
		worktreePath:  worktreePath,
		skipGitHooks:  cfg.ShouldSkipGitHooks(),
		managedBranch: true, // we created this branch, so we own it
	}, branchName, nil
}

// GetWorktreePath returns the path to the worktree
func (g *GitWorktree) GetWorktreePath() string {
	return g.worktreePath
}

// GetBranchName returns the name of the branch associated with this worktree
func (g *GitWorktree) GetBranchName() string {
	return g.branchName
}

// GetRepoPath returns the path to the repository
func (g *GitWorktree) GetRepoPath() string {
	return g.repoPath
}

// GetRepoName returns the name of the repository (last part of the repoPath).
func (g *GitWorktree) GetRepoName() string {
	return filepath.Base(g.repoPath)
}

// GetBaseCommitSHA returns the base commit SHA for the worktree
func (g *GitWorktree) GetBaseCommitSHA() string {
	return g.baseCommitSHA
}

// IsManagedBranch returns true if hivemind created this branch (and should delete it on cleanup).
func (g *GitWorktree) IsManagedBranch() bool {
	return g.managedBranch
}

// NewGitWorktreeForExistingBranch creates a GitWorktree for an existing branch that is not
// yet checked out anywhere. It will create a new worktree directory for the branch,
// but will NOT delete the branch on Cleanup (managedBranch=false).
func NewGitWorktreeForExistingBranch(repoPath, sessionName, branchName string) (*GitWorktree, error) {
	cfg := config.LoadConfig()

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		log.ErrorLog.Printf("git worktree path abs error, falling back to repoPath %s: %s", repoPath, err)
		absPath = repoPath
	}

	repoRoot, err := FindGitRepoRoot(absPath)
	if err != nil {
		return nil, err
	}

	worktreeDir, err := getWorktreeDirectory()
	if err != nil {
		return nil, err
	}

	worktreePath := filepath.Join(worktreeDir, branchName) + "_" + fmt.Sprintf("%x", time.Now().UnixNano())

	return &GitWorktree{
		repoPath:      repoRoot,
		sessionName:   sessionName,
		branchName:    branchName,
		worktreePath:  worktreePath,
		skipGitHooks:  cfg.ShouldSkipGitHooks(),
		managedBranch: false, // branch already existed; preserve it on cleanup
	}, nil
}

// NewGitWorktreeReusingExisting returns a GitWorktree that points at an already-checked-out
// worktree path. It does NOT call Setup() — the worktree directory already exists.
// managedBranch is false so Cleanup() will skip branch deletion.
func NewGitWorktreeReusingExisting(repoPath, worktreePath, branchName string) *GitWorktree {
	cfg := config.LoadConfig()
	return &GitWorktree{
		repoPath:      repoPath,
		worktreePath:  worktreePath,
		branchName:    branchName,
		skipGitHooks:  cfg.ShouldSkipGitHooks(),
		managedBranch: false,
	}
}
