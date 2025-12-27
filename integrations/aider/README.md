# Aider Simulation & 10-Agent Concurrent Testing Framework

## Overview

This package provides a **hyper-advanced Aider simulation system** designed for comprehensive testing using **10 concurrent Claude Code web VM agents**, following the best practices outlined in [`CLAUDE.md`](../../CLAUDE.md).

## Architecture

```
integrations/aider/
├── simulator.go          # Core Aider behavior simulation
├── testing/
│   ├── orchestrator.go   # 10-agent coordination & execution
│   ├── agents.go         # 10 specialized test agents
│   ├── runner.go         # Test execution & reporting
│   └── main_test.go      # Go test suite
└── README.md             # This file
```

## Features

### Aider Simulator (`simulator.go`)

- **Simulates Aider without actual installation**
- **Multiple simulation modes:**
  - `FastMode`: Minimal latency for rapid testing (1-10ms)
  - `RealisticMode`: Real-world response times (100-150ms)
  - `StressMode`: High-load conditions (200-400ms)
  - `ErrorMode`: Error injection for resilience testing
- **Comprehensive metrics:** Sessions, commands, latency, throughput
- **Concurrent session support:** Up to 50+ simultaneous sessions
- **File change tracking:** Simulates code modifications
- **Git integration simulation:** Commit and branch tracking

### 10-Agent Concurrent Test Framework

Following the **10-agent concurrent methodology** from `CLAUDE.md`, this framework implements:

#### Agent 1: Command Building & Validation
- Tests Aider command construction
- Validates flag handling and parameters
- Verifies mode-specific command generation

#### Agent 2: Mode Switching & State Management
- Tests switching between `ask`, `architect`, and `code` modes
- Validates state consistency across transitions
- Ensures mode-specific behavior

#### Agent 3: Model Selection Strategies
- Tests fastest/most-capable/round-robin selection
- Validates model capability scoring
- Ensures proper failover

#### Agent 4: Session Lifecycle Management
- Tests session creation, execution, and termination
- Validates cleanup and resource release
- Ensures proper state transitions

#### Agent 5: Concurrent Session Handling
- Tests 10+ simultaneous sessions
- Detects race conditions
- Validates concurrent safety

#### Agent 6: Error Handling & Recovery
- Tests error scenarios and timeouts
- Validates retry mechanisms
- Ensures graceful degradation

#### Agent 7: Git Integration
- Tests file change tracking
- Validates git operations
- Ensures worktree management

#### Agent 8: Configuration Management
- Tests config loading and validation
- Validates parameter handling
- Ensures config hot-reload

#### Agent 9: Performance & Stress Testing
- Runs high-load scenarios
- Measures throughput and latency
- Validates resource limits

#### Agent 10: Integration Patterns & Best Practices
- Tests real-world workflows
- Validates common usage patterns
- Ensures best practices compliance

## Quick Start

### Running Tests

```bash
# Run all tests
go test ./integrations/aider/testing/...

# Run specific test
go test ./integrations/aider/testing/ -run TestHyperAdvanced10AgentConcurrent

# Run with verbose output
go test ./integrations/aider/testing/... -v

# Run benchmarks
go test ./integrations/aider/testing/ -bench=.
```

### Programmatic Usage

```go
package main

import (
    "context"
    "fmt"
    "claude-squad/integrations/aider"
    "claude-squad/integrations/aider/testing"
)

func main() {
    ctx := context.Background()

    // Run fast tests
    result, err := testing.RunFastTests(ctx)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Tests: %d | Passed: %d | Failed: %d\n",
        result.TotalTests, result.PassedTests, result.FailedTests)

    // Run complete suite
    suite, err := testing.RunCompleteSuite(ctx)
    if err != nil {
        panic(err)
    }

    report := testing.GenerateSuiteReport(suite)
    fmt.Println(report)
}
```

### Simulator Usage

```go
package main

import (
    "context"
    "fmt"
    "claude-squad/integrations/aider"
)

func main() {
    ctx := context.Background()

    // Create simulator
    config := aider.DefaultSimulatorConfig()
    simulator, _ := aider.NewAiderSimulator(config)

    // Create session
    session, _ := simulator.CreateSession(ctx, "code", "gpt-4")

    // Execute command
    result, _ := simulator.ExecuteCommand(ctx, session.ID, "add feature")
    fmt.Printf("Modified %d files\n", result.FilesModified)

    // Get metrics
    metrics := simulator.GetMetrics()
    fmt.Printf("Total sessions: %d\n", metrics.TotalSessions)
    fmt.Printf("Avg latency: %dms\n", metrics.AverageLatencyMs)

    // Close session
    simulator.CloseSession(ctx, session.ID)
}
```

## Test Modes

### Fast Mode (Default)
```go
config := &aider.SimulatorConfig{
    Mode:        aider.FastMode,
    BaseLatency: 1,
}
```
- Minimal latency (1-10ms)
- Ideal for rapid development
- Used in CI/CD pipelines

### Realistic Mode
```go
config := &aider.SimulatorConfig{
    Mode:        aider.RealisticMode,
    BaseLatency: 100,
}
```
- Real-world response times (100-150ms)
- Simulates actual Aider behavior
- Best for integration testing

### Stress Mode
```go
config := &aider.SimulatorConfig{
    Mode:                  aider.StressMode,
    BaseLatency:           200,
    MaxConcurrentSessions: 100,
}
```
- High-load conditions (200-400ms)
- Tests resource limits
- Validates scalability

### Error Mode
```go
config := &aider.SimulatorConfig{
    Mode:      aider.ErrorMode,
    ErrorRate: 0.1, // 10% error rate
}
```
- Random error injection
- Tests resilience and recovery
- Validates error handling

## Metrics & Monitoring

### Simulator Metrics
- **Total Sessions**: Number of sessions created
- **Active Sessions**: Currently running sessions
- **Total Commands**: Commands executed
- **Average Latency**: Mean response time (ms)
- **Error Rate**: Percentage of failed operations
- **Throughput**: Operations per second
- **Concurrent Agents**: Number of active test agents

### Orchestrator Metrics
- **Total Agents**: Number of registered agents (should be 10)
- **Test Cases**: Total test scenarios executed
- **Success Rate**: Percentage of passing tests
- **Concurrency Level**: Average concurrent agents
- **Test Throughput**: Tests per second

## Expected Results

### Success Criteria
- **10 agents execute concurrently** ✓
- **Success rate >= 80%** ✓
- **All agents complete without deadlock** ✓
- **Metrics collected accurately** ✓
- **No race conditions detected** ✓

### Sample Output
```
=============================================================================
HYPER-ADVANCED 10-AGENT CONCURRENT AIDER SIMULATION TEST REPORT
=============================================================================

Test Configuration:
  - Total Agents: 10
  - Orchestrator State: completed
  - Total Duration: 2.5s

Overall Results:
  - Total Test Cases: 65
  - Passed: 63 (96.92%)
  - Failed: 2 (3.08%)

Agent Results:
-----------------------------------------------------------------------------

1. Command Building & Validation (command-validation)
   Test Cases: 5 | Passed: 5 | Failed: 0 | Success Rate: 100.00%
   Duration: 250ms

2. Mode Switching & State Management (mode-switching)
   Test Cases: 5 | Passed: 5 | Failed: 0 | Success Rate: 100.00%
   Duration: 275ms

[... 8 more agents ...]

=============================================================================
SIMULATOR METRICS
=============================================================================

Performance:
  - Total Sessions: 85
  - Total Commands: 245
  - Average Latency: 12 ms

Concurrency:
  - Concurrent Agents: 10

Throughput:
  - Command Throughput: 98.0/sec
```

## Best Practices

### 1. Use Appropriate Mode
- **Development**: Fast Mode
- **Integration**: Realistic Mode
- **Load Testing**: Stress Mode
- **Resilience**: Error Mode

### 2. Monitor Metrics
```go
metrics := simulator.GetMetrics()
if metrics.ErrorRate > 0.05 {
    log.Warningf("High error rate: %.2f%%", metrics.ErrorRate*100)
}
```

### 3. Cleanup Resources
```go
defer simulator.CloseSession(ctx, session.ID)
```

### 4. Handle Errors
```go
result, err := simulator.ExecuteCommand(ctx, sessionID, cmd)
if err != nil {
    // Implement retry logic
    return err
}
```

## Performance Benchmarks

```
BenchmarkConcurrentAgents-8     50    25.4 ms/op
BenchmarkSingleAgent-8        1000     1.2 ms/op
```

## Integration with Claude Squad

This simulator integrates with the existing `ollama/aider.go` integration:

```go
import (
    "claude-squad/ollama"
    "claude-squad/integrations/aider"
)

// Use existing Ollama integration
ollamaIntegration, _ := ollama.NewAiderIntegration(framework)
config := ollamaIntegration.CreateSessionConfig(ollama.CodeMode, "model-1")

// Combine with simulator for testing
simulator, _ := aider.NewAiderSimulator(aider.DefaultSimulatorConfig())
session, _ := simulator.CreateSession(ctx, string(config.Mode), config.Model)
```

## Contributing

When adding new test scenarios:

1. **Create a new agent** implementing `AgentExecutor`
2. **Register in `runner.go`** with priority
3. **Add tests** to `main_test.go`
4. **Update documentation**

Example:
```go
type MyNewAgent struct {
    name        string
    description string
}

func (a *MyNewAgent) Execute(ctx context.Context, simulator *aider.AiderSimulator) (*AgentTestResult, error) {
    // Implement test logic
    return result, nil
}
```

## Troubleshooting

### Tests Failing
- Check error rate in simulator config
- Verify concurrency limits
- Review agent timeout settings

### Low Success Rate
- Increase `MaxConcurrentSessions`
- Reduce `ErrorRate`
- Check for resource contention

### Slow Execution
- Use `FastMode` for development
- Reduce `BaseLatency`
- Limit concurrent operations

## References

- [CLAUDE.md](../../CLAUDE.md) - 10-agent concurrent methodology
- [ollama/aider.go](../../ollama/aider.go) - Production Aider integration
- [Aider Documentation](https://aider.chat/docs/)

## License

AGPL-3.0 (same as Claude Squad)
