# Resource Manager

A production-quality, smart resource manager for Go with rate limiting, semaphore-based concurrency control, and deadlock prevention.

## Overview

The Resource Manager provides comprehensive resource allocation and management capabilities with the following features:

- **Resource Pools**: CPU, Memory, File Handles, and Network resources
- **Token Bucket Rate Limiting**: Prevents resource exhaustion through controlled allocation rates
- **Semaphore-Based Concurrency Control**: Limits concurrent resource usage
- **Per-Agent Resource Quotas**: Enforces limits on individual agents
- **Dynamic Scaling**: Automatically adjusts capacity based on load
- **Deadlock Detection**: Uses wait-for graph analysis to prevent deadlocks
- **Comprehensive Statistics**: Tracks usage, peak loads, and performance metrics

## Architecture

### Core Components

#### 1. TokenBucket
Implements the token bucket algorithm for rate limiting:
- Configurable capacity and refill rate
- Automatic token refilling via background goroutine
- Context-aware blocking and non-blocking acquisition
- Thread-safe using `sync.Mutex` and `sync.Cond`

```go
tb, err := NewTokenBucket(capacity, refillRate)
defer tb.Stop()

// Blocking acquire
err = tb.Acquire(ctx, tokens)

// Non-blocking try acquire
if tb.TryAcquire(tokens) {
    // Success
}
```

#### 2. Semaphore
Counting semaphore for concurrency control:
- Limits concurrent access to resources
- Context-aware waiting
- Supports both acquire and try-acquire patterns

```go
sem, err := NewSemaphore(capacity)

// Acquire permits
err = sem.Acquire(ctx, n)
defer sem.Release(n)
```

#### 3. ResourcePool
Manages a specific type of resource:
- Combines semaphore and token bucket for dual control
- Tracks allocation statistics
- Supports dynamic capacity changes
- Records acquisition times and failures

```go
pool, err := NewResourcePool(CPU, capacity, rateLimit)
defer pool.Stop()

err = pool.Acquire(ctx, amount)
defer pool.Release(amount)

usage := pool.Usage() // Returns percentage
```

#### 4. ResourceQuota
Enforces per-agent resource limits:
- Configurable quotas per resource type
- Tracks current usage per agent
- Prevents quota violations

```go
quota := NewResourceQuota()
quota.SetQuota(agentID, CPU, limit)

err := quota.CheckQuota(agentID, CPU, amount)
```

#### 5. LoadMonitor
Monitors resource usage and triggers auto-scaling:
- Configurable scale-up and scale-down thresholds
- Periodic load checking
- Callback-based notifications
- Automatic capacity adjustments

```go
monitor := NewLoadMonitor(scaleUpThreshold, scaleDownThreshold, interval)
monitor.RegisterCallback(func(rt ResourceType, load float64) {
    fmt.Printf("Resource %s at %.2f%% load\n", rt, load)
})
monitor.Start(rm)
```

#### 6. DeadlockDetector
Detects potential deadlocks using wait-for graph:
- Maintains graph of resource dependencies
- Cycle detection using depth-first search
- Can be enabled/disabled via configuration
- Tracks resource holders and waiters

```go
detector := NewDeadlockDetector(enabled)

// Records are automatically maintained by ResourceManager
err := detector.RecordWait(agentID, resourceType)
if err == ErrDeadlockDetected {
    // Handle deadlock
}
```

#### 7. ResourceManager
Orchestrates all components:
- Central API for resource management
- Coordinates pools, quotas, monitoring, and deadlock detection
- Provides unified interface for acquisition and release

## Usage

### Basic Example

```go
// Create resource manager with default configuration
config := DefaultResourceManagerConfig()
rm, err := NewResourceManager(config)
if err != nil {
    log.Fatal(err)
}
defer rm.Stop()

// Set quota for an agent
rm.SetQuota("agent1", CPU, 50)

// Acquire resources
ctx := context.Background()
if err := rm.Acquire(ctx, "agent1", CPU, 10); err != nil {
    log.Fatal(err)
}
defer rm.Release("agent1", CPU, 10)

// Use resources...
```

### Configuration

```go
config := &Config{
    CPUCapacity:             100,
    MemoryCapacity:          1024 * 1024 * 1024, // 1GB
    FileHandlesCapacity:     1000,
    NetworkCapacity:         100,
    RateLimit:               50, // tokens per second
    EnableDeadlockDetection: true,
    ScaleUpThreshold:        80.0,  // scale up at 80% usage
    ScaleDownThreshold:      20.0,  // scale down at 20% usage
    MonitorInterval:         5 * time.Second,
}

rm, err := NewResourceManager(config)
```

### Advanced Features

#### Context Cancellation

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := rm.Acquire(ctx, agentID, CPU, amount); err != nil {
    if err == context.DeadlineExceeded {
        // Handle timeout
    }
}
```

#### Try Acquire (Non-blocking)

```go
acquired, err := rm.TryAcquire(agentID, CPU, amount)
if !acquired {
    // Resource not available, handle accordingly
}
defer rm.Release(agentID, CPU, amount)
```

#### Load Monitoring

```go
rm.RegisterLoadCallback(func(rt ResourceType, load float64) {
    if load > 90.0 {
        log.Printf("High load on %s: %.2f%%", rt, load)
    }
})
```

#### Dynamic Capacity Adjustment

```go
// Manually adjust capacity
if err := rm.SetPoolCapacity(CPU, newCapacity); err != nil {
    log.Printf("Failed to adjust capacity: %v", err)
}

// Auto-scaling happens automatically based on thresholds
```

#### Statistics and Monitoring

```go
// Get pool usage
usage, err := rm.GetPoolUsage(CPU)
fmt.Printf("CPU usage: %.2f%%\n", usage)

// Get detailed statistics
current, peak, acquisitions, failures, avgWait, err := rm.GetPoolStats(CPU)
fmt.Printf("Stats: current=%d, peak=%d, acquisitions=%d, failures=%d, avgWait=%v\n",
    current, peak, acquisitions, failures, avgWait)

// Get agent usage
usage := rm.GetUsage(agentID, CPU)
fmt.Printf("Agent %s CPU usage: %d\n", agentID, usage)
```

## Resource Types

The system supports four resource types:

- `CPU`: Computational resources
- `Memory`: Memory allocation
- `FileHandles`: File descriptors/handles
- `Network`: Network connections

## Thread Safety

All components are thread-safe and use appropriate synchronization primitives:

- `sync.Mutex`: For exclusive access to shared state
- `sync.RWMutex`: For read-heavy workloads
- `sync.Cond`: For efficient waiting and signaling
- Atomic operations where applicable

## Error Handling

The system defines clear error types:

- `ErrResourceExhausted`: Resources temporarily unavailable
- `ErrQuotaExceeded`: Agent quota limit reached
- `ErrDeadlockDetected`: Potential deadlock detected
- `ErrInvalidCapacity`: Invalid capacity specified
- `ErrInvalidRate`: Invalid rate limit specified
- `ErrResourceNotAcquired`: Attempting to release unacquired resource

## Performance Considerations

### Benchmarks

The implementation is optimized for high-throughput scenarios:

```bash
go test -bench=. -benchmem claude-squad/concurrency
```

### Best Practices

1. **Reuse ResourceManager instances**: Create once, use throughout application lifetime
2. **Set appropriate quotas**: Prevent individual agents from monopolizing resources
3. **Monitor statistics**: Use callbacks to track resource usage patterns
4. **Handle errors**: Always check for quota exceeded and deadlock errors
5. **Use context timeouts**: Prevent indefinite blocking on resource acquisition
6. **Clean shutdown**: Always call `Stop()` to clean up background goroutines

## Testing

Comprehensive test suite included:

```bash
# Run all tests
go test -v claude-squad/concurrency -run "^TestTokenBucket|^TestSemaphore|^TestResource"

# Run specific test
go test -v claude-squad/concurrency -run TestResourceManager$

# Run with race detection
go test -race claude-squad/concurrency

# Run benchmarks
go test -bench=BenchmarkResourceManager -benchmem claude-squad/concurrency
```

## Examples

See `resource_manager_example.go` for complete examples:

- Basic usage
- Deadlock detection
- Context cancellation
- Dynamic scaling
- Rate limiting
- Semaphore usage

Run examples:

```go
ExampleResourceManager()
ExampleResourceManagerWithDeadlockDetection()
ExampleResourceManagerWithContextCancellation()
ExampleResourceManagerDynamicScaling()
ExampleResourceManagerRateLimiting()
ExampleResourceManagerSemaphore()
```

## Design Patterns

### 1. Token Bucket Algorithm
Controls the rate at which resources can be acquired, smoothing out bursts and preventing overload.

### 2. Semaphore Pattern
Limits concurrent access to resources, ensuring capacity is not exceeded.

### 3. Wait-For Graph
Detects circular dependencies in resource allocation that could lead to deadlocks.

### 4. Observer Pattern
LoadMonitor uses callbacks to notify interested parties of load changes.

### 5. Resource Acquisition Is Initialization (RAII)
Use `defer` to ensure resources are always released:
```go
rm.Acquire(ctx, agentID, CPU, amount)
defer rm.Release(agentID, CPU, amount)
```

## Limitations

1. **No priority-based acquisition**: Resources allocated on FIFO basis
2. **No resource preemption**: Cannot reclaim resources from active users
3. **Fixed resource types**: Four predefined types (CPU, Memory, FileHandles, Network)
4. **In-memory only**: State not persisted across restarts

## Future Enhancements

Potential improvements for future versions:

- Priority-based resource allocation
- Resource preemption with callbacks
- Pluggable resource types
- Distributed resource management
- Persistent state
- Metrics export (Prometheus, etc.)
- Adaptive rate limiting based on errors
- Resource reservation system

## License

Same as parent project.

## Contributing

Contributions welcome! Please ensure:
- All tests pass
- Code is properly formatted (`go fmt`)
- No race conditions (`go test -race`)
- Comprehensive test coverage for new features
