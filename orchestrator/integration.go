package orchestrator

import (
	"claude-squad/log"
	"claude-squad/session"
	"context"
	"fmt"
	"time"
)

// ClaudeSquadExecutor integrates the orchestrator with Claude Squad sessions
type ClaudeSquadExecutor struct {
	storage      *session.Storage
	program      string
	autoYes      bool
	sessionLimit int
}

// NewClaudeSquadExecutor creates an executor that manages Claude Squad sessions
func NewClaudeSquadExecutor(storage *session.Storage, program string, autoYes bool) *ClaudeSquadExecutor {
	return &ClaudeSquadExecutor{
		storage:      storage,
		program:      program,
		autoYes:      autoYes,
		sessionLimit: MaxConcurrentAgents,
	}
}

// Execute runs a task by creating a Claude Squad session
func (e *ClaudeSquadExecutor) Execute(ctx context.Context, task *Task) (*string, error) {
	log.InfoLog.Printf("Executing task %s via Claude Squad: %s", task.ID, task.Description)

	// Create a new instance for this task
	instance, err := e.createInstanceForTask(task)
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	// Monitor the instance until completion or context cancellation
	result, err := e.monitorInstance(ctx, instance, task)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (e *ClaudeSquadExecutor) createInstanceForTask(task *Task) (*session.Instance, error) {
	// Generate a unique session ID based on task
	sessionID := fmt.Sprintf("task-%s-%d", task.ID[:8], time.Now().Unix())

	// Create instance configuration
	config := session.InstanceConfig{
		ID:          sessionID,
		Prompt:      task.Description,
		Program:     e.program,
		AutoYes:     e.autoYes,
		Metadata:    task.Metadata,
		BranchName:  fmt.Sprintf("oxigraph-task-%s", task.ID[:8]),
		Priority:    task.Priority,
		Dependencies: task.Dependencies,
	}

	// Create the instance
	instance, err := session.NewInstance(config, e.storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	// Start the instance
	if err := instance.Start(); err != nil {
		return nil, fmt.Errorf("failed to start instance: %w", err)
	}

	log.InfoLog.Printf("Created and started instance %s for task %s", sessionID, task.ID)
	return instance, nil
}

func (e *ClaudeSquadExecutor) monitorInstance(
	ctx context.Context,
	instance *session.Instance,
	task *Task,
) (string, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := 30 * time.Minute
	if task.Metadata != nil {
		if timeoutStr, ok := task.Metadata["timeout"]; ok {
			if d, err := time.ParseDuration(timeoutStr); err == nil {
				timeout = d
			}
		}
	}

	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-ctx.Done():
			log.InfoLog.Printf("Task %s cancelled, stopping instance", task.ID)
			instance.Stop()
			return "", ctx.Err()

		case <-ticker.C:
			status := instance.GetStatus()

			switch status {
			case session.StatusCompleted:
				log.InfoLog.Printf("Task %s completed successfully", task.ID)
				result := instance.GetResult()
				return result, nil

			case session.StatusFailed:
				log.ErrorLog.Printf("Task %s failed", task.ID)
				return "", fmt.Errorf("instance failed: %s", instance.GetError())

			case session.StatusRunning:
				// Check for timeout
				if time.Now().After(deadline) {
					log.ErrorLog.Printf("Task %s timed out after %v", task.ID, timeout)
					instance.Stop()
					return "", fmt.Errorf("task timed out after %v", timeout)
				}

				log.InfoLog.Printf("Task %s still running...", task.ID)

			default:
				log.InfoLog.Printf("Task %s in status: %s", task.ID, status)
			}
		}
	}
}

// OrchestratedSquad manages multiple Claude Squad instances using the orchestrator
type OrchestratedSquad struct {
	pool       *AgentPool
	storage    *session.Storage
	program    string
	autoYes    bool
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewOrchestratedSquad creates a new orchestrated squad
func NewOrchestratedSquad(
	orchestratorURL string,
	storage *session.Storage,
	program string,
	autoYes bool,
) (*OrchestratedSquad, error) {
	executor := NewClaudeSquadExecutor(storage, program, autoYes)

	pool, err := NewAgentPool(orchestratorURL, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent pool: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	squad := &OrchestratedSquad{
		pool:       pool,
		storage:    storage,
		program:    program,
		autoYes:    autoYes,
		ctx:        ctx,
		cancelFunc: cancel,
	}

	// Start the pool
	if err := pool.Start(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start pool: %w", err)
	}

	log.InfoLog.Println("Orchestrated Squad started successfully")

	return squad, nil
}

// SubmitTask submits a task to the orchestrated squad
func (s *OrchestratedSquad) SubmitTask(description string, priority int, dependencies []string) (string, error) {
	task := &Task{
		Description:  description,
		Priority:     priority,
		Dependencies: dependencies,
		Metadata: map[string]string{
			"program": s.program,
			"autoYes": fmt.Sprintf("%v", s.autoYes),
		},
	}

	return s.pool.SubmitTask(task)
}

// SubmitTaskWithMetadata submits a task with custom metadata
func (s *OrchestratedSquad) SubmitTaskWithMetadata(
	description string,
	priority int,
	dependencies []string,
	metadata map[string]string,
) (string, error) {
	// Merge with default metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["program"] = s.program
	metadata["autoYes"] = fmt.Sprintf("%v", s.autoYes)

	task := &Task{
		Description:  description,
		Priority:     priority,
		Dependencies: dependencies,
		Metadata:     metadata,
	}

	return s.pool.SubmitTask(task)
}

// GetAnalytics returns squad analytics
func (s *OrchestratedSquad) GetAnalytics() (*Analytics, error) {
	return s.pool.GetAnalytics()
}

// WaitForCompletion waits for all tasks to complete
func (s *OrchestratedSquad) WaitForCompletion() error {
	return s.pool.WaitForCompletion(s.ctx)
}

// Stop gracefully shuts down the squad
func (s *OrchestratedSquad) Stop() {
	log.InfoLog.Println("Stopping orchestrated squad...")
	s.cancelFunc()
	s.pool.Stop()
	log.InfoLog.Println("Orchestrated squad stopped")
}

// BatchSubmit submits multiple tasks in a batch
func (s *OrchestratedSquad) BatchSubmit(tasks []BatchTask) ([]string, error) {
	taskIDs := make([]string, 0, len(tasks))

	for _, bt := range tasks {
		taskID, err := s.SubmitTask(bt.Description, bt.Priority, bt.Dependencies)
		if err != nil {
			return taskIDs, fmt.Errorf("failed to submit task '%s': %w", bt.Description, err)
		}
		taskIDs = append(taskIDs, taskID)
	}

	return taskIDs, nil
}

// BatchTask represents a task in a batch submission
type BatchTask struct {
	Description  string
	Priority     int
	Dependencies []string
	Metadata     map[string]string
}

// CreateWorkflow creates a predefined workflow pattern
func (s *OrchestratedSquad) CreateWorkflow(workflowType string, params map[string]string) ([]string, error) {
	switch workflowType {
	case "analyze-refactor-test":
		return s.createAnalyzeRefactorTestWorkflow(params)
	case "parallel-aggregate":
		return s.createParallelAggregateWorkflow(params)
	case "sequential-pipeline":
		return s.createSequentialPipelineWorkflow(params)
	default:
		return nil, fmt.Errorf("unknown workflow type: %s", workflowType)
	}
}

func (s *OrchestratedSquad) createAnalyzeRefactorTestWorkflow(params map[string]string) ([]string, error) {
	target := params["target"]
	if target == "" {
		target = "codebase"
	}

	// Step 1: Analyze
	t1, err := s.SubmitTask(
		fmt.Sprintf("Analyze %s structure and identify improvement areas", target),
		10,
		nil,
	)
	if err != nil {
		return nil, err
	}

	// Step 2: Refactor
	t2, err := s.SubmitTask(
		fmt.Sprintf("Refactor %s based on analysis findings", target),
		9,
		[]string{t1},
	)
	if err != nil {
		return nil, err
	}

	// Step 3: Test
	t3, err := s.SubmitTask(
		fmt.Sprintf("Run comprehensive tests on refactored %s", target),
		8,
		[]string{t2},
	)
	if err != nil {
		return nil, err
	}

	return []string{t1, t2, t3}, nil
}

func (s *OrchestratedSquad) createParallelAggregateWorkflow(params map[string]string) ([]string, error) {
	countStr := params["parallel_count"]
	count := 5
	if countStr != "" {
		fmt.Sscanf(countStr, "%d", &count)
	}

	taskIDs := make([]string, 0, count+1)

	// Create parallel tasks
	parallelIDs := make([]string, 0, count)
	for i := 0; i < count; i++ {
		taskID, err := s.SubmitTask(
			fmt.Sprintf("Parallel task %d/%d: %s", i+1, count, params["task_template"]),
			5,
			nil,
		)
		if err != nil {
			return taskIDs, err
		}
		parallelIDs = append(parallelIDs, taskID)
		taskIDs = append(taskIDs, taskID)
	}

	// Aggregation task
	aggTask, err := s.SubmitTask(
		"Aggregate and synthesize results from parallel tasks",
		10,
		parallelIDs,
	)
	if err != nil {
		return taskIDs, err
	}
	taskIDs = append(taskIDs, aggTask)

	return taskIDs, nil
}

func (s *OrchestratedSquad) createSequentialPipelineWorkflow(params map[string]string) ([]string, error) {
	steps := []string{
		params["step1"],
		params["step2"],
		params["step3"],
		params["step4"],
	}

	taskIDs := make([]string, 0, len(steps))
	var prevTaskID string

	for i, step := range steps {
		if step == "" {
			continue
		}

		var deps []string
		if prevTaskID != "" {
			deps = []string{prevTaskID}
		}

		taskID, err := s.SubmitTask(
			fmt.Sprintf("Pipeline step %d: %s", i+1, step),
			10-i,
			deps,
		)
		if err != nil {
			return taskIDs, err
		}

		taskIDs = append(taskIDs, taskID)
		prevTaskID = taskID
	}

	return taskIDs, nil
}
