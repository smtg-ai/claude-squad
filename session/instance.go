package session

import (
	"claude-squad/config"
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
	// ProjectID is the ID of the project this instance belongs to
	ProjectID string

	// DiffStats stores the current git diff statistics
	diffStats *git.DiffStats

	// The below fields are initialized upon calling Start().

	started bool
	// tmuxSession is the tmux session for the instance.
	tmuxSession *tmux.TmuxSession
	// gitWorktree is the git worktree for the instance.
	gitWorktree *git.GitWorktree
	// consoleTmuxSession is the tmux session for the console tab.
	consoleTmuxSession *tmux.TmuxSession
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
		ProjectID: i.ProjectID,
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
		Title:     data.Title,
		Path:      data.Path,
		Branch:    data.Branch,
		Status:    data.Status,
		Height:    data.Height,
		Width:     data.Width,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
		Program:   data.Program,
		ProjectID: data.ProjectID,
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
}

func NewInstance(opts InstanceOptions) (*Instance, error) {
	t := time.Now()

	// Convert path to absolute
	absPath, err := filepath.Abs(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	return &Instance{
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

func (i *Instance) RepoName() (string, error) {
	if !i.started {
		return "", fmt.Errorf("cannot get repo name for instance that has not been started")
	}
	return i.gitWorktree.GetRepoName(), nil
}

func (i *Instance) SetStatus(status Status) {
	i.Status = status
}

// firstTimeSetup is true if this is a new instance. Otherwise, it's one loaded from storage.
func (i *Instance) Start(firstTimeSetup bool) error {
	if i.Title == "" {
		return fmt.Errorf("instance title cannot be empty")
	}

	tmuxSession := tmux.NewTmuxSession(i.Title, i.Program)
	i.tmuxSession = tmuxSession

	// Create console session - use shell as program for console
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	consoleTmuxSession := tmux.NewTmuxSession(i.Title+"-console", shell+" -i")
	i.consoleTmuxSession = consoleTmuxSession

	if firstTimeSetup {
		gitWorktree, branchName, err := git.NewGitWorktree(i.Path, i.Title)
		if err != nil {
			return fmt.Errorf("failed to create git worktree: %w", err)
		}
		i.gitWorktree = gitWorktree
		i.Branch = branchName
	}

	// Setup error handler to cleanup resources on any error
	var setupErr error
	defer func() {
		if setupErr != nil {
			if cleanupErr := i.Kill(); cleanupErr != nil {
				setupErr = fmt.Errorf("%v (cleanup error: %v)", setupErr, cleanupErr)
			}
		} else {
			i.started = true
		}
	}()

	if !firstTimeSetup {
		// Reuse existing sessions
		if err := tmuxSession.Restore(); err != nil {
			setupErr = fmt.Errorf("failed to restore existing session: %w", err)
			return setupErr
		}
		if err := i.consoleTmuxSession.Restore(); err != nil {
			// Console session restore failed, try to create a new one
			log.WarningLog.Printf("Failed to restore console session for %s: %v, creating new one", i.Title, err)
			if err := i.consoleTmuxSession.Start(i.gitWorktree.GetWorktreePath()); err != nil {
				log.WarningLog.Printf("Failed to start new console session for %s: %v", i.Title, err)
			}
		}
	} else {
		// Setup git worktree first
		if err := i.gitWorktree.Setup(); err != nil {
			setupErr = fmt.Errorf("failed to setup git worktree: %w", err)
			return setupErr
		}

		// Create new sessions
		if err := i.tmuxSession.Start(i.gitWorktree.GetWorktreePath()); err != nil {
			// Cleanup git worktree if tmux session creation fails
			if cleanupErr := i.gitWorktree.Cleanup(); cleanupErr != nil {
				err = fmt.Errorf("%v (cleanup error: %v)", err, cleanupErr)
			}
			setupErr = fmt.Errorf("failed to start new session: %w", err)
			return setupErr
		}

		// Start console session in the same worktree
		if err := i.consoleTmuxSession.Start(i.gitWorktree.GetWorktreePath()); err != nil {
			// Console session failure is not critical, just log it
			log.WarningLog.Printf("Failed to start console session for %s: %v", i.Title, err)
		}
	}

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

	// Always try to cleanup all resources, even if one fails
	// Clean up tmux sessions first since they're using the git worktree
	if i.tmuxSession != nil {
		if err := i.tmuxSession.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close tmux session: %w", err))
		}
	}

	if i.consoleTmuxSession != nil {
		if err := i.consoleTmuxSession.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close console tmux session: %w", err))
		}
	}

	// Then clean up git worktree
	if i.gitWorktree != nil {
		if err := i.gitWorktree.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("failed to cleanup git worktree: %w", err))
		}
	}

	return i.combineErrors(errs)
}

// ConsolePreview returns a preview of the console session content
func (i *Instance) ConsolePreview() (string, error) {
	if !i.started || i.consoleTmuxSession == nil {
		return "Console not available", nil
	}

	// Check if console session exists, if not try to create it
	if !i.consoleTmuxSession.DoesSessionExist() {
		log.WarningLog.Printf("Console session for %s doesn't exist, attempting to create", i.Title)
		if err := i.consoleTmuxSession.Start(i.gitWorktree.GetWorktreePath()); err != nil {
			return fmt.Sprintf("Console session unavailable: %v", err), nil
		}
	}

	content, err := i.consoleTmuxSession.CapturePaneContent()
	if err != nil {
		return fmt.Sprintf("Error getting console content: %v", err), nil
	}

	if content == "" {
		return "Console session ready. Press Enter to attach.", nil
	}

	return content, nil
}

// AttachToConsole attaches to the console session
func (i *Instance) AttachToConsole() (<-chan struct{}, error) {
	if !i.started || i.consoleTmuxSession == nil {
		return nil, fmt.Errorf("console session not available")
	}

	return i.consoleTmuxSession.Attach()
}

// ConsoleAlive checks if the console tmux session is still alive
func (i *Instance) ConsoleAlive() bool {
	if !i.started || i.consoleTmuxSession == nil {
		return false
	}
	return i.consoleTmuxSession.DoesSessionExist()
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

// Close is an alias for Kill to maintain backward compatibility
func (i *Instance) Close() error {
	if !i.started {
		return fmt.Errorf("cannot close instance that has not been started")
	}
	return i.Kill()
}

func (i *Instance) Preview() (string, error) {
	if !i.started || i.Status == Paused {
		return "", nil
	}
	// Capture all history instead of just visible content
	return i.tmuxSession.CapturePaneContentWithOptions("-", "-")
}

func (i *Instance) HasUpdated() (updated bool, hasPrompt bool) {
	if !i.started {
		return false, false
	}
	return i.tmuxSession.HasUpdated()
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

	var errs []error

	// Check if there are any changes to commit
	if dirty, err := i.gitWorktree.IsDirty(); err != nil {
		errs = append(errs, fmt.Errorf("failed to check if worktree is dirty: %w", err))
		log.ErrorLog.Print(err)
	} else if dirty {
		// Commit changes with timestamp
		commitMsg := fmt.Sprintf("[claudesquad] update from '%s' on %s (paused)", i.Title, time.Now().Format(time.RFC822))
		if err := i.gitWorktree.PushChanges(commitMsg, false); err != nil {
			errs = append(errs, fmt.Errorf("failed to commit changes: %w", err))
			log.ErrorLog.Print(err)
			// Return early if we can't commit changes to avoid corrupted state
			return i.combineErrors(errs)
		}
	}

	// Close tmux session first since it's using the git worktree
	if err := i.tmuxSession.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close tmux session: %w", err))
		log.ErrorLog.Print(err)
		// Return early if we can't close tmux to avoid corrupted state
		return i.combineErrors(errs)
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

// GetDiffStats returns the current git diff statistics
func (i *Instance) GetDiffStats() *git.DiffStats {
	return i.diffStats
}

// SendPrompt sends a prompt to the tmux session
func (i *Instance) SendPrompt(prompt string) error {
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

	return nil
}

// Restart terminates and restarts the tmux session while preserving worktree and other state
// This is useful when MCP configuration changes and Claude needs to be restarted with new config
func (i *Instance) Restart() error {
	if !i.started {
		return fmt.Errorf("cannot restart instance that has not been started")
	}
	if i.Status == Paused {
		return fmt.Errorf("cannot restart paused instance")
	}

	// Check if there are any changes to commit before restart
	if dirty, err := i.gitWorktree.IsDirty(); err != nil {
		log.ErrorLog.Printf("failed to check if worktree is dirty before restart: %v", err)
		// Continue with restart even if we can't check dirty status
	} else if dirty {
		// Commit changes with timestamp before restart
		commitMsg := fmt.Sprintf("[claudesquad] auto-commit before Claude restart on %s", time.Now().Format(time.RFC822))
		if err := i.gitWorktree.PushChanges(commitMsg, false); err != nil {
			log.ErrorLog.Printf("failed to commit changes before restart: %v", err)
			// Continue with restart even if commit fails - user can manually recover
		}
	}

	// Store current worktree path before closing sessions
	worktreePath := i.gitWorktree.GetWorktreePath()
	
	// Update program command with new MCP configuration for this worktree
	cfg := config.LoadConfig()
	i.Program = config.ModifyCommandWithMCPForWorktree(i.Program, cfg, worktreePath)

	// Close existing tmux sessions (but keep worktree)
	var errs []error

	if i.tmuxSession != nil {
		if err := i.tmuxSession.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close tmux session during restart: %w", err))
		}
	}

	if i.consoleTmuxSession != nil {
		if err := i.consoleTmuxSession.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close console tmux session during restart: %w", err))
		}
	}

	// Return early if we couldn't close sessions cleanly
	if len(errs) > 0 {
		return i.combineErrors(errs)
	}

	// Create new tmux sessions with updated configuration
	i.tmuxSession = tmux.NewTmuxSession(i.Title, i.Program)
	
	// Create new console session
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	i.consoleTmuxSession = tmux.NewTmuxSession(i.Title+"-console", shell+" -i")

	// Start the new tmux session with updated MCP configuration
	if err := i.tmuxSession.Start(worktreePath); err != nil {
		// If restart fails, try to restore original session if possible
		log.ErrorLog.Printf("Failed to start new tmux session for %s: %v", i.Title, err)
		
		// Create a new session with original program (fallback)
		fallbackSession := tmux.NewTmuxSession(i.Title, i.Program)
		if startErr := fallbackSession.Start(worktreePath); startErr == nil {
			log.InfoLog.Printf("Successfully started fallback session for %s after restart failure", i.Title)
			i.tmuxSession = fallbackSession
			i.SetStatus(Running)
			return fmt.Errorf("restart failed but fallback session started: %w", err)
		}
		
		// If both restart and fallback fail, mark as paused
		i.SetStatus(Paused)
		return fmt.Errorf("failed to restart tmux session and could not start fallback: %w", err)
	}

	// Start new console session  
	if err := i.consoleTmuxSession.Start(worktreePath); err != nil {
		// Console session failure is not critical, just log it
		log.WarningLog.Printf("Failed to start new console session for %s: %v", i.Title, err)
	}

	i.SetStatus(Running)
	log.InfoLog.Printf("Successfully restarted Claude instance %s with new MCP configuration", i.Title)
	return nil
}
