package git

import (
	"claude-squad/config"
	"claude-squad/log"
	"fmt"
	"path/filepath"
	"time"
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
	// isExistingBranch is true if the branch existed before the session was created.
	// When true, the branch will not be deleted on cleanup.
	isExistingBranch bool
	// baseRef is the ref to base a new branch on (e.g., "origin/main").
	// Only used during Setup for new worktrees. Empty means use HEAD.
	baseRef string
}

func NewGitWorktreeFromStorage(repoPath string, worktreePath string, sessionName string, branchName string, baseCommitSHA string, isExistingBranch bool) *GitWorktree {
	return &GitWorktree{
		repoPath:         repoPath,
		worktreePath:     worktreePath,
		sessionName:      sessionName,
		branchName:       branchName,
		baseCommitSHA:    baseCommitSHA,
		isExistingBranch: isExistingBranch,
	}
}

// resolveWorktreePaths resolves the repo root and generates a unique worktree path for the given branch name.
func resolveWorktreePaths(repoPath string, branchName string) (resolvedRepo string, worktreePath string, err error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		log.ErrorLog.Printf("git worktree path abs error, falling back to repoPath %s: %s", repoPath, err)
		absPath = repoPath
	}

	resolvedRepo, err = findGitRepoRoot(absPath)
	if err != nil {
		return "", "", err
	}

	worktreeDir, err := getWorktreeDirectory()
	if err != nil {
		return "", "", err
	}

	worktreePath = filepath.Join(worktreeDir, sanitizeBranchName(branchName))
	worktreePath = worktreePath + "_" + fmt.Sprintf("%x", time.Now().UnixNano())

	return resolvedRepo, worktreePath, nil
}

// NewGitWorktree creates a new GitWorktree instance
func NewGitWorktree(repoPath string, sessionName string) (tree *GitWorktree, branchname string, err error) {
	cfg := config.LoadConfig()
	branchName := fmt.Sprintf("%s%s", cfg.BranchPrefix, sessionName)
	// Sanitize the final branch name to handle invalid characters from any source
	// (e.g., backslashes from Windows domain usernames like DOMAIN\user)
	branchName = sanitizeBranchName(branchName)

	repoPath, worktreePath, err := resolveWorktreePaths(repoPath, branchName)
	if err != nil {
		return nil, "", err
	}

	return &GitWorktree{
		repoPath:     repoPath,
		sessionName:  sessionName,
		branchName:   branchName,
		worktreePath: worktreePath,
	}, branchName, nil
}

// NewGitWorktreeFromBranch creates a new GitWorktree that uses an existing branch.
// The branch will not be deleted on cleanup.
func NewGitWorktreeFromBranch(repoPath string, branchName string, sessionName string) (*GitWorktree, error) {
	repoPath, worktreePath, err := resolveWorktreePaths(repoPath, branchName)
	if err != nil {
		return nil, err
	}

	return &GitWorktree{
		repoPath:         repoPath,
		sessionName:      sessionName,
		branchName:       branchName,
		worktreePath:     worktreePath,
		isExistingBranch: true,
	}, nil
}

// NewGitWorktreeFromRef creates a new GitWorktree with a new branch based on a specific ref
// (e.g., "origin/main"). The new branch is named using the configured branch prefix + session name.
func NewGitWorktreeFromRef(repoPath string, baseRef string, sessionName string) (tree *GitWorktree, branchName string, err error) {
	cfg := config.LoadConfig()
	branchName = fmt.Sprintf("%s%s", cfg.BranchPrefix, sessionName)
	branchName = sanitizeBranchName(branchName)

	repoPath, worktreePath, err := resolveWorktreePaths(repoPath, branchName)
	if err != nil {
		return nil, "", err
	}

	return &GitWorktree{
		repoPath:     repoPath,
		sessionName:  sessionName,
		branchName:   branchName,
		worktreePath: worktreePath,
		baseRef:      baseRef,
	}, branchName, nil
}

// IsExistingBranch returns whether this worktree uses a pre-existing branch
func (g *GitWorktree) IsExistingBranch() bool {
	return g.isExistingBranch
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
