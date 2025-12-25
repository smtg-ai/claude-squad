package ollama

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ModelInstance represents a single Ollama model instance
type ModelInstance struct {
	// Configuration
	name    string
	baseURL string
	timeout time.Duration

	// Health tracking
	isHealthy      atomic.Bool
	lastHealthTime time.Time
	failureCount   int32
	successCount   int32

	// Concurrency control
	mu sync.Mutex
}

// Request represents a concurrent request to be processed
type Request struct {
	ModelName string
	Prompt    string
	Timeout   time.Duration
	ResultCh  chan RequestResult
}

// RequestResult contains the result of a model request
type RequestResult struct {
	Response string
	Error    error
	Duration time.Duration
	Model    string
}

// WorkerPool manages a pool of concurrent workers
type WorkerPool struct {
	workers   int
	requestCh chan *Request
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// OrchestratorMetrics tracks orchestrator performance metrics
type OrchestratorMetrics struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	AverageLatency     time.Duration
	HealthyModels      int
	TotalModels        int
}

// ModelOrchestrator manages multiple Ollama model instances with load balancing
// and health checking
type ModelOrchestrator struct {
	// Model instances
	models map[string]*ModelInstance
	mu     sync.RWMutex

	// Health checking
	healthCheckInterval time.Duration
	healthCheckTicker   *time.Ticker
	healthCheckDone     chan struct{}

	// Request handling
	workerPool *WorkerPool
	requestCh  chan *Request

	// HTTP client for health checks
	httpClient *http.Client

	// Graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Pool for reusing request objects
	requestPool *sync.Pool

	// Metrics
	totalRequests   int64
	successfulReqs  int64
	failedReqs      int64
	averageLatency  int64
	lastLoadBalance time.Time
}

// NewModelOrchestrator creates a new orchestrator for managing multiple Ollama models
func NewModelOrchestrator(healthCheckInterval time.Duration, numWorkers int) *ModelOrchestrator {
	ctx, cancel := context.WithCancel(context.Background())

	mo := &ModelOrchestrator{
		models:              make(map[string]*ModelInstance),
		healthCheckInterval: healthCheckInterval,
		healthCheckDone:     make(chan struct{}),
		requestCh:           make(chan *Request, numWorkers*2),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
		},
		ctx:             ctx,
		cancel:          cancel,
		lastLoadBalance: time.Now(),
	}

	// Initialize request pool for reuse
	mo.requestPool = &sync.Pool{
		New: func() interface{} {
			return &Request{}
		},
	}

	// Initialize worker pool
	mo.workerPool = &WorkerPool{
		workers:   numWorkers,
		requestCh: mo.requestCh,
		ctx:       ctx,
	}
	mo.workerPool.ctx, mo.workerPool.cancel = context.WithCancel(ctx)

	return mo
}

// RegisterModel adds a new model instance to the orchestrator
func (mo *ModelOrchestrator) RegisterModel(name, baseURL string, timeout time.Duration) error {
	mo.mu.Lock()
	defer mo.mu.Unlock()

	if _, exists := mo.models[name]; exists {
		return fmt.Errorf("model '%s' already registered", name)
	}

	model := &ModelInstance{
		name:           name,
		baseURL:        baseURL,
		timeout:        timeout,
		lastHealthTime: time.Now(),
	}
	model.isHealthy.Store(true)

	mo.models[name] = model
	return nil
}

// UnregisterModel removes a model instance from the orchestrator
func (mo *ModelOrchestrator) UnregisterModel(name string) error {
	mo.mu.Lock()
	defer mo.mu.Unlock()

	if _, exists := mo.models[name]; !exists {
		return fmt.Errorf("model '%s' not found", name)
	}

	delete(mo.models, name)
	return nil
}

// Start initializes worker pools and health checking
func (mo *ModelOrchestrator) Start() error {
	mo.mu.RLock()
	if len(mo.models) == 0 {
		mo.mu.RUnlock()
		return errors.New("no models registered")
	}
	mo.mu.RUnlock()

	// Start worker pool
	if err := mo.workerPool.Start(); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	// Start health check goroutine
	mo.wg.Add(1)
	go mo.healthCheckLoop()

	return nil
}

// Submit sends a request to the orchestrator for processing
func (mo *ModelOrchestrator) Submit(modelName, prompt string, timeout time.Duration) (chan RequestResult, error) {
	mo.mu.RLock()
	model, exists := mo.models[modelName]
	mo.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("model '%s' not registered", modelName)
	}

	if !model.isHealthy.Load() {
		return nil, fmt.Errorf("model '%s' is unhealthy", modelName)
	}

	// Get request from pool

	poolObj := mo.requestPool.Get()

	req, ok := poolObj.(*Request)

	if !ok {

		return nil, fmt.Errorf("failed to get request from pool: invalid type %T", poolObj)

	}
	req.ModelName = modelName
	req.Prompt = prompt
	req.Timeout = timeout
	req.ResultCh = make(chan RequestResult, 1)

	// Update metrics
	atomic.AddInt64(&mo.totalRequests, 1)

	select {
	case mo.requestCh <- req:
		return req.ResultCh, nil
	case <-mo.ctx.Done():
		return nil, errors.New("orchestrator is shutting down")
	default:
		return nil, errors.New("request queue is full")
	}
}

// SubmitBalanced submits a request to the least-loaded healthy model
func (mo *ModelOrchestrator) SubmitBalanced(prompt string, timeout time.Duration) (chan RequestResult, string, error) {
	mo.mu.RLock()
	if len(mo.models) == 0 {
		mo.mu.RUnlock()
		return nil, "", errors.New("no models available")
	}

	// Find healthiest model with lowest failure count
	var selectedModel *ModelInstance
	minFailures := int32(^uint32(0) >> 1)

	for _, model := range mo.models {
		if model.isHealthy.Load() {
			failures := atomic.LoadInt32(&model.failureCount)
			if failures < minFailures {
				minFailures = failures
				selectedModel = model
			}
		}
	}

	mo.mu.RUnlock()

	if selectedModel == nil {
		return nil, "", errors.New("no healthy models available")
	}

	resultCh, err := mo.Submit(selectedModel.name, prompt, timeout)
	if err != nil {
		return nil, "", err
	}

	return resultCh, selectedModel.name, nil
}

// ModelHealthStatus represents the health and statistics of a model instance
type ModelHealthStatus struct {
	Name         string
	IsHealthy    bool
	FailureCount int32
	SuccessCount int32
	LastHealthAt time.Time
	URL          string
}

// GetModelStatus returns the current health status of all registered models
func (mo *ModelOrchestrator) GetModelStatus() map[string]ModelHealthStatus {
	mo.mu.RLock()
	defer mo.mu.RUnlock()

	status := make(map[string]ModelHealthStatus)
	for name, model := range mo.models {
		model.mu.Lock()
		status[name] = ModelHealthStatus{
			Name:         model.name,
			IsHealthy:    model.isHealthy.Load(),
			FailureCount: atomic.LoadInt32(&model.failureCount),
			SuccessCount: atomic.LoadInt32(&model.successCount),
			LastHealthAt: model.lastHealthTime,
			URL:          model.baseURL,
		}
		model.mu.Unlock()
	}

	return status
}

// GetOrchestrationMetrics returns current orchestrator metrics
func (mo *ModelOrchestrator) GetOrchestrationMetrics() OrchestratorMetrics {
	total := atomic.LoadInt64(&mo.totalRequests)
	successful := atomic.LoadInt64(&mo.successfulReqs)
	failed := atomic.LoadInt64(&mo.failedReqs)
	avgLatency := atomic.LoadInt64(&mo.averageLatency)

	return OrchestratorMetrics{
		TotalRequests:      total,
		SuccessfulRequests: successful,
		FailedRequests:     failed,
		AverageLatency:     time.Duration(avgLatency),
		HealthyModels:      mo.countHealthyModels(),
		TotalModels:        mo.countTotalModels(),
	}
}

// healthCheckLoop periodically checks the health of all models
func (mo *ModelOrchestrator) healthCheckLoop() {
	defer mo.wg.Done()

	mo.healthCheckTicker = time.NewTicker(mo.healthCheckInterval)
	defer mo.healthCheckTicker.Stop()

	for {
		select {
		case <-mo.healthCheckTicker.C:
			mo.performHealthCheck()

		case <-mo.ctx.Done():
			close(mo.healthCheckDone)
			return
		}
	}
}

// performHealthCheck checks the health of all models
func (mo *ModelOrchestrator) performHealthCheck() {
	mo.mu.RLock()
	models := make([]*ModelInstance, 0, len(mo.models))
	for _, model := range mo.models {
		models = append(models, model)
	}
	mo.mu.RUnlock()

	var wg sync.WaitGroup
	for _, model := range models {
		wg.Add(1)
		go func(m *ModelInstance) {
			defer wg.Done()
			mo.checkModelHealth(m)
		}(model)
	}
	wg.Wait()
}

// checkModelHealth checks if a single model is healthy
func (mo *ModelOrchestrator) checkModelHealth(model *ModelInstance) {
	model.mu.Lock()
	defer model.mu.Unlock()

	// Create a health check context with timeout
	ctx, cancel := context.WithTimeout(mo.ctx, model.timeout)
	defer cancel()

	// Attempt a simple health check (ping)
	healthy := mo.pingModel(ctx, model)

	if healthy {
		model.isHealthy.Store(true)
		atomic.AddInt32(&model.successCount, 1)
		atomic.StoreInt32(&model.failureCount, 0)
	} else {
		failures := atomic.AddInt32(&model.failureCount, 1)
		// Mark unhealthy after 3 consecutive failures
		if failures >= 3 {
			model.isHealthy.Store(false)
		}
	}

	model.lastHealthTime = time.Now()
}

// pingModel performs a real HTTP health check on a model
func (mo *ModelOrchestrator) pingModel(ctx context.Context, model *ModelInstance) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", model.baseURL+"/api/version", nil)
	if err != nil {
		return false
	}

	resp, err := mo.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Shutdown gracefully shuts down the orchestrator and all workers
func (mo *ModelOrchestrator) Shutdown(timeout time.Duration) error {
	var errs []error

	// Signal cancellation to all goroutines
	mo.cancel()

	// Create a done channel for coordinating shutdown
	done := make(chan struct{})

	go func() {
		// Wait for all goroutines to finish
		mo.wg.Wait()

		// Stop worker pool
		if mo.workerPool != nil {
			mo.workerPool.Stop()
		}

		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		return nil

	case <-time.After(timeout):
		errs = append(errs, fmt.Errorf("shutdown timeout exceeded (%v)", timeout))

		// Force stop worker pool
		if mo.workerPool != nil {
			mo.workerPool.Stop()
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// countHealthyModels returns the number of healthy models
func (mo *ModelOrchestrator) countHealthyModels() int {
	mo.mu.RLock()
	defer mo.mu.RUnlock()

	count := 0
	for _, model := range mo.models {
		if model.isHealthy.Load() {
			count++
		}
	}
	return count
}

// countTotalModels returns the total number of registered models
func (mo *ModelOrchestrator) countTotalModels() int {
	mo.mu.RLock()
	defer mo.mu.RUnlock()

	return len(mo.models)
}

// WorkerPool implementation

// Start initializes the worker pool with specified number of workers
func (wp *WorkerPool) Start() error {
	if wp.workers <= 0 {
		return errors.New("number of workers must be greater than 0")
	}

	// Start worker goroutines
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}

	return nil
}

// worker processes requests from the request channel
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()

	for {
		select {
		case req := <-wp.requestCh:
			if req == nil {
				return
			}

			wp.processRequest(req)

		case <-wp.ctx.Done():
			return
		}
	}
}

// processRequest handles the actual execution of a model request
func (wp *WorkerPool) processRequest(req *Request) {
	start := time.Now()

	// Create a timeout context for the request
	ctx, cancel := context.WithTimeout(wp.ctx, req.Timeout)
	defer cancel()

	// Execute the request (placeholder implementation)
	response, err := wp.executeRequest(ctx, req)

	duration := time.Since(start)

	// Send result
	result := RequestResult{
		Response: response,
		Error:    err,
		Duration: duration,
		Model:    req.ModelName,
	}

	select {
	case req.ResultCh <- result:
		// Close the result channel after sending result to signal completion
		close(req.ResultCh)
	case <-wp.ctx.Done():
		// Close channel if context is done to prevent goroutine leaks
		close(req.ResultCh)
	}
}

// executeRequest executes a single request to a model
func (wp *WorkerPool) executeRequest(ctx context.Context, req *Request) (string, error) {
	// This is a placeholder implementation
	// In production, this would make an actual HTTP call to the Ollama API

	// Simulate processing time
	select {
	case <-time.After(100 * time.Millisecond):
		return fmt.Sprintf("Response to prompt: %s", req.Prompt), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Stop shuts down the worker pool and waits for all workers to finish
func (wp *WorkerPool) Stop() {
	if wp.cancel != nil {
		wp.cancel()
	}

	// Close request channel to signal workers to stop
	close(wp.requestCh)

	// Wait for all workers to finish
	wp.wg.Wait()
}

// RequestBatch represents a batch of requests to be processed
type RequestBatch struct {
	requests []*Request
	results  []chan RequestResult
	mu       sync.Mutex
}

// NewRequestBatch creates a new batch of requests
func NewRequestBatch() *RequestBatch {
	return &RequestBatch{
		requests: make([]*Request, 0),
		results:  make([]chan RequestResult, 0),
	}
}

// Add adds a request to the batch
func (rb *RequestBatch) Add(req *Request) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.requests = append(rb.requests, req)
	rb.results = append(rb.results, req.ResultCh)
}

// WaitAll waits for all results in the batch
func (rb *RequestBatch) WaitAll(timeout time.Duration) []RequestResult {
	rb.mu.Lock()
	results := rb.results
	rb.mu.Unlock()

	allResults := make([]RequestResult, 0, len(results))
	timeoutCh := time.After(timeout)

	for i := 0; i < len(results); i++ {
		select {
		case result := <-results[i]:
			allResults = append(allResults, result)

		case <-timeoutCh:
			allResults = append(allResults, RequestResult{
				Error: fmt.Errorf("batch request %d timed out", i),
			})
		}
	}

	return allResults
}

// OrchestratorModelPool provides a pool of model instances for reuse
type OrchestratorModelPool struct {
	pool *sync.Pool
}

// NewOrchestratorModelPool creates a new orchestrator model pool
func NewOrchestratorModelPool(initialCapacity int) *OrchestratorModelPool {
	return &OrchestratorModelPool{
		pool: &sync.Pool{
			New: func() interface{} {
				return &ModelInstance{
					lastHealthTime: time.Now(),
				}
			},
		},
	}
}

// Get retrieves a model instance from the pool
func (mp *OrchestratorModelPool) Get() (*ModelInstance, error) {
	poolObj := mp.pool.Get()
	model, ok := poolObj.(*ModelInstance)
	if !ok {
		return nil, fmt.Errorf("failed to get model instance from pool: invalid type %T", poolObj)
	}
	return model, nil
}

// Put returns a model instance to the pool
func (mp *OrchestratorModelPool) Put(model *ModelInstance) {
	model.mu.Lock()
	model.name = ""
	model.baseURL = ""
	atomic.StoreInt32(&model.failureCount, 0)
	atomic.StoreInt32(&model.successCount, 0)
	model.mu.Unlock()

	mp.pool.Put(model)
}

// CircuitBreaker implements circuit breaker pattern for model requests
type CircuitBreaker struct {
	maxFailures      int32
	failureThreshold int32
	resetTimeout     time.Duration
	state            atomic.Value // "closed", "open", "half-open"
	lastFailureTime  time.Time
	mu               sync.Mutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int32, resetTimeout time.Duration) *CircuitBreaker {
	cb := &CircuitBreaker{
		maxFailures:      maxFailures,
		failureThreshold: maxFailures,
		resetTimeout:     resetTimeout,
	}
	cb.state.Store("closed")
	return cb
}

// IsClosed returns true if the circuit is closed (accepting requests)
func (cb *CircuitBreaker) IsClosed() bool {
	state, ok := cb.state.Load().(string)
	if !ok {
		return false
	}
	return state == "closed"
}

// IsOpen returns true if the circuit is open (rejecting requests)
func (cb *CircuitBreaker) IsOpen() bool {
	state, ok := cb.state.Load().(string)
	if !ok {
		return false
	}
	return state == "open"
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state, ok := cb.state.Load().(string)
	if !ok {
		return
	}
	if state != "closed" {
		cb.state.Store("closed")
		cb.failureThreshold = cb.maxFailures
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureThreshold--
	cb.lastFailureTime = time.Now()

	if cb.failureThreshold <= 0 {
		cb.state.Store("open")
		cb.failureThreshold = cb.maxFailures
	}
}

// AllowRequest checks if a request is allowed based on circuit breaker state
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state, ok := cb.state.Load().(string)
	if !ok {
		return false
	}

	if state == "closed" {
		return true
	}

	if state == "open" {
		// Check if reset timeout has passed
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			cb.state.Store("half-open")
			return true
		}
		return false
	}

	// half-open state
	return true
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxTokens float64, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed under the rate limit
func (rl *RateLimiter) Allow(cost float64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.tokens = min(rl.maxTokens, rl.tokens+elapsed*rl.refillRate)
	rl.lastRefill = now

	if rl.tokens >= cost {
		rl.tokens -= cost
		return true
	}

	return false
}

// Reset resets the rate limiter
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.tokens = rl.maxTokens
	rl.lastRefill = time.Now()
}

// Helper function
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
