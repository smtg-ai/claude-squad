package orchestrator

import (
	"context"
	"fmt"
	"time"
)

// ExampleExecutor is a simple example implementation of AgentExecutor
type ExampleExecutor struct{}

// Execute implements the AgentExecutor interface
func (e *ExampleExecutor) Execute(ctx context.Context, task *Task) (*string, error) {
	// Simulate some work
	select {
	case <-time.After(2 * time.Second):
		result := fmt.Sprintf("Completed task %s: %s", task.ID, task.Description)
		return &result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ExampleUsage demonstrates how to use the orchestrator
func ExampleUsage() error {
	ctx := context.Background()

	// Create agent pool
	pool, err := NewAgentPool("http://localhost:5000", &ExampleExecutor{})
	if err != nil {
		return fmt.Errorf("failed to create agent pool: %w", err)
	}

	// Start the pool
	if err := pool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start pool: %w", err)
	}
	defer pool.Stop()

	// Submit a simple task
	task1, err := pool.SubmitTask(&Task{
		Description: "Analyze codebase structure",
		Priority:    10,
	})
	if err != nil {
		return fmt.Errorf("failed to submit task1: %w", err)
	}

	// Submit a task with dependency
	task2, err := pool.SubmitTask(&Task{
		Description:  "Refactor based on analysis",
		Priority:     8,
		Dependencies: []string{task1},
	})
	if err != nil {
		return fmt.Errorf("failed to submit task2: %w", err)
	}

	// Submit multiple parallel tasks
	taskIDs := []string{}
	for i := 0; i < 5; i++ {
		taskID, err := pool.SubmitTask(&Task{
			Description: fmt.Sprintf("Parallel task %d", i+1),
			Priority:    5,
		})
		if err != nil {
			return fmt.Errorf("failed to submit parallel task %d: %w", i+1, err)
		}
		taskIDs = append(taskIDs, taskID)
	}

	// Submit a final task that depends on all parallel tasks
	_, err = pool.SubmitTask(&Task{
		Description:  "Aggregate results from parallel tasks",
		Priority:     9,
		Dependencies: taskIDs,
	})
	if err != nil {
		return fmt.Errorf("failed to submit aggregation task: %w", err)
	}

	// Wait for all tasks to complete
	fmt.Println("Waiting for all tasks to complete...")
	if err := pool.WaitForCompletion(ctx); err != nil {
		return fmt.Errorf("error waiting for completion: %w", err)
	}

	// Get final analytics
	analytics, err := pool.GetAnalytics()
	if err != nil {
		return fmt.Errorf("failed to get final analytics: %w", err)
	}

	fmt.Printf("\nFinal Analytics:\n")
	fmt.Printf("  Total Tasks: %d\n", analytics.TotalTasks)
	fmt.Printf("  Completed: %d\n", analytics.StatusCounts[StatusCompleted])
	fmt.Printf("  Failed: %d\n", analytics.StatusCounts[StatusFailed])

	return nil
}

// AdvancedExample demonstrates advanced features
func AdvancedExample() error {
	ctx := context.Background()

	pool, err := NewAgentPool("http://localhost:5000", &ExampleExecutor{})
	if err != nil {
		return err
	}

	if err := pool.Start(ctx); err != nil {
		return err
	}
	defer pool.Stop()

	// Create a complex dependency graph
	// Task structure:
	//       ┌─── T2 ───┐
	//   T1 ─┤           ├─── T5
	//       └─── T3 ─── T4
	//
	// T6 is independent

	t1, _ := pool.SubmitTask(&Task{
		Description: "Read configuration files",
		Priority:    10,
		Metadata:    map[string]string{"type": "io"},
	})

	t2, _ := pool.SubmitTask(&Task{
		Description:  "Parse config schema",
		Priority:     9,
		Dependencies: []string{t1},
		Metadata:     map[string]string{"type": "parse"},
	})

	t3, _ := pool.SubmitTask(&Task{
		Description:  "Validate config values",
		Priority:     9,
		Dependencies: []string{t1},
		Metadata:     map[string]string{"type": "validate"},
	})

	t4, _ := pool.SubmitTask(&Task{
		Description:  "Generate config documentation",
		Priority:     7,
		Dependencies: []string{t3},
		Metadata:     map[string]string{"type": "generate"},
	})

	t5, _ := pool.SubmitTask(&Task{
		Description:  "Create final report",
		Priority:     8,
		Dependencies: []string{t2, t4},
		Metadata:     map[string]string{"type": "report"},
	})

	// Independent task that can run immediately
	t6, _ := pool.SubmitTask(&Task{
		Description: "Run background health checks",
		Priority:    3,
		Metadata:    map[string]string{"type": "monitor"},
	})

	// Monitor progress
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	done := make(chan bool)
	go func() {
		pool.WaitForCompletion(ctx)
		done <- true
	}()

	for {
		select {
		case <-done:
			fmt.Println("\nAll tasks completed!")

			// Show dependency chain for final task
			chain, _ := pool.GetTaskChain(t5)
			fmt.Printf("\nDependency chain for task %s:\n", t5)
			for _, dep := range chain {
				fmt.Printf("  - %s [%s]: %s\n", dep.ID, dep.Status, dep.Description)
			}

			return nil

		case <-ticker.C:
			analytics, _ := pool.GetAnalytics()
			fmt.Printf("[%s] Running: %d, Pending: %d, Completed: %d, Failed: %d\n",
				time.Now().Format("15:04:05"),
				analytics.RunningCount,
				analytics.StatusCounts[StatusPending],
				analytics.StatusCounts[StatusCompleted],
				analytics.StatusCounts[StatusFailed])

			// Show which tasks are running
			runningTasks, _ := pool.client.GetRunningTasks()
			if len(runningTasks) > 0 {
				fmt.Printf("  Currently executing: %v\n", runningTasks)
			}
		}
	}
}
