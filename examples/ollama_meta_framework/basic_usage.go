package main

import (
	"claude-squad/log"
	"claude-squad/ollama"
	"context"
	"fmt"
	"time"
)

// ExampleBasicUsage demonstrates simple single-agent execution using the meta framework.
// This example shows the fundamental workflow:
// 1. Create a TaskDispatcher with an agent function
// 2. Submit a task
// 3. Monitor execution with a progress callback
// 4. Wait for completion and check results
//
// Run with: go run basic_usage.go
func main() {
	// Initialize logging
	log.Initialize(false)
	defer log.Close()

	ctx := context.Background()

	// Define a simple agent function that simulates a work task
	// In a real scenario, this would call Ollama models or other agents
	agentFunc := func(ctx context.Context, task *ollama.Task) error {
		// Simulate some work
		fmt.Printf("[Agent] Processing task: %s\n", task.ID)

		// Your agent logic here - this could be:
		// - Calling Ollama API
		// - Running Claude Code
		// - Using Aider
		// - Any other agent execution

		// Simulate processing time
		select {
		case <-time.After(2 * time.Second):
			// Store result in task
			task.Result = map[string]interface{}{
				"status":    "completed",
				"message":   "Successfully processed task",
				"timestamp": time.Now(),
			}
			fmt.Printf("[Agent] Task %s completed\n", task.ID)
			return nil

		case <-ctx.Done():
			fmt.Printf("[Agent] Task %s cancelled\n", task.ID)
			return ctx.Err()
		}
	}

	// Create a task dispatcher with single worker
	dispatcher, err := ollama.NewTaskDispatcher(ctx, agentFunc, 1)
	if err != nil {
		log.ErrorLog.Printf("Failed to create dispatcher: %v", err)
		return
	}

	// Set up progress reporting callback
	dispatcher.SetProgressCallback(func(taskID string, status ollama.TaskStatus, progress int, message string) {
		fmt.Printf("[Progress] Task: %s | Status: %s | Progress: %d%% | Message: %s\n",
			taskID, status.String(), progress, message)
	})

	// Start the dispatcher's worker pool
	if err := dispatcher.Start(); err != nil {
		log.ErrorLog.Printf("Failed to start dispatcher: %v", err)
		return
	}

	// Create and submit a simple task
	task := &ollama.Task{
		ID:       "task-001",
		Priority: ollama.PriorityNormal,
		Payload: map[string]interface{}{
			"prompt": "Write a hello world program",
			"model":  "llama2",
		},
	}

	fmt.Println("\n=== Basic Usage Example ===")
	fmt.Println("Submitting single task to dispatcher...\n")

	if err := dispatcher.SubmitTask(task); err != nil {
		log.ErrorLog.Printf("Failed to submit task: %v", err)
		return
	}

	// Wait for the task to complete
	// In a real application, you could poll task status periodically
	time.Sleep(5 * time.Second)

	// Check the completed task
	completedTask, err := dispatcher.GetTask(task.ID)
	if err != nil {
		log.ErrorLog.Printf("Failed to get task: %v", err)
		return
	}

	// Display results
	fmt.Println("\n=== Task Results ===")
	fmt.Printf("Task ID: %s\n", completedTask.ID)
	fmt.Printf("Status: %s\n", completedTask.Status.String())
	fmt.Printf("Duration: %v\n", completedTask.CompletedAt.Sub(completedTask.StartedAt))
	fmt.Printf("Result: %v\n", completedTask.Result)

	if completedTask.Error != nil {
		fmt.Printf("Error: %v\n", completedTask.Error)
	}

	// Get dispatcher metrics
	metrics := dispatcher.GetMetrics()
	fmt.Println("\n=== Dispatcher Metrics ===")
	fmt.Printf("Total Tasks: %d\n", metrics.TotalTasks)
	fmt.Printf("Completed: %d\n", metrics.CompletedTasks)
	fmt.Printf("Failed: %d\n", metrics.FailedTasks)
	fmt.Printf("Worker Count: %d\n", metrics.WorkerCount)

	// Gracefully shutdown the dispatcher
	fmt.Println("\nShutting down dispatcher...")
	if err := dispatcher.Shutdown(5 * time.Second); err != nil {
		log.ErrorLog.Printf("Error during shutdown: %v", err)
	}

	fmt.Println("Done!")
}
