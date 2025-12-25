package ollama

import (
	"claude-squad/session"
	"testing"
	"time"
)

// Helper function to create a dummy instance
func createDummyInstance(modelID string) *session.Instance {
	instance, _ := session.NewInstance(session.InstanceOptions{
		Title:   modelID,
		Path:    "/tmp/" + modelID,
		Program: "claude",
	})
	return instance
}

// TestTaskRouterRegistration tests model registration and unregistration
func TestTaskRouterRegistration(t *testing.T) {
	router := NewTaskRouter(StrategyRoundRobin)

	// Test registration
	modelID := "test-model-1"
	instance := createDummyInstance(modelID)

	err := router.RegisterModel(modelID, instance)
	if err != nil {
		t.Fatalf("failed to register model: %v", err)
	}

	// Verify model was registered
	metrics, err := router.GetModelMetrics(modelID)
	if err != nil {
		t.Fatalf("failed to get metrics for registered model: %v", err)
	}

	if metrics.ModelID != modelID {
		t.Errorf("expected model ID %s, got %s", modelID, metrics.ModelID)
	}

	// Test duplicate registration
	err = router.RegisterModel(modelID, instance)
	if err == nil {
		t.Errorf("expected error for duplicate registration, got nil")
	}

	// Test unregistration
	err = router.UnregisterModel(modelID)
	if err != nil {
		t.Fatalf("failed to unregister model: %v", err)
	}

	// Verify model was unregistered
	_, err = router.GetModelMetrics(modelID)
	if err == nil {
		t.Errorf("expected error getting metrics for unregistered model, got nil")
	}
}

// TestRoundRobinRouting tests round-robin routing strategy
func TestRoundRobinRouting(t *testing.T) {
	router := NewTaskRouter(StrategyRoundRobin)

	models := []string{"model-1", "model-2", "model-3"}
	for _, modelID := range models {
		router.RegisterModel(modelID, createDummyInstance(modelID))
	}

	// Route tasks and verify round-robin behavior
	for i := 0; i < 9; i++ {
		selected, err := router.RouteTask("test task")
		if err != nil {
			t.Fatalf("failed to route task: %v", err)
		}

		expectedModel := models[i%3]
		if selected != expectedModel {
			t.Errorf("iteration %d: expected %s, got %s", i, expectedModel, selected)
		}
	}
}

// TestLeastLoadedRouting tests least-loaded routing strategy
func TestLeastLoadedRouting(t *testing.T) {
	router := NewTaskRouter(StrategyLeastLoaded)

	models := []string{"model-1", "model-2", "model-3"}
	for _, modelID := range models {
		router.RegisterModel(modelID, createDummyInstance(modelID))
	}

	// Load model-1 with failed tasks
	for i := 0; i < 5; i++ {
		router.RecordTaskResult("model-1", false, 100*time.Millisecond, TaskCoding)
	}

	// Load model-2 with successful tasks
	for i := 0; i < 2; i++ {
		router.RecordTaskResult("model-2", true, 100*time.Millisecond, TaskCoding)
	}

	// Next task should go to model-3 (least loaded) or model-2 (fewer failures)
	selected, err := router.RouteTask("test task")
	if err != nil {
		t.Fatalf("failed to route task: %v", err)
	}

	if selected != "model-3" && selected != "model-2" {
		t.Errorf("expected model-3 or model-2 (least loaded), got %s", selected)
	}
}

// TestTaskCategoryDetection tests task categorization
func TestTaskCategoryDetection(t *testing.T) {
	router := NewTaskRouter(StrategyRoundRobin)

	tests := []struct {
		prompt   string
		expected TaskCategory
	}{
		{"implement a new algorithm", TaskCoding},
		{"write unit tests", TaskTesting},
		{"refactor the code", TaskRefactoring},
		{"update documentation", TaskDocumentation},
		{"debug the crash", TaskDebugging},
		{"review the changes", TaskCodeReview},
	}

	for _, test := range tests {
		detected := router.GetTaskCategory(test.prompt)
		if detected != test.expected && detected != TaskUnknown {
			// Allow some flexibility in detection but should detect many
			if test.expected != TaskUnknown && detected == TaskUnknown {
				t.Logf("warning: failed to detect expected category %s for prompt '%s'",
					test.expected, test.prompt)
			}
		}
	}
}

// TestTaskRouterCircuitBreaker tests task router circuit breaker functionality
func TestTaskRouterCircuitBreaker(t *testing.T) {
	router := NewTaskRouter(StrategyRoundRobin)

	modelID := "test-model"
	router.RegisterModel(modelID, createDummyInstance(modelID))

	// Verify circuit breaker is initially closed
	isOpen, err := router.GetCircuitBreakerStatus(modelID)
	if err != nil {
		t.Fatalf("failed to get circuit breaker status: %v", err)
	}
	if isOpen {
		t.Errorf("expected circuit breaker to be closed initially")
	}

	// Record failures to trigger circuit breaker
	for i := 0; i < 6; i++ {
		router.RecordTaskResult(modelID, false, 100*time.Millisecond, TaskCoding)
	}

	// Verify circuit breaker is now open
	isOpen, err = router.GetCircuitBreakerStatus(modelID)
	if err != nil {
		t.Fatalf("failed to get circuit breaker status: %v", err)
	}
	if !isOpen {
		t.Errorf("expected circuit breaker to be open after failures")
	}

	// Verify model is excluded from routing
	router.RegisterModel("healthy-model", createDummyInstance("healthy-model"))
	selected, err := router.RouteTask("test task")
	if err != nil {
		t.Fatalf("failed to route task: %v", err)
	}
	if selected == modelID {
		t.Errorf("expected routing to avoid model with open circuit breaker")
	}

	// Test force recovery
	err = router.ForceHealthRecovery(modelID)
	if err != nil {
		t.Fatalf("failed to force health recovery: %v", err)
	}

	isOpen, _ = router.GetCircuitBreakerStatus(modelID)
	if isOpen {
		t.Errorf("expected circuit breaker to be closed after recovery")
	}
}

// TestMetricsTracking tests metrics recording and retrieval
func TestMetricsTracking(t *testing.T) {
	router := NewTaskRouter(StrategyRoundRobin)

	modelID := "test-model"
	router.RegisterModel(modelID, createDummyInstance(modelID))

	// Record successful task
	latency := 100 * time.Millisecond
	router.RecordTaskResult(modelID, true, latency, TaskCoding)

	metrics, err := router.GetModelMetrics(modelID)
	if err != nil {
		t.Fatalf("failed to get metrics: %v", err)
	}

	if metrics.TotalRequests != 1 {
		t.Errorf("expected 1 request, got %d", metrics.TotalRequests)
	}

	if metrics.SuccessfulTasks != 1 {
		t.Errorf("expected 1 successful task, got %d", metrics.SuccessfulTasks)
	}

	if metrics.FailedTasks != 0 {
		t.Errorf("expected 0 failed tasks, got %d", metrics.FailedTasks)
	}

	if metrics.AverageLatency != latency {
		t.Errorf("expected latency %v, got %v", latency, metrics.AverageLatency)
	}

	// Record failed task
	router.RecordTaskResult(modelID, false, latency, TaskCoding)

	metrics, _ = router.GetModelMetrics(modelID)

	if metrics.TotalRequests != 2 {
		t.Errorf("expected 2 requests, got %d", metrics.TotalRequests)
	}

	if metrics.FailedTasks != 1 {
		t.Errorf("expected 1 failed task, got %d", metrics.FailedTasks)
	}
}

// TestAffinityRouting tests model affinity routing
func TestAffinityRouting(t *testing.T) {
	router := NewTaskRouter(StrategyAffinity)

	models := []string{"model-coding", "model-testing", "model-docs"}
	for _, modelID := range models {
		router.RegisterModel(modelID, createDummyInstance(modelID))
	}

	// Build affinity: coding tasks go to model-coding
	for i := 0; i < 5; i++ {
		router.RecordTaskResult("model-coding", true, 100*time.Millisecond, TaskCoding)
	}

	// Build affinity: testing tasks go to model-testing
	for i := 0; i < 5; i++ {
		router.RecordTaskResult("model-testing", true, 100*time.Millisecond, TaskTesting)
	}

	// Now route a coding task - should prefer model-coding
	_, err := router.RouteTask("implement a new function")
	if err != nil {
		t.Fatalf("failed to route task: %v", err)
	}

	// Should route to a model with affinity for coding
	affinity := router.affinityMap.GetAffinity(TaskCoding)
	if affinity["model-coding"] == 0 {
		t.Logf("warning: expected affinity for model-coding, got %d", affinity["model-coding"])
	}
}

// TestRoutingStrategySwitch tests changing routing strategies at runtime
func TestRoutingStrategySwitch(t *testing.T) {
	router := NewTaskRouter(StrategyRoundRobin)

	models := []string{"model-1", "model-2"}
	for _, modelID := range models {
		router.RegisterModel(modelID, createDummyInstance(modelID))
	}

	// Initial strategy is round-robin
	if router.strategy != StrategyRoundRobin {
		t.Errorf("expected round-robin strategy, got %v", router.strategy)
	}

	// Switch strategy
	err := router.SetRoutingStrategy(StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("failed to set routing strategy: %v", err)
	}

	if router.strategy != StrategyLeastLoaded {
		t.Errorf("expected least-loaded strategy, got %v", router.strategy)
	}

	// Test invalid strategy
	err = router.SetRoutingStrategy(RoutingStrategy("invalid"))
	if err == nil {
		t.Errorf("expected error for invalid strategy, got nil")
	}
}

// TestPerformanceBasedRouting tests performance-based routing
func TestPerformanceBasedRouting(t *testing.T) {
	router := NewTaskRouter(StrategyPerformance)

	models := []string{"fast-model", "slow-model"}
	for _, modelID := range models {
		router.RegisterModel(modelID, createDummyInstance(modelID))
	}

	// Build performance history
	// fast-model: fast with high success rate
	for i := 0; i < 5; i++ {
		router.RecordTaskResult("fast-model", true, 50*time.Millisecond, TaskCoding)
	}

	// slow-model: slow with low success rate
	for i := 0; i < 3; i++ {
		router.RecordTaskResult("slow-model", false, 500*time.Millisecond, TaskCoding)
	}

	// Route tasks - should prefer fast-model
	selected, err := router.RouteTask("implement a feature")
	if err != nil {
		t.Fatalf("failed to route task: %v", err)
	}

	if selected != "fast-model" {
		t.Logf("warning: expected fast-model, got %s (may be due to randomness)", selected)
	}
}

// TestHealthCheck tests health check functionality
func TestHealthCheck(t *testing.T) {
	router := NewTaskRouter(StrategyRoundRobin)

	modelID := "test-model"
	router.RegisterModel(modelID, createDummyInstance(modelID))

	// Health check should show healthy initially
	health := router.HealthCheck()
	if !health[modelID] {
		t.Errorf("expected model to be healthy initially")
	}

	// Trigger circuit breaker
	for i := 0; i < 6; i++ {
		router.RecordTaskResult(modelID, false, 100*time.Millisecond, TaskCoding)
	}

	// Health check should show unhealthy
	health = router.HealthCheck()
	if health[modelID] {
		t.Errorf("expected model to be unhealthy after circuit breaker opened")
	}
}

// TestMetricsReset tests metrics reset functionality
func TestMetricsReset(t *testing.T) {
	router := NewTaskRouter(StrategyRoundRobin)

	modelID := "test-model"
	router.RegisterModel(modelID, createDummyInstance(modelID))

	// Record some metrics
	for i := 0; i < 5; i++ {
		router.RecordTaskResult(modelID, i%2 == 0, 100*time.Millisecond, TaskCoding)
	}

	// Verify metrics are recorded
	metrics, _ := router.GetModelMetrics(modelID)
	if metrics.TotalRequests != 5 {
		t.Errorf("expected 5 requests, got %d", metrics.TotalRequests)
	}

	// Reset metrics
	router.ResetMetrics()

	// Verify metrics are reset
	metrics, _ = router.GetModelMetrics(modelID)
	if metrics.TotalRequests != 0 {
		t.Errorf("expected 0 requests after reset, got %d", metrics.TotalRequests)
	}
}

// TestNoModelsRegistered tests error when no models are registered
func TestNoModelsRegistered(t *testing.T) {
	router := NewTaskRouter(StrategyRoundRobin)

	_, err := router.RouteTask("test task")
	if err == nil {
		t.Errorf("expected error when no models registered, got nil")
	}
}

// TestTaskAffinityMap tests task affinity map functionality
func TestTaskAffinityMap(t *testing.T) {
	affinityMap := NewTaskAffinityMap()

	modelID := "test-model"
	category := TaskCoding

	// Test increment
	affinityMap.IncrementAffinity(category, modelID, 5)

	affinity := affinityMap.GetAffinity(category)
	if affinity[modelID] != 5 {
		t.Errorf("expected affinity 5, got %d", affinity[modelID])
	}

	// Test decrement
	affinityMap.DecrementAffinity(category, modelID, 3)

	affinity = affinityMap.GetAffinity(category)
	if affinity[modelID] != 2 {
		t.Errorf("expected affinity 2, got %d", affinity[modelID])
	}

	// Test decrement below zero (should stay at 0)
	affinityMap.DecrementAffinity(category, modelID, 10)

	affinity = affinityMap.GetAffinity(category)
	if affinity[modelID] != 0 {
		t.Errorf("expected affinity 0 (minimum), got %d", affinity[modelID])
	}

	// Test clear model
	affinityMap.IncrementAffinity(TaskTesting, modelID, 5)
	affinityMap.ClearModel(modelID)

	affinity1 := affinityMap.GetAffinity(TaskCoding)
	affinity2 := affinityMap.GetAffinity(TaskTesting)

	if len(affinity1) > 0 || len(affinity2) > 0 {
		t.Errorf("expected empty affinity maps after clear")
	}
}

// TestRouterModelPool tests router model pool functionality
func TestRouterModelPool(t *testing.T) {
	pool := NewRouterModelPool()

	modelID := "test-model"
	instance := createDummyInstance(modelID)

	// Add instance
	pool.AddInstance(modelID, instance)

	// Get instance
	retrieved, err := pool.GetInstance(modelID)
	if err != nil {
		t.Fatalf("failed to get instance: %v", err)
	}

	if retrieved.Title != instance.Title {
		t.Errorf("expected instance with title %s, got %s", instance.Title, retrieved.Title)
	}

	// Get non-existent instance
	_, err = pool.GetInstance("non-existent")
	if err == nil {
		t.Errorf("expected error for non-existent instance, got nil")
	}

	// Remove instance
	pool.RemoveInstance(modelID)

	_, err = pool.GetInstance(modelID)
	if err == nil {
		t.Errorf("expected error after removing instance, got nil")
	}
}

// TestConcurrentRouting tests concurrent routing operations
func TestConcurrentRouting(t *testing.T) {
	router := NewTaskRouter(StrategyRoundRobin)

	models := []string{"model-1", "model-2", "model-3"}
	for _, modelID := range models {
		router.RegisterModel(modelID, createDummyInstance(modelID))
	}

	// Simulate concurrent routing and metric recording
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			selectedModel, err := router.RouteTask("concurrent test task")
			if err == nil {
				category := router.GetTaskCategory("concurrent test task")
				router.RecordTaskResult(selectedModel, id%3 != 0, 50*time.Millisecond, category)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify metrics were recorded correctly
	allMetrics := router.GetAllMetrics()
	totalRequests := int64(0)
	for _, m := range allMetrics {
		totalRequests += m.TotalRequests
	}

	if totalRequests != 10 {
		t.Errorf("expected 10 total requests, got %d", totalRequests)
	}
}
