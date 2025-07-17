package git

import (
	"claude-squad/config"
	"claude-squad/log"
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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
	// Parent branch to create the worktree from
	parentBranch string
}

func NewGitWorktreeFromStorage(repoPath string, worktreePath string, sessionName string, branchName string, baseCommitSHA string, parentBranch string) *GitWorktree {
	return &GitWorktree{
		repoPath:      repoPath,
		worktreePath:  worktreePath,
		sessionName:   sessionName,
		branchName:    branchName,
		baseCommitSHA: baseCommitSHA,
		parentBranch:  parentBranch,
	}
}

// NewGitWorktree creates a new GitWorktree instance
func NewGitWorktree(repoPath string, sessionName string, parentBranch string) (tree *GitWorktree, branchname string, err error) {
	cfg := config.LoadConfig()
	sanitizedName := sanitizeBranchName(sessionName)
	branchName := fmt.Sprintf("%s%s", cfg.BranchPrefix, sanitizedName)

	// Use default parent branch if not specified
	if parentBranch == "" {
		parentBranch = cfg.DefaultParentBranch
		if parentBranch == "" {
			parentBranch = "HEAD" // fallback to current behavior
		}
	}

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

	worktreePath := filepath.Join(worktreeDir, sanitizedName)
	worktreePath = worktreePath + "_" + fmt.Sprintf("%x", time.Now().UnixNano())

	return &GitWorktree{
		repoPath:     repoPath,
		sessionName:  sessionName,
		branchName:   branchName,
		worktreePath: worktreePath,
		parentBranch: parentBranch,
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

// GetParentBranch returns the parent branch for the worktree
func (g *GitWorktree) GetParentBranch() string {
	return g.parentBranch
}

// GetAvailableBranches returns a list of available branches in the repository
func (g *GitWorktree) GetAvailableBranches() ([]string, error) {
	return GetAvailableBranches(g.repoPath)
}

// GetAvailableBranches returns a list of available branches for a given repository path
func GetAvailableBranches(repoPath string) ([]string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	refs, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("failed to get references: %w", err)
	}

	var branches []string
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsBranch() {
			branchName := ref.Name().Short()
			branches = append(branches, branchName)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate references: %w", err)
	}

	return branches, nil
}
