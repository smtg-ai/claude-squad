package examples

import (
	"claude-squad/ollama"
	"fmt"
	"log"
	"time"
)

// ExampleBasicLoading demonstrates loading the default configuration
func ExampleBasicLoading() {
	// Load configuration from file or use defaults
	cfg := ollama.LoadOllamaConfig()

	fmt.Printf("Ollama enabled: %v\n", cfg.Enabled)
	fmt.Printf("Number of endpoints: %d\n", len(cfg.Endpoints))
	fmt.Printf("Default endpoint: %s\n", cfg.DefaultEndpointName)
}

// ExampleEndpointSelection demonstrates selecting and using endpoints
func ExampleEndpointSelection() {
	cfg := ollama.LoadOllamaConfig()

	// Get the default endpoint
	defaultEp := cfg.GetDefaultEndpoint()
	if defaultEp != nil {
		fmt.Printf("Default endpoint: %s (%s)\n", defaultEp.Name, defaultEp.URL)
	}

	// Get all enabled endpoints (in priority order)
	endpoints := cfg.GetEnabledEndpoints()
	fmt.Printf("Available endpoints (in priority order):\n")
	for i, ep := range endpoints {
		fmt.Printf("  %d. %s -> %s\n", i, ep.Name, ep.URL)
	}

	// Implement failover logic
	for _, ep := range endpoints {
		// Try to use this endpoint
		fmt.Printf("Attempting to connect to %s (%s)...\n", ep.Name, ep.URL)

		// In a real scenario, you would:
		// 1. Create HTTP client with timeout
		// 2. Make request to ep.URL
		// 3. If successful, use this endpoint
		// 4. If failed, continue to next endpoint

		// Example timeout usage
		_ = cfg.GetConnectionTimeout() // 10 seconds by default
		_ = cfg.GetRequestTimeout()     // 60 seconds by default
	}
}

// ExampleModelConfiguration demonstrates model-specific settings
func ExampleModelConfiguration() {
	cfg := ollama.LoadOllamaConfig()

	// Define model configurations
	models := map[string]ollama.ModelConfig{
		"llama2": {
			Temperature:   toFloat32Ptr(0.7),
			ContextWindow: toIntPtr(4096),
			TopP:          toFloat32Ptr(0.9),
			TopK:          toIntPtr(40),
			NumPredict:    toIntPtr(512),
		},
		"mistral": {
			Temperature:   toFloat32Ptr(0.8),
			ContextWindow: toIntPtr(8192),
			TopP:          toFloat32Ptr(0.95),
			NumPredict:    toIntPtr(1024),
		},
		"neural-chat": {
			Temperature:   toFloat32Ptr(0.6),
			ContextWindow: toIntPtr(4096),
		},
	}

	// Set model configurations
	for modelName, modelCfg := range models {
		cfg.SetModelConfig(modelName, modelCfg)
	}

	// Retrieve and use model configuration
	modelName := "llama2"
	if modelCfg := cfg.GetModelConfig(modelName); modelCfg != nil {
		if modelCfg.Temperature != nil {
			fmt.Printf("Model %s temperature: %.1f\n", modelName, *modelCfg.Temperature)
		}
		if modelCfg.ContextWindow != nil {
			fmt.Printf("Model %s context: %d tokens\n", modelName, *modelCfg.ContextWindow)
		}
	}
}

// ExampleEnvironmentOverrides demonstrates environment variable overrides
func ExampleEnvironmentOverrides() {
	// Set environment variables
	// export OLLAMA_ENDPOINT="http://remote-host:11434"
	// export OLLAMA_CONNECTION_TIMEOUT_MS="15000"
	// export OLLAMA_MAX_RETRIES="5"

	cfg := ollama.LoadOllamaConfig()

	// These values will be overridden by environment variables if set
	fmt.Printf("Connection timeout: %v\n", cfg.GetConnectionTimeout())
	fmt.Printf("Max retries: %d\n", cfg.RetryPolicy.MaxRetries)

	// Endpoints from environment variables
	for _, ep := range cfg.Endpoints {
		fmt.Printf("Endpoint: %s (%s)\n", ep.Name, ep.URL)
	}
}

// ExampleRetryLogic demonstrates using retry configuration
func ExampleRetryLogic() {
	cfg := ollama.LoadOllamaConfig()

	fmt.Printf("Retry policy:\n")
	fmt.Printf("  Max retries: %d\n", cfg.RetryPolicy.MaxRetries)
	fmt.Printf("  Backoff: %v\n", cfg.RetryPolicy.GetBackoff())

	// Example retry loop
	backoff := cfg.RetryPolicy.GetBackoff()
	for attempt := 0; attempt <= cfg.RetryPolicy.MaxRetries; attempt++ {
		fmt.Printf("Attempt %d...\n", attempt+1)

		// Try operation here
		if shouldFail := attempt < 2; shouldFail {
			fmt.Printf("  Failed, retrying in %v\n", backoff)
			time.Sleep(backoff)
			// Increase backoff for next attempt (exponential backoff)
			backoff = time.Duration(float64(backoff) * 1.5)
		} else {
			fmt.Println("  Success!")
			break
		}
	}
}

// ExampleMultipleEndpointSetup demonstrates configuring multiple endpoints
func ExampleMultipleEndpointSetup() {
	// Create a custom configuration with multiple endpoints
	cfg := &ollama.OllamaConfig{
		Endpoints: []ollama.OllamaEndpoint{
			{
				Name:     "local",
				URL:      "http://localhost:11434",
				Enabled:  true,
				Priority: 0,
			},
			{
				Name:     "remote-gpu",
				URL:      "http://gpu-server.internal:11434",
				Enabled:  true,
				Priority: 1,
			},
			{
				Name:     "backup",
				URL:      "http://backup-ollama:11434",
				Enabled:  false,
				Priority: 2,
			},
		},
		DefaultEndpointName: "local",
		ConnectionTimeoutMS: 10000,
		RequestTimeoutMS:    60000,
		RetryPolicy: ollama.RetryPolicy{
			MaxRetries: 3,
			BackoffMS:  500,
		},
		Enabled: true,
		Models:  make(map[string]ollama.ModelConfig),
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Printf("Invalid configuration: %v", err)
		return
	}

	fmt.Println("Multi-endpoint configuration:")
	for _, ep := range cfg.GetEnabledEndpoints() {
		fmt.Printf("  %s (priority %d): %s\n", ep.Name, ep.Priority, ep.URL)
	}
}

// ExampleLoadFromCustomFile demonstrates loading from a custom config file
func ExampleLoadFromCustomFile() {
	configPath := "/etc/ollama/config.json"

	cfg, err := ollama.LoadOllamaConfigFromFile(configPath)
	if err != nil {
		log.Printf("Failed to load config from %s: %v", configPath, err)
		// Fall back to defaults
		cfg = ollama.DefaultOllamaConfig()
	}

	fmt.Printf("Loaded configuration from %s\n", configPath)
	fmt.Printf("Default endpoint: %s\n", cfg.DefaultEndpointName)
}

// ExampleMergingConfigurations demonstrates merging configurations
func ExampleMergingConfigurations() {
	// Start with default configuration
	baseCfg := ollama.DefaultOllamaConfig()

	// Load configuration from file
	fileCfg, err := ollama.LoadOllamaConfigFromFile("/path/to/config.json")
	if err == nil {
		// Merge file configuration into base
		baseCfg = baseCfg.Merge(fileCfg)
	}

	// Create environment-specific overrides
	envCfg := &ollama.OllamaConfig{
		ConnectionTimeoutMS: 20000,
		RequestTimeoutMS:    120000,
	}

	// Merge environment overrides
	finalCfg := baseCfg.Merge(envCfg)

	fmt.Printf("Final configuration after merging:\n")
	fmt.Printf("  Connection timeout: %dms\n", finalCfg.ConnectionTimeoutMS)
	fmt.Printf("  Request timeout: %dms\n", finalCfg.RequestTimeoutMS)
}

// ExampleSavingConfiguration demonstrates saving configuration
func ExampleSavingConfiguration() {
	// Load or create configuration
	cfg := ollama.LoadOllamaConfig()

	// Modify configuration
	cfg.Enabled = true
	cfg.ConnectionTimeoutMS = 15000

	// Add model configuration
	temp := float32(0.7)
	ctx := 4096
	cfg.SetModelConfig("custom-model", ollama.ModelConfig{
		Temperature:   &temp,
		ContextWindow: &ctx,
	})

	// Save to disk
	if err := ollama.SaveOllamaConfig(cfg); err != nil {
		log.Printf("Failed to save configuration: %v", err)
	} else {
		fmt.Println("Configuration saved successfully")
	}
}

// ExampleValidation demonstrates configuration validation
func ExampleValidation() {
	cfg := &ollama.OllamaConfig{
		Endpoints: []ollama.OllamaEndpoint{
			{Name: "test", URL: "http://localhost:11434", Enabled: true},
		},
		ConnectionTimeoutMS: 10000,
		RequestTimeoutMS:    60000,
		RetryPolicy: ollama.RetryPolicy{
			MaxRetries: 3,
			BackoffMS:  500,
		},
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Printf("Validation failed: %v", err)
	} else {
		fmt.Println("Configuration is valid")
	}
}

// Helper functions to create pointers
func toFloat32Ptr(v float32) *float32 {
	return &v
}

func toIntPtr(v int) *int {
	return &v
}

// ExampleCompleteIntegration shows a complete integration example
func ExampleCompleteIntegration() {
	// Load configuration
	cfg := ollama.LoadOllamaConfig()

	// Validate
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Get endpoint
	endpoint := cfg.GetDefaultEndpoint()
	if endpoint == nil {
		log.Fatalf("No available endpoint")
	}

	fmt.Printf("Using endpoint: %s (%s)\n", endpoint.Name, endpoint.URL)

	// Get model configuration
	const targetModel = "llama2"
	modelCfg := cfg.GetModelConfig(targetModel)

	if modelCfg == nil {
		fmt.Printf("No custom config for %s, using defaults\n", targetModel)
	} else {
		fmt.Printf("Model %s configuration:\n", targetModel)
		if modelCfg.Temperature != nil {
			fmt.Printf("  Temperature: %.1f\n", *modelCfg.Temperature)
		}
		if modelCfg.ContextWindow != nil {
			fmt.Printf("  Context: %d tokens\n", *modelCfg.ContextWindow)
		}
	}

	// Setup timeouts
	connTimeout := cfg.GetConnectionTimeout()
	reqTimeout := cfg.GetRequestTimeout()
	fmt.Printf("Timeouts: connection=%v, request=%v\n", connTimeout, reqTimeout)

	// Setup retry policy
	fmt.Printf("Retry policy: max=%d, backoff=%v\n",
		cfg.RetryPolicy.MaxRetries,
		cfg.RetryPolicy.GetBackoff())
}
