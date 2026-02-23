package session

import (
	"fmt"
	"time"

	"github.com/ByteMirror/hivemind/session/git"
)

// Topic groups related instances, optionally sharing a single git worktree.
type Topic struct {
	Name               string
	SharedWorktree     bool
	AutoYes            bool
	Branch             string
	Path               string
	CreatedAt          time.Time
	ExistingBranchName string // non-empty when an existing branch was adopted
	gitWorktree        *git.GitWorktree
	started            bool
}

type TopicOptions struct {
	Name               string
	SharedWorktree     bool
	Path               string
	ExistingBranchName string // if set + SharedWorktree, adopt this branch
}

func NewTopic(opts TopicOptions) *Topic {
	return &Topic{
		Name:               opts.Name,
		SharedWorktree:     opts.SharedWorktree,
		Path:               opts.Path,
		CreatedAt:          time.Now(),
		ExistingBranchName: opts.ExistingBranchName,
	}
}

func (t *Topic) Setup() error {
	if !t.SharedWorktree {
		t.started = true
		return nil
	}

	if t.ExistingBranchName != "" {
		existingPath, err := git.FindWorktreePathForBranch(t.Path, t.ExistingBranchName)
		if err != nil {
			return fmt.Errorf("failed to check worktree for branch: %w", err)
		}
		if existingPath != "" {
			// Branch already checked out — reuse it, don't touch it on cleanup
			repoRoot, err := git.FindGitRepoRoot(t.Path)
			if err != nil {
				return fmt.Errorf("failed to find repo root: %w", err)
			}
			t.gitWorktree = git.NewGitWorktreeReusingExisting(repoRoot, existingPath, t.ExistingBranchName)
		} else {
			// Branch exists but not checked out — create new worktree for it
			wt, err := git.NewGitWorktreeForExistingBranch(t.Path, t.Name, t.ExistingBranchName)
			if err != nil {
				return fmt.Errorf("failed to create worktree for existing branch: %w", err)
			}
			if err := wt.Setup(); err != nil {
				return fmt.Errorf("failed to setup topic worktree: %w", err)
			}
			t.gitWorktree = wt
		}
		t.Branch = t.ExistingBranchName
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
	// If this topic adopted an existing external worktree, skip all cleanup:
	// the worktree and branch belong to someone else.
	if t.ExistingBranchName != "" && !t.gitWorktree.IsManagedBranch() {
		return nil
	}
	return t.gitWorktree.Cleanup()
}
