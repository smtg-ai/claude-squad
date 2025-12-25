package orchestrator

import (
	"context"
	"testing"
	"time"
)

// MockExecutor is a test implementation of AgentExecutor
type MockExecutor struct {
	ExecuteFunc func(ctx context.Context, task *Task) (*string, error)
}

func (m *MockExecutor) Execute(ctx context.Context, task *Task) (*string, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, task)
	}
	result := "mock result"
	return &result, nil
}

func TestNewAgentPool(t *testing.T) {
	// Note: This test requires the orchestrator service to be running
	// Skip if service is not available
	executor := &MockExecutor{}

	pool, err := NewAgentPool("http://localhost:5000", executor)
	if err != nil {
		t.Skipf("Skipping test: orchestrator service not available: %v", err)
		return
	}

	if pool == nil {
		t.Fatal("Expected pool to be created")
	}

	if pool.maxConcurrent != MaxConcurrentAgents {
		t.Errorf("Expected maxConcurrent to be %d, got %d", MaxConcurrentAgents, pool.maxConcurrent)
	}
}

func TestAgentPoolSubmitTask(t *testing.T) {
	executor := &MockExecutor{
		ExecuteFunc: func(ctx context.Context, task *Task) (*string, error) {
			time.Sleep(100 * time.Millisecond)
			result := "completed"
			return &result, nil
		},
	}

	pool, err := NewAgentPool("http://localhost:5000", executor)
	if err != nil {
		t.Skipf("Skipping test: orchestrator service not available: %v", err)
		return
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}
	defer pool.Stop()

	taskID, err := pool.SubmitTask(&Task{
		Description: "Test task",
		Priority:    5,
	})

	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	if taskID == "" {
		t.Error("Expected non-empty task ID")
	}
}

func TestAgentPoolConcurrency(t *testing.T) {
	executed := make(chan string, 15)

	executor := &MockExecutor{
		ExecuteFunc: func(ctx context.Context, task *Task) (*string, error) {
			executed <- task.ID
			time.Sleep(100 * time.Millisecond)
			result := "completed"
			return &result, nil
		},
	}

	pool, err := NewAgentPool("http://localhost:5000", executor)
	if err != nil {
		t.Skipf("Skipping test: orchestrator service not available: %v", err)
		return
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}
	defer pool.Stop()

	// Submit more tasks than max concurrency
	numTasks := 15
	for i := 0; i < numTasks; i++ {
		_, err := pool.SubmitTask(&Task{
			Description: "Concurrent test task",
			Priority:    5,
		})
		if err != nil {
			t.Fatalf("Failed to submit task %d: %v", i, err)
		}
	}

	// Wait a bit for tasks to start
	time.Sleep(500 * time.Millisecond)

	// Check that we don't exceed max concurrency
	running := pool.GetRunningCount()
	if running > MaxConcurrentAgents {
		t.Errorf("Running count %d exceeds max concurrency %d", running, MaxConcurrentAgents)
	}
}

func TestAgentPoolDependencies(t *testing.T) {
	executor := &MockExecutor{
		ExecuteFunc: func(ctx context.Context, task *Task) (*string, error) {
			time.Sleep(100 * time.Millisecond)
			result := "completed"
			return &result, nil
		},
	}

	pool, err := NewAgentPool("http://localhost:5000", executor)
	if err != nil {
		t.Skipf("Skipping test: orchestrator service not available: %v", err)
		return
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}
	defer pool.Stop()

	// Create task with dependency
	task1, err := pool.SubmitTask(&Task{
		Description: "Parent task",
		Priority:    10,
	})
	if err != nil {
		t.Fatalf("Failed to submit task1: %v", err)
	}

	task2, err := pool.SubmitTask(&Task{
		Description:  "Child task",
		Priority:     9,
		Dependencies: []string{task1},
	})
	if err != nil {
		t.Fatalf("Failed to submit task2: %v", err)
	}

	// Verify dependency chain
	chain, err := pool.GetTaskChain(task2)
	if err != nil {
		t.Fatalf("Failed to get task chain: %v", err)
	}

	if len(chain) == 0 {
		t.Error("Expected non-empty dependency chain")
	}
}

func TestAgentPoolAnalytics(t *testing.T) {
	executor := &MockExecutor{}

	pool, err := NewAgentPool("http://localhost:5000", executor)
	if err != nil {
		t.Skipf("Skipping test: orchestrator service not available: %v", err)
		return
	}

	analytics, err := pool.GetAnalytics()
	if err != nil {
		t.Fatalf("Failed to get analytics: %v", err)
	}

	if analytics.MaxConcurrent != MaxConcurrentAgents {
		t.Errorf("Expected max concurrent %d, got %d", MaxConcurrentAgents, analytics.MaxConcurrent)
	}

	if analytics.StatusCounts == nil {
		t.Error("Expected status counts to be initialized")
	}
}

func TestAgentPoolCancellation(t *testing.T) {
	executor := &MockExecutor{
		ExecuteFunc: func(ctx context.Context, task *Task) (*string, error) {
			// Long-running task
			select {
			case <-time.After(5 * time.Second):
				result := "completed"
				return &result, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}

	pool, err := NewAgentPool("http://localhost:5000", executor)
	if err != nil {
		t.Skipf("Skipping test: orchestrator service not available: %v", err)
		return
	}

	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}
	defer pool.Stop()

	taskID, err := pool.SubmitTask(&Task{
		Description: "Long task",
		Priority:    5,
	})
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// Wait for task to start
	time.Sleep(1 * time.Second)

	// Cancel the task
	if err := pool.CancelTask(taskID); err != nil {
		t.Errorf("Failed to cancel task: %v", err)
	}
}
