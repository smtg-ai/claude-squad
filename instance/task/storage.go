package task

import (
	"claude-squad/instance/task/git"
	"claude-squad/instance/task/tmux"
	"claude-squad/log"
	"encoding/json"
	"time"
)

// TaskData represents the serializable data of an Instance
type TaskData struct {
	Title     string    `json:"title"`
	Path      string    `json:"path"`
	Branch    string    `json:"branch"`
	Status    Status    `json:"status"`
	Height    int       `json:"height"`
	Width     int       `json:"width"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	AutoYes   bool      `json:"auto_yes"`

	Program   string          `json:"program"`
	Worktree  GitWorktreeData `json:"worktree"`
	DiffStats DiffStatsData   `json:"diff_stats"`
}

// ToInstanceData converts an Instance to its serializable form
func (i *Task) ToInstanceData() TaskData {
	data := TaskData{
		Title:     i.Title,
		Path:      i.Path,
		Branch:    i.Branch,
		Status:    i.Status,
		Height:    i.Height,
		Width:     i.Width,
		CreatedAt: i.CreatedAt,
		UpdatedAt: time.Now(),
		Program:   i.Program,
		AutoYes:   i.AutoYes,
	}

	// Only include worktree data if gitWorktree is initialized
	if i.gitWorktree != nil {
		data.Worktree = GitWorktreeData{
			RepoPath:      i.gitWorktree.GetRepoPath(),
			WorktreePath:  i.gitWorktree.GetWorktreePath(),
			SessionName:   i.Title,
			BranchName:    i.gitWorktree.GetBranchName(),
			BaseCommitSHA: i.gitWorktree.GetBaseCommitSHA(),
		}
	}

	// Only include diff stats if they exist
	if i.DiffStats != nil {
		data.DiffStats = DiffStatsData{
			Added:   i.DiffStats.Added,
			Removed: i.DiffStats.Removed,
			Content: i.DiffStats.Content,
		}
	}

	return data
}

// FromInstanceData creates a new Instance from serialized data
func FromInstanceData(data TaskData) (*Task, error) {
	instance := &Task{
		Title:     data.Title,
		Path:      data.Path,
		Branch:    data.Branch,
		Status:    data.Status,
		Height:    data.Height,
		Width:     data.Width,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
		Program:   data.Program,
		AutoYes:   data.AutoYes,
		gitWorktree: git.NewGitWorktreeFromStorage(
			data.Worktree.RepoPath,
			data.Worktree.WorktreePath,
			data.Worktree.SessionName,
			data.Worktree.BranchName,
			data.Worktree.BaseCommitSHA,
		),
		DiffStats: &git.DiffStats{
			Added:   data.DiffStats.Added,
			Removed: data.DiffStats.Removed,
			Content: data.DiffStats.Content,
		},
	}

	// Set started flag based on whether we have valid worktree data
	// This ensures GetGitWorktree() works even if tmux session isn't running
	if data.Worktree.RepoPath != "" && data.Worktree.BranchName != "" {
		instance.started = true
	}

	// Always create tmux session object first
	instance.tmuxSession = tmux.NewTmuxSession(instance.Title, instance.Program)

	if instance.Paused() {
		// For paused instances, try to restore the session if it exists, but don't fail if it doesn't
		if instance.tmuxSession.DoesSessionExist() {
			if err := instance.tmuxSession.Restore(); err != nil {
				// Log the error but don't fail - the session will be recreated when resumed
				// This prevents the app from failing to start if there are orphaned sessions
				// that can't be restored properly
				log.ErrorLog.Printf("failed to restore tmux session for paused task '%s': %v", instance.Title, err)
			}
		}
	} else {
		// For running instances, start the task which will handle session restoration
		if err := instance.Start(false); err != nil {
			return nil, err
		}
	}

	return instance, nil
}

// GitWorktreeData represents the serializable data of a GitWorktree
type GitWorktreeData struct {
	RepoPath      string `json:"repo_path"`
	WorktreePath  string `json:"worktree_path"`
	SessionName   string `json:"session_name"`
	BranchName    string `json:"branch_name"`
	BaseCommitSHA string `json:"base_commit_sha"`
}

// DiffStatsData represents the serializable data of a DiffStats
type DiffStatsData struct {
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
	Content string `json:"content"`
}

// Serialize returns the JSON encoding of the task's serializable state
func (t *Task) Serialize() []byte {
	data, err := json.Marshal(t.ToInstanceData())
	if err != nil {
		return nil
	}
	return data
}

// Deserialize populates the task from JSON
func (t *Task) Deserialize(data []byte) error {
	var td TaskData
	if err := json.Unmarshal(data, &td); err != nil {
		return err
	}
	newTask, err := FromInstanceData(td)
	if err != nil {
		return err
	}
	t.Title = newTask.Title
	t.Path = newTask.Path
	t.Branch = newTask.Branch
	t.Status = newTask.Status
	t.Program = newTask.Program
	t.Height = newTask.Height
	t.Width = newTask.Width
	t.CreatedAt = newTask.CreatedAt
	t.UpdatedAt = newTask.UpdatedAt
	t.AutoYes = newTask.AutoYes
	t.DiffStats = newTask.DiffStats
	return nil
}
