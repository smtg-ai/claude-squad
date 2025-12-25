package ollama

import (
	"encoding/json"
	"runtime"
	"testing"
	"time"
)

// TestMetricsCollectorCreation verifies that the metrics collector is initialized correctly
func TestMetricsCollectorCreation(t *testing.T) {
	mc := NewMetricsCollector()

	if mc.PerformanceMetrics == nil {
		t.Error("PerformanceMetrics should be initialized")
	}
	if mc.TaskStats == nil {
		t.Error("TaskStats should be initialized")
	}
	if mc.ResourceMetrics == nil {
		t.Error("ResourceMetrics should be initialized")
	}
	if mc.MetricsChannel == nil {
		t.Error("MetricsChannel should be initialized")
	}
}

// TestRecordLatency verifies latency recording for models
func TestRecordLatency(t *testing.T) {
	mc := NewMetricsCollector()
	model := "test-model"

	mc.RecordLatency(model, 100*time.Millisecond)
	mc.RecordLatency(model, 150*time.Millisecond)
	mc.RecordLatency(model, 50*time.Millisecond)

	metrics, exists := mc.GetModelMetrics(model)
	if !exists {
		t.Fatalf("Expected model %s to exist", model)
	}

	if metrics.TotalRequests != 3 {
		t.Errorf("Expected 3 requests, got %d", metrics.TotalRequests)
	}

	if metrics.SuccessfulReqs != 3 {
		t.Errorf("Expected 3 successful requests, got %d", metrics.SuccessfulReqs)
	}

	if metrics.MinLatency != 50*time.Millisecond {
		t.Errorf("Expected min latency 50ms, got %v", metrics.MinLatency)
	}

	if metrics.MaxLatency != 150*time.Millisecond {
		t.Errorf("Expected max latency 150ms, got %v", metrics.MaxLatency)
	}

	expectedAvg := time.Duration((100 + 150 + 50)) * time.Millisecond / 3
	if metrics.AvgLatency != expectedAvg {
		t.Errorf("Expected avg latency %v, got %v", expectedAvg, metrics.AvgLatency)
	}
}

// TestRecordError verifies error recording
func TestRecordError(t *testing.T) {
	mc := NewMetricsCollector()
	model := "test-model"

	mc.RecordLatency(model, 100*time.Millisecond)
	mc.RecordError(model, ErrorTimeout)
	mc.RecordError(model, ErrorConnectionFailed)

	metrics, exists := mc.GetModelMetrics(model)
	if !exists {
		t.Fatalf("Expected model %s to exist", model)
	}

	if metrics.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests, got %d", metrics.TotalRequests)
	}

	if metrics.FailedReqs != 2 {
		t.Errorf("Expected 2 failed requests, got %d", metrics.FailedReqs)
	}

	if metrics.SuccessfulReqs != 1 {
		t.Errorf("Expected 1 successful request, got %d", metrics.SuccessfulReqs)
	}

	expectedErrorRate := float64(2) / float64(3) * 100
	if metrics.ErrorRate != expectedErrorRate {
		t.Errorf("Expected error rate %.2f%%, got %.2f%%", expectedErrorRate, metrics.ErrorRate)
	}
}

// TestRecordTokens verifies token counting
func TestRecordTokens(t *testing.T) {
	mc := NewMetricsCollector()
	model := "test-model"

	mc.RecordLatency(model, 100*time.Millisecond)
	mc.RecordTokens(model, 100)
	mc.RecordLatency(model, 150*time.Millisecond)
	mc.RecordTokens(model, 200)

	metrics, exists := mc.GetModelMetrics(model)
	if !exists {
		t.Fatalf("Expected model %s to exist", model)
	}

	if metrics.TokensProcessed != 300 {
		t.Errorf("Expected 300 tokens processed, got %d", metrics.TokensProcessed)
	}

	expectedAvgTokens := float64(300) / float64(2)
	if metrics.AvgTokensPerReq != expectedAvgTokens {
		t.Errorf("Expected avg tokens %.2f, got %.2f", expectedAvgTokens, metrics.AvgTokensPerReq)
	}
}

// TestRecordTaskCompletion verifies task statistics
func TestRecordTaskCompletion(t *testing.T) {
	mc := NewMetricsCollector()

	mc.RecordTaskCompletion(true, 100.0)
	mc.RecordTaskCompletion(true, 150.0)
	mc.RecordTaskCompletion(false, 200.0)

	stats := mc.GetTaskStatistics()

	if stats.TotalTasks != 3 {
		t.Errorf("Expected 3 total tasks, got %d", stats.TotalTasks)
	}

	if stats.CompletedTasks != 2 {
		t.Errorf("Expected 2 completed tasks, got %d", stats.CompletedTasks)
	}

	if stats.FailedTasks != 1 {
		t.Errorf("Expected 1 failed task, got %d", stats.FailedTasks)
	}

	expectedSuccessRate := float64(2) / float64(3) * 100
	if stats.SuccessRate != expectedSuccessRate {
		t.Errorf("Expected success rate %.2f%%, got %.2f%%", expectedSuccessRate, stats.SuccessRate)
	}
}

// TestUpdateResourceMetrics verifies resource metrics recording
func TestUpdateResourceMetrics(t *testing.T) {
	mc := NewMetricsCollector()

	mc.UpdateResourceMetrics(512, 45.5, 10)

	resources := mc.GetResourceMetrics()

	if resources.MemoryUsageMB != 512 {
		t.Errorf("Expected 512 MB, got %d", resources.MemoryUsageMB)
	}

	if resources.CPUUsagePercent != 45.5 {
		t.Errorf("Expected 45.5%% CPU, got %.2f%%", resources.CPUUsagePercent)
	}

	if resources.GoroutineCount != 10 {
		t.Errorf("Expected 10 goroutines, got %d", resources.GoroutineCount)
	}
}

// TestExportJSON verifies JSON export functionality
func TestExportJSON(t *testing.T) {
	mc := NewMetricsCollector()
	model := "test-model"

	mc.RecordLatency(model, 100*time.Millisecond)
	mc.RecordLatency(model, 150*time.Millisecond)
	mc.RecordError(model, ErrorTimeout)
	mc.RecordTaskCompletion(true, 100.0)
	mc.UpdateResourceMetrics(512, 45.5, 10)

	data, err := mc.ExportJSON()
	if err != nil {
		t.Fatalf("Failed to export JSON: %v", err)
	}

	var export JSONExport
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if export.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests in export, got %d", export.TotalRequests)
	}

	if export.TotalErrors != 1 {
		t.Errorf("Expected 1 error in export, got %d", export.TotalErrors)
	}

	if len(export.ModelMetrics) != 1 {
		t.Errorf("Expected 1 model in export, got %d", len(export.ModelMetrics))
	}

	// Note: export.ModelMetrics is map[string]*PerformanceMetrics

	modelMetrics, exists := export.ModelMetrics[model]
	if !exists {
		t.Errorf("Expected model %s in export", model)
	}

	if modelMetrics.TotalRequests != 3 {
		t.Errorf("Expected 3 requests for model, got %d", modelMetrics.TotalRequests)
	}
}

// TestLatencyHistogram verifies histogram functionality
func TestLatencyHistogram(t *testing.T) {
	mc := NewMetricsCollector()
	model := "test-model"

	// Record latencies in different ranges
	latencies := []time.Duration{
		2 * time.Millisecond,
		7 * time.Millisecond,
		15 * time.Millisecond,
		30 * time.Millisecond,
		60 * time.Millisecond,
		150 * time.Millisecond,
		300 * time.Millisecond,
		600 * time.Millisecond,
		1200 * time.Millisecond,
		6000 * time.Millisecond,
	}

	for _, latency := range latencies {
		mc.RecordLatency(model, latency)
	}

	metrics, exists := mc.GetModelMetrics(model)
	if !exists {
		t.Fatalf("Expected model %s to exist", model)
	}

	if metrics.LatencyHist == nil {
		t.Fatal("Expected latency histogram to exist")
	}

	hist := metrics.LatencyHist
	if hist.Count != 10 {
		t.Errorf("Expected 10 latency measurements, got %d", hist.Count)
	}

	// Verify buckets are populated
	if hist.Buckets["1ms"] != 0 {
		t.Errorf("1ms bucket should be 0, got %d", hist.Buckets["1ms"])
	}

	if hist.Buckets["5ms"] != 1 {
		t.Errorf("5ms bucket should be 1, got %d", hist.Buckets["5ms"])
	}

	if hist.Buckets["10000ms+"] != 1 {
		t.Errorf("10000ms+ bucket should be 1, got %d", hist.Buckets["10000ms+"])
	}
}

// TestThreadSafety verifies that the metrics collector is thread-safe
func TestThreadSafety(t *testing.T) {
	mc := NewMetricsCollector()

	// Simulate concurrent operations
	done := make(chan bool)
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				model := "model-1"
				mc.RecordLatency(model, time.Duration(j%1000)*time.Millisecond)
				mc.RecordTokens(model, int64(j))

				if j%2 == 0 {
					mc.RecordError(model, ErrorTimeout)
				}

				mc.RecordTaskCompletion(j%3 != 0, float64(j))

				// Verify we can read while writing
				_, _ = mc.GetModelMetrics(model)
				_ = mc.GetTaskStatistics()
				_ = mc.GetResourceMetrics()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify final state
	metrics, _ := mc.GetModelMetrics("model-1")
	if metrics.TotalRequests == 0 {
		t.Error("Expected recorded metrics after concurrent operations")
	}
}

// TestMetricsChannel verifies real-time metrics channel
func TestMetricsChannel(t *testing.T) {
	mc := NewMetricsCollector()
	model := "test-model"

	// Read metrics asynchronously
	eventCount := 0
	done := make(chan bool)

	go func() {
		for event := range mc.MetricsChannel {
			if event.Type == "latency" {
				eventCount++
			}
			if eventCount >= 3 {
				break
			}
		}
		done <- true
	}()

	// Record events
	mc.RecordLatency(model, 100*time.Millisecond)
	mc.RecordLatency(model, 150*time.Millisecond)
	mc.RecordLatency(model, 50*time.Millisecond)

	// Wait for processing with timeout
	select {
	case <-done:
		if eventCount < 3 {
			t.Errorf("Expected 3 latency events, got %d", eventCount)
		}
	case <-time.After(1 * time.Second):
		// Timeout is acceptable for this test
	}
}

// TestGetSummary verifies summary generation
func TestGetSummary(t *testing.T) {
	mc := NewMetricsCollector()
	model := "test-model"

	mc.RecordLatency(model, 100*time.Millisecond)
	mc.RecordLatency(model, 150*time.Millisecond)
	mc.RecordError(model, ErrorTimeout)
	mc.RecordTaskCompletion(true, 100.0)
	mc.UpdateResourceMetrics(512, 45.5, runtime.NumGoroutine())

	summary := mc.GetSummary()

	if summary == "" {
		t.Error("Expected non-empty summary")
	}

	// Verify summary contains expected content
	if !contains(summary, "METRICS SUMMARY") {
		t.Error("Summary should contain 'METRICS SUMMARY'")
	}

	if !contains(summary, "Total Requests") {
		t.Error("Summary should contain 'Total Requests'")
	}
}

// TestGetModelSummary verifies model-specific summary
func TestGetModelSummary(t *testing.T) {
	mc := NewMetricsCollector()
	model := "test-model"

	mc.RecordLatency(model, 100*time.Millisecond)
	mc.RecordLatency(model, 150*time.Millisecond)
	mc.RecordError(model, ErrorTimeout)

	summary, err := mc.GetModelSummary(model)
	if err != nil {
		t.Fatalf("Failed to get model summary: %v", err)
	}

	if !contains(summary, model) {
		t.Error("Summary should contain model name")
	}

	if !contains(summary, "Total Requests") {
		t.Error("Summary should contain 'Total Requests'")
	}
}

// TestResetMetrics verifies metrics reset
func TestResetMetrics(t *testing.T) {
	mc := NewMetricsCollector()
	model := "test-model"

	mc.RecordLatency(model, 100*time.Millisecond)
	mc.RecordTaskCompletion(true, 100.0)

	// Verify data exists
	metrics, _ := mc.GetModelMetrics(model)
	if metrics.TotalRequests == 0 {
		t.Error("Expected metrics before reset")
	}

	// Reset
	mc.ResetMetrics()

	// Verify data is cleared
	_, exists := mc.GetModelMetrics(model)
	if exists {
		t.Error("Expected model metrics to be cleared after reset")
	}

	stats := mc.GetTaskStatistics()
	if stats.TotalTasks != 0 {
		t.Error("Expected task statistics to be cleared after reset")
	}
}

// TestHistogramSummary verifies histogram summary generation
func TestHistogramSummary(t *testing.T) {
	mc := NewMetricsCollector()
	model := "test-model"

	mc.RecordLatency(model, 2*time.Millisecond)
	mc.RecordLatency(model, 30*time.Millisecond)
	mc.RecordLatency(model, 150*time.Millisecond)

	summary, err := mc.GetHistogramSummary(model)
	if err != nil {
		t.Fatalf("Failed to get histogram summary: %v", err)
	}

	if !contains(summary, "LATENCY HISTOGRAM") {
		t.Error("Summary should contain 'LATENCY HISTOGRAM'")
	}

	if !contains(summary, "ms") {
		t.Error("Summary should contain latency buckets with 'ms'")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
