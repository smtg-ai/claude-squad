package concurrency

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriorityString(t *testing.T) {
	tests := []struct {
		priority Priority
		expected string
	}{
		{PriorityCritical, "Critical"},
		{PriorityHigh, "High"},
		{PriorityNormal, "Normal"},
		{PriorityLow, "Low"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.priority.String())
		})
	}
}

func TestTaskStatusString(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected string
	}{
		{TaskStatusPending, "Pending"},
		{TaskStatusRunning, "Running"},
		{TaskStatusCompleted, "Completed"},
		{TaskStatusFailed, "Failed"},
		{TaskStatusRetrying, "Retrying"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestExponentialBackoff(t *testing.T) {
	eb := NewExponentialBackoff()

	// Test that delays increase exponentially
	delay1 := eb.NextDelay(0)
	delay2 := eb.NextDelay(1)
	delay3 := eb.NextDelay(2)

	assert.Equal(t, 1*time.Second, delay1)
	assert.Equal(t, 2*time.Second, delay2)
	assert.Equal(t, 4*time.Second, delay3)

	// Test max delay cap
	delay10 := eb.NextDelay(10)
	assert.LessOrEqual(t, delay10, eb.MaxDelay)
}

func TestLinearBackoff(t *testing.T) {
	lb := &LinearBackoff{
		BaseDelay: 1 * time.Second,
		MaxDelay:  10 * time.Second,
	}

	delay1 := lb.NextDelay(0)
	delay2 := lb.NextDelay(1)
	delay3 := lb.NextDelay(2)

	assert.Equal(t, 1*time.Second, delay1)
	assert.Equal(t, 2*time.Second, delay2)
	assert.Equal(t, 3*time.Second, delay3)

	// Test max delay cap
	delay15 := lb.NextDelay(15)
	assert.Equal(t, lb.MaxDelay, delay15)
}

func TestDependencyResolver(t *testing.T) {
	dr := NewDependencyResolver()

	// Add tasks with dependencies
	err := dr.AddTask("task1", nil)
	assert.NoError(t, err)

	err = dr.AddTask("task2", []string{"task1"})
	assert.NoError(t, err)

	err = dr.AddTask("task3", []string{"task1", "task2"})
	assert.NoError(t, err)

	// Check execution eligibility
	assert.True(t, dr.CanExecute("task1"))
	assert.False(t, dr.CanExecute("task2"))
	assert.False(t, dr.CanExecute("task3"))

	// Mark task1 as completed
	dr.MarkCompleted("task1")
	assert.True(t, dr.CanExecute("task2"))
	assert.False(t, dr.CanExecute("task3"))

	// Mark task2 as completed
	dr.MarkCompleted("task2")
	assert.True(t, dr.CanExecute("task3"))
}

func TestDependencyResolverCircularDependency(t *testing.T) {
	dr := NewDependencyResolver()

	err := dr.AddTask("task1", []string{"task2"})
	assert.NoError(t, err)

	err = dr.AddTask("task2", []string{"task1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

func TestTaskQueueBasicEnqueueDequeue(t *testing.T) {
	tmpDir := t.TempDir()
	config := TaskQueueConfig{
		WorkerCount:     2,
		PersistencePath: filepath.Join(tmpDir, "queue.json"),
	}

	tq, err := NewTaskQueue(config)
	require.NoError(t, err)
	defer tq.Stop()

	// Create a simple task
	executed := false
	task := &Task{
		ID:       "test-task-1",
		Priority: PriorityNormal,
		Func: func(ctx context.Context) error {
			executed = true
			return nil
		},
	}

	// Enqueue the task
	err = tq.Enqueue(task)
	assert.NoError(t, err)

	// Dequeue and process the task
	ctx := context.Background()
	dequeuedTask, err := tq.Dequeue(ctx)
	assert.NoError(t, err)
	assert.Equal(t, task.ID, dequeuedTask.ID)

	err = tq.Process(dequeuedTask)
	assert.NoError(t, err)
	assert.True(t, executed)
	assert.Equal(t, TaskStatusCompleted, dequeuedTask.Status)
}

func TestTaskQueuePriorityOrdering(t *testing.T) {
	tmpDir := t.TempDir()
	config := TaskQueueConfig{
		WorkerCount:     1,
		PersistencePath: filepath.Join(tmpDir, "queue.json"),
	}

	tq, err := NewTaskQueue(config)
	require.NoError(t, err)
	defer tq.Stop()

	// Enqueue tasks with different priorities
	var executionOrder []string
	var mu sync.Mutex

	priorities := []Priority{PriorityLow, PriorityNormal, PriorityHigh, PriorityCritical}
	for i, priority := range priorities {
		taskID := fmt.Sprintf("task-%d", i)
		task := &Task{
			ID:       taskID,
			Priority: priority,
			Func: func(ctx context.Context) error {
				mu.Lock()
				executionOrder = append(executionOrder, taskID)
				mu.Unlock()
				return nil
			},
		}
		err := tq.Enqueue(task)
		assert.NoError(t, err)
	}

	// Process all tasks
	ctx := context.Background()
	for i := 0; i < len(priorities); i++ {
		task, err := tq.Dequeue(ctx)
		assert.NoError(t, err)
		err = tq.Process(task)
		assert.NoError(t, err)
	}

	// Verify that critical priority was executed first
	assert.Equal(t, "task-3", executionOrder[0]) // Critical
}

func TestTaskQueueRetry(t *testing.T) {
	tmpDir := t.TempDir()
	config := TaskQueueConfig{
		WorkerCount:     1,
		PersistencePath: filepath.Join(tmpDir, "queue.json"),
		BackoffStrategy: &LinearBackoff{
			BaseDelay: 10 * time.Millisecond,
			MaxDelay:  100 * time.Millisecond,
		},
	}

	tq, err := NewTaskQueue(config)
	require.NoError(t, err)
	defer tq.Stop()

	attempts := 0
	task := &Task{
		ID:         "retry-task",
		Priority:   PriorityNormal,
		MaxRetries: 3,
		Func: func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return fmt.Errorf("simulated failure")
			}
			return nil
		},
	}

	err = tq.Enqueue(task)
	assert.NoError(t, err)

	// Process the task (will fail and retry)
	ctx := context.Background()
	dequeuedTask, err := tq.Dequeue(ctx)
	assert.NoError(t, err)

	// First attempt - should fail
	err = tq.Process(dequeuedTask)
	assert.Error(t, err)
	assert.Equal(t, 1, dequeuedTask.RetryCount)
	assert.Equal(t, TaskStatusRetrying, dequeuedTask.Status)

	// Wait for retry to be scheduled and dequeue again
	time.Sleep(50 * time.Millisecond)
	dequeuedTask2, err := tq.Dequeue(ctx)
	assert.NoError(t, err)

	// Second attempt - should fail
	err = tq.Process(dequeuedTask2)
	assert.Error(t, err)
	assert.Equal(t, 2, dequeuedTask2.RetryCount)

	// Wait for retry and third attempt - should succeed
	time.Sleep(50 * time.Millisecond)
	dequeuedTask3, err := tq.Dequeue(ctx)
	assert.NoError(t, err)

	err = tq.Process(dequeuedTask3)
	assert.NoError(t, err)
	assert.Equal(t, TaskStatusCompleted, dequeuedTask3.Status)
	assert.Equal(t, 3, attempts)
}

func TestTaskQueueDeadLetterQueue(t *testing.T) {
	tmpDir := t.TempDir()
	config := TaskQueueConfig{
		WorkerCount:     1,
		PersistencePath: filepath.Join(tmpDir, "queue.json"),
		BackoffStrategy: &LinearBackoff{
			BaseDelay: 1 * time.Millisecond,
			MaxDelay:  5 * time.Millisecond,
		},
	}

	tq, err := NewTaskQueue(config)
	require.NoError(t, err)
	defer tq.Stop()

	task := &Task{
		ID:         "failing-task",
		Priority:   PriorityNormal,
		MaxRetries: 2,
		Func: func(ctx context.Context) error {
			return fmt.Errorf("always fails")
		},
	}

	err = tq.Enqueue(task)
	assert.NoError(t, err)

	ctx := context.Background()

	// Process and fail all retry attempts
	for i := 0; i <= task.MaxRetries; i++ {
		dequeuedTask, err := tq.Dequeue(ctx)
		assert.NoError(t, err)
		tq.Process(dequeuedTask)
		if i < task.MaxRetries {
			time.Sleep(10 * time.Millisecond) // Wait for retry scheduling
		}
	}

	// Verify task is in dead letter queue
	assert.Equal(t, TaskStatusFailed, task.Status)

	// The dead letter queue should have received the task
	select {
	case dlTask := <-tq.deadLetterQueue:
		assert.Equal(t, task.ID, dlTask.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("task not found in dead letter queue")
	}
}

func TestTaskQueueDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	config := TaskQueueConfig{
		WorkerCount:     2,
		PersistencePath: filepath.Join(tmpDir, "queue.json"),
	}

	tq, err := NewTaskQueue(config)
	require.NoError(t, err)
	defer tq.Stop()

	var executionOrder []string
	var mu sync.Mutex

	// Create tasks with dependencies
	task1 := &Task{
		ID:       "task1",
		Priority: PriorityNormal,
		Func: func(ctx context.Context) error {
			mu.Lock()
			executionOrder = append(executionOrder, "task1")
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	}

	task2 := &Task{
		ID:           "task2",
		Priority:     PriorityNormal,
		Dependencies: []string{"task1"},
		Func: func(ctx context.Context) error {
			mu.Lock()
			executionOrder = append(executionOrder, "task2")
			mu.Unlock()
			return nil
		},
	}

	task3 := &Task{
		ID:           "task3",
		Priority:     PriorityNormal,
		Dependencies: []string{"task1", "task2"},
		Func: func(ctx context.Context) error {
			mu.Lock()
			executionOrder = append(executionOrder, "task3")
			mu.Unlock()
			return nil
		},
	}

	// Enqueue in reverse order to test dependency resolution
	err = tq.Enqueue(task3)
	assert.NoError(t, err)
	err = tq.Enqueue(task2)
	assert.NoError(t, err)
	err = tq.Enqueue(task1)
	assert.NoError(t, err)

	// Start processing
	tq.Start()

	// Wait for all tasks to complete
	time.Sleep(200 * time.Millisecond)

	// Verify execution order respects dependencies
	mu.Lock()
	defer mu.Unlock()

	assert.Len(t, executionOrder, 3)
	assert.Equal(t, "task1", executionOrder[0])
	assert.Equal(t, "task2", executionOrder[1])
	assert.Equal(t, "task3", executionOrder[2])
}

func TestTaskQueuePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	persistPath := filepath.Join(tmpDir, "queue.json")

	// Create and populate queue
	config := TaskQueueConfig{
		WorkerCount:     1,
		PersistencePath: persistPath,
	}

	tq1, err := NewTaskQueue(config)
	require.NoError(t, err)

	task := &Task{
		ID:       "persist-task",
		Priority: PriorityHigh,
		Func: func(ctx context.Context) error {
			return nil
		},
		Metadata: map[string]string{"key": "value"},
	}

	err = tq1.Enqueue(task)
	assert.NoError(t, err)

	// Verify persistence file was created
	_, err = os.Stat(persistPath)
	assert.NoError(t, err)

	err = tq1.Stop()
	assert.NoError(t, err)

	// Create new queue and load state
	tq2, err := NewTaskQueue(config)
	require.NoError(t, err)
	defer tq2.Stop()

	// Register the task function (functions cannot be persisted)
	tq2.RegisterTaskFunc("persist-task", func(ctx context.Context) error {
		return nil
	})

	err = tq2.loadState()
	assert.NoError(t, err)

	// Verify task was restored
	restoredTask, err := tq2.GetTaskStatus("persist-task")
	assert.NoError(t, err)
	assert.Equal(t, task.ID, restoredTask.ID)
	assert.Equal(t, task.Priority, restoredTask.Priority)
}

func TestTaskQueueGetStats(t *testing.T) {
	tmpDir := t.TempDir()
	config := TaskQueueConfig{
		WorkerCount:     2,
		PersistencePath: filepath.Join(tmpDir, "queue.json"),
	}

	tq, err := NewTaskQueue(config)
	require.NoError(t, err)
	defer tq.Stop()

	// Add tasks with different statuses and priorities
	tasks := []*Task{
		{
			ID:       "task1",
			Priority: PriorityCritical,
			Status:   TaskStatusPending,
			Func:     func(ctx context.Context) error { return nil },
		},
		{
			ID:       "task2",
			Priority: PriorityHigh,
			Status:   TaskStatusRunning,
			Func:     func(ctx context.Context) error { return nil },
		},
		{
			ID:       "task3",
			Priority: PriorityNormal,
			Status:   TaskStatusCompleted,
			Func:     func(ctx context.Context) error { return nil },
		},
	}

	for _, task := range tasks {
		tq.mu.Lock()
		tq.tasks[task.ID] = task
		tq.mu.Unlock()
	}

	stats := tq.GetStats()
	assert.Equal(t, 3, stats["total_tasks"])
	assert.Equal(t, 2, stats["worker_count"])

	statusCounts := stats["status_counts"].(map[string]int)
	assert.Equal(t, 1, statusCounts["Pending"])
	assert.Equal(t, 1, statusCounts["Running"])
	assert.Equal(t, 1, statusCounts["Completed"])
}

func TestTaskQueueClearCompleted(t *testing.T) {
	tmpDir := t.TempDir()
	config := TaskQueueConfig{
		WorkerCount:     1,
		PersistencePath: filepath.Join(tmpDir, "queue.json"),
	}

	tq, err := NewTaskQueue(config)
	require.NoError(t, err)
	defer tq.Stop()

	// Add completed and pending tasks
	tq.mu.Lock()
	tq.tasks["completed1"] = &Task{
		ID:     "completed1",
		Status: TaskStatusCompleted,
	}
	tq.tasks["completed2"] = &Task{
		ID:     "completed2",
		Status: TaskStatusCompleted,
	}
	tq.tasks["pending1"] = &Task{
		ID:     "pending1",
		Status: TaskStatusPending,
	}
	tq.mu.Unlock()

	count := tq.ClearCompleted()
	assert.Equal(t, 2, count)

	tq.mu.RLock()
	assert.Len(t, tq.tasks, 1)
	_, exists := tq.tasks["pending1"]
	assert.True(t, exists)
	tq.mu.RUnlock()
}

func TestTaskQueueRetryFailedTask(t *testing.T) {
	tmpDir := t.TempDir()
	config := TaskQueueConfig{
		WorkerCount:     1,
		PersistencePath: filepath.Join(tmpDir, "queue.json"),
	}

	tq, err := NewTaskQueue(config)
	require.NoError(t, err)
	defer tq.Stop()

	task := &Task{
		ID:         "failed-task",
		Priority:   PriorityNormal,
		Status:     TaskStatusFailed,
		RetryCount: 3,
		Func:       func(ctx context.Context) error { return nil },
	}

	tq.mu.Lock()
	tq.tasks[task.ID] = task
	tq.mu.Unlock()

	err = tq.RetryFailedTask(task.ID)
	assert.NoError(t, err)

	assert.Equal(t, TaskStatusPending, task.Status)
	assert.Equal(t, 0, task.RetryCount)
	assert.Empty(t, task.LastError)
}

func TestTaskQueueWorkers(t *testing.T) {
	tmpDir := t.TempDir()
	config := TaskQueueConfig{
		WorkerCount:     3,
		PersistencePath: filepath.Join(tmpDir, "queue.json"),
	}

	tq, err := NewTaskQueue(config)
	require.NoError(t, err)

	var completed sync.WaitGroup
	var mu sync.Mutex
	processedTasks := make(map[string]bool)

	// Create multiple tasks
	for i := 0; i < 10; i++ {
		completed.Add(1)
		taskID := fmt.Sprintf("task-%d", i)
		task := &Task{
			ID:       taskID,
			Priority: PriorityNormal,
			Func: func(ctx context.Context) error {
				time.Sleep(10 * time.Millisecond)
				mu.Lock()
				processedTasks[taskID] = true
				mu.Unlock()
				completed.Done()
				return nil
			},
		}
		err := tq.Enqueue(task)
		assert.NoError(t, err)
	}

	// Start workers
	tq.Start()

	// Wait for all tasks to complete
	done := make(chan struct{})
	go func() {
		completed.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for tasks to complete")
	}

	tq.Stop()

	// Verify all tasks were processed
	mu.Lock()
	assert.Len(t, processedTasks, 10)
	mu.Unlock()
}

func BenchmarkTaskQueueEnqueue(b *testing.B) {
	tmpDir := b.TempDir()
	config := TaskQueueConfig{
		WorkerCount:     4,
		PersistencePath: filepath.Join(tmpDir, "queue.json"),
	}

	tq, _ := NewTaskQueue(config)
	defer tq.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := &Task{
			ID:       fmt.Sprintf("task-%d", i),
			Priority: PriorityNormal,
			Func:     func(ctx context.Context) error { return nil },
		}
		tq.Enqueue(task)
	}
}

func BenchmarkTaskQueueProcess(b *testing.B) {
	tmpDir := b.TempDir()
	config := TaskQueueConfig{
		WorkerCount:     4,
		PersistencePath: filepath.Join(tmpDir, "queue.json"),
	}

	tq, _ := NewTaskQueue(config)
	defer tq.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := &Task{
			ID:       fmt.Sprintf("task-%d", i),
			Priority: PriorityNormal,
			Func:     func(ctx context.Context) error { return nil },
		}
		tq.Process(task)
	}
}
