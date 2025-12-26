package ollama

import (
	"claude-squad/config"
	"claude-squad/log"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	OllamaConfigFileName     = "ollama.json"
	DefaultOllamaEndpoint    = "http://localhost:11434"
	DefaultConnectionTimeout = 10 * time.Second
	DefaultRequestTimeout    = 60 * time.Second
	DefaultMaxRetries        = 3
	DefaultRetryBackoff      = 500 * time.Millisecond
)

// Environment variable names for Ollama configuration overrides
const (
	// EnvOllamaEnabled enables or disables Ollama integration.
	// Valid values: "true", "false", "1", "0" (case-insensitive)
	// Default: true
	EnvOllamaEnabled = "OLLAMA_ENABLED"

	// EnvOllamaEndpoint specifies Ollama server endpoint(s).
	// Format: Single URL or comma-separated list (e.g., "http://host1:11434,http://host2:11434")
	// Default: http://localhost:11434
	EnvOllamaEndpoint = "OLLAMA_ENDPOINT"

	// EnvOllamaDefaultEndpoint specifies the name of the default endpoint to use.
	// Must match the "name" field of a configured endpoint.
	// Default: "local"
	EnvOllamaDefaultEndpoint = "OLLAMA_DEFAULT_ENDPOINT"

	// EnvOllamaConnectionTimeoutMS specifies connection timeout in milliseconds.
	// Valid range: 1-60000 (1ms to 1 minute)
	// Default: 10000 (10 seconds)
	EnvOllamaConnectionTimeoutMS = "OLLAMA_CONNECTION_TIMEOUT_MS"

	// EnvOllamaRequestTimeoutMS specifies request timeout in milliseconds.
	// Valid range: 1-600000 (1ms to 10 minutes)
	// Default: 60000 (60 seconds)
	EnvOllamaRequestTimeoutMS = "OLLAMA_REQUEST_TIMEOUT_MS"

	// EnvOllamaMaxRetries specifies maximum number of retry attempts.
	// Must be non-negative.
	// Default: 3
	EnvOllamaMaxRetries = "OLLAMA_MAX_RETRIES"

	// EnvOllamaRetryBackoffMS specifies retry backoff duration in milliseconds.
	// Must be positive.
	// Default: 500
	EnvOllamaRetryBackoffMS = "OLLAMA_RETRY_BACKOFF_MS"
)

// RetryPolicy defines the retry behavior for Ollama API calls.
// Default values: MaxRetries=3, BackoffMS=500
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts (default: 3)
	// JSON/YAML key: "max_retries"
	MaxRetries int `json:"max_retries" yaml:"max_retries"`

	// BackoffMS is the initial backoff duration in milliseconds (default: 500)
	// JSON/YAML key: "backoff_ms"
	BackoffMS int `json:"backoff_ms" yaml:"backoff_ms"`

	// backoff is the computed backoff duration (internal use only)
	backoff time.Duration `json:"-" yaml:"-"`
}

// ModelConfig contains configuration for a specific Ollama model.
// All fields are optional (nil means use model defaults).
type ModelConfig struct {
	// Temperature controls randomness of responses (0.0 to 1.0 or higher)
	// JSON/YAML key: "temperature"
	Temperature *float32 `json:"temperature,omitempty" yaml:"temperature,omitempty"`

	// ContextWindow is the maximum number of tokens in context
	// JSON/YAML key: "context_window"
	ContextWindow *int `json:"context_window,omitempty" yaml:"context_window,omitempty"`

	// TopP is the nucleus sampling parameter (0.0 to 1.0)
	// JSON/YAML key: "top_p"
	TopP *float32 `json:"top_p,omitempty" yaml:"top_p,omitempty"`

	// TopK limits token selection to top K options
	// JSON/YAML key: "top_k"
	TopK *int `json:"top_k,omitempty" yaml:"top_k,omitempty"`

	// RepeatPenalty penalizes repeated tokens
	// JSON/YAML key: "repeat_penalty"
	RepeatPenalty *float32 `json:"repeat_penalty,omitempty" yaml:"repeat_penalty,omitempty"`

	// NumPredict is the number of tokens to predict
	// JSON/YAML key: "num_predict"
	NumPredict *int `json:"num_predict,omitempty" yaml:"num_predict,omitempty"`

	// Stop sequences for generation
	// JSON/YAML key: "stop"
	Stop []string `json:"stop,omitempty" yaml:"stop,omitempty"`

	// System prompt for the model
	// JSON/YAML key: "system"
	System *string `json:"system,omitempty" yaml:"system,omitempty"`
}

// OllamaEndpoint represents a single Ollama server endpoint.
// Endpoints are tried in priority order (lower number = higher priority).
type OllamaEndpoint struct {
	// Name is a friendly identifier for this endpoint
	// JSON/YAML key: "name"
	Name string `json:"name" yaml:"name"`

	// URL is the endpoint URL (e.g., http://localhost:11434)
	// JSON/YAML key: "url"
	URL string `json:"url" yaml:"url"`

	// Enabled indicates if this endpoint should be used (default: true)
	// JSON/YAML key: "enabled"
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Priority determines the order in which endpoints are tried (lower is higher priority, default: 0)
	// JSON/YAML key: "priority"
	Priority int `json:"priority" yaml:"priority"`
}

// OllamaConfig holds the complete Ollama configuration.
// Configuration is loaded from: defaults → config file → environment variables.
// See DefaultOllamaConfig() for default values.
type OllamaConfig struct {
	// Endpoints is a list of available Ollama server endpoints (default: single "local" endpoint at http://localhost:11434)
	// JSON/YAML key: "endpoints"
	Endpoints []OllamaEndpoint `json:"endpoints" yaml:"endpoints"`

	// DefaultEndpointName is the name of the endpoint to use by default (default: "local")
	// JSON/YAML key: "default_endpoint"
	DefaultEndpointName string `json:"default_endpoint" yaml:"default_endpoint"`

	// ConnectionTimeoutMS is the timeout for establishing a connection in milliseconds (default: 10000 = 10 seconds)
	// JSON/YAML key: "connection_timeout_ms"
	ConnectionTimeoutMS int `json:"connection_timeout_ms" yaml:"connection_timeout_ms"`
	// connectionTimeout is the computed timeout duration (internal use only)
	connectionTimeout time.Duration `json:"-" yaml:"-"`

	// RequestTimeoutMS is the timeout for a single request in milliseconds (default: 60000 = 60 seconds)
	// JSON/YAML key: "request_timeout_ms"
	RequestTimeoutMS int `json:"request_timeout_ms" yaml:"request_timeout_ms"`
	// requestTimeout is the computed timeout duration (internal use only)
	requestTimeout time.Duration `json:"-" yaml:"-"`

	// RetryPolicy defines how to retry failed requests (default: MaxRetries=3, BackoffMS=500)
	// JSON/YAML key: "retry_policy"
	RetryPolicy RetryPolicy `json:"retry_policy" yaml:"retry_policy"`

	// Models contains model-specific configurations (default: empty map)
	// JSON/YAML key: "models"
	Models map[string]ModelConfig `json:"models" yaml:"models"`

	// Enabled indicates if Ollama integration is enabled (default: true)
	// JSON/YAML key: "enabled"
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// DefaultOllamaConfig returns a configuration with sensible defaults
func DefaultOllamaConfig() *OllamaConfig {
	return &OllamaConfig{
		Endpoints: []OllamaEndpoint{
			{
				Name:     "local",
				URL:      DefaultOllamaEndpoint,
				Enabled:  true,
				Priority: 0,
			},
		},
		DefaultEndpointName: "local",
		ConnectionTimeoutMS: int(DefaultConnectionTimeout.Milliseconds()),
		RequestTimeoutMS:    int(DefaultRequestTimeout.Milliseconds()),
		RetryPolicy: RetryPolicy{
			MaxRetries: DefaultMaxRetries,
			BackoffMS:  int(DefaultRetryBackoff.Milliseconds()),
		},
		Models:  make(map[string]ModelConfig),
		Enabled: true,
	}
}

// GetOllamaConfigDir returns the directory where Ollama config is stored
func GetOllamaConfigDir() (string, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config directory: %w", err)
	}
	return filepath.Join(configDir, "ollama"), nil
}

// validatePath checks for path traversal attempts
func validatePath(path string) error {
	// Clean the path to normalize it
	cleanPath := filepath.Clean(path)

	// Check for path traversal patterns
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path contains invalid traversal pattern: %s", path)
	}

	return nil
}

// LoadOllamaConfig loads Ollama configuration from file with environment variable overrides
func LoadOllamaConfig() *OllamaConfig {
	configDir, err := GetOllamaConfigDir()
	if err != nil {
		log.WarningLog.Printf("failed to get ollama config directory: %v, using defaults", err)
		return applyEnvironmentOverrides(DefaultOllamaConfig())
	}

	configPath := filepath.Join(configDir, OllamaConfigFileName)

	// Validate path for security
	if err := validatePath(configPath); err != nil {
		log.ErrorLog.Printf("invalid config path: %v, using defaults", err)
		return applyEnvironmentOverrides(DefaultOllamaConfig())
	}

	// Normalize path
	configPath = filepath.Clean(configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create and save default config if file doesn't exist
			defaultCfg := DefaultOllamaConfig()
			if saveErr := saveOllamaConfig(defaultCfg); saveErr != nil {
				log.WarningLog.Printf("failed to save default ollama config: %v", saveErr)
			}
			return applyEnvironmentOverrides(defaultCfg)
		}

		log.WarningLog.Printf("failed to read ollama config file: %v, using defaults", err)
		return applyEnvironmentOverrides(DefaultOllamaConfig())
	}

	ollamaCfg := DefaultOllamaConfig()

	// Try JSON first, then YAML
	if err := json.Unmarshal(data, ollamaCfg); err != nil {
		if yamlErr := yaml.Unmarshal(data, ollamaCfg); yamlErr != nil {
			log.ErrorLog.Printf("failed to parse ollama config (tried JSON and YAML): %v", err)
			return applyEnvironmentOverrides(DefaultOllamaConfig())
		}
	}

	// Validate and apply defaults
	ollamaCfg = validateAndApplyDefaults(ollamaCfg)

	// Apply environment variable overrides
	return applyEnvironmentOverrides(ollamaCfg)
}

// LoadOllamaConfigFromFile loads Ollama configuration from a specific file path
func LoadOllamaConfigFromFile(filePath string) (*OllamaConfig, error) {
	// Validate path for security
	if err := validatePath(filePath); err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}

	// Normalize path
	filePath = filepath.Clean(filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	ollamaCfg := DefaultOllamaConfig()

	// Detect format based on file extension
	if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {
		if err := yaml.Unmarshal(data, ollamaCfg); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	} else {
		// Try JSON first, then YAML
		if err := json.Unmarshal(data, ollamaCfg); err != nil {
			if yamlErr := yaml.Unmarshal(data, ollamaCfg); yamlErr != nil {
				return nil, fmt.Errorf("failed to parse config (tried JSON and YAML): %w", err)
			}
		}
	}

	ollamaCfg = validateAndApplyDefaults(ollamaCfg)
	return applyEnvironmentOverrides(ollamaCfg), nil
}

// validateAndApplyDefaults validates the configuration and applies sensible defaults
func validateAndApplyDefaults(cfg *OllamaConfig) *OllamaConfig {
	// Validate connection timeout (min > 0, max 60000ms = 1 minute)
	if cfg.ConnectionTimeoutMS <= 0 || cfg.ConnectionTimeoutMS > 60000 {
		originalTimeout := cfg.ConnectionTimeoutMS
		cfg.ConnectionTimeoutMS = int(DefaultConnectionTimeout.Milliseconds())
		if originalTimeout > 60000 {
			log.WarningLog.Printf("connection timeout %dms exceeds maximum 60000ms, reset to default %dms",
				originalTimeout, cfg.ConnectionTimeoutMS)
		}
	}
	cfg.connectionTimeout = time.Duration(cfg.ConnectionTimeoutMS) * time.Millisecond

	// Validate request timeout (min > 0, max 600000ms = 10 minutes)
	if cfg.RequestTimeoutMS <= 0 || cfg.RequestTimeoutMS > 600000 {
		originalTimeout := cfg.RequestTimeoutMS
		cfg.RequestTimeoutMS = int(DefaultRequestTimeout.Milliseconds())
		if originalTimeout > 600000 {
			log.WarningLog.Printf("request timeout %dms exceeds maximum 600000ms, reset to default %dms",
				originalTimeout, cfg.RequestTimeoutMS)
		}
	}
	cfg.requestTimeout = time.Duration(cfg.RequestTimeoutMS) * time.Millisecond

	// Validate retry policy
	if cfg.RetryPolicy.MaxRetries < 0 {
		cfg.RetryPolicy.MaxRetries = DefaultMaxRetries
	}
	if cfg.RetryPolicy.BackoffMS <= 0 {
		cfg.RetryPolicy.BackoffMS = int(DefaultRetryBackoff.Milliseconds())
	}
	cfg.RetryPolicy.backoff = time.Duration(cfg.RetryPolicy.BackoffMS) * time.Millisecond

	// Validate and clean endpoints
	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints = []OllamaEndpoint{
			{
				Name:     "local",
				URL:      DefaultOllamaEndpoint,
				Enabled:  true,
				Priority: 0,
			},
		}
	}

	// Ensure all endpoint URLs are properly formatted
	for i := range cfg.Endpoints {
		cfg.Endpoints[i].URL = strings.TrimSpace(cfg.Endpoints[i].URL)
		if cfg.Endpoints[i].URL == "" {
			cfg.Endpoints[i].URL = DefaultOllamaEndpoint
		}

		// Ensure URL doesn't have trailing slash
		cfg.Endpoints[i].URL = strings.TrimSuffix(cfg.Endpoints[i].URL, "/")
	}

	// Validate default endpoint
	if cfg.DefaultEndpointName == "" {
		if len(cfg.Endpoints) > 0 {
			cfg.DefaultEndpointName = cfg.Endpoints[0].Name
		} else {
			cfg.DefaultEndpointName = "local"
		}
	}

	// Ensure Models map is initialized
	if cfg.Models == nil {
		cfg.Models = make(map[string]ModelConfig)
	}

	return cfg
}

// applyEnvironmentOverrides applies environment variable overrides to the configuration
func applyEnvironmentOverrides(cfg *OllamaConfig) *OllamaConfig {
	// OLLAMA_ENABLED
	if enabled := os.Getenv(EnvOllamaEnabled); enabled != "" {
		cfg.Enabled = strings.ToLower(enabled) == "true" || enabled == "1"
	}

	// OLLAMA_ENDPOINT (comma-separated list of endpoints)
	if endpoint := os.Getenv(EnvOllamaEndpoint); endpoint != "" {
		endpoints := strings.Split(endpoint, ",")
		cfg.Endpoints = []OllamaEndpoint{}
		for i, ep := range endpoints {
			ep = strings.TrimSpace(ep)
			if ep != "" {
				cfg.Endpoints = append(cfg.Endpoints, OllamaEndpoint{
					Name:     fmt.Sprintf("env-endpoint-%d", i),
					URL:      ep,
					Enabled:  true,
					Priority: i,
				})
			}
		}
		if len(cfg.Endpoints) > 0 {
			cfg.DefaultEndpointName = cfg.Endpoints[0].Name
		}
	}

	// OLLAMA_DEFAULT_ENDPOINT
	if defaultEp := os.Getenv(EnvOllamaDefaultEndpoint); defaultEp != "" {
		cfg.DefaultEndpointName = strings.TrimSpace(defaultEp)
	}

	// OLLAMA_CONNECTION_TIMEOUT_MS
	if timeout := os.Getenv(EnvOllamaConnectionTimeoutMS); timeout != "" {
		if ms := parseIntEnv(timeout); ms > 0 {
			cfg.ConnectionTimeoutMS = ms
			cfg.connectionTimeout = time.Duration(ms) * time.Millisecond
		}
	}

	// OLLAMA_REQUEST_TIMEOUT_MS
	if timeout := os.Getenv(EnvOllamaRequestTimeoutMS); timeout != "" {
		if ms := parseIntEnv(timeout); ms > 0 {
			cfg.RequestTimeoutMS = ms
			cfg.requestTimeout = time.Duration(ms) * time.Millisecond
		}
	}

	// OLLAMA_MAX_RETRIES
	if retries := os.Getenv(EnvOllamaMaxRetries); retries != "" {
		if maxRetries := parseIntEnv(retries); maxRetries >= 0 {
			cfg.RetryPolicy.MaxRetries = maxRetries
		}
	}

	// OLLAMA_RETRY_BACKOFF_MS
	if backoff := os.Getenv(EnvOllamaRetryBackoffMS); backoff != "" {
		if ms := parseIntEnv(backoff); ms > 0 {
			cfg.RetryPolicy.BackoffMS = ms
			cfg.RetryPolicy.backoff = time.Duration(ms) * time.Millisecond
		}
	}

	return cfg
}

// parseIntEnv safely parses an integer environment variable
func parseIntEnv(value string) int {
	// Simple parser without importing strconv to match existing patterns
	result := 0
	for _, ch := range strings.TrimSpace(value) {
		if ch >= '0' && ch <= '9' {
			result = result*10 + int(ch-'0')
		} else {
			return -1 // Invalid value
		}
	}
	return result
}

// saveOllamaConfig saves the Ollama configuration to disk
func saveOllamaConfig(cfg *OllamaConfig) error {
	configDir, err := GetOllamaConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get ollama config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create ollama config directory: %w", err)
	}

	configPath := filepath.Join(configDir, OllamaConfigFileName)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ollama config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

// SaveOllamaConfig exports the saveOllamaConfig function for use by other packages
func SaveOllamaConfig(cfg *OllamaConfig) error {
	return saveOllamaConfig(cfg)
}

// GetConnectionTimeout returns the connection timeout as a time.Duration
func (c *OllamaConfig) GetConnectionTimeout() time.Duration {
	if c.connectionTimeout == 0 {
		return time.Duration(c.ConnectionTimeoutMS) * time.Millisecond
	}
	return c.connectionTimeout
}

// GetRequestTimeout returns the request timeout as a time.Duration
func (c *OllamaConfig) GetRequestTimeout() time.Duration {
	if c.requestTimeout == 0 {
		return time.Duration(c.RequestTimeoutMS) * time.Millisecond
	}
	return c.requestTimeout
}

// GetRetryBackoff returns the retry backoff duration
func (p *RetryPolicy) GetBackoff() time.Duration {
	if p.backoff == 0 {
		return time.Duration(p.BackoffMS) * time.Millisecond
	}
	return p.backoff
}

// GetDefaultEndpoint returns the default Ollama endpoint
func (c *OllamaConfig) GetDefaultEndpoint() *OllamaEndpoint {
	for i := range c.Endpoints {
		if c.Endpoints[i].Name == c.DefaultEndpointName && c.Endpoints[i].Enabled {
			return &c.Endpoints[i]
		}
	}

	// Fallback to first enabled endpoint
	for i := range c.Endpoints {
		if c.Endpoints[i].Enabled {
			return &c.Endpoints[i]
		}
	}

	return nil
}

// GetEnabledEndpoints returns all enabled endpoints sorted by priority
func (c *OllamaConfig) GetEnabledEndpoints() []OllamaEndpoint {
	var enabled []OllamaEndpoint
	for _, ep := range c.Endpoints {
		if ep.Enabled {
			enabled = append(enabled, ep)
		}
	}

	// Sort by priority (lower is higher priority)
	for i := 0; i < len(enabled); i++ {
		for j := i + 1; j < len(enabled); j++ {
			if enabled[j].Priority < enabled[i].Priority {
				enabled[i], enabled[j] = enabled[j], enabled[i]
			}
		}
	}

	return enabled
}

// GetModelConfig returns the configuration for a specific model
func (c *OllamaConfig) GetModelConfig(modelName string) *ModelConfig {
	if cfg, ok := c.Models[modelName]; ok {
		return &cfg
	}
	return nil
}

// SetModelConfig sets the configuration for a specific model
func (c *OllamaConfig) SetModelConfig(modelName string, cfg ModelConfig) {
	if modelName == "" {
		return
	}
	if c.Models == nil {
		c.Models = make(map[string]ModelConfig)
	}
	c.Models[modelName] = cfg
}

// Validate checks if the configuration is valid
func (c *OllamaConfig) Validate() error {
	if len(c.Endpoints) == 0 {
		return fmt.Errorf("at least one endpoint must be configured")
	}

	hasEnabled := false
	for _, ep := range c.Endpoints {
		if ep.Enabled {
			hasEnabled = true
		}
		if ep.URL == "" {
			return fmt.Errorf("endpoint %q has empty URL", ep.Name)
		}
	}

	if !hasEnabled {
		return fmt.Errorf("at least one endpoint must be enabled")
	}

	if c.RetryPolicy.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be non-negative")
	}

	if c.ConnectionTimeoutMS <= 0 {
		return fmt.Errorf("connection_timeout_ms must be positive")
	}

	if c.RequestTimeoutMS <= 0 {
		return fmt.Errorf("request_timeout_ms must be positive")
	}

	return nil
}

// Merge merges another OllamaConfig into this one, with the other taking precedence
func (c *OllamaConfig) Merge(other *OllamaConfig) *OllamaConfig {
	if other == nil {
		return c
	}

	if other.Enabled {
		c.Enabled = other.Enabled
	}

	if len(other.Endpoints) > 0 {
		c.Endpoints = other.Endpoints
	}

	if other.DefaultEndpointName != "" {
		c.DefaultEndpointName = other.DefaultEndpointName
	}

	if other.ConnectionTimeoutMS > 0 {
		c.ConnectionTimeoutMS = other.ConnectionTimeoutMS
		c.connectionTimeout = time.Duration(other.ConnectionTimeoutMS) * time.Millisecond
	}

	if other.RequestTimeoutMS > 0 {
		c.RequestTimeoutMS = other.RequestTimeoutMS
		c.requestTimeout = time.Duration(other.RequestTimeoutMS) * time.Millisecond
	}

	if other.RetryPolicy.MaxRetries >= 0 {
		c.RetryPolicy.MaxRetries = other.RetryPolicy.MaxRetries
	}

	if other.RetryPolicy.BackoffMS > 0 {
		c.RetryPolicy.BackoffMS = other.RetryPolicy.BackoffMS
		c.RetryPolicy.backoff = time.Duration(other.RetryPolicy.BackoffMS) * time.Millisecond
	}

	if other.Models != nil && len(other.Models) > 0 {
		c.Models = other.Models
	}

	return c
}
