# Task Router Implementation Guide

## Overview

The `ollama/router.go` package provides an intelligent task routing system with load balancing for distributing tasks across multiple Ollama model instances. It features multiple routing strategies, performance-based optimization, circuit breaker patterns, and model affinity tracking.

## Core Components

### 1. TaskRouter

The main router struct that orchestrates task distribution:

```go
router := NewTaskRouter(StrategyRoundRobin)
```

### 2. Routing Strategies

Six intelligent routing strategies are available:

- **Round-Robin** (`StrategyRoundRobin`): Distributes tasks evenly across models in sequence
- **Least-Loaded** (`StrategyLeastLoaded`): Routes to model with fewest pending tasks
- **Random** (`StrategyRandom`): Routes to random available model
- **Performance-Based** (`StrategyPerformance`): Routes based on model success rates and latency
- **Affinity-Based** (`StrategyAffinity`): Routes to models that previously succeeded with similar tasks
- **Hybrid** (`StrategyHybrid`): Combines affinity and performance strategies

### 3. Task Categories

Tasks are automatically categorized for smarter routing:

- `TaskCoding`: Implementation and feature development
- `TaskRefactoring`: Code cleanup and optimization
- `TaskTesting`: Test writing and validation
- `TaskDocumentation`: Documentation and comments
- `TaskDebugging`: Bug fixes and error investigation
- `TaskCodeReview`: Code review and approval tasks

## Quick Start

### Basic Usage

```go
// Create a router with round-robin strategy
router := NewTaskRouter(StrategyRoundRobin)

// Register models
instance, _ := session.NewInstance(session.InstanceOptions{
    Title:   "model-1",
    Path:    "/tmp/model-1",
    Program: "claude",
})
router.RegisterModel("model-1", instance)

// Route a task
selectedModel, err := router.RouteTask("implement a binary search algorithm")
if err != nil {
    log.Fatal(err)
}

// Record the result
latency := time.Duration(100 * time.Millisecond)
success := true
category := router.GetTaskCategory("implement a binary search algorithm")
router.RecordTaskResult(selectedModel, success, latency, category)

// Get metrics
metrics, _ := router.GetModelMetrics(selectedModel)
fmt.Printf("Model %s success rate: %.1f%%\n",
    selectedModel,
    float64(metrics.SuccessfulTasks) / float64(metrics.TotalRequests) * 100)
```

### Choosing a Strategy

**Use Round-Robin when:**
- Models have similar capabilities
- You want simple, predictable distribution
- Models should be utilized equally

**Use Least-Loaded when:**
- Models have different processing speeds
- You want to minimize queue depths
- Fairness in task distribution is important

**Use Performance-Based when:**
- Models have different reliability levels
- You want to maximize success rates
- Latency variation is significant

**Use Affinity-Based when:**
- You have task-specialized models
- Task types repeat frequently
- Performance improves with task familiarity

**Use Hybrid when:**
- You want dynamic strategy selection
- You have diverse task types and models
- You want to adapt automatically

## Features

### 1. Load Balancing

Distributes load based on selected strategy:

```go
// Switch strategies at runtime
router.SetRoutingStrategy(StrategyPerformance)

// Get current distribution
metrics := router.GetAllMetrics()
for modelID, m := range metrics {
    utilization := float64(m.TotalRequests) / float64(m.SuccessfulTasks)
    fmt.Printf("%s utilization: %.2f\n", modelID, utilization)
}
```

### 2. Model Affinity

Tracks which models excel at specific task types:

```go
// Affinity is built automatically through task recording
for i := 0; i < 10; i++ {
    router.RecordTaskResult("coding-specialist", true, 100*time.Millisecond, TaskCoding)
}

// Routes similar tasks to the same model
selectedModel, _ := router.RouteTask("implement a new function")
// Likely selects "coding-specialist"
```

### 3. Circuit Breaker Pattern

Automatically detects and isolates failing models:

```go
// Record failures
for i := 0; i < 6; i++ {
    router.RecordTaskResult("flaky-model", false, 100*time.Millisecond, TaskCoding)
}

// Circuit breaker automatically opens
isOpen, _ := router.GetCircuitBreakerStatus("flaky-model")
// isOpen == true

// Model is excluded from routing until recovery
selectedModel, _ := router.RouteTask("some task")
// selectedModel != "flaky-model"

// Manually recover if needed
router.ForceHealthRecovery("flaky-model")
```

### 4. Performance Tracking

Detailed metrics for each model:

```go
metrics, _ := router.GetModelMetrics("model-1")

fmt.Printf("Total Requests: %d\n", metrics.TotalRequests)
fmt.Printf("Successful Tasks: %d\n", metrics.SuccessfulTasks)
fmt.Printf("Failed Tasks: %d\n", metrics.FailedTasks)
fmt.Printf("Average Latency: %v\n", metrics.AverageLatency)
fmt.Printf("Circuit Breaker Open: %v\n", metrics.CircuitBreakerOpen)
```

### 5. Health Checks

Monitor overall system health:

```go
health := router.HealthCheck()
for modelID, isHealthy := range health {
    if !isHealthy {
        log.Printf("Model %s is unhealthy", modelID)
    }
}
```

## Advanced Usage

### Custom Task Category Detection

```go
type MyDetector struct{}

func (d *MyDetector) Detect(prompt string) TaskCategory {
    // Custom categorization logic
    if strings.Contains(prompt, "optimize") {
        return TaskRefactoring
    }
    return TaskUnknown
}

router.SetTaskCategoryDetector(&MyDetector{})
```

### Circuit Breaker Configuration

```go
router.circuitBreakerConfig = CircuitBreakerConfig{
    FailureThreshold: 5,           // Open after 5 failures
    SuccessThreshold: 3,           // Close after 3 successes
    Timeout:          30 * time.Second, // Recovery attempt interval
    HalfOpenRequests: 2,           // Requests allowed in half-open state
}
```

### Metrics Reset

```go
// Useful for testing or periodic resets
router.ResetMetrics()
```

## Architecture Pattern

```
TaskRouter
├── RouteTask(prompt)
│   ├── Detect Category (TaskCategoryDetector)
│   ├── Get Available Models (HealthCheck + CircuitBreaker)
│   └── Apply Strategy
│       ├── Round-Robin
│       ├── Least-Loaded
│       ├── Random
│       ├── Performance-Based
│       ├── Affinity-Based
│       └── Hybrid
│
├── RecordTaskResult(modelID, success, latency, category)
│   ├── Update Metrics
│   ├── Check Circuit Breaker
│   └── Update Affinity Map
│
└── HealthCheck()
    └── Evaluate Model Status
```

## Example Workflows

### Workflow 1: Performance Optimization

```go
// Start with round-robin
router := NewTaskRouter(StrategyRoundRobin)

// Build performance history with real tasks
for {
    task := getNextTask()
    model, _ := router.RouteTask(task)

    success, latency := executeTask(model, task)
    category := router.GetTaskCategory(task)

    router.RecordTaskResult(model, success, latency, category)
}

// Switch to performance-based for optimization
router.SetRoutingStrategy(StrategyPerformance)
```

### Workflow 2: Specialized Model Pool

```go
router := NewTaskRouter(StrategyAffinity)

// Register specialized models
models := map[string][]string{
    "coding-model": {"implement", "write", "create"},
    "test-model":   {"test", "verify", "validate"},
    "doc-model":    {"document", "comment", "explain"},
}

// Affinity automatically develops through usage
for task := range tasks {
    model, _ := router.RouteTask(task)
    success := executeTask(model, task)
    router.RecordTaskResult(model, success, 100*time.Millisecond, TaskCoding)
}
```

### Workflow 3: Failover with Circuit Breaker

```go
router := NewTaskRouter(StrategyLeastLoaded)

// Register primary and backup models
router.RegisterModel("primary-model", primaryInstance)
router.RegisterModel("backup-model", backupInstance)

// Circuit breaker automatically failsover
task := "critical-task"
model, _ := router.RouteTask(task)
success := executeTask(model, task)

if !success {
    // Record failure
    router.RecordTaskResult(model, false, latency, category)
}

// Next tasks automatically avoid primary-model if circuit opens
nextModel, _ := router.RouteTask("another-task")
// Will route to backup-model if primary has too many failures
```

## Performance Considerations

### Concurrency
- Thread-safe with `sync.RWMutex`
- Safe for concurrent routing and metric recording
- No locks during task execution

### Memory
- Metrics stored per model
- Affinity map grows with task categories and models
- Periodic reset recommended for long-running processes

### Latency
- Routing decision: O(n) where n = number of models
- Metric recording: O(1)
- Health check: O(n)

## Integration with Instance Management

The router integrates seamlessly with the existing Instance management:

```go
// Get instance for routed model
instance, _ := router.modelPool.GetInstance(selectedModel)

// Send prompt to instance
instance.SendPrompt(task)

// Monitor instance
updated, hasPrompt := instance.HasUpdated()
```

## Testing

Comprehensive test suite included:

```bash
go test -v ./ollama/router_test.go ./ollama/router.go ./ollama/router_examples.go
```

Key test cases:
- Registration and unregistration
- All routing strategies
- Circuit breaker behavior
- Affinity building
- Metrics tracking
- Concurrent operations

## Examples

Run included examples:

```go
// Example 1: Basic round-robin
Example1_BasicRoundRobinRouting()

// Example 2: Performance-based routing
Example2_PerformanceBasedRouting()

// Example 3: Model affinity
Example3_ModelAffinityRouting()

// Example 4: Circuit breaker
Example4_CircuitBreakerPattern()

// Example 5: Hybrid strategy
Example5_HybridRoutingStrategy()

// Example 6: Dynamic strategy switching
Example6_DynamicStrategySwapping()

// Example 7: Task categorization
Example7_TaskCategoryDetection()

// Example 8: Metrics analysis
Example8_MetricsTracking()

// Run all examples
RunAllExamples()
```

## Best Practices

1. **Start with Round-Robin**: Simple and predictable for baseline performance
2. **Collect Metrics**: Let models accumulate history before switching strategies
3. **Monitor Circuit Breaker**: Alert on circuit breaker openings
4. **Use Affinity for Specialization**: Assign task types to specialized models
5. **Periodic Health Checks**: Run health checks during idle periods
6. **Reset Metrics Periodically**: Prevent stale data from affecting decisions
7. **Log Strategy Changes**: Track when and why strategies change
8. **Test Failover Paths**: Verify circuit breaker and backup models work

## Troubleshooting

### All tasks going to one model
- Check if affinity is too strong
- Reset metrics and rebuild from scratch
- Switch to least-loaded strategy

### Circuit breaker constantly opens/closes
- Increase failure threshold
- Extend timeout duration
- Investigate underlying model instability

### Poor performance after strategy switch
- Insufficient history: let metrics accumulate first
- Wrong strategy for workload: try different strategies
- Stale metrics: reset and rebuild history

### Uneven load distribution
- Verify all models are healthy (health check)
- Check for circuit breaker states
- Ensure strategy is set correctly

## Future Enhancements

Potential improvements:
- Weighted load balancing
- Task priority queues
- Model resource monitoring (CPU, memory)
- Dynamic threshold adjustment
- Multi-datacenter routing
- Advanced ML-based strategy selection
- Persistent metrics storage
