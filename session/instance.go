package session

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/ByteMirror/hivemind/session/git"
	"github.com/ByteMirror/hivemind/session/tmux"
)

type Status int

const (
	// Running is the status when the instance is running and claude is working.
	Running Status = iota
	// Ready is if the claude instance is ready to be interacted with (waiting for user input).
	Ready
	// Loading is if the instance is loading (if we are starting it up or something).
	Loading
	// Paused is if the instance is paused (worktree removed but branch preserved).
	Paused
)

// Instance is a running instance of claude code.
type Instance struct {
	// Title is the title of the instance.
	Title string
	// Path is the path to the workspace.
	Path string
	// Branch is the branch of the instance.
	Branch string
	// Status is the status of the instance.
	Status Status
	// Program is the program to run in the instance.
	Program string
	// Height is the height of the instance.
	Height int
	// Width is the width of the instance.
	Width int
	// CreatedAt is the time the instance was created.
	CreatedAt time.Time
	// UpdatedAt is the time the instance was last updated.
	UpdatedAt time.Time
	// AutoYes is true if the instance should automatically press enter when prompted.
	AutoYes bool
	// SkipPermissions is true if the instance should run Claude with --dangerously-skip-permissions.
	SkipPermissions bool
	// TopicName is the name of the topic this instance belongs to (empty = ungrouped).
	TopicName string

	// sharedWorktree is true if this instance uses a topic's shared worktree (should not clean it up).
	sharedWorktree bool
	// LoadingStage tracks the current startup progress. Exported so the UI can read it.
	LoadingStage int
	// LoadingTotal is the total number of startup stages.
	LoadingTotal int
	// LoadingMessage describes the current loading step.
	LoadingMessage string

	// Notified is true when the instance finished (Running→Ready) and the user
	// hasn't selected it yet. Cleared when the user selects this instance.
	Notified bool

	// LastActiveAt is set whenever the instance is marked as Running.
	LastActiveAt time.Time

	// PromptDetected is true when the instance's program is waiting for user input.
	// Reset to false when the instance resumes running. Used by the sidebar to
	// persistently show a running indicator without flickering.
	PromptDetected bool

	// CPUPercent is the current CPU usage of the instance's process.
	CPUPercent float64
	// MemMB is the current memory usage in megabytes.
	MemMB float64

	// LastActivity is the most recently detected agent activity (ephemeral, not persisted).
	LastActivity *Activity

	// DiffStats stores the current git diff statistics
	diffStats *git.DiffStats

	// The below fields are initialized upon calling Start().

	started bool
	// tmuxSession is the tmux session for the instance.
	tmuxSession *tmux.TmuxSession
	// gitWorktree is the git worktree for the instance.
	gitWorktree *git.GitWorktree
}

// ToInstanceData converts an Instance to its serializable form
func (i *Instance) ToInstanceData() InstanceData {
	data := InstanceData{
		Title:           i.Title,
		Path:            i.Path,
		Branch:          i.Branch,
		Status:          i.Status,
		Height:          i.Height,
		Width:           i.Width,
		CreatedAt:       i.CreatedAt,
		UpdatedAt:       time.Now(),
		Program:         i.Program,
		AutoYes:         i.AutoYes,
		SkipPermissions: i.SkipPermissions,
		TopicName:       i.TopicName,
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
	if i.diffStats != nil {
		data.DiffStats = DiffStatsData{
			Added:   i.diffStats.Added,
			Removed: i.diffStats.Removed,
			Content: i.diffStats.Content,
		}
	}

	return data
}

// FromInstanceData creates a new Instance from serialized data
func FromInstanceData(data InstanceData) (*Instance, error) {
	instance := &Instance{
		Title:           data.Title,
		Path:            data.Path,
		Branch:          data.Branch,
		Status:          data.Status,
		Height:          data.Height,
		Width:           data.Width,
		CreatedAt:       data.CreatedAt,
		UpdatedAt:       data.UpdatedAt,
		Program:         data.Program,
		SkipPermissions: data.SkipPermissions,
		TopicName:       data.TopicName,
		gitWorktree: git.NewGitWorktreeFromStorage(
			data.Worktree.RepoPath,
			data.Worktree.WorktreePath,
			data.Worktree.SessionName,
			data.Worktree.BranchName,
			data.Worktree.BaseCommitSHA,
		),
		diffStats: &git.DiffStats{
			Added:   data.DiffStats.Added,
			Removed: data.DiffStats.Removed,
			Content: data.DiffStats.Content,
		},
	}

	if instance.Paused() {
		instance.started = true
		instance.tmuxSession = tmux.NewTmuxSession(instance.Title, instance.Program, instance.SkipPermissions)
	} else {
		if err := instance.Start(false); err != nil {
			return nil, err
		}
	}

	return instance, nil
}

// Options for creating a new instance
type InstanceOptions struct {
	// Title is the title of the instance.
	Title string
	// Path is the path to the workspace.
	Path string
	// Program is the program to run in the instance (e.g. "claude", "aider --model ollama_chat/gemma3:1b")
	Program string
	// If AutoYes is true, then
	AutoYes bool
	// SkipPermissions enables --dangerously-skip-permissions for Claude instances.
	SkipPermissions bool
	// TopicName assigns this instance to a topic.
	TopicName string
}

func NewInstance(opts InstanceOptions) (*Instance, error) {
	t := time.Now()

	// Convert path to absolute
	absPath, err := filepath.Abs(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	return &Instance{
		Title:           opts.Title,
		Status:          Ready,
		Path:            absPath,
		Program:         opts.Program,
		Height:          0,
		Width:           0,
		CreatedAt:       t,
		UpdatedAt:       t,
		AutoYes:         opts.AutoYes,
		SkipPermissions: opts.SkipPermissions,
		TopicName:       opts.TopicName,
	}, nil
}

func (i *Instance) RepoName() (string, error) {
	if !i.started {
		return "", fmt.Errorf("cannot get repo name for instance that has not been started")
	}
	return i.gitWorktree.GetRepoName(), nil
}

// GetRepoPath returns the repo path for this instance, or empty string if not started.
func (i *Instance) GetRepoPath() string {
	if !i.started || i.gitWorktree == nil {
		return ""
	}
	return i.gitWorktree.GetRepoPath()
}

func (i *Instance) SetStatus(status Status) {
	if i.Status == Running && status == Ready {
		i.Notified = true
		SendNotification("Hivemind", fmt.Sprintf("'%s' has finished", i.Title))
	}
	if status == Running || status == Loading {
		i.LastActiveAt = time.Now()
		i.PromptDetected = false
		i.Notified = false
	}
	i.Status = status
}

func (i *Instance) setLoadingProgress(stage int, message string) {
	i.LoadingStage = stage
	i.LoadingMessage = message
}

func (i *Instance) Started() bool {
	return i.started
}

// SetTitle sets the title of the instance. Returns an error if the instance has started.
// We cant change the title once it's been used for a tmux session etc.
func (i *Instance) SetTitle(title string) error {
	if i.started {
		return fmt.Errorf("cannot change title of a started instance")
	}
	i.Title = title
	return nil
}

func (i *Instance) Paused() bool {
	return i.Status == Paused
}

// TmuxAlive returns true if the tmux session is alive. This is a sanity check before attaching.
func (i *Instance) TmuxAlive() bool {
	return i.tmuxSession.DoesSessionExist()
}
