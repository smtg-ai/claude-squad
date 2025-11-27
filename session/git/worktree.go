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
	// Submodule worktrees
	submodules []*SubmoduleWorktree
}

func NewGitWorktreeFromStorage(repoPath string, worktreePath string, sessionName string, branchName string, baseCommitSHA string, submodulesData []SubmoduleWorktreeData) *GitWorktree {
	var submodules []*SubmoduleWorktree
	for _, data := range submodulesData {
		submodules = append(submodules, NewSubmoduleWorktreeFromStorage(data))
	}
	return &GitWorktree{
		repoPath:      repoPath,
		worktreePath:  worktreePath,
		sessionName:   sessionName,
		branchName:    branchName,
		baseCommitSHA: baseCommitSHA,
		submodules:    submodules,
	}
}

// GetSubmodules returns the list of submodule worktrees
func (g *GitWorktree) GetSubmodules() []*SubmoduleWorktree {
	return g.submodules
}

// GetSubmodulesData returns serializable data for all submodules
func (g *GitWorktree) GetSubmodulesData() []SubmoduleWorktreeData {
	var data []SubmoduleWorktreeData
	for _, sub := range g.submodules {
		data = append(data, sub.ToData())
	}
	return data
}

// NewGitWorktreeFromExisting creates a GitWorktree instance that references an existing worktree
// This is used when multiple sessions share the same worktree
func NewGitWorktreeFromExisting(existingWorktreePath string, sessionName string) (*GitWorktree, error) {
	// Get the main repository path from the worktree
	// For worktrees, 'git rev-parse --git-common-dir' returns the path to the main repo's .git
	repoPath, err := getMainRepoPath(existingWorktreePath)
	if err != nil {
		// Fallback to findGitRepoRoot if we can't get the main repo path
		repoPath, err = findGitRepoRoot(existingWorktreePath)
		if err != nil {
			return nil, fmt.Errorf("failed to find git repo root: %w", err)
		}
	}

	// Get the branch name from the worktree
	branchName, err := getCurrentBranchFromWorktree(existingWorktreePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get branch name: %w", err)
	}

	// Get base commit SHA
	baseCommitSHA, err := getHeadCommitSHA(existingWorktreePath)
	if err != nil {
		log.ErrorLog.Printf("failed to get base commit SHA: %v", err)
		// Not fatal, continue without it
	}

	g := &GitWorktree{
		repoPath:      repoPath,
		worktreePath:  existingWorktreePath,
		sessionName:   sessionName,
		branchName:    branchName,
		baseCommitSHA: baseCommitSHA,
	}

	// Detect existing submodule worktrees so they can be cleaned up properly
	if err := g.detectExistingSubmoduleWorktrees(); err != nil {
		log.ErrorLog.Printf("failed to detect submodule worktrees: %v", err)
		// Not fatal, continue without submodules
	}

	return g, nil
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

	repoPath, err = findGitRepoRoot(absPath)
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
		repoPath:     repoPath,
		sessionName:  sessionName,
		branchName:   branchName,
		worktreePath: worktreePath,
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
