# Ollama Meta Framework Examples

This directory contains comprehensive examples demonstrating the Ollama Meta Framework for managing multiple AI agents and concurrent task execution.

## Overview

The Ollama Meta Framework provides:
- **Task Dispatcher**: Concurrent task execution with worker pool management
- **Model Registry**: Registration and management of multiple Ollama models
- **Progress Tracking**: Real-time progress callbacks and metrics collection
- **Error Handling**: Robust error recovery and reporting
- **Task Routing**: Intelligent task distribution across models

## Examples

### 1. Basic Usage (`basic_usage.go`)

**Purpose**: Demonstrates the fundamental workflow of the meta framework.

**Key Concepts**:
- Creating a TaskDispatcher
- Submitting a single task
- Progress monitoring with callbacks
- Task result retrieval
- Metric collection

**How to Run**:
```bash
go run basic_usage.go
```

**What It Shows**:
- Simple agent function definition
- Single-worker execution
- Task lifecycle (pending → running → completed)
- Result handling and error checking

**Use Cases**:
- Learning the framework basics
- Understanding task structure
- Implementing simple one-off tasks

---

### 2. Concurrent Tasks (`concurrent_tasks.go`)

**Purpose**: Demonstrates parallel execution of multiple tasks using a worker pool.

**Key Concepts**:
- Multi-worker dispatcher setup
- Priority-based task scheduling
- Concurrent execution monitoring
- Metrics aggregation
- Error handling in parallel execution

**How to Run**:
```bash
go run concurrent_tasks.go
```

**What It Shows**:
- Creating a dispatcher with 5 workers
- Submitting 10 tasks with varying priorities
- Real-time status monitoring
- Success rate calculation
- Error collection and reporting

**Features**:
- Variable processing times
- Simulated task failures (10% chance)
- Context cancellation support
- Progress tracking across all tasks

**Use Cases**:
- Batch processing of multiple requests
- Load distribution across workers
- High-throughput task processing
- Fault-tolerant execution

---

### 3. Aider Integration (`aider_integration.go`)

**Purpose**: Shows integration with Aider using multiple Ollama models.

**Key Concepts**:
- Model registry with multiple models
- Task type to model mapping
- Aider command-line integration
- File-based task handling
- Model-specific configurations

**How to Run**:
```bash
# First, ensure Ollama is running:
ollama serve

# In another terminal, run the example:
go run aider_integration.go
```

**Prerequisites**:
- Ollama running locally (http://localhost:11434)
- At least one Ollama model pulled: `ollama pull llama2`
- Aider installed: `pip install aider-chat`

**What It Shows**:
- Creating a model registry with 3 different models
- Assigning specific use cases to models:
  - Code Generation: llama2:7b
  - Code Review: mistral:7b
  - Documentation: neural-chat:7b
- Task routing to appropriate models
- Integration with Aider's CLI

**Model Configuration**:
Each model is configured with:
- Context window size
- Timeout settings
- Concurrency limits
- Temperature and sampling parameters
- Use-case labels

**Use Cases**:
- Code generation and refactoring with Aider
- Multi-model task distribution
- Specialized model selection based on task type
- Complex file processing workflows

---

### 4. Custom Router (`custom_router.go`)

**Purpose**: Implements custom task routing strategy with load balancing.

**Key Concepts**:
- Custom routing logic implementation
- Load-aware model selection
- Task type-based routing rules
- Dynamic load tracking
- Intelligent task distribution

**How to Run**:
```bash
go run custom_router.go
```

**What It Shows**:
- Implementing a TaskRouter with:
  - Route rules for different task types
  - Load-balancing aware selection
  - Real-time load tracking
- Submitting diverse tasks
- Monitoring routing decisions
- Collecting routing statistics

**Routing Strategy**:
```
Task Type → Model Assignment
- "generation" → llama2:7b (fast)
- "analysis" → mistral:7b (accurate)
- "optimization" → neural-chat:7b (specialized)
```

**Advanced Features**:
- Atomic load counting for thread safety
- Dynamic fallback selection
- Per-model load reporting
- Routing statistics

**Use Cases**:
- Intelligent task distribution
- Load balancing across models
- Task-type specific model selection
- Advanced scheduling strategies

---

## Architecture

### Task Lifecycle

```
Task Created
    ↓
Task Submitted → SubmitTask()
    ↓
Status: Pending → task added to queue
    ↓
Status: Running → worker picks up task
    ↓
Agent Function Executes → actual work
    ↓
Status: Completed/Failed ← execution done
    ↓
Result Available → task.Result contains output
```

### Worker Pool Architecture

```
┌─────────────────────────────────────────┐
│        TaskDispatcher (Main)            │
├─────────────────────────────────────────┤
│  Task Queue (buffered channel)          │
│  ├─ Capacity: workerCount × 100         │
│  └─ Tasks stored with metadata          │
├─────────────────────────────────────────┤
│  Worker Pool (N goroutines)             │
│  ├─ Worker 0 → executing task           │
│  ├─ Worker 1 → executing task           │
│  └─ Worker N → idle, waiting for task   │
├─────────────────────────────────────────┤
│  Task Registry (RW Mutex protected)     │
│  ├─ task-001 → completed                │
│  ├─ task-002 → running                  │
│  └─ task-003 → pending                  │
└─────────────────────────────────────────┘
```

## Core Types

### Task
```go
type Task struct {
    ID          string          // Unique identifier
    Priority    int             // 0=High, 1=Normal, 2=Low
    Payload     interface{}     // Task input data
    Status      TaskStatus      // Current status
    Error       error           // Error if failed
    Result      interface{}     // Task output
    CreatedAt   time.Time       // Creation time
    StartedAt   time.Time       // Execution start
    CompletedAt time.Time       // Completion time
}
```

### TaskDispatcher
```go
type TaskDispatcher struct {
    workerCount   int              // Number of workers
    maxQueueSize  int              // Queue capacity
    agentFunc     AgentFunc        // Task processor
    progressCb    ProgressCallback // Progress reporter
    taskQueue     chan *Task       // Task channel
    taskMap       map[string]*Task // Task registry
    // ... synchronization and metrics fields
}
```

### ModelRegistry
```go
type ModelRegistry struct {
    models       map[string]*ModelMetadata  // Available models
    configs      map[string]*ModelConfig    // Model configs
    providers    []ModelProvider            // Model sources
    defaultModel string                     // Fallback model
}
```

## Usage Patterns

### Pattern 1: Simple Task Execution
```go
dispatcher, _ := ollama.NewTaskDispatcher(ctx, agentFunc, 1)
dispatcher.Start()
dispatcher.SubmitTask(task)
dispatcher.Shutdown(timeout)
```

### Pattern 2: Batch Processing
```go
dispatcher, _ := ollama.NewTaskDispatcher(ctx, agentFunc, 5)
dispatcher.Start()
for _, task := range tasks {
    dispatcher.SubmitTask(task)
}
// Monitor and wait
dispatcher.Shutdown(timeout)
```

### Pattern 3: Custom Routing
```go
router := NewTaskRouter(registry)
// In agent function:
selectedModel := router.SelectBestModel(taskType)
router.RecordTaskStart(selectedModel)
// ... execute task
router.RecordTaskEnd(selectedModel)
```

## Configuration Guide

### Creating a Dispatcher
```go
// Parameters:
// - ctx: context.Context for cancellation
// - agentFunc: the function that processes tasks
// - workerCount: number of concurrent workers (1-10)
dispatcher, err := ollama.NewTaskDispatcher(ctx, agentFunc, 3)
```

### Progress Callback
```go
dispatcher.SetProgressCallback(func(
    taskID string,
    status ollama.TaskStatus,
    progress int, // 0-100
    message string,
) {
    fmt.Printf("Task %s: %d%% - %s\n", taskID, progress, message)
})
```

### Task Priority
- **PriorityHigh (0)**: Executed first
- **PriorityNormal (1)**: Default priority
- **PriorityLow (2)**: Executed last

### Model Configuration
```go
config := &ollama.ModelConfig{
    Enabled:               true,
    Priority:              0,
    MaxConcurrentRequests: 5,
    TimeoutSeconds:        60,
    RequestOptions: ollama.RequestOptions{
        Stream:      true,
        Temperature: 0.7,
        TopK:        40,
        TopP:        0.9,
    },
    Labels: []string{"production", "code-generation"},
}
```

## Error Handling

### Common Error Types
- **FrameworkError**: Framework-level errors
- **ClientError**: HTTP client errors
- **ParseError**: Data parsing errors
- **Task Errors**: Task execution failures

### Error Retrieval
```go
// Get all execution errors
errors := dispatcher.GetErrors()
for _, err := range errors {
    fmt.Printf("Task %s failed on worker %d: %v\n",
        err.TaskID, err.WorkerID, err.Error)
}
```

## Metrics and Monitoring

### Dispatcher Metrics
```go
metrics := dispatcher.GetMetrics()
fmt.Printf("Total: %d, Completed: %d, Failed: %d, Pending: %d\n",
    metrics.TotalTasks,
    metrics.CompletedTasks,
    metrics.FailedTasks,
    metrics.PendingTasks)
```

### Task Status
```go
status, err := dispatcher.GetTaskStatus(taskID)
task, err := dispatcher.GetTask(taskID)
```

## Best Practices

1. **Worker Count**: Start with `(number of CPU cores) * 2`
2. **Timeout**: Set reasonable timeouts for your workloads
3. **Progress Monitoring**: Implement callbacks for long-running tasks
4. **Error Handling**: Always check errors and implement retry logic
5. **Graceful Shutdown**: Always call Shutdown() with appropriate timeout
6. **Context Usage**: Pass context for proper cancellation support
7. **Model Selection**: Use load-aware routing for optimal performance

## Troubleshooting

### Tasks Not Executing
- Check if dispatcher is started: `dispatcher.Start()`
- Verify worker count > 0
- Ensure context is not cancelled

### High Memory Usage
- Reduce queue size by lowering worker count
- Implement backpressure in task submission
- Monitor with `dispatcher.GetMetrics()`

### Task Timeouts
- Increase TimeoutSeconds in model config
- Check if agent function is blocking
- Verify context deadline

### Model Not Found
- Register model first: `registry.RegisterModel()`
- Check model name matches exactly
- Verify model is enabled

## Advanced Topics

### Custom Agent Functions
```go
agentFunc := func(ctx context.Context, task *ollama.Task) error {
    // Access task data
    payload := task.Payload.(map[string]interface{})

    // Do work with context cancellation support
    select {
    case <-time.After(duration):
        task.Result = result
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### Task Type Routing
```go
func routeTask(task *ollama.Task) string {
    payload := task.Payload.(map[string]interface{})
    taskType := payload["type"].(string)

    switch taskType {
    case "generation":
        return "llama2:7b"
    case "analysis":
        return "mistral:7b"
    default:
        return "neural-chat:7b"
    }
}
```

## Performance Characteristics

- **Throughput**: ~100-1000 tasks/sec depending on worker count
- **Latency**: Task queuing time typically < 1ms
- **Memory**: ~1-2MB per task in queue
- **Scalability**: Linear with worker count up to available CPU cores

## Contributing

To extend these examples:
1. Create new example files with clear naming
2. Add comprehensive comments
3. Include usage instructions
4. Document all key concepts
5. Add error handling examples

## License

AGPL-3.0 (Same as claude-squad)
