package ollama

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"
)

// LatencyHistogram tracks latency distribution across buckets
type LatencyHistogram struct {
	Buckets map[string]int64 // bucket name -> count
	Min     time.Duration
	Max     time.Duration
	Total   time.Duration
	Count   int64
}

// PerformanceMetrics tracks per-model performance metrics
type PerformanceMetrics struct {
	Model           string
	TotalRequests   int64
	SuccessfulReqs  int64
	FailedReqs      int64
	TotalLatency    time.Duration
	MinLatency      time.Duration
	MaxLatency      time.Duration
	AvgLatency      time.Duration
	Throughput      float64 // requests per second
	ErrorRate       float64 // percentage of failed requests
	LastUpdated     time.Time
	LatencyHist     *LatencyHistogram
	TokensProcessed int64
	AvgTokensPerReq float64
}

// TaskStatistics tracks task completion metrics
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

// ResourceMetrics tracks system resource utilization
type ResourceMetrics struct {
	MemoryUsageMB   int64
	CPUUsagePercent float64
	GoroutineCount  int
	Timestamp       time.Time
}

// MetricsCollector is the main collector for all agent performance metrics
type MetricsCollector struct {
	mu                 sync.RWMutex
	PerformanceMetrics map[string]*PerformanceMetrics
	TaskStats          *TaskStatistics
	ResourceMetrics    *ResourceMetrics
	StartTime          time.Time
	MetricsChannel     chan *MetricsEvent
	resourceChannel    chan *ResourceMetrics
	lastResourceUpdate time.Time
	latencyBuckets     []int64 // bucket boundaries in milliseconds
	maxModels          int     // maximum number of models to track (0 = unlimited)
}

// MetricsEvent represents a metrics update event
type MetricsEvent struct {
	Type      string      // "latency", "task_complete", "error", "resource"
	Model     string      // relevant model name
	Value     interface{} // the metric value
	Timestamp time.Time
}

// JSONExport represents the exportable metrics structure
type JSONExport struct {
	StartTime        time.Time                      `json:"start_time"`
	CollectionTime   time.Time                      `json:"collection_time"`
	UptimeSeconds    float64                        `json:"uptime_seconds"`
	ModelMetrics     map[string]*PerformanceMetrics `json:"model_metrics"`
	TaskStatistics   *TaskStatistics                `json:"task_statistics"`
	ResourceMetrics  *ResourceMetrics               `json:"resource_metrics"`
	TotalRequests    int64                          `json:"total_requests"`
	TotalErrors      int64                          `json:"total_errors"`
	OverallErrorRate float64                        `json:"overall_error_rate"`
}

// NewMetricsCollector creates a new metrics collector instance
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		PerformanceMetrics: make(map[string]*PerformanceMetrics),
		TaskStats:          &TaskStatistics{},
		ResourceMetrics:    &ResourceMetrics{Timestamp: time.Now()},
		StartTime:          time.Now(),
		MetricsChannel:     make(chan *MetricsEvent, 100),
		resourceChannel:    make(chan *ResourceMetrics, 10),
		lastResourceUpdate: time.Now(),
		// Latency buckets: 1ms, 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 5s
		latencyBuckets: []int64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000},
		maxModels:      1000, // Limit to 1000 models to prevent unbounded growth
	}
}

// RecordLatency records a latency measurement for a specific model
func (mc *MetricsCollector) RecordLatency(model string, latency time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.PerformanceMetrics[model] == nil {
		// Check if we've hit the model limit
		if mc.maxModels > 0 && len(mc.PerformanceMetrics) >= mc.maxModels {
			// Remove least recently used model to make room
			mc.evictLeastRecentlyUsedModel()
		}
		mc.PerformanceMetrics[model] = &PerformanceMetrics{
			Model:       model,
			MinLatency:  latency,
			MaxLatency:  latency,
			LatencyHist: mc.newLatencyHistogram(),
		}
	}

	metrics := mc.PerformanceMetrics[model]
	metrics.TotalRequests++
	metrics.SuccessfulReqs++
	metrics.TotalLatency += latency
	metrics.LastUpdated = time.Now()

	// Update min/max
	if latency < metrics.MinLatency {
		metrics.MinLatency = latency
	}
	if latency > metrics.MaxLatency {
		metrics.MaxLatency = latency
	}

	// Update histogram
	mc.updateHistogram(metrics.LatencyHist, latency)

	// Calculate average latency
	if metrics.TotalRequests > 0 {
		metrics.AvgLatency = metrics.TotalLatency / time.Duration(metrics.TotalRequests)
	}

	// Calculate throughput (requests per second over uptime)
	uptime := time.Since(mc.StartTime).Seconds()
	if uptime > 0 {
		metrics.Throughput = float64(metrics.TotalRequests) / uptime
	}

	// Send event
	select {
	case mc.MetricsChannel <- &MetricsEvent{
		Type:      "latency",
		Model:     model,
		Value:     latency,
		Timestamp: time.Now(),
	}:
	default:
		// Channel full, skip this event
	}
}

// RecordError records a failed request for a specific model
func (mc *MetricsCollector) RecordError(model string, err error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.PerformanceMetrics[model] == nil {
		// Check if we've hit the model limit
		if mc.maxModels > 0 && len(mc.PerformanceMetrics) >= mc.maxModels {
			// Remove least recently used model to make room
			mc.evictLeastRecentlyUsedModel()
		}
		mc.PerformanceMetrics[model] = &PerformanceMetrics{
			Model:       model,
			LatencyHist: mc.newLatencyHistogram(),
		}
	}

	metrics := mc.PerformanceMetrics[model]
	metrics.TotalRequests++
	metrics.FailedReqs++
	metrics.LastUpdated = time.Now()

	// Calculate error rate
	if metrics.TotalRequests > 0 {
		metrics.ErrorRate = float64(metrics.FailedReqs) / float64(metrics.TotalRequests) * 100
	}

	// Send event
	select {
	case mc.MetricsChannel <- &MetricsEvent{
		Type:      "error",
		Model:     model,
		Value:     err.Error(),
		Timestamp: time.Now(),
	}:
	default:
	}
}

// RecordTokens records tokens processed for a specific model
func (mc *MetricsCollector) RecordTokens(model string, tokenCount int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.PerformanceMetrics[model] == nil {
		// Check if we've hit the model limit
		if mc.maxModels > 0 && len(mc.PerformanceMetrics) >= mc.maxModels {
			// Remove least recently used model to make room
			mc.evictLeastRecentlyUsedModel()
		}
		mc.PerformanceMetrics[model] = &PerformanceMetrics{
			Model:       model,
			LatencyHist: mc.newLatencyHistogram(),
		}
	}

	metrics := mc.PerformanceMetrics[model]
	metrics.TokensProcessed += tokenCount
	if metrics.SuccessfulReqs > 0 {
		metrics.AvgTokensPerReq = float64(metrics.TokensProcessed) / float64(metrics.SuccessfulReqs)
	}
}

// RecordTaskCompletion records a task completion event
func (mc *MetricsCollector) RecordTaskCompletion(success bool, durationMs float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.TaskStats.TotalTasks++
	if success {
		mc.TaskStats.CompletedTasks++
	} else {
		mc.TaskStats.FailedTasks++
	}

	// Update completion time stats
	if mc.TaskStats.TotalTasks == 1 {
		mc.TaskStats.MinCompletionMs = durationMs
		mc.TaskStats.MaxCompletionMs = durationMs
		mc.TaskStats.AvgCompletionMs = durationMs
	} else {
		// Calculate rolling average
		totalMs := mc.TaskStats.AvgCompletionMs * float64(mc.TaskStats.TotalTasks-1)
		mc.TaskStats.AvgCompletionMs = (totalMs + durationMs) / float64(mc.TaskStats.TotalTasks)

		if durationMs < mc.TaskStats.MinCompletionMs {
			mc.TaskStats.MinCompletionMs = durationMs
		}
		if durationMs > mc.TaskStats.MaxCompletionMs {
			mc.TaskStats.MaxCompletionMs = durationMs
		}
	}

	// Calculate success rate
	if mc.TaskStats.TotalTasks > 0 {
		mc.TaskStats.SuccessRate = float64(mc.TaskStats.CompletedTasks) / float64(mc.TaskStats.TotalTasks) * 100
	}

	mc.TaskStats.LastUpdated = time.Now()

	// Send event
	select {
	case mc.MetricsChannel <- &MetricsEvent{
		Type:      "task_complete",
		Value:     success,
		Timestamp: time.Now(),
	}:
	default:
	}
}

// UpdateResourceMetrics updates system resource utilization metrics
func (mc *MetricsCollector) UpdateResourceMetrics(memoryMB int64, cpuPercent float64, goroutineCount int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.ResourceMetrics = &ResourceMetrics{
		MemoryUsageMB:   memoryMB,
		CPUUsagePercent: cpuPercent,
		GoroutineCount:  goroutineCount,
		Timestamp:       time.Now(),
	}

	mc.lastResourceUpdate = time.Now()

	// Send event
	select {
	case mc.MetricsChannel <- &MetricsEvent{
		Type:      "resource",
		Value:     mc.ResourceMetrics,
		Timestamp: time.Now(),
	}:
	default:
	}
}

// GetModelMetrics returns a copy of metrics for a specific model
func (mc *MetricsCollector) GetModelMetrics(model string) (*PerformanceMetrics, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	metrics, exists := mc.PerformanceMetrics[model]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modifications
	metricsCopy := *metrics
	if metrics.LatencyHist != nil {
		histCopy := *metrics.LatencyHist
		bucketsCopy := make(map[string]int64)
		for k, v := range metrics.LatencyHist.Buckets {
			bucketsCopy[k] = v
		}
		histCopy.Buckets = bucketsCopy
		metricsCopy.LatencyHist = &histCopy
	}
	return &metricsCopy, true
}

// GetAllModelMetrics returns a copy of all model metrics
func (mc *MetricsCollector) GetAllModelMetrics() map[string]*PerformanceMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]*PerformanceMetrics)
	for model, metrics := range mc.PerformanceMetrics {
		metricsCopy := *metrics
		if metrics.LatencyHist != nil {
			histCopy := *metrics.LatencyHist
			bucketsCopy := make(map[string]int64)
			for k, v := range metrics.LatencyHist.Buckets {
				bucketsCopy[k] = v
			}
			histCopy.Buckets = bucketsCopy
			metricsCopy.LatencyHist = &histCopy
		}
		result[model] = &metricsCopy
	}
	return result
}

// GetTaskStatistics returns a copy of task statistics
func (mc *MetricsCollector) GetTaskStatistics() *TaskStatistics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	taskStatsCopy := *mc.TaskStats
	return &taskStatsCopy
}

// GetResourceMetrics returns a copy of resource metrics
func (mc *MetricsCollector) GetResourceMetrics() *ResourceMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if mc.ResourceMetrics == nil {
		return nil
	}

	resourceCopy := *mc.ResourceMetrics
	return &resourceCopy
}

// GetMetricsChannel returns the channel for real-time metrics events
func (mc *MetricsCollector) GetMetricsChannel() <-chan *MetricsEvent {
	return mc.MetricsChannel
}

// ExportJSON exports all metrics as JSON
func (mc *MetricsCollector) ExportJSON() ([]byte, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	totalRequests := int64(0)
	totalErrors := int64(0)

	// Create model metrics copy
	modelMetricsCopy := make(map[string]*PerformanceMetrics)
	for model, metrics := range mc.PerformanceMetrics {
		metricsCopy := *metrics
		if metrics.LatencyHist != nil {
			histCopy := *metrics.LatencyHist
			bucketsCopy := make(map[string]int64)
			for k, v := range metrics.LatencyHist.Buckets {
				bucketsCopy[k] = v
			}
			histCopy.Buckets = bucketsCopy
			metricsCopy.LatencyHist = &histCopy
		}
		modelMetricsCopy[model] = &metricsCopy
		totalRequests += metrics.TotalRequests
		totalErrors += metrics.FailedReqs
	}

	overallErrorRate := 0.0
	if totalRequests > 0 {
		overallErrorRate = float64(totalErrors) / float64(totalRequests) * 100
	}

	taskStatsCopy := *mc.TaskStats
	resourceCopy := *mc.ResourceMetrics

	export := &JSONExport{
		StartTime:        mc.StartTime,
		CollectionTime:   time.Now(),
		UptimeSeconds:    time.Since(mc.StartTime).Seconds(),
		ModelMetrics:     modelMetricsCopy,
		TaskStatistics:   &taskStatsCopy,
		ResourceMetrics:  &resourceCopy,
		TotalRequests:    totalRequests,
		TotalErrors:      totalErrors,
		OverallErrorRate: overallErrorRate,
	}

	return json.MarshalIndent(export, "", "  ")
}

// ExportJSONToFile exports metrics to a JSON file
func (mc *MetricsCollector) ExportJSONToFile(filepath string) error {
	data, err := mc.ExportJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	return writeFile(filepath, data)
}

// GetSummary returns a string summary of all metrics
func (mc *MetricsCollector) GetSummary() string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	uptime := time.Since(mc.StartTime)
	totalRequests := int64(0)
	totalErrors := int64(0)
	totalLatency := time.Duration(0)

	for _, metrics := range mc.PerformanceMetrics {
		totalRequests += metrics.TotalRequests
		totalErrors += metrics.FailedReqs
		totalLatency += metrics.TotalLatency
	}

	overallErrorRate := 0.0
	if totalRequests > 0 {
		overallErrorRate = float64(totalErrors) / float64(totalRequests) * 100
	}

	avgLatency := time.Duration(0)
	if totalRequests > 0 {
		avgLatency = totalLatency / time.Duration(totalRequests)
	}

	summary := fmt.Sprintf(`
=== METRICS SUMMARY ===
Uptime: %v
Total Requests: %d
Total Errors: %d
Error Rate: %.2f%%
Average Latency: %v
Task Completion Rate: %.2f%%
Memory Usage: %d MB
CPU Usage: %.2f%%
Goroutines: %d
Models Tracked: %d
`, uptime, totalRequests, totalErrors, overallErrorRate, avgLatency,
		mc.TaskStats.SuccessRate, mc.ResourceMetrics.MemoryUsageMB,
		mc.ResourceMetrics.CPUUsagePercent, mc.ResourceMetrics.GoroutineCount,
		len(mc.PerformanceMetrics))

	return summary
}

// GetModelSummary returns a summary for a specific model
func (mc *MetricsCollector) GetModelSummary(model string) (string, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	metrics, exists := mc.PerformanceMetrics[model]
	if !exists {
		return "", fmt.Errorf("model %s not found", model)
	}

	summary := fmt.Sprintf(`
=== MODEL METRICS: %s ===
Total Requests: %d
Successful Requests: %d
Failed Requests: %d
Success Rate: %.2f%%
Error Rate: %.2f%%
Min Latency: %v
Max Latency: %v
Avg Latency: %v
Throughput: %.2f req/s
Tokens Processed: %d
Avg Tokens/Request: %.2f
Last Updated: %v
`, model,
		metrics.TotalRequests,
		metrics.SuccessfulReqs,
		metrics.FailedReqs,
		(float64(metrics.SuccessfulReqs)/float64(metrics.TotalRequests))*100,
		metrics.ErrorRate,
		metrics.MinLatency,
		metrics.MaxLatency,
		metrics.AvgLatency,
		metrics.Throughput,
		metrics.TokensProcessed,
		metrics.AvgTokensPerReq,
		metrics.LastUpdated)

	return summary, nil
}

// ResetMetrics resets all metrics
func (mc *MetricsCollector) ResetMetrics() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.PerformanceMetrics = make(map[string]*PerformanceMetrics)
	mc.TaskStats = &TaskStatistics{}
	mc.StartTime = time.Now()
}

// newLatencyHistogram creates a new latency histogram
func (mc *MetricsCollector) newLatencyHistogram() *LatencyHistogram {
	buckets := make(map[string]int64)
	for _, bucket := range mc.latencyBuckets {
		buckets[fmt.Sprintf("%dms", bucket)] = 0
	}
	buckets["10000ms+"] = 0

	return &LatencyHistogram{
		Buckets: buckets,
		Min:     time.Duration(0),
		Max:     time.Duration(0),
	}
}

// updateHistogram updates the latency histogram with a new measurement
func (mc *MetricsCollector) updateHistogram(hist *LatencyHistogram, latency time.Duration) {
	hist.Count++
	hist.Total += latency

	if hist.Count == 1 {
		hist.Min = latency
		hist.Max = latency
	} else {
		if latency < hist.Min {
			hist.Min = latency
		}
		if latency > hist.Max {
			hist.Max = latency
		}
	}

	latencyMs := latency.Milliseconds()
	bucketed := false

	for _, bucket := range mc.latencyBuckets {
		if latencyMs <= bucket {
			key := fmt.Sprintf("%dms", bucket)
			hist.Buckets[key]++
			bucketed = true
			break
		}
	}

	if !bucketed {
		hist.Buckets["10000ms+"]++
	}
}

// GetHistogramSummary returns a formatted summary of the latency histogram for a model
func (mc *MetricsCollector) GetHistogramSummary(model string) (string, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	metrics, exists := mc.PerformanceMetrics[model]
	if !exists {
		return "", fmt.Errorf("model %s not found", model)
	}

	if metrics.LatencyHist == nil {
		return "", fmt.Errorf("no latency histogram for model %s", model)
	}

	hist := metrics.LatencyHist
	summary := fmt.Sprintf("=== LATENCY HISTOGRAM: %s ===\nMin: %v, Max: %v, Avg: %v\n",
		model, hist.Min, hist.Max, hist.Total/time.Duration(hist.Count))

	// Sort buckets for consistent output
	keys := make([]string, 0, len(hist.Buckets))
	for k := range hist.Buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		count := hist.Buckets[key]
		percentage := 0.0
		if hist.Count > 0 {
			percentage = float64(count) / float64(hist.Count) * 100
		}
		summary += fmt.Sprintf("  %s: %d (%.2f%%)\n", key, count, percentage)
	}

	return summary, nil
}

// Close closes the metrics channel
func (mc *MetricsCollector) Close() {
	close(mc.MetricsChannel)
}

// RemoveModel removes a specific model's metrics from the collector
func (mc *MetricsCollector) RemoveModel(model string) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, exists := mc.PerformanceMetrics[model]; exists {
		delete(mc.PerformanceMetrics, model)
		return true
	}
	return false
}

// evictLeastRecentlyUsedModel removes the least recently updated model to make room
// Must be called with mc.mu locked
func (mc *MetricsCollector) evictLeastRecentlyUsedModel() {
	if len(mc.PerformanceMetrics) == 0 {
		return
	}

	var oldestModel string
	var oldestTime time.Time
	first := true

	for model, metrics := range mc.PerformanceMetrics {
		if first || metrics.LastUpdated.Before(oldestTime) {
			oldestModel = model
			oldestTime = metrics.LastUpdated
			first = false
		}
	}

	if oldestModel != "" {
		delete(mc.PerformanceMetrics, oldestModel)
		// Note: We don't log here as this is called with mutex locked
		// Logging would need to happen after unlock to avoid potential deadlocks
	}
}

// writeFile writes data to a file with proper permissions
func writeFile(filepath string, data []byte) error {
	return os.WriteFile(filepath, data, 0644)
}
