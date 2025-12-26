package ollama_test

import (
	"claude-squad/ollama"
	"context"
	"fmt"
	"time"
)

// ExampleNewModelOrchestrator demonstrates creating a new ModelOrchestrator
// for managing multiple Ollama model instances with health checking and load balancing.
func ExampleNewModelOrchestrator() {
	ctx := context.Background()

	// Create orchestrator with 30-second health checks and 4 workers
	mo := ollama.NewModelOrchestrator(ctx, 30*time.Second, 4)
	defer mo.Shutdown(5 * time.Second)

	fmt.Println("ModelOrchestrator created")
	// Output: ModelOrchestrator created
}

// ExampleModelOrchestrator_RegisterModel demonstrates registering a model instance
// with the orchestrator for load-balanced request routing.
func ExampleModelOrchestrator_RegisterModel() {
	ctx := context.Background()
	mo := ollama.NewModelOrchestrator(ctx, 30*time.Second, 4)
	defer mo.Shutdown(5 * time.Second)

	// Register a model with base URL and timeout
	err := mo.RegisterModel(ctx, "llama2", "http://localhost:11434", 10*time.Second)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Model registered successfully")
	// Output: Model registered successfully
}

// ExampleModelOrchestrator_Start demonstrates starting the orchestrator
// to begin processing requests and health checking.
func ExampleModelOrchestrator_Start() {
	ctx := context.Background()
	mo := ollama.NewModelOrchestrator(ctx, 30*time.Second, 4)
	defer mo.Shutdown(5 * time.Second)

	// Register at least one model before starting
	_ = mo.RegisterModel(ctx, "llama2", "http://localhost:11434", 10*time.Second)

	// Start the orchestrator
	err := mo.Start()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Orchestrator started")
	// Output: Orchestrator started
}

// ExampleModelOrchestrator_Submit demonstrates submitting a request to a specific model
// for asynchronous processing.
func ExampleModelOrchestrator_Submit() {
	ctx := context.Background()
	mo := ollama.NewModelOrchestrator(ctx, 30*time.Second, 4)
	defer mo.Shutdown(5 * time.Second)

	_ = mo.RegisterModel(ctx, "llama2", "http://localhost:11434", 10*time.Second)
	_ = mo.Start()

	// Submit a request to the model
	resultCh, err := mo.Submit("llama2", "Hello, world!", 10*time.Second)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Wait for the result
	select {
	case result := <-resultCh:
		if result.Error != nil {
			fmt.Printf("Request failed: %v\n", result.Error)
		} else {
			fmt.Printf("Request completed in %v\n", result.Duration)
		}
	case <-time.After(15 * time.Second):
		fmt.Println("Request timed out")
	}
}

// ExampleModelOrchestrator_SubmitBalanced demonstrates submitting a request
// to the least-loaded healthy model for automatic load balancing.
func ExampleModelOrchestrator_SubmitBalanced() {
	ctx := context.Background()
	mo := ollama.NewModelOrchestrator(ctx, 30*time.Second, 4)
	defer mo.Shutdown(5 * time.Second)

	// Register multiple models for load balancing
	_ = mo.RegisterModel(ctx, "llama2-1", "http://localhost:11434", 10*time.Second)
	_ = mo.RegisterModel(ctx, "llama2-2", "http://localhost:11435", 10*time.Second)
	_ = mo.Start()

	// Submit request with automatic model selection
	resultCh, selectedModel, err := mo.SubmitBalanced("Hello, world!", 10*time.Second)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Request routed to: %s\n", selectedModel)

	// Wait for the result
	select {
	case <-resultCh:
		fmt.Println("Request completed")
	case <-time.After(15 * time.Second):
		fmt.Println("Request timed out")
	}
}

// ExampleModelOrchestrator_GetModelStatus demonstrates retrieving health status
// for all registered models.
func ExampleModelOrchestrator_GetModelStatus() {
	ctx := context.Background()
	mo := ollama.NewModelOrchestrator(ctx, 30*time.Second, 4)
	defer mo.Shutdown(5 * time.Second)

	_ = mo.RegisterModel(ctx, "llama2", "http://localhost:11434", 10*time.Second)
	_ = mo.Start()

	// Get status for all models
	status := mo.GetModelStatus()

	for name, modelStatus := range status {
		fmt.Printf("Model: %s, Healthy: %v, Failures: %d\n",
			name, modelStatus.IsHealthy, modelStatus.FailureCount)
	}
}

// ExampleModelOrchestrator_GetOrchestrationMetrics demonstrates retrieving
// performance metrics from the orchestrator.
func ExampleModelOrchestrator_GetOrchestrationMetrics() {
	ctx := context.Background()
	mo := ollama.NewModelOrchestrator(ctx, 30*time.Second, 4)
	defer mo.Shutdown(5 * time.Second)

	_ = mo.RegisterModel(ctx, "llama2", "http://localhost:11434", 10*time.Second)
	_ = mo.Start()

	// Get orchestration metrics
	metrics := mo.GetOrchestrationMetrics()

	fmt.Printf("Total Requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Successful: %d\n", metrics.SuccessfulRequests)
	fmt.Printf("Failed: %d\n", metrics.FailedRequests)
	fmt.Printf("Healthy Models: %d/%d\n", metrics.HealthyModels, metrics.TotalModels)
}

// ExampleNewTaskDispatcher demonstrates creating a task dispatcher
// for concurrent task execution with a worker pool.
func ExampleNewTaskDispatcher() {
	ctx := context.Background()

	// Define an agent function that processes tasks
	agentFunc := func(ctx context.Context, task *ollama.Task) error {
		fmt.Printf("Processing task: %s\n", task.ID)
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	// Create dispatcher with 4 workers
	dispatcher, err := ollama.NewTaskDispatcher(ctx, agentFunc, 4)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer dispatcher.Shutdown(5 * time.Second)

	fmt.Println("TaskDispatcher created with 4 workers")
	// Output: TaskDispatcher created with 4 workers
}

// ExampleTaskDispatcher_SubmitTask demonstrates submitting a task
// for asynchronous execution.
func ExampleTaskDispatcher_SubmitTask() {
	ctx := context.Background()

	agentFunc := func(ctx context.Context, task *ollama.Task) error {
		// Simulate task processing
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	dispatcher, _ := ollama.NewTaskDispatcher(ctx, agentFunc, 2)
	defer dispatcher.Shutdown(5 * time.Second)

	_ = dispatcher.Start()

	// Create and submit a task
	task := &ollama.Task{
		ID:       "task-1",
		Priority: ollama.PriorityHigh,
		Payload:  "Process this data",
	}

	err := dispatcher.SubmitTask(task)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Task submitted successfully")
	// Wait for task to complete
	time.Sleep(200 * time.Millisecond)
	// Output: Task submitted successfully
}

// ExampleTaskDispatcher_GetMetrics demonstrates retrieving dispatcher metrics
// to monitor task execution performance.
func ExampleTaskDispatcher_GetMetrics() {
	ctx := context.Background()

	agentFunc := func(ctx context.Context, task *ollama.Task) error {
		return nil
	}

	dispatcher, _ := ollama.NewTaskDispatcher(ctx, agentFunc, 4)
	defer dispatcher.Shutdown(5 * time.Second)

	_ = dispatcher.Start()

	// Submit some tasks
	for i := 0; i < 5; i++ {
		task := &ollama.Task{
			ID:       fmt.Sprintf("task-%d", i),
			Priority: ollama.PriorityNormal,
		}
		_ = dispatcher.SubmitTask(task)
	}

	time.Sleep(500 * time.Millisecond)

	// Get metrics
	metrics := dispatcher.GetMetrics()
	fmt.Printf("Completed Tasks: %d\n", metrics.CompletedTasks)
	fmt.Printf("Worker Count: %d\n", metrics.WorkerCount)
}

// ExampleNewAgentPool demonstrates creating an agent pool
// for managing reusable session instances.
func ExampleNewAgentPool() {
	config := ollama.DefaultPoolConfig()
	config.MinPoolSize = 2
	config.MaxPoolSize = 5

	pool, err := ollama.NewAgentPool(config)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer pool.Close()

	fmt.Printf("AgentPool created with min=%d, max=%d\n",
		config.MinPoolSize, config.MaxPoolSize)
}

// ExampleAgentPool_Acquire demonstrates acquiring an agent from the pool
// for executing work.
func ExampleAgentPool_Acquire() {
	config := ollama.DefaultPoolConfig()
	config.MinPoolSize = 1
	config.MaxPoolSize = 3

	pool, _ := ollama.NewAgentPool(config)
	defer pool.Close()

	ctx := context.Background()

	// Acquire an agent from the pool
	agent, err := pool.Acquire(ctx)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Use the agent for work
	fmt.Printf("Acquired agent in state: %v\n", agent.GetState())

	// Release the agent back to the pool
	_ = pool.Release(agent)

	fmt.Println("Agent released back to pool")
}

// ExampleAgentPool_GetMetrics demonstrates retrieving pool metrics
// to monitor agent usage and performance.
func ExampleAgentPool_GetMetrics() {
	config := ollama.DefaultPoolConfig()
	pool, _ := ollama.NewAgentPool(config)
	defer pool.Close()

	// Get pool metrics
	metrics := pool.GetMetrics()

	fmt.Printf("Active Agents: %d\n", metrics.ActiveAgents)
	fmt.Printf("Idle Agents: %d\n", metrics.IdleAgents)
	fmt.Printf("Total Agents: %d\n", metrics.TotalAgents)
}

// ExampleNewCircuitBreaker demonstrates creating a circuit breaker
// to protect against cascading failures.
func ExampleNewCircuitBreaker() {
	// Create circuit breaker that opens after 3 failures
	// and resets after 10 seconds
	cb := ollama.NewCircuitBreaker(3, 10*time.Second)

	// Check if request is allowed
	if cb.AllowRequest() {
		fmt.Println("Request allowed")
		// Simulate successful request
		cb.RecordSuccess()
	}

	// Output: Request allowed
}

// ExampleCircuitBreaker_RecordFailure demonstrates recording failures
// and circuit breaker state transitions.
func ExampleCircuitBreaker_RecordFailure() {
	cb := ollama.NewCircuitBreaker(3, 10*time.Second)

	// Simulate multiple failures
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	// Check if circuit is now open
	if cb.IsOpen() {
		fmt.Println("Circuit breaker opened after 3 failures")
	}

	// Output: Circuit breaker opened after 3 failures
}

// ExampleNewRateLimiter demonstrates creating a token bucket rate limiter
// to control request rates.
func ExampleNewRateLimiter() {
	// Create rate limiter with 10 tokens, refilling at 2 tokens/second
	rl := ollama.NewRateLimiter(10, 2.0)

	// Check if request is allowed (costs 1 token)
	if rl.Allow(1.0) {
		fmt.Println("Request allowed")
	}

	// Output: Request allowed
}

// ExampleRateLimiter_Allow demonstrates rate limiting with token consumption.
func ExampleRateLimiter_Allow() {
	rl := ollama.NewRateLimiter(5, 1.0)

	// Consume all tokens
	allowed := 0
	for i := 0; i < 10; i++ {
		if rl.Allow(1.0) {
			allowed++
		}
	}

	fmt.Printf("Allowed %d out of 10 requests\n", allowed)
	// Output: Allowed 5 out of 10 requests
}

// ExampleNewRequestBatch demonstrates batch processing of multiple requests
// for improved throughput.
func ExampleNewRequestBatch() {
	ctx := context.Background()
	mo := ollama.NewModelOrchestrator(ctx, 30*time.Second, 4)
	defer mo.Shutdown(5 * time.Second)

	_ = mo.RegisterModel(ctx, "llama2", "http://localhost:11434", 10*time.Second)
	_ = mo.Start()

	// Create a batch
	batch := ollama.NewRequestBatch()

	// Add multiple requests to the batch
	for i := 0; i < 3; i++ {
		resultCh, err := mo.Submit("llama2", fmt.Sprintf("Request %d", i), 10*time.Second)
		if err == nil {
			batch.Add(&ollama.Request{
				ModelName: "llama2",
				Prompt:    fmt.Sprintf("Request %d", i),
				ResultCh:  resultCh,
			})
		}
	}

	// Wait for all results with timeout
	results := batch.WaitAll(15 * time.Second)

	fmt.Printf("Batch processed %d requests\n", len(results))
}
