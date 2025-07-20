package git

import (
	"claude-squad/config"
	"claude-squad/log"
	"fmt"
	"path/filepath"
	"time"
)

// sessionNameToBranchAndPath returns the git worktree name from the session name.
func sessionNameToBranchAndPath(sessionName string) (branch string, path string) {
	cfg := config.LoadConfig()
	sanitizedName := sanitizeBranchName(sessionName)
	return fmt.Sprintf("%s%s", cfg.BranchPrefix, sanitizedName), sanitizedName
}

// getWorktreeDirectory returns the worktree directory for a specific repository
// if repoPath is empty, returns the base worktrees directory
// if cache is true, returns the worktrees-cache directory
func getWorktreeDirectory(repoPath string, cache bool) (string, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}

	var worktreeDir string
	if cache {
		worktreeDir = "worktrees-cache"
	} else {
		worktreeDir = "worktrees"
	}

	if repoPath == "" {
		return filepath.Join(configDir, worktreeDir), nil
	}

	repoName := filepath.Base(repoPath)
	return filepath.Join(configDir, worktreeDir, repoName), nil
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

	// exists only after the worktree is Setup()

	// Base commit hash for the worktree
	baseCommitSHA string
}

func NewGitWorktreeFromStorage(repoPath string, worktreePath string, sessionName string, branchName string, baseCommitSHA string) *GitWorktree {
	return &GitWorktree{
		repoPath:      repoPath,
		worktreePath:  worktreePath,
		sessionName:   sessionName,
		branchName:    branchName,
		baseCommitSHA: baseCommitSHA,
	}
}

// newGitWorktree creates a new GitWorktree instance
func newGitWorktree(repoPath string, sessionName string) (tree *GitWorktree, err error) {
	branchName, path := sessionNameToBranchAndPath(sessionName)
	// Convert repoPath to absolute path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		log.ErrorLog.Printf("git worktree path abs error, falling back to repoPath %s: %s", repoPath, err)
		// If we can't get absolute path, use original path as fallback
		absPath = repoPath
	}

	repoPath, err = findGitRepoRoot(absPath)
	if err != nil {
		return nil, err
	}

	worktreeDir, err := getWorktreeDirectory(repoPath, false)
	if err != nil {
		return nil, err
	}

	worktreePath := filepath.Join(worktreeDir, path)
	worktreePath = worktreePath + "_" + fmt.Sprintf("%x", time.Now().UnixNano())

	return &GitWorktree{
		repoPath:     repoPath,
		sessionName:  sessionName,
		branchName:   branchName,
		worktreePath: worktreePath,
	}, nil
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
