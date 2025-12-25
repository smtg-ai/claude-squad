package concurrency

import (
	"claude-squad/session"
	"fmt"
	"time"
)

// Example demonstrates how to use the AgentOrchestrator
func Example() error {
	// Create orchestrator with custom configuration
	config := &OrchestratorConfig{
		MaxConcurrentTasks:     5,
		HealthCheckInterval:    30 * time.Second,
		TaskQueueSize:          50,
		EventBufferSize:        100,
		EnableAutoRecovery:     true,
		LoadBalancingAlgorithm: "least-loaded",
	}

	orchestrator := NewOrchestrator(config)
	defer orchestrator.Shutdown(30 * time.Second)

	// Subscribe to orchestrator events
	go func() {
		for event := range orchestrator.EventChannel() {
			fmt.Printf("[EVENT] %s: %s at %s\n",
				event.AgentID,
				event.Type,
				event.Timestamp.Format(time.RFC3339))
		}
	}()

	// Create and add agents
	for i := 1; i <= 3; i++ {
		agentID := fmt.Sprintf("agent-%d", i)

		// Create session instance
		instance, err := session.NewInstance(session.InstanceOptions{
			Title:   agentID,
			Path:    "/path/to/workspace",
			Program: "claude",
			AutoYes: false,
		})
		if err != nil {
			return fmt.Errorf("failed to create instance: %w", err)
		}

		// Start the instance
		if err := instance.Start(true); err != nil {
			return fmt.Errorf("failed to start instance: %w", err)
		}

		// Create managed agent
		agent := NewManagedAgent(agentID, instance)

		// Add to orchestrator
		if err := orchestrator.AddAgent(agent); err != nil {
			return fmt.Errorf("failed to add agent: %w", err)
		}

		fmt.Printf("Added agent: %s\n", agentID)
	}

	// Distribute tasks with different priorities
	tasks := []*Task{
		{
			ID:       "task-1",
			Prompt:   "Analyze the codebase for potential bugs",
			Priority: TaskPriorityHigh,
			Timeout:  5 * time.Minute,
			Metadata: map[string]interface{}{
				"category": "analysis",
			},
			ResultChan: make(chan *TaskResult, 1),
		},
		{
			ID:         "task-2",
			Prompt:     "Generate unit tests for the user service",
			Priority:   TaskPriorityNormal,
			Affinity:   []string{"agent-1"}, // Prefer agent-1
			Timeout:    3 * time.Minute,
			ResultChan: make(chan *TaskResult, 1),
		},
		{
			ID:         "task-3",
			Prompt:     "Refactor the authentication module",
			Priority:   TaskPriorityCritical,
			Timeout:    10 * time.Minute,
			ResultChan: make(chan *TaskResult, 1),
		},
	}

	// Distribute tasks
	for _, task := range tasks {
		if err := orchestrator.DistributeTask(task); err != nil {
			fmt.Printf("Failed to distribute task %s: %v\n", task.ID, err)
			continue
		}
		fmt.Printf("Distributed task: %s\n", task.ID)
	}

	// Collect results
	for _, task := range tasks {
		select {
		case result := <-task.ResultChan:
			if result.Success {
				fmt.Printf("Task %s completed successfully by %s in %s\n",
					result.TaskID,
					result.AgentID,
					result.Duration)
			} else {
				fmt.Printf("Task %s failed: %v\n", result.TaskID, result.Error)
			}
		case <-time.After(15 * time.Minute):
			fmt.Printf("Task %s timed out\n", task.ID)
		}
	}

	// Print metrics
	metrics := orchestrator.GetMetrics()
	fmt.Printf("\nOrchestrator Metrics:\n")
	for key, value := range metrics {
		fmt.Printf("  %s: %v\n", key, value)
	}

	// Get stats for each agent
	agentIDs := orchestrator.ListAgents()
	for _, agentID := range agentIDs {
		stats, err := orchestrator.GetAgentStats(agentID)
		if err != nil {
			fmt.Printf("Failed to get stats for %s: %v\n", agentID, err)
			continue
		}

		fmt.Printf("\nAgent %s Stats:\n", agentID)
		for key, value := range stats {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}

	// Demonstrate agent lifecycle management
	fmt.Println("\n=== Demonstrating Agent Lifecycle ===")

	// Pause an agent
	if err := orchestrator.PauseAgent("agent-2"); err != nil {
		fmt.Printf("Failed to pause agent-2: %v\n", err)
	} else {
		fmt.Println("Agent-2 paused successfully")
	}

	// Wait a bit
	time.Sleep(5 * time.Second)

	// Resume the agent
	if err := orchestrator.ResumeAgent("agent-2"); err != nil {
		fmt.Printf("Failed to resume agent-2: %v\n", err)
	} else {
		fmt.Println("Agent-2 resumed successfully")
	}

	// Remove an agent
	if err := orchestrator.RemoveAgent("agent-3"); err != nil {
		fmt.Printf("Failed to remove agent-3: %v\n", err)
	} else {
		fmt.Println("Agent-3 removed successfully")
	}

	return nil
}

// ExampleLoadBalancing demonstrates different load balancing strategies
func ExampleLoadBalancing() {
	// Round-robin load balancing
	config1 := DefaultOrchestratorConfig()
	config1.LoadBalancingAlgorithm = "round-robin"
	orchestrator1 := NewOrchestrator(config1)
	defer orchestrator1.Shutdown(10 * time.Second)

	// Least-loaded load balancing
	config2 := DefaultOrchestratorConfig()
	config2.LoadBalancingAlgorithm = "least-loaded"
	orchestrator2 := NewOrchestrator(config2)
	defer orchestrator2.Shutdown(10 * time.Second)

	// Random load balancing
	config3 := DefaultOrchestratorConfig()
	config3.LoadBalancingAlgorithm = "random"
	orchestrator3 := NewOrchestrator(config3)
	defer orchestrator3.Shutdown(10 * time.Second)

	fmt.Println("Created orchestrators with different load balancing strategies")
}

// ExampleCircuitBreaker demonstrates the circuit breaker pattern
func ExampleCircuitBreaker() {
	// Create a circuit breaker
	cb := NewCircuitBreaker(
		3,              // Max failures before opening
		30*time.Second, // Reset timeout
		2,              // Successful tests needed to close
	)

	// Simulate failures
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
		fmt.Printf("Failure %d recorded, state: %v\n", i+1, cb.GetState())
	}

	// Circuit should be open now
	if !cb.CanExecute() {
		fmt.Println("Circuit is open, execution blocked")
	}

	// Wait for reset timeout
	time.Sleep(30 * time.Second)

	// Transition to half-open
	if cb.TransitionToHalfOpen() {
		fmt.Println("Circuit transitioned to half-open")
	}

	// Record successful tests
	cb.RecordSuccess()
	fmt.Printf("After 1 success, state: %v\n", cb.GetState())

	cb.RecordSuccess()
	fmt.Printf("After 2 successes, state: %v\n", cb.GetState())

	// Circuit should be closed now
	if cb.CanExecute() {
		fmt.Println("Circuit is closed, execution allowed")
	}
}

// ExampleEventDriven demonstrates the event-driven architecture
func ExampleEventDriven() error {
	orchestrator := NewOrchestrator(DefaultOrchestratorConfig())
	defer orchestrator.Shutdown(10 * time.Second)

	// Create event listener
	eventListener := func(eventChan <-chan *AgentEvent) {
		for event := range eventChan {
			switch event.Type {
			case "AgentAdded":
				fmt.Printf("New agent added: %s\n", event.AgentID)
			case "AgentRemoved":
				fmt.Printf("Agent removed: %s\n", event.AgentID)
			case "TaskCompleted":
				fmt.Printf("Task completed on agent %s: %v\n",
					event.AgentID,
					event.Data["task_id"])
			case "HealthCheckFailed":
				fmt.Printf("Health check failed for agent %s: %v\n",
					event.AgentID,
					event.Data["error"])
			case "AgentRecovered":
				fmt.Printf("Agent %s recovered from failure\n", event.AgentID)
			case "AgentPaused":
				fmt.Printf("Agent %s paused\n", event.AgentID)
			case "AgentResumed":
				fmt.Printf("Agent %s resumed\n", event.AgentID)
			default:
				fmt.Printf("Unknown event: %s from %s\n", event.Type, event.AgentID)
			}
		}
	}

	// Start event listener in background
	go eventListener(orchestrator.EventChannel())

	// Simulate some operations that generate events
	// (Add agents, distribute tasks, etc.)

	return nil
}

// ExampleTaskAffinity demonstrates task affinity for agent selection
func ExampleTaskAffinity() error {
	orchestrator := NewOrchestrator(DefaultOrchestratorConfig())
	defer orchestrator.Shutdown(10 * time.Second)

	// Add specialized agents
	instance1, _ := session.NewInstance(session.InstanceOptions{
		Title:   "backend-specialist",
		Path:    "/path/to/backend",
		Program: "claude",
	})
	_ = instance1.Start(true)
	agent1 := NewManagedAgent("backend-specialist", instance1)
	orchestrator.AddAgent(agent1)

	instance2, _ := session.NewInstance(session.InstanceOptions{
		Title:   "frontend-specialist",
		Path:    "/path/to/frontend",
		Program: "claude",
	})
	_ = instance2.Start(true)
	agent2 := NewManagedAgent("frontend-specialist", instance2)
	orchestrator.AddAgent(agent2)

	// Create task with affinity for backend specialist
	backendTask := &Task{
		ID:         "backend-task-1",
		Prompt:     "Optimize database queries",
		Priority:   TaskPriorityHigh,
		Affinity:   []string{"backend-specialist"}, // Prefer backend specialist
		Timeout:    5 * time.Minute,
		ResultChan: make(chan *TaskResult, 1),
	}

	// Create task with affinity for frontend specialist
	frontendTask := &Task{
		ID:         "frontend-task-1",
		Prompt:     "Improve UI responsiveness",
		Priority:   TaskPriorityHigh,
		Affinity:   []string{"frontend-specialist"}, // Prefer frontend specialist
		Timeout:    5 * time.Minute,
		ResultChan: make(chan *TaskResult, 1),
	}

	// Distribute tasks
	orchestrator.DistributeTask(backendTask)
	orchestrator.DistributeTask(frontendTask)

	fmt.Println("Tasks distributed with affinity preferences")

	return nil
}

// ExampleConcurrencyLimits demonstrates configurable concurrency limits
func ExampleConcurrencyLimits() error {
	// Create orchestrator with strict concurrency limit
	config := &OrchestratorConfig{
		MaxConcurrentTasks:     2, // Only 2 tasks can run concurrently
		HealthCheckInterval:    30 * time.Second,
		TaskQueueSize:          100,
		EventBufferSize:        100,
		EnableAutoRecovery:     true,
		LoadBalancingAlgorithm: "least-loaded",
	}

	orchestrator := NewOrchestrator(config)
	defer orchestrator.Shutdown(10 * time.Second)

	// Add multiple agents
	for i := 1; i <= 5; i++ {
		agentID := fmt.Sprintf("agent-%d", i)
		instance, _ := session.NewInstance(session.InstanceOptions{
			Title:   agentID,
			Path:    "/path/to/workspace",
			Program: "claude",
		})
		_ = instance.Start(true)
		agent := NewManagedAgent(agentID, instance)
		orchestrator.AddAgent(agent)
	}

	// Submit many tasks
	for i := 1; i <= 10; i++ {
		task := &Task{
			ID:         fmt.Sprintf("task-%d", i),
			Prompt:     fmt.Sprintf("Process item %d", i),
			Priority:   TaskPriorityNormal,
			Timeout:    1 * time.Minute,
			ResultChan: make(chan *TaskResult, 1),
		}
		orchestrator.DistributeTask(task)
	}

	// Only 2 tasks will execute concurrently at any time
	fmt.Println("Tasks submitted - only 2 will execute concurrently")

	return nil
}
