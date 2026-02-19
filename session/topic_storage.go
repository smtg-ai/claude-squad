package session

import (
	"claude-squad/session/git"
	"time"
)

// TopicData represents the serializable data of a Topic.
type TopicData struct {
	Name           string          `json:"name"`
	SharedWorktree bool            `json:"shared_worktree"`
	Branch         string          `json:"branch,omitempty"`
	Path           string          `json:"path"`
	CreatedAt      time.Time       `json:"created_at"`
	Worktree       GitWorktreeData `json:"worktree,omitempty"`
}

// ToTopicData converts a Topic to its serializable form.
func (t *Topic) ToTopicData() TopicData {
	data := TopicData{
		Name:           t.Name,
		SharedWorktree: t.SharedWorktree,
		Branch:         t.Branch,
		Path:           t.Path,
		CreatedAt:      t.CreatedAt,
	}
	if t.gitWorktree != nil {
		data.Worktree = GitWorktreeData{
			RepoPath:      t.gitWorktree.GetRepoPath(),
			WorktreePath:  t.gitWorktree.GetWorktreePath(),
			SessionName:   t.Name,
			BranchName:    t.gitWorktree.GetBranchName(),
			BaseCommitSHA: t.gitWorktree.GetBaseCommitSHA(),
		}
	}
	return data
}

// FromTopicData creates a Topic from serialized data.
func FromTopicData(data TopicData) *Topic {
	topic := &Topic{
		Name:           data.Name,
		SharedWorktree: data.SharedWorktree,
		Branch:         data.Branch,
		Path:           data.Path,
		CreatedAt:      data.CreatedAt,
		started:        true,
	}
	if data.SharedWorktree && data.Worktree.WorktreePath != "" {
		topic.gitWorktree = git.NewGitWorktreeFromStorage(
			data.Worktree.RepoPath,
			data.Worktree.WorktreePath,
			data.Worktree.SessionName,
			data.Worktree.BranchName,
			data.Worktree.BaseCommitSHA,
		)
	}
	return topic
}
