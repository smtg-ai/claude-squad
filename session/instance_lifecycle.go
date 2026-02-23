package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ByteMirror/hivemind/log"
	"github.com/ByteMirror/hivemind/session/git"
	"github.com/ByteMirror/hivemind/session/tmux"

	"github.com/atotto/clipboard"
)

// firstTimeSetup is true if this is a new instance. Otherwise, it's one loaded from storage.
func (i *Instance) Start(firstTimeSetup bool) error {
	if i.Title == "" {
		return ErrTitleEmpty
	}

	if firstTimeSetup {
		i.LoadingTotal = 8
	} else {
		i.LoadingTotal = 6
	}
	i.LoadingStage = 0
	i.LoadingMessage = "Initializing..."

	i.setLoadingProgress(1, "Preparing session...")
	var tmuxSession *tmux.TmuxSession
	if i.tmuxSession != nil {
		tmuxSession = i.tmuxSession
	} else {
		tmuxSession = tmux.NewTmuxSession(i.Title, i.Program, i.SkipPermissions)
	}
	// Wire up tmux progress to instance loading progress
	tmuxStageOffset := 3 // tmux stages start at 4 for first-time, 2 for reload
	if !firstTimeSetup {
		tmuxStageOffset = 1
	}
	tmuxSession.ProgressFunc = func(stage int, desc string) {
		i.setLoadingProgress(tmuxStageOffset+stage, desc)
	}
	i.tmuxSession = tmuxSession

	skipWorktreeSetup := false
	if firstTimeSetup {
		i.setLoadingProgress(2, "Creating git worktree...")
		if i.branchOverride != "" {
			absPath, _ := filepath.Abs(i.Path)
			existingPath, _ := git.FindWorktreePathForBranch(absPath, i.branchOverride)
			if existingPath != "" {
				// Branch is already checked out — reuse its worktree directly
				repoRoot, _ := git.FindGitRepoRoot(absPath)
				i.gitWorktree = git.NewGitWorktreeReusingExisting(repoRoot, existingPath, i.branchOverride)
				i.Branch = i.branchOverride
				i.sharedWorktree = true // skip Kill() worktree cleanup
				skipWorktreeSetup = true
			} else {
				// Branch exists but not checked out — create a new worktree for it
				gitWorktree, err := git.NewGitWorktreeForExistingBranch(absPath, i.Title, i.branchOverride)
				if err != nil {
					return fmt.Errorf("failed to create worktree for existing branch: %w", err)
				}
				i.gitWorktree = gitWorktree
				i.Branch = i.branchOverride
			}
		} else {
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
			if cleanupErr := i.Kill(); cleanupErr != nil {
				setupErr = fmt.Errorf("%v (cleanup error: %v)", setupErr, cleanupErr)
			}
		} else {
			// Store with release semantics: tmuxSession and gitWorktree are
			// guaranteed to be visible to any goroutine that observes started==true.
			i.started.Store(true)
		}
	}()

	if !firstTimeSetup {
		i.setLoadingProgress(2, "Restoring session...")
		// Reuse existing session
		if err := tmuxSession.Restore(); err != nil {
			setupErr = fmt.Errorf("failed to restore existing session: %w", err)
			return setupErr
		}
	} else {
		i.setLoadingProgress(3, "Setting up git worktree...")
		// Setup git worktree first (skip if reusing an existing checked-out worktree)
		if !skipWorktreeSetup {
			if err := i.gitWorktree.Setup(); err != nil {
				setupErr = fmt.Errorf("failed to setup git worktree: %w", err)
				return setupErr
			}
		}

		if isClaudeProgram(i.Program) {
			worktreePath := i.gitWorktree.GetWorktreePath()
			repoPath := i.Path
			title := i.Title
			go func() {
				if err := registerMCPServer(worktreePath, repoPath, title); err != nil {
					log.WarningLog.Printf("failed to write MCP config: %v", err)
				}
			}()
		}

		i.setLoadingProgress(4, "Starting tmux session...")
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

	i.SetStatus(Running)

	return nil
}

// StartInSharedWorktree starts the instance using a topic's shared worktree.
// Unlike Start(), this does NOT create a new git worktree — it uses the one provided.
func (i *Instance) StartInSharedWorktree(worktree *git.GitWorktree, branch string) error {
	if i.Title == "" {
		return ErrTitleEmpty
	}

	i.LoadingTotal = 6
	i.setLoadingProgress(1, "Connecting to shared worktree...")

	i.gitWorktree = worktree
	i.Branch = branch
	i.sharedWorktree = true

	var tmuxSession *tmux.TmuxSession
	if i.tmuxSession != nil {
		tmuxSession = i.tmuxSession
	} else {
		tmuxSession = tmux.NewTmuxSession(i.Title, i.Program, i.SkipPermissions)
	}
	tmuxSession.ProgressFunc = func(stage int, desc string) {
		i.setLoadingProgress(1+stage, desc)
	}
	i.tmuxSession = tmuxSession

	// Ensure the shared worktree directory exists — it may have been
	// cleaned up if all previous instances in the topic were killed.
	if _, err := os.Stat(worktree.GetWorktreePath()); err != nil {
		i.setLoadingProgress(2, "Recreating shared worktree...")
		if err := worktree.Setup(); err != nil {
			return fmt.Errorf("failed to recreate shared worktree: %w", err)
		}
	}

	i.setLoadingProgress(3, "Starting tmux session...")

	if isClaudeProgram(i.Program) {
		wtPath := worktree.GetWorktreePath()
		repoPath := i.Path
		title := i.Title
		go func() {
			if err := registerMCPServer(wtPath, repoPath, title); err != nil {
				log.WarningLog.Printf("failed to write MCP config: %v", err)
			}
		}()
	}

	if err := i.tmuxSession.Start(worktree.GetWorktreePath()); err != nil {
		return fmt.Errorf("failed to start session in shared worktree: %w", err)
	}

	i.started.Store(true)
	i.SetStatus(Running)
	return nil
}

// Kill terminates the instance and cleans up all resources
func (i *Instance) Kill() error {
	if !i.started.Load() {
		// If instance was never started, just return success
		return nil
	}

	var errs []error

	// Always try to cleanup both resources, even if one fails
	// Clean up tmux session first since it's using the git worktree
	if i.tmuxSession != nil {
		if err := i.tmuxSession.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close tmux session: %w", err))
		}
	}

	// Then clean up git worktree (skip if shared — topic owns the worktree)
	if i.gitWorktree != nil && !i.sharedWorktree {
		if err := i.gitWorktree.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("failed to cleanup git worktree: %w", err))
		}
	}

	return errors.Join(errs...)
}

// Pause stops the tmux session and removes the worktree, preserving the branch
func (i *Instance) Pause() error {
	if !i.started.Load() {
		return ErrInstanceNotStarted
	}
	if i.Status == Paused {
		return ErrInstanceAlreadyPaused
	}

	var errs []error

	if !i.sharedWorktree {
		// Check if there are any changes to commit
		if dirty, err := i.gitWorktree.IsDirty(); err != nil {
			errs = append(errs, fmt.Errorf("failed to check if worktree is dirty: %w", err))
			log.ErrorLog.Print(err)
		} else if dirty {
			// Commit changes locally (without pushing to GitHub)
			commitMsg := fmt.Sprintf("[hivemind] update from '%s' on %s (paused)", i.Title, time.Now().Format(time.RFC822))
			if err := i.gitWorktree.CommitChanges(commitMsg); err != nil {
				errs = append(errs, fmt.Errorf("failed to commit changes: %w", err))
				log.ErrorLog.Print(err)
				// Return early if we can't commit changes to avoid corrupted state
				return errors.Join(errs...)
			}
		}
	}

	// Detach from tmux session instead of closing to preserve session output
	if err := i.tmuxSession.DetachSafely(); err != nil {
		errs = append(errs, fmt.Errorf("failed to detach tmux session: %w", err))
		log.ErrorLog.Print(err)
		// Continue with pause process even if detach fails
	}

	if !i.sharedWorktree {
		// Check if worktree exists before trying to remove it
		if _, err := os.Stat(i.gitWorktree.GetWorktreePath()); err == nil {
			// Remove worktree but keep branch
			if err := i.gitWorktree.Remove(); err != nil {
				errs = append(errs, fmt.Errorf("failed to remove git worktree: %w", err))
				log.ErrorLog.Print(err)
				return errors.Join(errs...)
			}

			// Only prune if remove was successful
			if err := i.gitWorktree.Prune(); err != nil {
				errs = append(errs, fmt.Errorf("failed to prune git worktrees: %w", err))
				log.ErrorLog.Print(err)
				return errors.Join(errs...)
			}
		}
	}

	if err := errors.Join(errs...); err != nil {
		log.ErrorLog.Print(err)
		return err
	}

	i.SetStatus(Paused)
	_ = clipboard.WriteAll(i.gitWorktree.GetBranchName())
	return nil
}

// Resume recreates the worktree and restarts the tmux session
func (i *Instance) Resume() error {
	if !i.started.Load() {
		return ErrInstanceNotStarted
	}
	if i.Status != Paused && i.Status != Loading {
		return fmt.Errorf("can only resume paused instances")
	}

	// Reset the dead flag so the resumed session will be polled normally.
	i.tmuxDead.Store(false)

	i.LoadingTotal = 4
	i.setLoadingProgress(1, "Checking branch...")

	// Check if branch is checked out
	if checked, err := i.gitWorktree.IsBranchCheckedOut(); err != nil {
		log.ErrorLog.Print(err)
		return fmt.Errorf("failed to check if branch is checked out: %w", err)
	} else if checked {
		return ErrBranchCheckedOut
	}

	i.setLoadingProgress(2, "Setting up worktree...")

	// Setup git worktree
	if err := i.gitWorktree.Setup(); err != nil {
		log.ErrorLog.Print(err)
		return fmt.Errorf("failed to setup git worktree: %w", err)
	}

	if isClaudeProgram(i.Program) {
		worktreePath := i.gitWorktree.GetWorktreePath()
		repoPath := i.Path
		title := i.Title
		go func() {
			if err := registerMCPServer(worktreePath, repoPath, title); err != nil {
				log.WarningLog.Printf("failed to write MCP config: %v", err)
			}
		}()
	}

	i.setLoadingProgress(3, "Restoring session...")

	// Check if tmux session still exists from pause, otherwise create new one
	if i.tmuxSession.DoesSessionExist() {
		// Session exists, just restore PTY connection to it
		if err := i.tmuxSession.Restore(); err != nil {
			log.ErrorLog.Print(err)
			i.setLoadingProgress(3, "Creating new session...")
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
		i.setLoadingProgress(3, "Creating new session...")
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

	i.setLoadingProgress(4, "Ready")
	i.SetStatus(Running)
	return nil
}

// Restart recreates the tmux session for an instance whose agent process has
// exited (tmuxDead). If the worktree directory still exists it is reused;
// otherwise the worktree is recreated from the branch (like Resume).
func (i *Instance) Restart() error {
	if !i.started.Load() {
		return ErrInstanceNotStarted
	}
	if !i.tmuxDead.Load() {
		return fmt.Errorf("instance is not dead, use Resume for paused instances")
	}

	i.LoadingTotal = 4
	i.setLoadingProgress(1, "Restarting agent...")

	worktreePath := i.gitWorktree.GetWorktreePath()

	// If the worktree directory is gone, recreate it from the branch
	if _, err := os.Stat(worktreePath); err != nil {
		i.setLoadingProgress(2, "Recreating worktree...")
		if err := i.gitWorktree.Setup(); err != nil {
			return fmt.Errorf("failed to recreate worktree: %w", err)
		}
	}

	if isClaudeProgram(i.Program) {
		repoPath := i.Path
		title := i.Title
		go func() {
			if err := registerMCPServer(worktreePath, repoPath, title); err != nil {
				log.WarningLog.Printf("failed to write MCP config: %v", err)
			}
		}()
	}

	i.setLoadingProgress(3, "Creating new session...")

	// The old tmux session is dead, create a fresh one with the same name
	if err := i.tmuxSession.Start(worktreePath); err != nil {
		return fmt.Errorf("failed to restart session: %w", err)
	}

	i.tmuxDead.Store(false)
	i.setLoadingProgress(4, "Ready")
	i.SetStatus(Running)
	return nil
}
