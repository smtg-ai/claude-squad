package ollama

import (
	"claude-squad/log"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// MaxWorkers defines the maximum number of concurrent agent executions
	MaxWorkers = 10

	// PriorityHigh represents high priority tasks
	PriorityHigh = 0
	// PriorityNormal represents normal priority tasks
	PriorityNormal = 1
	// PriorityLow represents low priority tasks
	PriorityLow = 2

	// maxErrors defines the maximum number of errors to store in the circular buffer
	maxErrors = 1000
)

// TaskStatus represents the current status of a task
type TaskStatus int

const (
	StatusPending TaskStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusCancelled
)

// String returns the string representation of TaskStatus
func (ts TaskStatus) String() string {
	switch ts {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// Task represents a unit of work to be executed by an agent
type Task struct {
	ID          string
	Priority    int
	Payload     interface{}
	Status      TaskStatus
	Error       error
	Result      interface{}
	CreatedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
}

// AgentFunc defines the function signature for task execution
// It receives the task context and the task to execute
type AgentFunc func(ctx context.Context, task *Task) error

// ProgressCallback is called to report task progress
// taskID: the ID of the task
// status: the current status of the task
// progress: progress percentage (0-100)
// message: optional message
type ProgressCallback func(taskID string, status TaskStatus, progress int, message string)

// TaskDispatcher manages concurrent task execution with a worker pool
type TaskDispatcher struct {
	// Configuration
	workerCount  int
	maxQueueSize int
	agentFunc    AgentFunc
	progressCb   ProgressCallback

	// Concurrency control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex

	// Task management
	taskQueue chan *Task
	taskMap   map[string]*Task
	taskMapMu sync.RWMutex

	// Error handling
	errors     []TaskExecutionError
	errorsMu   sync.Mutex
	errorIndex int // Current position in the circular buffer

	// State
	isRunning    bool
	shutdownOnce sync.Once

	// Metrics
	completedCount int
	failedCount    int
	cancelledCount int
	metricsMu      sync.RWMutex
}

// TaskExecutionError represents an error that occurred during task execution
type TaskExecutionError struct {
	TaskID    string
	Error     error
	Timestamp time.Time
	WorkerID  int
}

// NewTaskDispatcher creates a new task dispatcher with the specified agent function
// workerCount must be between 1 and MaxWorkers (10)
func NewTaskDispatcher(ctx context.Context, agentFunc AgentFunc, workerCount int) (*TaskDispatcher, error) {
	if agentFunc == nil {
		return nil, fmt.Errorf("agent function cannot be nil")
	}

	if workerCount <= 0 || workerCount > MaxWorkers {
		return nil, fmt.Errorf("worker count must be between 1 and %d, got %d", MaxWorkers, workerCount)
	}

	dispatcherCtx, cancel := context.WithCancel(ctx)

	dispatcher := &TaskDispatcher{
		workerCount:  workerCount,
		maxQueueSize: workerCount * 100, // Queue size = 100x worker count
		agentFunc:    agentFunc,
		ctx:          dispatcherCtx,
		cancel:       cancel,
		taskQueue:    make(chan *Task, workerCount*100),
		taskMap:      make(map[string]*Task),
		errors:       make([]TaskExecutionError, 0, maxErrors),
		errorIndex:   0,
	}

	log.InfoLog.Printf("TaskDispatcher created with %d workers", workerCount)
	return dispatcher, nil
}

// SetProgressCallback sets the progress callback function
func (d *TaskDispatcher) SetProgressCallback(cb ProgressCallback) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.progressCb = cb
}

// Start initializes and starts the worker pool
func (d *TaskDispatcher) Start() error {
	d.mu.Lock()
	if d.isRunning {
		d.mu.Unlock()
		return fmt.Errorf("dispatcher is already running")
	}
	d.isRunning = true
	d.mu.Unlock()

	// Start worker goroutines
	for i := 0; i < d.workerCount; i++ {
		d.wg.Add(1)
		go d.worker(i)
	}

	log.InfoLog.Printf("TaskDispatcher started with %d workers", d.workerCount)
	return nil
}

// SubmitTask adds a task to the queue for execution
func (d *TaskDispatcher) SubmitTask(task *Task) error {
	d.mu.RLock()
	if !d.isRunning {
		d.mu.RUnlock()
		return fmt.Errorf("dispatcher is not running")
	}
	d.mu.RUnlock()

	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	if task.ID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}

	// Validate priority is within valid range
	if task.Priority < PriorityHigh || task.Priority > PriorityLow {
		return fmt.Errorf("task priority must be between %d and %d, got %d", PriorityHigh, PriorityLow, task.Priority)
	}

	// Initialize task metadata
	task.Status = StatusPending
	task.CreatedAt = time.Now()

	// Store task in map for tracking
	d.taskMapMu.Lock()
	d.taskMap[task.ID] = task
	d.taskMapMu.Unlock()

	// Send task to queue with context awareness
	select {
	case d.taskQueue <- task:
		log.InfoLog.Printf("Task %s submitted with priority %d", task.ID, task.Priority)
		d.reportProgress(task.ID, StatusPending, 0, "task submitted")
		return nil
	case <-d.ctx.Done():
		return fmt.Errorf("dispatcher context cancelled")
	default:
		return fmt.Errorf("task queue is full")
	}
}

// SubmitBatch submits multiple tasks to the dispatcher
func (d *TaskDispatcher) SubmitBatch(tasks []*Task) error {
	var submitErrors []error

	for _, task := range tasks {
		if err := d.SubmitTask(task); err != nil {
			submitErrors = append(submitErrors, fmt.Errorf("task %s: %w", task.ID, err))
		}
	}

	if len(submitErrors) > 0 {
		return fmt.Errorf("batch submission encountered errors: %v", submitErrors)
	}

	return nil
}

// worker processes tasks from the queue
func (d *TaskDispatcher) worker(id int) {
	defer d.wg.Done()

	log.InfoLog.Printf("Worker %d started", id)

	for {
		select {
		case task, ok := <-d.taskQueue:
			if !ok {
				log.InfoLog.Printf("Worker %d shutting down", id)
				return
			}

			// Execute the task
			d.executeTask(task, id)

		case <-d.ctx.Done():
			log.InfoLog.Printf("Worker %d received cancellation signal", id)
			return
		}
	}
}

// executeTask executes a single task with proper error handling and progress tracking
func (d *TaskDispatcher) executeTask(task *Task, workerID int) {
	// Update task status
	d.updateTaskStatus(task.ID, StatusRunning)
	d.taskMapMu.Lock()
	task.StartedAt = time.Now()
	d.taskMapMu.Unlock()
	d.reportProgress(task.ID, StatusRunning, 10, "task execution started")

	log.InfoLog.Printf("Worker %d executing task %s", workerID, task.ID)

	// Create a context with timeout for the task
	taskCtx, cancel := context.WithCancel(d.ctx)
	defer cancel()

	// Execute the agent function
	err := d.agentFunc(taskCtx, task)

	d.taskMapMu.Lock()
	task.CompletedAt = time.Now()
	d.taskMapMu.Unlock()
	duration := task.CompletedAt.Sub(task.StartedAt)

	if err != nil {
		// Handle error
		d.taskMapMu.Lock()
		task.Error = err
		d.taskMapMu.Unlock()
		d.updateTaskStatus(task.ID, StatusFailed)
		d.recordError(TaskExecutionError{
			TaskID:    task.ID,
			Error:     err,
			Timestamp: time.Now(),
			WorkerID:  workerID,
		})
		d.incrementFailedCount()

		log.ErrorLog.Printf("Task %s failed on worker %d after %v: %v", task.ID, workerID, duration, err)
		d.reportProgress(task.ID, StatusFailed, 100, fmt.Sprintf("task failed: %v", err))
		// Clean up task from map after failure
		d.cleanupTask(task.ID)
		return
	}

	// Task completed successfully
	d.updateTaskStatus(task.ID, StatusCompleted)
	d.incrementCompletedCount()

	log.InfoLog.Printf("Task %s completed on worker %d in %v", task.ID, workerID, duration)
	d.reportProgress(task.ID, StatusCompleted, 100, "task completed successfully")
	// Clean up task from map after completion
	d.cleanupTask(task.ID)
}

// Wait blocks until all tasks are completed or the context is cancelled
func (d *TaskDispatcher) Wait() error {
	d.wg.Wait()
	log.InfoLog.Printf("All workers finished")
	return nil
}

// Shutdown gracefully shuts down the dispatcher
// It closes the task queue and waits for all workers to finish
func (d *TaskDispatcher) Shutdown(timeout time.Duration) error {
	var shutdownErr error
	d.shutdownOnce.Do(func() {
		log.InfoLog.Printf("Shutting down TaskDispatcher")

		// Stop accepting new tasks
		d.mu.Lock()
		d.isRunning = false
		d.mu.Unlock()

		// Close task queue to signal workers to stop
		close(d.taskQueue)

		// Create a channel to signal when all workers are done
		done := make(chan struct{})
		go func() {
			d.wg.Wait()
			close(done)
		}()

		// Wait for all workers with timeout
		select {
		case <-done:
			log.InfoLog.Printf("All workers shut down gracefully")
		case <-time.After(timeout):
			shutdownErr = fmt.Errorf("shutdown timeout exceeded after %v", timeout)
			log.WarningLog.Printf("%v", shutdownErr)
		}

		// Cancel context to force cleanup
		d.cancel()
	})

	return shutdownErr
}

// GetTaskStatus returns the current status of a task
func (d *TaskDispatcher) GetTaskStatus(taskID string) (TaskStatus, error) {
	d.taskMapMu.RLock()
	defer d.taskMapMu.RUnlock()

	task, exists := d.taskMap[taskID]
	if !exists {
		return StatusPending, fmt.Errorf("task %s not found", taskID)
	}

	return task.Status, nil
}

// GetTask returns the task details
func (d *TaskDispatcher) GetTask(taskID string) (*Task, error) {
	d.taskMapMu.RLock()
	defer d.taskMapMu.RUnlock()

	task, exists := d.taskMap[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	return task, nil
}

// CancelTask attempts to cancel a task
func (d *TaskDispatcher) CancelTask(taskID string) error {
	d.taskMapMu.Lock()
	defer d.taskMapMu.Unlock()

	task, exists := d.taskMap[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status == StatusPending {
		task.Status = StatusCancelled
		d.incrementCancelledCount()
		log.InfoLog.Printf("Task %s cancelled", taskID)
		d.reportProgress(taskID, StatusCancelled, 0, "task cancelled")
		return nil
	}

	if task.Status == StatusRunning {
		task.Status = StatusCancelled
		d.incrementCancelledCount()
		log.InfoLog.Printf("Cancellation requested for running task %s", taskID)
		d.reportProgress(taskID, StatusCancelled, 0, "task cancellation requested")
		return nil
	}

	return fmt.Errorf("cannot cancel task %s with status %s", taskID, task.Status.String())
}

// GetErrors returns all errors that occurred during task execution
func (d *TaskDispatcher) GetErrors() []TaskExecutionError {
	d.errorsMu.Lock()
	defer d.errorsMu.Unlock()

	// Return a copy to prevent external modification
	errorsCopy := make([]TaskExecutionError, len(d.errors))
	copy(errorsCopy, d.errors)
	return errorsCopy
}

// GetMetrics returns dispatcher metrics
func (d *TaskDispatcher) GetMetrics() DispatcherMetrics {
	d.metricsMu.RLock()
	defer d.metricsMu.RUnlock()

	d.taskMapMu.RLock()
	totalTasks := len(d.taskMap)
	d.taskMapMu.RUnlock()

	return DispatcherMetrics{
		TotalTasks:     totalTasks,
		CompletedTasks: d.completedCount,
		FailedTasks:    d.failedCount,
		CancelledTasks: d.cancelledCount,
		PendingTasks:   totalTasks - d.completedCount - d.failedCount - d.cancelledCount,
		WorkerCount:    d.workerCount,
	}
}

// DispatcherMetrics contains metrics about dispatcher performance
type DispatcherMetrics struct {
	TotalTasks     int
	CompletedTasks int
	FailedTasks    int
	CancelledTasks int
	PendingTasks   int
	WorkerCount    int
}

// Helper functions

// updateTaskStatus updates the status of a task
func (d *TaskDispatcher) updateTaskStatus(taskID string, status TaskStatus) {
	d.taskMapMu.Lock()
	defer d.taskMapMu.Unlock()

	if task, exists := d.taskMap[taskID]; exists {
		task.Status = status
	}
}

// recordError records an error that occurred during task execution with bounded rotation
func (d *TaskDispatcher) recordError(err TaskExecutionError) {
	d.errorsMu.Lock()
	defer d.errorsMu.Unlock()

	// If buffer is not full, append normally
	if len(d.errors) < maxErrors {
		d.errors = append(d.errors, err)
	} else {
		// Buffer is full, use rotation to keep only recent errors
		d.errorIndex = (d.errorIndex + 1) % maxErrors
		d.errors[d.errorIndex] = err
	}
}

// reportProgress calls the progress callback if set
func (d *TaskDispatcher) reportProgress(taskID string, status TaskStatus, progress int, message string) {
	d.mu.RLock()
	cb := d.progressCb
	d.mu.RUnlock()

	if cb != nil {
		cb(taskID, status, progress, message)
	}
}

// incrementCompletedCount increments the completed task counter
func (d *TaskDispatcher) incrementCompletedCount() {
	d.metricsMu.Lock()
	defer d.metricsMu.Unlock()
	d.completedCount++
}

// incrementFailedCount increments the failed task counter
func (d *TaskDispatcher) incrementFailedCount() {
	d.metricsMu.Lock()
	defer d.metricsMu.Unlock()
	d.failedCount++
}

// incrementCancelledCount increments the cancelled task counter
func (d *TaskDispatcher) incrementCancelledCount() {
	d.metricsMu.Lock()
	defer d.metricsMu.Unlock()
	d.cancelledCount++
}

// cleanupTask removes a completed task from the task map to prevent memory leaks
func (d *TaskDispatcher) cleanupTask(taskID string) {
	d.taskMapMu.Lock()
	defer d.taskMapMu.Unlock()
	delete(d.taskMap, taskID)
	log.InfoLog.Printf("Task %s cleaned up from task map", taskID)
}

// CombineErrors combines multiple errors into a single formatted error
func CombineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	if len(errs) == 1 {
		return errs[0]
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "encountered %d errors:\n", len(errs))
	for i, err := range errs {
		fmt.Fprintf(&builder, "  %d. %v\n", i+1, err)
	}
	return fmt.Errorf("%s", builder.String())
}
