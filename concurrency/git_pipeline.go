package concurrency

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// PipelineStage represents a stage in the git operations pipeline
type PipelineStage interface {
	// Execute runs the stage and returns any error
	Execute(ctx context.Context, repo *RepositoryContext) error
	// Rollback reverses the changes made by this stage
	Rollback(ctx context.Context, repo *RepositoryContext) error
	// Name returns the name of the stage for logging/reporting
	Name() string
}

// RepositoryContext holds the context for a single repository operation
type RepositoryContext struct {
	// Path to the repository
	RepoPath string
	// Git repository instance
	Repo *git.Repository
	// Branch name for operations
	BranchName string
	// Commit message for commit stage
	CommitMessage string
	// Remote name (typically "origin")
	RemoteName string
	// Conflict resolution strategy
	ConflictStrategy ConflictStrategy
	// Metadata for tracking stage execution
	Metadata map[string]interface{}
	// Mutex for thread-safe metadata access
	mu sync.RWMutex
}

// GetMetadata retrieves a metadata value thread-safely
func (rc *RepositoryContext) GetMetadata(key string) (interface{}, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	val, ok := rc.Metadata[key]
	return val, ok
}

// SetMetadata sets a metadata value thread-safely
func (rc *RepositoryContext) SetMetadata(key string, value interface{}) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.Metadata == nil {
		rc.Metadata = make(map[string]interface{})
	}
	rc.Metadata[key] = value
}

// ProgressUpdate represents a progress update from the pipeline
type ProgressUpdate struct {
	// Repository path
	RepoPath string
	// Stage name
	StageName string
	// Status of the operation
	Status ProgressStatus
	// Error if any
	Error error
	// Timestamp of the update
	Timestamp time.Time
	// Additional message
	Message string
}

// ProgressStatus represents the status of a pipeline operation
type ProgressStatus int

const (
	PipelineStatusStarted ProgressStatus = iota
	PipelineStatusInProgress
	PipelineStatusCompleted
	PipelineStatusFailed
	PipelineStatusRolledBack
)

func (ps ProgressStatus) String() string {
	switch ps {
	case PipelineStatusStarted:
		return "started"
	case PipelineStatusInProgress:
		return "in_progress"
	case PipelineStatusCompleted:
		return "completed"
	case PipelineStatusFailed:
		return "failed"
	case PipelineStatusRolledBack:
		return "rolled_back"
	default:
		return "unknown"
	}
}

// ConflictStrategy defines how to handle merge conflicts
type ConflictStrategy int

const (
	// ConflictOurs keeps our changes in case of conflicts
	ConflictOurs ConflictStrategy = iota
	// ConflictTheirs keeps their changes in case of conflicts
	ConflictTheirs
	// ConflictManual requires manual resolution
	ConflictManual
	// ConflictAbort aborts on any conflict
	ConflictAbort
)

// GitPipeline manages a series of git operations across multiple repositories
type GitPipeline struct {
	stages           []PipelineStage
	repositories     []*RepositoryContext
	progressChan     chan ProgressUpdate
	parallelExecutor *ParallelExecutor
	mu               sync.RWMutex
}

// NewGitPipeline creates a new git pipeline
func NewGitPipeline(maxWorkers int) *GitPipeline {
	progressChan := make(chan ProgressUpdate, 100)
	return &GitPipeline{
		stages:           make([]PipelineStage, 0),
		repositories:     make([]*RepositoryContext, 0),
		progressChan:     progressChan,
		parallelExecutor: NewParallelExecutor(maxWorkers, progressChan),
	}
}

// AddStage adds a stage to the pipeline
func (gp *GitPipeline) AddStage(stage PipelineStage) {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	gp.stages = append(gp.stages, stage)
}

// AddRepository adds a repository to the pipeline
func (gp *GitPipeline) AddRepository(repoPath, branchName, commitMessage, remoteName string, strategy ConflictStrategy) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository %s: %w", repoPath, err)
	}

	if remoteName == "" {
		remoteName = "origin"
	}

	repoCtx := &RepositoryContext{
		RepoPath:         repoPath,
		Repo:             repo,
		BranchName:       branchName,
		CommitMessage:    commitMessage,
		RemoteName:       remoteName,
		ConflictStrategy: strategy,
		Metadata:         make(map[string]interface{}),
	}

	gp.mu.Lock()
	defer gp.mu.Unlock()
	gp.repositories = append(gp.repositories, repoCtx)
	return nil
}

// Execute runs all stages across all repositories in parallel
func (gp *GitPipeline) Execute(ctx context.Context) error {
	gp.mu.RLock()
	stages := gp.stages
	repos := gp.repositories
	gp.mu.RUnlock()

	if len(stages) == 0 {
		return fmt.Errorf("no stages defined in pipeline")
	}

	if len(repos) == 0 {
		return fmt.Errorf("no repositories defined in pipeline")
	}

	// Execute each stage sequentially, but repositories in parallel
	for _, stage := range stages {
		if err := gp.parallelExecutor.ExecuteStage(ctx, stage, repos); err != nil {
			// Rollback all stages executed so far
			rollbackErr := gp.rollbackStages(ctx, stages, repos)
			if rollbackErr != nil {
				return fmt.Errorf("execution failed: %w, rollback also failed: %v", err, rollbackErr)
			}
			return fmt.Errorf("execution failed and rolled back: %w", err)
		}
	}

	return nil
}

// Rollback rolls back all executed stages
func (gp *GitPipeline) Rollback(ctx context.Context) error {
	gp.mu.RLock()
	stages := gp.stages
	repos := gp.repositories
	gp.mu.RUnlock()

	return gp.rollbackStages(ctx, stages, repos)
}

// rollbackStages performs the actual rollback
func (gp *GitPipeline) rollbackStages(ctx context.Context, stages []PipelineStage, repos []*RepositoryContext) error {
	// Rollback in reverse order
	var errors []error
	for i := len(stages) - 1; i >= 0; i-- {
		stage := stages[i]
		for _, repo := range repos {
			select {
			case gp.progressChan <- ProgressUpdate{
				RepoPath:  repo.RepoPath,
				StageName: stage.Name(),
				Status:    PipelineStatusRolledBack,
				Timestamp: time.Now(),
				Message:   "Rolling back",
			}:
			case <-ctx.Done():
				// Context cancelled, continue rollback without reporting
			default:
				// Progress channel full, continue rollback without blocking
			}

			if err := stage.Rollback(ctx, repo); err != nil {
				errors = append(errors, fmt.Errorf("rollback failed for %s at stage %s: %w",
					repo.RepoPath, stage.Name(), err))
			}
		}
	}

	if len(errors) > 0 {
		return combinePipelineErrors(errors)
	}
	return nil
}

// GetProgressChannel returns the progress update channel
func (gp *GitPipeline) GetProgressChannel() <-chan ProgressUpdate {
	return gp.progressChan
}

// Close closes the progress channel
func (gp *GitPipeline) Close() {
	close(gp.progressChan)
}

// ParallelExecutor executes git operations in parallel across repositories
type ParallelExecutor struct {
	maxWorkers   int
	progressChan chan ProgressUpdate
}

// NewParallelExecutor creates a new parallel executor
func NewParallelExecutor(maxWorkers int, progressChan chan ProgressUpdate) *ParallelExecutor {
	if maxWorkers <= 0 {
		maxWorkers = 4 // Default to 4 workers
	}
	return &ParallelExecutor{
		maxWorkers:   maxWorkers,
		progressChan: progressChan,
	}
}

// ExecuteStage executes a stage across all repositories in parallel
func (pe *ParallelExecutor) ExecuteStage(ctx context.Context, stage PipelineStage, repos []*RepositoryContext) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(repos))
	semaphore := make(chan struct{}, pe.maxWorkers)

	for _, repo := range repos {
		wg.Add(1)
		go func(r *RepositoryContext) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}

			// Send started update
			select {
			case pe.progressChan <- ProgressUpdate{
				RepoPath:  r.RepoPath,
				StageName: stage.Name(),
				Status:    PipelineStatusStarted,
				Timestamp: time.Now(),
			}:
			case <-ctx.Done():
				// Don't block if context cancelled
			default:
				// Progress channel full, continue without blocking
			}

			// Execute stage
			if err := stage.Execute(ctx, r); err != nil {
				select {
				case pe.progressChan <- ProgressUpdate{
					RepoPath:  r.RepoPath,
					StageName: stage.Name(),
					Status:    PipelineStatusFailed,
					Error:     err,
					Timestamp: time.Now(),
				}:
				case <-ctx.Done():
				default:
				}
				errChan <- fmt.Errorf("stage %s failed for %s: %w", stage.Name(), r.RepoPath, err)
				return
			}

			// Send completed update
			select {
			case pe.progressChan <- ProgressUpdate{
				RepoPath:  r.RepoPath,
				StageName: stage.Name(),
				Status:    PipelineStatusCompleted,
				Timestamp: time.Now(),
			}:
			case <-ctx.Done():
			default:
			}
		}(repo)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return combinePipelineErrors(errors)
	}

	return nil
}

// FetchStage fetches changes from remote
type FetchStage struct {
	remoteName string
	refSpecs   []string
}

// NewFetchStage creates a new fetch stage
func NewFetchStage(remoteName string, refSpecs []string) *FetchStage {
	if remoteName == "" {
		remoteName = "origin"
	}
	return &FetchStage{
		remoteName: remoteName,
		refSpecs:   refSpecs,
	}
}

func (fs *FetchStage) Name() string {
	return "fetch"
}

func (fs *FetchStage) Execute(ctx context.Context, repo *RepositoryContext) error {
	// Use git command for reliable fetch
	args := []string{"-C", repo.RepoPath, "fetch", fs.remoteName}
	if len(fs.refSpecs) > 0 {
		args = append(args, fs.refSpecs...)
	} else {
		args = append(args, "--all")
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fetch failed: %s (%w)", output, err)
	}

	repo.SetMetadata("fetch_completed", true)
	return nil
}

func (fs *FetchStage) Rollback(ctx context.Context, repo *RepositoryContext) error {
	// Fetch is read-only, no rollback needed
	return nil
}

// MergeStage merges changes from a branch
type MergeStage struct {
	sourceBranch     string
	conflictResolver *ConflictResolver
}

// NewMergeStage creates a new merge stage
func NewMergeStage(sourceBranch string, conflictResolver *ConflictResolver) *MergeStage {
	return &MergeStage{
		sourceBranch:     sourceBranch,
		conflictResolver: conflictResolver,
	}
}

func (ms *MergeStage) Name() string {
	return "merge"
}

func (ms *MergeStage) Execute(ctx context.Context, repo *RepositoryContext) error {
	// Get current HEAD before merge for rollback
	head, err := repo.Repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}
	repo.SetMetadata("pre_merge_head", head.Hash().String())

	// Perform merge using git command for better conflict handling
	cmd := exec.CommandContext(ctx, "git", "-C", repo.RepoPath, "merge", ms.sourceBranch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's a merge conflict
		if strings.Contains(string(output), "CONFLICT") {
			// Handle conflict based on strategy
			if ms.conflictResolver != nil {
				return ms.conflictResolver.Resolve(ctx, repo, repo.ConflictStrategy)
			}
			return fmt.Errorf("merge conflict detected and no resolver configured: %s", output)
		}
		return fmt.Errorf("merge failed: %s (%w)", output, err)
	}

	repo.SetMetadata("merge_completed", true)
	return nil
}

func (ms *MergeStage) Rollback(ctx context.Context, repo *RepositoryContext) error {
	// Reset to pre-merge state
	preMergeHead, ok := repo.GetMetadata("pre_merge_head")
	if !ok {
		return fmt.Errorf("no pre-merge HEAD found for rollback")
	}

	cmd := exec.CommandContext(ctx, "git", "-C", repo.RepoPath, "reset", "--hard", preMergeHead.(string))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to rollback merge: %s (%w)", output, err)
	}

	return nil
}

// CommitStage creates a commit with staged changes
type CommitStage struct {
	allowEmpty bool
}

// NewCommitStage creates a new commit stage
func NewCommitStage(allowEmpty bool) *CommitStage {
	return &CommitStage{
		allowEmpty: allowEmpty,
	}
}

func (cs *CommitStage) Name() string {
	return "commit"
}

func (cs *CommitStage) Execute(ctx context.Context, repo *RepositoryContext) error {
	// Get current HEAD for rollback
	head, err := repo.Repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}
	repo.SetMetadata("pre_commit_head", head.Hash().String())

	// Check if there are changes to commit
	w, err := repo.Repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := w.Status()
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	if status.IsClean() && !cs.allowEmpty {
		repo.SetMetadata("commit_skipped", true)
		return nil
	}

	// Stage all changes
	cmd := exec.CommandContext(ctx, "git", "-C", repo.RepoPath, "add", ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stage changes: %s (%w)", output, err)
	}

	// Create commit
	args := []string{"-C", repo.RepoPath, "commit", "-m", repo.CommitMessage}
	if cs.allowEmpty {
		args = append(args, "--allow-empty")
	}

	cmd = exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to commit: %s (%w)", output, err)
	}

	// Get new commit hash
	newHead, err := repo.Repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get new HEAD: %w", err)
	}
	repo.SetMetadata("commit_hash", newHead.Hash().String())
	repo.SetMetadata("commit_completed", true)

	return nil
}

func (cs *CommitStage) Rollback(ctx context.Context, repo *RepositoryContext) error {
	// Check if commit was skipped
	if skipped, ok := repo.GetMetadata("commit_skipped"); ok && skipped.(bool) {
		return nil
	}

	// Reset to pre-commit state
	preCommitHead, ok := repo.GetMetadata("pre_commit_head")
	if !ok {
		return fmt.Errorf("no pre-commit HEAD found for rollback")
	}

	cmd := exec.CommandContext(ctx, "git", "-C", repo.RepoPath, "reset", "--hard", preCommitHead.(string))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to rollback commit: %s (%w)", output, err)
	}

	return nil
}

// PushStage pushes changes to remote
type PushStage struct {
	force       bool
	setUpstream bool
}

// NewPushStage creates a new push stage
func NewPushStage(force, setUpstream bool) *PushStage {
	return &PushStage{
		force:       force,
		setUpstream: setUpstream,
	}
}

func (ps *PushStage) Name() string {
	return "push"
}

func (ps *PushStage) Execute(ctx context.Context, repo *RepositoryContext) error {
	args := []string{"-C", repo.RepoPath, "push"}

	if ps.setUpstream {
		args = append(args, "-u", repo.RemoteName, repo.BranchName)
	}

	if ps.force {
		args = append(args, "--force")
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("push failed: %s (%w)", output, err)
	}

	repo.SetMetadata("push_completed", true)
	return nil
}

func (ps *PushStage) Rollback(ctx context.Context, repo *RepositoryContext) error {
	// Rollback push by force pushing the previous commit
	preCommitHead, ok := repo.GetMetadata("pre_commit_head")
	if !ok {
		// If no commit was made, nothing to rollback
		return nil
	}

	cmd := exec.CommandContext(ctx, "git", "-C", repo.RepoPath, "push", "--force",
		repo.RemoteName, preCommitHead.(string)+":"+repo.BranchName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to rollback push: %s (%w)", output, err)
	}

	return nil
}

// ConflictResolver handles merge conflicts
type ConflictResolver struct{}

// NewConflictResolver creates a new conflict resolver
func NewConflictResolver() *ConflictResolver {
	return &ConflictResolver{}
}

// Resolve resolves conflicts based on the given strategy
func (cr *ConflictResolver) Resolve(ctx context.Context, repo *RepositoryContext, strategy ConflictStrategy) error {
	switch strategy {
	case ConflictOurs:
		return cr.resolveWithOurs(ctx, repo)
	case ConflictTheirs:
		return cr.resolveWithTheirs(ctx, repo)
	case ConflictAbort:
		return cr.abortMerge(ctx, repo)
	case ConflictManual:
		return fmt.Errorf("manual conflict resolution required for %s", repo.RepoPath)
	default:
		return fmt.Errorf("unknown conflict strategy: %d", strategy)
	}
}

// resolveWithOurs resolves conflicts by keeping our changes
func (cr *ConflictResolver) resolveWithOurs(ctx context.Context, repo *RepositoryContext) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repo.RepoPath, "checkout", "--ours", ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to resolve with ours: %s (%w)", output, err)
	}

	// Stage resolved files
	cmd = exec.CommandContext(ctx, "git", "-C", repo.RepoPath, "add", ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stage resolved files: %s (%w)", output, err)
	}

	// Complete merge
	cmd = exec.CommandContext(ctx, "git", "-C", repo.RepoPath, "commit", "--no-edit")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to complete merge: %s (%w)", output, err)
	}

	return nil
}

// resolveWithTheirs resolves conflicts by keeping their changes
func (cr *ConflictResolver) resolveWithTheirs(ctx context.Context, repo *RepositoryContext) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repo.RepoPath, "checkout", "--theirs", ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to resolve with theirs: %s (%w)", output, err)
	}

	// Stage resolved files
	cmd = exec.CommandContext(ctx, "git", "-C", repo.RepoPath, "add", ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stage resolved files: %s (%w)", output, err)
	}

	// Complete merge
	cmd = exec.CommandContext(ctx, "git", "-C", repo.RepoPath, "commit", "--no-edit")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to complete merge: %s (%w)", output, err)
	}

	return nil
}

// abortMerge aborts the merge operation
func (cr *ConflictResolver) abortMerge(ctx context.Context, repo *RepositoryContext) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repo.RepoPath, "merge", "--abort")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to abort merge: %s (%w)", output, err)
	}
	return fmt.Errorf("merge aborted due to conflicts")
}

// ProgressReporter monitors and reports pipeline progress
type ProgressReporter struct {
	progressChan <-chan ProgressUpdate
	stopChan     chan struct{}
	callback     func(ProgressUpdate)
	wg           sync.WaitGroup
}

// NewProgressReporter creates a new progress reporter
func NewProgressReporter(progressChan <-chan ProgressUpdate, callback func(ProgressUpdate)) *ProgressReporter {
	return &ProgressReporter{
		progressChan: progressChan,
		stopChan:     make(chan struct{}),
		callback:     callback,
	}
}

// Start begins monitoring progress updates
func (pr *ProgressReporter) Start() {
	pr.wg.Add(1)
	go func() {
		defer pr.wg.Done()
		for {
			select {
			case update, ok := <-pr.progressChan:
				if !ok {
					return
				}
				if pr.callback != nil {
					pr.callback(update)
				}
			case <-pr.stopChan:
				return
			}
		}
	}()
}

// Stop stops the progress reporter
func (pr *ProgressReporter) Stop() {
	close(pr.stopChan)
	pr.wg.Wait()
}

// BatchOperation represents a batch of git operations to be executed atomically
type BatchOperation struct {
	operations []func(context.Context, *RepositoryContext) error
	mu         sync.Mutex
}

// NewBatchOperation creates a new batch operation
func NewBatchOperation() *BatchOperation {
	return &BatchOperation{
		operations: make([]func(context.Context, *RepositoryContext) error, 0),
	}
}

// Add adds an operation to the batch
func (bo *BatchOperation) Add(op func(context.Context, *RepositoryContext) error) {
	bo.mu.Lock()
	defer bo.mu.Unlock()
	bo.operations = append(bo.operations, op)
}

// Execute executes all operations in the batch atomically
func (bo *BatchOperation) Execute(ctx context.Context, repo *RepositoryContext) error {
	bo.mu.Lock()
	operations := make([]func(context.Context, *RepositoryContext) error, len(bo.operations))
	copy(operations, bo.operations)
	bo.mu.Unlock()

	// Get current HEAD for potential rollback
	head, err := repo.Repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD for batch operation: %w", err)
	}
	originalHead := head.Hash().String()

	// Execute all operations
	for i, op := range operations {
		if err := op(ctx, repo); err != nil {
			// Rollback to original state
			rollbackErr := bo.rollback(ctx, repo, originalHead)
			if rollbackErr != nil {
				return fmt.Errorf("operation %d failed: %w, rollback also failed: %v", i, err, rollbackErr)
			}
			return fmt.Errorf("operation %d failed and rolled back: %w", i, err)
		}
	}

	return nil
}

// rollback rolls back the repository to the original state
func (bo *BatchOperation) rollback(ctx context.Context, repo *RepositoryContext, originalHead string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repo.RepoPath, "reset", "--hard", originalHead)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to rollback batch operation: %s (%w)", output, err)
	}
	return nil
}

// combinePipelineErrors combines multiple errors into a single error
func combinePipelineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}

	var sb strings.Builder
	sb.WriteString("multiple errors occurred:")
	for i, err := range errs {
		sb.WriteString(fmt.Sprintf("\n  %d. %v", i+1, err))
	}
	return fmt.Errorf("%s", sb.String())
}

// Helper functions for common pipeline configurations

// CreateFetchMergePipeline creates a pipeline for fetch and merge operations
func CreateFetchMergePipeline(maxWorkers int, sourceBranch string) *GitPipeline {
	pipeline := NewGitPipeline(maxWorkers)
	pipeline.AddStage(NewFetchStage("origin", nil))
	pipeline.AddStage(NewMergeStage(sourceBranch, NewConflictResolver()))
	return pipeline
}

// CreateFullPipeline creates a complete pipeline: fetch, merge, commit, push
func CreateFullPipeline(maxWorkers int, sourceBranch string, allowEmpty bool) *GitPipeline {
	pipeline := NewGitPipeline(maxWorkers)
	pipeline.AddStage(NewFetchStage("origin", nil))
	pipeline.AddStage(NewMergeStage(sourceBranch, NewConflictResolver()))
	pipeline.AddStage(NewCommitStage(allowEmpty))
	pipeline.AddStage(NewPushStage(false, true))
	return pipeline
}

// CreateCommitPushPipeline creates a pipeline for commit and push operations
func CreateCommitPushPipeline(maxWorkers int, allowEmpty bool) *GitPipeline {
	pipeline := NewGitPipeline(maxWorkers)
	pipeline.AddStage(NewCommitStage(allowEmpty))
	pipeline.AddStage(NewPushStage(false, true))
	return pipeline
}

// ValidateRepositoryState validates that a repository is in a good state for operations
func ValidateRepositoryState(repoPath string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// Check if repository is in a merge state
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "-q", "--verify", "MERGE_HEAD")
	if err := cmd.Run(); err == nil {
		return fmt.Errorf("repository is in a merge state, please resolve or abort the merge first")
	}

	// Check if repository is in a rebase state
	cmd = exec.Command("git", "-C", repoPath, "rev-parse", "-q", "--verify", "REBASE_HEAD")
	if err := cmd.Run(); err == nil {
		return fmt.Errorf("repository is in a rebase state, please resolve or abort the rebase first")
	}

	// Ensure we can get HEAD
	_, err = repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	return nil
}

// GetBranchCommits returns the commits between two branches
func GetBranchCommits(repo *git.Repository, baseBranch, targetBranch string) ([]*object.Commit, error) {
	baseRef, err := repo.Reference(plumbing.NewBranchReferenceName(baseBranch), false)
	if err != nil {
		return nil, fmt.Errorf("failed to get base branch reference: %w", err)
	}

	targetRef, err := repo.Reference(plumbing.NewBranchReferenceName(targetBranch), false)
	if err != nil {
		return nil, fmt.Errorf("failed to get target branch reference: %w", err)
	}

	commits := make([]*object.Commit, 0)
	targetCommit, err := repo.CommitObject(targetRef.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get target commit: %w", err)
	}

	baseHash := baseRef.Hash()
	iter := object.NewCommitPreorderIter(targetCommit, nil, nil)
	defer iter.Close()

	for {
		commit, err := iter.Next()
		if err != nil {
			break
		}

		if commit.Hash == baseHash {
			break
		}

		commits = append(commits, commit)
	}

	return commits, nil
}
