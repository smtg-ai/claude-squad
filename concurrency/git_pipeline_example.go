package concurrency

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
)

// ExampleBasicPipeline demonstrates basic pipeline usage
func ExampleBasicPipeline() {
	// Create a pipeline with 4 parallel workers
	pipeline := NewGitPipeline(4)

	// Add stages to the pipeline
	pipeline.AddStage(NewFetchStage("origin", nil))
	pipeline.AddStage(NewCommitStage(false))
	pipeline.AddStage(NewPushStage(false, true))

	// Add repositories to process
	err := pipeline.AddRepository(
		"/path/to/repo1",
		"main",
		"Update changes",
		"origin",
		ConflictAbort,
	)
	if err != nil {
		log.Fatalf("Failed to add repository: %v", err)
	}

	err = pipeline.AddRepository(
		"/path/to/repo2",
		"develop",
		"Sync changes",
		"origin",
		ConflictOurs,
	)
	if err != nil {
		log.Fatalf("Failed to add repository: %v", err)
	}

	// Set up progress monitoring
	go func() {
		for update := range pipeline.GetProgressChannel() {
			fmt.Printf("[%s] %s - %s: %s\n",
				update.Timestamp.Format("15:04:05"),
				update.RepoPath,
				update.StageName,
				update.Status,
			)
			if update.Error != nil {
				fmt.Printf("  Error: %v\n", update.Error)
			}
		}
	}()

	// Execute the pipeline
	ctx := context.Background()
	if err := pipeline.Execute(ctx); err != nil {
		log.Fatalf("Pipeline execution failed: %v", err)
	}

	pipeline.Close()
	fmt.Println("Pipeline completed successfully!")
}

// ExampleFetchMergePipeline demonstrates fetch and merge operations
func ExampleFetchMergePipeline() {
	// Create a pre-configured fetch-merge pipeline
	pipeline := CreateFetchMergePipeline(4, "origin/main")

	// Add repositories
	repos := []string{
		"/path/to/repo1",
		"/path/to/repo2",
		"/path/to/repo3",
	}

	for _, repoPath := range repos {
		if err := ValidateRepositoryState(repoPath); err != nil {
			log.Printf("Skipping %s: %v", repoPath, err)
			continue
		}

		err := pipeline.AddRepository(
			repoPath,
			"main",
			"Merge from origin/main",
			"origin",
			ConflictTheirs,
		)
		if err != nil {
			log.Printf("Failed to add %s: %v", repoPath, err)
			continue
		}
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := pipeline.Execute(ctx); err != nil {
		log.Fatalf("Pipeline failed: %v", err)
	}

	pipeline.Close()
}

// ExampleFullPipeline demonstrates complete workflow
func ExampleFullPipeline() {
	// Create full pipeline: fetch, merge, commit, push
	pipeline := CreateFullPipeline(8, "origin/main", false)

	// Set up detailed progress reporter
	reporter := NewProgressReporter(
		pipeline.GetProgressChannel(),
		func(update ProgressUpdate) {
			statusColor := "\033[32m" // Green
			if update.Status == PipelineStatusFailed {
				statusColor = "\033[31m" // Red
			} else if update.Status == PipelineStatusInProgress {
				statusColor = "\033[33m" // Yellow
			}

			fmt.Printf("%s[%s] %s - %s: %s\033[0m\n",
				statusColor,
				update.Timestamp.Format("15:04:05"),
				update.RepoPath,
				update.StageName,
				update.Status,
			)

			if update.Error != nil {
				fmt.Printf("  \033[31mError: %v\033[0m\n", update.Error)
			}
			if update.Message != "" {
				fmt.Printf("  Message: %s\n", update.Message)
			}
		},
	)

	reporter.Start()
	defer reporter.Stop()

	// Add repositories
	err := pipeline.AddRepository(
		"/path/to/repo",
		"feature-branch",
		"Automated sync from main",
		"origin",
		ConflictOurs,
	)
	if err != nil {
		log.Fatalf("Failed to add repository: %v", err)
	}

	// Execute
	ctx := context.Background()
	if err := pipeline.Execute(ctx); err != nil {
		log.Printf("Pipeline failed, rolling back: %v", err)
		if rollbackErr := pipeline.Rollback(ctx); rollbackErr != nil {
			log.Fatalf("Rollback also failed: %v", rollbackErr)
		}
		return
	}

	pipeline.Close()
	fmt.Println("All operations completed successfully!")
}

// ExampleBatchOperation demonstrates atomic batch operations
func ExampleBatchOperation() {
	pipeline := NewGitPipeline(4)

	// Create a batch operation
	batch := NewBatchOperation()

	// Add multiple operations to execute atomically
	batch.Add(func(ctx context.Context, repo *RepositoryContext) error {
		// Custom operation 1
		fmt.Printf("Executing operation 1 on %s\n", repo.RepoPath)
		return nil
	})

	batch.Add(func(ctx context.Context, repo *RepositoryContext) error {
		// Custom operation 2
		fmt.Printf("Executing operation 2 on %s\n", repo.RepoPath)
		return nil
	})

	// Add a custom stage that uses the batch operation
	customStage := &CustomBatchStage{batch: batch}
	pipeline.AddStage(customStage)

	err := pipeline.AddRepository(
		"/path/to/repo",
		"main",
		"Batch operations",
		"origin",
		ConflictAbort,
	)
	if err != nil {
		log.Fatalf("Failed to add repository: %v", err)
	}

	ctx := context.Background()
	if err := pipeline.Execute(ctx); err != nil {
		log.Fatalf("Batch execution failed: %v", err)
	}

	pipeline.Close()
}

// CustomBatchStage is a custom stage that uses BatchOperation
type CustomBatchStage struct {
	batch *BatchOperation
}

func (cbs *CustomBatchStage) Name() string {
	return "custom-batch"
}

func (cbs *CustomBatchStage) Execute(ctx context.Context, repo *RepositoryContext) error {
	return cbs.batch.Execute(ctx, repo)
}

func (cbs *CustomBatchStage) Rollback(ctx context.Context, repo *RepositoryContext) error {
	// Batch operation handles its own rollback
	return nil
}

// ExampleConflictResolution demonstrates different conflict resolution strategies
func ExampleConflictResolution() {
	// Pipeline with "ours" strategy
	pipeline1 := CreateFetchMergePipeline(4, "origin/main")
	pipeline1.AddRepository(
		"/path/to/repo",
		"main",
		"Merge with ours strategy",
		"origin",
		ConflictOurs, // Keep our changes in conflicts
	)

	// Pipeline with "theirs" strategy
	pipeline2 := CreateFetchMergePipeline(4, "origin/main")
	pipeline2.AddRepository(
		"/path/to/repo",
		"main",
		"Merge with theirs strategy",
		"origin",
		ConflictTheirs, // Keep their changes in conflicts
	)

	// Pipeline that aborts on conflicts
	pipeline3 := CreateFetchMergePipeline(4, "origin/main")
	pipeline3.AddRepository(
		"/path/to/repo",
		"main",
		"Merge with abort strategy",
		"origin",
		ConflictAbort, // Abort on any conflict
	)

	ctx := context.Background()

	// Try with ours strategy first
	if err := pipeline1.Execute(ctx); err != nil {
		log.Printf("Pipeline 1 failed: %v", err)
	}
	pipeline1.Close()
}

// ExampleCustomStage demonstrates creating a custom pipeline stage
func ExampleCustomStage() {
	pipeline := NewGitPipeline(4)

	// Add custom stage
	customStage := &TagStage{tagName: "release-v1.0.0", message: "Release version 1.0.0"}
	pipeline.AddStage(customStage)

	err := pipeline.AddRepository(
		"/path/to/repo",
		"main",
		"Create release tag",
		"origin",
		ConflictAbort,
	)
	if err != nil {
		log.Fatalf("Failed to add repository: %v", err)
	}

	ctx := context.Background()
	if err := pipeline.Execute(ctx); err != nil {
		log.Fatalf("Failed to create tag: %v", err)
	}

	pipeline.Close()
}

// TagStage is a custom stage that creates a git tag
type TagStage struct {
	tagName string
	message string
}

func (ts *TagStage) Name() string {
	return "tag"
}

func (ts *TagStage) Execute(ctx context.Context, repo *RepositoryContext) error {
	// Get current HEAD
	head, err := repo.Repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Create tag reference
	tagRef := fmt.Sprintf("refs/tags/%s", ts.tagName)
	ref := plumbing.NewHashReference(plumbing.ReferenceName(tagRef), head.Hash())

	err = repo.Repo.Storer.SetReference(ref)
	if err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}

	repo.SetMetadata("tag_created", ts.tagName)
	return nil
}

func (ts *TagStage) Rollback(ctx context.Context, repo *RepositoryContext) error {
	// Remove the tag
	tagRef := fmt.Sprintf("refs/tags/%s", ts.tagName)
	err := repo.Repo.Storer.RemoveReference(plumbing.ReferenceName(tagRef))
	if err != nil {
		return fmt.Errorf("failed to remove tag: %w", err)
	}
	return nil
}

// ExampleProgressTracking demonstrates detailed progress tracking
func ExampleGitPipelineProgressTracking() {
	pipeline := CreateFullPipeline(4, "origin/main", false)

	// Track progress with statistics
	stats := &PipelineStats{
		StartTime: time.Now(),
		Repos:     make(map[string]*RepoStats),
	}

	reporter := NewProgressReporter(
		pipeline.GetProgressChannel(),
		func(update ProgressUpdate) {
			stats.Update(update)
			stats.Print()
		},
	)

	reporter.Start()
	defer reporter.Stop()

	// Add repositories
	repos := []string{"/repo1", "/repo2", "/repo3"}
	for _, repo := range repos {
		pipeline.AddRepository(repo, "main", "Update", "origin", ConflictOurs)
	}

	ctx := context.Background()
	pipeline.Execute(ctx)
	pipeline.Close()

	stats.PrintFinal()
}

// PipelineStats tracks pipeline statistics
type PipelineStats struct {
	StartTime time.Time
	Repos     map[string]*RepoStats
	mu        sync.Mutex
}

// RepoStats tracks per-repository statistics
type RepoStats struct {
	Stages    map[string]ProgressStatus
	Completed int
	Failed    int
	Total     int
}

func (ps *PipelineStats) Update(update ProgressUpdate) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.Repos[update.RepoPath] == nil {
		ps.Repos[update.RepoPath] = &RepoStats{
			Stages: make(map[string]ProgressStatus),
		}
	}

	repo := ps.Repos[update.RepoPath]
	repo.Stages[update.StageName] = update.Status

	if update.Status == PipelineStatusCompleted {
		repo.Completed++
	} else if update.Status == PipelineStatusFailed {
		repo.Failed++
	}
}

func (ps *PipelineStats) Print() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	fmt.Printf("\n=== Pipeline Progress ===\n")
	fmt.Printf("Running for: %s\n", time.Since(ps.StartTime).Round(time.Second))
	for repoPath, stats := range ps.Repos {
		fmt.Printf("\n%s:\n", repoPath)
		for stage, status := range stats.Stages {
			fmt.Printf("  %s: %s\n", stage, status)
		}
		fmt.Printf("  Completed: %d, Failed: %d\n", stats.Completed, stats.Failed)
	}
	fmt.Println("========================")
}

func (ps *PipelineStats) PrintFinal() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	duration := time.Since(ps.StartTime)
	fmt.Printf("\n=== Final Statistics ===\n")
	fmt.Printf("Total Duration: %s\n", duration.Round(time.Second))
	fmt.Printf("Repositories Processed: %d\n", len(ps.Repos))

	totalCompleted := 0
	totalFailed := 0
	for _, stats := range ps.Repos {
		totalCompleted += stats.Completed
		totalFailed += stats.Failed
	}

	fmt.Printf("Total Completed: %d\n", totalCompleted)
	fmt.Printf("Total Failed: %d\n", totalFailed)
	fmt.Println("=======================")
}
