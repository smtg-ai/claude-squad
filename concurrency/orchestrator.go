package concurrency

import (
	"claude-squad/log"
	"claude-squad/session"
	"context"
	"fmt"
	"sync"
	"time"
)

// AgentState represents the current state of an agent
type AgentState int

const (
	// AgentStateIdle indicates the agent is idle and ready for tasks
	AgentStateIdle AgentState = iota
	// AgentStateRunning indicates the agent is currently executing a task
	AgentStateRunning
	// AgentStatePaused indicates the agent is paused
	AgentStatePaused
	// AgentStateFailed indicates the agent has failed health checks
	AgentStateFailed
	// AgentStateStopped indicates the agent has been stopped
	AgentStateStopped
)

// String returns the string representation of AgentState
func (s AgentState) String() string {
	switch s {
	case AgentStateIdle:
		return "Idle"
	case AgentStateRunning:
		return "Running"
	case AgentStatePaused:
		return "Paused"
	case AgentStateFailed:
		return "Failed"
	case AgentStateStopped:
		return "Stopped"
	default:
		return "Unknown"
	}
}

// TaskPriority represents the priority level of a task
type TaskPriority int

const (
	// TaskPriorityLow for background tasks
	TaskPriorityLow TaskPriority = iota
	// TaskPriorityNormal for standard tasks
	TaskPriorityNormal
	// TaskPriorityHigh for urgent tasks
	TaskPriorityHigh
	// TaskPriorityCritical for critical tasks that must be executed immediately
	TaskPriorityCritical
)

// Task represents a unit of work to be distributed to agents
type Task struct {
	// ID is the unique identifier for the task
	ID string
	// Prompt is the prompt to send to the agent
	Prompt string
	// Priority indicates the importance of the task
	Priority TaskPriority
	// Affinity specifies preferred agent IDs for this task
	Affinity []string
	// Timeout is the maximum duration for task execution
	Timeout time.Duration
	// Metadata stores additional task-specific data
	Metadata map[string]interface{}
	// ResultChan is the channel to send the task result
	ResultChan chan *TaskResult
}

// TaskResult represents the outcome of task execution
type TaskResult struct {
	// TaskID is the ID of the completed task
	TaskID string
	// AgentID is the ID of the agent that executed the task
	AgentID string
	// Success indicates whether the task completed successfully
	Success bool
	// Output contains the task output or error message
	Output string
	// Error contains any error that occurred during execution
	Error error
	// Duration is how long the task took to execute
	Duration time.Duration
	// CompletedAt is when the task completed
	CompletedAt time.Time
}

// AgentEvent represents events in the agent lifecycle
type AgentEvent struct {
	// AgentID is the ID of the agent that generated the event
	AgentID string
	// Type is the event type (StateChange, HealthCheckFailed, TaskCompleted, etc.)
	Type string
	// Timestamp is when the event occurred
	Timestamp time.Time
	// Data contains event-specific data
	Data map[string]interface{}
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	// CircuitClosed indicates normal operation
	CircuitClosed CircuitBreakerState = iota
	// CircuitOpen indicates the agent is failing and should not receive tasks
	CircuitOpen
	// CircuitHalfOpen indicates the agent is being tested for recovery
	CircuitHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern for agent failure handling
type CircuitBreaker struct {
	mu sync.RWMutex

	// Configuration
	maxFailures   int           // Maximum consecutive failures before opening
	resetTimeout  time.Duration // Time to wait before attempting recovery
	halfOpenTests int           // Number of successful tests needed to close

	// State
	state            CircuitBreakerState
	failures         int
	lastFailureTime  time.Time
	consecutiveTests int
}

// NewCircuitBreaker creates a new circuit breaker with the given parameters
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration, halfOpenTests int) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:   maxFailures,
		resetTimeout:  resetTimeout,
		halfOpenTests: halfOpenTests,
		state:         CircuitClosed,
	}
}

// CanExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailureTime) >= cb.resetTimeout {
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful execution
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitHalfOpen:
		cb.consecutiveTests++
		if cb.consecutiveTests >= cb.halfOpenTests {
			// Enough successful tests, close the circuit
			cb.state = CircuitClosed
			cb.failures = 0
			cb.consecutiveTests = 0
			log.InfoLog.Printf("Circuit breaker closed after successful recovery tests")
		}
	case CircuitClosed:
		cb.failures = 0
	}
}

// RecordFailure records a failed execution
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitClosed:
		if cb.failures >= cb.maxFailures {
			cb.state = CircuitOpen
			log.WarningLog.Printf("Circuit breaker opened after %d failures", cb.failures)
		}
	case CircuitHalfOpen:
		// Failed during testing, reopen the circuit
		cb.state = CircuitOpen
		cb.consecutiveTests = 0
		log.WarningLog.Printf("Circuit breaker reopened after test failure")
	}
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// TransitionToHalfOpen attempts to transition to half-open state
func (cb *CircuitBreaker) TransitionToHalfOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == CircuitOpen && time.Since(cb.lastFailureTime) >= cb.resetTimeout {
		cb.state = CircuitHalfOpen
		cb.consecutiveTests = 0
		log.InfoLog.Printf("Circuit breaker transitioned to half-open for testing")
		return true
	}
	return false
}

// ManagedAgent wraps a session.Instance with additional orchestration metadata
type ManagedAgent struct {
	mu sync.RWMutex

	// Core instance
	instance *session.Instance

	// Agent metadata
	id        string
	state     AgentState
	createdAt time.Time
	updatedAt time.Time

	// Health monitoring
	lastHealthCheck time.Time
	healthCheckOK   bool
	circuitBreaker  *CircuitBreaker

	// Task management
	currentTask     *Task
	taskStartTime   time.Time
	tasksCompleted  int
	tasksFailed     int
	totalExecTime   time.Duration
	avgResponseTime time.Duration

	// Load metrics
	loadScore float64 // 0.0 (idle) to 1.0 (fully loaded)

	// Control channels
	stopChan chan struct{}
	stopped  bool
}

// NewManagedAgent creates a new managed agent wrapping a session instance
func NewManagedAgent(id string, instance *session.Instance) *ManagedAgent {
	now := time.Now()
	return &ManagedAgent{
		id:             id,
		instance:       instance,
		state:          AgentStateIdle,
		createdAt:      now,
		updatedAt:      now,
		circuitBreaker: NewCircuitBreaker(3, 30*time.Second, 2),
		stopChan:       make(chan struct{}),
		loadScore:      0.0,
	}
}

// GetID returns the agent's unique identifier
func (a *ManagedAgent) GetID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.id
}

// GetState returns the current agent state
func (a *ManagedAgent) GetState() AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// SetState updates the agent state and timestamp
func (a *ManagedAgent) SetState(state AgentState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = state
	a.updatedAt = time.Now()
}

// GetLoadScore returns the current load score (0.0 to 1.0)
func (a *ManagedAgent) GetLoadScore() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.loadScore
}

// UpdateLoadScore calculates and updates the agent's load score
func (a *ManagedAgent) UpdateLoadScore() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Calculate load based on state and task history
	baseLoad := 0.0
	switch a.state {
	case AgentStateIdle:
		baseLoad = 0.0
	case AgentStateRunning:
		baseLoad = 0.8
	case AgentStatePaused:
		baseLoad = 0.5
	case AgentStateFailed:
		baseLoad = 1.0 // Max load to prevent assignment
	case AgentStateStopped:
		baseLoad = 1.0
	}

	// Adjust based on task execution time
	if a.currentTask != nil && !a.taskStartTime.IsZero() {
		elapsed := time.Since(a.taskStartTime)
		if a.currentTask.Timeout > 0 {
			timeoutFactor := float64(elapsed) / float64(a.currentTask.Timeout)
			if timeoutFactor > 1.0 {
				timeoutFactor = 1.0
			}
			baseLoad += timeoutFactor * 0.2
		}
	}

	// Adjust based on circuit breaker state
	cbState := a.circuitBreaker.GetState()
	if cbState == CircuitOpen {
		baseLoad = 1.0
	} else if cbState == CircuitHalfOpen {
		baseLoad += 0.3
	}

	// Clamp to [0.0, 1.0]
	if baseLoad > 1.0 {
		baseLoad = 1.0
	}
	if baseLoad < 0.0 {
		baseLoad = 0.0
	}

	a.loadScore = baseLoad
}

// IsHealthy checks if the agent is healthy
func (a *ManagedAgent) IsHealthy() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.healthCheckOK && a.circuitBreaker.GetState() != CircuitOpen
}

// PerformHealthCheck executes a health check on the agent
func (a *ManagedAgent) PerformHealthCheck() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lastHealthCheck = time.Now()

	// Check if instance is started and alive
	if !a.instance.Started() {
		a.healthCheckOK = false
		return fmt.Errorf("instance not started")
	}

	if !a.instance.TmuxAlive() {
		a.healthCheckOK = false
		return fmt.Errorf("tmux session not alive")
	}

	// Check for paused or failed states
	if a.state == AgentStatePaused || a.state == AgentStateFailed || a.state == AgentStateStopped {
		a.healthCheckOK = false
		return fmt.Errorf("agent in unhealthy state: %s", a.state.String())
	}

	a.healthCheckOK = true
	return nil
}

// ExecuteTask executes a task on this agent
func (a *ManagedAgent) ExecuteTask(ctx context.Context, task *Task) *TaskResult {
	startTime := time.Now()
	result := &TaskResult{
		TaskID:  task.ID,
		AgentID: a.GetID(),
		Success: false,
	}

	// Update agent state
	a.mu.Lock()
	a.currentTask = task
	a.taskStartTime = startTime
	a.state = AgentStateRunning
	a.updatedAt = time.Now()
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.currentTask = nil
		a.taskStartTime = time.Time{}
		a.state = AgentStateIdle
		a.updatedAt = time.Now()
		duration := time.Since(startTime)
		a.totalExecTime += duration
		if result.Success {
			a.tasksCompleted++
			a.circuitBreaker.RecordSuccess()
		} else {
			a.tasksFailed++
			a.circuitBreaker.RecordFailure()
		}
		// Update average response time
		totalTasks := a.tasksCompleted + a.tasksFailed
		if totalTasks > 0 {
			a.avgResponseTime = a.totalExecTime / time.Duration(totalTasks)
		}
		a.mu.Unlock()
	}()

	// Create execution context with timeout
	execCtx := ctx
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	// Execute task with timeout
	errChan := make(chan error, 1)
	go func() {
		err := a.instance.SendPrompt(task.Prompt)
		select {
		case errChan <- err:
		case <-execCtx.Done():
			// Context cancelled, don't block on send
		}
	}()

	select {
	case err := <-errChan:
		if err != nil {
			result.Error = fmt.Errorf("failed to send prompt: %w", err)
			result.Output = err.Error()
			return result
		}

		// Wait for task completion or timeout
		// In a real implementation, you'd monitor the instance for completion
		// For now, we'll simulate by checking instance status
		time.Sleep(100 * time.Millisecond)

		output, err := a.instance.Preview()
		if err != nil {
			result.Error = fmt.Errorf("failed to get preview: %w", err)
			result.Output = err.Error()
			return result
		}

		result.Success = true
		result.Output = output
		result.Duration = time.Since(startTime)
		result.CompletedAt = time.Now()
		return result

	case <-execCtx.Done():
		result.Error = fmt.Errorf("task execution timeout")
		result.Output = "Task timed out"
		result.Duration = time.Since(startTime)
		result.CompletedAt = time.Now()
		return result
	}
}

// Pause pauses the agent
func (a *ManagedAgent) Pause() error {
	a.mu.Lock()
	if a.state == AgentStatePaused {
		a.mu.Unlock()
		return fmt.Errorf("agent already paused")
	}
	a.mu.Unlock()

	if err := a.instance.Pause(); err != nil {
		return fmt.Errorf("failed to pause instance: %w", err)
	}

	a.SetState(AgentStatePaused)
	return nil
}

// Resume resumes a paused agent
func (a *ManagedAgent) Resume() error {
	a.mu.Lock()
	if a.state != AgentStatePaused {
		a.mu.Unlock()
		return fmt.Errorf("agent not paused, current state: %s", a.state.String())
	}
	a.mu.Unlock()

	if err := a.instance.Resume(); err != nil {
		return fmt.Errorf("failed to resume instance: %w", err)
	}

	a.SetState(AgentStateIdle)
	return nil
}

// Stop stops the agent and cleans up resources
func (a *ManagedAgent) Stop() error {
	a.mu.Lock()
	if a.stopped {
		a.mu.Unlock()
		return nil
	}
	a.stopped = true
	close(a.stopChan)
	a.mu.Unlock()

	if err := a.instance.Kill(); err != nil {
		return fmt.Errorf("failed to kill instance: %w", err)
	}

	a.SetState(AgentStateStopped)
	return nil
}

// GetStats returns agent statistics
func (a *ManagedAgent) GetStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"id":                a.id,
		"state":             a.state.String(),
		"tasks_completed":   a.tasksCompleted,
		"tasks_failed":      a.tasksFailed,
		"avg_response_time": a.avgResponseTime.String(),
		"total_exec_time":   a.totalExecTime.String(),
		"load_score":        a.loadScore,
		"health_ok":         a.healthCheckOK,
		"circuit_state":     a.circuitBreaker.GetState(),
		"last_health_check": a.lastHealthCheck,
		"created_at":        a.createdAt,
		"updated_at":        a.updatedAt,
	}
}

// OrchestratorConfig defines configuration for the orchestrator
type OrchestratorConfig struct {
	// MaxConcurrentTasks limits the number of tasks that can run concurrently
	MaxConcurrentTasks int
	// HealthCheckInterval is how often to perform health checks on agents
	HealthCheckInterval time.Duration
	// TaskQueueSize is the size of the task queue buffer
	TaskQueueSize int
	// EventBufferSize is the size of the event channel buffer
	EventBufferSize int
	// EnableAutoRecovery enables automatic recovery of failed agents
	EnableAutoRecovery bool
	// LoadBalancingAlgorithm specifies the load balancing strategy
	LoadBalancingAlgorithm string // "round-robin", "least-loaded", "random"
}

// DefaultOrchestratorConfig returns a default configuration
func DefaultOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		MaxConcurrentTasks:     10,
		HealthCheckInterval:    30 * time.Second,
		TaskQueueSize:          100,
		EventBufferSize:        100,
		EnableAutoRecovery:     true,
		LoadBalancingAlgorithm: "least-loaded",
	}
}

// AgentOrchestrator manages multiple AI agents with load balancing and health monitoring
type AgentOrchestrator struct {
	mu     sync.RWMutex
	config *OrchestratorConfig

	// Agent management
	agents          map[string]*ManagedAgent
	agentIDs        []string // Ordered list for round-robin
	roundRobinIndex int

	// Task management
	taskQueue     chan *Task
	activeTasks   map[string]*Task
	taskSemaphore chan struct{} // Limits concurrent tasks

	// Event system
	eventChan chan *AgentEvent

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics
	totalTasksDistributed int
	totalTasksCompleted   int
	totalTasksFailed      int
	startTime             time.Time
}

// NewOrchestrator creates a new agent orchestrator with the given configuration
func NewOrchestrator(config *OrchestratorConfig) *AgentOrchestrator {
	if config == nil {
		config = DefaultOrchestratorConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	o := &AgentOrchestrator{
		config:        config,
		agents:        make(map[string]*ManagedAgent),
		agentIDs:      make([]string, 0),
		taskQueue:     make(chan *Task, config.TaskQueueSize),
		activeTasks:   make(map[string]*Task),
		taskSemaphore: make(chan struct{}, config.MaxConcurrentTasks),
		eventChan:     make(chan *AgentEvent, config.EventBufferSize),
		ctx:           ctx,
		cancel:        cancel,
		startTime:     time.Now(),
	}

	// Start background workers
	o.startWorkers()

	return o
}

// AddAgent registers a new agent with the orchestrator
func (o *AgentOrchestrator) AddAgent(agent *ManagedAgent) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.agents[agent.GetID()]; exists {
		return fmt.Errorf("agent %s already exists", agent.GetID())
	}

	o.agents[agent.GetID()] = agent
	o.agentIDs = append(o.agentIDs, agent.GetID())

	o.publishEvent(&AgentEvent{
		AgentID:   agent.GetID(),
		Type:      "AgentAdded",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"agent_id": agent.GetID(),
		},
	})

	log.InfoLog.Printf("Agent %s added to orchestrator", agent.GetID())
	return nil
}

// RemoveAgent removes an agent from the orchestrator
func (o *AgentOrchestrator) RemoveAgent(agentID string) error {
	o.mu.Lock()
	agent, exists := o.agents[agentID]
	if !exists {
		o.mu.Unlock()
		return fmt.Errorf("agent %s not found", agentID)
	}

	delete(o.agents, agentID)
	// Remove from agentIDs slice
	for i, id := range o.agentIDs {
		if id == agentID {
			o.agentIDs = append(o.agentIDs[:i], o.agentIDs[i+1:]...)
			break
		}
	}
	o.mu.Unlock()

	// Stop the agent
	if err := agent.Stop(); err != nil {
		log.ErrorLog.Printf("Error stopping agent %s: %v", agentID, err)
	}

	o.publishEvent(&AgentEvent{
		AgentID:   agentID,
		Type:      "AgentRemoved",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"agent_id": agentID,
		},
	})

	log.InfoLog.Printf("Agent %s removed from orchestrator", agentID)
	return nil
}

// GetAgent returns an agent by ID
func (o *AgentOrchestrator) GetAgent(agentID string) (*ManagedAgent, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	agent, exists := o.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}

	return agent, nil
}

// ListAgents returns a list of all agent IDs
func (o *AgentOrchestrator) ListAgents() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make([]string, len(o.agentIDs))
	copy(result, o.agentIDs)
	return result
}

// DistributeTask distributes a task to an appropriate agent
func (o *AgentOrchestrator) DistributeTask(task *Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	if task.ID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}

	o.mu.Lock()
	o.totalTasksDistributed++
	o.activeTasks[task.ID] = task
	o.mu.Unlock()

	// Add task to queue
	select {
	case o.taskQueue <- task:
		log.InfoLog.Printf("Task %s queued for distribution", task.ID)
		return nil
	case <-o.ctx.Done():
		return fmt.Errorf("orchestrator is shutting down")
	default:
		// Queue is full
		o.mu.Lock()
		delete(o.activeTasks, task.ID)
		o.mu.Unlock()
		return fmt.Errorf("task queue is full, cannot accept task %s", task.ID)
	}
}

// selectAgent selects the best agent for a task based on load balancing algorithm
func (o *AgentOrchestrator) selectAgent(task *Task) (*ManagedAgent, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if len(o.agents) == 0 {
		return nil, fmt.Errorf("no agents available")
	}

	// First, try affinity-based selection
	if len(task.Affinity) > 0 {
		for _, agentID := range task.Affinity {
			if agent, exists := o.agents[agentID]; exists {
				if agent.GetState() == AgentStateIdle && agent.IsHealthy() && agent.circuitBreaker.CanExecute() {
					log.InfoLog.Printf("Selected agent %s based on affinity for task %s", agentID, task.ID)
					return agent, nil
				}
			}
		}
	}

	// Use configured load balancing algorithm
	switch o.config.LoadBalancingAlgorithm {
	case "round-robin":
		return o.selectAgentRoundRobin()
	case "least-loaded":
		return o.selectAgentLeastLoaded()
	case "random":
		return o.selectAgentRandom()
	default:
		return o.selectAgentLeastLoaded()
	}
}

// selectAgentRoundRobin selects an agent using round-robin algorithm
func (o *AgentOrchestrator) selectAgentRoundRobin() (*ManagedAgent, error) {
	startIndex := o.roundRobinIndex
	for i := 0; i < len(o.agentIDs); i++ {
		idx := (startIndex + i) % len(o.agentIDs)
		agentID := o.agentIDs[idx]
		agent := o.agents[agentID]

		if agent.GetState() == AgentStateIdle && agent.IsHealthy() && agent.circuitBreaker.CanExecute() {
			o.roundRobinIndex = (idx + 1) % len(o.agentIDs)
			return agent, nil
		}
	}
	return nil, fmt.Errorf("no healthy idle agents available")
}

// selectAgentLeastLoaded selects the agent with the lowest load score
func (o *AgentOrchestrator) selectAgentLeastLoaded() (*ManagedAgent, error) {
	var bestAgent *ManagedAgent
	minLoad := 2.0 // Higher than max possible load

	for _, agent := range o.agents {
		agent.UpdateLoadScore()
		if agent.GetState() == AgentStateIdle && agent.IsHealthy() && agent.circuitBreaker.CanExecute() {
			load := agent.GetLoadScore()
			if load < minLoad {
				minLoad = load
				bestAgent = agent
			}
		}
	}

	if bestAgent == nil {
		return nil, fmt.Errorf("no healthy idle agents available")
	}

	return bestAgent, nil
}

// selectAgentRandom selects a random healthy idle agent
func (o *AgentOrchestrator) selectAgentRandom() (*ManagedAgent, error) {
	// Collect eligible agents
	var eligible []*ManagedAgent
	for _, agent := range o.agents {
		if agent.GetState() == AgentStateIdle && agent.IsHealthy() && agent.circuitBreaker.CanExecute() {
			eligible = append(eligible, agent)
		}
	}

	if len(eligible) == 0 {
		return nil, fmt.Errorf("no healthy idle agents available")
	}

	// Use current time as pseudo-random source
	idx := int(time.Now().UnixNano()) % len(eligible)
	return eligible[idx], nil
}

// startWorkers starts background worker goroutines
func (o *AgentOrchestrator) startWorkers() {
	// Task distribution worker
	o.wg.Add(1)
	go o.taskDistributionWorker()

	// Health check worker
	o.wg.Add(1)
	go o.healthCheckWorker()

	// Recovery worker (if enabled)
	if o.config.EnableAutoRecovery {
		o.wg.Add(1)
		go o.recoveryWorker()
	}
}

// taskDistributionWorker processes tasks from the queue
func (o *AgentOrchestrator) taskDistributionWorker() {
	defer o.wg.Done()

	for {
		select {
		case <-o.ctx.Done():
			log.InfoLog.Println("Task distribution worker shutting down")
			return

		case task := <-o.taskQueue:
			// Acquire semaphore slot
			select {
			case o.taskSemaphore <- struct{}{}:
				// Got a slot, execute task
				o.wg.Add(1)
				go o.executeTask(task)
			case <-o.ctx.Done():
				return
			}
		}
	}
}

// executeTask executes a task on a selected agent
func (o *AgentOrchestrator) executeTask(task *Task) {
	defer o.wg.Done()
	defer func() {
		<-o.taskSemaphore // Release semaphore slot
		// Always cleanup activeTasks to prevent memory leak
		o.mu.Lock()
		delete(o.activeTasks, task.ID)
		o.mu.Unlock()
		log.InfoLog.Printf("Task %s removed from activeTasks map", task.ID)
	}()

	// Select agent
	agent, err := o.selectAgent(task)
	if err != nil {
		log.ErrorLog.Printf("Failed to select agent for task %s: %v", task.ID, err)
		result := &TaskResult{
			TaskID:      task.ID,
			Success:     false,
			Error:       err,
			Output:      fmt.Sprintf("Failed to select agent: %v", err),
			CompletedAt: time.Now(),
		}
		o.sendTaskResult(task, result)
		o.mu.Lock()
		o.totalTasksFailed++
		o.mu.Unlock()
		// Cleanup happens in defer above
		return
	}

	log.InfoLog.Printf("Executing task %s on agent %s", task.ID, agent.GetID())

	// Execute task
	result := agent.ExecuteTask(o.ctx, task)

	// Send result
	o.sendTaskResult(task, result)

	// Update metrics
	o.mu.Lock()
	if result.Success {
		o.totalTasksCompleted++
	} else {
		o.totalTasksFailed++
	}
	o.mu.Unlock()

	// Publish event
	o.publishEvent(&AgentEvent{
		AgentID:   agent.GetID(),
		Type:      "TaskCompleted",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"task_id":  task.ID,
			"success":  result.Success,
			"duration": result.Duration.String(),
		},
	})
}

// sendTaskResult sends the task result to the result channel
func (o *AgentOrchestrator) sendTaskResult(task *Task, result *TaskResult) {
	if task.ResultChan != nil {
		select {
		case task.ResultChan <- result:
			// Result sent successfully
		case <-time.After(5 * time.Second):
			log.WarningLog.Printf("Timeout sending result for task %s", task.ID)
		case <-o.ctx.Done():
			return
		}
	}
}

// healthCheckWorker periodically performs health checks on all agents
func (o *AgentOrchestrator) healthCheckWorker() {
	defer o.wg.Done()

	ticker := time.NewTicker(o.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			log.InfoLog.Println("Health check worker shutting down")
			return

		case <-ticker.C:
			o.performHealthChecks()
		}
	}
}

// performHealthChecks performs health checks on all agents
func (o *AgentOrchestrator) performHealthChecks() {
	o.mu.RLock()
	agents := make([]*ManagedAgent, 0, len(o.agents))
	for _, agent := range o.agents {
		agents = append(agents, agent)
	}
	o.mu.RUnlock()

	for _, agent := range agents {
		if err := agent.PerformHealthCheck(); err != nil {
			log.ErrorLog.Printf("Health check failed for agent %s: %v", agent.GetID(), err)

			o.publishEvent(&AgentEvent{
				AgentID:   agent.GetID(),
				Type:      "HealthCheckFailed",
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"error": err.Error(),
				},
			})

			// Mark agent as failed if circuit is open
			if agent.circuitBreaker.GetState() == CircuitOpen {
				agent.SetState(AgentStateFailed)
			}
		} else {
			// Update load score on successful health check
			agent.UpdateLoadScore()
		}
	}
}

// recoveryWorker attempts to recover failed agents
func (o *AgentOrchestrator) recoveryWorker() {
	defer o.wg.Done()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			log.InfoLog.Println("Recovery worker shutting down")
			return

		case <-ticker.C:
			o.attemptRecovery()
		}
	}
}

// attemptRecovery attempts to recover failed agents
func (o *AgentOrchestrator) attemptRecovery() {
	o.mu.RLock()
	agents := make([]*ManagedAgent, 0, len(o.agents))
	for _, agent := range o.agents {
		agents = append(agents, agent)
	}
	o.mu.RUnlock()

	for _, agent := range agents {
		if agent.GetState() == AgentStateFailed {
			// Try to transition circuit breaker to half-open
			if agent.circuitBreaker.TransitionToHalfOpen() {
				log.InfoLog.Printf("Attempting recovery for agent %s", agent.GetID())

				// Perform health check
				if err := agent.PerformHealthCheck(); err != nil {
					log.ErrorLog.Printf("Recovery health check failed for agent %s: %v", agent.GetID(), err)
					agent.circuitBreaker.RecordFailure()
				} else {
					log.InfoLog.Printf("Agent %s recovered successfully", agent.GetID())
					agent.SetState(AgentStateIdle)
					agent.circuitBreaker.RecordSuccess()

					o.publishEvent(&AgentEvent{
						AgentID:   agent.GetID(),
						Type:      "AgentRecovered",
						Timestamp: time.Now(),
						Data:      map[string]interface{}{},
					})
				}
			}
		}
	}
}

// publishEvent publishes an event to the event channel
func (o *AgentOrchestrator) publishEvent(event *AgentEvent) {
	select {
	case o.eventChan <- event:
		// Event published
	default:
		// Event channel full, log warning
		log.WarningLog.Printf("Event channel full, dropping event: %s for agent %s", event.Type, event.AgentID)
	}
}

// EventChannel returns the event channel for subscribing to orchestrator events
func (o *AgentOrchestrator) EventChannel() <-chan *AgentEvent {
	return o.eventChan
}

// GetMetrics returns orchestrator metrics
func (o *AgentOrchestrator) GetMetrics() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	uptime := time.Since(o.startTime)
	activeAgents := 0
	idleAgents := 0
	failedAgents := 0

	for _, agent := range o.agents {
		switch agent.GetState() {
		case AgentStateIdle:
			idleAgents++
			activeAgents++
		case AgentStateRunning:
			activeAgents++
		case AgentStateFailed:
			failedAgents++
		}
	}

	return map[string]interface{}{
		"total_agents":            len(o.agents),
		"active_agents":           activeAgents,
		"idle_agents":             idleAgents,
		"failed_agents":           failedAgents,
		"active_tasks":            len(o.activeTasks),
		"queued_tasks":            len(o.taskQueue),
		"total_tasks_distributed": o.totalTasksDistributed,
		"total_tasks_completed":   o.totalTasksCompleted,
		"total_tasks_failed":      o.totalTasksFailed,
		"uptime":                  uptime.String(),
		"max_concurrent_tasks":    o.config.MaxConcurrentTasks,
	}
}

// Shutdown gracefully shuts down the orchestrator
func (o *AgentOrchestrator) Shutdown(timeout time.Duration) error {
	log.InfoLog.Println("Shutting down orchestrator...")

	// Cancel context to stop workers
	o.cancel()

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		o.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.InfoLog.Println("All workers stopped successfully")
	case <-time.After(timeout):
		log.ErrorLog.Println("Timeout waiting for workers to stop")
		return fmt.Errorf("shutdown timeout exceeded")
	}

	// Clean up any remaining active tasks to prevent memory leak
	o.mu.Lock()
	remainingTasks := len(o.activeTasks)
	if remainingTasks > 0 {
		log.WarningLog.Printf("Cleaning up %d active tasks that were not processed", remainingTasks)
		for taskID := range o.activeTasks {
			delete(o.activeTasks, taskID)
		}
	}
	o.mu.Unlock()

	// Stop all agents
	o.mu.RLock()
	agents := make([]*ManagedAgent, 0, len(o.agents))
	for _, agent := range o.agents {
		agents = append(agents, agent)
	}
	o.mu.RUnlock()

	var errs []error
	for _, agent := range agents {
		if err := agent.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop agent %s: %w", agent.GetID(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}

	log.InfoLog.Println("Orchestrator shutdown complete")
	return nil
}

// PauseAgent pauses a specific agent
func (o *AgentOrchestrator) PauseAgent(agentID string) error {
	agent, err := o.GetAgent(agentID)
	if err != nil {
		return err
	}

	if err := agent.Pause(); err != nil {
		return err
	}

	o.publishEvent(&AgentEvent{
		AgentID:   agentID,
		Type:      "AgentPaused",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{},
	})

	return nil
}

// ResumeAgent resumes a paused agent
func (o *AgentOrchestrator) ResumeAgent(agentID string) error {
	agent, err := o.GetAgent(agentID)
	if err != nil {
		return err
	}

	if err := agent.Resume(); err != nil {
		return err
	}

	o.publishEvent(&AgentEvent{
		AgentID:   agentID,
		Type:      "AgentResumed",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{},
	})

	return nil
}

// GetAgentStats returns statistics for a specific agent
func (o *AgentOrchestrator) GetAgentStats(agentID string) (map[string]interface{}, error) {
	agent, err := o.GetAgent(agentID)
	if err != nil {
		return nil, err
	}

	return agent.GetStats(), nil
}
