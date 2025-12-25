package ollama

import (
	"claude-squad/log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain runs before all tests to set up the test environment
func TestMain(m *testing.M) {
	// Initialize the logger before any tests run
	log.Initialize(false)
	defer log.Close()

	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestDefaultOllamaConfig(t *testing.T) {
	t.Run("creates config with sensible defaults", func(t *testing.T) {
		cfg := DefaultOllamaConfig()

		assert.NotNil(t, cfg)
		assert.True(t, cfg.Enabled)
		assert.Equal(t, 1, len(cfg.Endpoints))
		assert.Equal(t, DefaultOllamaEndpoint, cfg.Endpoints[0].URL)
		assert.Equal(t, "local", cfg.Endpoints[0].Name)
		assert.True(t, cfg.Endpoints[0].Enabled)
		assert.Equal(t, "local", cfg.DefaultEndpointName)
		assert.Equal(t, int(DefaultConnectionTimeout.Milliseconds()), cfg.ConnectionTimeoutMS)
		assert.Equal(t, int(DefaultRequestTimeout.Milliseconds()), cfg.RequestTimeoutMS)
		assert.Equal(t, DefaultMaxRetries, cfg.RetryPolicy.MaxRetries)
		assert.NotNil(t, cfg.Models)
	})
}

func TestValidateAndApplyDefaults(t *testing.T) {
	t.Run("applies defaults to empty config", func(t *testing.T) {
		cfg := &OllamaConfig{}
		result := validateAndApplyDefaults(cfg)

		assert.Equal(t, int(DefaultConnectionTimeout.Milliseconds()), result.ConnectionTimeoutMS)
		assert.Equal(t, int(DefaultRequestTimeout.Milliseconds()), result.RequestTimeoutMS)
		assert.Equal(t, DefaultMaxRetries, result.RetryPolicy.MaxRetries)
		assert.Equal(t, 1, len(result.Endpoints))
	})

	t.Run("validates connection timeout", func(t *testing.T) {
		cfg := &OllamaConfig{ConnectionTimeoutMS: -1}
		result := validateAndApplyDefaults(cfg)

		assert.Equal(t, int(DefaultConnectionTimeout.Milliseconds()), result.ConnectionTimeoutMS)
	})

	t.Run("validates request timeout", func(t *testing.T) {
		cfg := &OllamaConfig{RequestTimeoutMS: 0}
		result := validateAndApplyDefaults(cfg)

		assert.Equal(t, int(DefaultRequestTimeout.Milliseconds()), result.RequestTimeoutMS)
	})

	t.Run("removes trailing slashes from URLs", func(t *testing.T) {
		cfg := &OllamaConfig{
			Endpoints: []OllamaEndpoint{
				{Name: "test", URL: "http://localhost:11434/", Enabled: true},
			},
		}
		result := validateAndApplyDefaults(cfg)

		assert.Equal(t, "http://localhost:11434", result.Endpoints[0].URL)
	})

	t.Run("initializes models map", func(t *testing.T) {
		cfg := &OllamaConfig{}
		result := validateAndApplyDefaults(cfg)

		assert.NotNil(t, result.Models)
	})
}

func TestApplyEnvironmentOverrides(t *testing.T) {
	originalEnv := make(map[string]string)
	envVars := []string{
		"OLLAMA_ENABLED",
		"OLLAMA_ENDPOINT",
		"OLLAMA_DEFAULT_ENDPOINT",
		"OLLAMA_CONNECTION_TIMEOUT_MS",
		"OLLAMA_REQUEST_TIMEOUT_MS",
		"OLLAMA_MAX_RETRIES",
		"OLLAMA_RETRY_BACKOFF_MS",
	}

	// Save original environment
	for _, v := range envVars {
		originalEnv[v] = os.Getenv(v)
	}

	defer func() {
		// Restore original environment
		for k, v := range originalEnv {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	t.Run("applies OLLAMA_ENABLED override", func(t *testing.T) {
		os.Setenv("OLLAMA_ENABLED", "false")
		cfg := DefaultOllamaConfig()
		result := applyEnvironmentOverrides(cfg)

		assert.False(t, result.Enabled)
	})

	t.Run("applies OLLAMA_ENDPOINT override", func(t *testing.T) {
		os.Setenv("OLLAMA_ENDPOINT", "http://remote-host:11434")
		cfg := DefaultOllamaConfig()
		result := applyEnvironmentOverrides(cfg)

		assert.Equal(t, 1, len(result.Endpoints))
		assert.Equal(t, "http://remote-host:11434", result.Endpoints[0].URL)
	})

	t.Run("applies multiple endpoints from OLLAMA_ENDPOINT", func(t *testing.T) {
		os.Setenv("OLLAMA_ENDPOINT", "http://host1:11434,http://host2:11434")
		cfg := DefaultOllamaConfig()
		result := applyEnvironmentOverrides(cfg)

		assert.Equal(t, 2, len(result.Endpoints))
		assert.Equal(t, "http://host1:11434", result.Endpoints[0].URL)
		assert.Equal(t, "http://host2:11434", result.Endpoints[1].URL)
	})

	t.Run("applies OLLAMA_CONNECTION_TIMEOUT_MS override", func(t *testing.T) {
		os.Setenv("OLLAMA_CONNECTION_TIMEOUT_MS", "5000")
		cfg := DefaultOllamaConfig()
		result := applyEnvironmentOverrides(cfg)

		assert.Equal(t, 5000, result.ConnectionTimeoutMS)
	})

	t.Run("applies OLLAMA_REQUEST_TIMEOUT_MS override", func(t *testing.T) {
		os.Setenv("OLLAMA_REQUEST_TIMEOUT_MS", "120000")
		cfg := DefaultOllamaConfig()
		result := applyEnvironmentOverrides(cfg)

		assert.Equal(t, 120000, result.RequestTimeoutMS)
	})

	t.Run("applies OLLAMA_MAX_RETRIES override", func(t *testing.T) {
		os.Setenv("OLLAMA_MAX_RETRIES", "5")
		cfg := DefaultOllamaConfig()
		result := applyEnvironmentOverrides(cfg)

		assert.Equal(t, 5, result.RetryPolicy.MaxRetries)
	})
}

func TestGetConnectionTimeout(t *testing.T) {
	t.Run("returns timeout as duration", func(t *testing.T) {
		cfg := &OllamaConfig{ConnectionTimeoutMS: 5000}
		timeout := cfg.GetConnectionTimeout()

		assert.Equal(t, 5*time.Second, timeout)
	})
}

func TestGetRequestTimeout(t *testing.T) {
	t.Run("returns timeout as duration", func(t *testing.T) {
		cfg := &OllamaConfig{RequestTimeoutMS: 60000}
		timeout := cfg.GetRequestTimeout()

		assert.Equal(t, 60*time.Second, timeout)
	})
}

func TestGetDefaultEndpoint(t *testing.T) {
	t.Run("returns default endpoint by name", func(t *testing.T) {
		cfg := &OllamaConfig{
			Endpoints: []OllamaEndpoint{
				{Name: "primary", URL: "http://primary:11434", Enabled: true},
				{Name: "backup", URL: "http://backup:11434", Enabled: true},
			},
			DefaultEndpointName: "backup",
		}

		ep := cfg.GetDefaultEndpoint()
		assert.NotNil(t, ep)
		assert.Equal(t, "backup", ep.Name)
	})

	t.Run("falls back to first enabled endpoint", func(t *testing.T) {
		cfg := &OllamaConfig{
			Endpoints: []OllamaEndpoint{
				{Name: "primary", URL: "http://primary:11434", Enabled: true},
				{Name: "backup", URL: "http://backup:11434", Enabled: true},
			},
			DefaultEndpointName: "nonexistent",
		}

		ep := cfg.GetDefaultEndpoint()
		assert.NotNil(t, ep)
		assert.Equal(t, "primary", ep.Name)
	})

	t.Run("ignores disabled default endpoint", func(t *testing.T) {
		cfg := &OllamaConfig{
			Endpoints: []OllamaEndpoint{
				{Name: "primary", URL: "http://primary:11434", Enabled: false},
				{Name: "backup", URL: "http://backup:11434", Enabled: true},
			},
			DefaultEndpointName: "primary",
		}

		ep := cfg.GetDefaultEndpoint()
		assert.NotNil(t, ep)
		assert.Equal(t, "backup", ep.Name)
	})
}

func TestGetEnabledEndpoints(t *testing.T) {
	t.Run("returns only enabled endpoints", func(t *testing.T) {
		cfg := &OllamaConfig{
			Endpoints: []OllamaEndpoint{
				{Name: "primary", URL: "http://primary:11434", Enabled: true, Priority: 0},
				{Name: "backup", URL: "http://backup:11434", Enabled: false},
				{Name: "tertiary", URL: "http://tertiary:11434", Enabled: true, Priority: 1},
			},
		}

		enabled := cfg.GetEnabledEndpoints()
		assert.Equal(t, 2, len(enabled))
		assert.Equal(t, "primary", enabled[0].Name)
		assert.Equal(t, "tertiary", enabled[1].Name)
	})

	t.Run("sorts by priority", func(t *testing.T) {
		cfg := &OllamaConfig{
			Endpoints: []OllamaEndpoint{
				{Name: "primary", URL: "http://primary:11434", Enabled: true, Priority: 1},
				{Name: "backup", URL: "http://backup:11434", Enabled: true, Priority: 0},
			},
		}

		enabled := cfg.GetEnabledEndpoints()
		assert.Equal(t, "backup", enabled[0].Name)
		assert.Equal(t, "primary", enabled[1].Name)
	})
}

func TestGetModelConfig(t *testing.T) {
	t.Run("returns model config if exists", func(t *testing.T) {
		temp := float32(0.7)
		cfg := &OllamaConfig{
			Models: map[string]ModelConfig{
				"llama2": {Temperature: &temp},
			},
		}

		modelCfg := cfg.GetModelConfig("llama2")
		assert.NotNil(t, modelCfg)
		assert.Equal(t, &temp, modelCfg.Temperature)
	})

	t.Run("returns nil if model doesn't exist", func(t *testing.T) {
		cfg := &OllamaConfig{Models: make(map[string]ModelConfig)}

		modelCfg := cfg.GetModelConfig("nonexistent")
		assert.Nil(t, modelCfg)
	})
}

func TestSetModelConfig(t *testing.T) {
	t.Run("sets model config", func(t *testing.T) {
		cfg := &OllamaConfig{Models: make(map[string]ModelConfig)}
		temp := float32(0.8)
		modelCfg := ModelConfig{Temperature: &temp}

		cfg.SetModelConfig("llama2", modelCfg)

		assert.NotNil(t, cfg.GetModelConfig("llama2"))
		assert.Equal(t, &temp, cfg.GetModelConfig("llama2").Temperature)
	})
}

func TestValidate(t *testing.T) {
	t.Run("valid config passes validation", func(t *testing.T) {
		cfg := DefaultOllamaConfig()
		err := cfg.Validate()

		assert.NoError(t, err)
	})

	t.Run("requires at least one endpoint", func(t *testing.T) {
		cfg := &OllamaConfig{Endpoints: []OllamaEndpoint{}}
		err := cfg.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one endpoint")
	})

	t.Run("requires at least one enabled endpoint", func(t *testing.T) {
		cfg := &OllamaConfig{
			Endpoints: []OllamaEndpoint{
				{Name: "test", URL: "http://localhost:11434", Enabled: false},
			},
		}
		err := cfg.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "enabled")
	})

	t.Run("rejects endpoints with empty URLs", func(t *testing.T) {
		cfg := &OllamaConfig{
			Endpoints: []OllamaEndpoint{
				{Name: "test", URL: "", Enabled: true},
			},
		}
		err := cfg.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty URL")
	})

	t.Run("rejects negative max retries", func(t *testing.T) {
		cfg := DefaultOllamaConfig()
		cfg.RetryPolicy.MaxRetries = -1
		err := cfg.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max_retries")
	})

	t.Run("rejects non-positive timeouts", func(t *testing.T) {
		cfg := DefaultOllamaConfig()
		cfg.ConnectionTimeoutMS = 0
		err := cfg.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection_timeout_ms")
	})
}

func TestMerge(t *testing.T) {
	t.Run("merges configurations", func(t *testing.T) {
		cfg1 := DefaultOllamaConfig()
		cfg1.Enabled = true

		cfg2 := &OllamaConfig{
			Enabled:             false,
			ConnectionTimeoutMS: 5000,
		}

		result := cfg1.Merge(cfg2)

		assert.False(t, result.Enabled)
		assert.Equal(t, 5000, result.ConnectionTimeoutMS)
	})

	t.Run("handles nil merge", func(t *testing.T) {
		cfg := DefaultOllamaConfig()
		result := cfg.Merge(nil)

		assert.Equal(t, cfg, result)
	})
}

func TestLoadOllamaConfig(t *testing.T) {
	t.Run("returns default config when file doesn't exist", func(t *testing.T) {
		// Use temporary HOME to avoid interfering with real config
		originalHome := os.Getenv("HOME")
		tempHome := t.TempDir()
		os.Setenv("HOME", tempHome)
		defer os.Setenv("HOME", originalHome)

		cfg := LoadOllamaConfig()

		assert.NotNil(t, cfg)
		assert.True(t, cfg.Enabled)
		assert.Equal(t, 1, len(cfg.Endpoints))
	})

	t.Run("loads valid JSON config file", func(t *testing.T) {
		tempHome := t.TempDir()
		configDir := filepath.Join(tempHome, ".claude-squad", "ollama")
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		configPath := filepath.Join(configDir, OllamaConfigFileName)
		configContent := `{
			"enabled": true,
			"endpoints": [
				{
					"name": "test",
					"url": "http://test:11434",
					"enabled": true,
					"priority": 0
				}
			],
			"default_endpoint": "test",
			"connection_timeout_ms": 5000,
			"request_timeout_ms": 60000,
			"retry_policy": {
				"max_retries": 5,
				"backoff_ms": 1000
			},
			"models": {}
		}`
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempHome)
		defer os.Setenv("HOME", originalHome)

		cfg := LoadOllamaConfig()

		assert.NotNil(t, cfg)
		assert.True(t, cfg.Enabled)
		assert.Equal(t, 1, len(cfg.Endpoints))
		assert.Equal(t, "test", cfg.Endpoints[0].Name)
		assert.Equal(t, 5, cfg.RetryPolicy.MaxRetries)
	})
}

func TestLoadOllamaConfigFromFile(t *testing.T) {
	t.Run("loads JSON config from file", func(t *testing.T) {
		tempFile := t.TempDir()
		configPath := filepath.Join(tempFile, "config.json")

		configContent := `{
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
		}`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		cfg, err := LoadOllamaConfigFromFile(configPath)

		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.True(t, cfg.Enabled)
		assert.Equal(t, "local", cfg.Endpoints[0].Name)
	})

	t.Run("loads YAML config from file", func(t *testing.T) {
		tempFile := t.TempDir()
		configPath := filepath.Join(tempFile, "config.yaml")

		configContent := `enabled: true
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
models: {}`

		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		cfg, err := LoadOllamaConfigFromFile(configPath)

		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.True(t, cfg.Enabled)
	})

	t.Run("handles missing file", func(t *testing.T) {
		cfg, err := LoadOllamaConfigFromFile("/nonexistent/path/config.json")

		assert.Error(t, err)
		assert.Nil(t, cfg)
	})
}

func TestSaveOllamaConfig(t *testing.T) {
	t.Run("saves config to file", func(t *testing.T) {
		originalHome := os.Getenv("HOME")
		tempHome := t.TempDir()
		os.Setenv("HOME", tempHome)
		defer os.Setenv("HOME", originalHome)

		testCfg := DefaultOllamaConfig()
		testCfg.Enabled = false
		testCfg.ConnectionTimeoutMS = 5000

		err := SaveOllamaConfig(testCfg)
		assert.NoError(t, err)

		// Verify the file was created
		configDir := filepath.Join(tempHome, ".claude-squad", "ollama")
		configPath := filepath.Join(configDir, OllamaConfigFileName)

		assert.FileExists(t, configPath)

		// Load and verify content
		loadedCfg := LoadOllamaConfig()
		assert.Equal(t, false, loadedCfg.Enabled)
		assert.Equal(t, 5000, loadedCfg.ConnectionTimeoutMS)
	})
}
