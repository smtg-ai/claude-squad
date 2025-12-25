# Batch Operations API

A production-quality batch operations system for managing multiple claude-squad instances with transaction-like semantics, parallel execution, progress tracking, and comprehensive error handling.

## Table of Contents

- [Overview](#overview)
- [Core Concepts](#core-concepts)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [API Reference](#api-reference)
- [Examples](#examples)
- [Best Practices](#best-practices)
- [Error Handling](#error-handling)

## Overview

The Batch Operations API provides a robust framework for executing operations across multiple claude-squad instances with:

- **Transaction-like semantics**: All-or-nothing execution with automatic rollback
- **Parallel execution**: Configurable concurrency for optimal performance
- **Progress tracking**: Real-time progress callbacks and metrics
- **Partial failure handling**: Graceful handling of partial failures with retry capabilities
- **Operation chaining**: Sequential execution of multiple operations
- **Context-aware**: Full support for context cancellation and timeouts

## Core Concepts

### Operation Interface

All batch operations implement the `Operation` interface:

```go
type Operation interface {
    Execute(ctx context.Context, instance *session.Instance) error
    Rollback(ctx context.Context, instance *session.Instance) error
    Validate(instance *session.Instance) error
    Name() string
}
```

### BatchExecutor

The `BatchExecutor` manages parallel execution of operations across multiple instances:

```go
executor := NewBatchExecutor(maxConcurrency)
executor.SetTimeout(5 * time.Minute)
result := executor.Execute(ctx, instances, operation, tracker)
```

### PartialResult

Operations return a `PartialResult` containing detailed information about successes and failures:

```go
type PartialResult struct {
    Successes []*OperationResult
    Failures  []*OperationResult
    Total     int
    StartTime time.Time
    EndTime   time.Time
}
```

### ProgressTracker

Track operation progress in real-time with callbacks:

```go
tracker := NewProgressTracker(totalOperations)
tracker.OnProgress(func(result *OperationResult, completed, total int) {
    fmt.Printf("Progress: %d/%d (%.1f%%)\n",
        completed, total, float64(completed)/float64(total)*100)
})
```

## Installation

The batch operations package is part of the claude-squad concurrency module:

```bash
import "claude-squad/concurrency"
```

## Quick Start

### Basic Batch Kill

```go
package main

import (
    "context"
    "fmt"

    "claude-squad/concurrency"
    "claude-squad/session"
)

func main() {
    // Create instances
    instances := []*session.Instance{
        {Title: "instance-1"},
        {Title: "instance-2"},
        {Title: "instance-3"},
    }

    // Create batch executor
    executor := concurrency.NewBatchExecutor(5)

    // Execute kill operation
    ctx := context.Background()
    result := executor.Execute(ctx, instances,
        concurrency.NewBatchKillOperation(), nil)

    fmt.Printf("Killed %d instances\n", len(result.Successes))
}
```

### Batch Operation with Progress Tracking

```go
tracker := concurrency.NewProgressTracker(len(instances))
tracker.OnProgress(func(result *concurrency.OperationResult, completed, total int) {
    if result.Error != nil {
        fmt.Printf("Failed: %s on %s\n",
            result.Operation.Name(), result.Instance.Title)
    }
})

result := executor.Execute(ctx, instances, operation, tracker)
```

### Transaction with Rollback

```go
// If any operation fails, all successful operations are rolled back
result, err := executor.ExecuteWithRollback(ctx, instances,
    concurrency.NewBatchPauseOperation(), tracker)

if err != nil {
    fmt.Printf("Operation failed and rolled back: %v\n", err)
}
```

## API Reference

### Built-in Operations

#### BatchKillOperation

Kills multiple instances in parallel.

```go
op := concurrency.NewBatchKillOperation()
result := executor.Execute(ctx, instances, op, tracker)
```

**Validation**: Instance must be started
**Rollback**: Not supported (terminal operation)

#### BatchPauseOperation

Pauses multiple instances (commits changes, removes worktree, preserves branch).

```go
op := concurrency.NewBatchPauseOperation()
result := executor.Execute(ctx, instances, op, tracker)
```

**Validation**: Instance must be started and not already paused
**Rollback**: Calls Resume() to restore instance

#### BatchResumeOperation

Resumes multiple paused instances.

```go
op := concurrency.NewBatchResumeOperation()
result := executor.Execute(ctx, instances, op, tracker)
```

**Validation**: Instance must be started and paused
**Rollback**: Calls Pause() to re-pause instance

#### BatchStartOperation

Starts multiple instances.

```go
op := concurrency.NewBatchStartOperation(firstTimeSetup)
result := executor.Execute(ctx, instances, op, tracker)
```

**Validation**: Instance must not be already started, title must not be empty
**Rollback**: Calls Kill() to clean up

#### BatchPromptOperation

Sends a prompt to multiple instances.

```go
op := concurrency.NewBatchPromptOperation("Fix all bugs")
result := executor.Execute(ctx, instances, op, tracker)
```

**Validation**: Instance must be started and not paused, prompt must not be empty
**Rollback**: Not supported (cannot unsend prompt)

### Advanced Operations

#### CompositeOperation

Combines multiple operations into a single operation.

```go
composite := concurrency.NewCompositeOperation(
    "PauseAndPrompt",
    concurrency.NewBatchPauseOperation(),
    concurrency.NewBatchPromptOperation("Review changes"),
)
```

#### ConditionalOperation

Executes an operation only if a condition is met.

```go
conditional := concurrency.NewConditionalOperation(
    concurrency.NewBatchPauseOperation(),
    func(i *session.Instance) bool {
        return i.Status == session.Running
    },
)
```

#### RetryOperation

Wraps an operation with retry logic.

```go
retry := concurrency.NewRetryOperation(
    concurrency.NewBatchPauseOperation(),
    3,              // max retries
    1*time.Second,  // delay between retries
)
```

### TransactionManager

Simplified interface for transactional operations.

```go
tm := concurrency.NewTransactionManager(maxConcurrency)
tm.SetTimeout(5 * time.Minute)

err := tm.Execute(ctx, instances, operation, tracker)
if err != nil {
    // All operations were rolled back
}
```

### OperationChain

Execute multiple operations sequentially.

```go
chain := concurrency.NewOperationChain(stopOnFailure)
chain.
    Add(concurrency.NewBatchPauseOperation()).
    Add(concurrency.NewBatchPromptOperation("Fix bugs")).
    Add(concurrency.NewBatchResumeOperation())

results := chain.Execute(ctx, instances, executor, tracker)
```

## Examples

### Example 1: Pause All Instances with Progress Tracking

```go
instances := manager.GetAllInstances()

executor := concurrency.NewBatchExecutor(10)
tracker := concurrency.NewProgressTracker(len(instances))

tracker.OnProgress(func(result *concurrency.OperationResult, completed, total int) {
    percentage := float64(completed) / float64(total) * 100
    fmt.Printf("[%.1f%%] %s: %s\n", percentage,
        result.Instance.Title, result.Operation.Name())
})

result := executor.Execute(context.Background(), instances,
    concurrency.NewBatchPauseOperation(), tracker)

fmt.Printf("\nResult: %d succeeded, %d failed (%.1f%% success rate)\n",
    len(result.Successes), len(result.Failures), result.SuccessRate())
```

### Example 2: Partial Failure Handling

```go
result := executor.Execute(ctx, instances, operation, nil)

if !result.AllSucceeded() && len(result.Successes) > 0 {
    // Handle partial failure
    fmt.Printf("Partial failure: %d/%d succeeded\n",
        len(result.Successes), result.Total)

    // Retry only failed instances
    failedInstances := make([]*session.Instance, len(result.Failures))
    for i, failure := range result.Failures {
        failedInstances[i] = failure.Instance
    }

    retryResult := executor.Execute(ctx, failedInstances, operation, nil)
}
```

### Example 3: Complex Workflow with Chaining

```go
// Create a workflow that pauses, sends prompt, waits, and resumes
chain := concurrency.NewOperationChain(true) // stop on failure

chain.
    Add(concurrency.NewBatchPauseOperation()).
    Add(concurrency.NewBatchPromptOperation("Analyze codebase")).
    Add(concurrency.NewBatchResumeOperation())

executor := concurrency.NewBatchExecutor(5)
tracker := concurrency.NewProgressTracker(len(instances) * 3)

results := chain.Execute(context.Background(), instances, executor, tracker)

for i, result := range results {
    fmt.Printf("Step %d: %d succeeded, %d failed\n",
        i+1, len(result.Successes), len(result.Failures))
}
```

### Example 4: Context Cancellation

```go
// Set a timeout for the batch operation
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result := executor.Execute(ctx, instances, operation, tracker)

// Check which operations were cancelled
for _, failure := range result.Failures {
    if failure.Error == context.DeadlineExceeded {
        fmt.Printf("Operation timed out on %s\n", failure.Instance.Title)
    }
}
```

### Example 5: Custom Operation

```go
type CustomHealthCheckOperation struct {
    maxResponseTime time.Duration
}

func (op *CustomHealthCheckOperation) Name() string {
    return "HealthCheck"
}

func (op *CustomHealthCheckOperation) Validate(instance *session.Instance) error {
    if !instance.Started() {
        return fmt.Errorf("instance not started")
    }
    return nil
}

func (op *CustomHealthCheckOperation) Execute(ctx context.Context,
    instance *session.Instance) error {

    // Custom health check logic
    if !instance.TmuxAlive() {
        return fmt.Errorf("tmux session is not alive")
    }

    // Check if instance is responsive
    _, err := instance.Preview()
    return err
}

func (op *CustomHealthCheckOperation) Rollback(ctx context.Context,
    instance *session.Instance) error {
    return nil // Health checks don't need rollback
}

// Use the custom operation
healthCheck := &CustomHealthCheckOperation{
    maxResponseTime: 5 * time.Second,
}
result := executor.Execute(ctx, instances, healthCheck, tracker)
```

## Best Practices

### 1. Set Appropriate Concurrency Limits

```go
// Don't overload the system
executor := concurrency.NewBatchExecutor(runtime.NumCPU() * 2)
```

### 2. Always Use Context for Long-Running Operations

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

result := executor.Execute(ctx, instances, operation, tracker)
```

### 3. Handle Partial Failures Gracefully

```go
if !result.AllSucceeded() {
    // Log failures
    for _, failure := range result.Failures {
        log.Printf("Failed on %s: %v", failure.Instance.Title, failure.Error)
    }

    // Decide whether to retry or continue
    if len(result.Successes) > len(result.Failures) {
        // Majority succeeded, continue
    }
}
```

### 4. Use Progress Tracking for User Feedback

```go
tracker := concurrency.NewProgressTracker(len(instances))
tracker.OnProgress(func(result *concurrency.OperationResult, completed, total int) {
    // Update UI or log progress
    updateProgressBar(completed, total)
})
```

### 5. Validate Before Executing

All operations automatically call `Validate()` before `Execute()`, but you can pre-validate:

```go
for _, instance := range instances {
    if err := operation.Validate(instance); err != nil {
        log.Printf("Instance %s will fail: %v", instance.Title, err)
    }
}
```

### 6. Use Transactions for Critical Operations

```go
// Use transaction manager for operations that must be atomic
tm := concurrency.NewTransactionManager(5)
err := tm.Execute(ctx, instances, operation, tracker)
// If err != nil, all operations were automatically rolled back
```

### 7. Set Reasonable Timeouts

```go
executor.SetTimeout(2 * time.Minute) // Per-operation timeout
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
defer cancel() // Overall batch timeout
```

## Error Handling

### Operation Errors

Each operation result includes detailed error information:

```go
for _, failure := range result.Failures {
    fmt.Printf("Instance: %s\n", failure.Instance.Title)
    fmt.Printf("Operation: %s\n", failure.Operation.Name())
    fmt.Printf("Error: %v\n", failure.Error)
    fmt.Printf("Duration: %v\n", failure.Duration())
}
```

### Combined Errors

The `PartialResult.Error()` method returns a combined error:

```go
if err := result.Error(); err != nil {
    // Returns a formatted error with all failures
    log.Printf("Batch operation failed: %v", err)
}
```

### Rollback Failures

When using `ExecuteWithRollback`, check for rollback failures:

```go
result, err := executor.ExecuteWithRollback(ctx, instances, operation, tracker)
if err != nil {
    if strings.Contains(err.Error(), "rollback partially failed") {
        // Some operations failed to rollback
        log.Printf("Warning: Inconsistent state: %v", err)
    }
}
```

### Validation Errors

Validation errors are captured before execution:

```go
for _, failure := range result.Failures {
    if strings.Contains(failure.Error.Error(), "validation failed") {
        // This instance was invalid before execution
    }
}
```

## Performance Considerations

### Concurrency Tuning

- Start with `runtime.NumCPU()` and adjust based on I/O vs CPU bound operations
- For I/O-bound operations (network, disk), use higher concurrency (2-4x CPU cores)
- For CPU-bound operations, use concurrency equal to or slightly higher than CPU cores

### Memory Usage

Each concurrent operation maintains state in memory. For large batches:

```go
// Process in chunks for very large instance counts
chunkSize := 100
for i := 0; i < len(instances); i += chunkSize {
    end := min(i+chunkSize, len(instances))
    chunk := instances[i:end]
    result := executor.Execute(ctx, chunk, operation, tracker)
}
```

### Progress Callback Performance

Keep progress callbacks fast and non-blocking:

```go
tracker.OnProgress(func(result *concurrency.OperationResult, completed, total int) {
    // Fast logging or metrics update
    metrics.RecordCompletion(result.Duration())

    // Avoid slow operations like database writes
    // Use buffering or async processing instead
})
```

## Thread Safety

All components are thread-safe:

- `BatchExecutor`: Safe for concurrent use
- `ProgressTracker`: Safe for concurrent callback registration and updates
- `PartialResult`: Results are safely accumulated from goroutines

## Testing

The package includes comprehensive tests. Run them with:

```bash
go test -v ./concurrency -run TestBatch
```

For benchmarks:

```bash
go test -bench=BenchmarkBatchExecutor -benchmem ./concurrency
```

## License

Part of the claude-squad project. See LICENSE.md for details.
