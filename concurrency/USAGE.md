# Git Pipeline - Usage Guide

## Overview

The Git Pipeline implementation provides a production-quality framework for executing git operations in parallel across multiple repositories with full rollback support, conflict resolution, and progress tracking.

## Files Created

- `/home/user/claude-squad/concurrency/git_pipeline.go` - Main implementation (881 lines)
- `/home/user/claude-squad/concurrency/git_pipeline_example.go` - Usage examples (369 lines)

## Core Components

### 1. GitPipeline

Main orchestrator that manages stages and repositories.

```go
pipeline := NewGitPipeline(4) // 4 parallel workers
pipeline.AddStage(NewFetchStage("origin", nil))
pipeline.AddStage(NewCommitStage(false))
pipeline.AddStage(NewPushStage(false, true))

err := pipeline.AddRepository(
    "/path/to/repo",
    "main",
    "Commit message",
    "origin",
    ConflictOurs,
)

ctx := context.Background()
err = pipeline.Execute(ctx)
```

**Methods:**
- `AddStage(stage PipelineStage)` - Add a stage to the pipeline
- `AddRepository(...)` - Add a repository to process
- `Execute(ctx context.Context)` - Execute all stages across all repos
- `Rollback(ctx context.Context)` - Rollback all executed stages
- `GetProgressChannel()` - Get channel for progress updates
- `Close()` - Close progress channel

### 2. PipelineStage Interface

```go
type PipelineStage interface {
    Execute(ctx context.Context, repo *RepositoryContext) error
    Rollback(ctx context.Context, repo *RepositoryContext) error
    Name() string
}
```

### 3. Concrete Stages

#### FetchStage
Fetches changes from remote repository.

```go
stage := NewFetchStage("origin", []string{"main", "develop"})
```

#### MergeStage
Merges changes with conflict resolution support.

```go
resolver := NewConflictResolver()
stage := NewMergeStage("origin/main", resolver)
```

#### CommitStage
Creates commits with staged changes.

```go
stage := NewCommitStage(false) // allowEmpty = false
```

#### PushStage
Pushes changes to remote repository.

```go
stage := NewPushStage(false, true) // force=false, setUpstream=true
```

### 4. ParallelExecutor

Executes stages across repositories in parallel with configurable worker count.

```go
executor := NewParallelExecutor(8, progressChan)
err := executor.ExecuteStage(ctx, stage, repos)
```

**Features:**
- Semaphore-based worker pool
- Context cancellation support
- Error collection from all workers
- Progress updates for each operation

### 5. ConflictResolver

Handles merge conflicts with multiple strategies.

```go
resolver := NewConflictResolver()
err := resolver.Resolve(ctx, repo, ConflictOurs)
```

**Strategies:**
- `ConflictOurs` - Keep our changes
- `ConflictTheirs` - Keep their changes
- `ConflictManual` - Require manual resolution
- `ConflictAbort` - Abort on conflicts

### 6. ProgressReporter

Channel-based progress monitoring.

```go
reporter := NewProgressReporter(
    pipeline.GetProgressChannel(),
    func(update ProgressUpdate) {
        fmt.Printf("[%s] %s - %s: %s\n",
            update.Timestamp.Format("15:04:05"),
            update.RepoPath,
            update.StageName,
            update.Status,
        )
    },
)

reporter.Start()
defer reporter.Stop()
```

**Progress Statuses:**
- `StatusStarted`
- `StatusInProgress`
- `StatusCompleted`
- `StatusFailed`
- `StatusRolledBack`

## Common Use Cases

### 1. Fetch and Merge Multiple Repos

```go
pipeline := CreateFetchMergePipeline(4, "origin/main")

repos := []string{"/repo1", "/repo2", "/repo3"}
for _, repo := range repos {
    pipeline.AddRepository(repo, "main", "Merge", "origin", ConflictTheirs)
}

ctx := context.Background()
pipeline.Execute(ctx)
```

### 2. Complete Workflow (Fetch, Merge, Commit, Push)

```go
pipeline := CreateFullPipeline(8, "origin/main", false)
pipeline.AddRepository("/repo", "feature", "Update", "origin", ConflictOurs)

ctx := context.Background()
if err := pipeline.Execute(ctx); err != nil {
    pipeline.Rollback(ctx) // Auto-rollback on error
}
```

### 3. Commit and Push Only

```go
pipeline := CreateCommitPushPipeline(4, false)
pipeline.AddRepository("/repo", "main", "Changes", "origin", ConflictAbort)

ctx := context.Background()
pipeline.Execute(ctx)
```

### 4. Custom Pipeline with Progress Tracking

```go
pipeline := NewGitPipeline(4)
pipeline.AddStage(NewFetchStage("origin", nil))
pipeline.AddStage(NewCommitStage(false))
pipeline.AddStage(NewPushStage(false, true))

// Monitor progress
go func() {
    for update := range pipeline.GetProgressChannel() {
        log.Printf("[%s] %s - %s: %s",
            update.Timestamp.Format("15:04:05"),
            update.RepoPath,
            update.StageName,
            update.Status,
        )
    }
}()

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

pipeline.Execute(ctx)
pipeline.Close()
```

### 5. Atomic Batch Operations

```go
batch := NewBatchOperation()

batch.Add(func(ctx context.Context, repo *RepositoryContext) error {
    // Custom operation 1
    return nil
})

batch.Add(func(ctx context.Context, repo *RepositoryContext) error {
    // Custom operation 2
    return nil
})

// All operations execute atomically with automatic rollback on failure
batch.Execute(ctx, repoContext)
```

## Advanced Features

### Repository Validation

```go
if err := ValidateRepositoryState("/path/to/repo"); err != nil {
    log.Fatalf("Repository not ready: %v", err)
}
```

Checks for:
- Repository accessibility
- Merge/rebase state
- Valid HEAD reference

### Custom Stages

Create custom stages by implementing the `PipelineStage` interface:

```go
type MyCustomStage struct {
    config string
}

func (s *MyCustomStage) Name() string {
    return "my-custom-stage"
}

func (s *MyCustomStage) Execute(ctx context.Context, repo *RepositoryContext) error {
    // Store state for rollback
    repo.SetMetadata("original_state", someValue)

    // Perform operation
    return doSomething(repo)
}

func (s *MyCustomStage) Rollback(ctx context.Context, repo *RepositoryContext) error {
    // Restore original state
    originalState, _ := repo.GetMetadata("original_state")
    return restoreState(repo, originalState)
}
```

### Metadata Access

Thread-safe metadata storage in RepositoryContext:

```go
// Set metadata
repo.SetMetadata("key", value)

// Get metadata
value, ok := repo.GetMetadata("key")
```

### Error Handling

The pipeline automatically:
- Collects errors from parallel operations
- Combines multiple errors with detailed messages
- Triggers rollback on any stage failure
- Preserves error context with `fmt.Errorf` wrapping

## Design Patterns

### 1. Pipeline Pattern
Sequential stages executed in order, with parallel execution across repositories.

### 2. Strategy Pattern
Conflict resolution strategies encapsulated and interchangeable.

### 3. Observer Pattern
Progress updates published to channels for monitoring.

### 4. Command Pattern
Each stage encapsulates an operation with undo capability.

### 5. Worker Pool Pattern
Semaphore-based parallel execution with configurable workers.

## Performance Characteristics

- **Parallel Execution**: Multiple repositories processed simultaneously
- **Worker Pool**: Configurable concurrency (default: 4 workers)
- **Non-blocking Progress**: Channel-based updates don't block execution
- **Context Cancellation**: Immediate cancellation support
- **Memory Efficient**: Streaming progress updates, bounded channels

## Error Recovery

### Automatic Rollback
All stages implement rollback, executed in reverse order:

```go
if err := pipeline.Execute(ctx); err != nil {
    // Automatic rollback already performed
    log.Printf("Pipeline failed and rolled back: %v", err)
}
```

### Manual Rollback
```go
if err := pipeline.Rollback(ctx); err != nil {
    log.Fatalf("Rollback failed: %v", err)
}
```

## Testing Considerations

The implementation uses:
- Interfaces for easy mocking (PipelineStage)
- Dependency injection (ConflictResolver, ProgressReporter)
- Context for cancellation in tests
- Channel-based communication for testability

## Best Practices

1. **Always use context**: Enable cancellation and timeouts
2. **Monitor progress**: Set up progress reporters for visibility
3. **Validate repos first**: Use `ValidateRepositoryState()` before adding
4. **Handle errors**: Check errors from Execute() and Rollback()
5. **Close pipeline**: Always call `Close()` to clean up channels
6. **Choose strategy**: Select appropriate conflict resolution strategy
7. **Worker tuning**: Adjust worker count based on I/O vs CPU workload

## Integration with Existing Code

The implementation follows patterns from `/home/user/claude-squad/session/git/`:
- Uses `github.com/go-git/go-git/v5` library
- Follows error wrapping conventions (`fmt.Errorf` with `%w`)
- Uses `exec.Command` for git operations
- Thread-safe with sync.Mutex patterns
- Follows Go naming conventions

## Dependencies

Required packages (already in go.mod):
- `github.com/go-git/go-git/v5`
- `github.com/go-git/go-git/v5/plumbing`
- `github.com/go-git/go-git/v5/plumbing/object`

Standard library:
- `context`
- `fmt`
- `os/exec`
- `strings`
- `sync`
- `time`
