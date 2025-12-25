package ollama

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ExampleOrchestrator demonstrates how to use the ModelOrchestrator
func ExampleOrchestrator() {
	// Create a new orchestrator with health checks every 10 seconds and 4 workers
	orchestrator := NewModelOrchestrator(10*time.Second, 4)

	// Register multiple Ollama models
	err := orchestrator.RegisterModel("llama2", "http://localhost:11434", 30*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	err = orchestrator.RegisterModel("neural-chat", "http://localhost:11435", 30*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	err = orchestrator.RegisterModel("mistral", "http://localhost:11436", 30*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	// Start the orchestrator
	if err := orchestrator.Start(); err != nil {
		log.Fatal(err)
	}
	defer func() {
		// Graceful shutdown with 5-second timeout
		if err := orchestrator.Shutdown(5 * time.Second); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()

	// Example 1: Submit request to specific model
	fmt.Println("=== Example 1: Submit to Specific Model ===")
	resultCh, err := orchestrator.Submit("llama2", "What is Go?", 30*time.Second)
	if err != nil {
		log.Printf("Error submitting request: %v", err)
		return
	}

	select {
	case result := <-resultCh:
		fmt.Printf("Model: %s\nResponse: %s\nDuration: %v\n", result.Model, result.Response, result.Duration)
		if result.Error != nil {
			fmt.Printf("Error: %v\n", result.Error)
		}
	case <-time.After(35 * time.Second):
		fmt.Println("Request timeout")
	}

	// Example 2: Load-balanced submission
	fmt.Println("\n=== Example 2: Load-Balanced Submission ===")
	resultCh, selectedModel, err := orchestrator.SubmitBalanced("Explain concurrency", 30*time.Second)
	if err != nil {
		log.Printf("Error with load-balanced submission: %v", err)
		return
	}

	select {
	case result := <-resultCh:
		fmt.Printf("Selected Model: %s\nResponse: %s\nDuration: %v\n", selectedModel, result.Response, result.Duration)
	case <-time.After(35 * time.Second):
		fmt.Println("Request timeout")
	}

	// Example 3: Concurrent requests
	fmt.Println("\n=== Example 3: Concurrent Requests ===")
	var wg sync.WaitGroup
	prompts := []string{
		"What is machine learning?",
		"Explain neural networks",
		"What is Golang?",
		"How does distributed computing work?",
	}

	results := make(chan RequestResult, len(prompts))

	for _, prompt := range prompts {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			resultCh, modelName, err := orchestrator.SubmitBalanced(p, 30*time.Second)
			if err != nil {
				results <- RequestResult{Error: err}
				return
			}

			select {
			case result := <-resultCh:
				fmt.Printf("[%s] Prompt: %s -> Duration: %v\n", modelName, p, result.Duration)
				results <- result
			case <-time.After(35 * time.Second):
				results <- RequestResult{Error: fmt.Errorf("timeout for prompt: %s", p)}
			}
		}(prompt)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		if result.Error != nil {
			fmt.Printf("Error: %v\n", result.Error)
		}
	}

	// Example 4: Batch requests
	fmt.Println("\n=== Example 4: Batch Requests ===")
	batch := NewRequestBatch()

	batchPrompts := []string{
		"What is containerization?",
		"Explain Docker",
		"What is Kubernetes?",
	}

	for _, prompt := range batchPrompts {
		resultCh, _, err := orchestrator.SubmitBalanced(prompt, 30*time.Second)
		if err != nil {
			log.Printf("Error submitting batch request: %v", err)
			continue
		}

		req := &Request{
			Prompt:   prompt,
			ResultCh: resultCh,
		}
		batch.Add(req)
	}

	batchResults := batch.WaitAll(35 * time.Second)
	for i, result := range batchResults {
		fmt.Printf("Batch Result %d: Duration=%v, Error=%v\n", i, result.Duration, result.Error)
	}

	// Example 5: Check model status
	fmt.Println("\n=== Example 5: Model Status ===")
	status := orchestrator.GetModelStatus()
	for name, modelStatus := range status {
		fmt.Printf("Model: %s\n  Healthy: %v\n  Successes: %d\n  Failures: %d\n  Last Health Check: %v\n",
			name, modelStatus.IsHealthy, modelStatus.SuccessCount, modelStatus.FailureCount, modelStatus.LastHealthAt)
	}

	// Example 6: Get orchestrator metrics
	fmt.Println("\n=== Example 6: Orchestrator Metrics ===")
	metrics := orchestrator.GetOrchestrationMetrics()
	fmt.Printf("Total Requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Successful Requests: %d\n", metrics.SuccessfulRequests)
	fmt.Printf("Failed Requests: %d\n", metrics.FailedRequests)
	fmt.Printf("Average Latency: %v\n", metrics.AverageLatency)
	fmt.Printf("Healthy Models: %d/%d\n", metrics.HealthyModels, metrics.TotalModels)

	// Example 7: Circuit breaker pattern
	fmt.Println("\n=== Example 7: Circuit Breaker Pattern ===")
	cb := NewCircuitBreaker(3, 5*time.Second)

	// Simulate failures
	for i := 0; i < 5; i++ {
		if cb.AllowRequest() {
			fmt.Printf("Request %d allowed\n", i+1)
			if i < 3 {
				cb.RecordFailure()
				fmt.Printf("Request %d failed\n", i+1)
			} else {
				cb.RecordSuccess()
				fmt.Printf("Request %d succeeded\n", i+1)
			}
		} else {
			fmt.Printf("Request %d rejected (circuit open)\n", i+1)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Example 8: Rate limiter
	fmt.Println("\n=== Example 8: Rate Limiter ===")
	rateLimiter := NewRateLimiter(10, 2) // 10 tokens max, refill 2 tokens/sec

	for i := 0; i < 15; i++ {
		if rateLimiter.Allow(1) {
			fmt.Printf("Request %d allowed\n", i+1)
		} else {
			fmt.Printf("Request %d rate limited\n", i+1)
		}
	}

	// Example 9: Dynamic model registration
	fmt.Println("\n=== Example 9: Dynamic Model Management ===")
	newModel := "phi"
	if err := orchestrator.RegisterModel(newModel, "http://localhost:11437", 30*time.Second); err != nil {
		log.Printf("Error registering new model: %v", err)
	} else {
		fmt.Printf("Successfully registered model: %s\n", newModel)
	}

	status = orchestrator.GetModelStatus()
	fmt.Printf("Total models now: %d\n", len(status))

	// Unregister a model
	if err := orchestrator.UnregisterModel(newModel); err != nil {
		log.Printf("Error unregistering model: %v", err)
	} else {
		fmt.Printf("Successfully unregistered model: %s\n", newModel)
	}

	// Example 10: Context-aware shutdown
	fmt.Println("\n=== Example 10: Context-Aware Shutdown ===")
	orchestrator2 := NewModelOrchestrator(5*time.Second, 2)
	if err := orchestrator2.RegisterModel("test-model", "http://localhost:11438", 20*time.Second); err != nil {
		log.Fatal(err)
	}

	if err := orchestrator2.Start(); err != nil {
		log.Fatal(err)
	}

	// Submit some requests
	for i := 0; i < 3; i++ {
		orchestrator2.SubmitBalanced(fmt.Sprintf("Test prompt %d", i), 20*time.Second)
	}

	// Graceful shutdown with context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- orchestrator2.Shutdown(5 * time.Second)
	}()

	select {
	case err := <-shutdownDone:
		if err != nil {
			fmt.Printf("Shutdown error: %v\n", err)
		} else {
			fmt.Println("Orchestrator shutdown successfully")
		}
	case <-ctx.Done():
		fmt.Println("Context deadline exceeded during shutdown")
	}
}

// ExampleOrchestratorModelPool demonstrates the orchestrator model pool usage
func ExampleOrchestratorModelPool() {
	fmt.Println("\n=== Orchestrator Model Pool Example ===")

	pool := NewOrchestratorModelPool(5)

	// Get a model from the pool
	model := pool.Get()
	model.name = "test-model"
	model.baseURL = "http://localhost:11434"

	fmt.Printf("Model from pool: %s at %s\n", model.name, model.baseURL)

	// Use the model...

	// Put it back in the pool for reuse
	pool.Put(model)

	// Get another model (might be the same one)
	model2 := pool.Get()
	fmt.Printf("Model name after reuse: %s (should be empty)\n", model2.name)
}

// ExampleWorkerPool demonstrates the worker pool
func ExampleWorkerPool() {
	fmt.Println("\n=== Worker Pool Example ===")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wp := &WorkerPool{
		workers:   3,
		requestCh: make(chan *Request, 10),
		ctx:       ctx,
	}
	wp.ctx, wp.cancel = context.WithCancel(ctx)

	if err := wp.Start(); err != nil {
		log.Fatal(err)
	}
	defer wp.Stop()

	// Send some requests
	for i := 0; i < 5; i++ {
		req := &Request{
			ModelName: "test-model",
			Prompt:    fmt.Sprintf("Test prompt %d", i),
			Timeout:   5 * time.Second,
			ResultCh:  make(chan RequestResult, 1),
		}

		wp.requestCh <- req

		// Wait for result
		select {
		case result := <-req.ResultCh:
			fmt.Printf("Result %d: Duration=%v\n", i, result.Duration)
		case <-time.After(10 * time.Second):
			fmt.Printf("Result %d: Timeout\n", i)
		}
	}
}
