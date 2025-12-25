package concurrency

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"
)

// Example demonstrates various use cases for the worker pool.

// InstanceOperation represents a job that performs an operation on an instance.
type InstanceOperation struct {
	instanceID string
	operation  string
	priority   int
	timeout    time.Duration
}

func (op *InstanceOperation) Execute(ctx context.Context) (interface{}, error) {
	// Simulate instance operation
	select {
	case <-time.After(op.timeout):
		// Simulate random failures (10% chance)
		if rand.Float64() < 0.1 {
			return nil, fmt.Errorf("operation %s on instance %s failed", op.operation, op.instanceID)
		}
		return fmt.Sprintf("completed %s on instance %s", op.operation, op.instanceID), nil
	case <-ctx.Done():
		return nil, fmt.Errorf("operation %s on instance %s cancelled: %w", op.operation, op.instanceID, ctx.Err())
	}
}

func (op *InstanceOperation) Priority() int {
	return op.priority
}

func (op *InstanceOperation) ID() string {
	return fmt.Sprintf("%s-%s", op.instanceID, op.operation)
}

// ExampleBasicUsage demonstrates basic worker pool usage.
func ExampleWorkerPoolBasicUsage() {
	// Create pool with custom configuration
	config := WorkerPoolConfig{
		MaxWorkers:          5,
		QueueSize:           100,
		WorkerTimeout:       30 * time.Second,
		HealthCheckInterval: 10 * time.Second,
	}
	pool := NewWorkerPool(config)

	// Start the pool
	if err := pool.Start(); err != nil {
		log.Fatalf("Failed to start worker pool: %v", err)
	}

	// Submit jobs
	for i := 0; i < 20; i++ {
		job := &InstanceOperation{
			instanceID: fmt.Sprintf("instance-%d", i),
			operation:  "deploy",
			priority:   i % 5, // Priority 0-4
			timeout:    time.Duration(100+rand.Intn(400)) * time.Millisecond,
		}

		if err := pool.Submit(context.Background(), job); err != nil {
			log.Printf("Failed to submit job: %v", err)
		}
	}

	// Collect results in a goroutine
	go func() {
		for result := range pool.Results() {
			if result.Error != nil {
				log.Printf("Job %s failed after %v: %v", result.JobID, result.Duration, result.Error)
			} else {
				log.Printf("Job %s succeeded in %v: %v", result.JobID, result.Duration, result.Result)
			}
		}
	}()

	// Wait for jobs to complete
	time.Sleep(5 * time.Second)

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := pool.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}

	// Print final metrics
	metrics := pool.Metrics()
	fmt.Printf("\nFinal Metrics:\n%s\n", metrics.String())
}

// ExamplePriorityProcessing demonstrates priority-based job processing.
func ExamplePriorityProcessing() {
	config := DefaultWorkerPoolConfig()
	config.MaxWorkers = 2 // Limited workers to show priority effect
	pool := NewWorkerPool(config)

	if err := pool.Start(); err != nil {
		log.Fatalf("Failed to start worker pool: %v", err)
	}

	// Submit jobs with different priorities
	operations := []struct {
		priority int
		name     string
	}{
		{priority: 1, name: "low-priority-cleanup"},
		{priority: 10, name: "critical-deployment"},
		{priority: 5, name: "medium-update"},
		{priority: 10, name: "critical-rollback"},
		{priority: 1, name: "low-priority-backup"},
	}

	for i, op := range operations {
		job := &InstanceOperation{
			instanceID: fmt.Sprintf("instance-%d", i),
			operation:  op.name,
			priority:   op.priority,
			timeout:    200 * time.Millisecond,
		}

		if err := pool.Submit(context.Background(), job); err != nil {
			log.Printf("Failed to submit job: %v", err)
		}
	}

	// Process results
	go func() {
		for result := range pool.Results() {
			log.Printf("Processed: %s (priority %d) in %v",
				result.JobID,
				// You would normally track priority separately
				0,
				result.Duration)
		}
	}()

	time.Sleep(3 * time.Second)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool.Shutdown(shutdownCtx)
}

// ExampleErrorAggregation demonstrates error collection and aggregation.
func ExampleErrorAggregation() {
	config := DefaultWorkerPoolConfig()
	pool := NewWorkerPool(config)

	if err := pool.Start(); err != nil {
		log.Fatalf("Failed to start worker pool: %v", err)
	}

	// Submit jobs (some will fail randomly)
	for i := 0; i < 10; i++ {
		job := &InstanceOperation{
			instanceID: fmt.Sprintf("instance-%d", i),
			operation:  "risky-operation",
			priority:   1,
			timeout:    100 * time.Millisecond,
		}
		pool.Submit(context.Background(), job)
	}

	// Collect all results
	go func() {
		time.Sleep(2 * time.Second)
		pool.Shutdown(context.Background())
	}()

	results, err := CollectResults(pool.Results())

	fmt.Printf("Processed %d jobs\n", len(results))
	if err != nil {
		fmt.Printf("Errors occurred:\n%v\n", err)
	}

	// Analyze results
	var successCount, failCount int
	for _, result := range results {
		if result.Error != nil {
			failCount++
		} else {
			successCount++
		}
	}

	fmt.Printf("Success: %d, Failed: %d\n", successCount, failCount)
}

// ExampleWorkerHealthMonitoring demonstrates worker health monitoring.
func ExampleWorkerHealthMonitoring() {
	config := DefaultWorkerPoolConfig()
	config.MaxWorkers = 5
	config.HealthCheckInterval = 1 * time.Second
	pool := NewWorkerPool(config)

	if err := pool.Start(); err != nil {
		log.Fatalf("Failed to start worker pool: %v", err)
	}

	// Monitor worker health in background
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			workers := pool.Workers()
			fmt.Printf("\n=== Worker Health Report ===\n")
			for _, worker := range workers {
				fmt.Printf("Worker %d: Status=%s, Jobs=%d, LastHeartbeat=%v ago",
					worker.ID(),
					worker.Status(),
					worker.JobsProcessed(),
					time.Since(worker.LastHeartbeat()))

				if err := worker.LastError(); err != nil {
					fmt.Printf(", LastError=%v", err)
				}
				fmt.Println()
			}

			metrics := pool.Metrics()
			fmt.Printf("\n%s\n", metrics.String())
		}
	}()

	// Submit continuous jobs
	for i := 0; i < 50; i++ {
		job := &InstanceOperation{
			instanceID: fmt.Sprintf("instance-%d", i),
			operation:  "monitor-test",
			priority:   1,
			timeout:    time.Duration(50+rand.Intn(200)) * time.Millisecond,
		}
		pool.Submit(context.Background(), job)
		time.Sleep(100 * time.Millisecond)
	}

	time.Sleep(8 * time.Second)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool.Shutdown(shutdownCtx)
}

// ExampleBatchProcessing demonstrates batch processing of multiple instances.
func ExampleBatchProcessing() {
	config := DefaultWorkerPoolConfig()
	config.MaxWorkers = 10
	pool := NewWorkerPool(config)

	if err := pool.Start(); err != nil {
		log.Fatalf("Failed to start worker pool: %v", err)
	}

	// Simulate batch operations on multiple instances
	instances := []string{"prod-1", "prod-2", "prod-3", "staging-1", "staging-2", "dev-1", "dev-2"}
	operations := []string{"backup", "update", "health-check", "cleanup"}

	for _, instance := range instances {
		for _, operation := range operations {
			// Production instances get higher priority
			priority := 1
			if instance[:4] == "prod" {
				priority = 10
			} else if instance[:7] == "staging" {
				priority = 5
			}

			job := &InstanceOperation{
				instanceID: instance,
				operation:  operation,
				priority:   priority,
				timeout:    time.Duration(100+rand.Intn(300)) * time.Millisecond,
			}

			if err := pool.Submit(context.Background(), job); err != nil {
				log.Printf("Failed to submit job: %v", err)
			}
		}
	}

	// Track results per instance
	instanceResults := make(map[string][]JobResult)

	go func() {
		for result := range pool.Results() {
			// Extract instance ID from job ID
			instanceID := result.JobID[:len(result.JobID)-len("-backup")] // Simplified extraction
			instanceResults[instanceID] = append(instanceResults[instanceID], result)

			if result.Error != nil {
				log.Printf("❌ %s failed: %v", result.JobID, result.Error)
			} else {
				log.Printf("✅ %s completed in %v", result.JobID, result.Duration)
			}
		}
	}()

	time.Sleep(5 * time.Second)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool.Shutdown(shutdownCtx)

	// Print summary
	fmt.Printf("\n=== Batch Processing Summary ===\n")
	for instance, results := range instanceResults {
		fmt.Printf("%s: %d operations completed\n", instance, len(results))
	}

	metrics := pool.Metrics()
	fmt.Printf("\n%s\n", metrics.String())
}

// ExampleGracefulShutdown demonstrates proper shutdown handling.
func ExampleWorkerPoolGracefulShutdown() {
	config := DefaultWorkerPoolConfig()
	pool := NewWorkerPool(config)

	if err := pool.Start(); err != nil {
		log.Fatalf("Failed to start worker pool: %v", err)
	}

	// Submit long-running jobs
	for i := 0; i < 20; i++ {
		job := &InstanceOperation{
			instanceID: fmt.Sprintf("instance-%d", i),
			operation:  "long-operation",
			priority:   1,
			timeout:    2 * time.Second,
		}
		pool.Submit(context.Background(), job)
	}

	// Process results
	resultCount := 0
	go func() {
		for result := range pool.Results() {
			resultCount++
			log.Printf("Completed job %s (%d/%d)", result.JobID, resultCount, 20)
		}
	}()

	// Simulate interrupt after 3 seconds
	time.Sleep(3 * time.Second)
	log.Println("Initiating graceful shutdown...")

	// Graceful shutdown with reasonable timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := pool.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown completed with warnings: %v", err)
	} else {
		log.Println("Shutdown completed successfully")
	}

	// Final statistics
	metrics := pool.Metrics()
	fmt.Printf("\nProcessed %d/%d jobs before shutdown\n",
		metrics.JobsCompleted.Load()+metrics.JobsFailed.Load(),
		metrics.JobsSubmitted.Load())
}
