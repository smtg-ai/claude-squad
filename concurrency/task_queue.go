package concurrency

import (
	"claude-squad/log"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// QueuePriority represents task priority levels for the queue
type QueuePriority int

const (
	QueuePriorityLow QueuePriority = iota
	QueuePriorityNormal
	QueuePriorityHigh
	QueuePriorityCritical
)

// String returns the string representation of priority
func (p QueuePriority) String() string {
	switch p {
	case QueuePriorityCritical:
		return "Critical"
	case QueuePriorityHigh:
		return "High"
	case QueuePriorityNormal:
		return "Normal"
	case QueuePriorityLow:
		return "Low"
	default:
		return "Unknown"
	}
}

// TaskStatus represents the current state of a task
type TaskStatus int

const (
	TaskStatusPending TaskStatus = iota
	TaskStatusRunning
	TaskStatusCompleted
	TaskStatusFailed
	TaskStatusRetrying
)

// String returns the string representation of task status
func (ts TaskStatus) String() string {
	switch ts {
	case TaskStatusPending:
		return "Pending"
	case TaskStatusRunning:
		return "Running"
	case TaskStatusCompleted:
		return "Completed"
	case TaskStatusFailed:
		return "Failed"
	case TaskStatusRetrying:
		return "Retrying"
	default:
		return "Unknown"
	}
}

// TaskFunc is the function signature for task execution
type TaskFunc func(ctx context.Context) error

// QueueTask represents a unit of work in the queue
type QueueTask struct {
	ID           string        `json:"id"`
	Priority     QueuePriority `json:"priority"`
	Dependencies []string      `json:"dependencies"`
	RetryCount   int           `json:"retry_count"`
	MaxRetries   int           `json:"max_retries"`
	Status       TaskStatus    `json:"status"`
	CreatedAt    time.Time     `json:"created_at"`
	StartedAt    *time.Time    `json:"started_at,omitempty"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	LastError    string        `json:"last_error,omitempty"`
	Metadata     interface{}   `json:"metadata,omitempty"`

	// Function to execute (not serialized)
	Func TaskFunc `json:"-"`
}

// BackoffStrategy defines the interface for retry backoff strategies
type BackoffStrategy interface {
	// NextDelay calculates the delay before the next retry attempt
	NextDelay(retryCount int) time.Duration
}

// ExponentialBackoff implements exponential backoff strategy
type ExponentialBackoff struct {
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	Multiplier float64
}

// NewExponentialBackoff creates a new exponential backoff strategy with defaults
func NewExponentialBackoff() *ExponentialBackoff {
	return &ExponentialBackoff{
		BaseDelay:  1 * time.Second,
		MaxDelay:   5 * time.Minute,
		Multiplier: 2.0,
	}
}

// NextDelay calculates the exponential backoff delay
func (eb *ExponentialBackoff) NextDelay(retryCount int) time.Duration {
	delay := float64(eb.BaseDelay) * math.Pow(eb.Multiplier, float64(retryCount))
	delayDuration := time.Duration(delay)

	if delayDuration > eb.MaxDelay {
		return eb.MaxDelay
	}
	return delayDuration
}

// LinearBackoff implements linear backoff strategy
type LinearBackoff struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

// NextDelay calculates the linear backoff delay
func (lb *LinearBackoff) NextDelay(retryCount int) time.Duration {
	delay := lb.BaseDelay * time.Duration(retryCount+1)
	if delay > lb.MaxDelay {
		return lb.MaxDelay
	}
	return delay
}

// DependencyResolver manages task dependencies and DAG execution
type DependencyResolver struct {
	mu               sync.RWMutex
	completedTasks   map[string]bool
	taskDependencies map[string][]string
}

// NewDependencyResolver creates a new dependency resolver
func NewDependencyResolver() *DependencyResolver {
	return &DependencyResolver{
		completedTasks:   make(map[string]bool),
		taskDependencies: make(map[string][]string),
	}
}

// AddTask registers a task and its dependencies
func (dr *DependencyResolver) AddTask(taskID string, dependencies []string) error {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	// Check for circular dependencies
	if err := dr.checkCircularDependency(taskID, dependencies); err != nil {
		return err
	}

	dr.taskDependencies[taskID] = dependencies
	return nil
}

// MarkCompleted marks a task as completed
func (dr *DependencyResolver) MarkCompleted(taskID string) {
	dr.mu.Lock()
	defer dr.mu.Unlock()
	dr.completedTasks[taskID] = true
}

// CanExecute checks if all dependencies of a task are completed
func (dr *DependencyResolver) CanExecute(taskID string) bool {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	dependencies, exists := dr.taskDependencies[taskID]
	if !exists {
		return true
	}

	for _, depID := range dependencies {
		if !dr.completedTasks[depID] {
			return false
		}
	}
	return true
}

// checkCircularDependency detects circular dependencies using DFS
func (dr *DependencyResolver) checkCircularDependency(taskID string, dependencies []string) error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(string) bool
	dfs = func(id string) bool {
		visited[id] = true
		recStack[id] = true

		deps, exists := dr.taskDependencies[id]
		if !exists && id == taskID {
			deps = dependencies
		}

		for _, depID := range deps {
			if !visited[depID] {
				if dfs(depID) {
					return true
				}
			} else if recStack[depID] {
				return true
			}
		}

		recStack[id] = false
		return false
	}

	if dfs(taskID) {
		return fmt.Errorf("circular dependency detected for task %s", taskID)
	}
	return nil
}

// TaskQueue manages concurrent task execution with priorities and dependencies
type TaskQueue struct {
	mu                 sync.RWMutex
	tasks              map[string]*QueueTask
	priorityQueues     map[QueuePriority]chan *QueueTask
	deadLetterQueue    chan *QueueTask
	dependencyResolver *DependencyResolver
	backoffStrategy    BackoffStrategy
	persistencePath    string
	workerCount        int
	stopCh             chan struct{}
	wg                 sync.WaitGroup
	taskRegistry       map[string]TaskFunc // Maps task IDs to their functions
	ctx                context.Context
	cancel             context.CancelFunc
}

// TaskQueueConfig holds configuration for the task queue
type TaskQueueConfig struct {
	WorkerCount     int
	PersistencePath string
	BackoffStrategy BackoffStrategy
}

// NewTaskQueue creates a new task queue with the given configuration
func NewTaskQueue(config TaskQueueConfig) (*TaskQueue, error) {
	if config.WorkerCount <= 0 {
		config.WorkerCount = 4
	}

	if config.BackoffStrategy == nil {
		config.BackoffStrategy = NewExponentialBackoff()
	}

	ctx, cancel := context.WithCancel(context.Background())

	tq := &TaskQueue{
		tasks:              make(map[string]*QueueTask),
		priorityQueues:     make(map[QueuePriority]chan *QueueTask),
		deadLetterQueue:    make(chan *QueueTask, 100),
		dependencyResolver: NewDependencyResolver(),
		backoffStrategy:    config.BackoffStrategy,
		persistencePath:    config.PersistencePath,
		workerCount:        config.WorkerCount,
		stopCh:             make(chan struct{}),
		taskRegistry:       make(map[string]TaskFunc),
		ctx:                ctx,
		cancel:             cancel,
	}

	// Initialize priority queues
	tq.priorityQueues[QueuePriorityCritical] = make(chan *QueueTask, 50)
	tq.priorityQueues[QueuePriorityHigh] = make(chan *QueueTask, 100)
	tq.priorityQueues[QueuePriorityNormal] = make(chan *QueueTask, 200)
	tq.priorityQueues[QueuePriorityLow] = make(chan *QueueTask, 300)

	// Load persisted state if path is provided
	if config.PersistencePath != "" {
		if err := tq.loadState(); err != nil {
			log.WarningLog.Printf("failed to load task queue state: %v", err)
		}
	}

	return tq, nil
}

// RegisterTaskFunc registers a task function with a given name
func (tq *TaskQueue) RegisterTaskFunc(name string, fn TaskFunc) {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	tq.taskRegistry[name] = fn
}

// Enqueue adds a task to the queue
func (tq *TaskQueue) Enqueue(task *QueueTask) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	if task.ID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}

	if task.Func == nil {
		return fmt.Errorf("task function cannot be nil")
	}

	tq.mu.Lock()
	defer tq.mu.Unlock()

	// Check if task already exists
	if _, exists := tq.tasks[task.ID]; exists {
		return fmt.Errorf("task with ID %s already exists", task.ID)
	}

	// Set defaults
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	if task.Status == TaskStatus(0) {
		task.Status = TaskStatusPending
	}

	// Register dependencies
	if len(task.Dependencies) > 0 {
		if err := tq.dependencyResolver.AddTask(task.ID, task.Dependencies); err != nil {
			return fmt.Errorf("failed to add task dependencies: %w", err)
		}
	}

	// Store task
	tq.tasks[task.ID] = task

	// Persist state
	if err := tq.persistState(); err != nil {
		log.WarningLog.Printf("failed to persist task queue state: %v", err)
	}

	// Enqueue to priority queue if dependencies are satisfied
	if tq.dependencyResolver.CanExecute(task.ID) {
		select {
		case tq.priorityQueues[task.Priority] <- task:
			log.InfoLog.Printf("enqueued task %s with priority %s", task.ID, task.Priority)
		default:
			return fmt.Errorf("priority queue for %s is full", task.Priority)
		}
	} else {
		log.InfoLog.Printf("task %s waiting for dependencies", task.ID)
	}

	return nil
}

// Dequeue retrieves the next task to execute based on priority
func (tq *TaskQueue) Dequeue(ctx context.Context) (*QueueTask, error) {
	// QueuePriority order: Critical > High > Normal > Low
	priorities := []QueuePriority{QueuePriorityCritical, QueuePriorityHigh, QueuePriorityNormal, QueuePriorityLow}

	for {
		// Check each priority level
		for _, priority := range priorities {
			select {
			case task := <-tq.priorityQueues[priority]:
				return task, nil
			default:
				// No task at this priority level, try next
				continue
			}
		}

		// No tasks available, wait or return
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-tq.stopCh:
			return nil, fmt.Errorf("task queue stopped")
		case <-time.After(100 * time.Millisecond):
			// Brief wait before checking again
			continue
		}
	}
}

// Process executes a task with retry logic
func (tq *TaskQueue) Process(task *QueueTask) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	// Update task status
	tq.mu.Lock()
	task.Status = TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now
	tq.mu.Unlock()

	log.InfoLog.Printf("processing task %s (priority: %s, attempt: %d/%d)",
		task.ID, task.Priority, task.RetryCount+1, task.MaxRetries+1)

	// Execute task with timeout context
	taskCtx, cancel := context.WithTimeout(tq.ctx, 10*time.Minute)
	defer cancel()

	err := task.Func(taskCtx)

	tq.mu.Lock()
	defer tq.mu.Unlock()

	if err != nil {
		task.LastError = err.Error()
		log.ErrorLog.Printf("task %s failed: %v", task.ID, err)

		// Check if we should retry
		if task.RetryCount < task.MaxRetries {
			task.RetryCount++
			task.Status = TaskStatusRetrying

			// Calculate backoff delay
			delay := tq.backoffStrategy.NextDelay(task.RetryCount - 1)
			log.InfoLog.Printf("retrying task %s in %v (attempt %d/%d)",
				task.ID, delay, task.RetryCount+1, task.MaxRetries+1)

			// Schedule retry after delay
			go func(t *QueueTask, d time.Duration) {
				time.Sleep(d)
				select {
				case tq.priorityQueues[t.Priority] <- t:
					log.InfoLog.Printf("task %s requeued for retry", t.ID)
				default:
					log.ErrorLog.Printf("failed to requeue task %s: queue full", t.ID)
					tq.moveToDeadLetterQueue(t)
				}
			}(task, delay)

			return err
		}

		// Max retries exceeded, move to dead letter queue
		task.Status = TaskStatusFailed
		completedAt := time.Now()
		task.CompletedAt = &completedAt

		tq.moveToDeadLetterQueue(task)

		if err := tq.persistState(); err != nil {
			log.WarningLog.Printf("failed to persist state after task failure: %v", err)
		}

		return fmt.Errorf("task %s failed after %d retries: %w", task.ID, task.RetryCount, err)
	}

	// Task completed successfully
	task.Status = TaskStatusCompleted
	completedAt := time.Now()
	task.CompletedAt = &completedAt

	log.InfoLog.Printf("task %s completed successfully", task.ID)

	// Mark as completed in dependency resolver
	tq.dependencyResolver.MarkCompleted(task.ID)

	// Check if any tasks are waiting for this dependency
	tq.checkAndEnqueueDependentTasks(task.ID)

	if err := tq.persistState(); err != nil {
		log.WarningLog.Printf("failed to persist state after task completion: %v", err)
	}

	return nil
}

// moveToDeadLetterQueue moves a failed task to the dead letter queue
func (tq *TaskQueue) moveToDeadLetterQueue(task *QueueTask) {
	select {
	case tq.deadLetterQueue <- task:
		log.WarningLog.Printf("task %s moved to dead letter queue", task.ID)
	default:
		log.ErrorLog.Printf("dead letter queue is full, cannot move task %s", task.ID)
	}
}

// checkAndEnqueueDependentTasks checks for tasks waiting on completed dependencies
func (tq *TaskQueue) checkAndEnqueueDependentTasks(completedTaskID string) {
	for taskID, task := range tq.tasks {
		// Skip if not pending or already in queue
		if task.Status != TaskStatusPending {
			continue
		}

		// Check if this task depends on the completed task
		hasDependency := false
		for _, depID := range task.Dependencies {
			if depID == completedTaskID {
				hasDependency = true
				break
			}
		}

		if !hasDependency {
			continue
		}

		// Check if all dependencies are now satisfied
		if tq.dependencyResolver.CanExecute(taskID) {
			select {
			case tq.priorityQueues[task.Priority] <- task:
				log.InfoLog.Printf("task %s dependencies satisfied, enqueued", taskID)
			default:
				log.ErrorLog.Printf("failed to enqueue task %s: queue full", taskID)
			}
		}
	}
}

// Start begins processing tasks with the configured number of workers
func (tq *TaskQueue) Start() {
	log.InfoLog.Printf("starting task queue with %d workers", tq.workerCount)

	for i := 0; i < tq.workerCount; i++ {
		tq.wg.Add(1)
		go tq.worker(i)
	}

	// Start dead letter queue processor
	tq.wg.Add(1)
	go tq.deadLetterProcessor()
}

// worker processes tasks from the queue
func (tq *TaskQueue) worker(id int) {
	defer tq.wg.Done()

	log.InfoLog.Printf("worker %d started", id)

	for {
		select {
		case <-tq.stopCh:
			log.InfoLog.Printf("worker %d stopping", id)
			return
		default:
			task, err := tq.Dequeue(tq.ctx)
			if err != nil {
				if err == context.Canceled {
					log.InfoLog.Printf("worker %d context canceled", id)
					return
				}
				continue
			}

			if task != nil {
				if err := tq.Process(task); err != nil {
					log.ErrorLog.Printf("worker %d: error processing task %s: %v", id, task.ID, err)
				}
			}
		}
	}
}

// deadLetterProcessor handles tasks in the dead letter queue
func (tq *TaskQueue) deadLetterProcessor() {
	defer tq.wg.Done()

	log.InfoLog.Printf("dead letter queue processor started")

	for {
		select {
		case <-tq.stopCh:
			log.InfoLog.Printf("dead letter queue processor stopping")
			return
		case task := <-tq.deadLetterQueue:
			log.ErrorLog.Printf("dead letter task: ID=%s, QueuePriority=%s, Retries=%d, Error=%s",
				task.ID, task.Priority, task.RetryCount, task.LastError)

			// Here you could implement additional logic like:
			// - Sending notifications
			// - Writing to a separate log file
			// - Triggering alerts
		}
	}
}

// Stop gracefully shuts down the task queue
func (tq *TaskQueue) Stop() error {
	log.InfoLog.Printf("stopping task queue")

	// Signal all workers to stop
	close(tq.stopCh)

	// Cancel context to interrupt any running tasks
	tq.cancel()

	// Wait for all workers to finish
	tq.wg.Wait()

	// Persist final state
	if err := tq.persistState(); err != nil {
		return fmt.Errorf("failed to persist final state: %w", err)
	}

	log.InfoLog.Printf("task queue stopped successfully")
	return nil
}

// GetTaskStatus returns the current status of a task
func (tq *TaskQueue) GetTaskStatus(taskID string) (*QueueTask, error) {
	tq.mu.RLock()
	defer tq.mu.RUnlock()

	task, exists := tq.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	return task, nil
}

// GetStats returns statistics about the task queue
func (tq *TaskQueue) GetStats() map[string]interface{} {
	tq.mu.RLock()
	defer tq.mu.RUnlock()

	stats := make(map[string]interface{})

	statusCounts := make(map[string]int)
	priorityCounts := make(map[string]int)

	for _, task := range tq.tasks {
		statusCounts[task.Status.String()]++
		priorityCounts[task.Priority.String()]++
	}

	stats["total_tasks"] = len(tq.tasks)
	stats["status_counts"] = statusCounts
	stats["priority_counts"] = priorityCounts
	stats["worker_count"] = tq.workerCount
	stats["dead_letter_queue_size"] = len(tq.deadLetterQueue)

	return stats
}

// persistState saves the current queue state to disk
func (tq *TaskQueue) persistState() error {
	if tq.persistencePath == "" {
		return nil
	}

	tq.mu.RLock()
	defer tq.mu.RUnlock()

	// Prepare state for serialization
	type persistedTask struct {
		ID           string      `json:"id"`
		Priority     QueuePriority    `json:"priority"`
		Dependencies []string    `json:"dependencies"`
		RetryCount   int         `json:"retry_count"`
		MaxRetries   int         `json:"max_retries"`
		Status       TaskStatus  `json:"status"`
		CreatedAt    time.Time   `json:"created_at"`
		StartedAt    *time.Time  `json:"started_at,omitempty"`
		CompletedAt  *time.Time  `json:"completed_at,omitempty"`
		LastError    string      `json:"last_error,omitempty"`
		Metadata     interface{} `json:"metadata,omitempty"`
	}

	state := struct {
		Tasks map[string]*persistedTask `json:"tasks"`
	}{
		Tasks: make(map[string]*persistedTask),
	}

	for id, task := range tq.tasks {
		state.Tasks[id] = &persistedTask{
			ID:           task.ID,
			Priority:     task.Priority,
			Dependencies: task.Dependencies,
			RetryCount:   task.RetryCount,
			MaxRetries:   task.MaxRetries,
			Status:       task.Status,
			CreatedAt:    task.CreatedAt,
			StartedAt:    task.StartedAt,
			CompletedAt:  task.CompletedAt,
			LastError:    task.LastError,
			Metadata:     task.Metadata,
		}
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(tq.persistencePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create persistence directory: %w", err)
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task queue state: %w", err)
	}

	// Write to file
	if err := os.WriteFile(tq.persistencePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write task queue state: %w", err)
	}

	return nil
}

// loadState restores the queue state from disk
func (tq *TaskQueue) loadState() error {
	if tq.persistencePath == "" {
		return nil
	}

	data, err := os.ReadFile(tq.persistencePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No state file exists yet
		}
		return fmt.Errorf("failed to read task queue state: %w", err)
	}

	type persistedTask struct {
		ID           string      `json:"id"`
		Priority     QueuePriority    `json:"priority"`
		Dependencies []string    `json:"dependencies"`
		RetryCount   int         `json:"retry_count"`
		MaxRetries   int         `json:"max_retries"`
		Status       TaskStatus  `json:"status"`
		CreatedAt    time.Time   `json:"created_at"`
		StartedAt    *time.Time  `json:"started_at,omitempty"`
		CompletedAt  *time.Time  `json:"completed_at,omitempty"`
		LastError    string      `json:"last_error,omitempty"`
		Metadata     interface{} `json:"metadata,omitempty"`
	}

	state := struct {
		Tasks map[string]*persistedTask `json:"tasks"`
	}{}

	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal task queue state: %w", err)
	}

	tq.mu.Lock()
	defer tq.mu.Unlock()

	// Restore tasks (note: task functions cannot be persisted and must be re-registered)
	for id, pt := range state.Tasks {
		task := &QueueTask{
			ID:           pt.ID,
			Priority:     pt.Priority,
			Dependencies: pt.Dependencies,
			RetryCount:   pt.RetryCount,
			MaxRetries:   pt.MaxRetries,
			Status:       pt.Status,
			CreatedAt:    pt.CreatedAt,
			StartedAt:    pt.StartedAt,
			CompletedAt:  pt.CompletedAt,
			LastError:    pt.LastError,
			Metadata:     pt.Metadata,
		}

		// Look up registered function
		if fn, exists := tq.taskRegistry[id]; exists {
			task.Func = fn
		}

		tq.tasks[id] = task

		// Restore dependency relationships
		if len(task.Dependencies) > 0 {
			tq.dependencyResolver.AddTask(task.ID, task.Dependencies)
		}

		// Mark completed tasks in dependency resolver
		if task.Status == TaskStatusCompleted {
			tq.dependencyResolver.MarkCompleted(task.ID)
		}
	}

	log.InfoLog.Printf("loaded %d tasks from persistence", len(tq.tasks))
	return nil
}

// ClearCompleted removes all completed tasks from the queue
func (tq *TaskQueue) ClearCompleted() int {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	count := 0
	for id, task := range tq.tasks {
		if task.Status == TaskStatusCompleted {
			delete(tq.tasks, id)
			count++
		}
	}

	if count > 0 {
		if err := tq.persistState(); err != nil {
			log.WarningLog.Printf("failed to persist state after clearing completed tasks: %v", err)
		}
	}

	return count
}

// RetryFailedTask attempts to retry a failed task
func (tq *TaskQueue) RetryFailedTask(taskID string) error {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	task, exists := tq.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != TaskStatusFailed {
		return fmt.Errorf("task %s is not in failed state (current: %s)", taskID, task.Status)
	}

	// Reset task for retry
	task.Status = TaskStatusPending
	task.RetryCount = 0
	task.LastError = ""
	task.StartedAt = nil
	task.CompletedAt = nil

	// Re-enqueue if dependencies are satisfied
	if tq.dependencyResolver.CanExecute(task.ID) {
		select {
		case tq.priorityQueues[task.Priority] <- task:
			log.InfoLog.Printf("failed task %s requeued for retry", task.ID)
		default:
			return fmt.Errorf("priority queue for %s is full", task.Priority)
		}
	}

	return nil
}
