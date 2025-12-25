# Ollama Configuration System - Complete Index

## Quick Start

1. **Load configuration:**
   ```go
   cfg := ollama.LoadOllamaConfig()
   ```

2. **Get an endpoint:**
   ```go
   endpoint := cfg.GetDefaultEndpoint()
   ```

3. **Configure a model:**
   ```go
   cfg.SetModelConfig("llama2", ollama.ModelConfig{...})
   ```

## File Structure

```
/home/user/claude-squad/ollama/
├── config.go                              (524 lines)
├── config_test.go                         (530 lines)
├── README.md                              (339 lines)
├── QUICK_REFERENCE.md                     (Quick lookup guide)
├── IMPLEMENTATION_SUMMARY.md              (Detailed overview)
├── INDEX.md                               (This file)
└── examples/
    ├── ollama-config.example.json         (55 lines)
    ├── ollama-config.example.yaml         (110 lines)
    └── usage_examples.go                  (328 lines)
```

## Documentation by Use Case

### I want to...

#### Understand the basics
- Start here: **README.md** - Complete overview with examples
- Then read: **QUICK_REFERENCE.md** - Quick lookup of common tasks

#### Get up and running
1. Copy example config: `cp examples/ollama-config.example.json ~/.claude-squad/ollama/ollama.json`
2. Read: **examples/usage_examples.go** - Code examples
3. Load in your code: `cfg := ollama.LoadOllamaConfig()`

#### Configure Ollama
- Edit: `~/.claude-squad/ollama/ollama.json`
- Or use: Environment variables starting with `OLLAMA_`
- See: **QUICK_REFERENCE.md** for all env vars

#### Add models
- Use: `cfg.SetModelConfig("model-name", ollama.ModelConfig{...})`
- See: **examples/ollama-config.example.json** for model examples

#### Set up multiple endpoints with failover
- See: **examples/ollama-config.example.yaml** - Production example
- Use priority field to control order (0 = highest priority)

#### Use environment variables
- Reference: **QUICK_REFERENCE.md** - All 7 env variables
- Example: `export OLLAMA_ENDPOINT="http://remote-host:11434"`

#### Integrate with main app config
- See: **README.md** - Integration section
- Code: Import both `config` and `ollama` packages

#### Find implementation details
- Read: **IMPLEMENTATION_SUMMARY.md** - Full technical details
- Code: **config.go** - Source code (well-commented)

#### Debug issues
- Check: **QUICK_REFERENCE.md** - Troubleshooting section
- Verify: Configuration validation with `cfg.Validate()`
- Read: Log messages in /tmp/claudesquad.log

## Core Concepts

### Endpoints
- Represent Ollama servers you can connect to
- Each has: name, URL, enabled flag, priority
- Priority: 0 = tried first, 1 = second, etc.
- Selection: Default endpoint or first enabled

### Models
- Model-specific parameters stored as map
- Fields are optional (use pointers)
- Include: temperature, context, sampling, prompts
- Access: `GetModelConfig("name")` or `SetModelConfig("name", config)`

### Configuration Loading
1. Start with built-in defaults
2. Override with file config (`~/.claude-squad/ollama/ollama.json`)
3. Override with environment variables

### Timeouts & Retries
- Connection timeout: How long to establish connection
- Request timeout: How long to wait for response
- Max retries: How many times to retry on failure
- Backoff: Wait time between retries

## API Reference

### Main Functions

| Function | Purpose | Returns |
|----------|---------|---------|
| `LoadOllamaConfig()` | Load from default location | *OllamaConfig |
| `LoadOllamaConfigFromFile(path)` | Load from custom file | *OllamaConfig, error |
| `SaveOllamaConfig(cfg)` | Save to default location | error |
| `DefaultOllamaConfig()` | Get defaults | *OllamaConfig |
| `GetOllamaConfigDir()` | Get config directory | string, error |

### Methods on OllamaConfig

| Method | Purpose | Returns |
|--------|---------|---------|
| `GetConnectionTimeout()` | Get conn timeout | time.Duration |
| `GetRequestTimeout()` | Get request timeout | time.Duration |
| `GetDefaultEndpoint()` | Get default endpoint | *OllamaEndpoint |
| `GetEnabledEndpoints()` | Get all enabled endpoints | []OllamaEndpoint |
| `GetModelConfig(name)` | Get model config | *ModelConfig |
| `SetModelConfig(name, cfg)` | Set model config | - |
| `Validate()` | Validate configuration | error |
| `Merge(other)` | Merge two configs | *OllamaConfig |

### Methods on RetryPolicy

| Method | Purpose | Returns |
|--------|---------|---------|
| `GetBackoff()` | Get backoff duration | time.Duration |

## Common Patterns

### Load and use default endpoint
```go
cfg := ollama.LoadOllamaConfig()
ep := cfg.GetDefaultEndpoint()
// Use ep.URL
```

### Implement endpoint failover
```go
cfg := ollama.LoadOllamaConfig()
for _, ep := range cfg.GetEnabledEndpoints() {
    // Try ep.URL
    // If success, break
    // If fail, continue to next
}
```

### Configure a model
```go
cfg := ollama.LoadOllamaConfig()
temp := float32(0.7)
cfg.SetModelConfig("llama2", ollama.ModelConfig{
    Temperature: &temp,
})
```

### Implement retry logic
```go
cfg := ollama.LoadOllamaConfig()
for attempt := 0; attempt <= cfg.RetryPolicy.MaxRetries; attempt++ {
    // Try operation
    if err == nil {
        break
    }
    time.Sleep(cfg.RetryPolicy.GetBackoff())
}
```

### Use with main config
```go
mainCfg := config.LoadConfig()
ollamaCfg := ollama.LoadOllamaConfig()

if ollamaCfg.Enabled {
    ep := ollamaCfg.GetDefaultEndpoint()
}
```

## Default Values

| Setting | Default |
|---------|---------|
| Endpoint | http://localhost:11434 |
| Connection Timeout | 10 seconds |
| Request Timeout | 60 seconds |
| Max Retries | 3 |
| Retry Backoff | 500ms |
| Config File | ~/.claude-squad/ollama/ollama.json |

## Environment Variables (7 total)

```
OLLAMA_ENABLED                    # true/false to enable/disable
OLLAMA_ENDPOINT                   # Single or comma-separated URLs
OLLAMA_DEFAULT_ENDPOINT           # Default endpoint name
OLLAMA_CONNECTION_TIMEOUT_MS      # Connection timeout in ms
OLLAMA_REQUEST_TIMEOUT_MS         # Request timeout in ms
OLLAMA_MAX_RETRIES                # Maximum retry attempts
OLLAMA_RETRY_BACKOFF_MS           # Backoff in ms
```

## Configuration File Path

```
~/.claude-squad/ollama/ollama.json  # Default location
```

Supports both JSON and YAML formats. File is created automatically on first run.

## Structs

### OllamaConfig
Main configuration container with all settings.

### OllamaEndpoint
Single Ollama server endpoint definition.

### ModelConfig
Model-specific parameters (optional fields).

### RetryPolicy
Retry behavior configuration.

## Testing

Run all tests:
```bash
cd /home/user/claude-squad
go test ./ollama -v
```

Tests include:
- Configuration loading and saving
- Environment variable overrides
- Validation logic
- Endpoint selection
- Model configuration
- File format handling (JSON/YAML)
- Configuration merging

## Statistics

| Metric | Value |
|--------|-------|
| Implementation Lines | 524 |
| Test Lines | 530 |
| Documentation Lines | 339 |
| Example Lines | 493 |
| Total Lines | 1,886 |
| Test Cases | 20+ |
| Core Structs | 4 |
| Public Functions | 15+ |
| Helper Methods | 10+ |

## Features

✓ Multiple Ollama endpoints
✓ Priority-based failover
✓ Model-specific configuration
✓ JSON/YAML format support
✓ Environment variable overrides
✓ Sensible defaults
✓ Comprehensive validation
✓ Configuration persistence
✓ Timeout management
✓ Retry policies
✓ Configuration merging
✓ Extensive test coverage
✓ Complete documentation

## Next Steps

1. **To get started:**
   - Copy example config to `~/.claude-squad/ollama/ollama.json`
   - Load with `ollama.LoadOllamaConfig()`

2. **For details:**
   - Read `README.md` for complete reference
   - Check `QUICK_REFERENCE.md` for quick lookup
   - See `examples/` for code samples

3. **For implementation:**
   - Review `IMPLEMENTATION_SUMMARY.md`
   - Read `config.go` source code
   - Check `config_test.go` for test patterns

4. **To customize:**
   - Edit `~/.claude-squad/ollama/ollama.json`
   - Set environment variables
   - Or use API: `cfg.SetModelConfig()`, etc.

## Support

- Configuration validation: Use `cfg.Validate()`
- Error messages: Check `/tmp/claudesquad.log`
- Format issues: See example files
- API questions: Read function comments in `config.go`

## Integration Example

```go
import (
    "claude-squad/config"
    "claude-squad/ollama"
    "log"
)

func main() {
    // Load both configurations
    mainCfg := config.LoadConfig()
    ollamaCfg := ollama.LoadOllamaConfig()

    // Validate
    if err := ollamaCfg.Validate(); err != nil {
        log.Fatal(err)
    }

    // Use Ollama
    if ollamaCfg.Enabled {
        ep := ollamaCfg.GetDefaultEndpoint()
        log.Printf("Using Ollama at %s", ep.URL)

        // Configure model
        modelCfg := ollamaCfg.GetModelConfig("llama2")
        if modelCfg == nil {
            temp := float32(0.7)
            ollamaCfg.SetModelConfig("llama2", ollama.ModelConfig{
                Temperature: &temp,
            })
        }
    }
}
```

---

**Last Updated:** 2024-12-25

**Total Implementation:** 1,886 lines of production-ready code and documentation

**Ready for:** Integration and extension
