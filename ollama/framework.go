package ollama

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// OllamaFramework is the main entry point for Ollama integration with Claude Squad
//
// It manages the client connection, model registry, and health monitoring.
// The framework supports:
// - Automatic model discovery and registration
// - Health checking with configurable intervals
// - Request queuing and rate limiting
// - Extensible provider pattern for model sources
//
// Example usage:
//
//	config := &FrameworkConfig{
//		ClientConfig: &ClientConfig{
//			BaseURL: "http://localhost:11434",
//			Timeout: 30,
//		},
//		HealthCheckInterval: 10 * time.Second,
//	}
//	framework, err := NewOllamaFramework(config)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer framework.Close()
//
//	if healthy, err := framework.IsHealthy(context.Background()); !healthy {
//		log.Fatal("Ollama is not healthy")
//	}
type OllamaFramework struct {
	client                *Client
	registry              *ModelRegistry
	config                *FrameworkConfig
	healthChecker         HealthChecker
	mu                    sync.RWMutex
	isHealthy             bool
	lastHealthCheckTime   time.Time
	healthCheckTicker     *time.Ticker
	ctx                   context.Context
	cancel                context.CancelFunc
	wg                    sync.WaitGroup
	requestQueue          chan *frameworkRequest
	activeRequests        int
	maxConcurrentRequests int
}

// FrameworkConfig encapsulates the configuration for the OllamaFramework
type FrameworkConfig struct {
	// ClientConfig is the configuration for the Ollama API client
	ClientConfig *ClientConfig

	// HealthCheckInterval is how often to check Ollama health (0 = disabled)
	HealthCheckInterval time.Duration

	// MaxConcurrentRequests limits the number of concurrent framework requests
	MaxConcurrentRequests int

	// AutoSyncModels enables automatic model synchronization on startup
	AutoSyncModels bool

	// ModelProviders are custom model providers to register on startup
	ModelProviders []ModelProvider

	// DefaultTimeout is the default timeout for operations
	DefaultTimeout time.Duration
}

// frameworkRequest wraps a request being processed by the framework
type frameworkRequest struct {
	fn     func() error
	result chan error
}

// NewOllamaFramework creates a new OllamaFramework instance with the provided configuration
func NewOllamaFramework(config *FrameworkConfig) (*OllamaFramework, error) {
	if config == nil {
		return nil, NewFrameworkError(ErrCodeInvalidRequest, "framework config cannot be nil", nil)
	}

	if config.ClientConfig == nil {
		return nil, NewFrameworkError(ErrCodeInvalidRequest, "client config cannot be nil", nil)
	}

	// Create the client
	client, err := NewClient(config.ClientConfig)
	if err != nil {
		return nil, err
	}

	// Set defaults
	if config.MaxConcurrentRequests <= 0 {
		config.MaxConcurrentRequests = 10
	}

	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	f := &OllamaFramework{
		client:                client,
		registry:              NewModelRegistry(),
		config:                config,
		healthChecker:         client, // Use client as health checker by default
		mu:                    sync.RWMutex{},
		isHealthy:             false,
		ctx:                   ctx,
		cancel:                cancel,
		requestQueue:          make(chan *frameworkRequest, config.MaxConcurrentRequests),
		maxConcurrentRequests: config.MaxConcurrentRequests,
	}

	// Register model providers
	for _, provider := range config.ModelProviders {
		if err := f.registry.RegisterProvider(provider); err != nil {
			cancel()
			return nil, err
		}
	}

	// Start background workers
	f.wg.Add(1)
	go f.requestWorker()

	// Start health checker if enabled
	if config.HealthCheckInterval > 0 {
		f.healthCheckTicker = time.NewTicker(config.HealthCheckInterval)
		f.wg.Add(1)
		go f.healthCheckWorker()
	}

	// Perform initial health check
	healthy, _ := f.healthChecker.CheckHealth(ctx)
	f.setHealthy(healthy)

	// Auto-sync models if enabled
	if config.AutoSyncModels {
		if err := f.registry.SyncModels(ctx); err != nil {
			// Log error but don't fail initialization
			fmt.Printf("warning: failed to sync models: %v\n", err)
		}
	}

	return f, nil
}

// requestWorker processes requests from the queue
func (f *OllamaFramework) requestWorker() {
	defer f.wg.Done()
	for {
		select {
		case <-f.ctx.Done():
			return
		case req := <-f.requestQueue:
			f.mu.Lock()
			f.activeRequests++
			f.mu.Unlock()

			req.result <- req.fn()

			f.mu.Lock()
			f.activeRequests--
			f.mu.Unlock()
		}
	}
}

// healthCheckWorker periodically checks the health of the Ollama instance
func (f *OllamaFramework) healthCheckWorker() {
	defer f.wg.Done()
	for {
		select {
		case <-f.ctx.Done():
			return
		case <-f.healthCheckTicker.C:
			healthy, _ := f.healthChecker.CheckHealth(f.ctx)
			f.setHealthy(healthy)
		}
	}
}

// setHealthy updates the health status
func (f *OllamaFramework) setHealthy(healthy bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.isHealthy = healthy
	f.lastHealthCheckTime = time.Now()
}

// IsHealthy returns true if Ollama is currently healthy
func (f *OllamaFramework) IsHealthy(ctx context.Context) (bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.isHealthy, nil
}

// GetClient returns the underlying Ollama client
func (f *OllamaFramework) GetClient() *Client {
	return f.client
}

// GetRegistry returns the model registry
func (f *OllamaFramework) GetRegistry() *ModelRegistry {
	return f.registry
}

// Generate sends a generation request through the framework
func (f *OllamaFramework) Generate(ctx context.Context, model string, prompt string, opts *RequestOptions) (*GenerateResponse, error) {
	if !f.isHealthy {
		return nil, NewFrameworkError(ErrCodeConnectionFailed, "Ollama instance is not healthy", nil)
	}

	result := make(chan *GenerateResponse, 1)
	errChan := make(chan error, 1)

	go func() {
		resp, err := f.client.Generate(ctx, model, prompt, opts)
		if err != nil {
			errChan <- err
			return
		}
		result <- resp
	}()

	select {
	case <-ctx.Done():
		return nil, NewFrameworkError(ErrCodeTimeout, "request context cancelled", ctx.Err())
	case err := <-errChan:
		return nil, err
	case resp := <-result:
		return resp, nil
	}
}

// ListModels returns all available models from the registry
func (f *OllamaFramework) ListModels(ctx context.Context) ([]*ModelMetadata, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.registry.ListModels(), nil
}

// ListEnabledModels returns all enabled models from the registry
func (f *OllamaFramework) ListEnabledModels(ctx context.Context) ([]*ModelMetadata, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.registry.ListEnabledModels(), nil
}

// SyncModels synchronizes models from all registered providers
func (f *OllamaFramework) SyncModels(ctx context.Context) error {
	return f.registry.SyncModels(ctx)
}

// RegisterModel registers a model with the framework
func (f *OllamaFramework) RegisterModel(model *ModelMetadata, config *FrameworkModelConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.registry.RegisterModel(model, config)
}

// GetModel retrieves a model by name
func (f *OllamaFramework) GetModel(ctx context.Context, name string) (*ModelMetadata, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.registry.GetModel(name)
}

// PullModel downloads a model to the Ollama instance
func (f *OllamaFramework) PullModel(ctx context.Context, modelName string) error {
	if !f.isHealthy {
		return NewFrameworkError(ErrCodeConnectionFailed, "Ollama instance is not healthy", nil)
	}

	return f.client.PullModel(ctx, modelName)
}

// DeleteModel removes a model from the Ollama instance
func (f *OllamaFramework) DeleteModel(ctx context.Context, modelName string) error {
	if !f.isHealthy {
		return NewFrameworkError(ErrCodeConnectionFailed, "Ollama instance is not healthy", nil)
	}

	if err := f.client.DeleteModel(ctx, modelName); err != nil {
		return err
	}

	return f.registry.RemoveModel(modelName)
}

// GenerateEmbedding generates embeddings for a given prompt
func (f *OllamaFramework) GenerateEmbedding(ctx context.Context, model string, prompt string) ([]float32, error) {
	if !f.isHealthy {
		return nil, NewFrameworkError(ErrCodeConnectionFailed, "Ollama instance is not healthy", nil)
	}

	return f.client.GenerateEmbedding(ctx, model, prompt)
}

// GetActiveRequests returns the number of currently active requests
func (f *OllamaFramework) GetActiveRequests() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.activeRequests
}

// GetLastHealthCheckTime returns the time of the last health check
func (f *OllamaFramework) GetLastHealthCheckTime() time.Time {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.lastHealthCheckTime
}

// Close gracefully shuts down the framework
func (f *OllamaFramework) Close() error {
	f.cancel()

	// Stop health check ticker if it exists
	if f.healthCheckTicker != nil {
		f.healthCheckTicker.Stop()
	}

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		f.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		return NewFrameworkError(ErrCodeTimeout, "shutdown timeout", nil)
	}
}

// Status returns detailed framework status information
type Status struct {
	IsHealthy           bool
	LastHealthCheckTime time.Time
	ActiveRequests      int
	RegisteredModels    int
	EnabledModels       int
	FrameworkUptime     time.Duration
	ClientConnected     bool
}

// GetStatus returns the current framework status
func (f *OllamaFramework) GetStatus(ctx context.Context) (*Status, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return &Status{
		IsHealthy:           f.isHealthy,
		LastHealthCheckTime: f.lastHealthCheckTime,
		ActiveRequests:      f.activeRequests,
		RegisteredModels:    len(f.registry.ListModels()),
		EnabledModels:       len(f.registry.ListEnabledModels()),
		ClientConnected:     f.client != nil,
	}, nil
}
