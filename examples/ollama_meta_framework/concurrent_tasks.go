package main

import (
	"claude-squad/log"
	"claude-squad/ollama"
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// ExampleConcurrentTasks demonstrates parallel execution of 10 tasks using the meta framework.
// This example showcases:
// 1. Creating a dispatcher with multiple workers
// 2. Submitting tasks with different priorities
// 3. Real-time progress monitoring
// 4. Error handling and recovery
// 5. Collecting metrics across all tasks
//
// Run with: go run concurrent_tasks.go
func main() {
	// Initialize logging
	log.Initialize(false)
	defer log.Close()

	ctx := context.Background()
	seed := time.Now().UnixNano()
	rand.Seed(seed)

	// Agent function that simulates variable processing times
	agentFunc := func(ctx context.Context, task *ollama.Task) error {
		// Extract task parameters
		taskType := "unknown"
		if payload, ok := task.Payload.(map[string]interface{}); ok {
			if t, exists := payload["type"]; exists {
				taskType = t.(string)
			}
		}

		fmt.Printf("[Worker] Starting %s task: %s\n", taskType, task.ID)

		// Simulate variable processing time based on task type
		processingTime := time.Duration(1000+rand.Intn(3000)) * time.Millisecond

		// Simulate occasional failures (10% chance)
		if rand.Float64() < 0.1 {
			fmt.Printf("[Worker] Task %s failed (simulated)\n", task.ID)
			return fmt.Errorf("task processing failed")
		}

		// Wait for completion with context cancellation support
		select {
		case <-time.After(processingTime):
			task.Result = map[string]interface{}{
				"type":          taskType,
				"processed_at":  time.Now(),
				"duration_ms":   processingTime.Milliseconds(),
				"worker_result": "success",
			}
			fmt.Printf("[Worker] Task %s completed in %v\n", task.ID, processingTime)
			return nil

		case <-ctx.Done():
			fmt.Printf("[Worker] Task %s cancelled\n", task.ID)
			return ctx.Err()
		}
	}

	// Create dispatcher with 5 workers for parallel execution
	dispatcher, err := ollama.NewTaskDispatcher(ctx, agentFunc, 5)
	if err != nil {
		log.ErrorLog.Printf("Failed to create dispatcher: %v", err)
		return
	}

	// Track task results with a goroutine-safe map
	var resultsMu sync.Mutex
	results := make(map[string]*ollama.Task)

	// Set up comprehensive progress callback
	dispatcher.SetProgressCallback(func(taskID string, status ollama.TaskStatus, progress int, message string) {
		// Only print status changes to reduce noise
		if progress == 0 || progress == 100 {
			fmt.Printf("[Progress] %s: %s (%d%%) - %s\n", taskID, status.String(), progress, message)
		}
	})

	// Start the dispatcher
	if err := dispatcher.Start(); err != nil {
		log.ErrorLog.Printf("Failed to start dispatcher: %v", err)
		return
	}

	fmt.Println("\n=== Concurrent Tasks Example ===")
	fmt.Println("Submitting 10 tasks with variable priorities...\n")

	// Create and submit 10 tasks with varying priorities
	tasks := make([]*ollama.Task, 0, 10)
	taskTypes := []string{"analysis", "generation", "review", "optimization", "validation"}

	for i := 0; i < 10; i++ {
		taskType := taskTypes[i%len(taskTypes)]
		priority := ollama.PriorityLow
		if i%3 == 0 {
			priority = ollama.PriorityHigh
		}

		task := &ollama.Task{
			ID:       fmt.Sprintf("task-%03d", i+1),
			Priority: priority,
			Payload: map[string]interface{}{
				"type":     taskType,
				"sequence": i + 1,
				"model":    "llama2:7b",
			},
		}

		tasks = append(tasks, task)

		if err := dispatcher.SubmitTask(task); err != nil {
			log.ErrorLog.Printf("Failed to submit task %s: %v", task.ID, err)
		}
	}

	fmt.Printf("Submitted %d tasks. Waiting for completion...\n\n", len(tasks))

	// Monitor execution in background
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				metrics := dispatcher.GetMetrics()
				if metrics.PendingTasks > 0 {
					fmt.Printf("[Status] Completed: %d | Failed: %d | Pending: %d | Workers: %d\n",
						metrics.CompletedTasks, metrics.FailedTasks, metrics.PendingTasks, metrics.WorkerCount)
				}

				// Check if all tasks are done
				if metrics.PendingTasks == 0 && metrics.CompletedTasks+metrics.FailedTasks > 0 {
					close(done)
					return
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for all tasks to complete
	<-done
	time.Sleep(500 * time.Millisecond) // Small delay to ensure last operations complete

	// Collect final results
	fmt.Println("\n=== Final Results ===")
	for _, task := range tasks {
		completed, err := dispatcher.GetTask(task.ID)
		if err != nil {
			log.ErrorLog.Printf("Error retrieving task %s: %v", task.ID, err)
			continue
		}

		resultsMu.Lock()
		results[task.ID] = completed
		resultsMu.Unlock()

		duration := completed.CompletedAt.Sub(completed.StartedAt)
		statusIcon := "✓"
		if completed.Status == ollama.StatusFailed {
			statusIcon = "✗"
		}

		fmt.Printf("%s %s: %s (%.2fs)\n",
			statusIcon, task.ID, completed.Status.String(),
			duration.Seconds())
	}

	// Print comprehensive metrics
	metrics := dispatcher.GetMetrics()
	fmt.Println("\n=== Dispatcher Metrics ===")
	fmt.Printf("Total Tasks: %d\n", metrics.TotalTasks)
	fmt.Printf("Completed: %d\n", metrics.CompletedTasks)
	fmt.Printf("Failed: %d\n", metrics.FailedTasks)
	fmt.Printf("Cancelled: %d\n", metrics.CancelledTasks)
	fmt.Printf("Success Rate: %.1f%%\n",
		float64(metrics.CompletedTasks)/float64(metrics.TotalTasks)*100)
	fmt.Printf("Worker Count: %d\n", metrics.WorkerCount)

	// Check for errors
	executionErrors := dispatcher.GetErrors()
	if len(executionErrors) > 0 {
		fmt.Printf("\n=== Errors ===\n")
		for _, execErr := range executionErrors {
			fmt.Printf("[Worker %d] Task %s: %v\n", execErr.WorkerID, execErr.TaskID, execErr.Error)
		}
	}

	// Graceful shutdown
	fmt.Println("\nShutting down dispatcher...")
	if err := dispatcher.Shutdown(10 * time.Second); err != nil {
		log.ErrorLog.Printf("Error during shutdown: %v", err)
	}

	fmt.Println("Done!")
}
