package session

import (
	"time"

	"github.com/ByteMirror/hivemind/session/git"
)

// TopicData represents the serializable data of a Topic.
type TopicData struct {
	Name               string          `json:"name"`
	SharedWorktree     bool            `json:"shared_worktree"`
	AutoYes            bool            `json:"auto_yes"`
	Branch             string          `json:"branch,omitempty"`
	Path               string          `json:"path"`
	CreatedAt          time.Time       `json:"created_at"`
	ExistingBranchName string          `json:"existing_branch_name,omitempty"`
	Worktree           GitWorktreeData `json:"worktree,omitempty"`
}

// ToTopicData converts a Topic to its serializable form.
func (t *Topic) ToTopicData() TopicData {
	data := TopicData{
		Name:               t.Name,
		SharedWorktree:     t.SharedWorktree,
		AutoYes:            t.AutoYes,
		Branch:             t.Branch,
		Path:               t.Path,
		CreatedAt:          t.CreatedAt,
		ExistingBranchName: t.ExistingBranchName,
	}
	if t.gitWorktree != nil {
		data.Worktree = GitWorktreeData{
			RepoPath:        t.gitWorktree.GetRepoPath(),
			WorktreePath:    t.gitWorktree.GetWorktreePath(),
			SessionName:     t.Name,
			BranchName:      t.gitWorktree.GetBranchName(),
			BaseCommitSHA:   t.gitWorktree.GetBaseCommitSHA(),
			UnmanagedBranch: !t.gitWorktree.IsManagedBranch(),
		}
	}
	return data
}

// FromTopicData creates a Topic from serialized data.
func FromTopicData(data TopicData) *Topic {
	topic := &Topic{
		Name:               data.Name,
		SharedWorktree:     data.SharedWorktree,
		AutoYes:            data.AutoYes,
		Branch:             data.Branch,
		Path:               data.Path,
		CreatedAt:          data.CreatedAt,
		ExistingBranchName: data.ExistingBranchName,
		started:            true,
	}
	if data.SharedWorktree && data.Worktree.WorktreePath != "" {
		topic.gitWorktree = git.NewGitWorktreeFromStorage(
			data.Worktree.RepoPath,
			data.Worktree.WorktreePath,
			data.Worktree.SessionName,
			data.Worktree.BranchName,
			data.Worktree.BaseCommitSHA,
			!data.Worktree.UnmanagedBranch,
		)
	}
	return topic
}
