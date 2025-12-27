// Package aider provides example usage of the Aider simulation and testing framework
package aider

import (
	"context"
	"fmt"
	"time"
)

// ExampleBasicSimulation demonstrates basic simulator usage
func ExampleBasicSimulation() {
	ctx := context.Background()

	// Create simulator with default config
	config := DefaultSimulatorConfig()
	simulator, err := NewAiderSimulator(config)
	if err != nil {
		panic(err)
	}

	// Create a session
	session, err := simulator.CreateSession(ctx, "code", "gpt-4")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Created session: %s\n", session.ID)

	// Execute commands
	commands := []string{
		"add authentication feature",
		"write unit tests",
		"optimize performance",
	}

	for _, cmd := range commands {
		result, err := simulator.ExecuteCommand(ctx, session.ID, cmd)
		if err != nil {
			fmt.Printf("Command failed: %v\n", err)
			continue
		}

		fmt.Printf("Command: %s | Files Modified: %d | Latency: %dms\n",
			cmd, result.FilesModified, result.LatencyMs)
	}

	// Close session
	err = simulator.CloseSession(ctx, session.ID)
	if err != nil {
		panic(err)
	}

	// Get metrics
	metrics := simulator.GetMetrics()
	fmt.Printf("\nMetrics:\n")
	fmt.Printf("  Total Sessions: %d\n", metrics.TotalSessions)
	fmt.Printf("  Total Commands: %d\n", metrics.TotalCommands)
	fmt.Printf("  Average Latency: %dms\n", metrics.AverageLatencyMs)
}

// ExampleConcurrentSessions demonstrates handling multiple sessions
func ExampleConcurrentSessions() {
	ctx := context.Background()
	config := DefaultSimulatorConfig()
	config.Mode = FastMode
	simulator, _ := NewAiderSimulator(config)

	// Create multiple sessions
	sessions := make([]*SimulatedSession, 5)
	for i := 0; i < 5; i++ {
		session, err := simulator.CreateSession(ctx, "code", fmt.Sprintf("model-%d", i))
		if err != nil {
			panic(err)
		}
		sessions[i] = session
		fmt.Printf("Created session %d: %s\n", i+1, session.ID)
	}

	// Execute commands in each session
	for i, session := range sessions {
		result, err := simulator.ExecuteCommand(ctx, session.ID, fmt.Sprintf("task-%d", i))
		if err != nil {
			fmt.Printf("Session %d error: %v\n", i, err)
			continue
		}
		fmt.Printf("Session %d: %d files modified\n", i+1, result.FilesModified)
	}

	// Close all sessions
	for _, session := range sessions {
		simulator.CloseSession(ctx, session.ID)
	}

	fmt.Printf("\nActive sessions: %d\n", simulator.GetMetrics().ActiveSessions)
}

// ExampleStressTest demonstrates stress testing capabilities
func ExampleStressTest() {
	ctx := context.Background()
	config := DefaultSimulatorConfig()
	config.Mode = StressMode
	simulator, _ := NewAiderSimulator(config)

	fmt.Println("Running stress test...")
	startTime := time.Now()

	result, err := simulator.StressTest(ctx, 50, 10)
	if err != nil {
		panic(err)
	}

	fmt.Printf("\nStress Test Results:\n")
	fmt.Printf("  Total Operations: %d\n", result.TotalOperations)
	fmt.Printf("  Success: %d\n", result.SuccessCount)
	fmt.Printf("  Failed: %d\n", result.FailureCount)
	fmt.Printf("  Duration: %s\n", result.Duration)
	fmt.Printf("  Throughput: %.2f ops/sec\n", result.Throughput)
	fmt.Printf("  Actual Duration: %s\n", time.Since(startTime))
}

// ExampleModeComparison compares different simulation modes
func ExampleModeComparison() {
	ctx := context.Background()
	modes := []SimulationMode{FastMode, RealisticMode, StressMode}

	for _, mode := range modes {
		config := DefaultSimulatorConfig()
		config.Mode = mode
		simulator, _ := NewAiderSimulator(config)

		// Create session and execute command
		session, _ := simulator.CreateSession(ctx, "code", "test-model")
		startTime := time.Now()
		result, _ := simulator.ExecuteCommand(ctx, session.ID, "test command")
		duration := time.Since(startTime)

		fmt.Printf("Mode: %s | Latency: %dms | Actual: %s\n",
			mode, result.LatencyMs, duration)

		simulator.CloseSession(ctx, session.ID)
	}
}

// ExampleMetricsMonitoring demonstrates real-time metrics monitoring
func ExampleMetricsMonitoring() {
	ctx := context.Background()
	config := DefaultSimulatorConfig()
	config.Mode = FastMode
	simulator, _ := NewAiderSimulator(config)

	// Register as an agent
	simulator.RegisterAgent("monitoring-agent")
	defer simulator.UnregisterAgent("monitoring-agent")

	// Create and execute operations
	for i := 0; i < 10; i++ {
		session, _ := simulator.CreateSession(ctx, "code", fmt.Sprintf("model-%d", i))

		for j := 0; j < 3; j++ {
			_, err := simulator.ExecuteCommand(ctx, session.ID, fmt.Sprintf("cmd-%d", j))
			if err == nil {
				simulator.RecordValidation(true)
			} else {
				simulator.RecordValidation(false)
			}
		}

		simulator.CloseSession(ctx, session.ID)

		// Print metrics every 3 iterations
		if (i+1)%3 == 0 {
			metrics := simulator.GetMetrics()
			fmt.Printf("\nIteration %d Metrics:\n", i+1)
			fmt.Printf("  Sessions: %d | Commands: %d | Validations: %d passed, %d failed\n",
				metrics.TotalSessions, metrics.TotalCommands,
				metrics.ValidationsPassed, metrics.ValidationsFailed)
		}
	}

	// Final metrics
	finalMetrics := simulator.GetMetrics()
	fmt.Printf("\nFinal Metrics:\n")
	fmt.Printf("  Total Sessions: %d\n", finalMetrics.TotalSessions)
	fmt.Printf("  Total Commands: %d\n", finalMetrics.TotalCommands)
	fmt.Printf("  Average Latency: %dms\n", finalMetrics.AverageLatencyMs)
	fmt.Printf("  Command Throughput: %.2f/sec\n", finalMetrics.CommandThroughput)
	fmt.Printf("  Uptime: %s\n", finalMetrics.Uptime)
}

// ExampleErrorHandling demonstrates error handling and recovery
func ExampleErrorHandling() {
	ctx := context.Background()
	config := DefaultSimulatorConfig()
	config.Mode = ErrorMode
	config.ErrorRate = 0.2 // 20% error rate
	simulator, _ := NewAiderSimulator(config)

	attempts := 10
	successes := 0
	failures := 0

	fmt.Println("Testing error handling (20% error rate)...")

	for i := 0; i < attempts; i++ {
		session, err := simulator.CreateSession(ctx, "code", fmt.Sprintf("model-%d", i))
		if err != nil {
			failures++
			fmt.Printf("Attempt %d: Session creation failed\n", i+1)
			continue
		}

		_, err = simulator.ExecuteCommand(ctx, session.ID, "test command")
		if err != nil {
			failures++
			fmt.Printf("Attempt %d: Command execution failed\n", i+1)
		} else {
			successes++
			fmt.Printf("Attempt %d: Success\n", i+1)
		}

		simulator.CloseSession(ctx, session.ID)
	}

	fmt.Printf("\nResults: %d successes, %d failures (%.1f%% failure rate)\n",
		successes, failures, float64(failures)/float64(attempts)*100)
}

// ExampleSessionLifecycle demonstrates complete session lifecycle
func ExampleSessionLifecycle() {
	ctx := context.Background()
	config := DefaultSimulatorConfig()
	simulator, _ := NewAiderSimulator(config)

	fmt.Println("Session Lifecycle Example")
	fmt.Println("========================\n")

	// 1. Create session
	fmt.Println("1. Creating session...")
	session, err := simulator.CreateSession(ctx, "code", "gpt-4")
	if err != nil {
		panic(err)
	}
	fmt.Printf("   Session created: %s (mode: %s, model: %s)\n",
		session.ID, session.Mode, session.Model)

	// 2. Check initial state
	fmt.Println("\n2. Initial state:")
	fmt.Printf("   Status: %s\n", session.Status)
	fmt.Printf("   Command Count: %d\n", session.CommandCount)

	// 3. Execute commands
	fmt.Println("\n3. Executing commands...")
	commands := []string{"add feature", "write tests", "refactor"}
	for i, cmd := range commands {
		result, err := simulator.ExecuteCommand(ctx, session.ID, cmd)
		if err != nil {
			fmt.Printf("   Command %d failed: %v\n", i+1, err)
			continue
		}
		fmt.Printf("   Command %d: '%s' -> %d files modified\n",
			i+1, cmd, result.FilesModified)
	}

	// 4. Check updated state
	fmt.Println("\n4. Updated state:")
	activeSessions := simulator.GetActiveSessions()
	for _, s := range activeSessions {
		if s.ID == session.ID {
			fmt.Printf("   Command Count: %d\n", s.CommandCount)
			fmt.Printf("   Status: %s\n", s.Status)
			break
		}
	}

	// 5. Close session
	fmt.Println("\n5. Closing session...")
	err = simulator.CloseSession(ctx, session.ID)
	if err != nil {
		panic(err)
	}
	fmt.Println("   Session closed successfully")

	// 6. Final metrics
	fmt.Println("\n6. Final metrics:")
	metrics := simulator.GetMetrics()
	fmt.Printf("   Total Sessions: %d\n", metrics.TotalSessions)
	fmt.Printf("   Active Sessions: %d\n", metrics.ActiveSessions)
	fmt.Printf("   Total Commands: %d\n", metrics.TotalCommands)
}
