# Health Monitor Quick Start Guide

## Installation

The health monitoring system is located in `/home/user/claude-squad/concurrency/health_monitor.go`

## 30-Second Quick Start

```go
package main

import (
    "claude-squad/concurrency"
    "log"
    "time"
)

func main() {
    // Create and configure monitor
    monitor := concurrency.NewHealthMonitor(concurrency.DefaultHealthMonitorConfig())

    // Add health checks
    monitor.RegisterHealthCheck(concurrency.NewTmuxHealthCheck("claudesquad_", 50))
    monitor.RegisterHealthCheck(concurrency.NewGitHealthCheck("/workspace"))

    // Add alert handler
    monitor.RegisterAlertHandler(func(alert concurrency.Alert) {
        log.Printf("[ALERT] %s: %s", alert.Component, alert.Message)
    })

    // Start monitoring
    monitor.Start()
    defer monitor.Stop()

    // Check health anytime
    time.Sleep(5 * time.Second)
    status, _ := monitor.GetHealth()
    log.Printf("System health: %s", status)
}
```

## Key API Methods

### HealthMonitor

```go
// Lifecycle
monitor.Start() error              // Start monitoring
monitor.Stop() error               // Stop monitoring gracefully

// Registration
monitor.RegisterHealthCheck(check HealthCheck)           // Add health check
monitor.RegisterRecoveryAction(component, action)        // Add recovery action
monitor.RegisterAlertHandler(handler AlertHandler)       // Add alert handler

// Queries
monitor.GetHealth() (HealthStatus, map[string]HealthCheckResult)  // Overall health
monitor.GetComponentHealth(name string) (HealthCheckResult, bool) // Component health
monitor.GetComponentHistory(name string, n int) []HealthCheckResult
monitor.GetComponentTrend(name string, n int) (improving, degrading bool)
monitor.IsHealthy() bool           // Quick health check

// Alerts
monitor.GetAlerts() []Alert        // Get all alerts
monitor.ClearAlerts()              // Clear alert list
```

### Health Checks

```go
// Built-in checks
tmuxCheck := concurrency.NewTmuxHealthCheck(prefix string, maxSessions int)
gitCheck := concurrency.NewGitHealthCheck(repoPath string)
agentCheck := concurrency.NewAgentHealthCheck(instanceChecker func() (total, healthy int, err error))

// Custom check interface
type HealthCheck interface {
    Name() string
    Check(ctx context.Context) HealthCheckResult
}
```

### Recovery Actions

```go
// Interface
type RecoveryAction interface {
    Execute(ctx context.Context) error
    Description() string
}

// Usage
monitor.RegisterRecoveryAction("tmux", &MyRecoveryAction{})
```

## Configuration Options

```go
type HealthMonitorConfig struct {
    CheckInterval   time.Duration  // How often to check (default: 30s)
    HistorySize     int            // History buffer size (default: 100)
    MaxAlerts       int            // Max alerts to keep (default: 1000)
    AlertThrottle   time.Duration  // Min time between alerts (default: 5min)
    RecoveryEnabled bool           // Enable auto-recovery (default: false)
}

// Get defaults
config := concurrency.DefaultHealthMonitorConfig()

// Or customize
config := concurrency.HealthMonitorConfig{
    CheckInterval:   15 * time.Second,
    HistorySize:     200,
    MaxAlerts:       500,
    AlertThrottle:   2 * time.Minute,
    RecoveryEnabled: true,
}
```

## Health Status Values

```go
concurrency.Healthy    // Normal operation (value: 1)
concurrency.Degraded   // Reduced functionality (value: 2)
concurrency.Unhealthy  // Critical issues (value: 3)
concurrency.Unknown    // Cannot determine (value: 0)
```

## Common Patterns

### Pattern 1: Basic Monitoring

```go
monitor := concurrency.NewHealthMonitor(concurrency.DefaultHealthMonitorConfig())
monitor.RegisterHealthCheck(concurrency.NewTmuxHealthCheck("prefix_", 50))
monitor.Start()
defer monitor.Stop()
```

### Pattern 2: With Alerts

```go
monitor.RegisterAlertHandler(func(alert concurrency.Alert) {
    if alert.Status == concurrency.Unhealthy {
        // Send to pager
        sendCriticalAlert(alert)
    }
})
```

### Pattern 3: With Recovery

```go
config := concurrency.DefaultHealthMonitorConfig()
config.RecoveryEnabled = true
monitor := concurrency.NewHealthMonitor(config)

monitor.RegisterRecoveryAction("tmux", &RestartTmuxAction{})
```

### Pattern 4: Health Dashboard

```go
ticker := time.NewTicker(5 * time.Second)
for range ticker.C {
    status, results := monitor.GetHealth()
    fmt.Printf("Overall: %s\n", status)
    for name, result := range results {
        fmt.Printf("  %s: %s\n", name, result.Status)
    }
}
```

### Pattern 5: HTTP Health Endpoint

```go
http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    status, results := monitor.GetHealth()

    if status == concurrency.Unhealthy {
        w.WriteHeader(http.StatusServiceUnavailable)
    } else {
        w.WriteHeader(http.StatusOK)
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": status.String(),
        "components": results,
    })
})
```

### Pattern 6: Trend Analysis

```go
// Check if component health is improving or degrading
improving, degrading := monitor.GetComponentTrend("agents", 20)

if degrading {
    // Take proactive action
    log.Warn("Agent health is degrading - increasing resources")
    scaleUpResources()
}
```

## Custom Health Check Example

```go
type DatabaseHealthCheck struct {
    db *sql.DB
}

func (d *DatabaseHealthCheck) Name() string {
    return "database"
}

func (d *DatabaseHealthCheck) Check(ctx context.Context) concurrency.HealthCheckResult {
    result := concurrency.HealthCheckResult{
        Timestamp: time.Now(),
        Metadata:  make(map[string]interface{}),
    }

    // Ping database with timeout
    ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()

    if err := d.db.PingContext(ctx); err != nil {
        result.Status = concurrency.Unhealthy
        result.Message = fmt.Sprintf("database ping failed: %v", err)
        return result
    }

    // Check connection count
    stats := d.db.Stats()
    result.Metadata["open_connections"] = stats.OpenConnections
    result.Metadata["idle_connections"] = stats.Idle

    if stats.OpenConnections > 50 {
        result.Status = concurrency.Degraded
        result.Message = "high connection count"
    } else {
        result.Status = concurrency.Healthy
        result.Message = "database is healthy"
    }

    return result
}

// Register it
monitor.RegisterHealthCheck(&DatabaseHealthCheck{db: myDB})
```

## Custom Recovery Action Example

```go
type RestartServiceRecovery struct {
    serviceName string
}

func (r *RestartServiceRecovery) Execute(ctx context.Context) error {
    log.Printf("Restarting service: %s", r.serviceName)

    // Respect context timeout
    cmd := exec.CommandContext(ctx, "systemctl", "restart", r.serviceName)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("failed to restart %s: %w", r.serviceName, err)
    }

    // Wait for service to be healthy
    time.Sleep(5 * time.Second)
    return nil
}

func (r *RestartServiceRecovery) Description() string {
    return fmt.Sprintf("restart %s service", r.serviceName)
}

// Register it
monitor.RegisterRecoveryAction("myservice", &RestartServiceRecovery{"myservice"})
```

## Testing

```go
// Create mock check for testing
type MockHealthCheck struct {
    status concurrency.HealthStatus
}

func (m *MockHealthCheck) Name() string { return "mock" }

func (m *MockHealthCheck) Check(ctx context.Context) concurrency.HealthCheckResult {
    return concurrency.HealthCheckResult{
        Status:    m.status,
        Message:   "mock check",
        Timestamp: time.Now(),
        Metadata:  make(map[string]interface{}),
    }
}

// Use in tests
mock := &MockHealthCheck{status: concurrency.Healthy}
monitor.RegisterHealthCheck(mock)

// Change status to test alerts
mock.status = concurrency.Unhealthy
```

## Files

- **health_monitor.go** (740 lines) - Main implementation
- **health_monitor_test.go** (581 lines) - Comprehensive test suite
- **health_monitor_example.go** (373 lines) - Advanced usage examples
- **HEALTH_MONITOR_README.md** (475 lines) - Full documentation

## Next Steps

1. Review `health_monitor.go` for implementation details
2. Check `health_monitor_test.go` for test examples
3. Explore `health_monitor_example.go` for advanced patterns
4. Read `HEALTH_MONITOR_README.md` for comprehensive documentation

## Support

For integration questions or issues, refer to the main README or examine the test files for usage patterns.
