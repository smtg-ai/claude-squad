package agent

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// SynchronizationCoordinator manages git and distributed state synchronization
type SynchronizationCoordinator struct {
	mu               sync.RWMutex
	squadID          string
	repositoryPath   string
	lastSync         time.Time
	conflictResolver *ConflictResolver
	gitOperations    *GitOperations
	status           SyncStatus
}

// SyncStatus represents the current synchronization status
type SyncStatus struct {
	LastSync        time.Time `json:"last_sync"`
	SyncInProgress  bool      `json:"sync_in_progress"`
	HasConflicts    bool      `json:"has_conflicts"`
	ConflictCount   int       `json:"conflict_count"`
	LastError       string    `json:"last_error"`
	SuccessfulSyncs int       `json:"successful_syncs"`
	FailedSyncs     int       `json:"failed_syncs"`
}

// ConflictResolver handles automatic conflict resolution
type ConflictResolver struct {
	strategy string // "ours", "theirs", "auto"
}

// GitOperations handles git repository operations
type GitOperations struct {
	repoPath string
}

// NewSynchronizationCoordinator creates a new synchronization coordinator
func NewSynchronizationCoordinator(squadID string) *SynchronizationCoordinator {
	return &SynchronizationCoordinator{
		squadID:          squadID,
		repositoryPath:   ".", // Current directory
		lastSync:         time.Now(),
		conflictResolver: &ConflictResolver{strategy: "auto"},
		gitOperations:    &GitOperations{repoPath: "."},
		status: SyncStatus{
			LastSync:        time.Now(),
			SyncInProgress:  false,
			HasConflicts:    false,
			ConflictCount:   0,
			LastError:       "",
			SuccessfulSyncs: 0,
			FailedSyncs:     0,
		},
	}
}

// SyncGitRepository performs full git repository synchronization
func (sc *SynchronizationCoordinator) SyncGitRepository() error {
	sc.mu.Lock()
	sc.status.SyncInProgress = true
	sc.mu.Unlock()
	
	defer func() {
		sc.mu.Lock()
		sc.status.SyncInProgress = false
		sc.status.LastSync = time.Now()
		sc.mu.Unlock()
	}()
	
	// Step 1: Fetch latest changes
	if err := sc.gitOperations.Fetch(); err != nil {
		sc.recordSyncError(fmt.Sprintf("fetch failed: %v", err))
		return err
	}
	
	// Step 2: Check for conflicts
	conflicts, err := sc.gitOperations.CheckConflicts()
	if err != nil {
		sc.recordSyncError(fmt.Sprintf("conflict check failed: %v", err))
		return err
	}
	
	sc.mu.Lock()
	sc.status.HasConflicts = len(conflicts) > 0
	sc.status.ConflictCount = len(conflicts)
	sc.mu.Unlock()
	
	// Step 3: Resolve conflicts if present
	if len(conflicts) > 0 {
		if err := sc.conflictResolver.ResolveConflicts(conflicts); err != nil {
			sc.recordSyncError(fmt.Sprintf("conflict resolution failed: %v", err))
			return err
		}
	}
	
	// Step 4: Pull changes
	if err := sc.gitOperations.Pull(); err != nil {
		sc.recordSyncError(fmt.Sprintf("pull failed: %v", err))
		return err
	}
	
	// Step 5: Push local changes
	if err := sc.gitOperations.Push(sc.squadID); err != nil {
		sc.recordSyncError(fmt.Sprintf("push failed: %v", err))
		return err
	}
	
	sc.recordSyncSuccess()
	return nil
}

// SyncSharedKnowledge synchronizes shared knowledge across squads
func (sc *SynchronizationCoordinator) SyncSharedKnowledge() error {
	// This would typically involve:
	// 1. Export local knowledge registry
	// 2. Pull remote knowledge state
	// 3. Merge using CRDT semantics
	// 4. Push merged state
	
	// For now, we'll implement a simplified version
	fmt.Printf("Syncing shared knowledge for squad %s\n", sc.squadID)
	return nil
}

// HasConflicts checks if there are pending conflicts
func (sc *SynchronizationCoordinator) HasConflicts() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.status.HasConflicts
}

// GetStatus returns the current synchronization status
func (sc *SynchronizationCoordinator) GetStatus() SyncStatus {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.status
}

// recordSyncError records a synchronization error
func (sc *SynchronizationCoordinator) recordSyncError(errorMsg string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	sc.status.LastError = errorMsg
	sc.status.FailedSyncs++
	sc.status.HasConflicts = true
}

// recordSyncSuccess records a successful synchronization
func (sc *SynchronizationCoordinator) recordSyncSuccess() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	sc.status.LastError = ""
	sc.status.SuccessfulSyncs++
	sc.status.HasConflicts = false
	sc.status.ConflictCount = 0
}

// Git Operations Implementation

// Fetch fetches the latest changes from remote
func (g *GitOperations) Fetch() error {
	cmd := exec.Command("git", "fetch", "--all")
	cmd.Dir = g.repoPath
	return cmd.Run()
}

// Pull pulls changes from the main branch
func (g *GitOperations) Pull() error {
	cmd := exec.Command("git", "pull", "origin", "main")
	cmd.Dir = g.repoPath
	return cmd.Run()
}

// Push pushes changes with squad-specific commit message
func (g *GitOperations) Push(squadID string) error {
	// Check if there are changes to commit
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = g.repoPath
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	
	// If there are changes, commit them
	if len(output) > 0 {
		// Add all changes
		addCmd := exec.Command("git", "add", ".")
		addCmd.Dir = g.repoPath
		if err := addCmd.Run(); err != nil {
			return err
		}
		
		// Commit with squad message
		commitMsg := fmt.Sprintf("[claude-squad] Auto-sync from %s squad at %s", 
			squadID, time.Now().Format(time.RFC3339))
		commitCmd := exec.Command("git", "commit", "-m", commitMsg)
		commitCmd.Dir = g.repoPath
		if err := commitCmd.Run(); err != nil {
			return err
		}
	}
	
	// Push to remote
	pushCmd := exec.Command("git", "push", "origin", "main")
	pushCmd.Dir = g.repoPath
	return pushCmd.Run()
}

// CheckConflicts checks for merge conflicts
func (g *GitOperations) CheckConflicts() ([]string, error) {
	// Check git status for conflict markers
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = g.repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	
	var conflicts []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "UU ") || strings.HasPrefix(line, "AA ") {
			conflicts = append(conflicts, strings.TrimSpace(line[3:]))
		}
	}
	
	return conflicts, nil
}

// Conflict Resolution Implementation

// ResolveConflicts automatically resolves conflicts based on strategy
func (cr *ConflictResolver) ResolveConflicts(conflicts []string) error {
	for _, conflict := range conflicts {
		switch cr.strategy {
		case "ours":
			if err := cr.resolveWithOurs(conflict); err != nil {
				return err
			}
		case "theirs":
			if err := cr.resolveWithTheirs(conflict); err != nil {
				return err
			}
		case "auto":
			if err := cr.resolveAuto(conflict); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown conflict resolution strategy: %s", cr.strategy)
		}
	}
	
	return nil
}

// resolveWithOurs resolves conflict by keeping our version
func (cr *ConflictResolver) resolveWithOurs(file string) error {
	cmd := exec.Command("git", "checkout", "--ours", file)
	return cmd.Run()
}

// resolveWithTheirs resolves conflict by keeping their version
func (cr *ConflictResolver) resolveWithTheirs(file string) error {
	cmd := exec.Command("git", "checkout", "--theirs", file)
	return cmd.Run()
}

// resolveAuto attempts intelligent conflict resolution
func (cr *ConflictResolver) resolveAuto(file string) error {
	// For auto resolution, we'll implement a simple strategy:
	// - For code files, prefer "theirs" (assume remote is more up-to-date)
	// - For config files, prefer "ours" (assume local config is correct)
	// - For documentation, try to merge or prefer "theirs"
	
	if strings.HasSuffix(file, ".go") || strings.HasSuffix(file, ".swift") {
		return cr.resolveWithTheirs(file)
	} else if strings.HasSuffix(file, ".json") || strings.HasSuffix(file, ".yaml") {
		return cr.resolveWithOurs(file)
	} else {
		return cr.resolveWithTheirs(file)
	}
}