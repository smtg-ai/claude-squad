package ollama

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ModelMetadata contains comprehensive information about an Ollama model
type ModelMetadata struct {
	// Name is the unique identifier for the model (e.g., "llama2", "mistral")
	Name string

	// FullName is the fully qualified model name (e.g., "llama2:7b", "mistral:7b-instruct")
	FullName string

	// DisplayName is a human-readable name for the model
	DisplayName string

	// Description is a detailed description of the model's capabilities and use cases
	Description string

	// Version is the version of the model
	Version string

	// Size is the model size in bytes
	Size int64

	// Parameters is the number of parameters in the model
	Parameters string

	// Modified is the timestamp when the model was last modified
	Modified time.Time

	// CreatedAt is when the model was created/registered
	CreatedAt time.Time

	// Digest is the SHA256 digest of the model
	Digest string

	// Status indicates the current operational status of the model
	Status FrameworkModelStatus

	// Attributes contains model-specific attributes and capabilities
	Attributes map[string]interface{}
}

// FrameworkModelConfig represents the framework-level configuration for a model
type FrameworkModelConfig struct {
	// Enabled indicates if this model should be used
	Enabled bool

	// Priority determines the order of model selection (lower is higher priority)
	Priority int

	// MaxConcurrentRequests limits concurrent requests to this model
	MaxConcurrentRequests int

	// TimeoutSeconds is the request timeout for this model
	TimeoutSeconds int

	// RequestOptions contains default request options for this model
	RequestOptions RequestOptions

	// CustomHeaders are model-specific HTTP headers
	CustomHeaders map[string]string

	// Labels are tags for categorizing the model
	Labels []string

	// Metadata contains custom metadata for the model
	Metadata map[string]interface{}
}

// ModelRegistry manages available models and their configurations
type ModelRegistry struct {
	mu           sync.RWMutex
	models       map[string]*ModelMetadata
	configs      map[string]*FrameworkModelConfig
	providers    []ModelProvider
	defaultModel string
}

// NewModelRegistry creates a new ModelRegistry instance
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		models:    make(map[string]*ModelMetadata),
		configs:   make(map[string]*FrameworkModelConfig),
		providers: make([]ModelProvider, 0),
	}
}

// RegisterModel registers a model in the registry
func (mr *ModelRegistry) RegisterModel(model *ModelMetadata, config *FrameworkModelConfig) error {
	if model == nil {
		return NewFrameworkError(ErrCodeInvalidRequest, "model cannot be nil", nil)
	}
	if model.Name == "" {
		return NewFrameworkError(ErrCodeInvalidRequest, "model name cannot be empty", nil)
	}

	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.models[model.Name] = model

	// Use provided config or create a default one
	if config != nil {
		mr.configs[model.Name] = config
	} else {
		mr.configs[model.Name] = &FrameworkModelConfig{
			Enabled:               true,
			Priority:              0,
			MaxConcurrentRequests: 1,
			TimeoutSeconds:        30,
			RequestOptions:        RequestOptions{Stream: false},
			CustomHeaders:         make(map[string]string),
			Labels:                []string{},
			Metadata:              make(map[string]interface{}),
		}
	}

	return nil
}

// GetModel retrieves a model by name from the registry
func (mr *ModelRegistry) GetModel(name string) (*ModelMetadata, error) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	model, exists := mr.models[name]
	if !exists {
		return nil, NewFrameworkError(ErrCodeNotFound, fmt.Sprintf("model %q not found", name), nil)
	}
	return model, nil
}

// GetModelConfig retrieves the configuration for a model
func (mr *ModelRegistry) GetModelConfig(name string) (*FrameworkModelConfig, error) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	config, exists := mr.configs[name]
	if !exists {
		return nil, NewFrameworkError(ErrCodeNotFound, fmt.Sprintf("model config for %q not found", name), nil)
	}
	return config, nil
}

// ListModels returns all registered models
func (mr *ModelRegistry) ListModels() []*ModelMetadata {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	models := make([]*ModelMetadata, 0, len(mr.models))
	for _, model := range mr.models {
		models = append(models, model)
	}
	return models
}

// ListEnabledModels returns all enabled models
func (mr *ModelRegistry) ListEnabledModels() []*ModelMetadata {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	models := make([]*ModelMetadata, 0)
	for name, model := range mr.models {
		if config, exists := mr.configs[name]; exists && config.Enabled {
			models = append(models, model)
		}
	}
	return models
}

// RemoveModel removes a model from the registry
func (mr *ModelRegistry) RemoveModel(name string) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if _, exists := mr.models[name]; !exists {
		return NewFrameworkError(ErrCodeNotFound, fmt.Sprintf("model %q not found", name), nil)
	}
	delete(mr.models, name)
	delete(mr.configs, name)
	return nil
}

// SetDefaultModel sets the default model for requests
func (mr *ModelRegistry) SetDefaultModel(name string) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if _, exists := mr.models[name]; !exists {
		return NewFrameworkError(ErrCodeInvalidModel, fmt.Sprintf("model %q not found", name), nil)
	}
	mr.defaultModel = name
	return nil
}

// GetDefaultModel returns the name of the default model
func (mr *ModelRegistry) GetDefaultModel() string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	return mr.defaultModel
}

// RegisterProvider adds a ModelProvider to the registry
func (mr *ModelRegistry) RegisterProvider(provider ModelProvider) error {
	if provider == nil {
		return NewFrameworkError(ErrCodeInvalidRequest, "provider cannot be nil", nil)
	}

	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.providers = append(mr.providers, provider)
	return nil
}

// SyncModels fetches and registers models from all providers
func (mr *ModelRegistry) SyncModels(ctx context.Context) error {
	// Copy providers list under read lock
	mr.mu.RLock()
	providers := append([]ModelProvider{}, mr.providers...)
	mr.mu.RUnlock()

	for _, provider := range providers {
		models, err := provider.FetchModels(ctx)
		if err != nil {
			return NewFrameworkError(ErrCodeInternal, fmt.Sprintf("failed to sync models from provider %q", provider.Name()), err)
		}

		for _, model := range models {
			// Only register if not already registered, to preserve custom configs
			mr.mu.RLock()
			_, exists := mr.models[model.Name]
			mr.mu.RUnlock()

			if !exists {
				if err := mr.RegisterModel(model, nil); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// IsModelAvailable checks if a model is available and enabled
func (mr *ModelRegistry) IsModelAvailable(name string) bool {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	model, exists := mr.models[name]
	if !exists {
		return false
	}

	config, exists := mr.configs[name]
	if !exists {
		return false
	}

	return config.Enabled && model.Status == FrameworkModelStatusAvailable
}

// UpdateModelStatus updates the status of a model
func (mr *ModelRegistry) UpdateModelStatus(name string, status FrameworkModelStatus) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	model, exists := mr.models[name]
	if !exists {
		return NewFrameworkError(ErrCodeNotFound, fmt.Sprintf("model %q not found", name), nil)
	}

	model.Status = status
	return nil
}
