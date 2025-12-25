package ollama

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestModelOrchestrator_RegisterModel(t *testing.T) {
	mo := NewModelOrchestrator(5*time.Second, 2)

	err := mo.RegisterModel("llama2", "http://localhost:11434", 10*time.Second)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Try to register same model again
	err = mo.RegisterModel("llama2", "http://localhost:11434", 10*time.Second)
	if err == nil {
		t.Fatalf("Expected error for duplicate model registration")
	}

	// Verify model is registered
	if mo.countTotalModels() != 1 {
		t.Fatalf("Expected 1 model, got %d", mo.countTotalModels())
	}
}

func TestModelOrchestrator_UnregisterModel(t *testing.T) {
	mo := NewModelOrchestrator(5*time.Second, 2)

	err := mo.RegisterModel("llama2", "http://localhost:11434", 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	err = mo.UnregisterModel("llama2")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Try to unregister non-existent model
	err = mo.UnregisterModel("nonexistent")
	if err == nil {
		t.Fatalf("Expected error for non-existent model")
	}

	if mo.countTotalModels() != 0 {
		t.Fatalf("Expected 0 models, got %d", mo.countTotalModels())
	}
}

func TestModelOrchestrator_Start(t *testing.T) {
	mo := NewModelOrchestrator(5*time.Second, 2)

	// Should fail with no models registered
	err := mo.Start()
	if err == nil {
		t.Fatalf("Expected error when starting with no models")
	}

	err = mo.RegisterModel("llama2", "http://localhost:11434", 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	err = mo.Start()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	defer mo.Shutdown(2 * time.Second)
}

func TestModelOrchestrator_Submit(t *testing.T) {
	mo := NewModelOrchestrator(5*time.Second, 2)
	err := mo.RegisterModel("llama2", "http://localhost:11434", 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	err = mo.Start()
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}
	defer mo.Shutdown(2 * time.Second)

	resultCh, err := mo.Submit("llama2", "test prompt", 10*time.Second)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should get a result channel
	if resultCh == nil {
		t.Fatalf("Expected result channel, got nil")
	}

	// Try to submit to non-existent model
	_, err = mo.Submit("nonexistent", "test", 10*time.Second)
	if err == nil {
		t.Fatalf("Expected error for non-existent model")
	}
}

func TestModelOrchestrator_SubmitBalanced(t *testing.T) {
	mo := NewModelOrchestrator(5*time.Second, 2)
	err := mo.RegisterModel("llama2", "http://localhost:11434", 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}
	err = mo.RegisterModel("mistral", "http://localhost:11435", 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	err = mo.Start()
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}
	defer mo.Shutdown(2 * time.Second)

	resultCh, modelName, err := mo.SubmitBalanced("test prompt", 10*time.Second)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resultCh == nil {
		t.Fatalf("Expected result channel, got nil")
	}

	if modelName == "" {
		t.Fatalf("Expected model name, got empty string")
	}
}

func TestModelOrchestrator_GetModelStatus(t *testing.T) {
	mo := NewModelOrchestrator(5*time.Second, 2)
	err := mo.RegisterModel("llama2", "http://localhost:11434", 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	status := mo.GetModelStatus()
	if len(status) != 1 {
		t.Fatalf("Expected 1 model status, got %d", len(status))
	}

	modelStatus, ok := status["llama2"]
	if !ok {
		t.Fatalf("Expected 'llama2' in status")
	}

	if !modelStatus.IsHealthy {
		t.Fatalf("Expected model to be healthy")
	}

	if modelStatus.URL != "http://localhost:11434" {
		t.Fatalf("Expected correct URL, got %s", modelStatus.URL)
	}
}

func TestModelOrchestrator_GetOrchestrationMetrics(t *testing.T) {
	mo := NewModelOrchestrator(5*time.Second, 2)
	err := mo.RegisterModel("llama2", "http://localhost:11434", 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	err = mo.Start()
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}
	defer mo.Shutdown(2 * time.Second)

	metrics := mo.GetOrchestrationMetrics()
	if metrics.TotalModels != 1 {
		t.Fatalf("Expected 1 model, got %d", metrics.TotalModels)
	}

	if metrics.HealthyModels != 1 {
		t.Fatalf("Expected 1 healthy model, got %d", metrics.HealthyModels)
	}
}

func TestModelOrchestrator_Shutdown(t *testing.T) {
	mo := NewModelOrchestrator(5*time.Second, 2)
	err := mo.RegisterModel("llama2", "http://localhost:11434", 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	err = mo.Start()
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}

	err = mo.Shutdown(2 * time.Second)
	if err != nil {
		t.Fatalf("Expected no error during shutdown, got %v", err)
	}

	// Verify shutdown is complete by checking if submit fails
	_, err = mo.Submit("llama2", "test", 10*time.Second)
	if err == nil {
		t.Fatalf("Expected error after shutdown")
	}
}

func TestRequestBatch(t *testing.T) {
	mo := NewModelOrchestrator(5*time.Second, 2)
	err := mo.RegisterModel("llama2", "http://localhost:11434", 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	err = mo.Start()
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}
	defer mo.Shutdown(2 * time.Second)

	batch := NewRequestBatch()

	for i := 0; i < 3; i++ {
		resultCh, _, err := mo.SubmitBalanced("test prompt", 10*time.Second)
		if err != nil {
			t.Fatalf("Failed to submit request: %v", err)
		}

		req := &Request{
			ResultCh: resultCh,
		}
		batch.Add(req)
	}

	results := batch.WaitAll(5 * time.Second)
	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}
}

func TestOrchestratorModelPool(t *testing.T) {
	pool := NewOrchestratorModelPool(5)

	model1 := pool.Get()
	model1.name = "test"

	pool.Put(model1)

	model2 := pool.Get()
	if model2.name != "" {
		t.Fatalf("Expected empty name after reuse, got %s", model2.name)
	}
}

func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Second)

	if !cb.IsClosed() {
		t.Fatalf("Expected circuit to be closed initially")
	}

	// Record failures
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if !cb.IsOpen() {
		t.Fatalf("Expected circuit to be open after failures")
	}

	// Wait for reset timeout
	time.Sleep(1100 * time.Millisecond)

	// After timeout, should allow request
	if !cb.AllowRequest() {
		t.Fatalf("Expected circuit to allow request after timeout")
	}

	// Record success to close circuit
	cb.RecordSuccess()

	if !cb.IsClosed() {
		t.Fatalf("Expected circuit to be closed after success")
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10, 2)

	// Should allow 10 initial tokens
	for i := 0; i < 10; i++ {
		if !rl.Allow(1) {
			t.Fatalf("Expected to allow request %d", i)
		}
	}

	// 11th request should fail
	if rl.Allow(1) {
		t.Fatalf("Expected to reject 11th request")
	}

	// Wait for tokens to refill
	time.Sleep(1100 * time.Millisecond)

	// Should now allow more requests
	if !rl.Allow(1) {
		t.Fatalf("Expected to allow request after refill")
	}
}

func TestWorkerPool(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wp := &WorkerPool{
		workers:   2,
		requestCh: make(chan *Request, 10),
		ctx:       ctx,
	}
	wp.ctx, wp.cancel = context.WithCancel(ctx)

	err := wp.Start()
	if err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	defer wp.Stop()

	// Submit a request
	req := &Request{
		ModelName: "test",
		Prompt:    "test",
		Timeout:   5 * time.Second,
		ResultCh:  make(chan RequestResult, 1),
	}

	wp.requestCh <- req

	// Wait for result
	select {
	case result := <-req.ResultCh:
		if result.Duration <= 0 {
			t.Fatalf("Expected non-zero duration")
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("Timeout waiting for result")
	}
}

func TestConcurrentSubmissions(t *testing.T) {
	mo := NewModelOrchestrator(5*time.Second, 4)
	err := mo.RegisterModel("llama2", "http://localhost:11434", 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	err = mo.Start()
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}
	defer mo.Shutdown(2 * time.Second)

	var wg sync.WaitGroup
	numRequests := 20

	successCount := atomic.Int32{}
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resultCh, _, err := mo.SubmitBalanced("test prompt", 10*time.Second)
			if err == nil && resultCh != nil {
				select {
				case <-resultCh:
					successCount.Add(1)
				case <-time.After(5 * time.Second):
				}
			}
		}()
	}

	wg.Wait()

	if successCount.Load() != int32(numRequests) {
		t.Fatalf("Expected %d successful requests, got %d", numRequests, successCount.Load())
	}
}

func BenchmarkModelOrchestrator_Submit(b *testing.B) {
	mo := NewModelOrchestrator(5*time.Second, 4)
	mo.RegisterModel("llama2", "http://localhost:11434", 10*time.Second)
	mo.Start()
	defer mo.Shutdown(2 * time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resultCh, err := mo.Submit("llama2", "test prompt", 10*time.Second)
		if err == nil && resultCh != nil {
			select {
			case <-resultCh:
			case <-time.After(1 * time.Second):
			}
		}
	}
}

func BenchmarkModelOrchestrator_SubmitBalanced(b *testing.B) {
	mo := NewModelOrchestrator(5*time.Second, 4)
	mo.RegisterModel("llama2", "http://localhost:11434", 10*time.Second)
	mo.RegisterModel("mistral", "http://localhost:11435", 10*time.Second)
	mo.Start()
	defer mo.Shutdown(2 * time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resultCh, _, err := mo.SubmitBalanced("test prompt", 10*time.Second)
		if err == nil && resultCh != nil {
			select {
			case <-resultCh:
			case <-time.After(1 * time.Second):
			}
		}
	}
}
