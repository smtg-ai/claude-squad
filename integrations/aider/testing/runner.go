// Package testing provides the main test runner for executing 10-agent concurrent tests
package testing

import (
	"claude-squad/integrations/aider"
	"context"
	"fmt"
)

// RunHyperAdvancedTests executes the complete 10-agent concurrent test suite
// following the methodology described in CLAUDE.md
func RunHyperAdvancedTests(ctx context.Context, config *aider.SimulatorConfig) (*OrchestratorResult, error) {
	// Create simulator
	simulator, err := aider.NewAiderSimulator(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create simulator: %w", err)
	}

	// Create orchestrator
	orchestrator, err := NewTestOrchestrator(simulator)
	if err != nil {
		return nil, fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Register all 10 agents
	agents := []*TestAgent{
		{
			ID:          "agent-1",
			Type:        "command-validation",
			Name:        "Command Building & Validation",
			Description: "Tests Aider command construction and validation",
			Executor:    NewCommandValidationAgent(),
			Priority:    1,
			Config:      DefaultAgentConfig(),
		},
		{
			ID:          "agent-2",
			Type:        "mode-switching",
			Name:        "Mode Switching & State Management",
			Description: "Tests mode transitions and state consistency",
			Executor:    NewModeSwitchingAgent(),
			Priority:    2,
			Config:      DefaultAgentConfig(),
		},
		{
			ID:          "agent-3",
			Type:        "model-selection",
			Name:        "Model Selection Strategies",
			Description: "Tests model selection algorithms",
			Executor:    NewModelSelectionAgent(),
			Priority:    3,
			Config:      DefaultAgentConfig(),
		},
		{
			ID:          "agent-4",
			Type:        "session-lifecycle",
			Name:        "Session Lifecycle Management",
			Description: "Tests session creation, execution, and termination",
			Executor:    NewSessionLifecycleAgent(),
			Priority:    4,
			Config:      DefaultAgentConfig(),
		},
		{
			ID:          "agent-5",
			Type:        "concurrent-sessions",
			Name:        "Concurrent Session Handling",
			Description: "Tests concurrent operations and race conditions",
			Executor:    NewConcurrentSessionAgent(),
			Priority:    5,
			Config:      DefaultAgentConfig(),
		},
		{
			ID:          "agent-6",
			Type:        "error-handling",
			Name:        "Error Handling & Recovery",
			Description: "Tests error scenarios and recovery",
			Executor:    NewErrorHandlingAgent(),
			Priority:    6,
			Config:      DefaultAgentConfig(),
		},
		{
			ID:          "agent-7",
			Type:        "git-integration",
			Name:        "Git Integration",
			Description: "Tests git operations and file tracking",
			Executor:    NewGitIntegrationAgent(),
			Priority:    7,
			Config:      DefaultAgentConfig(),
		},
		{
			ID:          "agent-8",
			Type:        "config-management",
			Name:        "Configuration Management",
			Description: "Tests configuration validation",
			Executor:    NewConfigManagementAgent(),
			Priority:    8,
			Config:      DefaultAgentConfig(),
		},
		{
			ID:          "agent-9",
			Type:        "performance-stress",
			Name:        "Performance & Stress Testing",
			Description: "Tests performance under load",
			Executor:    NewPerformanceStressAgent(),
			Priority:    9,
			Config:      DefaultAgentConfig(),
		},
		{
			ID:          "agent-10",
			Type:        "integration-patterns",
			Name:        "Integration Patterns & Best Practices",
			Description: "Tests real-world usage patterns",
			Executor:    NewIntegrationPatternsAgent(),
			Priority:    10,
			Config:      DefaultAgentConfig(),
		},
	}

	// Register all agents
	for _, agent := range agents {
		if err := orchestrator.RegisterAgent(agent); err != nil {
			return nil, fmt.Errorf("failed to register agent %s: %w", agent.Name, err)
		}
	}

	// Execute all agents concurrently
	result, err := orchestrator.Execute(ctx)
	if err != nil {
		return nil, fmt.Errorf("test execution failed: %w", err)
	}

	return result, nil
}

// RunWithDefaultConfig runs tests with default simulator configuration
func RunWithDefaultConfig(ctx context.Context) (*OrchestratorResult, error) {
	config := aider.DefaultSimulatorConfig()
	return RunHyperAdvancedTests(ctx, config)
}

// RunFastTests runs tests in fast mode for rapid validation
func RunFastTests(ctx context.Context) (*OrchestratorResult, error) {
	config := aider.DefaultSimulatorConfig()
	config.Mode = aider.FastMode
	config.BaseLatency = 1
	return RunHyperAdvancedTests(ctx, config)
}

// RunRealisticTests runs tests in realistic mode
func RunRealisticTests(ctx context.Context) (*OrchestratorResult, error) {
	config := aider.DefaultSimulatorConfig()
	config.Mode = aider.RealisticMode
	config.BaseLatency = 100
	return RunHyperAdvancedTests(ctx, config)
}

// RunStressTests runs tests in stress mode
func RunStressTests(ctx context.Context) (*OrchestratorResult, error) {
	config := aider.DefaultSimulatorConfig()
	config.Mode = aider.StressMode
	config.BaseLatency = 200
	config.MaxConcurrentSessions = 100
	return RunHyperAdvancedTests(ctx, config)
}

// RunErrorTests runs tests with error injection
func RunErrorTests(ctx context.Context) (*OrchestratorResult, error) {
	config := aider.DefaultSimulatorConfig()
	config.Mode = aider.ErrorMode
	config.ErrorRate = 0.1 // 10% error rate
	return RunHyperAdvancedTests(ctx, config)
}

// TestSuite represents a complete test suite with multiple runs
type TestSuite struct {
	Name    string
	Runs    []*TestRun
	Summary *TestSuiteSummary
}

// TestRun represents a single test execution
type TestRun struct {
	Name      string
	Mode      aider.SimulationMode
	Result    *OrchestratorResult
	StartTime string
	EndTime   string
}

// TestSuiteSummary provides aggregated results
type TestSuiteSummary struct {
	TotalRuns        int
	TotalTests       int
	TotalPassed      int
	TotalFailed      int
	OverallSuccessRate float64
	FastestRun       string
	SlowestRun       string
}

// RunCompleteSuite runs all test modes and generates comprehensive report
func RunCompleteSuite(ctx context.Context) (*TestSuite, error) {
	suite := &TestSuite{
		Name: "Hyper-Advanced 10-Agent Concurrent Aider Simulation Test Suite",
		Runs: make([]*TestRun, 0),
	}

	testConfigs := []struct {
		name string
		mode aider.SimulationMode
	}{
		{"Fast Mode", aider.FastMode},
		{"Realistic Mode", aider.RealisticMode},
		{"Stress Mode", aider.StressMode},
	}

	totalTests := 0
	totalPassed := 0
	totalFailed := 0

	for _, tc := range testConfigs {
		config := aider.DefaultSimulatorConfig()
		config.Mode = tc.mode

		if tc.mode == aider.FastMode {
			config.BaseLatency = 1
		} else if tc.mode == aider.RealisticMode {
			config.BaseLatency = 100
		} else if tc.mode == aider.StressMode {
			config.BaseLatency = 200
			config.MaxConcurrentSessions = 100
		}

		result, err := RunHyperAdvancedTests(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("failed to run %s: %w", tc.name, err)
		}

		run := &TestRun{
			Name:   tc.name,
			Mode:   tc.mode,
			Result: result,
		}

		suite.Runs = append(suite.Runs, run)

		totalTests += result.TotalTests
		totalPassed += result.PassedTests
		totalFailed += result.FailedTests
	}

	suite.Summary = &TestSuiteSummary{
		TotalRuns:          len(suite.Runs),
		TotalTests:         totalTests,
		TotalPassed:        totalPassed,
		TotalFailed:        totalFailed,
		OverallSuccessRate: float64(totalPassed) / float64(totalTests) * 100,
	}

	return suite, nil
}

// GenerateSuiteReport generates a comprehensive suite report
func GenerateSuiteReport(suite *TestSuite) string {
	report := fmt.Sprintf(`
=============================================================================
%s
=============================================================================

Suite Summary:
  - Total Test Runs: %d
  - Total Test Cases: %d
  - Total Passed: %d
  - Total Failed: %d
  - Overall Success Rate: %.2f%%

`, suite.Name, suite.Summary.TotalRuns, suite.Summary.TotalTests,
		suite.Summary.TotalPassed, suite.Summary.TotalFailed,
		suite.Summary.OverallSuccessRate)

	for i, run := range suite.Runs {
		successRate := 0.0
		if run.Result.TotalTests > 0 {
			successRate = float64(run.Result.PassedTests) / float64(run.Result.TotalTests) * 100
		}

		report += fmt.Sprintf(`
-----------------------------------------------------------------------------
Run %d: %s
-----------------------------------------------------------------------------
  Mode: %s
  Total Tests: %d
  Passed: %d
  Failed: %d
  Success Rate: %.2f%%
  Duration: %s

  Orchestrator Metrics:
    - Total Agents: %d
    - Test Throughput: %.2f tests/sec
    - Concurrency Level: %.1f

  Simulator Metrics:
    - Total Sessions: %d
    - Total Commands: %d
    - Average Latency: %d ms
    - Error Rate: %.4f
`, i+1, run.Name, run.Mode, run.Result.TotalTests,
			run.Result.PassedTests, run.Result.FailedTests, successRate,
			run.Result.Duration,
			run.Result.Metrics.TotalAgents,
			run.Result.Metrics.TestThroughput,
			run.Result.Metrics.ConcurrencyLevel,
			run.Result.SimulatorData.TotalSessions,
			run.Result.SimulatorData.TotalCommands,
			run.Result.SimulatorData.AverageLatencyMs,
			run.Result.SimulatorData.ErrorRate)
	}

	report += "\n=============================================================================\n"
	return report
}
