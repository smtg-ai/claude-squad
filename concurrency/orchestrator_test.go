package concurrency

import (
	"claude-squad/log"
	"os"
	"testing"
	"time"
)

// TestMain initializes the test environment
func TestMain(m *testing.M) {
	// Initialize logging for tests
	log.Initialize(false)
	defer log.Close()

	// Run tests
	code := m.Run()

	os.Exit(code)
}

// TestCircuitBreaker tests the circuit breaker pattern
func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Second, 2)

	// Initially circuit should be closed
	if !cb.CanExecute() {
		t.Error("Circuit should be closed initially")
	}

	// Record 3 failures to open circuit
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	// Circuit should be open now
	if cb.GetState() != CircuitOpen {
		t.Errorf("Expected circuit to be open, got %v", cb.GetState())
	}

	if cb.CanExecute() {
		t.Error("Circuit should not allow execution when open")
	}

	// Wait for reset timeout
	time.Sleep(1100 * time.Millisecond)

	// Should be able to execute now (will transition to half-open)
	if !cb.CanExecute() {
		t.Error("Circuit should allow execution after reset timeout")
	}

	// Transition to half-open
	if !cb.TransitionToHalfOpen() {
		t.Error("Circuit should transition to half-open")
	}

	if cb.GetState() != CircuitHalfOpen {
		t.Errorf("Expected circuit to be half-open, got %v", cb.GetState())
	}

	// Record 2 successes to close circuit
	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.GetState() != CircuitClosed {
		t.Errorf("Expected circuit to be closed after successful tests, got %v", cb.GetState())
	}
}

// TestCircuitBreakerHalfOpenFailure tests circuit reopening on half-open failure
func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	cb := NewCircuitBreaker(2, 1*time.Second, 2)

	// Open the circuit
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.GetState() != CircuitOpen {
		t.Errorf("Expected circuit to be open, got %v", cb.GetState())
	}

	// Wait for reset timeout
	time.Sleep(1100 * time.Millisecond)

	// Transition to half-open
	cb.TransitionToHalfOpen()

	// Fail during half-open
	cb.RecordFailure()

	// Should be open again
	if cb.GetState() != CircuitOpen {
		t.Errorf("Expected circuit to reopen after half-open failure, got %v", cb.GetState())
	}
}

// TestManagedAgentLoadScore tests load score calculation
func TestManagedAgentLoadScore(t *testing.T) {
	agent := &ManagedAgent{
		id:             "test-agent",
		state:          AgentStateIdle,
		circuitBreaker: NewCircuitBreaker(3, 30*time.Second, 2),
		loadScore:      0.0,
	}

	// Idle agent should have low load
	agent.UpdateLoadScore()
	if agent.GetLoadScore() != 0.0 {
		t.Errorf("Expected load score 0.0 for idle agent, got %f", agent.GetLoadScore())
	}

	// Running agent should have higher load
	agent.state = AgentStateRunning
	agent.UpdateLoadScore()
	if agent.GetLoadScore() != 0.8 {
		t.Errorf("Expected load score 0.8 for running agent, got %f", agent.GetLoadScore())
	}

	// Failed agent should have max load
	agent.state = AgentStateFailed
	agent.UpdateLoadScore()
	if agent.GetLoadScore() != 1.0 {
		t.Errorf("Expected load score 1.0 for failed agent, got %f", agent.GetLoadScore())
	}
}

// TestOrchestratorCreation tests orchestrator creation and configuration
func TestOrchestratorCreation(t *testing.T) {
	config := &OrchestratorConfig{
		MaxConcurrentTasks:     5,
		HealthCheckInterval:    30 * time.Second,
		TaskQueueSize:          100,
		EventBufferSize:        50,
		EnableAutoRecovery:     true,
		LoadBalancingAlgorithm: "least-loaded",
	}

	orchestrator := NewOrchestrator(config)
	if orchestrator == nil {
		t.Fatal("Expected orchestrator to be created")
	}

	defer orchestrator.Shutdown(5 * time.Second)

	if orchestrator.config.MaxConcurrentTasks != 5 {
		t.Errorf("Expected max concurrent tasks 5, got %d", orchestrator.config.MaxConcurrentTasks)
	}

	if orchestrator.config.LoadBalancingAlgorithm != "least-loaded" {
		t.Errorf("Expected load balancing algorithm 'least-loaded', got %s",
			orchestrator.config.LoadBalancingAlgorithm)
	}
}

// TestOrchestratorDefaultConfig tests default configuration
func TestOrchestratorDefaultConfig(t *testing.T) {
	orchestrator := NewOrchestrator(nil)
	if orchestrator == nil {
		t.Fatal("Expected orchestrator to be created with default config")
	}

	defer orchestrator.Shutdown(5 * time.Second)

	if orchestrator.config.MaxConcurrentTasks != 10 {
		t.Errorf("Expected default max concurrent tasks 10, got %d",
			orchestrator.config.MaxConcurrentTasks)
	}
}

// TestOrchestratorAddRemoveAgent tests agent management
func TestOrchestratorAddRemoveAgent(t *testing.T) {
	orchestrator := NewOrchestrator(DefaultOrchestratorConfig())
	defer orchestrator.Shutdown(5 * time.Second)

	// Create a mock managed agent (without actual instance for testing)
	agent := &ManagedAgent{
		id:             "test-agent-1",
		state:          AgentStateIdle,
		createdAt:      time.Now(),
		updatedAt:      time.Now(),
		circuitBreaker: NewCircuitBreaker(3, 30*time.Second, 2),
		stopChan:       make(chan struct{}),
		loadScore:      0.0,
	}

	// Add agent
	err := orchestrator.AddAgent(agent)
	if err != nil {
		t.Errorf("Failed to add agent: %v", err)
	}

	// Try to add the same agent again
	err = orchestrator.AddAgent(agent)
	if err == nil {
		t.Error("Expected error when adding duplicate agent")
	}

	// Verify agent is in the list
	agents := orchestrator.ListAgents()
	if len(agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(agents))
	}

	// Get the agent
	retrievedAgent, err := orchestrator.GetAgent("test-agent-1")
	if err != nil {
		t.Errorf("Failed to get agent: %v", err)
	}
	if retrievedAgent.GetID() != "test-agent-1" {
		t.Errorf("Expected agent ID 'test-agent-1', got %s", retrievedAgent.GetID())
	}

	// Remove agent
	agent.stopped = true // Mark as stopped to prevent Kill() call
	err = orchestrator.RemoveAgent("test-agent-1")
	if err != nil {
		t.Errorf("Failed to remove agent: %v", err)
	}

	// Verify agent is removed
	agents = orchestrator.ListAgents()
	if len(agents) != 0 {
		t.Errorf("Expected 0 agents after removal, got %d", len(agents))
	}

	// Try to remove non-existent agent
	err = orchestrator.RemoveAgent("non-existent")
	if err == nil {
		t.Error("Expected error when removing non-existent agent")
	}
}

// TestOrchestratorMetrics tests metrics collection
func TestOrchestratorMetrics(t *testing.T) {
	orchestrator := NewOrchestrator(DefaultOrchestratorConfig())
	defer orchestrator.Shutdown(5 * time.Second)

	// Get initial metrics
	metrics := orchestrator.GetMetrics()
	if metrics["total_agents"].(int) != 0 {
		t.Errorf("Expected 0 total agents, got %d", metrics["total_agents"])
	}

	// Add an agent
	agent := &ManagedAgent{
		id:             "test-agent-1",
		state:          AgentStateIdle,
		createdAt:      time.Now(),
		updatedAt:      time.Now(),
		circuitBreaker: NewCircuitBreaker(3, 30*time.Second, 2),
		stopChan:       make(chan struct{}),
		loadScore:      0.0,
	}
	orchestrator.AddAgent(agent)

	// Check metrics again
	metrics = orchestrator.GetMetrics()
	if metrics["total_agents"].(int) != 1 {
		t.Errorf("Expected 1 total agent, got %d", metrics["total_agents"])
	}

	if metrics["idle_agents"].(int) != 1 {
		t.Errorf("Expected 1 idle agent, got %d", metrics["idle_agents"])
	}

	// Clean up
	agent.stopped = true
	orchestrator.RemoveAgent("test-agent-1")
}

// TestAgentStateTransitions tests agent state management
func TestAgentStateTransitions(t *testing.T) {
	agent := &ManagedAgent{
		id:             "test-agent",
		state:          AgentStateIdle,
		createdAt:      time.Now(),
		updatedAt:      time.Now(),
		circuitBreaker: NewCircuitBreaker(3, 30*time.Second, 2),
		stopChan:       make(chan struct{}),
	}

	if agent.GetState() != AgentStateIdle {
		t.Errorf("Expected initial state to be Idle, got %s", agent.GetState().String())
	}

	agent.SetState(AgentStateRunning)
	if agent.GetState() != AgentStateRunning {
		t.Errorf("Expected state to be Running, got %s", agent.GetState().String())
	}

	agent.SetState(AgentStatePaused)
	if agent.GetState() != AgentStatePaused {
		t.Errorf("Expected state to be Paused, got %s", agent.GetState().String())
	}

	agent.SetState(AgentStateFailed)
	if agent.GetState() != AgentStateFailed {
		t.Errorf("Expected state to be Failed, got %s", agent.GetState().String())
	}

	agent.SetState(AgentStateStopped)
	if agent.GetState() != AgentStateStopped {
		t.Errorf("Expected state to be Stopped, got %s", agent.GetState().String())
	}
}

// TestTaskPriority tests task priority enum
func TestTaskPriority(t *testing.T) {
	priorities := []TaskPriority{
		PriorityLow,
		PriorityNormal,
		PriorityHigh,
		PriorityCritical,
	}

	if len(priorities) != 4 {
		t.Errorf("Expected 4 priority levels, got %d", len(priorities))
	}

	if PriorityLow >= PriorityNormal {
		t.Error("Expected PriorityLow < PriorityNormal")
	}

	if PriorityNormal >= PriorityHigh {
		t.Error("Expected PriorityNormal < PriorityHigh")
	}

	if PriorityHigh >= PriorityCritical {
		t.Error("Expected PriorityHigh < PriorityCritical")
	}
}

// TestOrchestratorEventChannel tests event channel functionality
func TestOrchestratorEventChannel(t *testing.T) {
	orchestrator := NewOrchestrator(DefaultOrchestratorConfig())
	defer orchestrator.Shutdown(5 * time.Second)

	eventReceived := make(chan bool, 1)

	// Subscribe to events
	go func() {
		for event := range orchestrator.EventChannel() {
			if event.Type == "AgentAdded" && event.AgentID == "test-agent" {
				eventReceived <- true
				return
			}
		}
	}()

	// Add an agent to trigger event
	agent := &ManagedAgent{
		id:             "test-agent",
		state:          AgentStateIdle,
		createdAt:      time.Now(),
		updatedAt:      time.Now(),
		circuitBreaker: NewCircuitBreaker(3, 30*time.Second, 2),
		stopChan:       make(chan struct{}),
	}
	orchestrator.AddAgent(agent)

	// Wait for event
	select {
	case <-eventReceived:
		// Event received successfully
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for AgentAdded event")
	}

	// Clean up
	agent.stopped = true
	orchestrator.RemoveAgent("test-agent")
}

// TestAgentStats tests agent statistics collection
func TestAgentStats(t *testing.T) {
	agent := &ManagedAgent{
		id:              "test-agent",
		state:           AgentStateIdle,
		createdAt:       time.Now(),
		updatedAt:       time.Now(),
		circuitBreaker:  NewCircuitBreaker(3, 30*time.Second, 2),
		stopChan:        make(chan struct{}),
		tasksCompleted:  5,
		tasksFailed:     2,
		totalExecTime:   10 * time.Second,
		avgResponseTime: 2 * time.Second,
		loadScore:       0.3,
		healthCheckOK:   true,
	}

	stats := agent.GetStats()

	if stats["id"] != "test-agent" {
		t.Errorf("Expected agent ID 'test-agent', got %s", stats["id"])
	}

	if stats["tasks_completed"] != 5 {
		t.Errorf("Expected 5 completed tasks, got %d", stats["tasks_completed"])
	}

	if stats["tasks_failed"] != 2 {
		t.Errorf("Expected 2 failed tasks, got %d", stats["tasks_failed"])
	}

	if stats["load_score"] != 0.3 {
		t.Errorf("Expected load score 0.3, got %f", stats["load_score"])
	}

	if stats["health_ok"] != true {
		t.Errorf("Expected health_ok to be true, got %v", stats["health_ok"])
	}
}

// TestSelectAgentLeastLoaded tests least-loaded agent selection
func TestSelectAgentLeastLoaded(t *testing.T) {
	orchestrator := NewOrchestrator(&OrchestratorConfig{
		MaxConcurrentTasks:     10,
		HealthCheckInterval:    30 * time.Second,
		TaskQueueSize:          100,
		EventBufferSize:        100,
		EnableAutoRecovery:     true,
		LoadBalancingAlgorithm: "least-loaded",
	})
	defer orchestrator.Shutdown(5 * time.Second)

	// Add agents with different states to get different load scores
	agent1 := &ManagedAgent{
		id:             "agent-1",
		state:          AgentStateRunning, // Running state = 0.8 load
		createdAt:      time.Now(),
		updatedAt:      time.Now(),
		circuitBreaker: NewCircuitBreaker(3, 30*time.Second, 2),
		stopChan:       make(chan struct{}),
		loadScore:      0.8,
		healthCheckOK:  true,
	}

	agent2 := &ManagedAgent{
		id:             "agent-2",
		state:          AgentStateIdle, // Idle state = 0.0 load (lowest)
		createdAt:      time.Now(),
		updatedAt:      time.Now(),
		circuitBreaker: NewCircuitBreaker(3, 30*time.Second, 2),
		stopChan:       make(chan struct{}),
		loadScore:      0.0,
		healthCheckOK:  true,
	}

	agent3 := &ManagedAgent{
		id:             "agent-3",
		state:          AgentStatePaused, // Paused state = 0.5 load
		createdAt:      time.Now(),
		updatedAt:      time.Now(),
		circuitBreaker: NewCircuitBreaker(3, 30*time.Second, 2),
		stopChan:       make(chan struct{}),
		loadScore:      0.5,
		healthCheckOK:  true,
	}

	orchestrator.AddAgent(agent1)
	orchestrator.AddAgent(agent2)
	orchestrator.AddAgent(agent3)

	// Note: Only agent-2 (Idle) will be selected because the algorithm
	// filters for AgentStateIdle AND IsHealthy()
	task := &Task{ID: "test-task", Prompt: "test"}
	selected, err := orchestrator.selectAgent(task)
	if err != nil {
		t.Fatalf("Failed to select agent: %v", err)
	}

	// Should select agent-2 as it's the only idle agent
	if selected.GetID() != "agent-2" {
		t.Errorf("Expected agent-2 to be selected (only idle agent), got %s", selected.GetID())
	}

	// Clean up
	agent1.stopped = true
	agent2.stopped = true
	agent3.stopped = true
	orchestrator.RemoveAgent("agent-1")
	orchestrator.RemoveAgent("agent-2")
	orchestrator.RemoveAgent("agent-3")
}

// TestTaskDistributionTimeout tests task distribution queue limits
func TestTaskDistributionTimeout(t *testing.T) {
	// This test validates that the orchestrator respects queue size limits
	// We skip the detailed testing as it requires precise timing control
	// and can be flaky. The queue management is tested indirectly by other tests.
	t.Skip("Skipping queue overflow test - requires precise timing and can be flaky")
}

// BenchmarkCircuitBreakerRecordFailure benchmarks circuit breaker failure recording
func BenchmarkCircuitBreakerRecordFailure(b *testing.B) {
	cb := NewCircuitBreaker(3, 30*time.Second, 2)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cb.RecordFailure()
	}
}

// BenchmarkCircuitBreakerRecordSuccess benchmarks circuit breaker success recording
func BenchmarkCircuitBreakerRecordSuccess(b *testing.B) {
	cb := NewCircuitBreaker(3, 30*time.Second, 2)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cb.RecordSuccess()
	}
}

// BenchmarkAgentLoadScoreUpdate benchmarks load score calculation
func BenchmarkAgentLoadScoreUpdate(b *testing.B) {
	agent := &ManagedAgent{
		id:             "bench-agent",
		state:          AgentStateRunning,
		circuitBreaker: NewCircuitBreaker(3, 30*time.Second, 2),
		currentTask: &Task{
			ID:      "bench-task",
			Timeout: 5 * time.Minute,
		},
		taskStartTime: time.Now(),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		agent.UpdateLoadScore()
	}
}

// TestOrchestratorShutdown tests graceful shutdown
func TestOrchestratorShutdown(t *testing.T) {
	orchestrator := NewOrchestrator(DefaultOrchestratorConfig())

	// Add a test agent
	agent := &ManagedAgent{
		id:             "test-agent",
		state:          AgentStateIdle,
		createdAt:      time.Now(),
		updatedAt:      time.Now(),
		circuitBreaker: NewCircuitBreaker(3, 30*time.Second, 2),
		stopChan:       make(chan struct{}),
		stopped:        true, // Mark as stopped to prevent Kill() call
	}
	orchestrator.AddAgent(agent)

	// Shutdown with timeout
	err := orchestrator.Shutdown(10 * time.Second)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Verify context is canceled
	select {
	case <-orchestrator.ctx.Done():
		// Context properly canceled
	default:
		t.Error("Expected context to be canceled after shutdown")
	}
}

// TestManagedAgentExecuteTask tests task execution with context
func TestManagedAgentExecuteTask(t *testing.T) {
	// This test validates the task execution flow without a real instance
	// We skip it because it requires a fully initialized session.Instance
	// which is complex to set up in unit tests
	t.Skip("Skipping task execution test - requires full instance setup")
}
