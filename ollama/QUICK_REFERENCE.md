# Ollama Configuration Quick Reference

## Configuration File Location
```
~/.claude-squad/ollama/ollama.json
```

## Minimal Configuration
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
  "models": {}
}
```

## Common Tasks

### Load Configuration
```go
cfg := ollama.LoadOllamaConfig()
```

### Get Default Endpoint
```go
endpoint := cfg.GetDefaultEndpoint()
fmt.Println(endpoint.URL)
```

### Get All Endpoints
```go
endpoints := cfg.GetEnabledEndpoints()
for _, ep := range endpoints {
    fmt.Println(ep.Name, ep.URL)
}
```

### Configure a Model
```go
temp := float32(0.7)
ctx := 4096
cfg.SetModelConfig("llama2", ollama.ModelConfig{
    Temperature:   &temp,
    ContextWindow: &ctx,
})
```

### Get Model Configuration
```go
modelCfg := cfg.GetModelConfig("llama2")
if modelCfg != nil {
    // Use modelCfg
}
```

### Use Timeouts
```go
connTimeout := cfg.GetConnectionTimeout()
reqTimeout := cfg.GetRequestTimeout()
```

### Implement Retry Logic
```go
for attempt := 0; attempt <= cfg.RetryPolicy.MaxRetries; attempt++ {
    // Try operation
    if err == nil {
        break
    }
    time.Sleep(cfg.RetryPolicy.GetBackoff())
}
```

### Validate Configuration
```go
if err := cfg.Validate(); err != nil {
    log.Fatal(err)
}
```

### Save Configuration
```go
err := ollama.SaveOllamaConfig(cfg)
```

### Load From Custom File
```go
cfg, err := ollama.LoadOllamaConfigFromFile("/path/to/config.json")
```

### Merge Configurations
```go
cfg1 := ollama.DefaultOllamaConfig()
cfg2, _ := ollama.LoadOllamaConfigFromFile("custom.json")
merged := cfg1.Merge(cfg2)
```

## Environment Variables

Override any configuration via environment variables:

```bash
# Enable/disable
export OLLAMA_ENABLED=true

# Single endpoint
export OLLAMA_ENDPOINT="http://remote-host:11434"

# Multiple endpoints (comma-separated)
export OLLAMA_ENDPOINT="http://host1:11434,http://host2:11434"

# Default endpoint
export OLLAMA_DEFAULT_ENDPOINT="host1"

# Timeouts (milliseconds)
export OLLAMA_CONNECTION_TIMEOUT_MS=15000
export OLLAMA_REQUEST_TIMEOUT_MS=120000

# Retry policy
export OLLAMA_MAX_RETRIES=5
export OLLAMA_RETRY_BACKOFF_MS=1000
```

## Type Reference

### OllamaConfig
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
    Name     string
    URL      string
    Enabled  bool
    Priority int
}
```

### ModelConfig
```go
type ModelConfig struct {
    Temperature    *float32
    ContextWindow  *int
    TopP           *float32
    TopK           *int
    RepeatPenalty  *float32
    NumPredict     *int
    Stop           []string
    System         *string
}
```

### RetryPolicy
```go
type RetryPolicy struct {
    MaxRetries int
    BackoffMS  int
}
```

## Default Values

| Setting | Default |
|---------|---------|
| Endpoint | http://localhost:11434 |
| Connection Timeout | 10 seconds (10000ms) |
| Request Timeout | 60 seconds (60000ms) |
| Max Retries | 3 |
| Retry Backoff | 500ms |

## Validations

Configuration must have:
- ✓ At least one endpoint
- ✓ At least one endpoint enabled
- ✓ All endpoints with valid URLs
- ✓ Positive timeouts
- ✓ Non-negative max retries

## Main Functions

| Function | Purpose |
|----------|---------|
| `LoadOllamaConfig()` | Load from default location |
| `LoadOllamaConfigFromFile(path)` | Load from custom file |
| `SaveOllamaConfig(cfg)` | Persist configuration |
| `DefaultOllamaConfig()` | Get defaults |
| `GetOllamaConfigDir()` | Get config directory |

## Methods on OllamaConfig

| Method | Returns |
|--------|---------|
| `GetConnectionTimeout()` | time.Duration |
| `GetRequestTimeout()` | time.Duration |
| `GetDefaultEndpoint()` | *OllamaEndpoint |
| `GetEnabledEndpoints()` | []OllamaEndpoint |
| `GetModelConfig(name)` | *ModelConfig |
| `SetModelConfig(name, cfg)` | - |
| `Validate()` | error |
| `Merge(other)` | *OllamaConfig |

## Examples

### Basic Usage
```go
package main

import (
    "claude-squad/ollama"
    "log"
)

func main() {
    cfg := ollama.LoadOllamaConfig()
    ep := cfg.GetDefaultEndpoint()
    log.Printf("Using: %s", ep.URL)
}
```

### With Error Handling
```go
cfg := ollama.LoadOllamaConfig()
if err := cfg.Validate(); err != nil {
    log.Fatalf("Invalid config: %v", err)
}

endpoints := cfg.GetEnabledEndpoints()
if len(endpoints) == 0 {
    log.Fatal("No endpoints available")
}
```

### With Model Configuration
```go
cfg := ollama.LoadOllamaConfig()
model := cfg.GetModelConfig("llama2")

// Use defaults if not configured
temp := float32(0.7)
if model == nil || model.Temperature == nil {
    model = &ollama.ModelConfig{Temperature: &temp}
}
```

## Files

- `/home/user/claude-squad/ollama/config.go` - Main implementation
- `/home/user/claude-squad/ollama/config_test.go` - Tests
- `/home/user/claude-squad/ollama/README.md` - Full documentation
- `/home/user/claude-squad/ollama/examples/ollama-config.example.json` - JSON example
- `/home/user/claude-squad/ollama/examples/ollama-config.example.yaml` - YAML example
- `/home/user/claude-squad/ollama/examples/usage_examples.go` - Code examples

## Run Tests

```bash
cd /home/user/claude-squad
go test ./ollama -v
```

## Integration with Main Config

```go
import (
    "claude-squad/config"
    "claude-squad/ollama"
)

func main() {
    mainCfg := config.LoadConfig()
    ollamaCfg := ollama.LoadOllamaConfig()

    if ollamaCfg.Enabled {
        // Use Ollama
    }
}
```

## Troubleshooting

### Configuration not loading
- Check file location: `~/.claude-squad/ollama/ollama.json`
- Check file permissions (must be readable)
- Check JSON/YAML syntax
- Check environment variable overrides

### Endpoint not found
- Verify endpoint is enabled: `endpoint.Enabled = true`
- Check default endpoint name matches
- Call `GetDefaultEndpoint()` returns nil if none enabled

### Timeout issues
- Increase `ConnectionTimeoutMS` for slow networks
- Increase `RequestTimeoutMS` for long-running operations
- Use environment variables to override without changing file

### Model config not applied
- Verify model name matches exactly
- Use `GetModelConfig()` to debug
- Check for nil pointers in optional fields

---

For complete documentation, see `README.md`
For examples, see `examples/` directory
For implementation details, see `IMPLEMENTATION_SUMMARY.md`
