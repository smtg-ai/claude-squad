# Health Monitoring System

A production-grade health monitoring system for the claude-squad application that provides comprehensive health checks, alerting, auto-recovery, and trend analysis.

## Overview

The health monitoring system continuously monitors critical components of the claude-squad application:
- **Tmux sessions**: Monitors session health and prevents resource exhaustion
- **Git operations**: Tracks repository and worktree health
- **Agent instances**: Monitors running agent health and availability

## Features

### Core Components

1. **HealthMonitor** - Main orchestrator that coordinates all health monitoring activities
2. **HealthCheck Interface** - Extensible interface for implementing custom health checks
3. **HealthAggregator** - Aggregates component health into overall system health
4. **AlertManager** - Manages health alerts with throttling and notification
5. **RecoveryAction Interface** - Enables automatic remediation of health issues
6. **HealthHistory** - Maintains circular buffer of historical health data for trend analysis

### Health Status Levels

- **Healthy**: Component operating normally
- **Degraded**: Component experiencing reduced functionality but still operational
- **Unhealthy**: Component experiencing critical issues requiring immediate attention
- **Unknown**: Health status cannot be determined

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     HealthMonitor                           │
│  ┌───────────────────────────────────────────────────────┐ │
│  │  HealthChecks (async goroutines)                      │ │
│  │  ├─ TmuxHealthCheck                                   │ │
│  │  ├─ GitHealthCheck                                    │ │
│  │  └─ AgentHealthCheck                                  │ │
│  └────────────────┬────────────────────────────────────────┘ │
│                   │                                          │
│  ┌────────────────▼──────────────────┐                      │
│  │   HealthAggregator                │                      │
│  │   (Overall System Health)         │                      │
│  └────────────────┬──────────────────┘                      │
│                   │                                          │
│  ┌────────────────▼──────────────────┐                      │
│  │   AlertManager                    │                      │
│  │   ├─ Console Logger               │                      │
│  │   ├─ Metrics Collector            │                      │
│  │   └─ Critical Alerts (Pager)      │                      │
│  └───────────────────────────────────┘                      │
│                                                              │
│  ┌──────────────────────────────────────┐                   │
│  │   HealthHistory (per component)      │                   │
│  │   ├─ Circular Buffer                 │                   │
│  │   ├─ Trend Analysis                  │                   │
│  │   └─ Historical Queries              │                   │
│  └──────────────────────────────────────┘                   │
│                                                              │
│  ┌──────────────────────────────────────┐                   │
│  │   RecoveryActions (auto-remediation) │                   │
│  │   ├─ Restart Tmux Sessions           │                   │
│  │   ├─ Prune Git Worktrees             │                   │
│  │   └─ Restart Unhealthy Agents        │                   │
│  └──────────────────────────────────────┘                   │
└─────────────────────────────────────────────────────────────┘
```

## Usage

### Basic Setup

```go
import "claude-squad/concurrency"

// Create configuration
config := concurrency.DefaultHealthMonitorConfig()
config.CheckInterval = 30 * time.Second
config.RecoveryEnabled = true

// Create monitor
monitor := concurrency.NewHealthMonitor(config)

// Register health checks
monitor.RegisterHealthCheck(concurrency.NewTmuxHealthCheck("claudesquad_", 50))
monitor.RegisterHealthCheck(concurrency.NewGitHealthCheck("/path/to/repo"))
monitor.RegisterHealthCheck(concurrency.NewAgentHealthCheck(func() (int, int, error) {
    // Return total instances, healthy instances, error
    return 10, 9, nil
}))

// Start monitoring
if err := monitor.Start(); err != nil {
    log.Fatal(err)
}

// Later: stop monitoring
defer monitor.Stop()
```

### Advanced Configuration

```go
config := concurrency.HealthMonitorConfig{
    CheckInterval:   15 * time.Second,  // Check every 15 seconds
    HistorySize:     200,                // Keep 200 historical records
    MaxAlerts:       500,                // Maximum alerts to retain
    AlertThrottle:   2 * time.Minute,   // Minimum time between same alerts
    RecoveryEnabled: true,               // Enable auto-recovery
}

monitor := concurrency.NewHealthMonitor(config)
```

### Registering Alert Handlers

```go
// Console logger
monitor.RegisterAlertHandler(func(alert concurrency.Alert) {
    log.Printf("[ALERT] %s | %s | %s",
        alert.Component,
        alert.Status,
        alert.Message,
    )
})

// Critical alerts (for PagerDuty, Slack, etc.)
monitor.RegisterAlertHandler(func(alert concurrency.Alert) {
    if alert.Status == concurrency.Unhealthy {
        sendToPager(alert)
    }
})

// Metrics collection (for Prometheus)
monitor.RegisterAlertHandler(func(alert concurrency.Alert) {
    healthAlertMetric.WithLabelValues(
        alert.Component,
        alert.Status.String(),
    ).Inc()
})
```

### Implementing Custom Health Checks

```go
type CustomHealthCheck struct {
    name string
}

func (c *CustomHealthCheck) Name() string {
    return c.name
}

func (c *CustomHealthCheck) Check(ctx context.Context) concurrency.HealthCheckResult {
    result := concurrency.HealthCheckResult{
        Timestamp: time.Now(),
        Metadata:  make(map[string]interface{}),
    }

    // Perform your health check logic
    // Respect context cancellation for timeouts
    select {
    case <-ctx.Done():
        result.Status = concurrency.Degraded
        result.Message = "health check timed out"
        return result
    default:
        // Your check logic here
    }

    result.Status = concurrency.Healthy
    result.Message = "component is healthy"
    return result
}

// Register the custom check
monitor.RegisterHealthCheck(&CustomHealthCheck{name: "custom"})
```

### Implementing Recovery Actions

```go
type CustomRecovery struct{}

func (c *CustomRecovery) Execute(ctx context.Context) error {
    // Perform recovery action
    // Respect context for timeout
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
        // Recovery logic here
    }
    return nil
}

func (c *CustomRecovery) Description() string {
    return "custom recovery action"
}

// Register recovery action
monitor.RegisterRecoveryAction("component-name", &CustomRecovery{})
```

### Querying Health Status

```go
// Get overall health
status, results := monitor.GetHealth()
fmt.Printf("Overall: %s\n", status)

// Get specific component health
result, exists := monitor.GetComponentHealth("tmux")
if exists {
    fmt.Printf("Tmux: %s - %s\n", result.Status, result.Message)
}

// Check if system is healthy
if monitor.IsHealthy() {
    fmt.Println("System is healthy!")
}

// Get historical data
history := monitor.GetComponentHistory("git", 10)
for _, h := range history {
    fmt.Printf("%s: %s\n", h.Timestamp, h.Status)
}

// Analyze trends
improving, degrading := monitor.GetComponentTrend("agents", 20)
if degrading {
    fmt.Println("Warning: agents health is degrading!")
}
```

### Working with Alerts

```go
// Get all alerts
alerts := monitor.GetAlerts()
for _, alert := range alerts {
    fmt.Printf("[%s] %s: %s\n",
        alert.Timestamp.Format(time.RFC3339),
        alert.Component,
        alert.Message,
    )
}

// Clear alerts
monitor.ClearAlerts()
```

## Built-in Health Checks

### TmuxHealthCheck

Monitors tmux session health:
- Verifies tmux is installed and accessible
- Counts active sessions with specified prefix
- Detects when session count exceeds configured maximum
- Returns degraded status if too many sessions exist

```go
check := concurrency.NewTmuxHealthCheck("claudesquad_", 50)
// Will alert if more than 50 sessions with prefix "claudesquad_"
```

### GitHealthCheck

Monitors git repository health:
- Verifies git is installed and accessible
- Checks repository accessibility
- Tracks uncommitted changes
- Monitors worktree count
- Detects repository corruption

```go
check := concurrency.NewGitHealthCheck("/path/to/repo")
```

### AgentHealthCheck

Monitors agent instance health:
- Queries total and healthy instance counts
- Calculates health ratio
- Returns:
  - Healthy: ≥90% instances healthy
  - Degraded: 50-89% instances healthy
  - Unhealthy: <50% instances healthy

```go
check := concurrency.NewAgentHealthCheck(func() (int, int, error) {
    total := getInstanceCount()
    healthy := getHealthyInstanceCount()
    return total, healthy, nil
})
```

## Integration Patterns

### With HTTP Health Endpoint

```go
func healthHandler(w http.ResponseWriter, r *http.Request) {
    status, results := monitor.GetHealth()

    response := map[string]interface{}{
        "status": status.String(),
        "components": results,
        "timestamp": time.Now(),
    }

    httpStatus := http.StatusOK
    if status == concurrency.Unhealthy {
        httpStatus = http.StatusServiceUnavailable
    } else if status == concurrency.Degraded {
        httpStatus = http.StatusOK // Still accepting traffic
    }

    w.WriteHeader(httpStatus)
    json.NewEncoder(w).Encode(response)
}
```

### With Kubernetes Liveness Probe

```go
func livenessHandler(w http.ResponseWriter, r *http.Request) {
    if monitor.IsHealthy() {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
        w.Write([]byte("UNHEALTHY"))
    }
}
```

### With Prometheus Metrics

```go
var (
    healthGauge = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "claudesquad_component_health",
            Help: "Health status of components (0=unknown, 1=healthy, 2=degraded, 3=unhealthy)",
        },
        []string{"component"},
    )
)

monitor.RegisterAlertHandler(func(alert concurrency.Alert) {
    healthGauge.WithLabelValues(alert.Component).Set(float64(alert.Status))
})
```

### With Logging System

```go
import "github.com/sirupsen/logrus"

monitor.RegisterAlertHandler(func(alert concurrency.Alert) {
    fields := logrus.Fields{
        "component": alert.Component,
        "status":    alert.Status.String(),
        "timestamp": alert.Timestamp,
    }

    switch alert.Status {
    case concurrency.Unhealthy:
        logrus.WithFields(fields).Error(alert.Message)
    case concurrency.Degraded:
        logrus.WithFields(fields).Warn(alert.Message)
    default:
        logrus.WithFields(fields).Info(alert.Message)
    }
})
```

## Configuration Best Practices

1. **Check Interval**: Balance between responsiveness and overhead
   - High-traffic systems: 30-60 seconds
   - Low-traffic systems: 15-30 seconds
   - Critical systems: 10-15 seconds

2. **History Size**: Depends on check interval and analysis needs
   - For trend analysis: Keep at least 1 hour of data
   - Formula: `historySize = (3600 / checkIntervalSeconds) * hoursToKeep`
   - Example: For 30s intervals, 1 hour = 120 records

3. **Alert Throttle**: Prevent alert fatigue
   - Start with 5 minutes
   - Adjust based on recovery time
   - Critical components: 2-3 minutes
   - Non-critical: 10-15 minutes

4. **Recovery Actions**: Use with caution
   - Test thoroughly before enabling in production
   - Implement proper logging
   - Add circuit breakers to prevent recovery loops
   - Monitor recovery success/failure rates

## Thread Safety

All components are thread-safe and can be safely accessed from multiple goroutines:
- Uses `sync.RWMutex` for read-heavy operations
- Proper synchronization in circular buffers
- Context-based cancellation for graceful shutdown
- No data races (verified with `go test -race`)

## Performance Characteristics

- **Memory**: O(historySize * numComponents) for history storage
- **CPU**: Minimal overhead, health checks run in separate goroutines
- **Latency**: Health checks timeout after 10 seconds
- **Scalability**: Handles 100+ components without performance degradation

## Testing

Run the test suite:

```bash
go test claude-squad/concurrency -v -run "TestHealth.*"
```

Run with race detector:

```bash
go test claude-squad/concurrency -race
```

## Examples

See the following files for complete examples:
- `health_monitor_example.go` - Advanced usage patterns
- `health_monitor_test.go` - Test cases and examples

## Troubleshooting

### Health checks timing out
- Reduce check interval
- Increase timeout in `performCheck` (default 10s)
- Check if external dependencies are slow

### Too many alerts
- Increase `AlertThrottle` duration
- Review alert severity thresholds
- Implement alert deduplication

### Memory usage growing
- Reduce `HistorySize`
- Reduce `MaxAlerts`
- Ensure `ClearAlerts()` is called periodically

### Recovery actions not executing
- Verify `RecoveryEnabled: true` in config
- Check component status is `Unhealthy` (not just `Degraded`)
- Review recovery action logs for errors

## Future Enhancements

Potential improvements:
- [ ] Persistent storage for health history
- [ ] Web dashboard for visualization
- [ ] Alert routing rules
- [ ] Health check dependencies
- [ ] Automatic threshold adjustment
- [ ] Integration with external monitoring systems
- [ ] Health check circuit breakers

## License

Part of the claude-squad project. See LICENSE.md for details.
