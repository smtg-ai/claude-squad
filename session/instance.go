package session

import (
	"claude-squad/log"
	"claude-squad/session/git"
	"claude-squad/session/tmux"
	"path/filepath"

	"fmt"
	"os"
	"strings"
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
	// Prompt is the initial prompt to pass to the instance on startup
	Prompt string
	// Imported is true if this session was imported from an existing tmux session
	// not originally managed by Claude Squad.
	Imported bool

	// DiffStats stores the current git diff statistics
	diffStats *git.DiffStats

	// selectedBranch is the existing branch to start on (empty = new branch from HEAD)
	selectedBranch string

	// previewDirty indicates that the instance content has changed and
	// the preview pane needs to be refreshed on the next tick.
	previewDirty bool

	// loadingStatus holds a progress message shown in the preview pane during startup.
	loadingStatus string

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
		Imported:  i.Imported,
	}

	// Only include worktree data if gitWorktree is initialized
	if i.gitWorktree != nil {
		data.Worktree = GitWorktreeData{
			RepoPath:         i.gitWorktree.GetRepoPath(),
			WorktreePath:     i.gitWorktree.GetWorktreePath(),
			SessionName:      i.Title,
			BranchName:       i.gitWorktree.GetBranchName(),
			BaseCommitSHA:    i.gitWorktree.GetBaseCommitSHA(),
			IsExistingBranch: i.gitWorktree.IsExistingBranch(),
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
	if data.Imported {
		// Imported sessions: reconnect to the existing tmux session
		instance, err := NewImportedInstance(data.Title, data.Title, data.Path, data.Program)
		if err != nil {
			return nil, fmt.Errorf("failed to restore imported session: %w", err)
		}
		instance.CreatedAt = data.CreatedAt
		instance.UpdatedAt = data.UpdatedAt
		return instance, nil
	}

	instance := &Instance{
		Title:     data.Title,
		Path:      data.Path,
		Branch:    data.Branch,
		Status:    data.Status,
		Height:    data.Height,
		Width:     data.Width,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
		Program:   data.Program,
		gitWorktree: git.NewGitWorktreeFromStorage(
			data.Worktree.RepoPath,
			data.Worktree.WorktreePath,
			data.Worktree.SessionName,
			data.Worktree.BranchName,
			data.Worktree.BaseCommitSHA,
			data.Worktree.IsExistingBranch,
		),
		diffStats: &git.DiffStats{
			Added:   data.DiffStats.Added,
			Removed: data.DiffStats.Removed,
			Content: data.DiffStats.Content,
		},
	}

	if instance.Paused() {
		instance.started = true
		instance.tmuxSession = tmux.NewTmuxSession(instance.Title, instance.Program)
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
	// Branch is an existing branch name to start the session on (empty = new branch from HEAD)
	Branch string
}

// NewImportedInstance creates an Instance that wraps an existing tmux session
// not originally managed by Claude Squad. The session is used as-is; no git
// worktree is created.
func NewImportedInstance(title string, tmuxSessionName string, workDir string, program string) (*Instance, error) {
	t := time.Now()
	absPath, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	instance := &Instance{
		Title:     title,
		Status:    Running,
		Path:      absPath,
		Program:   program,
		CreatedAt: t,
		UpdatedAt: t,
		Imported:  true,
		started:   true,
	}

	// Create a TmuxSession that wraps the existing session
	ts := tmux.NewTmuxSessionFromExisting(tmuxSessionName, program)
	if err := ts.Restore(); err != nil {
		return nil, fmt.Errorf("failed to connect to tmux session: %w", err)
	}
	instance.tmuxSession = ts

	return instance, nil
}

func NewInstance(opts InstanceOptions) (*Instance, error) {
	t := time.Now()

	// Convert path to absolute
	absPath, err := filepath.Abs(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	return &Instance{
		Title:          opts.Title,
		Status:         Ready,
		Path:           absPath,
		Program:        opts.Program,
		Height:         0,
		Width:          0,
		CreatedAt:      t,
		UpdatedAt:      t,
		AutoYes:        false,
		selectedBranch: opts.Branch,
	}, nil
}

func (i *Instance) RepoName() (string, error) {
	if !i.started {
		return "", fmt.Errorf("cannot get repo name for instance that has not been started")
	}
	if i.gitWorktree == nil {
		return "", fmt.Errorf("no git worktree for instance")
	}
	return i.gitWorktree.GetRepoName(), nil
}

func (i *Instance) SetStatus(status Status) {
	i.Status = status
}

// MarkPreviewDirty marks the instance as needing a preview refresh.
func (i *Instance) MarkPreviewDirty() { i.previewDirty = true }

// ClearPreviewDirty clears the dirty flag after preview has been captured.
func (i *Instance) ClearPreviewDirty() { i.previewDirty = false }

// IsPreviewDirty returns whether the instance needs a preview refresh.
func (i *Instance) IsPreviewDirty() bool { return i.previewDirty }

// GetLoadingStatus returns the current loading progress message.
func (i *Instance) GetLoadingStatus() string { return i.loadingStatus }

// setLoadingStatus updates the loading progress message.
func (i *Instance) setLoadingStatus(msg string) { i.loadingStatus = msg }

// SetSelectedBranch sets the branch to use when starting the instance.
func (i *Instance) SetSelectedBranch(branch string) {
	i.selectedBranch = branch
}

// firstTimeSetup is true if this is a new instance. Otherwise, it's one loaded from storage.
func (i *Instance) Start(firstTimeSetup bool) error {
	if i.Title == "" {
		return fmt.Errorf("instance title cannot be empty")
	}

	var tmuxSession *tmux.TmuxSession
	if i.tmuxSession != nil {
		// Use existing tmux session (useful for testing)
		tmuxSession = i.tmuxSession
	} else {
		// Create new tmux session
		tmuxSession = tmux.NewTmuxSession(i.Title, i.Program)
	}
	i.tmuxSession = tmuxSession

	i.setLoadingStatus("Preparing workspace for '" + i.Title + "'...")

	if firstTimeSetup {
		if i.selectedBranch != "" {
			i.setLoadingStatus("Checking out branch '" + i.selectedBranch + "'...")
			gitWorktree, err := git.NewGitWorktreeFromBranch(i.Path, i.selectedBranch, i.Title)
			if err != nil {
				return fmt.Errorf("failed to create git worktree from branch: %w", err)
			}
			i.gitWorktree = gitWorktree
			i.Branch = i.selectedBranch
		} else {
			i.setLoadingStatus("Creating a fresh branch...")
			gitWorktree, branchName, err := git.NewGitWorktree(i.Path, i.Title)
			if err != nil {
				return fmt.Errorf("failed to create git worktree: %w", err)
			}
			i.gitWorktree = gitWorktree
			i.Branch = branchName
		}
	}

	// Setup error handler to cleanup resources on any error
	var setupErr error
	defer func() {
		if setupErr != nil {
			i.setLoadingStatus("Something went wrong, cleaning up...")
			if cleanupErr := i.Kill(); cleanupErr != nil {
				setupErr = fmt.Errorf("%v (cleanup error: %v)", setupErr, cleanupErr)
			}
		} else {
			i.started = true
		}
	}()

	if !firstTimeSetup {
		i.setLoadingStatus("Reconnecting to existing session...")
		// Reuse existing session
		if err := tmuxSession.Restore(); err != nil {
			setupErr = fmt.Errorf("failed to restore existing session: %w", err)
			return setupErr
		}
	} else {
		// Setup git worktree first
		i.setLoadingStatus("Setting up git worktree (isolating your workspace)...")
		if err := i.gitWorktree.Setup(); err != nil {
			setupErr = fmt.Errorf("failed to setup git worktree: %w", err)
			return setupErr
		}

		i.setLoadingStatus("Spinning up tmux session (almost there!)...")
		// Create new session
		if err := i.tmuxSession.Start(i.gitWorktree.GetWorktreePath()); err != nil {
			// Cleanup git worktree if tmux session creation fails
			if cleanupErr := i.gitWorktree.Cleanup(); cleanupErr != nil {
				err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
			}
			setupErr = fmt.Errorf("failed to start new session: %w", err)
			return setupErr
		}
	}

	i.setLoadingStatus("Ready! Handing off to your agent...")
	i.SetStatus(Running)

	return nil
}

// Kill terminates the instance and cleans up all resources
func (i *Instance) Kill() error {
	if !i.started {
		// If instance was never started, just return success
		return nil
	}

	var errs []error

	if i.tmuxSession != nil {
		if i.Imported {
			// For imported sessions, just close the PTY -- don't kill the tmux session
			if err := i.tmuxSession.ClosePty(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close pty for imported session: %w", err))
			}
		} else {
			// Always try to cleanup both resources, even if one fails
			// Clean up tmux session first since it's using the git worktree
			if err := i.tmuxSession.Close(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close tmux session: %w", err))
			}
		}
	}

	// Only clean up worktree for non-imported sessions
	if !i.Imported && i.gitWorktree != nil {
		if err := i.gitWorktree.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("failed to cleanup git worktree: %w", err))
		}
	}

	return i.combineErrors(errs)
}

// combineErrors combines multiple errors into a single error
func (i *Instance) combineErrors(errs []error) error {
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

func (i *Instance) Preview() (string, error) {
	if !i.started || i.Status == Paused {
		return "", nil
	}
	return i.tmuxSession.CapturePaneContent()
}

func (i *Instance) HasUpdated() (updated bool, hasPrompt bool) {
	if !i.started {
		return false, false
	}
	return i.tmuxSession.HasUpdated()
}

// CheckAndHandleTrustPrompt checks for and dismisses the trust prompt for supported programs.
func (i *Instance) CheckAndHandleTrustPrompt() bool {
	if !i.started || i.tmuxSession == nil {
		return false
	}
	program := i.Program
	if !strings.HasSuffix(program, tmux.ProgramClaude) &&
		!strings.HasSuffix(program, tmux.ProgramAider) &&
		!strings.HasSuffix(program, tmux.ProgramGemini) {
		return false
	}
	return i.tmuxSession.CheckAndHandleTrustPrompt()
}

// TapEnter sends an enter key press to the tmux session if AutoYes is enabled.
func (i *Instance) TapEnter() {
	if !i.started || !i.AutoYes {
		return
	}
	if err := i.tmuxSession.TapEnter(); err != nil {
		log.ErrorLog.Printf("error tapping enter: %v", err)
	}
}

func (i *Instance) Attach() (chan struct{}, error) {
	if !i.started {
		return nil, fmt.Errorf("cannot attach instance that has not been started")
	}
	return i.tmuxSession.Attach()
}

func (i *Instance) SetPreviewSize(width, height int) error {
	if !i.started || i.Status == Paused {
		return fmt.Errorf("cannot set preview size for instance that has not been started or " +
			"is paused")
	}
	return i.tmuxSession.SetDetachedSize(width, height)
}

// GetGitWorktree returns the git worktree for the instance
func (i *Instance) GetGitWorktree() (*git.GitWorktree, error) {
	if !i.started {
		return nil, fmt.Errorf("cannot get git worktree for instance that has not been started")
	}
	return i.gitWorktree, nil
}

// GetWorktreePath returns the worktree path for the instance, or empty string if unavailable
func (i *Instance) GetWorktreePath() string {
	if i.gitWorktree == nil {
		return ""
	}
	return i.gitWorktree.GetWorktreePath()
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

// Pause stops the tmux session and removes the worktree, preserving the branch
func (i *Instance) Pause() error {
	if !i.started {
		return fmt.Errorf("cannot pause instance that has not been started")
	}
	if i.Status == Paused {
		return fmt.Errorf("instance is already paused")
	}
	if i.Imported {
		return fmt.Errorf("cannot pause imported sessions (no managed worktree)")
	}

	var errs []error

	// Check if there are any changes to commit
	if dirty, err := i.gitWorktree.IsDirty(); err != nil {
		errs = append(errs, fmt.Errorf("failed to check if worktree is dirty: %w", err))
		log.ErrorLog.Print(err)
	} else if dirty {
		// Commit changes locally (without pushing to GitHub)
		commitMsg := fmt.Sprintf("[claudesquad] update from '%s' on %s (paused)", i.Title, time.Now().Format(time.RFC822))
		if err := i.gitWorktree.CommitChanges(commitMsg); err != nil {
			errs = append(errs, fmt.Errorf("failed to commit changes: %w", err))
			log.ErrorLog.Print(err)
			// Return early if we can't commit changes to avoid corrupted state
			return i.combineErrors(errs)
		}
	}

	// Detach from tmux session instead of closing to preserve session output
	if err := i.tmuxSession.DetachSafely(); err != nil {
		errs = append(errs, fmt.Errorf("failed to detach tmux session: %w", err))
		log.ErrorLog.Print(err)
		// Continue with pause process even if detach fails
	}

	// Check if worktree exists before trying to remove it
	if _, err := os.Stat(i.gitWorktree.GetWorktreePath()); err == nil {
		// Remove worktree but keep branch
		if err := i.gitWorktree.Remove(); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove git worktree: %w", err))
			log.ErrorLog.Print(err)
			return i.combineErrors(errs)
		}

		// Only prune if remove was successful
		if err := i.gitWorktree.Prune(); err != nil {
			errs = append(errs, fmt.Errorf("failed to prune git worktrees: %w", err))
			log.ErrorLog.Print(err)
			return i.combineErrors(errs)
		}
	}

	if err := i.combineErrors(errs); err != nil {
		log.ErrorLog.Print(err)
		return err
	}

	i.SetStatus(Paused)
	_ = clipboard.WriteAll(i.gitWorktree.GetBranchName())
	return nil
}

// Resume recreates the worktree and restarts the tmux session
func (i *Instance) Resume() error {
	if !i.started {
		return fmt.Errorf("cannot resume instance that has not been started")
	}
	if i.Status != Paused {
		return fmt.Errorf("can only resume paused instances")
	}

	// Check if branch is checked out
	if checked, err := i.gitWorktree.IsBranchCheckedOut(); err != nil {
		log.ErrorLog.Print(err)
		return fmt.Errorf("failed to check if branch is checked out: %w", err)
	} else if checked {
		return fmt.Errorf("cannot resume: branch is checked out, please switch to a different branch")
	}

	// Setup git worktree
	if err := i.gitWorktree.Setup(); err != nil {
		log.ErrorLog.Print(err)
		return fmt.Errorf("failed to setup git worktree: %w", err)
	}

	// Check if tmux session still exists from pause, otherwise create new one
	if i.tmuxSession.DoesSessionExist() {
		// Session exists, just restore PTY connection to it
		if err := i.tmuxSession.Restore(); err != nil {
			log.ErrorLog.Print(err)
			// If restore fails, fall back to creating new session
			if err := i.tmuxSession.Start(i.gitWorktree.GetWorktreePath()); err != nil {
				log.ErrorLog.Print(err)
				// Cleanup git worktree if tmux session creation fails
				if cleanupErr := i.gitWorktree.Cleanup(); cleanupErr != nil {
					err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
					log.ErrorLog.Print(err)
				}
				return fmt.Errorf("failed to start new session: %w", err)
			}
		}
	} else {
		// Create new tmux session
		if err := i.tmuxSession.Start(i.gitWorktree.GetWorktreePath()); err != nil {
			log.ErrorLog.Print(err)
			// Cleanup git worktree if tmux session creation fails
			if cleanupErr := i.gitWorktree.Cleanup(); cleanupErr != nil {
				err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
				log.ErrorLog.Print(err)
			}
			return fmt.Errorf("failed to start new session: %w", err)
		}
	}

	i.SetStatus(Running)
	return nil
}

// UpdateDiffStats updates the git diff statistics for this instance
func (i *Instance) UpdateDiffStats() error {
	if !i.started {
		i.diffStats = nil
		return nil
	}

	if i.Status == Paused {
		// Keep the previous diff stats if the instance is paused
		return nil
	}

	if i.gitWorktree == nil {
		// Imported sessions have no worktree, so no diff
		i.diffStats = nil
		return nil
	}

	stats := i.gitWorktree.Diff()
	if stats.Error != nil {
		if strings.Contains(stats.Error.Error(), "base commit SHA not set") {
			// Worktree is not fully set up yet, not an error
			i.diffStats = nil
			return nil
		}
		return fmt.Errorf("failed to get diff stats: %w", stats.Error)
	}

	i.diffStats = stats
	return nil
}

// ComputeDiff runs the expensive git diff I/O and returns the result without
// mutating instance state. Safe to call from a background goroutine.
func (i *Instance) ComputeDiff() *git.DiffStats {
	if !i.started || i.Status == Paused || i.gitWorktree == nil {
		return nil
	}
	return i.gitWorktree.Diff()
}

// SetDiffStats sets the diff statistics on the instance. Should be called from
// the main event loop to avoid data races with View.
func (i *Instance) SetDiffStats(stats *git.DiffStats) {
	i.diffStats = stats
}

// GetDiffStats returns the current git diff statistics
func (i *Instance) GetDiffStats() *git.DiffStats {
	return i.diffStats
}

// SendPrompt sends a prompt to the tmux session.
// It types the prompt text and presses Enter. If retryEnter is true,
// it also retries Enter after a delay in case the first was absorbed
// by a trust/startup prompt.
func (i *Instance) SendPrompt(prompt string, retryEnter ...bool) error {
	if !i.started {
		return fmt.Errorf("instance not started")
	}
	if i.tmuxSession == nil {
		return fmt.Errorf("tmux session not initialized")
	}
	if err := i.tmuxSession.SendKeys(prompt); err != nil {
		return fmt.Errorf("error sending keys to tmux session: %w", err)
	}

	// Brief pause to prevent carriage return from being interpreted as newline
	time.Sleep(100 * time.Millisecond)
	if err := i.tmuxSession.TapEnter(); err != nil {
		return fmt.Errorf("error tapping enter: %w", err)
	}

	// If requested, retry Enter after a delay. This handles the case where
	// the agent's startup screen (trust prompt, etc.) absorbs the first Enter.
	if len(retryEnter) > 0 && retryEnter[0] {
		go func() {
			time.Sleep(3 * time.Second)
			_ = i.tmuxSession.TapEnter()
		}()
	}

	return nil
}


// PreviewFullHistory captures the entire tmux pane output including full scrollback history
func (i *Instance) PreviewFullHistory() (string, error) {
	if !i.started || i.Status == Paused {
		return "", nil
	}
	return i.tmuxSession.CapturePaneContentWithOptions("-", "-")
}

// SetTmuxSession sets the tmux session for testing purposes
func (i *Instance) SetTmuxSession(session *tmux.TmuxSession) {
	i.tmuxSession = session
}

// SendKeys sends keys to the tmux session
func (i *Instance) SendKeys(keys string) error {
	if !i.started || i.Status == Paused {
		return fmt.Errorf("cannot send keys to instance that has not been started or is paused")
	}
	return i.tmuxSession.SendKeys(keys)
}
