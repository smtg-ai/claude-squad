# Ollama Metrics Package

Comprehensive agent performance monitoring for the Ollama framework. This package provides thread-safe metrics collection, real-time event streams, and JSON export capabilities.

## Overview

The `metrics` package enables detailed tracking of:
- Per-model performance metrics (latency, throughput, error rates)
- Task completion statistics
- System resource utilization
- Latency distribution histograms
- Token processing metrics

## Features

### 1. MetricsCollector - Main Component

The `MetricsCollector` struct is the central hub for all metric collection:

```go
mc := ollama.NewMetricsCollector()
defer mc.Close()
```

**Thread-Safe**: Uses `sync.RWMutex` for concurrent access from multiple goroutines.

### 2. Per-Model Performance Metrics

Track individual performance metrics for each model:

```go
type PerformanceMetrics struct {
    Model           string
    TotalRequests   int64
    SuccessfulReqs  int64
    FailedReqs      int64
    TotalLatency    time.Duration
    MinLatency      time.Duration
    MaxLatency      time.Duration
    AvgLatency      time.Duration
    Throughput      float64         // requests per second
    ErrorRate       float64         // percentage
    LastUpdated     time.Time
    LatencyHist     *LatencyHistogram
    TokensProcessed int64
    AvgTokensPerReq float64
}
```

### 3. Latency Histogram

Automatic latency bucketing with predefined intervals:
- 1ms, 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 5s, 10s+

```go
type LatencyHistogram struct {
    Buckets map[string]int64  // bucket name -> count
    Min     time.Duration
    Max     time.Duration
    Total   time.Duration
    Count   int64
}
```

### 4. Task Statistics

Track task-level metrics:

```go
type TaskStatistics struct {
    TotalTasks      int64
    CompletedTasks  int64
    FailedTasks     int64
    AvgCompletionMs float64
    MinCompletionMs float64
    MaxCompletionMs float64
    SuccessRate     float64
    LastUpdated     time.Time
}
```

### 5. Resource Metrics

Monitor system resource usage:

```go
type ResourceMetrics struct {
    MemoryUsageMB   int64
    CPUUsagePercent float64
    GoroutineCount  int
    Timestamp       time.Time
}
```

## Usage Examples

### Basic Usage

```go
package main

import (
    "time"
    "claude-squad/ollama"
)

func main() {
    mc := ollama.NewMetricsCollector()
    defer mc.Close()

    // Record a successful request
    mc.RecordLatency("llama2", 150*time.Millisecond)
    mc.RecordTokens("llama2", 500)

    // Record a failed request
    mc.RecordError("llama2", ollama.ErrorTimeout)

    // Record task completion
    mc.RecordTaskCompletion(true, 150.0) // duration in ms

    // Update resource metrics
    mc.UpdateResourceMetrics(512, 45.5, 10)

    // Get metrics for a specific model
    metrics, exists := mc.GetModelMetrics("llama2")
    if exists {
        println("Error Rate:", metrics.ErrorRate)
    }
}
```

### Real-Time Metrics Monitoring

```go
mc := ollama.NewMetricsCollector()
defer mc.Close()

// Listen to metrics events
go func() {
    for event := range mc.MetricsChannel {
        switch event.Type {
        case "latency":
            fmt.Printf("Latency: %v\n", event.Value)
        case "error":
            fmt.Printf("Error: %v\n", event.Value)
        case "task_complete":
            fmt.Printf("Task Success: %v\n", event.Value)
        case "resource":
            res := event.Value.(*ollama.ResourceMetrics)
            fmt.Printf("Memory: %dMB\n", res.MemoryUsageMB)
        }
    }
}()

// Perform operations...
```

### JSON Export

```go
// Export to memory
jsonData, err := mc.ExportJSON()
if err != nil {
    log.Fatal(err)
}

// Export to file
err = mc.ExportJSONToFile("/path/to/metrics.json")
if err != nil {
    log.Fatal(err)
}
```

### Summary Reports

```go
// Overall metrics summary
fmt.Println(mc.GetSummary())

// Model-specific summary
summary, err := mc.GetModelSummary("llama2")
fmt.Println(summary)

// Latency histogram summary
histSummary, err := mc.GetHistogramSummary("llama2")
fmt.Println(histSummary)
```

## API Reference

### MetricsCollector Methods

#### Recording Metrics

- `RecordLatency(model string, latency time.Duration)` - Record response latency
- `RecordError(model string, err error)` - Record failed request
- `RecordTokens(model string, tokenCount int64)` - Record token count
- `RecordTaskCompletion(success bool, durationMs float64)` - Record task completion
- `UpdateResourceMetrics(memoryMB int64, cpuPercent float64, goroutineCount int)` - Update system resources

#### Retrieving Metrics

- `GetModelMetrics(model string) (*PerformanceMetrics, bool)` - Get metrics for specific model
- `GetAllModelMetrics() map[string]*PerformanceMetrics` - Get all model metrics
- `GetTaskStatistics() *TaskStatistics` - Get task statistics
- `GetResourceMetrics() *ResourceMetrics` - Get current resource metrics

#### Exporting Metrics

- `ExportJSON() ([]byte, error)` - Export as JSON
- `ExportJSONToFile(filepath string) error` - Export to JSON file

#### Utilities

- `GetSummary() string` - Get overall summary
- `GetModelSummary(model string) (string, error)` - Get model summary
- `GetHistogramSummary(model string) (string, error)` - Get latency histogram summary
- `GetMetricsChannel() <-chan *MetricsEvent` - Get metrics event channel
- `ResetMetrics()` - Reset all metrics
- `Close()` - Close metrics channel

## JSON Export Format

```json
{
  "start_time": "2025-12-25T10:30:00Z",
  "collection_time": "2025-12-25T10:35:00Z",
  "uptime_seconds": 300,
  "model_metrics": {
    "llama2": {
      "model": "llama2",
      "total_requests": 100,
      "successful_reqs": 95,
      "failed_reqs": 5,
      "avg_latency": "150ms",
      "throughput": 20.5,
      "error_rate": 5.0,
      "tokens_processed": 50000,
      "avg_tokens_per_req": 526.3
    }
  },
  "task_statistics": {
    "total_tasks": 100,
    "completed_tasks": 95,
    "failed_tasks": 5,
    "success_rate": 95.0
  },
  "resource_metrics": {
    "memory_usage_mb": 512,
    "cpu_usage_percent": 45.5,
    "goroutine_count": 10
  },
  "total_requests": 100,
  "total_errors": 5,
  "overall_error_rate": 5.0
}
```

## Thread Safety

All methods are thread-safe using `sync.RWMutex`:
- Multiple goroutines can safely record metrics concurrently
- Reading metrics doesn't block writing and vice versa
- Mutations to returned metrics don't affect internal state (defensive copies)

## Performance Considerations

- **Channel Buffer**: MetricsChannel has buffer size of 100 to prevent blocking
- **Lock Granularity**: Read locks are used for metric queries
- **Defensive Copies**: All getters return copies to prevent external mutations
- **Histogram Buckets**: 10 predefined buckets for efficient latency tracking

## Error Handling

Common errors are predefined in `errors.go`:

```go
- ErrorTimeout
- ErrorConnectionFailed
- ErrorInvalidModel
- ErrorResourceExhausted
- ErrorModelNotLoaded
- ErrorInvalidRequest
```

Create custom errors:

```go
err := ollama.NewOllamaError("CUSTOM", "Custom error message", underlyingErr)
```

## Testing

Run all metrics tests:

```bash
go test -v ./ollama/metrics_test.go ./ollama/metrics.go ./ollama/errors.go
```

Tests include:
- Creation and initialization
- Latency recording and histogram tracking
- Error recording
- Token counting
- Task completion statistics
- Resource metric updates
- JSON export
- Thread safety
- Real-time channel events
- Summary generation
- Metrics reset

## Example Applications

See `example_metrics.go` for complete examples:

- `ExampleMetricsUsage()` - Basic usage with multiple models
- `ExampleMetricsMonitoring()` - Real-time event monitoring
- `ExampleMetricsWithPeriodicExport()` - Periodic metrics export
- `ExampleMultiModelMetrics()` - Multi-model tracking
- `ExampleMetricsReset()` - Resetting metrics
- `ExampleThreadSafeMetrics()` - Concurrent operations

## Best Practices

1. **Create Single Collector**: Use one `MetricsCollector` per application
2. **Defer Close()**: Always defer `mc.Close()` to clean up channels
3. **Batch Exports**: Export metrics periodically, not on every operation
4. **Monitor Channels**: Handle metrics events in separate goroutines
5. **Resource Updates**: Update resource metrics every 5-10 seconds
6. **Error Recording**: Record errors immediately for accurate error rates
7. **Regular Summaries**: Generate summaries for operational insights

## Limitations and Future Enhancements

- Histogram buckets are fixed and cannot be customized
- No automatic persistence to databases
- No built-in alerting thresholds
- Metrics are in-memory only (reset on app restart)

## Contributing

When extending the metrics package:
- Maintain thread safety with `sync.RWMutex`
- Follow existing naming conventions
- Add corresponding tests
- Update this documentation
- Ensure defensive copying in getters
