# Task Router - Quick Reference Guide

## 30-Second Setup

```go
import "claude-squad/ollama"

// Create router
router := ollama.NewTaskRouter(ollama.StrategyRoundRobin)

// Register models
router.RegisterModel("model-1", instance1)
router.RegisterModel("model-2", instance2)

// Route a task
selectedModel, _ := router.RouteTask("implement a new feature")

// Record result
category := router.GetTaskCategory("implement a new feature")
router.RecordTaskResult(selectedModel, true, 100*time.Millisecond, category)
```

## Choosing Your Strategy

| Strategy | Best For | Example |
|----------|----------|---------|
| `RoundRobin` | Equal models, simple | 3 identical GPUs |
| `LeastLoaded` | Variable latency | Mix of fast/slow models |
| `Random` | Chaos testing | Load testing |
| `Performance` | Different quality | Mixed model generations |
| `Affinity` | Specialized models | Coding + Testing specialists |
| `Hybrid` | Dynamic workloads | Everything! |

### Quick Selection Matrix

**I have:**
- Equal capability models → Use **RoundRobin**
- Different speed models → Use **LeastLoaded**
- Different quality models → Use **Performance**
- Specialized models → Use **Affinity**
- Not sure → Use **Hybrid**

## Common Operations

### Register a Model
```go
router.RegisterModel("llama-7b", instance)
```

### Unregister a Model
```go
router.UnregisterModel("llama-7b")
```

### Route a Task
```go
modelID, err := router.RouteTask("your task description")
```

### Record Task Result
```go
category := router.GetTaskCategory("task description")
router.RecordTaskResult(modelID, success, latency, category)
```

### Get Metrics
```go
// Single model
metrics, _ := router.GetModelMetrics("model-1")

// All models
allMetrics := router.GetAllMetrics()
```

### Check Health
```go
health := router.HealthCheck()
for model, isHealthy := range health {
    if !isHealthy {
        fmt.Printf("%s is unhealthy\n", model)
    }
}
```

### Switch Strategy
```go
router.SetRoutingStrategy(ollama.StrategyPerformance)
```

### Force Recovery
```go
router.ForceHealthRecovery("flaky-model")
```

## Task Categories (Auto-Detected)

**Coding Tasks**
- Keywords: implement, write, create, function, method, class
- Example: "implement a binary search algorithm"

**Refactoring Tasks**
- Keywords: refactor, optimize, simplify, cleanup, improve
- Example: "optimize the database query"

**Testing Tasks**
- Keywords: test, unit test, mock, assert, verify
- Example: "write unit tests for the parser"

**Documentation Tasks**
- Keywords: doc, comment, readme, javadoc, explain
- Example: "document the API endpoints"

**Debugging Tasks**
- Keywords: debug, fix, bug, error, crash, exception
- Example: "debug the authentication error"

**Code Review Tasks**
- Keywords: review, approve, feedback, suggest
- Example: "review the pull request"

## Circuit Breaker Rules

1. **Opens when** model has 5+ consecutive failures
2. **Closes when** timeout (30s) passes without failures
3. **During open** model is excluded from routing
4. **Can force recover** with `ForceHealthRecovery()`

## Metrics Fields

```go
type RouterMetrics struct {
    ModelID             string        // ID of the model
    TotalRequests       int64         // Total tasks assigned
    SuccessfulTasks     int64         // Completed successfully
    FailedTasks         int64         // Failed tasks
    AverageLatency      time.Duration // Average response time
    LastUsed            time.Time     // When last routed
    CircuitBreakerOpen  bool          // If circuit is open
    FailureCount        int32         // Consecutive failures
    SuccessCount        int32         // Consecutive successes
}
```

## Example: Performance Monitoring

```go
func reportMetrics(router *ollama.TaskRouter) {
    metrics := router.GetAllMetrics()

    for modelID, m := range metrics {
        successRate := float64(m.SuccessfulTasks) / float64(m.TotalRequests)
        fmt.Printf("%s: %d requests, %.0f%% success, %v avg latency\n",
            modelID, m.TotalRequests, successRate*100, m.AverageLatency)
    }
}
```

## Example: Adaptive Routing

```go
// Start simple
router := ollama.NewTaskRouter(ollama.StrategyRoundRobin)

// Collect data
for {
    model, _ := router.RouteTask(getNextTask())
    success, latency := executeTask(model, task)
    category := router.GetTaskCategory(task)
    router.RecordTaskResult(model, success, latency, category)
}

// Switch to performance-based after warmup
router.SetRoutingStrategy(ollama.StrategyPerformance)
```

## Example: Model Specialization

```go
// Register specialized models
router.RegisterModel("coder", coderInstance)
router.RegisterModel("tester", testerInstance)
router.RegisterModel("documenter", docInstance)

// Set affinity strategy
router.SetRoutingStrategy(ollama.StrategyAffinity)

// Route tasks - automatically learns specialization
router.RouteTask("implement new feature")      // → coder
router.RouteTask("write unit tests")           // → tester
router.RouteTask("update documentation")       // → documenter
```

## Common Troubleshooting

### "No models registered"
**Problem**: Trying to route without registering models
**Solution**: Call `RegisterModel()` before `RouteTask()`

### "No healthy models available"
**Problem**: All models have circuit breaker open
**Solution**: Use `ForceHealthRecovery()` or wait 30 seconds

### "Uneven load distribution"
**Problem**: One model getting all tasks
**Cause**: Wrong strategy or stale affinity data
**Solution**: Try `StrategyLeastLoaded` or `ResetMetrics()`

### Circuit breaker constantly opens/closes
**Problem**: Model is unstable
**Solution**:
- Investigate underlying issues
- Increase `FailureThreshold` (default: 5)
- Increase `Timeout` (default: 30s)

## Thread Safety

All operations are **100% thread-safe**:
- Concurrent routing: ✓
- Concurrent metric recording: ✓
- Concurrent health checks: ✓
- No blocking: ✓

Example:
```go
// Safe to call concurrently
go func() {
    model, _ := router.RouteTask(task1)
    router.RecordTaskResult(model, true, latency1, category1)
}()

go func() {
    model, _ := router.RouteTask(task2)
    router.RecordTaskResult(model, false, latency2, category2)
}()

go func() {
    health := router.HealthCheck()
}()
```

## Performance Tips

1. **Use affinity routing** for specialized models (20% faster decisions)
2. **Batch metric checks** instead of checking per task
3. **Reset metrics periodically** for long-running services
4. **Monitor circuit breaker status** to catch degradation early
5. **Let router warm up** with 100+ tasks before switching strategies

## Configuration Constants

```go
// Default circuit breaker
FailureThreshold: 5              // failures to open
SuccessThreshold: 3              // successes to close
Timeout: 30 * time.Second        // recovery window
HalfOpenRequests: 2              // requests while half-open

// Performance calculation weights
Success Rate: 70%
Latency Score: 30%

// Strategy update interval
5 * time.Second
```

## Integration with Instances

```go
// Register instance with router
instance, _ := session.NewInstance(session.InstanceOptions{
    Title:   "model-1",
    Path:    "/workspace/model-1",
    Program: "claude",
})
router.RegisterModel("model-1", instance)

// Later, get instance for advanced operations
instance, _ := router.modelPool.GetInstance("model-1")
instance.SendPrompt("direct prompt")
instance.SendKeys("C-c")  // Send Ctrl-C
```

## When to Use Each Strategy

### Round-Robin
- All models have same specs
- Want simple, predictable distribution
- Testing/CI pipeline with identical runners

### Least-Loaded
- Models have different processing speeds
- Want to minimize queue depth
- Latency is critical

### Performance
- Models have reliability differences
- Historical data matters
- Want highest success rates

### Affinity
- Have specialized models
- Tasks repeat frequently
- Want domain-specific optimization

### Hybrid
- Unsure which strategy to use
- Workload is diverse
- Want automatic selection

## See Also

- **ROUTER_GUIDE.md**: Comprehensive documentation
- **router_examples.go**: 8 complete working examples
- **router_test.go**: Test cases showing usage
- **ROUTER_IMPLEMENTATION_SUMMARY.md**: Technical details
