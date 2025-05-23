package git

import (
	"chronos/log"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SyncOptions defines the available options for synchronization
type SyncOptions struct {
	// PullFromMain indicates whether to pull changes from the main branch
	PullFromMain bool
	// UpdateSubmodules indicates whether to update submodules recursively
	UpdateSubmodules bool
	// AutoResolveConflicts attempts to auto-resolve conflicts (uses ours strategy)
	AutoResolveConflicts bool
	// CommitMessage to use if changes are automatically committed
	CommitMessage string
}

// DefaultSyncOptions returns default synchronization options
func DefaultSyncOptions() SyncOptions {
	return SyncOptions{
		PullFromMain:         true,
		UpdateSubmodules:     true,
		AutoResolveConflicts: false,
		CommitMessage:        fmt.Sprintf("Auto-sync update at %s", time.Now().Format(time.RFC3339)),
	}
}

// SyncStatus represents the result of a sync operation
type SyncStatus struct {
	Success          bool
	UpdatedFromMain  bool
	UpdatedSubmodule bool
	Conflicts        bool
	ConflictsResolved bool
	Message          string
	Error            error
}

// DetectMainBranch attempts to identify the main/default branch of the repository
func (g *GitWorktree) DetectMainBranch() (string, error) {
	// First try to get the default branch from the remote
	output, err := g.runGitCommand(g.repoPath, "remote", "show", "origin")
	if err == nil {
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "HEAD branch:") {
				return strings.TrimSpace(strings.TrimPrefix(line, "HEAD branch:")), nil
			}
		}
	}

	// Fall back to checking if main or master exist
	for _, branch := range []string{"main", "master"} {
		output, err := g.runGitCommand(g.repoPath, "branch", "--list", branch)
		if err == nil && strings.TrimSpace(output) != "" {
			return branch, nil
		}
	}

	// Check for other common default branches
	output, err = g.runGitCommand(g.repoPath, "branch", "--list")
	if err == nil {
		branches := strings.Split(output, "\n")
		if len(branches) > 0 {
			// Return the first branch
			for _, branch := range branches {
				if strings.HasPrefix(branch, "*") {
					return strings.TrimSpace(strings.TrimPrefix(branch, "*")), nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not detect main branch")
}

// UpdateFromMain pulls changes from the main branch into the current branch
func (g *GitWorktree) UpdateFromMain() (bool, error) {
	mainBranch, err := g.DetectMainBranch()
	if err != nil {
		return false, fmt.Errorf("failed to detect main branch: %w", err)
	}

	log.InfoLog.Printf("Syncing from main branch: %s", mainBranch)

	// Make sure we have latest from remote
	_, err = g.runGitCommand(g.worktreePath, "fetch", "origin", mainBranch)
	if err != nil {
		return false, fmt.Errorf("failed to fetch from remote: %w", err)
	}

	// Check if there are any changes to pull
	output, err := g.runGitCommand(g.worktreePath, "rev-list", "--count", fmt.Sprintf("HEAD..origin/%s", mainBranch))
	if err != nil {
		return false, fmt.Errorf("failed to check for upstream changes: %w", err)
	}

	if strings.TrimSpace(output) == "0" {
		log.InfoLog.Printf("No changes to pull from %s", mainBranch)
		return false, nil
	}

	// Try to merge from main branch
	output, err = g.runGitCommand(g.worktreePath, "merge", fmt.Sprintf("origin/%s", mainBranch))
	if err != nil {
		// If there are conflicts, abort the merge and inform the user
		if strings.Contains(output, "Automatic merge failed") {
			if _, abortErr := g.runGitCommand(g.worktreePath, "merge", "--abort"); abortErr != nil {
				log.ErrorLog.Printf("Failed to abort merge: %v", abortErr)
			}
			return false, fmt.Errorf("merge conflicts detected: %w", err)
		}
		return false, fmt.Errorf("failed to merge from %s: %w", mainBranch, err)
	}

	log.InfoLog.Printf("Successfully updated from %s", mainBranch)
	return true, nil
}

// UpdateFromMainWithStrategy pulls changes from main and uses a specified merge strategy
func (g *GitWorktree) UpdateFromMainWithStrategy(strategy string) (bool, error) {
	mainBranch, err := g.DetectMainBranch()
	if err != nil {
		return false, fmt.Errorf("failed to detect main branch: %w", err)
	}

	log.InfoLog.Printf("Syncing from main branch: %s with strategy: %s", mainBranch, strategy)

	// Make sure we have latest from remote
	_, err = g.runGitCommand(g.worktreePath, "fetch", "origin", mainBranch)
	if err != nil {
		return false, fmt.Errorf("failed to fetch from remote: %w", err)
	}

	// Try to merge from main branch with the specified strategy
	output, err := g.runGitCommand(g.worktreePath, "merge", "-X", strategy, fmt.Sprintf("origin/%s", mainBranch))
	if err != nil {
		// If there are conflicts even with strategy, abort the merge
		if strings.Contains(output, "Automatic merge failed") {
			if _, abortErr := g.runGitCommand(g.worktreePath, "merge", "--abort"); abortErr != nil {
				log.ErrorLog.Printf("Failed to abort merge: %v", abortErr)
			}
			return false, fmt.Errorf("merge conflicts detected even with %s strategy: %w", strategy, err)
		}
		return false, fmt.Errorf("failed to merge from %s with strategy %s: %w", mainBranch, strategy, err)
	}

	log.InfoLog.Printf("Successfully updated from %s with strategy %s", mainBranch, strategy)
	return true, nil
}

// UpdateSubmodules updates all submodules recursively
func (g *GitWorktree) UpdateSubmodules() (bool, error) {
	// First check if this repo has submodules
	if _, err := os.Stat(filepath.Join(g.worktreePath, ".gitmodules")); os.IsNotExist(err) {
		log.InfoLog.Printf("No .gitmodules found, skipping submodule update")
		return false, nil
	}

	// Initialize submodules if they haven't been initialized
	_, err := g.runGitCommand(g.worktreePath, "submodule", "init")
	if err != nil {
		return false, fmt.Errorf("failed to initialize submodules: %w", err)
	}

	// Update submodules recursively
	output, err := g.runGitCommand(g.worktreePath, "submodule", "update", "--recursive", "--remote")
	if err != nil {
		return false, fmt.Errorf("failed to update submodules: %w", err)
	}

	if strings.TrimSpace(output) != "" {
		log.InfoLog.Printf("Submodules updated: %s", output)
		return true, nil
	}

	log.InfoLog.Printf("Submodules are already up to date")
	return false, nil
}

// Sync synchronizes the worktree with the remote repository
// It can update from the main branch and update submodules based on the options
func (g *GitWorktree) Sync(options SyncOptions) SyncStatus {
	status := SyncStatus{
		Success:          true,
		UpdatedFromMain:  false,
		UpdatedSubmodule: false,
		Conflicts:        false,
		ConflictsResolved: false,
		Message:          "Synchronization completed successfully",
	}

	// Check if there are uncommitted changes
	isDirty, err := g.IsDirty()
	if err != nil {
		status.Success = false
		status.Error = fmt.Errorf("failed to check for uncommitted changes: %w", err)
		status.Message = status.Error.Error()
		return status
	}

	if isDirty {
		log.InfoLog.Print("Uncommitted changes detected, stashing changes before sync")
		if _, err := g.runGitCommand(g.worktreePath, "stash", "save", "Auto-stash before sync"); err != nil {
			status.Success = false
			status.Error = fmt.Errorf("failed to stash changes: %w", err)
			status.Message = status.Error.Error()
			return status
		}
	}

	// Pull changes from the main branch if requested
	if options.PullFromMain {
		updated, err := g.UpdateFromMain()
		if err != nil {
			if strings.Contains(err.Error(), "merge conflicts detected") {
				status.Conflicts = true
				log.WarningLog.Printf("Conflicts detected during sync: %v", err)
				
				// Try auto-resolve if requested
				if options.AutoResolveConflicts {
					log.InfoLog.Print("Attempting to auto-resolve conflicts with 'ours' strategy")
					if _, err := g.UpdateFromMainWithStrategy("ours"); err != nil {
						status.Success = false
						status.Error = fmt.Errorf("failed to auto-resolve conflicts: %w", err)
						status.Message = status.Error.Error()
					} else {
						status.ConflictsResolved = true
						log.InfoLog.Print("Conflicts auto-resolved successfully")
					}
				} else {
					status.Success = false
					status.Error = err
					status.Message = "Merge conflicts detected. Please resolve manually."
				}
			} else {
				status.Success = false
				status.Error = fmt.Errorf("failed to update from main branch: %w", err)
				status.Message = status.Error.Error()
			}
		} else {
			status.UpdatedFromMain = updated
			if updated {
				status.Message = "Updated from main branch. "
			} else {
				status.Message = "No updates from main branch needed. "
			}
		}
	}

	// Update submodules if requested
	if options.UpdateSubmodules && status.Success {
		updated, err := g.UpdateSubmodules()
		if err != nil {
			status.Success = false
			status.Error = fmt.Errorf("failed to update submodules: %w", err)
			status.Message += status.Error.Error()
		} else {
			status.UpdatedSubmodule = updated
			if updated {
				status.Message += "Submodules updated successfully."
			} else {
				status.Message += "No submodule updates needed."
			}
		}
	}

	// Pop stashed changes if we stashed them earlier
	if isDirty {
		log.InfoLog.Print("Popping stashed changes")
		if _, err := g.runGitCommand(g.worktreePath, "stash", "pop"); err != nil {
			if !strings.Contains(err.Error(), "No stash entries found") {
				log.WarningLog.Printf("Failed to pop stash: %v", err)
				status.Message += " Warning: Failed to restore stashed changes."
			}
		}
	}

	// Auto-commit changes if there are conflicts that were resolved
	if status.ConflictsResolved {
		// Commit the merge resolution
		if _, err := g.runGitCommand(g.worktreePath, "commit", "-m", options.CommitMessage); err != nil {
			log.WarningLog.Printf("Failed to commit merged changes: %v", err)
		} else {
			log.InfoLog.Print("Auto-committed merged changes")
		}
	}

	return status
}

// ListSubmodules returns a list of submodules in the repository
func (g *GitWorktree) ListSubmodules() ([]string, error) {
	// Check if .gitmodules exists
	if _, err := os.Stat(filepath.Join(g.worktreePath, ".gitmodules")); os.IsNotExist(err) {
		return []string{}, nil
	}

	output, err := g.runGitCommand(g.worktreePath, "submodule", "status")
	if err != nil {
		return nil, fmt.Errorf("failed to list submodules: %w", err)
	}

	var submodules []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		// Parse the submodule path from the output
		// Format is: [+-U]<sha1> <path> [(<pretty-name>)]
		parts := strings.Fields(strings.TrimSpace(line))
		if len(parts) >= 2 {
			submodules = append(submodules, parts[1])
		}
	}

	return submodules, nil
}

// GetSubmoduleStatus returns the status of each submodule
func (g *GitWorktree) GetSubmoduleStatus() (map[string]string, error) {
	// Check if .gitmodules exists
	if _, err := os.Stat(filepath.Join(g.worktreePath, ".gitmodules")); os.IsNotExist(err) {
		return map[string]string{}, nil
	}

	output, err := g.runGitCommand(g.worktreePath, "submodule", "status")
	if err != nil {
		return nil, fmt.Errorf("failed to get submodule status: %w", err)
	}

	status := make(map[string]string)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		// Parse the status from the output
		// Format is: [+-U]<sha1> <path> [(<pretty-name>)]
		line = strings.TrimSpace(line)
		prefix := ""
		
		if strings.HasPrefix(line, "+") {
			prefix = "+"
			line = line[1:]
		} else if strings.HasPrefix(line, "-") {
			prefix = "-"
			line = line[1:]
		} else if strings.HasPrefix(line, "U") {
			prefix = "U"
			line = line[1:]
		}
		
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			path := parts[1]
			switch prefix {
			case "+":
				status[path] = "outdated"
			case "-":
				status[path] = "uninitialized"
			case "U":
				status[path] = "conflict"
			default:
				status[path] = "up-to-date"
			}
		}
	}

	return status, nil
}

// checkGHCLIForSync checks if GitHub CLI is available for sync operations
func checkGHCLIForSync() error {
	cmd := exec.Command("gh", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("GitHub CLI (gh) is not installed or not in PATH: %w", err)
	}
	return nil
}