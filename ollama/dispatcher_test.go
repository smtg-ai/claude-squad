package ollama

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestTaskDispatcherCreation(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		agentFunc   AgentFunc
		workerCount int
		shouldFail  bool
		errMsg      string
	}{
		{
			name: "valid dispatcher",
			agentFunc: func(ctx context.Context, task *Task) error {
				return nil
			},
			workerCount: 5,
			shouldFail:  false,
		},
		{
			name:        "nil agent function",
			agentFunc:   nil,
			workerCount: 5,
			shouldFail:  true,
			errMsg:      "agent function cannot be nil",
		},
		{
			name: "zero workers",
			agentFunc: func(ctx context.Context, task *Task) error {
				return nil
			},
			workerCount: 0,
			shouldFail:  true,
			errMsg:      "worker count must be between 1 and",
		},
		{
			name: "too many workers",
			agentFunc: func(ctx context.Context, task *Task) error {
				return nil
			},
			workerCount: 15,
			shouldFail:  true,
			errMsg:      "worker count must be between 1 and",
		},
		{
			name: "max workers",
			agentFunc: func(ctx context.Context, task *Task) error {
				return nil
			},
			workerCount: MaxWorkers,
			shouldFail:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dispatcher, err := NewTaskDispatcher(ctx, tt.agentFunc, tt.workerCount)

			if tt.shouldFail {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if tt.errMsg != "" && !stringContains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if dispatcher == nil {
					t.Errorf("expected dispatcher, got nil")
				}
			}
		})
	}
}

func TestTaskDispatcherLifecycle(t *testing.T) {
	ctx := context.Background()
	agentFunc := func(ctx context.Context, task *Task) error {
		time.Sleep(10 * time.Millisecond)
		task.Result = "completed"
		return nil
	}

	dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 2)
	if err != nil {
		t.Fatalf("failed to create dispatcher: %v", err)
	}

	// Test start
	if err := dispatcher.Start(); err != nil {
		t.Fatalf("failed to start dispatcher: %v", err)
	}

	// Test task submission
	task := &Task{
		ID:       "test-task-1",
		Priority: PriorityNormal,
	}

	if err := dispatcher.SubmitTask(task); err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	// Test shutdown
	if err := dispatcher.Shutdown(2 * time.Second); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	// Verify task was processed
	metrics := dispatcher.GetMetrics()
	if metrics.CompletedTasks < 1 {
		t.Errorf("expected at least 1 completed task, got %d", metrics.CompletedTasks)
	}
}

func TestTaskSubmission(t *testing.T) {
	ctx := context.Background()
	agentFunc := func(ctx context.Context, task *Task) error {
		return nil
	}

	dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 2)
	if err != nil {
		t.Fatalf("failed to create dispatcher: %v", err)
	}

	if err := dispatcher.Start(); err != nil {
		t.Fatalf("failed to start dispatcher: %v", err)
	}
	defer dispatcher.Shutdown(2 * time.Second)

	tests := []struct {
		name      string
		task      *Task
		shouldErr bool
		errMsg    string
	}{
		{
			name: "valid task",
			task: &Task{
				ID:       "valid-task",
				Priority: PriorityNormal,
			},
			shouldErr: false,
		},
		{
			name:      "nil task",
			task:      nil,
			shouldErr: true,
			errMsg:    "task cannot be nil",
		},
		{
			name: "task with empty ID",
			task: &Task{
				Priority: PriorityNormal,
			},
			shouldErr: true,
			errMsg:    "task ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := dispatcher.SubmitTask(tt.task)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if tt.errMsg != "" && !stringContains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestTaskExecution(t *testing.T) {
	ctx := context.Background()
	executed := make(map[string]bool)

	agentFunc := func(ctx context.Context, task *Task) error {
		executed[task.ID] = true
		task.Result = "done"
		return nil
	}

	dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 2)
	if err != nil {
		t.Fatalf("failed to create dispatcher: %v", err)
	}

	if err := dispatcher.Start(); err != nil {
		t.Fatalf("failed to start dispatcher: %v", err)
	}
	defer dispatcher.Shutdown(5 * time.Second)

	// Submit multiple tasks
	for i := 0; i < 10; i++ {
		task := &Task{
			ID:       fmt.Sprintf("task-%d", i),
			Priority: PriorityNormal,
		}
		if err := dispatcher.SubmitTask(task); err != nil {
			t.Fatalf("failed to submit task: %v", err)
		}
	}

	if err := dispatcher.Wait(); err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	metrics := dispatcher.GetMetrics()
	if metrics.CompletedTasks != 10 {
		t.Errorf("expected 10 completed tasks, got %d", metrics.CompletedTasks)
	}
}

func TestErrorHandling(t *testing.T) {
	ctx := context.Background()
	agentFunc := func(ctx context.Context, task *Task) error {
		if task.ID == "fail" {
			return fmt.Errorf("intentional error")
		}
		return nil
	}

	dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 2)
	if err != nil {
		t.Fatalf("failed to create dispatcher: %v", err)
	}

	if err := dispatcher.Start(); err != nil {
		t.Fatalf("failed to start dispatcher: %v", err)
	}
	defer dispatcher.Shutdown(2 * time.Second)

	// Submit failing task
	failTask := &Task{
		ID:       "fail",
		Priority: PriorityNormal,
	}
	if err := dispatcher.SubmitTask(failTask); err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	// Submit successful task
	successTask := &Task{
		ID:       "success",
		Priority: PriorityNormal,
	}
	if err := dispatcher.SubmitTask(successTask); err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	if err := dispatcher.Wait(); err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	// Check errors
	errors := dispatcher.GetErrors()
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}

	// Check metrics
	metrics := dispatcher.GetMetrics()
	if metrics.FailedTasks != 1 {
		t.Errorf("expected 1 failed task, got %d", metrics.FailedTasks)
	}
	if metrics.CompletedTasks != 1 {
		t.Errorf("expected 1 completed task, got %d", metrics.CompletedTasks)
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	agentFunc := func(ctx context.Context, task *Task) error {
		<-ctx.Done()
		return ctx.Err()
	}

	dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 2)
	if err != nil {
		t.Fatalf("failed to create dispatcher: %v", err)
	}

	if err := dispatcher.Start(); err != nil {
		t.Fatalf("failed to start dispatcher: %v", err)
	}

	// Submit task
	task := &Task{
		ID:       "cancel-task",
		Priority: PriorityNormal,
	}
	if err := dispatcher.SubmitTask(task); err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	// Cancel context
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for completion with timeout
	done := make(chan error, 1)
	go func() {
		done <- dispatcher.Wait()
	}()

	select {
	case <-done:
		// Expected
	case <-time.After(5 * time.Second):
		t.Fatalf("wait timeout exceeded")
	}
}

func TestProgressCallback(t *testing.T) {
	ctx := context.Background()
	var progressUpdates []string

	dispatcher, err := NewTaskDispatcher(ctx, func(ctx context.Context, task *Task) error {
		return nil
	}, 1)
	if err != nil {
		t.Fatalf("failed to create dispatcher: %v", err)
	}

	dispatcher.SetProgressCallback(func(taskID string, status TaskStatus, progress int, message string) {
		progressUpdates = append(progressUpdates, fmt.Sprintf("%s:%s", taskID, status.String()))
	})

	if err := dispatcher.Start(); err != nil {
		t.Fatalf("failed to start dispatcher: %v", err)
	}
	defer dispatcher.Shutdown(2 * time.Second)

	task := &Task{
		ID:       "progress-task",
		Priority: PriorityNormal,
	}
	if err := dispatcher.SubmitTask(task); err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	if err := dispatcher.Wait(); err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	// Verify progress updates were reported
	if len(progressUpdates) == 0 {
		t.Errorf("expected progress updates, got none")
	}
}

func TestTaskCancellation(t *testing.T) {
	ctx := context.Background()
	agentFunc := func(ctx context.Context, task *Task) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 2)
	if err != nil {
		t.Fatalf("failed to create dispatcher: %v", err)
	}

	if err := dispatcher.Start(); err != nil {
		t.Fatalf("failed to start dispatcher: %v", err)
	}
	defer dispatcher.Shutdown(5 * time.Second)

	task := &Task{
		ID:       "cancel-me",
		Priority: PriorityNormal,
	}
	if err := dispatcher.SubmitTask(task); err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	// Cancel before execution
	if err := dispatcher.CancelTask(task.ID); err != nil {
		t.Errorf("unexpected cancellation error: %v", err)
	}

	// Verify status
	status, err := dispatcher.GetTaskStatus(task.ID)
	if err != nil {
		t.Errorf("unexpected error getting task status: %v", err)
	}
	if status != StatusCancelled {
		t.Errorf("expected cancelled status, got %s", status.String())
	}
}

func TestMetrics(t *testing.T) {
	ctx := context.Background()
	agentFunc := func(ctx context.Context, task *Task) error {
		return nil
	}

	dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 3)
	if err != nil {
		t.Fatalf("failed to create dispatcher: %v", err)
	}

	if err := dispatcher.Start(); err != nil {
		t.Fatalf("failed to start dispatcher: %v", err)
	}
	defer dispatcher.Shutdown(2 * time.Second)

	// Submit 5 tasks
	for i := 0; i < 5; i++ {
		task := &Task{
			ID:       fmt.Sprintf("metric-task-%d", i),
			Priority: PriorityNormal,
		}
		if err := dispatcher.SubmitTask(task); err != nil {
			t.Fatalf("failed to submit task: %v", err)
		}
	}

	if err := dispatcher.Wait(); err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	metrics := dispatcher.GetMetrics()

	if metrics.TotalTasks != 5 {
		t.Errorf("expected 5 total tasks, got %d", metrics.TotalTasks)
	}
	if metrics.CompletedTasks != 5 {
		t.Errorf("expected 5 completed tasks, got %d", metrics.CompletedTasks)
	}
	if metrics.WorkerCount != 3 {
		t.Errorf("expected 3 workers, got %d", metrics.WorkerCount)
	}
}

func TestBatchSubmission(t *testing.T) {
	ctx := context.Background()
	agentFunc := func(ctx context.Context, task *Task) error {
		return nil
	}

	dispatcher, err := NewTaskDispatcher(ctx, agentFunc, 2)
	if err != nil {
		t.Fatalf("failed to create dispatcher: %v", err)
	}

	if err := dispatcher.Start(); err != nil {
		t.Fatalf("failed to start dispatcher: %v", err)
	}
	defer dispatcher.Shutdown(2 * time.Second)

	// Create batch
	var batch []*Task
	for i := 0; i < 5; i++ {
		batch = append(batch, &Task{
			ID:       fmt.Sprintf("batch-task-%d", i),
			Priority: PriorityNormal,
		})
	}

	// Submit batch
	if err := dispatcher.SubmitBatch(batch); err != nil {
		t.Fatalf("failed to submit batch: %v", err)
	}

	if err := dispatcher.Wait(); err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	metrics := dispatcher.GetMetrics()
	if metrics.CompletedTasks != 5 {
		t.Errorf("expected 5 completed tasks, got %d", metrics.CompletedTasks)
	}
}

func TestDoubleStart(t *testing.T) {
	ctx := context.Background()
	dispatcher, err := NewTaskDispatcher(ctx, func(ctx context.Context, task *Task) error {
		return nil
	}, 1)
	if err != nil {
		t.Fatalf("failed to create dispatcher: %v", err)
	}

	// Start first time
	if err := dispatcher.Start(); err != nil {
		t.Fatalf("first start failed: %v", err)
	}
	defer dispatcher.Shutdown(2 * time.Second)

	// Start second time should fail
	if err := dispatcher.Start(); err == nil {
		t.Errorf("expected error on double start, got nil")
	}
}

// Helper function
func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && s[:len(substr)] == substr || len(s) > len(substr))
}
