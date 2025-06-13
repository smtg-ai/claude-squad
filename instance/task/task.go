package task

import (
	"claude-squad/instance/task/git"
	"claude-squad/instance/task/tmux"
	"claude-squad/keys"
	"claude-squad/log"
	"path/filepath"

	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/atotto/clipboard"
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

// Task is a running instance of claude code.
type Task struct {
	// Subscribers to completion signal
	completionSubscribers []chan struct{}
	completionMu          sync.Mutex
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
	// Prompt is the initial prompt to pass to the instance on startup
	Prompt string

	// DiffStats stores the current git diff statistics
	diffStats *git.DiffStats

	// The below fields are initialized upon calling Start().

	started bool
	// tmuxSession is the tmux session for the instance.
	tmuxSession *tmux.TmuxSession
	// gitWorktree is the git worktree for the instance.
	gitWorktree *git.GitWorktree
}

// Options for creating a new instance
type TaskOptions struct {
	// Title is the title of the instance.
	Title string
	// Path is the path to the workspace.
	Path string
	// Program is the program to run in the instance (e.g. "claude", "aider --model ollama_chat/gemma3:1b").
	Program string
	// AutoYes is true if the instance should automatically press enter when prompted.
	AutoYes bool

	// resetCompletionSubscribers is a function that resets the completion subscribers.
	resetCompletionSubscribers func()
}

func NewTask(opts TaskOptions) (*Task, error) {
	t := time.Now()

	// Convert path to absolute
	absPath, err := filepath.Abs(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	return &Task{
		Title:     opts.Title,
		Status:    Ready,
		Path:      absPath,
		Program:   opts.Program,
		Height:    0,
		Width:     0,
		CreatedAt: t,
		UpdatedAt: t,
		AutoYes:   false,
	}, nil
}

func (t *Task) RepoName() (string, error) {
	if t.gitWorktree == nil {
		return "", fmt.Errorf("git worktree not initialized")
	}

	return t.gitWorktree.GetRepoName(), nil
}

func (t *Task) SetStatus(status Status) {
	t.Status = status
}

// MenuItems returns the available menu keys for a Task.
func (t *Task) MenuItems() []keys.KeyName {
	return nil // Or customize as needed
}

// StatusText returns the status of the instance as a string.
func (t *Task) StatusText() string {
	return t.Title
}

// firstTimeSetup is true if this is a new instance. Otherwise, it's one loaded from storage.
func (t *Task) Start(firstTimeSetup bool) error {
	if t.Title == "" {
		return fmt.Errorf("instance title cannot be empty")
	}

	tmuxSession := tmux.NewTmuxSession(t.Title, t.Program)
	tmuxSession.OnUserInput = t.resetCompletionSubscribers

	t.tmuxSession = tmuxSession

	if firstTimeSetup {
		gitWorktree, branchName, err := git.NewGitWorktree(t.Path, t.Title)
		if err != nil {
			return fmt.Errorf("failed to create git worktree: %w", err)
		}
		t.gitWorktree = gitWorktree
		t.Branch = branchName
	}

	// Setup error handler to cleanup resources on any error
	var setupErr error
	defer func() {
		if setupErr != nil {
			if cleanupErr := t.Kill(); cleanupErr != nil {
				setupErr = fmt.Errorf("%v (cleanup error: %v)", setupErr, cleanupErr)
			}
		} else {
			t.started = true
		}
	}()

	if !firstTimeSetup {
		// Reuse existing session
		if err := tmuxSession.Restore(); err != nil {
			setupErr = fmt.Errorf("failed to restore existing session: %w", err)
			return setupErr
		}
	} else {
		// Setup git worktree first
		if err := t.gitWorktree.Setup(); err != nil {
			setupErr = fmt.Errorf("failed to setup git worktree: %w", err)
			return setupErr
		}

		// Create new session
		if err := t.tmuxSession.Start(t.gitWorktree.GetWorktreePath()); err != nil {
			// Cleanup git worktree if tmux session creation fails
			if cleanupErr := t.gitWorktree.Cleanup(); cleanupErr != nil {
				err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
			}
			setupErr = fmt.Errorf("failed to start new session: %w", err)
			return setupErr
		}
	}

	t.SetStatus(Running)

	return nil
}

// Kill terminates the instance and cleans up all resources
func (t *Task) Kill() error {
	if !t.started {
		// If instance was never started, just return success
		return nil
	}

	var errs []error

	// Always try to cleanup both resources, even if one fails
	// Clean up tmux session first since it's using the git worktree
	if t.tmuxSession != nil {
		if err := t.tmuxSession.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close tmux session: %w", err))
		}
	}

	// Then clean up git worktree
	if t.gitWorktree != nil {
		if err := t.gitWorktree.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("failed to cleanup git worktree: %w", err))
		}
	}

	return t.combineErrors(errs)
}

// combineErrors combines multiple errors into a single error
func (t *Task) combineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}

	errMsg := "multiple cleanup errors occurred:"
	for _, err := range errs {
		errMsg += "\n  - " + err.Error()
	}
	return fmt.Errorf("%s", errMsg)
}

// Close is an alias for Kill to maintain backward compatibility
func (t *Task) Close() error {
	if !t.started {
		return fmt.Errorf("cannot close instance that has not been started")
	}
	return t.Kill()
}

func (t *Task) Preview() (string, error) {
	if !t.started || t.Status == Paused {
		return "", nil
	}
	return t.tmuxSession.CapturePaneContent(false)
}

func (t *Task) FullOutput() (string, error) {
	if !t.started || t.Status == Paused {
		return "", nil
	}
	return t.tmuxSession.CapturePaneContent(true)
}

func (t *Task) HasUpdated() (updated bool, hasPrompt bool) {
	if !t.started {
		return false, false
	}
	return t.tmuxSession.HasUpdated()
}

// TapEnter sends an enter key press to the tmux session if AutoYes is enabled.
func (t *Task) TapEnter() {
	if !t.started || !t.AutoYes {
		return
	}
	if err := t.tmuxSession.TapEnter(); err != nil {
		log.ErrorLog.Printf("error tapping enter: %v", err)
	}
}

func (t *Task) Attach() (chan struct{}, error) {
	if !t.started {
		return nil, fmt.Errorf("cannot attach instance that has not been started")
	}
	return t.tmuxSession.Attach()
}

func (t *Task) SetPreviewSize(width, height int) error {
	if !t.started || t.Status == Paused {
		return fmt.Errorf("cannot set preview size for instance that has not been started or " +
			"is paused")
	}
	return t.tmuxSession.SetDetachedSize(width, height)
}

// GetGitWorktree returns the git worktree for the instance
func (t *Task) GetGitWorktree() (*git.GitWorktree, error) {
	if !t.started {
		return nil, fmt.Errorf("cannot get git worktree for instance that has not been started")
	}
	return t.gitWorktree, nil
}

func (t *Task) Started() bool {
	return t.started
}

// SetTitle sets the title of the instance. Returns an error if the instance has started.
// We cant change the title once it's been used for a tmux session etc.
func (t *Task) SetTitle(title string) error {
	if t.started {
		return fmt.Errorf("cannot change title of a started instance")
	}
	t.Title = title
	return nil
}

func (t *Task) Paused() bool {
	return t.Status == Paused
}

// TmuxAlive returns true if the tmux session is alive. This is a sanity check before attaching.
func (t *Task) TmuxAlive() bool {
	return t.tmuxSession.DoesSessionExist()
}

// Pause stops the tmux session and removes the worktree, preserving the branch
func (t *Task) Pause() error {
	if !t.started {
		return fmt.Errorf("cannot pause instance that has not been started")
	}
	if t.Status == Paused {
		return fmt.Errorf("instance is already paused")
	}

	var errs []error

	// Check if there are any changes to commit
	if dirty, err := t.gitWorktree.IsDirty(); err != nil {
		errs = append(errs, fmt.Errorf("failed to check if worktree is dirty: %w", err))
		log.ErrorLog.Print(err)
	} else if dirty {
		// Commit changes with timestamp
		commitMsg := fmt.Sprintf("[claudesquad] update from '%s' on %s (paused)", t.Title, time.Now().Format(time.RFC822))
		if err := t.gitWorktree.PushChanges(commitMsg, false); err != nil {
			errs = append(errs, fmt.Errorf("failed to commit changes: %w", err))
			log.ErrorLog.Print(err)
			// Return early if we can't commit changes to avoid corrupted state
			return t.combineErrors(errs)
		}
	}

	// Close tmux session first since it's using the git worktree
	if err := t.tmuxSession.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close tmux session: %w", err))
		log.ErrorLog.Print(err)
		// Return early if we can't close tmux to avoid corrupted state
		return t.combineErrors(errs)
	}

	// Check if worktree exists before trying to remove it
	if _, err := os.Stat(t.gitWorktree.GetWorktreePath()); err == nil {
		// Remove worktree but keep branch
		if err := t.gitWorktree.Remove(); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove git worktree: %w", err))
			log.ErrorLog.Print(err)
			return t.combineErrors(errs)
		}

		// Only prune if remove was successful
		if err := t.gitWorktree.Prune(); err != nil {
			errs = append(errs, fmt.Errorf("failed to prune git worktrees: %w", err))
			log.ErrorLog.Print(err)
			return t.combineErrors(errs)
		}
	}

	if err := t.combineErrors(errs); err != nil {
		log.ErrorLog.Print(err)
		return err
	}

	t.SetStatus(Paused)
	_ = clipboard.WriteAll(t.gitWorktree.GetBranchName())
	return nil
}

// Resume recreates the worktree and restarts the tmux session
func (t *Task) Resume() error {
	if !t.started {
		return fmt.Errorf("cannot resume instance that has not been started")
	}
	if t.Status != Paused {
		return fmt.Errorf("can only resume paused instances")
	}

	// Check if branch is checked out
	if checked, err := t.gitWorktree.IsBranchCheckedOut(); err != nil {
		log.ErrorLog.Print(err)
		return fmt.Errorf("failed to check if branch is checked out: %w", err)
	} else if checked {
		return fmt.Errorf("cannot resume: branch is checked out, please switch to a different branch")
	}

	// Setup git worktree
	if err := t.gitWorktree.Setup(); err != nil {
		log.ErrorLog.Print(err)
		return fmt.Errorf("failed to setup git worktree: %w", err)
	}

	// Create new tmux session
	if err := t.tmuxSession.Start(t.gitWorktree.GetWorktreePath()); err != nil {
		log.ErrorLog.Print(err)
		// Cleanup git worktree if tmux session creation fails
		if cleanupErr := t.gitWorktree.Cleanup(); cleanupErr != nil {
			err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
			log.ErrorLog.Print(err)
		}
		return fmt.Errorf("failed to start new session: %w", err)
	}

	t.SetStatus(Running)
	return nil
}

// UpdateDiffStats updates the git diff statistics for this instance
func (t *Task) UpdateDiffStats() error {
	if !t.started {
		t.diffStats = nil
		return nil
	}

	if t.Status == Paused {
		// Keep the previous diff stats if the instance is paused
		return nil
	}

	stats := t.gitWorktree.Diff()
	if stats.Error != nil {
		if strings.Contains(stats.Error.Error(), "base commit SHA not set") {
			// Worktree is not fully set up yet, not an error
			t.diffStats = nil
			return nil
		}
		return fmt.Errorf("failed to get diff stats: %w", stats.Error)
	}

	t.diffStats = stats
	return nil
}

// GetDiffStats returns the current git diff statistics
func (t *Task) GetDiffStats() *git.DiffStats {
	return t.diffStats
}

// SendPrompt sends a prompt to the tmux session
func (t *Task) SendPrompt(prompt string) error {
	t.resetCompletionSubscribers()
	if !t.started {
		return fmt.Errorf("instance not started")
	}
	if t.tmuxSession == nil {
		return fmt.Errorf("tmux session not initialized")
	}
	if err := t.tmuxSession.SendKeys(prompt); err != nil {
		return fmt.Errorf("error sending keys to tmux session: %w", err)
	}

	// Brief pause to prevent carriage return from being interpreted as newline
	time.Sleep(100 * time.Millisecond)
	if err := t.tmuxSession.TapEnter(); err != nil {
		return fmt.Errorf("error tapping enter: %w", err)
	}

	return nil
}

func (t *Task) WaitForCompletion() error {
	if !t.started {
		return fmt.Errorf("instance not started")
	}
	if t.tmuxSession == nil {
		return fmt.Errorf("tmux session not initialized")
	}

	ch := t.subscribeCompletion()

	select {
	case <-ch:
		return nil
	case <-time.After(30 * time.Minute): // Timeout after 30 minutes
		return fmt.Errorf("timed out waiting for instance to complete")
	}
}

// subscribeCompletion returns a channel that will be closed when the instance completes.
func (t *Task) subscribeCompletion() <-chan struct{} {
	ch := make(chan struct{})
	t.completionMu.Lock()
	t.completionSubscribers = append(t.completionSubscribers, ch)
	t.completionMu.Unlock()
	return ch
}

// signalCompletionSubscribers closes all subscriber channels and resets the list.
func (t *Task) signalCompletionSubscribers() {
	t.completionMu.Lock()
	defer t.completionMu.Unlock()
	for _, ch := range t.completionSubscribers {
		close(ch)
	}
	t.completionSubscribers = nil
}

// resetCompletionSubscribers closes all subscriber channels and resets the list.
func (t *Task) resetCompletionSubscribers() {
	t.completionMu.Lock()
	defer t.completionMu.Unlock()
	for _, ch := range t.completionSubscribers {
		select {
		case <-ch:
			// already closed
		default:
			close(ch)
		}
	}
	t.completionSubscribers = nil
}

func (t *Task) IsRunning() bool {
	return t.started
}
