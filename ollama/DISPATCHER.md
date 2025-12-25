# TaskDispatcher - Concurrent Task Execution Framework

A comprehensive, production-ready concurrent task dispatcher for executing up to 10 parallel agent tasks with advanced features including worker pool management, priority support, context-based cancellation, error aggregation, and progress tracking.

## Overview

The `TaskDispatcher` is designed to manage concurrent execution of independent tasks with the following capabilities:

- **Worker Pool**: Configurable pool of up to 10 concurrent workers
- **Task Queue**: Priority-based task queuing system
- **Context Management**: Full support for context-based cancellation and timeouts
- **Error Aggregation**: Collects and aggregates errors from all workers
- **Progress Tracking**: Real-time progress callbacks for task monitoring
- **Concurrency Safety**: Thread-safe operations with proper mutex coordination
- **WaitGroup Coordination**: Graceful shutdown and task completion synchronization
- **Metrics**: Built-in performance metrics and statistics

## Architecture

### Core Components

```
TaskDispatcher
├── Worker Pool (1-10 workers)
├── Task Queue (Priority-based)
├── Error Aggregator
├── Progress Tracker
├── Metrics Collector
└── Context Manager
```

## Key Features

### 1. Worker Pool

- Configurable from 1 to 10 workers (MaxWorkers = 10)
- Each worker independently processes tasks from the queue
- Graceful shutdown with timeout support

```go
// Create dispatcher with 5 workers
dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 5)
if err != nil {
    log.Fatal(err)
}
```

### 2. Task Priority

Three priority levels are supported:
- `PriorityHigh` (0) - Highest priority
- `PriorityNormal` (1) - Standard priority
- `PriorityLow` (2) - Lowest priority

```go
task := &Task{
    ID:       "task-1",
    Priority: PriorityHigh,
    Payload:  map[string]interface{}{"key": "value"},
}
```

### 3. Task Status Tracking

Tasks progress through the following states:
- `StatusPending` - Task created but not yet executed
- `StatusRunning` - Task is currently being executed
- `StatusCompleted` - Task completed successfully
- `StatusFailed` - Task execution resulted in an error
- `StatusCancelled` - Task was cancelled before execution

### 4. Context-Based Cancellation

Full support for context cancellation at both dispatcher and task levels:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 5)
// Cancelling ctx will stop all workers and cancel all pending tasks
```

### 5. Error Aggregation

All errors from task execution are collected and can be retrieved:

```go
errors := dispatcher.GetErrors()
for _, taskErr := range errors {
    fmt.Printf("Task %s failed: %v\n", taskErr.TaskID, taskErr.Error)
}
```

### 6. Progress Callbacks

Monitor task progress in real-time:

```go
dispatcher.SetProgressCallback(func(taskID string, status TaskStatus, progress int, message string) {
    fmt.Printf("[%s] %s: %d%% - %s\n", taskID, status.String(), progress, message)
})
```

### 7. Metrics and Statistics

Track dispatcher performance:

```go
metrics := dispatcher.GetMetrics()
fmt.Printf("Completed: %d, Failed: %d, Pending: %d\n",
    metrics.CompletedTasks,
    metrics.FailedTasks,
    metrics.PendingTasks)
```

## Usage Examples

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "time"
)

func main() {
    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Define agent function
    agentFunc := func(ctx context.Context, task *Task) error {
        // Perform work
        time.Sleep(1 * time.Second)
        task.Result = "completed"
        return nil
    }

    // Create dispatcher with 5 workers
    dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 5)
    if err != nil {
        panic(err)
    }

    // Start dispatcher
    if err := dispatcher.Start(); err != nil {
        panic(err)
    }
    defer dispatcher.Shutdown(5 * time.Second)

    // Submit tasks
    for i := 0; i < 20; i++ {
        task := &Task{
            ID:       fmt.Sprintf("task-%d", i),
            Priority: PriorityNormal,
        }
        if err := dispatcher.SubmitTask(task); err != nil {
            fmt.Printf("Failed to submit task: %v\n", err)
        }
    }

    // Wait for completion
    dispatcher.Wait()

    // Check results
    metrics := dispatcher.GetMetrics()
    fmt.Printf("Completed: %d, Failed: %d\n", metrics.CompletedTasks, metrics.FailedTasks)
}
```

### With Progress Tracking

```go
dispatcher := setupDispatcher(ctx, agentFunc, 3)

// Set progress callback
dispatcher.SetProgressCallback(func(taskID string, status TaskStatus, progress int, message string) {
    fmt.Printf("[%s] %s: %d%% - %s\n", taskID, status.String(), progress, message)
})

dispatcher.Start()
defer dispatcher.Shutdown(5 * time.Second)

// Submit and wait for tasks
submitTasksBatch(dispatcher, 10)
dispatcher.Wait()
```

### Error Handling

```go
agentFunc := func(ctx context.Context, task *Task) error {
    if err := performWork(task); err != nil {
        return fmt.Errorf("work failed: %w", err)
    }
    return nil
}

dispatcher, _ := NewTaskDispatcher(ctx, agentFunc, 4)
dispatcher.Start()
defer dispatcher.Shutdown(5 * time.Second)

submitTasks(dispatcher, 20)
dispatcher.Wait()

// Aggregate errors
errors := dispatcher.GetErrors()
if len(errors) > 0 {
    fmt.Printf("Encountered %d errors:\n", len(errors))
    for _, e := range errors {
        fmt.Printf("  Task %s: %v\n", e.TaskID, e.Error)
    }
}
```

### Task Cancellation

```go
dispatcher, _ := NewTaskDispatcher(ctx, agentFunc, 2)
dispatcher.Start()
defer dispatcher.Shutdown(5 * time.Second)

taskID := "long-running-task"
task := &Task{ID: taskID, Priority: PriorityNormal}
dispatcher.SubmitTask(task)

// Cancel task
time.AfterFunc(2*time.Second, func() {
    if err := dispatcher.CancelTask(taskID); err != nil {
        fmt.Printf("Cancel failed: %v\n", err)
    }
})

dispatcher.Wait()
```

### Batch Submission

```go
// Create batch of tasks
var batch []*Task
for i := 0; i < 100; i++ {
    batch = append(batch, &Task{
        ID:       fmt.Sprintf("batch-task-%d", i),
        Priority: PriorityNormal,
        Payload:  map[string]interface{}{"index": i},
    })
}

// Submit entire batch
if err := dispatcher.SubmitBatch(batch); err != nil {
    fmt.Printf("Batch submission error: %v\n", err)
}
```

### Maximum Workers

```go
// Create dispatcher with maximum workers
dispatcher, err := NewTaskDispatcher(ctx, agentFunc, ollama.MaxWorkers)
if err != nil {
    panic(err)
}

// Process large number of tasks efficiently
dispatcher.Start()
defer dispatcher.Shutdown(5 * time.Second)

for i := 0; i < 100; i++ {
    task := &Task{
        ID: fmt.Sprintf("heavy-task-%d", i),
        Priority: PriorityNormal,
    }
    dispatcher.SubmitTask(task)
}

dispatcher.Wait()
metrics := dispatcher.GetMetrics()
fmt.Printf("Processed %d tasks with %d workers\n",
    metrics.CompletedTasks, metrics.WorkerCount)
```

## API Reference

### TaskDispatcher Methods

#### Creating a Dispatcher

```go
dispatcher, err := NewTaskDispatcher(ctx, agentFunc, workerCount)
```

- **Parameters**:
  - `ctx`: Context for cancellation
  - `agentFunc`: Function to execute tasks
  - `workerCount`: Number of workers (1-10)
- **Returns**: TaskDispatcher instance or error

#### Lifecycle Management

```go
// Start the dispatcher
err := dispatcher.Start()

// Submit a task
err := dispatcher.SubmitTask(task)

// Submit multiple tasks
err := dispatcher.SubmitBatch(tasks)

// Wait for all tasks to complete
err := dispatcher.Wait()

// Shutdown with timeout
err := dispatcher.Shutdown(timeout)
```

#### Task Management

```go
// Get task status
status, err := dispatcher.GetTaskStatus(taskID)

// Get task details
task, err := dispatcher.GetTask(taskID)

// Cancel a task
err := dispatcher.CancelTask(taskID)
```

#### Progress and Monitoring

```go
// Set progress callback
dispatcher.SetProgressCallback(callback)

// Get all errors
errors := dispatcher.GetErrors()

// Get metrics
metrics := dispatcher.GetMetrics()
```

### Task Structure

```go
type Task struct {
    ID          string        // Unique task identifier
    Priority    int           // Task priority level
    Payload     interface{}   // Task input data
    Status      TaskStatus    // Current status
    Error       error         // Error if execution failed
    Result      interface{}   // Task output/result
    CreatedAt   time.Time     // Task creation time
    StartedAt   time.Time     // Task start time
    CompletedAt time.Time     // Task completion time
}
```

### AgentFunc Signature

```go
type AgentFunc func(ctx context.Context, task *Task) error
```

The agent function:
- Receives task context and task data
- Must check context for cancellation
- Should update task.Result on success
- Should return error on failure

### ProgressCallback Signature

```go
type ProgressCallback func(taskID string, status TaskStatus, progress int, message string)
```

Called with:
- `taskID`: Task identifier
- `status`: Current task status
- `progress`: Progress percentage (0-100)
- `message`: Optional status message

### DispatcherMetrics

```go
type DispatcherMetrics struct {
    TotalTasks     int // Total tasks submitted
    CompletedTasks int // Successfully completed tasks
    FailedTasks    int // Failed tasks
    CancelledTasks int // Cancelled tasks
    PendingTasks   int // Remaining pending tasks
    WorkerCount    int // Number of workers
}
```

## Concurrency Patterns

### Pattern 1: Parallel Agent Execution

```go
// Execute multiple agent tasks in parallel
workers := 10
dispatcher, _ := NewTaskDispatcher(ctx, agentFunc, workers)
dispatcher.Start()
defer dispatcher.Shutdown(10 * time.Second)

// Submit all tasks
for i := 0; i < 100; i++ {
    dispatcher.SubmitTask(&Task{ID: fmt.Sprintf("task-%d", i)})
}

// Wait for all to complete
dispatcher.Wait()
```

### Pattern 2: Priority-Based Execution

```go
// Submit tasks with different priorities
highPriority := &Task{ID: "urgent", Priority: PriorityHigh}
normalPriority := &Task{ID: "normal", Priority: PriorityNormal}
lowPriority := &Task{ID: "background", Priority: PriorityLow}

dispatcher.SubmitTask(highPriority)
dispatcher.SubmitTask(normalPriority)
dispatcher.SubmitTask(lowPriority)
```

### Pattern 3: Timeout with Cancellation

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

dispatcher, _ := NewTaskDispatcher(ctx, agentFunc, 5)
dispatcher.Start()

// Tasks will be cancelled if they exceed 30 second timeout
submitTasks(dispatcher, 50)
dispatcher.Wait() // Returns if context is cancelled
```

### Pattern 4: Error Recovery

```go
dispatcher.SetProgressCallback(func(taskID string, status TaskStatus, progress int, message string) {
    if status == StatusFailed {
        // Handle failure - maybe retry or log
        fmt.Printf("Task %s failed: %s\n", taskID, message)
    }
})

// All errors are collected and can be reviewed
dispatcher.Wait()
errors := dispatcher.GetErrors()
for _, err := range errors {
    // Handle or report error
}
```

## Performance Considerations

### Optimal Configuration

- **Worker Count**: Use 1-10 based on task parallelism and system resources
- **Queue Size**: Automatically set to `workerCount * 100`
- **Context Timeout**: Set appropriate timeout for expected task duration
- **Batch Size**: Submit tasks in reasonable batches to avoid queue overflow

### Best Practices

1. **Resource Management**
   - Always call `Shutdown()` to clean up resources
   - Use defer for guaranteed cleanup

2. **Error Handling**
   - Check `GetErrors()` after completion
   - Set progress callbacks for monitoring

3. **Context Usage**
   - Create context with appropriate timeout
   - Respect context cancellation in agent function

4. **Task Design**
   - Keep agent functions focused and independent
   - Update task.Result for successful completions
   - Return meaningful errors on failures

5. **Monitoring**
   - Use progress callbacks for real-time updates
   - Check metrics for performance analysis
   - Log important events for debugging

## Thread Safety

All public methods are thread-safe:
- Multiple goroutines can submit tasks concurrently
- Task status queries are safe during execution
- Metrics are safely aggregated from multiple workers
- Progress callbacks are called safely from worker goroutines

## Examples from Source

### Basic Example (dispatcher_example.go)

```go
// See ExampleBasicDispatcher() for complete implementation
// Creates dispatcher with 5 workers
// Submits 20 tasks
// Reports final metrics
```

### Advanced Examples Available

- `ExampleDispatcherWithProgress` - Real-time progress tracking
- `ExampleDispatcherWithErrors` - Error handling and aggregation
- `ExampleDispatcherWithCancellation` - Task cancellation
- `ExampleDispatcherWithContextCancellation` - Context-based cancellation
- `ExampleDispatcherWithPriorities` - Priority-based scheduling
- `ExampleDispatcherWithMaxWorkers` - Full capacity operation

## Logging

The dispatcher integrates with the claude-squad logging system:
- Info logs for dispatcher lifecycle events
- Warning logs for timeout conditions
- Error logs for task execution failures

Logs are written to the configured log file and include:
- Worker startup/shutdown
- Task submission/completion
- Error details with task context
- Shutdown and cleanup events

## Testing

Comprehensive test suite included (dispatcher_test.go):
- Creation and lifecycle tests
- Task submission and execution
- Error handling and recovery
- Cancellation scenarios
- Progress callback verification
- Metrics validation
- Batch operations
- Edge cases and error conditions

Run tests with:
```bash
go test -v ./ollama -run TestTaskDispatcher
```

## Integration Example

Here's how to integrate the dispatcher into your application:

```go
package main

import (
    "claude-squad/ollama"
    "context"
    "log"
    "time"
)

func processWithDispatcher(ctx context.Context, tasks []*ollama.Task) error {
    // Define agent function
    agentFunc := func(ctx context.Context, task *ollama.Task) error {
        // Your agent logic here
        return executeAgent(ctx, task)
    }

    // Create and start dispatcher
    dispatcher, err := ollama.NewTaskDispatcher(ctx, agentFunc, 5)
    if err != nil {
        return err
    }

    if err := dispatcher.Start(); err != nil {
        return err
    }
    defer dispatcher.Shutdown(10 * time.Second)

    // Submit tasks
    if err := dispatcher.SubmitBatch(tasks); err != nil {
        return err
    }

    // Wait for completion
    if err := dispatcher.Wait(); err != nil {
        return err
    }

    // Check results
    errors := dispatcher.GetErrors()
    metrics := dispatcher.GetMetrics()

    log.Printf("Completed: %d, Failed: %d, Cancelled: %d",
        metrics.CompletedTasks, metrics.FailedTasks, metrics.CancelledTasks)

    return nil
}

func executeAgent(ctx context.Context, task *ollama.Task) error {
    // Your implementation here
    return nil
}
```

## Troubleshooting

### Tasks Not Executing

1. Verify dispatcher is started with `Start()`
2. Check that context is not cancelled
3. Ensure worker count > 0
4. Verify `SubmitTask()` returns no error

### High Error Rate

1. Review error details with `GetErrors()`
2. Check progress callbacks for detailed status
3. Verify agent function error handling
4. Consider increasing worker count for I/O-bound tasks

### Slow Execution

1. Check system resources
2. Monitor `GetMetrics()` for pending tasks
3. Consider increasing worker count (up to MaxWorkers)
4. Profile agent function for bottlenecks

### Memory Issues

1. Monitor queue size (auto-limited to workerCount * 100)
2. Ensure task payloads are reasonable
3. Clear task results if not needed
4. Use batch processing for large task sets

## Related Patterns

The TaskDispatcher uses patterns from:
- **daemon/daemon.go**: WaitGroup coordination, context cancellation
- **session/git/worktree_ops.go**: Parallel operations, error aggregation

## Summary

The TaskDispatcher provides a robust, production-ready framework for concurrent task execution with enterprise-grade features:

- ✅ Up to 10 parallel workers
- ✅ Priority-based task scheduling
- ✅ Context-aware cancellation
- ✅ Comprehensive error handling
- ✅ Real-time progress tracking
- ✅ Thread-safe operations
- ✅ Detailed metrics and monitoring
- ✅ Graceful shutdown
- ✅ Full test coverage
- ✅ Logging integration

Use it to efficiently parallelize agent workloads while maintaining control, observability, and reliability.
