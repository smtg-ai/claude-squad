package testing

import (
	"claude-squad/integrations/aider"
	"context"
	"fmt"
	"testing"
	"time"
)

// TestHyperAdvanced10AgentConcurrent runs the complete 10-agent concurrent test suite
func TestHyperAdvanced10AgentConcurrent(t *testing.T) {
	ctx := context.Background()

	config := &aider.SimulatorConfig{
		Mode:                  aider.FastMode,
		BaseLatency:           10,
		ErrorRate:             0.0,
		MaxConcurrentSessions: 50,
		EnableMetrics:         true,
		SessionTimeout:        10 * time.Minute,
	}

	result, err := RunHyperAdvancedTests(ctx, config)
	if err != nil {
		t.Fatalf("Test execution failed: %v", err)
	}

	// Create orchestrator to generate report
	simulator, _ := aider.NewAiderSimulator(config)
	orchestrator, _ := NewTestOrchestrator(simulator)

	// Register agents (for report generation)
	agents := []*TestAgent{
		{ID: "agent-1", Name: "Command Building & Validation", Executor: NewCommandValidationAgent()},
		{ID: "agent-2", Name: "Mode Switching & State Management", Executor: NewModeSwitchingAgent()},
		{ID: "agent-3", Name: "Model Selection Strategies", Executor: NewModelSelectionAgent()},
		{ID: "agent-4", Name: "Session Lifecycle Management", Executor: NewSessionLifecycleAgent()},
		{ID: "agent-5", Name: "Concurrent Session Handling", Executor: NewConcurrentSessionAgent()},
		{ID: "agent-6", Name: "Error Handling & Recovery", Executor: NewErrorHandlingAgent()},
		{ID: "agent-7", Name: "Git Integration", Executor: NewGitIntegrationAgent()},
		{ID: "agent-8", Name: "Configuration Management", Executor: NewConfigManagementAgent()},
		{ID: "agent-9", Name: "Performance & Stress Testing", Executor: NewPerformanceStressAgent()},
		{ID: "agent-10", Name: "Integration Patterns & Best Practices", Executor: NewIntegrationPatternsAgent()},
	}

	for _, agent := range agents {
		_ = orchestrator.RegisterAgent(agent)
	}

	// Populate results
	for _, agentResult := range result.AgentResults {
		orchestrator.mu.Lock()
		orchestrator.results[agentResult.AgentID] = agentResult
		orchestrator.mu.Unlock()
	}

	orchestrator.startTime = time.Now().Add(-result.Duration)
	orchestrator.endTime = time.Now()
	orchestrator.totalTests = int32(result.TotalTests)
	orchestrator.passedTests = int32(result.PassedTests)
	orchestrator.failedTests = int32(result.FailedTests)
	orchestrator.orchestratorState = "completed"

	report := orchestrator.GenerateReport()
	fmt.Println(report)

	// Validate results
	if result.TotalTests == 0 {
		t.Error("No tests were executed")
	}

	if result.PassedTests == 0 {
		t.Error("No tests passed")
	}

	successRate := float64(result.PassedTests) / float64(result.TotalTests) * 100
	t.Logf("Success Rate: %.2f%% (%d/%d)", successRate, result.PassedTests, result.TotalTests)

	if successRate < 80.0 {
		t.Errorf("Success rate too low: %.2f%% (expected >= 80%%)", successRate)
	}

	// Validate that all 10 agents ran
	if len(result.AgentResults) != 10 {
		t.Errorf("Expected 10 agent results, got %d", len(result.AgentResults))
	}

	// Validate concurrency
	if result.Metrics.TotalAgents != 10 {
		t.Errorf("Expected 10 total agents, got %d", result.Metrics.TotalAgents)
	}

	// Validate performance
	if result.Metrics.TestThroughput == 0 {
		t.Error("Test throughput is zero")
	}

	t.Logf("Test throughput: %.2f tests/sec", result.Metrics.TestThroughput)
	t.Logf("Total duration: %s", result.Duration)
}

// TestFastMode tests in fast mode
func TestFastMode(t *testing.T) {
	ctx := context.Background()
	result, err := RunFastTests(ctx)
	if err != nil {
		t.Fatalf("Fast mode test failed: %v", err)
	}

	if result.TotalTests == 0 {
		t.Error("No tests executed")
	}

	t.Logf("Fast mode: %d tests, %d passed, %d failed",
		result.TotalTests, result.PassedTests, result.FailedTests)
}

// TestRealisticMode tests in realistic mode
func TestRealisticMode(t *testing.T) {
	ctx := context.Background()
	result, err := RunRealisticTests(ctx)
	if err != nil {
		t.Fatalf("Realistic mode test failed: %v", err)
	}

	if result.TotalTests == 0 {
		t.Error("No tests executed")
	}

	t.Logf("Realistic mode: %d tests, %d passed, %d failed",
		result.TotalTests, result.PassedTests, result.FailedTests)
}

// TestStressMode tests in stress mode
func TestStressMode(t *testing.T) {
	ctx := context.Background()
	result, err := RunStressTests(ctx)
	if err != nil {
		t.Fatalf("Stress mode test failed: %v", err)
	}

	if result.TotalTests == 0 {
		t.Error("No tests executed")
	}

	t.Logf("Stress mode: %d tests, %d passed, %d failed",
		result.TotalTests, result.PassedTests, result.FailedTests)
}

// TestCompleteSuite runs the complete test suite
func TestCompleteSuite(t *testing.T) {
	ctx := context.Background()
	suite, err := RunCompleteSuite(ctx)
	if err != nil {
		t.Fatalf("Complete suite failed: %v", err)
	}

	report := GenerateSuiteReport(suite)
	fmt.Println(report)

	if suite.Summary.TotalRuns != 3 {
		t.Errorf("Expected 3 runs, got %d", suite.Summary.TotalRuns)
	}

	if suite.Summary.OverallSuccessRate < 80.0 {
		t.Errorf("Overall success rate too low: %.2f%%", suite.Summary.OverallSuccessRate)
	}

	t.Logf("Suite complete: %d runs, %.2f%% success rate",
		suite.Summary.TotalRuns, suite.Summary.OverallSuccessRate)
}

// BenchmarkConcurrentAgents benchmarks the 10-agent concurrent execution
func BenchmarkConcurrentAgents(b *testing.B) {
	ctx := context.Background()
	config := aider.DefaultSimulatorConfig()
	config.Mode = aider.FastMode
	config.BaseLatency = 1

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = RunHyperAdvancedTests(ctx, config)
	}
}

// BenchmarkSingleAgent benchmarks a single agent execution
func BenchmarkSingleAgent(b *testing.B) {
	ctx := context.Background()
	config := aider.DefaultSimulatorConfig()
	config.Mode = aider.FastMode
	config.BaseLatency = 1

	simulator, _ := aider.NewAiderSimulator(config)
	agent := NewCommandValidationAgent()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = agent.Execute(ctx, simulator)
	}
}

// TestAgentIsolation verifies that agents run independently
func TestAgentIsolation(t *testing.T) {
	ctx := context.Background()
	config := aider.DefaultSimulatorConfig()
	config.Mode = aider.FastMode

	result, err := RunHyperAdvancedTests(ctx, config)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	// Verify each agent has independent results
	agentIDs := make(map[string]bool)
	for _, agentResult := range result.AgentResults {
		if agentIDs[agentResult.AgentID] {
			t.Errorf("Duplicate agent ID: %s", agentResult.AgentID)
		}
		agentIDs[agentResult.AgentID] = true

		if agentResult.TestCases == 0 {
			t.Errorf("Agent %s executed no tests", agentResult.AgentID)
		}
	}

	if len(agentIDs) != 10 {
		t.Errorf("Expected 10 unique agents, got %d", len(agentIDs))
	}
}

// TestSimulatorMetrics validates simulator metrics collection
func TestSimulatorMetrics(t *testing.T) {
	ctx := context.Background()
	config := aider.DefaultSimulatorConfig()
	config.Mode = aider.FastMode
	config.EnableMetrics = true

	result, err := RunHyperAdvancedTests(ctx, config)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	metrics := result.SimulatorData

	if metrics.TotalSessions == 0 {
		t.Error("No sessions created")
	}

	if metrics.TotalCommands == 0 {
		t.Error("No commands executed")
	}

	if metrics.AverageLatencyMs == 0 {
		t.Error("Average latency is zero")
	}

	if metrics.ValidationsPassed == 0 && metrics.ValidationsFailed == 0 {
		t.Error("No validations recorded")
	}

	t.Logf("Metrics - Sessions: %d, Commands: %d, Latency: %dms",
		metrics.TotalSessions, metrics.TotalCommands, metrics.AverageLatencyMs)
}

// TestErrorHandling validates error handling across all agents
func TestErrorHandling(t *testing.T) {
	ctx := context.Background()
	config := aider.DefaultSimulatorConfig()
	config.Mode = aider.ErrorMode
	config.ErrorRate = 0.05 // 5% error rate

	result, err := RunHyperAdvancedTests(ctx, config)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	// Even with errors, tests should complete
	if result.TotalTests == 0 {
		t.Error("No tests executed")
	}

	// Some tests should still pass
	if result.PassedTests == 0 {
		t.Error("All tests failed (expected some to pass even with error injection)")
	}

	t.Logf("Error mode: %d/%d tests passed with %.0f%% error rate",
		result.PassedTests, result.TotalTests, config.ErrorRate*100)
}
