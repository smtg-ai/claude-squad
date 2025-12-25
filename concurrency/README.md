# Concurrent Agent Orchestrator

A production-ready concurrent agent orchestration system for managing multiple AI agents with advanced features including load balancing, health monitoring, circuit breaker pattern, and event-driven architecture.

## Overview

The Concurrent Agent Orchestrator provides a robust framework for managing multiple claude-squad instances concurrently. It builds on top of the existing `session.Instance` infrastructure and adds enterprise-grade features for reliability, scalability, and observability.

## Features

### 1. Agent Lifecycle Management
- **Start**: Initialize and start new agents
- **Pause**: Temporarily pause agents while preserving state
- **Resume**: Resume paused agents
- **Stop**: Gracefully shutdown agents

### 2. Load Balancing
Multiple load balancing algorithms are supported:
- **Round-Robin**: Distribute tasks evenly across agents in sequence
- **Least-Loaded**: Assign tasks to the agent with the lowest load score
- **Random**: Randomly distribute tasks across healthy agents

### 3. Health Monitoring
- Automatic health checks at configurable intervals
- Agent health status tracking
- Automatic detection of failed agents
- Integration with circuit breaker pattern

### 4. Circuit Breaker Pattern
Prevents cascading failures by:
- Opening circuit after consecutive failures
- Automatically attempting recovery after timeout
- Half-open state for testing recovery
- Recording success/failure metrics

### 5. Task Distribution
- Priority-based task queuing (Low, Normal, High, Critical)
- Agent affinity support for specialized tasks
- Configurable concurrency limits
- Timeout support for long-running tasks
- Task result tracking

### 6. Event-Driven Architecture
Real-time event streaming for:
- Agent state changes
- Task completion
- Health check failures
- Agent recovery
- System metrics

### 7. Inter-Agent Communication
- Event channels for agent coordination
- Task result channels
- Metrics aggregation

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                  AgentOrchestrator                      │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐       │
│  │  Task      │  │  Health    │  │  Recovery  │       │
│  │  Worker    │  │  Worker    │  │  Worker    │       │
│  └────────────┘  └────────────┘  └────────────┘       │
│                                                         │
│  ┌──────────────────────────────────────────────┐     │
│  │            Load Balancer                     │     │
│  │  (Round-Robin/Least-Loaded/Random)           │     │
│  └──────────────────────────────────────────────┘     │
│                                                         │
│  ┌──────────────────────────────────────────────┐     │
│  │              Task Queue                      │     │
│  └──────────────────────────────────────────────┘     │
│                                                         │
│  ┌──────────────────────────────────────────────┐     │
│  │            Event Channel                     │     │
│  └──────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────┘
           │              │              │
           ▼              ▼              ▼
    ┌────────────┐ ┌────────────┐ ┌────────────┐
    │  Managed   │ │  Managed   │ │  Managed   │
    │  Agent 1   │ │  Agent 2   │ │  Agent N   │
    └────────────┘ └────────────┘ └────────────┘
           │              │              │
           ▼              ▼              ▼
    ┌────────────┐ ┌────────────┐ ┌────────────┐
    │  Instance  │ │  Instance  │ │  Instance  │
    │  (Tmux +   │ │  (Tmux +   │ │  (Tmux +   │
    │  Worktree) │ │  Worktree) │ │  Worktree) │
    └────────────┘ └────────────┘ └────────────┘
```

## Usage

### Basic Example

```go
package main

import (
    "claude-squad/concurrency"
    "claude-squad/session"
    "fmt"
    "time"
)

func main() {
    // Create orchestrator with default configuration
    orchestrator := concurrency.NewOrchestrator(
        concurrency.DefaultOrchestratorConfig(),
    )
    defer orchestrator.Shutdown(30 * time.Second)

    // Create and add agents
    for i := 1; i <= 3; i++ {
        agentID := fmt.Sprintf("agent-%d", i)

        instance, err := session.NewInstance(session.InstanceOptions{
            Title:   agentID,
            Path:    "/path/to/workspace",
            Program: "claude",
        })
        if err != nil {
            panic(err)
        }

        if err := instance.Start(true); err != nil {
            panic(err)
        }

        agent := concurrency.NewManagedAgent(agentID, instance)
        orchestrator.AddAgent(agent)
    }

    // Distribute a task
    task := &concurrency.Task{
        ID:       "task-1",
        Prompt:   "Analyze this codebase for bugs",
        Priority: concurrency.TaskPriorityHigh,
        Timeout:  5 * time.Minute,
        ResultChan: make(chan *concurrency.TaskResult, 1),
    }

    orchestrator.DistributeTask(task)

    // Wait for result
    result := <-task.ResultChan
    if result.Success {
        fmt.Printf("Task completed: %s\n", result.Output)
    } else {
        fmt.Printf("Task failed: %v\n", result.Error)
    }
}
```

### Custom Configuration

```go
config := &concurrency.OrchestratorConfig{
    MaxConcurrentTasks:     5,
    HealthCheckInterval:    30 * time.Second,
    TaskQueueSize:          50,
    EventBufferSize:        100,
    EnableAutoRecovery:     true,
    LoadBalancingAlgorithm: "least-loaded",
}

orchestrator := concurrency.NewOrchestrator(config)
```

### Event Monitoring

```go
go func() {
    for event := range orchestrator.EventChannel() {
        switch event.Type {
        case "TaskCompleted":
            fmt.Printf("Task %s completed\n", event.Data["task_id"])
        case "HealthCheckFailed":
            fmt.Printf("Agent %s health check failed\n", event.AgentID)
        case "AgentRecovered":
            fmt.Printf("Agent %s recovered\n", event.AgentID)
        }
    }
}()
```

### Task Creation

**IMPORTANT**: Task result channels must be buffered (capacity >= 1) to prevent goroutine leaks.

Use the `NewTask()` helper for safe task creation:

```go
// Recommended: Use NewTask helper
task := concurrency.NewTask(
    "task-1",
    "Analyze this codebase",
    concurrency.TaskPriorityHigh,
    5*time.Minute,
)
```

Or manually create with buffered channel:

```go
// Manual creation: ensure ResultChan is buffered
task := &concurrency.Task{
    ID:         "task-1",
    Prompt:     "Analyze this codebase",
    Priority:   concurrency.TaskPriorityHigh,
    Timeout:    5 * time.Minute,
    ResultChan: make(chan *concurrency.TaskResult, 1), // MUST be buffered
}
```

### Task Affinity

```go
// Create task that prefers specific agents
task := &concurrency.Task{
    ID:       "specialized-task",
    Prompt:   "Optimize database queries",
    Priority: concurrency.TaskPriorityHigh,
    Affinity: []string{"backend-specialist-1", "backend-specialist-2"},
    Timeout:  10 * time.Minute,
    ResultChan: make(chan *concurrency.TaskResult, 1),
}
```

### Agent Lifecycle Control

```go
// Pause an agent
orchestrator.PauseAgent("agent-1")

// Resume an agent
orchestrator.ResumeAgent("agent-1")

// Remove an agent
orchestrator.RemoveAgent("agent-1")
```

### Metrics and Monitoring

```go
// Get orchestrator metrics
metrics := orchestrator.GetMetrics()
fmt.Printf("Active Tasks: %d\n", metrics["active_tasks"])
fmt.Printf("Total Completed: %d\n", metrics["total_tasks_completed"])

// Get agent-specific stats
stats, err := orchestrator.GetAgentStats("agent-1")
if err == nil {
    fmt.Printf("Agent Load: %f\n", stats["load_score"])
    fmt.Printf("Tasks Completed: %d\n", stats["tasks_completed"])
}
```

## API Reference

### AgentOrchestrator

#### Methods

- `NewOrchestrator(config *OrchestratorConfig) *AgentOrchestrator`
  - Creates a new orchestrator instance

- `AddAgent(agent *ManagedAgent) error`
  - Registers a new agent with the orchestrator

- `RemoveAgent(agentID string) error`
  - Removes an agent from the orchestrator

- `GetAgent(agentID string) (*ManagedAgent, error)`
  - Retrieves an agent by ID

- `ListAgents() []string`
  - Returns a list of all agent IDs

- `DistributeTask(task *Task) error`
  - Distributes a task to an appropriate agent

- `PauseAgent(agentID string) error`
  - Pauses a specific agent

- `ResumeAgent(agentID string) error`
  - Resumes a paused agent

- `GetMetrics() map[string]interface{}`
  - Returns orchestrator metrics

- `GetAgentStats(agentID string) (map[string]interface{}, error)`
  - Returns statistics for a specific agent

- `EventChannel() <-chan *AgentEvent`
  - Returns the event channel for subscribing to events

- `Shutdown(timeout time.Duration) error`
  - Gracefully shuts down the orchestrator

### ManagedAgent

#### Methods

- `NewManagedAgent(id string, instance *session.Instance) *ManagedAgent`
  - Creates a new managed agent

- `GetID() string`
  - Returns the agent's unique identifier

- `GetState() AgentState`
  - Returns the current agent state

- `SetState(state AgentState)`
  - Updates the agent state

- `GetLoadScore() float64`
  - Returns the current load score (0.0 to 1.0)

- `UpdateLoadScore()`
  - Calculates and updates the agent's load score

- `IsHealthy() bool`
  - Checks if the agent is healthy

- `PerformHealthCheck() error`
  - Executes a health check on the agent

- `ExecuteTask(ctx context.Context, task *Task) *TaskResult`
  - Executes a task on this agent

- `Pause() error`
  - Pauses the agent

- `Resume() error`
  - Resumes a paused agent

- `Stop() error`
  - Stops the agent and cleans up resources

- `GetStats() map[string]interface{}`
  - Returns agent statistics

### CircuitBreaker

#### Methods

- `NewCircuitBreaker(maxFailures int, resetTimeout time.Duration, halfOpenTests int) *CircuitBreaker`
  - Creates a new circuit breaker

- `CanExecute() bool`
  - Checks if the circuit breaker allows execution

- `RecordSuccess()`
  - Records a successful execution

- `RecordFailure()`
  - Records a failed execution

- `GetState() CircuitBreakerState`
  - Returns the current circuit breaker state

- `TransitionToHalfOpen() bool`
  - Attempts to transition to half-open state

## States and Enums

### AgentState
- `AgentStateIdle`: Agent is idle and ready for tasks
- `AgentStateRunning`: Agent is currently executing a task
- `AgentStatePaused`: Agent is paused
- `AgentStateFailed`: Agent has failed health checks
- `AgentStateStopped`: Agent has been stopped

### TaskPriority
- `TaskPriorityLow`: Background tasks
- `TaskPriorityNormal`: Standard tasks
- `TaskPriorityHigh`: Urgent tasks
- `TaskPriorityCritical`: Critical tasks that must be executed immediately

### CircuitBreakerState
- `CircuitClosed`: Normal operation
- `CircuitOpen`: Agent is failing and should not receive tasks
- `CircuitHalfOpen`: Agent is being tested for recovery

## Event Types

- `AgentAdded`: New agent registered
- `AgentRemoved`: Agent removed from orchestrator
- `AgentPaused`: Agent paused
- `AgentResumed`: Agent resumed
- `TaskCompleted`: Task execution completed
- `HealthCheckFailed`: Agent health check failed
- `AgentRecovered`: Agent recovered from failure

## Configuration Options

### OrchestratorConfig

- `MaxConcurrentTasks int`: Maximum number of tasks that can run concurrently (default: 10)
- `HealthCheckInterval time.Duration`: How often to perform health checks (default: 30s)
- `TaskQueueSize int`: Size of the task queue buffer (default: 100)
- `EventBufferSize int`: Size of the event channel buffer (default: 100)
- `EnableAutoRecovery bool`: Enable automatic recovery of failed agents (default: true)
- `LoadBalancingAlgorithm string`: Load balancing strategy (default: "least-loaded")
  - Options: "round-robin", "least-loaded", "random"

## WorkerPool

The concurrency package also provides a production-ready WorkerPool implementation for general-purpose parallel job execution with priority queuing and health monitoring.

### Overview

WorkerPool manages a fixed pool of workers that execute jobs concurrently with:
- **Priority-based job queuing**: Jobs are executed in priority order
- **Worker health monitoring**: Automatic detection of stuck workers
- **Comprehensive metrics**: Track throughput, latency, and worker status
- **Graceful shutdown**: Wait for in-flight jobs to complete
- **Context support**: Full context cancellation support

### Job Interface

Jobs must implement the Job interface:

```go
type Job interface {
    Execute(ctx context.Context) (interface{}, error)
    Priority() int  // Higher values = higher priority
    ID() string     // Unique identifier
}
```

### Configuration

```go
type WorkerPoolConfig struct {
    // MaxWorkers is the maximum number of concurrent workers (default: 10)
    MaxWorkers int

    // QueueSize is the maximum size of the job queue (default: 1000)
    QueueSize int

    // WorkerTimeout is the maximum time a job can run (default: 5 minutes)
    WorkerTimeout time.Duration

    // HealthCheckInterval is how often to check worker health (default: 30 seconds)
    HealthCheckInterval time.Duration
}
```

### Basic Usage

```go
// Create worker pool with default configuration
config := DefaultWorkerPoolConfig()
pool := NewWorkerPool(config)

// Start the pool
if err := pool.Start(); err != nil {
    log.Fatal(err)
}
defer pool.Shutdown(context.Background())

// Submit jobs
ctx := context.Background()
job := &MyJob{id: "job-1", priority: 10}
if err := pool.Submit(ctx, job); err != nil {
    log.Fatal(err)
}

// Receive results
go func() {
    for result := range pool.Results() {
        if result.Error != nil {
            log.Printf("Job %s failed: %v", result.JobID, result.Error)
        } else {
            log.Printf("Job %s completed: %v", result.JobID, result.Result)
        }
    }
}()
```

### Custom Job Implementation

```go
type AnalysisJob struct {
    id       string
    priority int
    data     []byte
}

func (j *AnalysisJob) ID() string {
    return j.id
}

func (j *AnalysisJob) Priority() int {
    return j.priority
}

func (j *AnalysisJob) Execute(ctx context.Context) (interface{}, error) {
    // Perform analysis
    result, err := analyzeData(ctx, j.data)
    if err != nil {
        return nil, err
    }
    return result, nil
}
```

### WorkerPool Methods

- `NewWorkerPool(config WorkerPoolConfig) *WorkerPool` - Create a new worker pool
- `Start() error` - Start processing jobs (thread-safe, call once)
- `Submit(ctx context.Context, job Job) error` - Submit a job for execution (thread-safe)
- `Results() <-chan JobResult` - Get the results channel (thread-safe)
- `Shutdown(ctx context.Context) error` - Gracefully shutdown (thread-safe, call once)
- `Metrics() *Metrics` - Get current metrics (thread-safe)
- `Workers() []*Worker` - Get worker information (thread-safe)

### Metrics

The WorkerPool tracks comprehensive metrics:

```go
metrics := pool.Metrics()
fmt.Printf("Jobs: %d submitted, %d completed, %d failed\n",
    metrics.JobsSubmitted.Load(),
    metrics.JobsCompleted.Load(),
    metrics.JobsFailed.Load(),
)
fmt.Printf("Latency: avg=%v, min=%v, max=%v\n",
    metrics.AverageLatency(),
    time.Duration(metrics.MinLatency.Load()),
    time.Duration(metrics.MaxLatency.Load()),
)
fmt.Printf("Workers: %d active, %d idle\n",
    metrics.ActiveWorkers.Load(),
    metrics.IdleWorkers.Load(),
)
```

### Worker Status

Monitor individual worker health:

```go
workers := pool.Workers()
for _, worker := range workers {
    fmt.Printf("Worker %d: status=%s, jobs=%d, last_error=%v\n",
        worker.ID(),
        worker.Status(),
        worker.JobsProcessed(),
        worker.LastError(),
    )
}
```

### Thread Safety

All WorkerPool public methods are thread-safe and can be called concurrently:
- `Submit()` - Safe for concurrent job submission
- `Results()` - Safe for multiple readers
- `Metrics()` - Uses atomic operations for safe access
- `Workers()` - Returns a copy, safe for concurrent access

### Error Handling

```go
// Submit returns error if pool not started or queue full
if err := pool.Submit(ctx, job); err != nil {
    if err.Error() == "job queue is full" {
        // Handle backpressure
    }
}

// Shutdown returns error if timeout exceeded
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := pool.Shutdown(ctx); err != nil {
    log.Printf("Shutdown timeout: %v", err)
}
```

### Best Practices for WorkerPool

1. **Configure appropriate worker count**: Set `MaxWorkers` based on CPU cores and I/O requirements
2. **Set reasonable timeouts**: Configure `WorkerTimeout` to prevent hung jobs
3. **Monitor metrics**: Regularly check metrics to detect performance issues
4. **Handle backpressure**: Check for "queue is full" errors and implement retry logic
5. **Process results asynchronously**: Read from `Results()` channel in a separate goroutine
6. **Graceful shutdown**: Always call `Shutdown()` with appropriate timeout
7. **Use context for cancellation**: Pass context to jobs for early cancellation

---

## Best Practices

1. **Always use defer for cleanup**
   ```go
   orchestrator := concurrency.NewOrchestrator(config)
   defer orchestrator.Shutdown(30 * time.Second)
   ```

2. **Monitor events in a separate goroutine**
   ```go
   go func() {
       for event := range orchestrator.EventChannel() {
           // Handle events
       }
   }()
   ```

3. **Set appropriate timeouts for tasks**
   ```go
   task.Timeout = 5 * time.Minute // Prevent indefinite execution
   ```

4. **Use task affinity for specialized workloads**
   ```go
   task.Affinity = []string{"specialized-agent-1"}
   ```

5. **Monitor agent health and metrics regularly**
   ```go
   metrics := orchestrator.GetMetrics()
   if metrics["failed_agents"].(int) > 0 {
       // Handle failed agents
   }
   ```

6. **Configure concurrency limits based on resources**
   ```go
   config.MaxConcurrentTasks = runtime.NumCPU() * 2
   ```

7. **Handle task results asynchronously**
   ```go
   go func() {
       result := <-task.ResultChan
       // Process result
   }()
   ```

## Performance Considerations

- **Load Balancing**: "least-loaded" algorithm provides better distribution but has slightly higher overhead
- **Health Checks**: Balance frequency with performance impact (recommended: 30-60 seconds)
- **Task Queue Size**: Set based on expected workload (100-1000 for most use cases)
- **Concurrency Limits**: Set based on available system resources and agent requirements

## Error Handling

The orchestrator provides comprehensive error handling:

1. **Task Distribution Errors**: Returned immediately if queue is full or orchestrator is shutting down
2. **Agent Errors**: Caught by circuit breaker and health checks
3. **Health Check Failures**: Trigger events and potentially open circuit breaker
4. **Shutdown Errors**: Logged and aggregated for debugging

## Thread Safety

All public methods are thread-safe and can be called concurrently:
- Agent registration/removal
- Task distribution
- Metrics retrieval
- Agent state management

## Testing

See `orchestrator_example.go` for comprehensive examples of:
- Basic usage
- Load balancing strategies
- Circuit breaker behavior
- Event-driven architecture
- Task affinity
- Concurrency limits

## License

Part of the claude-squad project.

## Contributing

When extending the orchestrator:
1. Maintain thread safety using mutexes
2. Add comprehensive documentation
3. Update examples
4. Consider backward compatibility
5. Add appropriate logging

## Future Enhancements

Potential improvements:
- Persistent task queue (survive restarts)
- Distributed orchestration across multiple machines
- Advanced scheduling algorithms (deadline-based, cost-optimized)
- Agent pooling and autoscaling
- Integration with monitoring systems (Prometheus, Grafana)
- Task dependencies and workflows
- Priority queues with preemption
