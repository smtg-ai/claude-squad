# Ollama Configuration Management

This package provides comprehensive configuration management for Ollama endpoints and models within the Claude Squad application.

## Features

- **Multiple Endpoint Support**: Configure and manage multiple Ollama server endpoints with failover capabilities
- **Model-Specific Settings**: Configure temperature, context windows, and other parameters per model
- **Flexible Configuration**: Load from JSON or YAML files
- **Environment Variable Overrides**: Override configuration via environment variables
- **Validation**: Automatic validation with sensible defaults
- **Timeout & Retry Management**: Configure connection timeouts and retry policies
- **Priority-Based Failover**: Endpoints are tried in priority order

## Configuration Loading Order

Configuration is loaded in the following order (later overrides earlier):

1. **Defaults**: Built-in sensible defaults
2. **Configuration File**: `~/.claude-squad/ollama/ollama.json` or `.yaml`
3. **Environment Variables**: `OLLAMA_*` environment variables

## Configuration Structure

### OllamaConfig (Top-level)

```go
type OllamaConfig struct {
    Endpoints              []OllamaEndpoint
    DefaultEndpointName    string
    ConnectionTimeoutMS    int
    RequestTimeoutMS       int
    RetryPolicy            RetryPolicy
    Models                 map[string]ModelConfig
    Enabled                bool
}
```

### OllamaEndpoint

```go
type OllamaEndpoint struct {
    Name     string  // Friendly name for the endpoint
    URL      string  // Full URL (e.g., http://localhost:11434)
    Enabled  bool    // Whether this endpoint is active
    Priority int     // Sort order (0 = highest priority)
}
```

### RetryPolicy

```go
type RetryPolicy struct {
    MaxRetries int  // Maximum number of retry attempts
    BackoffMS  int  // Initial backoff in milliseconds
}
```

### ModelConfig

```go
type ModelConfig struct {
    Temperature    *float32  // Response randomness (0.0-1.0+)
    ContextWindow  *int      // Maximum tokens in context
    TopP           *float32  // Nucleus sampling (0.0-1.0)
    TopK           *int      // Top-K token filtering
    RepeatPenalty  *float32  // Penalty for token repetition
    NumPredict     *int      // Maximum tokens to generate
    Stop           []string  // Stop sequences
    System         *string   // System prompt
}
```

## Default Values

- **Default Endpoint**: `http://localhost:11434`
- **Connection Timeout**: 10 seconds
- **Request Timeout**: 60 seconds
- **Max Retries**: 3
- **Retry Backoff**: 500ms

## Usage Examples

### Basic Usage

```go
package main

import (
    "claude-squad/ollama"
    "log"
)

func main() {
    // Load configuration (from file or defaults)
    cfg := ollama.LoadOllamaConfig()

    // Get the default endpoint
    endpoint := cfg.GetDefaultEndpoint()
    if endpoint != nil {
        log.Printf("Using endpoint: %s (%s)", endpoint.Name, endpoint.URL)
    }

    // Get all enabled endpoints (in priority order)
    endpoints := cfg.GetEnabledEndpoints()
    for _, ep := range endpoints {
        log.Printf("Available: %s -> %s", ep.Name, ep.URL)
    }
}
```

### Model-Specific Configuration

```go
func main() {
    cfg := ollama.LoadOllamaConfig()

    // Get configuration for a specific model
    modelCfg := cfg.GetModelConfig("llama2")
    if modelCfg != nil && modelCfg.Temperature != nil {
        log.Printf("Temperature for llama2: %f", *modelCfg.Temperature)
    }

    // Set configuration for a model
    temp := float32(0.7)
    ctx := 4096
    cfg.SetModelConfig("new-model", ollama.ModelConfig{
        Temperature:   &temp,
        ContextWindow: &ctx,
    })
}
```

### Environment Variable Overrides

```bash
# Single endpoint
export OLLAMA_ENDPOINT="http://remote-host:11434"

# Multiple endpoints (comma-separated)
export OLLAMA_ENDPOINT="http://host1:11434,http://host2:11434"

# Set default endpoint
export OLLAMA_DEFAULT_ENDPOINT="remote-host"

# Timeouts (milliseconds)
export OLLAMA_CONNECTION_TIMEOUT_MS="5000"
export OLLAMA_REQUEST_TIMEOUT_MS="120000"

# Retry policy
export OLLAMA_MAX_RETRIES="5"
export OLLAMA_RETRY_BACKOFF_MS="1000"

# Enable/disable
export OLLAMA_ENABLED="true"
```

### Configuration File

Configuration files are stored at `~/.claude-squad/ollama/ollama.json` (or `.yaml`).

#### JSON Example

```json
{
  "enabled": true,
  "endpoints": [
    {
      "name": "local",
      "url": "http://localhost:11434",
      "enabled": true,
      "priority": 0
    }
  ],
  "default_endpoint": "local",
  "connection_timeout_ms": 10000,
  "request_timeout_ms": 60000,
  "retry_policy": {
    "max_retries": 3,
    "backoff_ms": 500
  },
  "models": {
    "llama2": {
      "temperature": 0.7,
      "context_window": 4096
    }
  }
}
```

#### YAML Example

```yaml
enabled: true
endpoints:
  - name: local
    url: http://localhost:11434
    enabled: true
    priority: 0
default_endpoint: local
connection_timeout_ms: 10000
request_timeout_ms: 60000
retry_policy:
  max_retries: 3
  backoff_ms: 500
models:
  llama2:
    temperature: 0.7
    context_window: 4096
```

## Endpoint Failover

Endpoints are automatically used in priority order (lower number = higher priority):

```go
cfg := ollama.LoadOllamaConfig()

// Get all enabled endpoints in priority order
endpoints := cfg.GetEnabledEndpoints()

// Try each endpoint in order
for _, ep := range endpoints {
    // Attempt connection to ep.URL
    // On success, break
    // On failure, try next endpoint
}
```

## Validation

Validate a configuration:

```go
cfg := ollama.LoadOllamaConfig()

if err := cfg.Validate(); err != nil {
    log.Fatalf("Invalid configuration: %v", err)
}
```

The validator checks:
- At least one endpoint is configured
- At least one endpoint is enabled
- All endpoints have valid URLs
- Timeouts are positive
- Max retries is non-negative

## Merging Configurations

Merge two configurations:

```go
cfg1 := ollama.DefaultOllamaConfig()
cfg2, _ := ollama.LoadOllamaConfigFromFile("/path/to/custom.json")

// Merge cfg2 into cfg1
merged := cfg1.Merge(cfg2)
```

## Saving Configuration

Save configuration to disk:

```go
cfg := ollama.LoadOllamaConfig()

// Modify configuration
temp := float32(0.5)
cfg.SetModelConfig("llama2", ollama.ModelConfig{
    Temperature: &temp,
})

// Save
err := ollama.SaveOllamaConfig(cfg)
if err != nil {
    log.Fatalf("Failed to save config: %v", err)
}
```

## Integration with Main Config

The Ollama configuration can be integrated with the main application config:

```go
func main() {
    // Load main config
    mainConfig := config.LoadConfig()

    // Load Ollama config
    ollamaConfig := ollama.LoadOllamaConfig()

    // Use both configurations
    if ollamaConfig.Enabled {
        log.Printf("Ollama is enabled")
        ep := ollamaConfig.GetDefaultEndpoint()
        if ep != nil {
            log.Printf("Using endpoint: %s", ep.URL)
        }
    }
}
```

## Testing

Run the configuration tests:

```bash
go test ./ollama -v
```

## Files

- `config.go` - Main configuration implementation
- `config_test.go` - Comprehensive unit tests
- `examples/ollama-config.example.json` - JSON configuration example
- `examples/ollama-config.example.yaml` - YAML configuration example
- `README.md` - This documentation

## Error Handling

The package gracefully handles errors:

- Missing configuration files: Returns defaults
- Invalid JSON/YAML: Returns defaults with warning
- Invalid environment variables: Skips with warning
- Missing endpoints: Uses defaults

All errors are logged via the `log` package.

## Future Enhancements

Potential improvements:
- Support for authentication (API keys, tokens)
- Per-endpoint timeouts
- Connection pooling configuration
- Load balancing strategies
- Health check endpoints
- Automatic endpoint discovery
