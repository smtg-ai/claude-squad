package git

// WorktreeSource is an interface for getting a git worktree.
type WorktreeSource interface {
	// GetGitWorktree returns a new git worktree for the given repoPath and sessionName.
	// It calls Setup() on the worktree. It should be used for new instances and not for
	// resuming existing instances because the setup sematnics are different for existing
	// worktrees.
	GetGitWorktree(repoPath string, sessionName string) (*GitWorktree, error)
}
