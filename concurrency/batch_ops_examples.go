package concurrency

import (
	"context"
	"fmt"
	"log"
	"time"

	"claude-squad/session"
)

// ExampleBatchKillInstances demonstrates killing multiple instances in parallel
func ExampleBatchKillInstances() {
	// Create instances (in a real scenario, these would be running instances)
	instances := []*session.Instance{
		{Title: "instance-1"},
		{Title: "instance-2"},
		{Title: "instance-3"},
	}

	// Create batch executor with max 5 concurrent operations
	executor := NewBatchExecutor(5)
	executor.SetTimeout(30 * time.Second)

	// Create kill operation
	killOp := NewBatchKillOperation()

	// Create progress tracker
	tracker := NewProgressTracker(len(instances))
	tracker.OnProgress(func(result *OperationResult, completed, total int) {
		status := "SUCCESS"
		if result.Error != nil {
			status = "FAILED"
		}
		fmt.Printf("[%d/%d] %s: %s on instance '%s' (took %v)\n",
			completed, total, status, result.Operation.Name(),
			result.Instance.Title, result.Duration())
	})

	// Execute kill operation
	ctx := context.Background()
	result := executor.Execute(ctx, instances, killOp, tracker)

	fmt.Printf("\nBatch kill completed: %d succeeded, %d failed\n",
		len(result.Successes), len(result.Failures))

	if !result.AllSucceeded() {
		fmt.Printf("Errors occurred: %v\n", result.Error())
	}
}

// ExampleBatchPauseWithRollback demonstrates pausing with automatic rollback on failure
func ExampleBatchPauseWithRollback() {
	instances := []*session.Instance{
		{Title: "instance-1"},
		{Title: "instance-2"},
		{Title: "instance-3"},
	}

	executor := NewBatchExecutor(3)
	pauseOp := NewBatchPauseOperation()

	tracker := NewProgressTracker(len(instances))
	tracker.OnProgress(func(result *OperationResult, completed, total int) {
		fmt.Printf("Progress: %d/%d - Instance: %s\n",
			completed, total, result.Instance.Title)
	})

	ctx := context.Background()
	result, err := executor.ExecuteWithRollback(ctx, instances, pauseOp, tracker)

	if err != nil {
		fmt.Printf("Pause operation failed and was rolled back: %v\n", err)
		fmt.Printf("Rolled back instances: %d\n", len(result.Successes))
	} else {
		fmt.Printf("All instances paused successfully\n")
	}
}

// ExampleTransactionManager demonstrates transactional batch operations
func ExampleTransactionManager() {
	instances := []*session.Instance{
		{Title: "instance-1"},
		{Title: "instance-2"},
	}

	// Create transaction manager with concurrency of 2
	tm := NewTransactionManager(2)
	tm.SetTimeout(1 * time.Minute)

	// Create pause operation
	pauseOp := NewBatchPauseOperation()

	tracker := NewProgressTracker(len(instances))
	tracker.OnProgress(func(result *OperationResult, completed, total int) {
		log.Printf("Transaction progress: %d/%d", completed, total)
	})

	ctx := context.Background()
	err := tm.Execute(ctx, instances, pauseOp, tracker)

	if err != nil {
		fmt.Printf("Transaction failed (all operations rolled back): %v\n", err)
	} else {
		fmt.Printf("Transaction completed successfully\n")
	}
}

// ExampleOperationChain demonstrates chaining multiple operations
func ExampleOperationChain() {
	instances := []*session.Instance{
		{Title: "instance-1"},
		{Title: "instance-2"},
	}

	// Create operation chain that stops on first failure
	chain := NewOperationChain(true)

	// Add operations in sequence: pause, then send prompt, then resume
	chain.
		Add(NewBatchPauseOperation()).
		Add(NewBatchPromptOperation("Fix all bugs")).
		Add(NewBatchResumeOperation())

	executor := NewBatchExecutor(2)
	tracker := NewProgressTracker(len(instances) * 3) // 3 operations per instance

	ctx := context.Background()
	results := chain.Execute(ctx, instances, executor, tracker)

	fmt.Printf("Executed %d operations in chain\n", len(results))
	for i, result := range results {
		fmt.Printf("Operation %d: %d succeeded, %d failed\n",
			i+1, len(result.Successes), len(result.Failures))
	}
}

// ExampleProgressTracking demonstrates detailed progress tracking
func ExampleBatchProgressTracking() {
	instances := make([]*session.Instance, 10)
	for i := 0; i < 10; i++ {
		instances[i] = &session.Instance{Title: fmt.Sprintf("instance-%d", i+1)}
	}

	executor := NewBatchExecutor(3) // Max 3 concurrent operations
	killOp := NewBatchKillOperation()

	tracker := NewProgressTracker(len(instances))

	// Multiple progress callbacks can be registered
	tracker.OnProgress(func(result *OperationResult, completed, total int) {
		percentage := float64(completed) / float64(total) * 100
		fmt.Printf("Overall progress: %.1f%% (%d/%d)\n", percentage, completed, total)
	})

	tracker.OnProgress(func(result *OperationResult, completed, total int) {
		if result.Error != nil {
			log.Printf("ERROR on %s: %v", result.Instance.Title, result.Error)
		}
	})

	ctx := context.Background()
	result := executor.Execute(ctx, instances, killOp, tracker)

	fmt.Printf("Final result: %.1f%% success rate\n", result.SuccessRate())
}

// ExamplePartialFailureHandling demonstrates handling partial failures
func ExamplePartialFailureHandling() {
	instances := []*session.Instance{
		{Title: "instance-1"},
		{Title: "instance-2"},
		{Title: "instance-3"},
	}

	executor := NewBatchExecutor(5)
	pauseOp := NewBatchPauseOperation()

	ctx := context.Background()
	result := executor.Execute(ctx, instances, pauseOp, nil)

	// Check for partial failures
	if !result.AllSucceeded() && len(result.Successes) > 0 {
		fmt.Printf("Partial failure: %d succeeded, %d failed\n",
			len(result.Successes), len(result.Failures))

		// Handle successful instances
		fmt.Println("\nSuccessful operations:")
		for _, success := range result.Successes {
			fmt.Printf("  - %s: completed in %v\n",
				success.Instance.Title, success.Duration())
		}

		// Handle failed instances
		fmt.Println("\nFailed operations:")
		for _, failure := range result.Failures {
			fmt.Printf("  - %s: %v\n",
				failure.Instance.Title, failure.Error)
		}

		// Retry only the failed instances
		failedInstances := make([]*session.Instance, len(result.Failures))
		for i, failure := range result.Failures {
			failedInstances[i] = failure.Instance
		}

		fmt.Println("\nRetrying failed instances...")
		retryResult := executor.Execute(ctx, failedInstances, pauseOp, nil)
		fmt.Printf("Retry result: %d succeeded, %d failed\n",
			len(retryResult.Successes), len(retryResult.Failures))
	}
}

// ExampleContextCancellation demonstrates canceling batch operations
func ExampleContextCancellation() {
	instances := make([]*session.Instance, 20)
	for i := 0; i < 20; i++ {
		instances[i] = &session.Instance{Title: fmt.Sprintf("instance-%d", i+1)}
	}

	executor := NewBatchExecutor(3)
	killOp := NewBatchKillOperation()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tracker := NewProgressTracker(len(instances))
	tracker.OnProgress(func(result *OperationResult, completed, total int) {
		fmt.Printf("Progress: %d/%d\n", completed, total)
	})

	result := executor.Execute(ctx, instances, killOp, tracker)

	// Check if context was cancelled
	cancelledCount := 0
	for _, failure := range result.Failures {
		if failure.Error == context.DeadlineExceeded {
			cancelledCount++
		}
	}

	fmt.Printf("Completed: %d, Cancelled: %d\n",
		len(result.Successes), cancelledCount)
}

// ExampleCompositeOperation demonstrates combining multiple operations
func ExampleCompositeOperation() {
	instances := []*session.Instance{
		{Title: "instance-1"},
	}

	// Create a composite operation that pauses and then sends a prompt
	composite := NewCompositeOperation(
		"PauseAndPrompt",
		NewBatchPauseOperation(),
		NewBatchPromptOperation("Review changes"),
	)

	executor := NewBatchExecutor(1)
	tracker := NewProgressTracker(len(instances))

	ctx := context.Background()
	result := executor.Execute(ctx, instances, composite, tracker)

	if result.AllSucceeded() {
		fmt.Println("Composite operation completed successfully")
	} else {
		fmt.Printf("Composite operation failed: %v\n", result.Error())
	}
}

// ExampleConditionalOperation demonstrates conditional execution
func ExampleConditionalOperation() {
	instances := []*session.Instance{
		{Title: "instance-1", Status: session.Running},
		{Title: "instance-2", Status: session.Paused},
		{Title: "instance-3", Status: session.Running},
	}

	// Only pause instances that are currently running
	conditionalPause := NewConditionalOperation(
		NewBatchPauseOperation(),
		func(i *session.Instance) bool {
			return i.Status == session.Running
		},
	)

	executor := NewBatchExecutor(3)
	ctx := context.Background()
	result := executor.Execute(ctx, instances, conditionalPause, nil)

	fmt.Printf("Paused %d running instances\n", len(result.Successes))
}

// ExampleRetryOperation demonstrates retry logic
func ExampleRetryOperation() {
	instances := []*session.Instance{
		{Title: "instance-1"},
	}

	// Create a pause operation with retry logic
	// Will retry up to 3 times with 1 second delay between retries
	retryablePause := NewRetryOperation(
		NewBatchPauseOperation(),
		3,             // max retries
		1*time.Second, // delay between retries
	)

	executor := NewBatchExecutor(1)
	ctx := context.Background()
	result := executor.Execute(ctx, instances, retryablePause, nil)

	if result.AllSucceeded() {
		fmt.Println("Operation succeeded (possibly after retries)")
	} else {
		fmt.Printf("Operation failed after retries: %v\n", result.Error())
	}
}

// ExampleComplexWorkflow demonstrates a complex workflow
func ExampleComplexWorkflow() {
	// Simulate a complex workflow: pause all instances, send prompts, monitor, then resume
	instances := []*session.Instance{
		{Title: "instance-1"},
		{Title: "instance-2"},
		{Title: "instance-3"},
	}

	executor := NewBatchExecutor(5)
	ctx := context.Background()

	// Step 1: Pause all instances
	fmt.Println("Step 1: Pausing all instances...")
	pauseResult := executor.Execute(ctx, instances, NewBatchPauseOperation(), nil)
	if !pauseResult.AllSucceeded() {
		fmt.Printf("Failed to pause some instances: %v\n", pauseResult.Error())
		return
	}
	fmt.Printf("Paused %d instances\n", len(pauseResult.Successes))

	// Step 2: Send prompts to all instances
	fmt.Println("\nStep 2: Sending prompts to instances...")
	promptResult := executor.Execute(ctx, instances,
		NewBatchPromptOperation("Analyze the codebase"), nil)
	if !promptResult.AllSucceeded() {
		fmt.Printf("Failed to send prompts: %v\n", promptResult.Error())
	}

	// Step 3: Resume instances (only those that were successfully paused)
	fmt.Println("\nStep 3: Resuming instances...")
	successfulInstances := make([]*session.Instance, len(pauseResult.Successes))
	for i, success := range pauseResult.Successes {
		successfulInstances[i] = success.Instance
	}

	resumeResult := executor.Execute(ctx, successfulInstances, NewBatchResumeOperation(), nil)
	fmt.Printf("Resumed %d instances\n", len(resumeResult.Successes))

	// Summary
	fmt.Println("\n=== Workflow Summary ===")
	fmt.Printf("Total instances: %d\n", len(instances))
	fmt.Printf("Successfully paused: %d\n", len(pauseResult.Successes))
	fmt.Printf("Successfully prompted: %d\n", len(promptResult.Successes))
	fmt.Printf("Successfully resumed: %d\n", len(resumeResult.Successes))
}

// ExampleCustomOperation demonstrates creating a custom operation
type CustomHealthCheckOperation struct {
	timeout time.Duration
}

func (op *CustomHealthCheckOperation) Name() string {
	return "HealthCheck"
}

func (op *CustomHealthCheckOperation) Validate(instance *session.Instance) error {
	if !instance.Started() {
		return fmt.Errorf("instance not started")
	}
	return nil
}

func (op *CustomHealthCheckOperation) Execute(ctx context.Context, instance *session.Instance) error {
	// Implement custom health check logic
	// For example, check if tmux session is alive
	if !instance.TmuxAlive() {
		return fmt.Errorf("tmux session is not alive")
	}
	return nil
}

func (op *CustomHealthCheckOperation) Rollback(ctx context.Context, instance *session.Instance) error {
	// Health checks typically don't need rollback
	return nil
}

func ExampleCustomHealthCheck() {
	instances := []*session.Instance{
		{Title: "instance-1"},
		{Title: "instance-2"},
	}

	healthCheck := &CustomHealthCheckOperation{
		timeout: 5 * time.Second,
	}

	executor := NewBatchExecutor(5)
	tracker := NewProgressTracker(len(instances))

	ctx := context.Background()
	result := executor.Execute(ctx, instances, healthCheck, tracker)

	healthyCount := len(result.Successes)
	unhealthyCount := len(result.Failures)

	fmt.Printf("Health check complete: %d healthy, %d unhealthy\n",
		healthyCount, unhealthyCount)

	if unhealthyCount > 0 {
		fmt.Println("\nUnhealthy instances:")
		for _, failure := range result.Failures {
			fmt.Printf("  - %s: %v\n", failure.Instance.Title, failure.Error)
		}
	}
}
