//go:build !pro

package git

func NewTreeSource(_ string) WorktreeSource {
	return &SimpleWorktreeSource{}
}

func CleanupWorktreeCache() error {
	return nil
}

// SimpleWorktreeSource is a passthrough for newGitWorktree.
type SimpleWorktreeSource struct{}

func (s *SimpleWorktreeSource) GetGitWorktree(repoPath string, sessionName string) (
	*GitWorktree,
	error,
) {
	worktree, err := newGitWorktree(repoPath, sessionName)
	if err != nil {
		return nil, err
	}
	if err := worktree.Setup(); err != nil {
		return nil, err
	}
	return worktree, nil
}
