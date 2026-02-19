package session

import (
	"claude-squad/session/git"
	"fmt"
	"time"
)

// Topic groups related instances, optionally sharing a single git worktree.
type Topic struct {
	Name           string
	SharedWorktree bool
	Branch         string
	Path           string
	CreatedAt      time.Time
	gitWorktree    *git.GitWorktree
	started        bool
}

type TopicOptions struct {
	Name           string
	SharedWorktree bool
	Path           string
}

func NewTopic(opts TopicOptions) *Topic {
	return &Topic{
		Name:           opts.Name,
		SharedWorktree: opts.SharedWorktree,
		Path:           opts.Path,
		CreatedAt:      time.Now(),
	}
}

func (t *Topic) Setup() error {
	if !t.SharedWorktree {
		t.started = true
		return nil
	}
	gitWorktree, branchName, err := git.NewGitWorktree(t.Path, t.Name)
	if err != nil {
		return fmt.Errorf("failed to create topic worktree: %w", err)
	}
	if err := gitWorktree.Setup(); err != nil {
		return fmt.Errorf("failed to setup topic worktree: %w", err)
	}
	t.gitWorktree = gitWorktree
	t.Branch = branchName
	t.started = true
	return nil
}

func (t *Topic) GetWorktreePath() string {
	if t.gitWorktree == nil {
		return ""
	}
	return t.gitWorktree.GetWorktreePath()
}

func (t *Topic) GetGitWorktree() *git.GitWorktree {
	return t.gitWorktree
}

func (t *Topic) Started() bool {
	return t.started
}

func (t *Topic) Cleanup() error {
	if t.gitWorktree == nil {
		return nil
	}
	return t.gitWorktree.Cleanup()
}
