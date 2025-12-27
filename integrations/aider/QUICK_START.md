# Aider Simulation - Quick Start Guide

## 5-Minute Quick Start

### Run Tests

```bash
# Run the 10-agent concurrent test (fast mode, ~500ms)
go test ./integrations/aider/testing/ -run TestHyperAdvanced10AgentConcurrent -v

# Run complete suite (all modes, ~47s)
go test ./integrations/aider/testing/ -run TestCompleteSuite -v

# Run all tests
go test ./integrations/aider/testing/... -v
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

    // Quick test run
    result, _ := testing.RunFastTests(ctx)
    fmt.Printf("✅ %d/%d tests passed\n", result.PassedTests, result.TotalTests)
}
```

### Basic Simulator Usage

```go
package main

import (
    "context"
    "fmt"
    "claude-squad/integrations/aider"
)

func main() {
    ctx := context.Background()

    // 1. Create simulator
    simulator, _ := aider.NewAiderSimulator(aider.DefaultSimulatorConfig())

    // 2. Create session
    session, _ := simulator.CreateSession(ctx, "code", "gpt-4")

    // 3. Execute command
    result, _ := simulator.ExecuteCommand(ctx, session.ID, "add feature")
    fmt.Printf("Modified %d files in %dms\n", result.FilesModified, result.LatencyMs)

    // 4. Get metrics
    metrics := simulator.GetMetrics()
    fmt.Printf("Total sessions: %d, Avg latency: %dms\n",
        metrics.TotalSessions, metrics.AverageLatencyMs)

    // 5. Close session
    simulator.CloseSession(ctx, session.ID)
}
```

## Test Modes

### Fast Mode (Default) - 1-10ms latency
```go
config := &aider.SimulatorConfig{Mode: aider.FastMode, BaseLatency: 1}
```
**Use for:** Rapid development, CI/CD

### Realistic Mode - 100-150ms latency
```go
config := &aider.SimulatorConfig{Mode: aider.RealisticMode, BaseLatency: 100}
```
**Use for:** Integration testing, pre-production validation

### Stress Mode - 200-400ms latency
```go
config := &aider.SimulatorConfig{Mode: aider.StressMode, BaseLatency: 200}
```
**Use for:** Load testing, scalability validation

### Error Mode - Random failures
```go
config := &aider.SimulatorConfig{Mode: aider.ErrorMode, ErrorRate: 0.1}
```
**Use for:** Resilience testing, error handling validation

## Common Operations

### Run Specific Agent Test

```go
ctx := context.Background()
config := aider.DefaultSimulatorConfig()
simulator, _ := aider.NewAiderSimulator(config)

// Test agent 5 (Concurrent Session Handling)
agent := testing.NewConcurrentSessionAgent()
result, _ := agent.Execute(ctx, simulator)

fmt.Printf("Agent 5: %d/%d tests passed\n", result.Passed, result.TestCases)
```

### Monitor Metrics in Real-Time

```go
simulator.RegisterAgent("my-agent")
defer simulator.UnregisterAgent("my-agent")

// Execute operations
session, _ := simulator.CreateSession(ctx, "code", "model-1")
simulator.ExecuteCommand(ctx, session.ID, "test")
simulator.RecordValidation(true)

// Check metrics
metrics := simulator.GetMetrics()
fmt.Printf("Concurrent agents: %d\n", metrics.ConcurrentAgents)
fmt.Printf("Validations: %d passed, %d failed\n",
    metrics.ValidationsPassed, metrics.ValidationsFailed)
```

### Run Stress Test

```go
simulator, _ := aider.NewAiderSimulator(aider.DefaultSimulatorConfig())

// Run stress test: 50 sessions × 10 commands = 500 operations
result, _ := simulator.StressTest(ctx, 50, 10)

fmt.Printf("Total: %d ops\n", result.TotalOperations)
fmt.Printf("Success: %d\n", result.SuccessCount)
fmt.Printf("Failed: %d\n", result.FailureCount)
fmt.Printf("Throughput: %.2f ops/sec\n", result.Throughput)
```

## Expected Output

### Successful Test Run

```
=== RUN   TestHyperAdvanced10AgentConcurrent

=============================================================================
HYPER-ADVANCED 10-AGENT CONCURRENT AIDER SIMULATION TEST REPORT
=============================================================================

Overall Results:
  - Total Test Cases: 88
  - Passed: 88 (100.00%)
  - Failed: 0 (0.00%)

Agent Results:
-----------------------------------------------------------------------------
1. Command Building & Validation - 5/5 passed ✅
2. Mode Switching & State Management - 5/5 passed ✅
3. Model Selection Strategies - 5/5 passed ✅
4. Session Lifecycle Management - 5/5 passed ✅
5. Concurrent Session Handling - 50/50 passed ✅
6. Error Handling & Recovery - 5/5 passed ✅
7. Git Integration - 5/5 passed ✅
8. Configuration Management - 5/5 passed ✅
9. Performance & Stress Testing - 1/1 passed ✅
10. Integration Patterns & Best Practices - 2/2 passed ✅

--- PASS: TestHyperAdvanced10AgentConcurrent (0.50s)
```

## Troubleshooting

### Tests Take Too Long
**Solution:** Use `FastMode`
```go
config.Mode = aider.FastMode
config.BaseLatency = 1
```

### Tests Failing
**Solution:** Check error rate
```go
config.ErrorRate = 0.0  // Disable error injection
```

### Want More Concurrency
**Solution:** Increase session limit
```go
config.MaxConcurrentSessions = 100
```

## Integration Examples

### With Ollama Integration

```go
import (
    "claude-squad/ollama"
    "claude-squad/integrations/aider"
)

// Get Ollama config
ollamaIntegration, _ := ollama.NewAiderIntegration(framework)
sessionConfig := ollamaIntegration.CreateSessionConfig(ollama.CodeMode, "model-1")

// Use in simulator
simulator, _ := aider.NewAiderSimulator(aider.DefaultSimulatorConfig())
session, _ := simulator.CreateSession(ctx, string(sessionConfig.Mode), sessionConfig.Model)
```

### With Session Management

```go
import "claude-squad/session"

// Simulate Aider session creation
opts := session.InstanceOptions{
    Program: "aider",
    Title:   "test-session",
}

// Use simulator to test before creating actual instance
simulator.CreateSession(ctx, "code", "test-model")
```

## Next Steps

1. **Read Full Documentation:** [README.md](README.md)
2. **Review Test Results:** [AIDER_SIMULATION_TEST_RESULTS.md](../../AIDER_SIMULATION_TEST_RESULTS.md)
3. **Study Methodology:** [CLAUDE.md](../../CLAUDE.md)
4. **Run Benchmarks:** `go test -bench=. ./integrations/aider/testing/`

## Useful Commands

```bash
# Run with coverage
go test ./integrations/aider/testing/... -cover

# Run with race detection
go test ./integrations/aider/testing/... -race

# Run specific test
go test ./integrations/aider/testing/ -run TestFastMode -v

# Generate coverage report
go test ./integrations/aider/testing/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Support

For issues or questions:
1. Check [README.md](README.md) for detailed documentation
2. Review [AIDER_SIMULATION_TEST_RESULTS.md](../../AIDER_SIMULATION_TEST_RESULTS.md) for expected behavior
3. See example usage in [example_usage.go](example_usage.go)

---

**Ready to test?** Run: `go test ./integrations/aider/testing/ -run TestHyperAdvanced10AgentConcurrent -v`
