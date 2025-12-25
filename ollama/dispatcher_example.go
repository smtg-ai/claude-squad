package ollama

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// ExampleBasicDispatcher demonstrates basic dispatcher usage
func ExampleBasicDispatcher() error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Define the agent function that executes tasks
	agentFunc := func(ctx context.Context, task *Task) error {
		// Simulate agent work
		sleepDuration := time.Duration(rand.Intn(1000)) * time.Millisecond
		select {
		case <-time.After(sleepDuration):
			task.Result = fmt.Sprintf("Task %s processed", task.ID)
			return nil
		case <-ctx.Done():
			return fmt.Errorf("task execution cancelled")
		}
	}

	// Create dispatcher with 5 workers
	dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 5)
	if err != nil {
		return fmt.Errorf("failed to create dispatcher: %w", err)
	}

	// Start the dispatcher
	if err := dispatcher.Start(); err != nil {
		return fmt.Errorf("failed to start dispatcher: %w", err)
	}
	defer dispatcher.Shutdown(5 * time.Second)

	// Submit tasks
	for i := 0; i < 20; i++ {
		task := &Task{
			ID:       fmt.Sprintf("task-%d", i),
			Priority: PriorityNormal,
			Payload:  map[string]interface{}{"index": i},
		}
		if err := dispatcher.SubmitTask(task); err != nil {
			return fmt.Errorf("failed to submit task: %w", err)
		}
	}

	// Wait for all tasks to complete
	if err := dispatcher.Wait(); err != nil {
		return fmt.Errorf("dispatcher wait failed: %w", err)
	}

	// Get and display metrics
	metrics := dispatcher.GetMetrics()
	fmt.Printf("Dispatcher Metrics:\n")
	fmt.Printf("  Total Tasks: %d\n", metrics.TotalTasks)
	fmt.Printf("  Completed: %d\n", metrics.CompletedTasks)
	fmt.Printf("  Failed: %d\n", metrics.FailedTasks)
	fmt.Printf("  Cancelled: %d\n", metrics.CancelledTasks)

	return nil
}

// ExampleDispatcherWithProgress demonstrates progress tracking
func ExampleDispatcherWithProgress() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	agentFunc := func(ctx context.Context, task *Task) error {
		sleepDuration := time.Duration(rand.Intn(2000)) * time.Millisecond
		select {
		case <-time.After(sleepDuration):
			task.Result = fmt.Sprintf("Processed: %v", task.Payload)
			return nil
		case <-ctx.Done():
			return fmt.Errorf("task cancelled")
		}
	}

	dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 3)
	if err != nil {
		return fmt.Errorf("failed to create dispatcher: %w", err)
	}

	// Set progress callback
	dispatcher.SetProgressCallback(func(taskID string, status TaskStatus, progress int, message string) {
		fmt.Printf("[%s] Task %s: %s (%d%%) - %s\n",
			time.Now().Format("15:04:05"),
			taskID,
			status.String(),
			progress,
			message)
	})

	if err := dispatcher.Start(); err != nil {
		return fmt.Errorf("failed to start dispatcher: %w", err)
	}
	defer dispatcher.Shutdown(5 * time.Second)

	// Submit batch of tasks with different priorities
	tasks := []*Task{
		{
			ID:       "high-priority-1",
			Priority: PriorityHigh,
			Payload:  "important task",
		},
		{
			ID:       "normal-priority-1",
			Priority: PriorityNormal,
			Payload:  "regular task",
		},
		{
			ID:       "low-priority-1",
			Priority: PriorityLow,
			Payload:  "background task",
		},
	}

	if err := dispatcher.SubmitBatch(tasks); err != nil {
		return fmt.Errorf("failed to submit batch: %w", err)
	}

	if err := dispatcher.Wait(); err != nil {
		return fmt.Errorf("dispatcher wait failed: %w", err)
	}

	return nil
}

// ExampleDispatcherWithErrors demonstrates error handling
func ExampleDispatcherWithErrors() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Agent function that may fail based on task payload
	agentFunc := func(ctx context.Context, task *Task) error {
		sleepDuration := time.Duration(rand.Intn(500)) * time.Millisecond
		select {
		case <-time.After(sleepDuration):
			// Simulate random failures
			if rand.Float64() < 0.3 {
				return fmt.Errorf("simulated agent error for task %s", task.ID)
			}
			task.Result = fmt.Sprintf("Task %s succeeded", task.ID)
			return nil
		case <-ctx.Done():
			return fmt.Errorf("task execution cancelled")
		}
	}

	dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 4)
	if err != nil {
		return fmt.Errorf("failed to create dispatcher: %w", err)
	}

	if err := dispatcher.Start(); err != nil {
		return fmt.Errorf("failed to start dispatcher: %w", err)
	}
	defer dispatcher.Shutdown(5 * time.Second)

	// Submit tasks
	for i := 0; i < 10; i++ {
		task := &Task{
			ID:       fmt.Sprintf("risky-task-%d", i),
			Priority: PriorityNormal,
		}
		if err := dispatcher.SubmitTask(task); err != nil {
			return fmt.Errorf("failed to submit task: %w", err)
		}
	}

	if err := dispatcher.Wait(); err != nil {
		return fmt.Errorf("dispatcher wait failed: %w", err)
	}

	// Collect and report all errors
	errors := dispatcher.GetErrors()
	if len(errors) > 0 {
		fmt.Printf("Task execution errors (%d):\n", len(errors))
		for _, te := range errors {
			fmt.Printf("  Task %s (Worker %d): %v\n", te.TaskID, te.WorkerID, te.Error)
		}
	}

	metrics := dispatcher.GetMetrics()
	fmt.Printf("\nFinal Metrics:\n")
	fmt.Printf("  Total: %d, Completed: %d, Failed: %d, Cancelled: %d\n",
		metrics.TotalTasks,
		metrics.CompletedTasks,
		metrics.FailedTasks,
		metrics.CancelledTasks)

	return nil
}

// ExampleDispatcherWithCancellation demonstrates task cancellation
func ExampleDispatcherWithCancellation() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	agentFunc := func(ctx context.Context, task *Task) error {
		// Simulate long-running task
		for i := 0; i < 10; i++ {
			select {
			case <-time.After(500 * time.Millisecond):
				// Continue processing
			case <-ctx.Done():
				return fmt.Errorf("task cancelled by context")
			}
		}
		task.Result = "Completed"
		return nil
	}

	dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 2)
	if err != nil {
		return fmt.Errorf("failed to create dispatcher: %w", err)
	}

	dispatcher.SetProgressCallback(func(taskID string, status TaskStatus, progress int, message string) {
		fmt.Printf("[Progress] %s: %s - %s\n", taskID, status.String(), message)
	})

	if err := dispatcher.Start(); err != nil {
		return fmt.Errorf("failed to start dispatcher: %w", err)
	}
	defer dispatcher.Shutdown(5 * time.Second)

	// Submit tasks
	var taskIDs []string
	for i := 0; i < 5; i++ {
		taskID := fmt.Sprintf("long-task-%d", i)
		task := &Task{
			ID:       taskID,
			Priority: PriorityNormal,
		}
		taskIDs = append(taskIDs, taskID)
		if err := dispatcher.SubmitTask(task); err != nil {
			return fmt.Errorf("failed to submit task: %w", err)
		}
	}

	// Cancel some tasks after a short delay
	go func() {
		time.Sleep(1 * time.Second)
		for _, taskID := range taskIDs[:2] {
			if err := dispatcher.CancelTask(taskID); err != nil {
				fmt.Printf("Cancellation error: %v\n", err)
			}
		}
	}()

	if err := dispatcher.Wait(); err != nil {
		return fmt.Errorf("dispatcher wait failed: %w", err)
	}

	metrics := dispatcher.GetMetrics()
	fmt.Printf("Final Status - Completed: %d, Failed: %d, Cancelled: %d\n",
		metrics.CompletedTasks,
		metrics.FailedTasks,
		metrics.CancelledTasks)

	return nil
}

// ExampleDispatcherWithContextCancellation demonstrates context-based cancellation
func ExampleDispatcherWithContextCancellation() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agentFunc := func(ctx context.Context, task *Task) error {
		// Simulate work that respects context cancellation
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for i := 0; i < 20; i++ {
			select {
			case <-ticker.C:
				// Simulate work
			case <-ctx.Done():
				return fmt.Errorf("context cancelled")
			}
		}
		task.Result = "Completed"
		return nil
	}

	dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 3)
	if err != nil {
		return fmt.Errorf("failed to create dispatcher: %w", err)
	}

	if err := dispatcher.Start(); err != nil {
		return fmt.Errorf("failed to start dispatcher: %w", err)
	}

	// Submit tasks
	for i := 0; i < 10; i++ {
		task := &Task{
			ID:       fmt.Sprintf("context-task-%d", i),
			Priority: PriorityNormal,
		}
		if err := dispatcher.SubmitTask(task); err != nil {
			return fmt.Errorf("failed to submit task: %w", err)
		}
	}

	// Cancel context after a delay
	time.AfterFunc(2*time.Second, func() {
		fmt.Println("Cancelling dispatcher context...")
		cancel()
	})

	// Wait with timeout
	done := make(chan error, 1)
	go func() {
		done <- dispatcher.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-time.After(10 * time.Second):
		return fmt.Errorf("wait timeout exceeded")
	}

	metrics := dispatcher.GetMetrics()
	fmt.Printf("Final Status - Completed: %d, Failed: %d, Cancelled: %d\n",
		metrics.CompletedTasks,
		metrics.FailedTasks,
		metrics.CancelledTasks)

	return nil
}

// ExampleDispatcherWithPriorities demonstrates priority-based task handling
func ExampleDispatcherWithPriorities() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	startTime := time.Now()

	agentFunc := func(ctx context.Context, task *Task) error {
		// Simulate agent work
		sleepDuration := time.Duration(rand.Intn(500)) * time.Millisecond
		select {
		case <-time.After(sleepDuration):
			task.Result = fmt.Sprintf("Executed after %v", time.Since(startTime))
			return nil
		case <-ctx.Done():
			return fmt.Errorf("task cancelled")
		}
	}

	dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 2)
	if err != nil {
		return fmt.Errorf("failed to create dispatcher: %w", err)
	}

	dispatcher.SetProgressCallback(func(taskID string, status TaskStatus, progress int, message string) {
		if status == StatusCompleted {
			task, _ := dispatcher.GetTask(taskID)
			fmt.Printf("[%s] Priority: %d, Result: %v\n",
				taskID,
				task.Priority,
				task.Result)
		}
	})

	if err := dispatcher.Start(); err != nil {
		return fmt.Errorf("failed to start dispatcher: %w", err)
	}
	defer dispatcher.Shutdown(5 * time.Second)

	// Submit tasks with different priorities
	priorities := []int{PriorityLow, PriorityNormal, PriorityHigh}
	for i, priority := range priorities {
		for j := 0; j < 3; j++ {
			task := &Task{
				ID:       fmt.Sprintf("priority-%d-task-%d", priority, j),
				Priority: priority,
				Payload:  map[string]interface{}{"index": i*3 + j},
			}
			if err := dispatcher.SubmitTask(task); err != nil {
				return fmt.Errorf("failed to submit task: %w", err)
			}
		}
	}

	if err := dispatcher.Wait(); err != nil {
		return fmt.Errorf("dispatcher wait failed: %w", err)
	}

	return nil
}

// ExampleDispatcherWithMaxWorkers demonstrates using maximum worker count
func ExampleDispatcherWithMaxWorkers() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	agentFunc := func(ctx context.Context, task *Task) error {
		// Simulate CPU-bound work
		time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
		task.Result = "Completed"
		return nil
	}

	// Create dispatcher with maximum workers (10)
	dispatcher, err := NewTaskDispatcher(ctx, agentFunc, MaxWorkers)
	if err != nil {
		return fmt.Errorf("failed to create dispatcher: %w", err)
	}

	if err := dispatcher.Start(); err != nil {
		return fmt.Errorf("failed to start dispatcher: %w", err)
	}
	defer dispatcher.Shutdown(5 * time.Second)

	// Submit many tasks
	for i := 0; i < 50; i++ {
		task := &Task{
			ID:       fmt.Sprintf("max-workers-task-%d", i),
			Priority: PriorityNormal,
		}
		if err := dispatcher.SubmitTask(task); err != nil {
			return fmt.Errorf("failed to submit task: %w", err)
		}
	}

	startTime := time.Now()
	if err := dispatcher.Wait(); err != nil {
		return fmt.Errorf("dispatcher wait failed: %w", err)
	}
	elapsed := time.Since(startTime)

	metrics := dispatcher.GetMetrics()
	fmt.Printf("Processed %d tasks in %v using %d workers\n",
		metrics.CompletedTasks,
		elapsed,
		metrics.WorkerCount)

	return nil
}
