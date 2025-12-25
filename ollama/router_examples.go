package ollama

import (
	"claude-squad/log"
	"claude-squad/session"
	"context"
	"fmt"
	"strings"
	"time"
)

// Example1_BasicRoundRobinRouting demonstrates basic round-robin routing
func Example1_BasicRoundRobinRouting() error {
	// Create router with round-robin strategy
	router := NewTaskRouter(StrategyRoundRobin)

	// Register models (in real usage, these would be actual instances)
	models := map[string]string{
		"model-gemma-1b":    "Lightweight coding model",
		"model-llama-7b":    "Medium-sized general model",
		"model-mistral-12b": "Larger more capable model",
	}

	for modelID := range models {
		// Create dummy instances for demonstration
		dummyInstance, _ := session.NewInstance(session.InstanceOptions{
			Title:   modelID,
			Path:    "/tmp/" + modelID,
			Program: "claude",
		})

		if err := router.RegisterModel(modelID, dummyInstance); err != nil {
			return fmt.Errorf("failed to register model: %w", err)
		}
	}

	// Route multiple tasks
	tasks := []string{
		"Implement a binary search algorithm",
		"Write unit tests for the parser",
		"Refactor the database connection pool",
		"Document the API endpoints",
	}

	fmt.Println("=== Round-Robin Routing Example ===")
	for i, task := range tasks {
		selectedModel, err := router.RouteTask(context.Background(), task)
		if err != nil {
			return err
		}

		// Simulate task execution
		success := i%2 == 0 // Even tasks succeed
		latency := time.Duration((i+1)*50) * time.Millisecond

		category := router.GetTaskCategory(task)
		if err := router.RecordTaskResult(selectedModel, success, latency, category); err != nil {
			return err
		}

		status := "SUCCESS"
		if !success {
			status = "FAILED"
		}

		fmt.Printf(
			"Task %d: '%s'\n  -> Routed to: %s\n  -> Category: %s\n  -> Result: %s\n\n",
			i+1, task, selectedModel, category, status,
		)
	}

	// Display metrics
	fmt.Println("=== Final Metrics ===")
	metrics := router.GetAllMetrics()
	for modelID, m := range metrics {
		successRate := float64(0)
		if m.TotalRequests > 0 {
			successRate = float64(m.SuccessfulTasks) / float64(m.TotalRequests) * 100
		}
		fmt.Printf(
			"Model: %s\n  Requests: %d | Success Rate: %.1f%% | Avg Latency: %v\n\n",
			modelID, m.TotalRequests, successRate, m.AverageLatency,
		)
	}

	return nil
}

// Example2_PerformanceBasedRouting demonstrates performance-based routing
func Example2_PerformanceBasedRouting() error {
	// Create router with performance-based strategy
	router := NewTaskRouter(StrategyPerformance)

	// Register models
	modelConfigs := []struct {
		id      string
		latency time.Duration
	}{
		{"fast-model", 50 * time.Millisecond},
		{"medium-model", 150 * time.Millisecond},
		{"slow-model", 500 * time.Millisecond},
	}

	for _, config := range modelConfigs {
		dummyInstance, _ := session.NewInstance(session.InstanceOptions{
			Title:   config.id,
			Path:    "/tmp/" + config.id,
			Program: "claude",
		})
		router.RegisterModel(config.id, dummyInstance)
	}

	fmt.Println("=== Performance-Based Routing Example ===")

	// Simulate multiple task executions to build performance history
	tasks := []struct {
		prompt  string
		success bool
		model   string
		latency time.Duration
	}{
		{"Implement feature X", true, "fast-model", 50 * time.Millisecond},
		{"Fix bug in parser", true, "medium-model", 150 * time.Millisecond},
		{"Add logging", false, "slow-model", 500 * time.Millisecond},
		{"Write tests", true, "fast-model", 55 * time.Millisecond},
		{"Refactor code", true, "medium-model", 160 * time.Millisecond},
		{"Update docs", false, "slow-model", 510 * time.Millisecond},
	}

	// Build performance history
	for _, task := range tasks {
		category := router.GetTaskCategory(task.prompt)
		router.RecordTaskResult(task.model, task.success, task.latency, category)
	}

	// Now route new tasks - should prefer fast-model
	newTasks := []string{
		"Implement a new feature",
		"Write comprehensive documentation",
		"Debug the authentication module",
	}

	for i, task := range newTasks {
		selectedModel, _ := router.RouteTask(context.Background(), task)
		category := router.GetTaskCategory(task)

		fmt.Printf(
			"Task %d: '%s'\n  -> Category: %s\n  -> Routed to: %s (performance-based)\n\n",
			i+1, task, category, selectedModel,
		)
	}

	return nil
}

// Example3_ModelAffinityRouting demonstrates model affinity-based routing
func Example3_ModelAffinityRouting() error {
	// Create router with affinity strategy
	router := NewTaskRouter(StrategyAffinity)

	// Register specialized models
	models := map[string]string{
		"coding-specialist": "Specialized in writing code",
		"test-specialist":   "Specialized in writing tests",
		"doc-specialist":    "Specialized in documentation",
	}

	for modelID := range models {
		dummyInstance, _ := session.NewInstance(session.InstanceOptions{
			Title:   modelID,
			Path:    "/tmp/" + modelID,
			Program: "claude",
		})
		router.RegisterModel(modelID, dummyInstance)
	}

	fmt.Println("=== Model Affinity Routing Example ===")

	// Build affinity by successfully routing tasks to specialized models
	affinityTasks := []struct {
		prompt  string
		model   string
		success bool
	}{
		{"Implement feature A", "coding-specialist", true},
		{"Implement feature B", "coding-specialist", true},
		{"Implement function C", "coding-specialist", true},
		{"Write unit tests for X", "test-specialist", true},
		{"Write integration tests", "test-specialist", true},
		{"Write API documentation", "doc-specialist", true},
		{"Document function signatures", "doc-specialist", true},
	}

	// Build affinity relationships
	fmt.Println("Building affinity relationships...")
	for _, task := range affinityTasks {
		category := router.GetTaskCategory(task.prompt)
		router.RecordTaskResult(task.model, task.success, 100*time.Millisecond, category)
		fmt.Printf(
			"  Task: '%s' -> %s [%s] (Affinity ++)\n",
			task.prompt, task.model, category,
		)
	}

	fmt.Println("\nRouting new tasks based on affinity...")

	// Now route similar tasks - should use affinity
	newTasks := []struct {
		prompt        string
		expectedModel string
	}{
		{"Create a sorting algorithm", "coding-specialist"},
		{"Test the new API endpoint", "test-specialist"},
		{"Update the README file", "doc-specialist"},
	}

	for i, task := range newTasks {
		selectedModel, _ := router.RouteTask(context.Background(), task.prompt)
		category := router.GetTaskCategory(task.prompt)

		match := "✓"
		if selectedModel != task.expectedModel {
			match = "✗"
		}

		fmt.Printf(
			"Task %d: '%s'\n  -> Category: %s\n  -> Routed to: %s (expected: %s) %s\n\n",
			i+1, task.prompt, category, selectedModel, task.expectedModel, match,
		)
	}

	// Display affinity map
	fmt.Println("=== Affinity Map ===")
	for _, category := range []TaskCategory{TaskCoding, TaskTesting, TaskDocumentation} {
		affinity := router.affinityMap.GetAffinity(category)
		if len(affinity) > 0 {
			fmt.Printf("Category: %s\n", category)
			for modelID, score := range affinity {
				fmt.Printf("  %s: %d\n", modelID, score)
			}
		}
	}

	return nil
}

// Example4_CircuitBreakerPattern demonstrates circuit breaker functionality
func Example4_CircuitBreakerPattern() error {
	// Create router with least-loaded strategy
	router := NewTaskRouter(StrategyLeastLoaded)

	// Register models
	models := []string{"model-a", "model-b", "model-c"}
	for _, modelID := range models {
		dummyInstance, _ := session.NewInstance(session.InstanceOptions{
			Title:   modelID,
			Path:    "/tmp/" + modelID,
			Program: "claude",
		})
		router.RegisterModel(modelID, dummyInstance)
	}

	fmt.Println("=== Circuit Breaker Pattern Example ===")
	fmt.Println("Simulating failures for model-a...")

	// Simulate repeated failures for model-a
	for i := 0; i < 6; i++ {
		category := TaskCoding
		router.RecordTaskResult("model-a", false, 100*time.Millisecond, category)

		isOpen, _ := router.GetCircuitBreakerStatus("model-a")
		fmt.Printf("Failure %d: Circuit Breaker Open = %v\n", i+1, isOpen)
	}

	fmt.Println("\nRouting tasks after circuit breaker opened...")

	// Now route tasks - should avoid model-a
	for i := 0; i < 3; i++ {
		selectedModel, _ := router.RouteTask(context.Background(), "Implement new feature")

		if selectedModel == "model-a" {
			fmt.Printf("Task %d: Routed to %s (UNEXPECTED)\n", i+1, selectedModel)
		} else {
			fmt.Printf("Task %d: Routed to %s (avoided model-a with open circuit)\n", i+1, selectedModel)
		}
	}

	fmt.Println("\nForcing health recovery for model-a...")
	router.ForceHealthRecovery("model-a")

	// Try routing again
	selectedModel, _ := router.RouteTask(context.Background(), "Implement another feature")
	fmt.Printf("After recovery: Routed to %s\n", selectedModel)

	return nil
}

// Example5_HybridRoutingStrategy demonstrates hybrid routing
func Example5_HybridRoutingStrategy() error {
	// Create router with hybrid strategy
	router := NewTaskRouter(StrategyHybrid)

	// Register models
	models := []string{"efficient-model", "powerful-model", "balanced-model"}
	for _, modelID := range models {
		dummyInstance, _ := session.NewInstance(session.InstanceOptions{
			Title:   modelID,
			Path:    "/tmp/" + modelID,
			Program: "claude",
		})
		router.RegisterModel(modelID, dummyInstance)
	}

	fmt.Println("=== Hybrid Routing Strategy Example ===")

	// Build some affinity data
	initialTasks := []struct {
		prompt string
		model  string
	}{
		{"Write Python code", "efficient-model"},
		{"Write Python tests", "efficient-model"},
		{"Complex algorithm design", "powerful-model"},
		{"Architecture documentation", "balanced-model"},
	}

	fmt.Println("Building initial affinity data...")
	for _, task := range initialTasks {
		category := router.GetTaskCategory(task.prompt)
		router.RecordTaskResult(task.model, true, 100*time.Millisecond, category)
		fmt.Printf("  %s -> %s\n", task.prompt, task.model)
	}

	fmt.Println("\nRouting with hybrid strategy (uses affinity + performance)...")

	// Route similar tasks
	similarTasks := []string{
		"Write a Python function",
		"Test the Python module",
		"Design scalable architecture",
	}

	for i, task := range similarTasks {
		selectedModel, _ := router.RouteTask(context.Background(), task)
		category := router.GetTaskCategory(task)

		fmt.Printf(
			"Task %d: '%s'\n  -> Category: %s\n  -> Routed to: %s (hybrid strategy)\n\n",
			i+1, task, category, selectedModel,
		)
	}

	return nil
}

// Example6_DynamicStrategySwapping demonstrates changing strategies at runtime
func Example6_DynamicStrategySwapping() error {
	fmt.Println("=== Dynamic Strategy Swapping Example ===")

	// Start with round-robin
	router := NewTaskRouter(StrategyRoundRobin)

	// Register models
	models := []string{"model-1", "model-2", "model-3"}
	for _, modelID := range models {
		dummyInstance, _ := session.NewInstance(session.InstanceOptions{
			Title:   modelID,
			Path:    "/tmp/" + modelID,
			Program: "claude",
		})
		router.RegisterModel(modelID, dummyInstance)
	}

	// Sample task
	task := "Implement a new feature"

	// Route with round-robin
	fmt.Println("1. Round-Robin Strategy:")
	for i := 0; i < 3; i++ {
		model, _ := router.RouteTask(context.Background(), task)
		fmt.Printf("   Request %d -> %s\n", i+1, model)
	}

	// Switch to random
	router.SetRoutingStrategy(StrategyRandom)
	fmt.Println("\n2. Random Strategy:")
	for i := 0; i < 3; i++ {
		model, _ := router.RouteTask(context.Background(), task)
		fmt.Printf("   Request %d -> %s\n", i+1, model)
	}

	// Switch to least-loaded
	router.SetRoutingStrategy(StrategyLeastLoaded)
	fmt.Println("\n3. Least-Loaded Strategy:")
	for i := 0; i < 3; i++ {
		model, _ := router.RouteTask(context.Background(), task)
		category := router.GetTaskCategory(task)
		router.RecordTaskResult(model, true, 100*time.Millisecond, category)
		fmt.Printf("   Request %d -> %s\n", i+1, model)
	}

	return nil
}

// Example7_TaskCategoryDetection demonstrates task categorization
func Example7_TaskCategoryDetection() error {
	fmt.Println("=== Task Category Detection Example ===")

	router := NewTaskRouter(StrategyRoundRobin)

	testCases := []struct {
		prompt   string
		expected TaskCategory
	}{
		{"Implement a new sorting algorithm", TaskCoding},
		{"Write unit tests for the parser", TaskTesting},
		{"Refactor the database connection", TaskRefactoring},
		{"Update the API documentation", TaskDocumentation},
		{"Debug the authentication error", TaskDebugging},
		{"Review the proposed changes", TaskCodeReview},
		{"Apply some optimization to the cache", TaskRefactoring},
	}

	fmt.Println("Category Detection Results:")
	for _, testCase := range testCases {
		detected := router.GetTaskCategory(testCase.prompt)
		match := "✓"
		if detected != testCase.expected {
			match = "✗"
		}

		fmt.Printf(
			"Prompt: '%s'\n  -> Detected: %s (expected: %s) %s\n\n",
			testCase.prompt, detected, testCase.expected, match,
		)
	}

	return nil
}

// Example8_MetricsTracking demonstrates metrics collection and analysis
func Example8_MetricsTracking() error {
	fmt.Println("=== Metrics Tracking Example ===")

	router := NewTaskRouter(StrategyPerformance)

	// Register models
	models := []string{"model-fast", "model-medium", "model-slow"}
	for _, modelID := range models {
		dummyInstance, _ := session.NewInstance(session.InstanceOptions{
			Title:   modelID,
			Path:    "/tmp/" + modelID,
			Program: "claude",
		})
		router.RegisterModel(modelID, dummyInstance)
	}

	// Simulate task executions
	executions := []struct {
		model    string
		success  bool
		latency  time.Duration
		category TaskCategory
	}{
		{"model-fast", true, 50 * time.Millisecond, TaskCoding},
		{"model-fast", true, 60 * time.Millisecond, TaskCoding},
		{"model-fast", false, 45 * time.Millisecond, TaskTesting},
		{"model-medium", true, 150 * time.Millisecond, TaskTesting},
		{"model-medium", true, 160 * time.Millisecond, TaskRefactoring},
		{"model-medium", false, 140 * time.Millisecond, TaskCoding},
		{"model-slow", false, 500 * time.Millisecond, TaskDocumentation},
		{"model-slow", false, 510 * time.Millisecond, TaskDocumentation},
	}

	// Execute and record
	for _, exec := range executions {
		router.RecordTaskResult(exec.model, exec.success, exec.latency, exec.category)
	}

	// Display metrics
	fmt.Println("Model Performance Metrics:")
	fmt.Println(strings.Repeat("=", 70))

	metrics := router.GetAllMetrics()
	for _, modelID := range models {
		m := metrics[modelID]

		successRate := float64(0)
		if m.TotalRequests > 0 {
			successRate = float64(m.SuccessfulTasks) / float64(m.TotalRequests) * 100
		}

		fmt.Printf(
			"Model: %s\n"+
				"  Total Requests: %d\n"+
				"  Successful: %d\n"+
				"  Failed: %d\n"+
				"  Success Rate: %.1f%%\n"+
				"  Avg Latency: %v\n"+
				"  Circuit Breaker: %v\n\n",
			modelID,
			m.TotalRequests,
			m.SuccessfulTasks,
			m.FailedTasks,
			successRate,
			m.AverageLatency,
			m.CircuitBreakerOpen,
		)
	}

	return nil
}

// RunAllExamples runs all routing strategy examples
func RunAllExamples() error {
	examples := []struct {
		name string
		fn   func() error
	}{
		{"Basic Round-Robin Routing", Example1_BasicRoundRobinRouting},
		{"Performance-Based Routing", Example2_PerformanceBasedRouting},
		{"Model Affinity Routing", Example3_ModelAffinityRouting},
		{"Circuit Breaker Pattern", Example4_CircuitBreakerPattern},
		{"Hybrid Routing Strategy", Example5_HybridRoutingStrategy},
		{"Dynamic Strategy Swapping", Example6_DynamicStrategySwapping},
		{"Task Category Detection", Example7_TaskCategoryDetection},
		{"Metrics Tracking", Example8_MetricsTracking},
	}

	for _, example := range examples {
		fmt.Printf("\n%s\n%s\n", example.name, strings.Repeat("=", 70))
		if err := example.fn(); err != nil {
			log.ErrorLog.Printf("error in %s: %v", example.name, err)
			return err
		}
		fmt.Println()
	}

	return nil
}
