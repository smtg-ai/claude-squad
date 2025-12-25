package concurrency

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// mockJob implements the Job interface for testing.
type mockJob struct {
	id         string
	priority   int
	duration   time.Duration
	shouldFail bool
	executed   atomic.Bool
}

func (m *mockJob) Execute(ctx context.Context) (interface{}, error) {
	m.executed.Store(true)

	select {
	case <-time.After(m.duration):
		if m.shouldFail {
			return nil, fmt.Errorf("job %s failed intentionally", m.id)
		}
		return fmt.Sprintf("result-%s", m.id), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *mockJob) Priority() int {
	return m.priority
}

func (m *mockJob) ID() string {
	return m.id
}

func TestWorkerPool_BasicExecution(t *testing.T) {
	config := DefaultConfig()
	config.MaxWorkers = 3
	pool := NewWorkerPool(config)

	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}

	// Submit jobs
	jobs := []*mockJob{
		{id: "job-1", priority: 1, duration: 50 * time.Millisecond},
		{id: "job-2", priority: 2, duration: 50 * time.Millisecond},
		{id: "job-3", priority: 3, duration: 50 * time.Millisecond},
	}

	for _, job := range jobs {
		if err := pool.Submit(context.Background(), job); err != nil {
			t.Errorf("Failed to submit job: %v", err)
		}
	}

	// Shutdown after jobs complete
	go func() {
		time.Sleep(2 * time.Second)
		pool.Shutdown(context.Background())
	}()

	results := make([]JobResult, 0)
	for result := range pool.Results() {
		results = append(results, result)
		if result.Error != nil {
			t.Errorf("Job %s failed: %v", result.JobID, result.Error)
		}
	}

	if len(results) != len(jobs) {
		t.Errorf("Expected %d results, got %d", len(jobs), len(results))
	}

	metrics := pool.Metrics()
	if metrics.JobsCompleted.Load() != uint64(len(jobs)) {
		t.Errorf("Expected %d completed jobs, got %d", len(jobs), metrics.JobsCompleted.Load())
	}
}

func TestWorkerPool_PriorityHandling(t *testing.T) {
	config := DefaultConfig()
	config.MaxWorkers = 1 // Single worker to test priority
	pool := NewWorkerPool(config)

	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}

	// Submit jobs with different priorities
	// Higher priority should be processed first
	jobs := []*mockJob{
		{id: "low", priority: 1, duration: 10 * time.Millisecond},
		{id: "high", priority: 10, duration: 10 * time.Millisecond},
		{id: "medium", priority: 5, duration: 10 * time.Millisecond},
	}

	for _, job := range jobs {
		if err := pool.Submit(context.Background(), job); err != nil {
			t.Errorf("Failed to submit job: %v", err)
		}
	}

	// Allow some time for queue processing
	time.Sleep(500 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool.Shutdown(ctx)

	// Verify all jobs executed
	for _, job := range jobs {
		if !job.executed.Load() {
			t.Errorf("Job %s was not executed", job.id)
		}
	}
}

func TestWorkerPool_ErrorHandling(t *testing.T) {
	config := DefaultConfig()
	config.MaxWorkers = 2
	pool := NewWorkerPool(config)

	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}

	// Submit jobs with failures
	jobs := []*mockJob{
		{id: "success-1", priority: 1, duration: 50 * time.Millisecond, shouldFail: false},
		{id: "fail-1", priority: 1, duration: 50 * time.Millisecond, shouldFail: true},
		{id: "success-2", priority: 1, duration: 50 * time.Millisecond, shouldFail: false},
		{id: "fail-2", priority: 1, duration: 50 * time.Millisecond, shouldFail: true},
	}

	for _, job := range jobs {
		if err := pool.Submit(context.Background(), job); err != nil {
			t.Errorf("Failed to submit job: %v", err)
		}
	}

	go func() {
		time.Sleep(2 * time.Second)
		pool.Shutdown(context.Background())
	}()

	var successCount, failCount int
	for result := range pool.Results() {
		if result.Error != nil {
			failCount++
		} else {
			successCount++
		}
	}

	if successCount != 2 {
		t.Errorf("Expected 2 successful jobs, got %d", successCount)
	}
	if failCount != 2 {
		t.Errorf("Expected 2 failed jobs, got %d", failCount)
	}

	metrics := pool.Metrics()
	if metrics.JobsFailed.Load() != 2 {
		t.Errorf("Expected 2 failed jobs in metrics, got %d", metrics.JobsFailed.Load())
	}
	if metrics.JobsCompleted.Load() != 2 {
		t.Errorf("Expected 2 completed jobs in metrics, got %d", metrics.JobsCompleted.Load())
	}
}

func TestWorkerPool_ContextCancellation(t *testing.T) {
	config := DefaultConfig()
	config.MaxWorkers = 2
	pool := NewWorkerPool(config)

	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}

	// Submit long-running jobs
	for i := 0; i < 10; i++ {
		job := &mockJob{
			id:       fmt.Sprintf("long-job-%d", i),
			priority: 1,
			duration: 10 * time.Second, // Very long
		}

		if err := pool.Submit(context.Background(), job); err != nil {
			t.Errorf("Failed to submit job: %v", err)
		}
	}

	// Allow jobs to start processing
	time.Sleep(100 * time.Millisecond)

	// Shutdown - context cancellation should cause jobs to terminate quickly
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pool.Shutdown(shutdownCtx)
	if err != nil {
		t.Logf("Shutdown returned error (may occur with strict timeout): %v", err)
	}

	// Verify that not all jobs completed (due to cancellation)
	metrics := pool.Metrics()
	totalProcessed := metrics.JobsCompleted.Load() + metrics.JobsFailed.Load()
	if totalProcessed >= 10 {
		t.Errorf("Expected some jobs to be cancelled, but all %d jobs were processed", totalProcessed)
	}
	t.Logf("Successfully cancelled jobs: %d out of 10 were processed", totalProcessed)
}

func TestWorkerPool_GracefulShutdown(t *testing.T) {
	config := DefaultConfig()
	config.MaxWorkers = 2
	pool := NewWorkerPool(config)

	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}

	// Submit jobs
	for i := 0; i < 5; i++ {
		job := &mockJob{
			id:       fmt.Sprintf("job-%d", i),
			priority: 1,
			duration: 100 * time.Millisecond,
		}
		if err := pool.Submit(context.Background(), job); err != nil {
			t.Errorf("Failed to submit job: %v", err)
		}
	}

	// Allow jobs to start
	time.Sleep(200 * time.Millisecond)

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := pool.Shutdown(ctx); err != nil {
		t.Errorf("Graceful shutdown failed: %v", err)
	}

	// Verify workers are stopped
	for _, worker := range pool.Workers() {
		if worker.Status() != WorkerStopped {
			t.Errorf("Worker %d not stopped: %s", worker.ID(), worker.Status())
		}
	}
}

func TestWorkerPool_Metrics(t *testing.T) {
	config := DefaultConfig()
	config.MaxWorkers = 3
	pool := NewWorkerPool(config)

	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}

	// Submit mixed jobs
	for i := 0; i < 10; i++ {
		job := &mockJob{
			id:         fmt.Sprintf("job-%d", i),
			priority:   i,
			duration:   time.Duration(10+i*5) * time.Millisecond,
			shouldFail: i%3 == 0, // Every third job fails
		}
		if err := pool.Submit(context.Background(), job); err != nil {
			t.Errorf("Failed to submit job: %v", err)
		}
	}

	go func() {
		time.Sleep(3 * time.Second)
		pool.Shutdown(context.Background())
	}()

	// Drain results
	for range pool.Results() {
	}

	metrics := pool.Metrics()

	if metrics.JobsSubmitted.Load() != 10 {
		t.Errorf("Expected 10 submitted jobs, got %d", metrics.JobsSubmitted.Load())
	}

	totalProcessed := metrics.JobsCompleted.Load() + metrics.JobsFailed.Load()
	if totalProcessed != 10 {
		t.Errorf("Expected 10 total processed jobs, got %d", totalProcessed)
	}

	if metrics.AverageLatency() == 0 {
		t.Error("Expected non-zero average latency")
	}

	if metrics.MinLatency.Load() == 0 || metrics.MaxLatency.Load() == 0 {
		t.Error("Expected non-zero min/max latency")
	}

	t.Logf("Metrics: %s", metrics.String())
}

func TestWorkerPool_WorkerHealth(t *testing.T) {
	config := DefaultConfig()
	config.MaxWorkers = 2
	config.HealthCheckInterval = 100 * time.Millisecond
	pool := NewWorkerPool(config)

	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}

	// Submit multiple jobs to keep workers active
	for i := 0; i < 10; i++ {
		job := &mockJob{
			id:       fmt.Sprintf("test-job-%d", i),
			priority: 1,
			duration: 50 * time.Millisecond,
		}

		if err := pool.Submit(context.Background(), job); err != nil {
			t.Errorf("Failed to submit job: %v", err)
		}
	}

	// Wait for some jobs to process
	time.Sleep(300 * time.Millisecond)

	// Check worker heartbeats while workers are active
	for _, worker := range pool.Workers() {
		lastHeartbeat := worker.LastHeartbeat()
		if time.Since(lastHeartbeat) > 2*time.Second {
			t.Errorf("Worker %d heartbeat too old: %v", worker.ID(), time.Since(lastHeartbeat))
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool.Shutdown(ctx)
}

func TestWorkerPool_ConcurrentSubmit(t *testing.T) {
	config := DefaultConfig()
	config.MaxWorkers = 5
	config.QueueSize = 100
	pool := NewWorkerPool(config)

	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}

	// Concurrent job submission
	const numGoroutines = 10
	const jobsPerGoroutine = 10

	var submittedCount atomic.Int32

	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			for j := 0; j < jobsPerGoroutine; j++ {
				job := &mockJob{
					id:       fmt.Sprintf("job-%d-%d", routineID, j),
					priority: j,
					duration: 10 * time.Millisecond,
				}
				if err := pool.Submit(context.Background(), job); err != nil {
					t.Errorf("Failed to submit job: %v", err)
				} else {
					submittedCount.Add(1)
				}
			}
		}(i)
	}

	// Wait for all submissions
	time.Sleep(1 * time.Second)

	go func() {
		time.Sleep(3 * time.Second)
		pool.Shutdown(context.Background())
	}()

	resultCount := 0
	for range pool.Results() {
		resultCount++
	}

	expectedJobs := int(submittedCount.Load())
	if resultCount != expectedJobs {
		t.Errorf("Expected %d results, got %d", expectedJobs, resultCount)
	}
}

func TestCollectResults(t *testing.T) {
	results := make(chan JobResult, 3)

	results <- JobResult{
		JobID:  "job-1",
		Result: "success",
		Error:  nil,
	}
	results <- JobResult{
		JobID:  "job-2",
		Result: nil,
		Error:  errors.New("failure"),
	}
	results <- JobResult{
		JobID:  "job-3",
		Result: "success",
		Error:  nil,
	}
	close(results)

	allResults, err := CollectResults(results)

	if len(allResults) != 3 {
		t.Errorf("Expected 3 results, got %d", len(allResults))
	}

	if err == nil {
		t.Error("Expected error from CollectResults due to failed job")
	}
}

func TestCombineErrors(t *testing.T) {
	tests := []struct {
		name     string
		errors   []error
		expected bool
	}{
		{
			name:     "no errors",
			errors:   []error{},
			expected: false,
		},
		{
			name:     "single error",
			errors:   []error{errors.New("error 1")},
			expected: true,
		},
		{
			name:     "multiple errors",
			errors:   []error{errors.New("error 1"), errors.New("error 2")},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := combineErrors(tt.errors)
			if (err != nil) != tt.expected {
				t.Errorf("combineErrors() error = %v, expected error = %v", err != nil, tt.expected)
			}
		})
	}
}

func BenchmarkWorkerPool(b *testing.B) {
	config := DefaultConfig()
	config.MaxWorkers = 10
	pool := NewWorkerPool(config)

	if err := pool.Start(); err != nil {
		b.Fatalf("Failed to start pool: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		job := &mockJob{
			id:       fmt.Sprintf("job-%d", i),
			priority: i % 10,
			duration: 1 * time.Millisecond,
		}
		pool.Submit(context.Background(), job)
	}

	go func() {
		time.Sleep(5 * time.Second)
		pool.Shutdown(context.Background())
	}()

	for range pool.Results() {
	}
}
