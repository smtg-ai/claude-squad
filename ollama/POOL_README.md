# Agent Pool Manager with Auto-Scaling

A robust, feature-rich agent pool manager implementation providing dynamic scaling, resource management, and comprehensive metrics for managing `session.Instance` objects.

## Overview

The Agent Pool Manager (`AgentPool`) is designed to efficiently manage a pool of `session.Instance` agents with the following capabilities:

- **Dynamic Auto-Scaling**: Automatically scales pool size based on workload (1-10 agents)
- **Agent Lifecycle Management**: Spawn, kill, and recycle agents with proper resource cleanup
- **Resource Quotas**: Enforce memory, CPU, age, and request limits
- **Warm Pool Maintenance**: Keep a minimum number of ready agents available
- **Comprehensive Metrics**: Track active, idle, requests, and recycles
- **Thread-Safe Operations**: Uses sync.Pool, atomic operations, and RWMutex
- **Storage Integration**: Persist and restore pool state via session.Storage
- **Context Support**: Proper context cancellation and timeout handling

## Core Components

### Agent

Wraps a `session.Instance` with pool-specific metadata:

```go
type Agent struct {
    instance      *session.Instance
    state         AgentState
    lastUsedAt    time.Time
    totalRequests int64
    recycleCount  int32
    // ...
}
```

**Agent States:**
- `AgentStateIdle`: Available and waiting for use
- `AgentStateActive`: Currently in use
- `AgentStateRecycling`: Being recycled
- `AgentStateTerminated`: Terminated and cleaned up

### AgentPool

Main pool manager struct:

```go
type AgentPool struct {
    agents          map[string]*Agent
    availableQueue  chan *Agent
    minPoolSize     int
    maxPoolSize     int
    activeCount     atomic.Int64
    idleCount       atomic.Int64
    totalRequests   atomic.Int64
    totalRecycles   atomic.Int64
    // ... resource quotas, storage, metrics
}
```

### ResourceQuota

Defines resource limits:

```go
type ResourceQuota struct {
    MaxMemoryMB      int64
    MaxCPUPercent    float64
    MaxInstanceAge   time.Duration
    MaxRecyclesPerID int32
    RequestsPerQuota int64
}
```

### PoolMetrics

Snapshot of pool statistics:

```go
type PoolMetrics struct {
    ActiveAgents     int64
    IdleAgents       int64
    TotalAgents      int64
    TotalRequests    int64
    TotalRecycles    int64
    SpawnedAgents    int64
    TerminatedAgents int64
    LastScaleEvent   time.Time
    ScaleDirection   string // "UP", "DOWN"
}
```

## Usage Examples

### Basic Pool Creation

```go
config := DefaultPoolConfig()
config.MinPoolSize = 2
config.MaxPoolSize = 5

pool, err := NewAgentPool(config)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()
```

### Acquiring and Releasing Agents

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Acquire an agent
agent, err := pool.Acquire(ctx)
if err != nil {
    log.Fatal(err)
}

// Use the agent
instance := agent.GetInstance()
instance.SendPrompt("your command")

// Release back to pool
pool.Release(agent)
```

### Monitoring Metrics

```go
// Get pool size
active, idle, total := pool.GetPoolSize()
fmt.Printf("Active: %d, Idle: %d, Total: %d\n", active, idle, total)

// Get detailed metrics
metrics := pool.GetMetrics()
fmt.Printf("Total requests: %d\n", metrics.TotalRequests)
fmt.Printf("Total recycles: %d\n", metrics.TotalRecycles)

// Get comprehensive stats
stats := pool.GetAgentPoolStats()
```

### Configuring Resource Quotas

```go
config := DefaultPoolConfig()
config.ResourceQuota = ResourceQuota{
    MaxMemoryMB:      1024,           // 1GB per instance
    MaxCPUPercent:    90.0,           // 90% CPU threshold
    MaxInstanceAge:   2 * time.Hour,  // Recycle after 2 hours
    MaxRecyclesPerID: 100,            // Max recycles per agent
    RequestsPerQuota: 5000,           // Track requests
}

pool, _ := NewAgentPool(config)
```

### Auto-Scaling Configuration

```go
config := PoolConfig{
    MinPoolSize:         1,     // Minimum agents
    MaxPoolSize:         10,    // Maximum agents (hard cap)
    IdleTimeout:         5 * time.Minute,
    RecycleThreshold:    1000,  // Requests before recycle
    MaintenanceInterval: 30 * time.Second,
    ResourceQuota:       DefaultPoolConfig().ResourceQuota,
}

pool, _ := NewAgentPool(config)
```

## Key Features

### 1. Dynamic Auto-Scaling

The pool automatically scales based on utilization:

- **Scale Up**: When utilization > 80% and below max size
- **Scale Down**: When utilization < 20% and above min size
- Runs during periodic maintenance (configurable interval)

### 2. Agent Lifecycle Management

#### Spawning
```go
agent, err := pool.spawnAgent()
// Creates new session.Instance and starts it
```

#### Recycling
```go
err := pool.recycleAgent(agent)
// Kills old agent and spawns replacement
// Triggered by:
// - Max instance age exceeded
// - Recycle count threshold reached
// - Request count threshold exceeded
```

#### Killing
```go
err := pool.killAgent(agent)
// Calls instance.Kill() with proper cleanup
```

### 3. Warm Pool Maintenance

```go
// Ensure minimum agents are ready
pool.WarmPool()

// Drain excess idle agents
pool.DrainPool()
```

### 4. Thread-Safe Operations

- Uses `atomic.Int64` for counters (lock-free)
- `sync.RWMutex` for agent map access
- `sync.Pool` for object reuse (optional feature)
- Safe concurrent acquire/release operations

### 5. Health Checking

```go
// Health checks on acquire
if !pool.isAgentHealthy(agent) {
    pool.killAgent(agent)
    return pool.Acquire(ctx) // Retry
}
```

Checks:
- Agent state is not Terminated
- Instance is initialized
- Instance is still running

### 6. Storage Integration

```go
// Configure storage
config.Storage = storageInstance

// Save pool state
pool.SaveState(ctx)

// Load pool state
pool.LoadState(ctx)
```

### 7. Comprehensive Metrics

Track per-agent:
- Total requests processed
- Recycle count
- Idle time
- State transitions

Track pool-wide:
- Active/idle/total agents
- Total requests
- Total recycles
- Spawned/terminated count
- Scale direction

## Concurrency Guarantees

- **Acquire/Release**: Atomic counters and channel-based synchronization
- **State Management**: Atomic operations prevent data races
- **Metrics**: Thread-safe RWMutex protection
- **Agent Map**: RWMutex-protected access
- **Queue**: Buffered channel (lock-free operations)

## Error Handling

### Context Cancellation
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

agent, err := pool.Acquire(ctx)
if err == context.DeadlineExceeded {
    // Handle timeout
}
```

### Closed Pool
```go
pool.Close()

agent, err := pool.Acquire(ctx)
// Returns: "pool is closed"
```

### Health Check Failures
```go
// Automatically:
// 1. Kill unhealthy agent
// 2. Spawn replacement
// 3. Retry acquire
```

## Performance Characteristics

| Operation | Time Complexity | Space |
|-----------|-----------------|-------|
| Acquire | O(1) average | O(1) |
| Release | O(1) average | O(1) |
| Spawn | O(n) where n = git setup | Instance size |
| Kill | O(n) where n = cleanup | Released |
| Metrics | O(n) agents | O(1) snapshot |
| Recycle | O(n) | Instance size |

## Configuration Best Practices

### Development
```go
PoolConfig{
    MinPoolSize:         1,
    MaxPoolSize:         3,
    IdleTimeout:         30 * time.Second,
    RecycleThreshold:    100,
    MaintenanceInterval: 5 * time.Second,
}
```

### Production
```go
PoolConfig{
    MinPoolSize:         4,
    MaxPoolSize:         10,
    IdleTimeout:         5 * time.Minute,
    RecycleThreshold:    1000,
    MaintenanceInterval: 30 * time.Second,
    ResourceQuota: ResourceQuota{
        MaxMemoryMB:      1024,
        MaxCPUPercent:    90.0,
        MaxInstanceAge:   2 * time.Hour,
        MaxRecyclesPerID: 100,
        RequestsPerQuota: 5000,
    },
}
```

### High-Load Scenarios
```go
PoolConfig{
    MinPoolSize:         8,
    MaxPoolSize:         10,
    IdleTimeout:         2 * time.Minute,
    RecycleThreshold:    5000,
    MaintenanceInterval: 10 * time.Second,
    ResourceQuota: ResourceQuota{
        MaxMemoryMB:      2048,
        MaxCPUPercent:    95.0,
        MaxInstanceAge:   1 * time.Hour,
        MaxRecyclesPerID: 200,
        RequestsPerQuota: 10000,
    },
}
```

## Testing

Comprehensive test suite provided in `pool_test.go`:

```bash
go test ./ollama -run TestAgentPool
```

Tests cover:
- Agent lifecycle (creation, state transitions)
- Pool initialization and constraints
- Acquire/Release operations
- Metrics tracking
- Context cancellation
- Pool closure
- Recycling logic
- Auto-scaling behavior

## Integration with session Package

The pool integrates seamlessly with:

- **session.Instance**: Wrapped in Agent struct
- **session.Storage**: Can save/load pool state
- **session.Status**: Respects instance status
- **session.GitWorktree**: Preserved across recycling

Example:
```go
instance, _ := session.NewInstance(opts)
agent := NewAgent(instance)

// Later...
retrieved := agent.GetInstance()
worktree, _ := retrieved.GetGitWorktree()
```

## Limitations and Constraints

1. **Hard Cap**: Maximum pool size is 10 agents
2. **Minimum**: At least 1 agent in pool
3. **Git Setup**: Each spawn requires git worktree creation (slow operation)
4. **Memory**: Each agent holds a full Instance with tmux session and git worktree
5. **No Persistence**: Agent sessions are not automatically persisted between pool restarts

## Future Enhancements

Potential improvements:
- Agent preloading from storage
- Adaptive maintenance intervals
- Custom scaling policies
- Agent affinity/placement
- Metrics export (Prometheus)
- Load-based request routing
- Circuit breaker for spawn failures
- Agent health metrics (memory, CPU)

## Thread Safety Summary

| Component | Method | Thread-Safe |
|-----------|--------|------------|
| Agent | GetState() | Yes (RWMutex) |
| Agent | IncrementRequests() | Yes (atomic) |
| Pool | Acquire() | Yes (channels, atomics) |
| Pool | Release() | Yes (channels, atomics) |
| Pool | GetPoolSize() | Yes (atomics) |
| Pool | GetMetrics() | Yes (RWMutex) |
| Pool | performMaintenance() | No (internal only) |
| Pool | Close() | Yes (atomic swap) |

## License

Part of claude-squad project

## Author

Claude Code Agent Framework
