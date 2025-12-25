package orchestrator

import (
	"claude-squad/log"
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	// MaxConcurrentAgents is the maximum number of concurrent agents
	MaxConcurrentAgents = 10

	// StatusPending indicates a task is waiting to be executed
	StatusPending = "pending"

	// StatusRunning indicates a task is currently being executed
	StatusRunning = "running"

	// StatusCompleted indicates a task completed successfully
	StatusCompleted = "completed"

	// StatusFailed indicates a task failed
	StatusFailed = "failed"
)

// AgentExecutor defines the interface for executing agent tasks
type AgentExecutor interface {
	Execute(ctx context.Context, task *Task) (*string, error)
}

// AgentPool manages concurrent agent execution with intelligent task distribution
type AgentPool struct {
	client        *Client
	executor      AgentExecutor
	maxConcurrent int
	activeSlots   chan struct{}
	wg            sync.WaitGroup
	mu            sync.RWMutex
	running       map[string]context.CancelFunc
	stopChan      chan struct{}
	pollingTicker *time.Ticker
}

// NewAgentPool creates a new agent pool with Oxigraph-backed task management
func NewAgentPool(orchestratorURL string, executor AgentExecutor) (*AgentPool, error) {
	client := NewClient(orchestratorURL)

	// Health check
	if err := client.Health(); err != nil {
		return nil, fmt.Errorf("orchestrator service not healthy: %w", err)
	}

	pool := &AgentPool{
		client:        client,
		executor:      executor,
		maxConcurrent: MaxConcurrentAgents,
		activeSlots:   make(chan struct{}, MaxConcurrentAgents),
		running:       make(map[string]context.CancelFunc),
		stopChan:      make(chan struct{}),
		pollingTicker: time.NewTicker(2 * time.Second),
	}

	// Initialize all slots as available
	for i := 0; i < MaxConcurrentAgents; i++ {
		pool.activeSlots <- struct{}{}
	}

	return pool, nil
}

// Start begins the agent pool's task processing loop
func (p *AgentPool) Start(ctx context.Context) error {
	log.InfoLog.Printf("Starting agent pool with max concurrency: %d", p.maxConcurrent)

	go p.processLoop(ctx)

	return nil
}

// Stop gracefully shuts down the agent pool
func (p *AgentPool) Stop() {
	log.InfoLog.Println("Stopping agent pool...")

	close(p.stopChan)
	p.pollingTicker.Stop()

	// Cancel all running tasks
	p.mu.Lock()
	for taskID, cancel := range p.running {
		log.InfoLog.Printf("Cancelling task %s", taskID)
		cancel()
	}
	p.mu.Unlock()

	// Wait for all goroutines to finish
	p.wg.Wait()

	log.InfoLog.Println("Agent pool stopped")
}

// processLoop continuously polls for tasks and executes them
func (p *AgentPool) processLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		case <-p.pollingTicker.C:
			p.processAvailableTasks(ctx)
		}
	}
}

// processAvailableTasks retrieves and executes ready tasks
func (p *AgentPool) processAvailableTasks(ctx context.Context) {
	// Get current analytics
	analytics, err := p.client.GetAnalytics()
	if err != nil {
		log.ErrorLog.Printf("Failed to get analytics: %v", err)
		return
	}

	if analytics.AvailableSlots <= 0 {
		// All slots are busy
		return
	}

	// Get optimized task distribution
	taskIDs, err := p.client.OptimizeDistribution()
	if err != nil {
		log.ErrorLog.Printf("Failed to optimize distribution: %v", err)
		return
	}

	if len(taskIDs) == 0 {
		// No tasks ready to execute
		return
	}

	log.InfoLog.Printf("Found %d optimized tasks to execute (available slots: %d)",
		len(taskIDs), analytics.AvailableSlots)

	// Launch tasks up to available capacity
	for _, taskID := range taskIDs {
		select {
		case <-p.activeSlots:
			// Got a slot, launch the task
			p.wg.Add(1)
			go p.executeTask(ctx, taskID)
		default:
			// No more slots available
			return
		}
	}
}

// executeTask executes a single task
func (p *AgentPool) executeTask(ctx context.Context, taskID string) {
	defer func() {
		p.wg.Done()
		// Release the slot
		p.activeSlots <- struct{}{}

		// Remove from running map
		p.mu.Lock()
		delete(p.running, taskID)
		p.mu.Unlock()
	}()

	// Create cancellable context for this task
	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Register in running map
	p.mu.Lock()
	p.running[taskID] = cancel
	p.mu.Unlock()

	log.InfoLog.Printf("Executing task %s", taskID)

	// Update status to running
	if err := p.client.UpdateTaskStatus(taskID, StatusRunning, nil); err != nil {
		log.ErrorLog.Printf("Failed to update task %s to running: %v", taskID, err)
		return
	}

	// Get task details (we need the full task for execution)
	// For now, create a minimal task - in production, add GetTask endpoint
	task := &Task{
		ID: taskID,
	}

	// Execute the task
	result, err := p.executor.Execute(taskCtx, task)

	if err != nil {
		log.ErrorLog.Printf("Task %s failed: %v", taskID, err)
		errorMsg := err.Error()
		if updateErr := p.client.UpdateTaskStatus(taskID, StatusFailed, &errorMsg); updateErr != nil {
			log.ErrorLog.Printf("Failed to update task %s to failed: %v", taskID, updateErr)
		}
		return
	}

	// Task completed successfully
	log.InfoLog.Printf("Task %s completed successfully", taskID)
	if err := p.client.UpdateTaskStatus(taskID, StatusCompleted, result); err != nil {
		log.ErrorLog.Printf("Failed to update task %s to completed: %v", taskID, err)
	}
}

// SubmitTask submits a new task to the orchestrator
func (p *AgentPool) SubmitTask(task *Task) (string, error) {
	if task.Status == "" {
		task.Status = StatusPending
	}

	if task.CreatedAt == "" {
		task.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	taskID, err := p.client.CreateTask(task)
	if err != nil {
		return "", fmt.Errorf("failed to create task: %w", err)
	}

	log.InfoLog.Printf("Submitted task %s: %s", taskID, task.Description)
	return taskID, nil
}

// GetAnalytics returns current pool analytics
func (p *AgentPool) GetAnalytics() (*Analytics, error) {
	return p.client.GetAnalytics()
}

// GetTaskChain returns the dependency chain for a task
func (p *AgentPool) GetTaskChain(taskID string) ([]DependencyChain, error) {
	return p.client.GetTaskChain(taskID)
}

// CancelTask cancels a running task
func (p *AgentPool) CancelTask(taskID string) error {
	p.mu.RLock()
	cancel, exists := p.running[taskID]
	p.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task %s is not running", taskID)
	}

	log.InfoLog.Printf("Cancelling task %s", taskID)
	cancel()

	return nil
}

// GetRunningCount returns the number of currently running tasks
func (p *AgentPool) GetRunningCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.running)
}

// GetAvailableSlots returns the number of available execution slots
func (p *AgentPool) GetAvailableSlots() int {
	return len(p.activeSlots)
}

// WaitForCompletion waits for all tasks to complete or context to be cancelled
func (p *AgentPool) WaitForCompletion(ctx context.Context) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			analytics, err := p.client.GetAnalytics()
			if err != nil {
				return fmt.Errorf("failed to get analytics: %w", err)
			}

			if analytics.RunningCount == 0 && analytics.StatusCounts[StatusPending] == 0 {
				log.InfoLog.Println("All tasks completed")
				return nil
			}

			log.InfoLog.Printf("Waiting for completion: %d running, %d pending",
				analytics.RunningCount, analytics.StatusCounts[StatusPending])
		}
	}
}
