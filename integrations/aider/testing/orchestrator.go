// Package testing provides a 10-agent concurrent test framework for Aider simulation
// following the hyper-advanced methodology described in CLAUDE.md
package testing

import (
	"claude-squad/integrations/aider"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// TestOrchestrator coordinates 10 concurrent test agents
type TestOrchestrator struct {
	mu                sync.RWMutex
	simulator         *aider.AiderSimulator
	agents            []*TestAgent
	results           map[string]*AgentTestResult
	startTime         time.Time
	endTime           time.Time
	totalTests        int32
	passedTests       int32
	failedTests       int32
	orchestratorState string
	metrics           *OrchestratorMetrics
}

// OrchestratorMetrics tracks orchestrator-level metrics
type OrchestratorMetrics struct {
	TotalAgents       int
	ActiveAgents      int32
	TotalTestCases    int32
	PassedTestCases   int32
	FailedTestCases   int32
	TotalDuration     time.Duration
	AgentUtilization  float64
	ConcurrencyLevel  float64
	TestThroughput    float64
}

// AgentTestResult contains results from a single agent
type AgentTestResult struct {
	AgentID          string
	AgentType        string
	TestCases        int
	Passed           int
	Failed           int
	Duration         time.Duration
	Errors           []string
	Validations      []string
	PerformanceStats map[string]interface{}
}

// TestAgent represents a single testing agent
type TestAgent struct {
	ID              string
	Type            string
	Name            string
	Description     string
	Executor        AgentExecutor
	Priority        int
	Config          *AgentConfig
	StartTime       time.Time
	EndTime         time.Time
	Status          string
	mu              sync.RWMutex
}

// AgentExecutor defines the interface for agent test execution
type AgentExecutor interface {
	Execute(ctx context.Context, simulator *aider.AiderSimulator) (*AgentTestResult, error)
	GetName() string
	GetDescription() string
}

// AgentConfig configures individual agent behavior
type AgentConfig struct {
	Timeout       time.Duration
	MaxRetries    int
	ConcurrentOps int
	EnableMetrics bool
}

// NewTestOrchestrator creates a new 10-agent test orchestrator
func NewTestOrchestrator(simulator *aider.AiderSimulator) (*TestOrchestrator, error) {
	if simulator == nil {
		return nil, fmt.Errorf("simulator cannot be nil")
	}

	return &TestOrchestrator{
		simulator:         simulator,
		agents:            make([]*TestAgent, 0, 10),
		results:           make(map[string]*AgentTestResult),
		orchestratorState: "initialized",
		metrics:           &OrchestratorMetrics{},
	}, nil
}

// RegisterAgent registers a test agent with the orchestrator
func (to *TestOrchestrator) RegisterAgent(agent *TestAgent) error {
	to.mu.Lock()
	defer to.mu.Unlock()

	if len(to.agents) >= 10 {
		return fmt.Errorf("maximum of 10 agents already registered")
	}

	if agent.Config == nil {
		agent.Config = DefaultAgentConfig()
	}

	to.agents = append(to.agents, agent)
	to.metrics.TotalAgents = len(to.agents)

	return nil
}

// DefaultAgentConfig returns default agent configuration
func DefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		Timeout:       5 * time.Minute,
		MaxRetries:    3,
		ConcurrentOps: 10,
		EnableMetrics: true,
	}
}

// Execute runs all 10 agents concurrently
func (to *TestOrchestrator) Execute(ctx context.Context) (*OrchestratorResult, error) {
	to.mu.Lock()
	if len(to.agents) != 10 {
		to.mu.Unlock()
		return nil, fmt.Errorf("exactly 10 agents required, got %d", len(to.agents))
	}
	to.orchestratorState = "running"
	to.startTime = time.Now()
	to.mu.Unlock()

	// Create wait group for all agents
	var wg sync.WaitGroup
	resultsChan := make(chan *AgentTestResult, 10)
	errorsChan := make(chan error, 10)

	// Launch all 10 agents concurrently
	for i, agent := range to.agents {
		wg.Add(1)
		agentNum := i + 1

		go func(ag *TestAgent, num int) {
			defer wg.Done()

			// Register agent with simulator
			to.simulator.RegisterAgent(ag.ID)
			defer to.simulator.UnregisterAgent(ag.ID)

			// Update agent status
			ag.mu.Lock()
			ag.Status = "running"
			ag.StartTime = time.Now()
			ag.mu.Unlock()

			atomic.AddInt32(&to.metrics.ActiveAgents, 1)

			// Create agent context with timeout
			agentCtx, cancel := context.WithTimeout(ctx, ag.Config.Timeout)
			defer cancel()

			// Execute agent tests
			result, err := ag.Executor.Execute(agentCtx, to.simulator)

			atomic.AddInt32(&to.metrics.ActiveAgents, -1)

			// Update agent status
			ag.mu.Lock()
			ag.EndTime = time.Now()
			ag.Status = "completed"
			ag.mu.Unlock()

			if err != nil {
				errorsChan <- fmt.Errorf("agent %d (%s) error: %w", num, ag.Name, err)
				// Still send result if available
				if result != nil {
					resultsChan <- result
				}
			} else {
				resultsChan <- result
			}
		}(agent, agentNum)
	}

	// Wait for all agents to complete
	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorsChan)
	}()

	// Collect results
	errors := make([]string, 0)
	for err := range errorsChan {
		errors = append(errors, err.Error())
	}

	results := make([]*AgentTestResult, 0, 10)
	for result := range resultsChan {
		results = append(results, result)
		to.mu.Lock()
		to.results[result.AgentID] = result
		to.mu.Unlock()

		atomic.AddInt32(&to.totalTests, int32(result.TestCases))
		atomic.AddInt32(&to.passedTests, int32(result.Passed))
		atomic.AddInt32(&to.failedTests, int32(result.Failed))
	}

	to.mu.Lock()
	to.endTime = time.Now()
	to.orchestratorState = "completed"
	to.mu.Unlock()

	// Calculate final metrics
	duration := time.Since(to.startTime)
	metrics := to.calculateMetrics(duration)

	return &OrchestratorResult{
		AgentResults:  results,
		TotalTests:    int(to.totalTests),
		PassedTests:   int(to.passedTests),
		FailedTests:   int(to.failedTests),
		Duration:      duration,
		Errors:        errors,
		Metrics:       metrics,
		SimulatorData: to.simulator.GetMetrics(),
	}, nil
}

// calculateMetrics calculates final orchestrator metrics
func (to *TestOrchestrator) calculateMetrics(duration time.Duration) *OrchestratorMetrics {
	totalTests := atomic.LoadInt32(&to.totalTests)
	passedTests := atomic.LoadInt32(&to.passedTests)
	failedTests := atomic.LoadInt32(&to.failedTests)

	throughput := 0.0
	if duration.Seconds() > 0 {
		throughput = float64(totalTests) / duration.Seconds()
	}

	// Calculate average concurrency (all 10 agents ran in parallel)
	concurrencyLevel := 10.0

	return &OrchestratorMetrics{
		TotalAgents:      10,
		ActiveAgents:     0,
		TotalTestCases:   totalTests,
		PassedTestCases:  passedTests,
		FailedTestCases:  failedTests,
		TotalDuration:    duration,
		AgentUtilization: 100.0, // All 10 agents used
		ConcurrencyLevel: concurrencyLevel,
		TestThroughput:   throughput,
	}
}

// OrchestratorResult contains the complete test results
type OrchestratorResult struct {
	AgentResults  []*AgentTestResult
	TotalTests    int
	PassedTests   int
	FailedTests   int
	Duration      time.Duration
	Errors        []string
	Metrics       *OrchestratorMetrics
	SimulatorData *aider.SimulatorMetrics
}

// GenerateReport generates a comprehensive test report
func (to *TestOrchestrator) GenerateReport() string {
	to.mu.RLock()
	defer to.mu.RUnlock()

	report := fmt.Sprintf(`
=============================================================================
HYPER-ADVANCED 10-AGENT CONCURRENT AIDER SIMULATION TEST REPORT
=============================================================================

Test Configuration:
  - Total Agents: %d
  - Orchestrator State: %s
  - Start Time: %s
  - End Time: %s
  - Total Duration: %s

Overall Results:
  - Total Test Cases: %d
  - Passed: %d (%.2f%%)
  - Failed: %d (%.2f%%)

`,
		len(to.agents),
		to.orchestratorState,
		to.startTime.Format(time.RFC3339),
		to.endTime.Format(time.RFC3339),
		to.endTime.Sub(to.startTime),
		to.totalTests,
		to.passedTests,
		float64(to.passedTests)/float64(to.totalTests)*100,
		to.failedTests,
		float64(to.failedTests)/float64(to.totalTests)*100,
	)

	report += "\nAgent Results:\n"
	report += "-----------------------------------------------------------------------------\n"

	for i, agent := range to.agents {
		result, exists := to.results[agent.ID]
		if !exists {
			report += fmt.Sprintf("%d. %s [NO RESULTS]\n", i+1, agent.Name)
			continue
		}

		successRate := 0.0
		if result.TestCases > 0 {
			successRate = float64(result.Passed) / float64(result.TestCases) * 100
		}

		report += fmt.Sprintf(`
%d. %s (%s)
   Description: %s
   Test Cases: %d | Passed: %d | Failed: %d | Success Rate: %.2f%%
   Duration: %s
   Status: %s
`,
			i+1,
			agent.Name,
			agent.Type,
			agent.Description,
			result.TestCases,
			result.Passed,
			result.Failed,
			successRate,
			result.Duration,
			agent.Status,
		)
	}

	// Add simulator metrics
	simMetrics := to.simulator.GetMetrics()
	report += fmt.Sprintf(`
=============================================================================
SIMULATOR METRICS
=============================================================================

Performance:
  - Total Sessions: %d
  - Active Sessions: %d
  - Total Commands: %d
  - Total Errors: %d
  - Average Latency: %d ms

Concurrency:
  - Concurrent Agents: %d
  - Test Scenarios: %d

Validation:
  - Passed: %d
  - Failed: %d

Throughput:
  - Session Create Rate: %.2f/sec
  - Command Throughput: %.2f/sec
  - Error Rate: %.4f

Uptime: %s

=============================================================================
`,
		simMetrics.TotalSessions,
		simMetrics.ActiveSessions,
		simMetrics.TotalCommands,
		simMetrics.TotalErrors,
		simMetrics.AverageLatencyMs,
		simMetrics.ConcurrentAgents,
		simMetrics.TestScenarios,
		simMetrics.ValidationsPassed,
		simMetrics.ValidationsFailed,
		simMetrics.SessionCreateRate,
		simMetrics.CommandThroughput,
		simMetrics.ErrorRate,
		simMetrics.Uptime,
	)

	return report
}

// GetResults returns all agent results
func (to *TestOrchestrator) GetResults() map[string]*AgentTestResult {
	to.mu.RLock()
	defer to.mu.RUnlock()

	resultsCopy := make(map[string]*AgentTestResult)
	for k, v := range to.results {
		resultsCopy[k] = v
	}
	return resultsCopy
}

// GetAgent returns an agent by ID
func (to *TestOrchestrator) GetAgent(id string) (*TestAgent, error) {
	to.mu.RLock()
	defer to.mu.RUnlock()

	for _, agent := range to.agents {
		if agent.ID == id {
			return agent, nil
		}
	}
	return nil, fmt.Errorf("agent not found: %s", id)
}

// GetAgentCount returns the number of registered agents
func (to *TestOrchestrator) GetAgentCount() int {
	to.mu.RLock()
	defer to.mu.RUnlock()
	return len(to.agents)
}
