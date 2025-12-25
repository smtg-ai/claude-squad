package ollama

import (
	"claude-squad/log"
	"claude-squad/session"
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// TaskCategory represents the type of task to be routed
type TaskCategory string

const (
	TaskCoding        TaskCategory = "coding"
	TaskRefactoring   TaskCategory = "refactoring"
	TaskTesting       TaskCategory = "testing"
	TaskDocumentation TaskCategory = "documentation"
	TaskDebugging     TaskCategory = "debugging"
	TaskCodeReview    TaskCategory = "code_review"
	TaskUnknown       TaskCategory = "unknown"
)

// RoutingStrategy defines how tasks are routed to models
type RoutingStrategy string

const (
	StrategyRoundRobin  RoutingStrategy = "round_robin"
	StrategyLeastLoaded RoutingStrategy = "least_loaded"
	StrategyRandom      RoutingStrategy = "random"
	StrategyPerformance RoutingStrategy = "performance"
	StrategyAffinity    RoutingStrategy = "affinity"
	StrategyHybrid      RoutingStrategy = "hybrid"
)

// RouterMetrics tracks performance metrics for a model in the router
type RouterMetrics struct {
	ModelID              string
	TotalRequests        int64
	SuccessfulTasks      int64
	FailedTasks          int64
	AverageLatency       time.Duration
	LastUsed             time.Time
	CircuitBreakerOpen   bool
	FailureCount         int32
	SuccessCount         int32
	FailureWindow        time.Time
	CircuitBreakerOpens  int64 // Count of times circuit opened
	CircuitBreakerCloses int64 // Count of times circuit closed
	CircuitBreakerHalfs  int64 // Count of times circuit entered half-open state
}

// TaskAffinityMap tracks which models are best suited for task types
type TaskAffinityMap struct {
	mu       sync.RWMutex
	affinity map[TaskCategory]map[string]int // taskType -> modelID -> affinity_score
}

// CircuitBreakerConfig defines circuit breaker behavior
type CircuitBreakerConfig struct {
	FailureThreshold int           // Number of failures to open circuit
	SuccessThreshold int           // Number of successes to close circuit
	Timeout          time.Duration // Time before attempting to recover
	HalfOpenRequests int           // Requests to allow in half-open state
}

// RouterModelPool manages a pool of model instances for routing
type RouterModelPool struct {
	mu        sync.RWMutex
	models    map[string]*RouterMetrics
	instances map[string]*session.Instance
}

// TaskRouter handles intelligent routing of tasks to models with load balancing
type TaskRouter struct {
	mu                     sync.RWMutex
	modelPool              *RouterModelPool
	strategy               RoutingStrategy
	metrics                map[string]*RouterMetrics
	affinityMap            *TaskAffinityMap
	circuitBreakerConfig   CircuitBreakerConfig
	roundRobinIndex        int32
	taskCategoryDetector   TaskCategoryDetector
	performanceWeights     map[TaskCategory]map[string]float64
	performanceWeightsMu   sync.RWMutex
	maxFailureRetries      int
	lastStrategyUpdate     time.Time
	strategyUpdateInterval time.Duration
}

// getMetricsMap returns the metrics map, lazily initializing if nil
// This prevents panics if TaskRouter is created without using NewTaskRouter
func (tr *TaskRouter) getMetricsMap() map[string]*RouterMetrics {
	if tr.metrics == nil {
		tr.metrics = make(map[string]*RouterMetrics)
	}
	return tr.metrics
}

// getPerformanceWeights returns the performance weights map, lazily initializing if nil
func (tr *TaskRouter) getPerformanceWeights() map[TaskCategory]map[string]float64 {
	if tr.performanceWeights == nil {
		tr.performanceWeights = make(map[TaskCategory]map[string]float64)
	}
	return tr.performanceWeights
}

// TaskCategoryDetector defines an interface for detecting task categories
type TaskCategoryDetector interface {
	Detect(prompt string) TaskCategory
}

// DefaultTaskCategoryDetector implements basic task categorization
type DefaultTaskCategoryDetector struct{}

// Detect implements task category detection based on keywords
func (d *DefaultTaskCategoryDetector) Detect(prompt string) TaskCategory {
	keywords := map[TaskCategory][]string{
		TaskCoding: {
			"implement", "write", "create", "function", "method",
			"class", "interface", "struct", "algorithm", "code",
		},
		TaskRefactoring: {
			"refactor", "cleanup", "optimize", "simplify", "restructure",
			"rename", "extract", "consolidate", "improve", "performance",
		},
		TaskTesting: {
			"test", "unit test", "integration test", "test case", "mock",
			"assert", "expect", "verify", "coverage", "pytest", "jest",
		},
		TaskDocumentation: {
			"doc", "comment", "readme", "javadoc", "docstring", "explain",
			"description", "guide", "tutorial", "example", "changelog",
		},
		TaskDebugging: {
			"debug", "fix", "bug", "error", "crash", "panic", "stack trace",
			"issue", "problem", "wrong", "not working", "exception",
		},
		TaskCodeReview: {
			"review", "approve", "feedback", "suggest", "improve", "quality",
			"standard", "best practice", "lint", "style", "convention",
		},
	}

	for category, words := range keywords {
		for _, word := range words {
			if containsWord(prompt, word) {
				return category
			}
		}
	}

	return TaskUnknown
}

// NewTaskRouter creates a new TaskRouter with specified strategy
func NewTaskRouter(strategy RoutingStrategy) *TaskRouter {
	tr := &TaskRouter{
		strategy:               strategy,
		modelPool:              NewRouterModelPool(),
		metrics:                make(map[string]*RouterMetrics),
		affinityMap:            NewTaskAffinityMap(),
		roundRobinIndex:        0,
		taskCategoryDetector:   &DefaultTaskCategoryDetector{},
		performanceWeights:     make(map[TaskCategory]map[string]float64),
		maxFailureRetries:      3,
		lastStrategyUpdate:     time.Now(),
		strategyUpdateInterval: 5 * time.Second,
		circuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Timeout:          30 * time.Second,
			HalfOpenRequests: 2,
		},
	}

	return tr
}

// RegisterModel adds a model instance to the router
func (tr *TaskRouter) RegisterModel(modelID string, instance *session.Instance) error {
	if modelID == "" {
		return fmt.Errorf("model ID cannot be empty")
	}
	if instance == nil {
		return fmt.Errorf("instance cannot be nil")
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()

	metrics := tr.getMetricsMap()
	if _, exists := metrics[modelID]; exists {
		return fmt.Errorf("model %s already registered", modelID)
	}

	metrics[modelID] = &RouterMetrics{
		ModelID:              modelID,
		TotalRequests:        0,
		SuccessfulTasks:      0,
		FailedTasks:          0,
		AverageLatency:       0,
		LastUsed:             time.Now(),
		CircuitBreakerOpen:   false,
		FailureCount:         0,
		SuccessCount:         0,
		FailureWindow:        time.Now(),
		CircuitBreakerOpens:  0,
		CircuitBreakerCloses: 0,
		CircuitBreakerHalfs:  0,
	}

	tr.modelPool.AddInstance(modelID, instance)
	log.InfoLog.Printf("registered model %s", modelID)

	return nil
}

// UnregisterModel removes a model from the router
func (tr *TaskRouter) UnregisterModel(modelID string) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	metrics := tr.getMetricsMap()
	if _, exists := metrics[modelID]; !exists {
		return fmt.Errorf("model %s not registered", modelID)
	}

	delete(metrics, modelID)
	tr.modelPool.RemoveInstance(modelID)
	tr.affinityMap.ClearModel(modelID)
	log.InfoLog.Printf("unregistered model %s", modelID)

	return nil
}

// RouteTask determines which model should handle the given task
func (tr *TaskRouter) RouteTask(ctx context.Context, taskPrompt string, previousContext ...string) (string, error) {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	tr.mu.RLock()
	defer tr.mu.RUnlock()

	metrics := tr.getMetricsMap()
	if len(metrics) == 0 {
		return "", fmt.Errorf("no models registered")
	}

	category := tr.taskCategoryDetector.Detect(taskPrompt)

	// Check health and remove unhealthy models
	availableModels := tr.getAvailableModels()
	if len(availableModels) == 0 {
		return "", fmt.Errorf("no healthy models available")
	}

	var selectedModel string
	var err error

	switch tr.strategy {
	case StrategyRoundRobin:
		selectedModel, err = tr.routeRoundRobin(availableModels)
	case StrategyLeastLoaded:
		selectedModel, err = tr.routeLeastLoaded(availableModels)
	case StrategyRandom:
		selectedModel, err = tr.routeRandom(availableModels)
	case StrategyPerformance:
		selectedModel, err = tr.routePerformance(availableModels, category)
	case StrategyAffinity:
		selectedModel, err = tr.routeAffinity(availableModels, category, previousContext...)
	case StrategyHybrid:
		selectedModel, err = tr.routeHybrid(availableModels, category)
	default:
		selectedModel, err = tr.routeRoundRobin(availableModels)
	}

	if err != nil {
		return "", err
	}

	log.InfoLog.Printf(
		"routed task (category: %s) to model %s using strategy %s",
		category, selectedModel, tr.strategy,
	)

	return selectedModel, nil
}

// RecordTaskResult records the result of a task execution
func (tr *TaskRouter) RecordTaskResult(modelID string, success bool, latency time.Duration, category TaskCategory) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	metricsMap := tr.getMetricsMap()
	metrics, exists := metricsMap[modelID]
	if !exists {
		return fmt.Errorf("model %s not registered", modelID)
	}

	metrics.TotalRequests++
	metrics.LastUsed = time.Now()

	if success {
		metrics.SuccessfulTasks++

		// Check if circuit was open and should transition to closed
		wasOpen := metrics.CircuitBreakerOpen

		atomic.AddInt32(&metrics.SuccessCount, 1)
		successCount := atomic.LoadInt32(&metrics.SuccessCount)

		// Close circuit after reaching success threshold
		if wasOpen && int(successCount) >= tr.circuitBreakerConfig.SuccessThreshold {
			metrics.CircuitBreakerOpen = false
			atomic.StoreInt32(&metrics.FailureCount, 0)
			atomic.StoreInt32(&metrics.SuccessCount, 0)
			atomic.AddInt64(&metrics.CircuitBreakerCloses, 1)
			log.InfoLog.Printf("circuit breaker closed for model %s after %d successes",
				modelID, successCount)
		}

		metrics.FailureWindow = time.Now()

		// Update affinity for successful tasks
		tr.affinityMap.IncrementAffinity(category, modelID, 1)
	} else {
		metrics.FailedTasks++
		atomic.AddInt32(&metrics.FailureCount, 1)
		atomic.StoreInt32(&metrics.SuccessCount, 0) // Reset success count on failure

		// Check circuit breaker
		failureCount := atomic.LoadInt32(&metrics.FailureCount)
		wasOpen := metrics.CircuitBreakerOpen

		if int(failureCount) >= tr.circuitBreakerConfig.FailureThreshold && !wasOpen {
			metrics.CircuitBreakerOpen = true
			atomic.AddInt64(&metrics.CircuitBreakerOpens, 1)
			log.WarningLog.Printf("circuit breaker opened for model %s after %d failures",
				modelID, failureCount)
		}

		// Decrease affinity for failed tasks
		tr.affinityMap.DecrementAffinity(category, modelID, 2)
	}

	// Update average latency
	if metrics.AverageLatency == 0 {
		metrics.AverageLatency = latency
	} else {
		metrics.AverageLatency = (metrics.AverageLatency + latency) / 2
	}

	return nil
}

// HealthCheck checks the health of registered models
func (tr *TaskRouter) HealthCheck(ctx context.Context) map[string]bool {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return make(map[string]bool)
	default:
	}

	tr.mu.RLock()
	defer tr.mu.RUnlock()

	health := make(map[string]bool)
	now := time.Now()

	metricsMap := tr.getMetricsMap()
	for modelID, metrics := range metricsMap {
		// Check context cancellation during iteration
		select {
		case <-ctx.Done():
			return health
		default:
		}

		isHealthy := true

		// Circuit breaker check
		if metrics.CircuitBreakerOpen {
			if now.Sub(metrics.FailureWindow) > tr.circuitBreakerConfig.Timeout {
				// Try to recover (half-open state)
				atomic.AddInt64(&metrics.CircuitBreakerHalfs, 1)
				log.InfoLog.Printf("circuit breaker entering half-open state for model %s", modelID)
				// Keep circuit open but allow limited test requests
				// The RecordTaskResult will close it after SuccessThreshold successes
				isHealthy = true
			} else {
				isHealthy = false
			}
		}

		health[modelID] = isHealthy
	}

	return health
}

// GetModelMetrics returns metrics for a specific model
// Note: Returns pointer (not value) for individual metric lookups to allow nil returns.
// This differs from orchestrator.GetOrchestrationMetrics() which returns by value
// for aggregate metrics. The pointer return allows error handling for missing models.
func (tr *TaskRouter) GetModelMetrics(modelID string) (*RouterMetrics, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	metricsMap := tr.getMetricsMap()
	metrics, exists := metricsMap[modelID]
	if !exists {
		return nil, fmt.Errorf("model %s not registered", modelID)
	}

	// Return a copy to prevent external modification
	metricsCopy := *metrics
	return &metricsCopy, nil
}

// GetAllMetrics returns metrics for all registered models
func (tr *TaskRouter) GetAllMetrics() map[string]*RouterMetrics {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	metricsMap := tr.getMetricsMap()
	metricsCopy := make(map[string]*RouterMetrics)
	for modelID, metrics := range metricsMap {
		m := *metrics
		metricsCopy[modelID] = &m
	}

	return metricsCopy
}

// SetRoutingStrategy changes the routing strategy
func (tr *TaskRouter) SetRoutingStrategy(strategy RoutingStrategy) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	validStrategies := map[RoutingStrategy]bool{
		StrategyRoundRobin:  true,
		StrategyLeastLoaded: true,
		StrategyRandom:      true,
		StrategyPerformance: true,
		StrategyAffinity:    true,
		StrategyHybrid:      true,
	}

	if !validStrategies[strategy] {
		return fmt.Errorf("invalid routing strategy: %s", strategy)
	}

	oldStrategy := tr.strategy
	tr.strategy = strategy
	log.InfoLog.Printf("changed routing strategy from %s to %s", oldStrategy, strategy)

	return nil
}

// --- Routing Strategy Implementations ---

// routeRoundRobin implements round-robin scheduling
func (tr *TaskRouter) routeRoundRobin(availableModels []string) (string, error) {
	if len(availableModels) == 0 {
		return "", fmt.Errorf("no available models")
	}

	index := atomic.AddInt32(&tr.roundRobinIndex, 1) - 1
	modelID := availableModels[index%int32(len(availableModels))]

	return modelID, nil
}

// routeLeastLoaded routes to the model with fewest active requests
func (tr *TaskRouter) routeLeastLoaded(availableModels []string) (string, error) {
	if len(availableModels) == 0 {
		return "", fmt.Errorf("no available models")
	}

	var leastLoadedModel string
	minLoad := int64(math.MaxInt64)

	metricsMap := tr.getMetricsMap()
	for _, modelID := range availableModels {
		metrics, exists := metricsMap[modelID]
		if !exists || metrics == nil {
			continue // Skip models without metrics
		}
		// Calculate current load based on total requests vs successful tasks
		load := metrics.TotalRequests - metrics.SuccessfulTasks

		if load < minLoad {
			minLoad = load
			leastLoadedModel = modelID
		}
	}

	if leastLoadedModel == "" {
		return "", fmt.Errorf("no models with metrics available")
	}

	return leastLoadedModel, nil
}

// routeRandom routes to a random available model
func (tr *TaskRouter) routeRandom(availableModels []string) (string, error) {
	if len(availableModels) == 0 {
		return "", fmt.Errorf("no available models")
	}

	return availableModels[rand.Intn(len(availableModels))], nil
}

// routePerformance routes based on model performance for the task category
func (tr *TaskRouter) routePerformance(availableModels []string, category TaskCategory) (string, error) {
	if len(availableModels) == 0 {
		return "", fmt.Errorf("no available models")
	}

	// Recalculate performance weights if cache is stale
	now := time.Now()
	tr.performanceWeightsMu.RLock()
	needsUpdate := now.Sub(tr.lastStrategyUpdate) > tr.strategyUpdateInterval
	tr.performanceWeightsMu.RUnlock()

	if needsUpdate {
		tr.performanceWeightsMu.Lock()
		// Double-check after acquiring write lock
		if now.Sub(tr.lastStrategyUpdate) > tr.strategyUpdateInterval {
			tr.calculatePerformanceWeights()
			tr.lastStrategyUpdate = now
		}
		tr.performanceWeightsMu.Unlock()
	}

	var bestModel string
	bestScore := float64(-1)

	metricsMap := tr.getMetricsMap()
	for _, modelID := range availableModels {
		metrics, exists := metricsMap[modelID]
		if !exists || metrics == nil {
			continue // Skip models without metrics
		}

		// Calculate success rate
		successRate := float64(0)
		if metrics.TotalRequests > 0 {
			successRate = float64(metrics.SuccessfulTasks) / float64(metrics.TotalRequests)
		}

		// Lower latency is better (invert it)
		latencyScore := float64(1) / (1 + float64(metrics.AverageLatency.Milliseconds()))

		// Combined score
		score := (successRate * 0.7) + (latencyScore * 0.3)

		if score > bestScore {
			bestScore = score
			bestModel = modelID
		}
	}

	if bestModel == "" {
		// Fallback to round-robin if no models have scores
		return tr.routeRoundRobin(availableModels)
	}

	return bestModel, nil
}

// routeAffinity routes based on model affinity for task categories
func (tr *TaskRouter) routeAffinity(availableModels []string, category TaskCategory, previousContext ...string) (string, error) {
	if len(availableModels) == 0 {
		return "", fmt.Errorf("no available models")
	}

	// Check if we should stick with previous model (from context)
	if len(previousContext) > 0 && previousContext[0] != "" {
		for _, modelID := range availableModels {
			if modelID == previousContext[0] {
				return modelID, nil
			}
		}
	}

	// Find model with highest affinity for this category
	var bestModel string
	bestAffinity := -1

	affinity := tr.affinityMap.GetAffinity(category)
	for _, modelID := range availableModels {
		score := affinity[modelID]
		if score > bestAffinity {
			bestAffinity = score
			bestModel = modelID
		}
	}

	if bestModel == "" {
		// Fallback to least loaded if no affinity data
		return tr.routeLeastLoaded(availableModels)
	}

	return bestModel, nil
}

// routeHybrid uses a combination of strategies
func (tr *TaskRouter) routeHybrid(availableModels []string, category TaskCategory) (string, error) {
	if len(availableModels) == 0 {
		return "", fmt.Errorf("no available models")
	}

	// Try affinity first if we have data
	affinity := tr.affinityMap.GetAffinity(category)
	hasAffinityData := false
	for _, score := range affinity {
		if score > 0 {
			hasAffinityData = true
			break
		}
	}

	if hasAffinityData {
		return tr.routeAffinity(availableModels, category)
	}

	// Fall back to performance-based routing
	return tr.routePerformance(availableModels, category)
}

// --- Helper Methods ---

// getAvailableModels returns a list of healthy models
func (tr *TaskRouter) getAvailableModels() []string {
	var available []string
	now := time.Now()

	metricsMap := tr.getMetricsMap()
	for modelID, metrics := range metricsMap {
		isHealthy := true

		// Circuit breaker check
		if metrics.CircuitBreakerOpen {
			if now.Sub(metrics.FailureWindow) > tr.circuitBreakerConfig.Timeout {
				// Try to recover (half-open state)
				// The half-open state tracking is done in HealthCheck
				// Here we just allow it to be considered available
				isHealthy = true
			} else {
				isHealthy = false
			}
		}

		if isHealthy {
			available = append(available, modelID)
		}
	}

	return available
}

// calculatePerformanceWeights pre-calculates performance weights for all categories
// Note: Caller must hold performanceWeightsMu write lock
func (tr *TaskRouter) calculatePerformanceWeights() {
	perfWeights := tr.getPerformanceWeights()
	// Clear existing weights
	for k := range perfWeights {
		delete(perfWeights, k)
	}

	categories := []TaskCategory{
		TaskCoding, TaskRefactoring, TaskTesting, TaskDocumentation, TaskDebugging, TaskCodeReview,
	}

	metricsMap := tr.getMetricsMap()
	for _, category := range categories {
		weights := make(map[string]float64)

		for modelID := range metricsMap {
			metrics, exists := metricsMap[modelID]
			if !exists || metrics == nil {
				continue // Skip models without metrics
			}

			successRate := float64(0)
			if metrics.TotalRequests > 0 {
				successRate = float64(metrics.SuccessfulTasks) / float64(metrics.TotalRequests)
			}

			latencyScore := float64(1) / (1 + float64(metrics.AverageLatency.Milliseconds()))
			weights[modelID] = (successRate * 0.7) + (latencyScore * 0.3)
		}

		perfWeights[category] = weights
	}
}

// --- Task Affinity Map ---

// NewTaskAffinityMap creates a new task affinity map
func NewTaskAffinityMap() *TaskAffinityMap {
	return &TaskAffinityMap{
		affinity: make(map[TaskCategory]map[string]int),
	}
}

// IncrementAffinity increases affinity score for a model-category pair
func (tam *TaskAffinityMap) IncrementAffinity(category TaskCategory, modelID string, amount int) {
	if modelID == "" || amount < 0 {
		return
	}

	tam.mu.Lock()
	defer tam.mu.Unlock()

	if tam.affinity[category] == nil {
		tam.affinity[category] = make(map[string]int)
	}

	tam.affinity[category][modelID] += amount
}

// DecrementAffinity decreases affinity score for a model-category pair
func (tam *TaskAffinityMap) DecrementAffinity(category TaskCategory, modelID string, amount int) {
	if modelID == "" || amount < 0 {
		return
	}

	tam.mu.Lock()
	defer tam.mu.Unlock()

	if tam.affinity[category] == nil {
		tam.affinity[category] = make(map[string]int)
	}

	tam.affinity[category][modelID] -= amount
	if tam.affinity[category][modelID] < 0 {
		tam.affinity[category][modelID] = 0
	}
}

// GetAffinity returns affinity scores for all models for a task category
func (tam *TaskAffinityMap) GetAffinity(category TaskCategory) map[string]int {
	tam.mu.RLock()
	defer tam.mu.RUnlock()

	affinity := make(map[string]int)
	if scores, exists := tam.affinity[category]; exists {
		for modelID, score := range scores {
			affinity[modelID] = score
		}
	}

	return affinity
}

// ClearModel removes a model from all affinity maps
func (tam *TaskAffinityMap) ClearModel(modelID string) {
	tam.mu.Lock()
	defer tam.mu.Unlock()

	for category := range tam.affinity {
		delete(tam.affinity[category], modelID)
	}
}

// --- Router Model Pool ---

// NewRouterModelPool creates a new router model pool
func NewRouterModelPool() *RouterModelPool {
	return &RouterModelPool{
		models:    make(map[string]*RouterMetrics),
		instances: make(map[string]*session.Instance),
	}
}

// AddInstance adds a model instance to the pool
func (rmp *RouterModelPool) AddInstance(modelID string, instance *session.Instance) {
	rmp.mu.Lock()
	defer rmp.mu.Unlock()

	rmp.instances[modelID] = instance
}

// RemoveInstance removes a model instance from the pool
func (rmp *RouterModelPool) RemoveInstance(modelID string) {
	rmp.mu.Lock()
	defer rmp.mu.Unlock()

	delete(rmp.instances, modelID)
}

// GetInstance retrieves a model instance
func (rmp *RouterModelPool) GetInstance(modelID string) (*session.Instance, error) {
	rmp.mu.RLock()
	defer rmp.mu.RUnlock()

	instance, exists := rmp.instances[modelID]
	if !exists {
		return nil, fmt.Errorf("model instance %s not found", modelID)
	}

	return instance, nil
}

// GetAllInstances returns all model instances
func (rmp *RouterModelPool) GetAllInstances() map[string]*session.Instance {
	rmp.mu.RLock()
	defer rmp.mu.RUnlock()

	instancesCopy := make(map[string]*session.Instance)
	for modelID, instance := range rmp.instances {
		instancesCopy[modelID] = instance
	}

	return instancesCopy
}

// --- Utility Functions ---

// containsWord checks if a string contains a word (case-insensitive)
func containsWord(text, word string) bool {
	lowerText := toLower(text)
	lowerWord := toLower(word)
	return containsStr(lowerText, lowerWord)
}

// toLower converts a string to lowercase (simple implementation)
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// contains checks if a string contains a substring
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// SetTaskCategoryDetector sets a custom task category detector
func (tr *TaskRouter) SetTaskCategoryDetector(detector TaskCategoryDetector) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.taskCategoryDetector = detector
}

// GetTaskCategory returns the detected category for a prompt
func (tr *TaskRouter) GetTaskCategory(prompt string) TaskCategory {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return tr.taskCategoryDetector.Detect(prompt)
}

// ResetMetrics resets all metrics (useful for testing or periodic resets)
func (tr *TaskRouter) ResetMetrics() {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	metricsMap := tr.getMetricsMap()
	for _, metrics := range metricsMap {
		metrics.TotalRequests = 0
		metrics.SuccessfulTasks = 0
		metrics.FailedTasks = 0
		metrics.AverageLatency = 0
		atomic.StoreInt32(&metrics.FailureCount, 0)
		atomic.StoreInt32(&metrics.SuccessCount, 0)
		metrics.CircuitBreakerOpen = false
	}

	tr.affinityMap = NewTaskAffinityMap()
}

// GetCircuitBreakerStatus returns the circuit breaker status for a model
func (tr *TaskRouter) GetCircuitBreakerStatus(modelID string) (bool, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	metricsMap := tr.getMetricsMap()
	metrics, exists := metricsMap[modelID]
	if !exists {
		return false, fmt.Errorf("model %s not registered", modelID)
	}

	return metrics.CircuitBreakerOpen, nil
}

// ForceHealthRecovery attempts to recover a circuit-breaker opened model
func (tr *TaskRouter) ForceHealthRecovery(modelID string) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	metricsMap := tr.getMetricsMap()
	metrics, exists := metricsMap[modelID]
	if !exists {
		return fmt.Errorf("model %s not registered", modelID)
	}

	metrics.CircuitBreakerOpen = false
	atomic.StoreInt32(&metrics.FailureCount, 0)
	metrics.FailureWindow = time.Now()

	log.InfoLog.Printf("forced health recovery for model %s", modelID)

	return nil
}

// SetCircuitBreakerConfig updates the circuit breaker configuration
func (tr *TaskRouter) SetCircuitBreakerConfig(config CircuitBreakerConfig) error {
	if config.FailureThreshold <= 0 {
		return fmt.Errorf("failure threshold must be greater than 0")
	}
	if config.SuccessThreshold <= 0 {
		return fmt.Errorf("success threshold must be greater than 0")
	}
	if config.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}
	if config.HalfOpenRequests <= 0 {
		return fmt.Errorf("half-open requests must be greater than 0")
	}

	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.circuitBreakerConfig = config
	log.InfoLog.Printf("updated circuit breaker config: failure=%d, success=%d, timeout=%v, half-open=%d",
		config.FailureThreshold, config.SuccessThreshold, config.Timeout, config.HalfOpenRequests)

	return nil
}

// GetCircuitBreakerConfig returns the current circuit breaker configuration
func (tr *TaskRouter) GetCircuitBreakerConfig() CircuitBreakerConfig {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return tr.circuitBreakerConfig
}
