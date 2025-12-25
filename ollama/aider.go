package ollama

import (
	"claude-squad/log"
	"claude-squad/session"
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"
)

// AiderMode represents the different operational modes for Aider
type AiderMode string

const (
	// AskMode: ask mode is for asking questions about code
	AskMode AiderMode = "ask"
	// ArchitectMode: architect mode is for planning and designing solutions
	ArchitectMode AiderMode = "architect"
	// CodeMode: code mode is for writing and modifying code (default)
	CodeMode AiderMode = "code"
)

// ModelSelectionStrategy defines how to select among available Ollama models
type ModelSelectionStrategy string

const (
	// FastestModel: select the model with the lowest latency
	FastestModel ModelSelectionStrategy = "fastest"
	// MostCapable: select the model with the highest capability score
	MostCapable ModelSelectionStrategy = "most_capable"
	// RoundRobin: rotate through models in sequence
	RoundRobin ModelSelectionStrategy = "round_robin"
)

// SessionConfig represents configuration for an Aider session
type SessionConfig struct {
	// Mode is the operational mode for the session
	Mode AiderMode
	// Model is the Ollama model to use (e.g., "ollama_chat/gemma3:1b")
	Model string
	// AutoCommit enables automatic git commits
	AutoCommit bool
	// VerboseMode enables verbose output
	VerboseMode bool
	// MaxContextLines limits the context provided to the model
	MaxContextLines int
	// GitOnly restricts Aider to git-tracked files only
	GitOnly bool
	// ScanGlobs specifies file patterns to include
	ScanGlobs []string
	// IgnoreGlobs specifies file patterns to exclude
	IgnoreGlobs []string
	// Architecture is the target architecture mode (only for architect mode)
	Architecture string
}

// AiderIntegration provides Aider-specific integration with Ollama models
type AiderIntegration struct {
	// mu protects concurrent access to shared state
	mu sync.RWMutex
	// framework is the underlying Ollama framework
	framework *OllamaFramework
	// registry is the model registry
	registry *ModelRegistry
	// selectionStrategy determines how to select models
	selectionStrategy ModelSelectionStrategy
	// roundRobinIndex tracks the current position for round-robin selection
	roundRobinIndex int
	// modelCache stores model selection results for reuse
	modelCache map[string]string
	// lastUpdated tracks when the model list was last refreshed
	lastUpdated time.Time
	// modelMetrics stores performance metrics for models
	modelMetrics map[string]*AiderModelMetrics
}

// AiderModelMetrics stores Aider-specific performance characteristics for models
type AiderModelMetrics struct {
	// Name is the model identifier
	Name string
	// CapabilityScore is a relative score of the model's capabilities (1-100)
	CapabilityScore int
	// AverageLatencyMs is the average response time in milliseconds
	AverageLatencyMs int64
	// Tags are optional categorization tags
	Tags []string
}

// NewAiderIntegration creates a new AiderIntegration with the given framework
func NewAiderIntegration(framework *OllamaFramework) (*AiderIntegration, error) {
	if framework == nil {
		return nil, NewOllamaError("INVALID_REQUEST", "framework cannot be nil", nil)
	}

	return &AiderIntegration{
		framework:         framework,
		registry:          framework.registry,
		selectionStrategy: MostCapable,
		roundRobinIndex:   0,
		modelCache:        make(map[string]string),
		modelMetrics:      make(map[string]*AiderModelMetrics),
		lastUpdated:       time.Now(),
	}, nil
}

// RegisterModelMetrics adds performance metrics for a model
func (ai *AiderIntegration) RegisterModelMetrics(metrics AiderModelMetrics) error {
	if metrics.Name == "" {
		return NewOllamaError("INVALID_REQUEST", "model name cannot be empty", nil)
	}
	if metrics.CapabilityScore < 1 || metrics.CapabilityScore > 100 {
		return NewOllamaError("INVALID_REQUEST", "capability score must be between 1 and 100", nil)
	}

	ai.mu.Lock()
	defer ai.mu.Unlock()

	ai.modelMetrics[metrics.Name] = &metrics
	ai.lastUpdated = time.Now()

	log.InfoLog.Printf("Registered metrics for model: %s (capability: %d, latency: %dms)",
		metrics.Name, metrics.CapabilityScore, metrics.AverageLatencyMs)

	return nil
}

// SetSelectionStrategy sets the model selection strategy
func (ai *AiderIntegration) SetSelectionStrategy(strategy ModelSelectionStrategy) error {
	if strategy != FastestModel && strategy != MostCapable && strategy != RoundRobin {
		return fmt.Errorf("invalid selection strategy: %s", strategy)
	}

	ai.mu.Lock()
	defer ai.mu.Unlock()

	ai.selectionStrategy = strategy
	log.InfoLog.Printf("Set model selection strategy to: %s", strategy)

	return nil
}

// GetAvailableModels returns a copy of the available models from the registry
func (ai *AiderIntegration) GetAvailableModels(ctx context.Context) ([]*ModelMetadata, error) {
	models := ai.registry.ListEnabledModels()
	if len(models) == 0 {
		return nil, NewOllamaError("INVALID_REQUEST", "no models available for selection", nil)
	}
	return models, nil
}

// SelectModel selects the best model based on the configured strategy
func (ai *AiderIntegration) SelectModel(ctx context.Context) (string, error) {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	models := ai.registry.ListEnabledModels()
	if len(models) == 0 {
		return "", NewOllamaError("INVALID_REQUEST", "no models available for selection", nil)
	}

	var selectedName string

	switch ai.selectionStrategy {
	case FastestModel:
		selectedName = ai.selectFastestModelName(models)
	case MostCapable:
		selectedName = ai.selectMostCapableModelName(models)
	case RoundRobin:
		selectedName = ai.selectRoundRobinModelName(models)
	default:
		return "", NewOllamaError("INVALID_REQUEST", fmt.Sprintf("unknown selection strategy: %s", ai.selectionStrategy), nil)
	}

	if selectedName == "" {
		return "", NewOllamaError("INVALID_REQUEST", "failed to select a model", nil)
	}

	return selectedName, nil
}

// selectFastestModelName returns the name of the model with the lowest latency
func (ai *AiderIntegration) selectFastestModelName(models []*ModelMetadata) string {
	if len(models) == 0 {
		return ""
	}

	fastest := models[0]
	fastestLatency := int64(^uint64(0) >> 1) // max int64

	for _, model := range models {
		metrics, ok := ai.modelMetrics[model.Name]
		if ok && metrics.AverageLatencyMs < fastestLatency {
			fastest = model
			fastestLatency = metrics.AverageLatencyMs
		}
	}
	return fastest.Name
}

// selectMostCapableModelName returns the name of the model with the highest capability score
func (ai *AiderIntegration) selectMostCapableModelName(models []*ModelMetadata) string {
	if len(models) == 0 {
		return ""
	}

	mostCapable := models[0]
	maxCapability := 0

	for _, model := range models {
		metrics, ok := ai.modelMetrics[model.Name]
		if ok && metrics.CapabilityScore > maxCapability {
			mostCapable = model
			maxCapability = metrics.CapabilityScore
		}
	}
	return mostCapable.Name
}

// selectRoundRobinModelName returns model names in sequence
func (ai *AiderIntegration) selectRoundRobinModelName(models []*ModelMetadata) string {
	if len(models) == 0 {
		return ""
	}

	selected := models[ai.roundRobinIndex]
	ai.roundRobinIndex = (ai.roundRobinIndex + 1) % len(models)
	return selected.Name
}

// BuildCommand constructs the complete aider command with model and flags
func (ai *AiderIntegration) BuildCommand(ctx context.Context, config SessionConfig) (string, error) {
	if config.Model == "" {
		selectedModel, err := ai.SelectModel(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to select model: %w", err)
		}
		config.Model = selectedModel
	}

	// Validate the model format
	if !strings.Contains(config.Model, "/") {
		config.Model = fmt.Sprintf("ollama_chat/%s", config.Model)
	}

	cmd := []string{"aider"}

	// Add model parameter
	cmd = append(cmd, fmt.Sprintf("--model=%s", config.Model))

	// Add mode-specific flags
	switch config.Mode {
	case AskMode:
		cmd = append(cmd, "--no-auto-commits")
		if config.Architecture != "" {
			cmd = append(cmd, fmt.Sprintf("--architecture=%s", config.Architecture))
		}
	case ArchitectMode:
		cmd = append(cmd, "--architect")
		if config.Architecture != "" {
			cmd = append(cmd, fmt.Sprintf("--architecture=%s", config.Architecture))
		}
	case CodeMode:
		// Code mode is default, no special flags needed
	}

	// Add general configuration flags
	if config.AutoCommit {
		cmd = append(cmd, "--auto-commits")
	} else {
		cmd = append(cmd, "--no-auto-commits")
	}

	if config.VerboseMode {
		cmd = append(cmd, "--verbose")
	}

	if config.MaxContextLines > 0 {
		cmd = append(cmd, fmt.Sprintf("--max-context-window=%d", config.MaxContextLines))
	}

	if config.GitOnly {
		cmd = append(cmd, "--git-only")
	}

	// Add file patterns
	if len(config.ScanGlobs) > 0 {
		for _, glob := range config.ScanGlobs {
			cmd = append(cmd, fmt.Sprintf("--scan=%s", glob))
		}
	}

	if len(config.IgnoreGlobs) > 0 {
		for _, glob := range config.IgnoreGlobs {
			cmd = append(cmd, fmt.Sprintf("--ignore=%s", glob))
		}
	}

	return strings.Join(cmd, " "), nil
}

// CreateSessionConfig creates a SessionConfig for a specific use case
func (ai *AiderIntegration) CreateSessionConfig(mode AiderMode, model string) SessionConfig {
	return SessionConfig{
		Mode:            mode,
		Model:           model,
		AutoCommit:      true,
		VerboseMode:     false,
		MaxContextLines: 0,
		GitOnly:         true,
		ScanGlobs:       []string{},
		IgnoreGlobs:     []string{},
		Architecture:    "",
	}
}

// CreateSessionWithInstance creates and configures a new Instance for Aider
func (ai *AiderIntegration) CreateSessionWithInstance(ctx context.Context, opts session.InstanceOptions, config SessionConfig) (*session.Instance, error) {
	// Build the aider command
	command, err := ai.BuildCommand(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to build aider command: %w", err)
	}

	// Update the program in options
	opts.Program = command

	// Create the instance
	instance, err := session.NewInstance(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	log.InfoLog.Printf("Created Aider instance with command: %s", command)

	return instance, nil
}

// GetModelsByTag returns all models with a specific tag
func (ai *AiderIntegration) GetModelsByTag(tag string) ([]*ModelMetadata, error) {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	models := ai.registry.ListEnabledModels()
	var tagged []*ModelMetadata

	for _, model := range models {
		metrics, ok := ai.modelMetrics[model.Name]
		if ok {
			for _, t := range metrics.Tags {
				if t == tag {
					tagged = append(tagged, model)
					break
				}
			}
		}
	}

	if len(tagged) == 0 {
		return nil, NewOllamaError("NOT_FOUND", fmt.Sprintf("no models found with tag: %s", tag), nil)
	}

	return tagged, nil
}

// GetModelsSortedByCapability returns models sorted by capability score (highest first)
func (ai *AiderIntegration) GetModelsSortedByCapability() []*ModelMetadata {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	models := ai.registry.ListEnabledModels()
	modelsCopy := make([]*ModelMetadata, len(models))
	copy(modelsCopy, models)

	sort.Slice(modelsCopy, func(i, j int) bool {
		iMetrics, iOk := ai.modelMetrics[modelsCopy[i].Name]
		jMetrics, jOk := ai.modelMetrics[modelsCopy[j].Name]

		iScore := 0
		if iOk {
			iScore = iMetrics.CapabilityScore
		}
		jScore := 0
		if jOk {
			jScore = jMetrics.CapabilityScore
		}

		return iScore > jScore
	})

	return modelsCopy
}

// GetModelsSortedByLatency returns models sorted by latency (fastest first)
func (ai *AiderIntegration) GetModelsSortedByLatency() []*ModelMetadata {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	models := ai.registry.ListEnabledModels()
	modelsCopy := make([]*ModelMetadata, len(models))
	copy(modelsCopy, models)

	sort.Slice(modelsCopy, func(i, j int) bool {
		iMetrics, iOk := ai.modelMetrics[modelsCopy[i].Name]
		jMetrics, jOk := ai.modelMetrics[modelsCopy[j].Name]

		iLatency := int64(^uint64(0) >> 1) // max int64
		if iOk {
			iLatency = iMetrics.AverageLatencyMs
		}
		jLatency := int64(^uint64(0) >> 1)
		if jOk {
			jLatency = jMetrics.AverageLatencyMs
		}

		return iLatency < jLatency
	})

	return modelsCopy
}

// UpdateModelMetrics updates the metrics for a specific model
func (ai *AiderIntegration) UpdateModelMetrics(modelName string, latencyMs int64, capabilityScore int) error {
	if latencyMs < 0 {
		return NewOllamaError("INVALID_REQUEST", "latency cannot be negative", nil)
	}
	if capabilityScore < 1 || capabilityScore > 100 {
		return NewOllamaError("INVALID_REQUEST", "capability score must be between 1 and 100", nil)
	}

	ai.mu.Lock()
	defer ai.mu.Unlock()

	// Check if model exists in registry
	if _, err := ai.registry.GetModel(modelName); err != nil {
		return err
	}

	if metrics, ok := ai.modelMetrics[modelName]; ok {
		metrics.AverageLatencyMs = latencyMs
		metrics.CapabilityScore = capabilityScore
	} else {
		ai.modelMetrics[modelName] = &AiderModelMetrics{
			Name:             modelName,
			AverageLatencyMs: latencyMs,
			CapabilityScore:  capabilityScore,
			Tags:             []string{},
		}
	}

	log.InfoLog.Printf("Updated metrics for model %s: latency=%dms, capability=%d",
		modelName, latencyMs, capabilityScore)

	return nil
}

// GetModelInfo returns detailed information about a specific model
func (ai *AiderIntegration) GetModelInfo(modelName string) (*ModelMetadata, error) {
	return ai.registry.GetModel(modelName)
}

// ClearModelCache clears the cached model selections
func (ai *AiderIntegration) ClearModelCache() {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	ai.modelCache = make(map[string]string)
}

// SessionOptions provides convenience constructors for common Aider session configurations
type SessionOptions struct {
	// EnableAutoCommit controls automatic git commits
	EnableAutoCommit bool
	// IncludeVerboseOutput enables verbose logging
	IncludeVerboseOutput bool
	// LimitContextTo sets the maximum context lines (0 = unlimited)
	LimitContextTo int
	// RestrictToGitTrackedFiles limits to files in git
	RestrictToGitTrackedFiles bool
	// CustomScanPatterns specifies additional files to include
	CustomScanPatterns []string
	// CustomIgnorePatterns specifies files to exclude
	CustomIgnorePatterns []string
	// ArchitectureDescription for architect mode
	ArchitectureDescription string
}

// DefaultSessionOptions returns sensible defaults for Aider sessions
func DefaultSessionOptions() SessionOptions {
	return SessionOptions{
		EnableAutoCommit:          true,
		IncludeVerboseOutput:      false,
		LimitContextTo:            0,
		RestrictToGitTrackedFiles: true,
		CustomScanPatterns:        []string{},
		CustomIgnorePatterns:      []string{},
		ArchitectureDescription:   "",
	}
}

// ApplySessionOptions applies options to a SessionConfig
func (ai *AiderIntegration) ApplySessionOptions(config *SessionConfig, opts SessionOptions) {
	config.AutoCommit = opts.EnableAutoCommit
	config.VerboseMode = opts.IncludeVerboseOutput
	config.MaxContextLines = opts.LimitContextTo
	config.GitOnly = opts.RestrictToGitTrackedFiles
	config.ScanGlobs = opts.CustomScanPatterns
	config.IgnoreGlobs = opts.CustomIgnorePatterns
	config.Architecture = opts.ArchitectureDescription
}

// PresetConfigurations provides pre-configured session settings for common use cases
type PresetConfigurations struct {
	// FastMode uses the fastest available model with minimal context
	FastMode SessionConfig
	// BalancedMode uses a balanced model with moderate context
	BalancedMode SessionConfig
	// PrecisionMode uses the most capable model with full context
	PrecisionMode SessionConfig
}

// GetPresetConfigurations returns pre-configured session settings
func (ai *AiderIntegration) GetPresetConfigurations() PresetConfigurations {
	fastestModels := ai.GetModelsSortedByLatency()
	capableModels := ai.GetModelsSortedByCapability()

	fastModel := ""
	capableModel := ""

	if len(fastestModels) > 0 {
		fastModel = fastestModels[0].Name
	}
	if len(capableModels) > 0 {
		capableModel = capableModels[0].Name
	}

	return PresetConfigurations{
		FastMode: SessionConfig{
			Mode:            CodeMode,
			Model:           fastModel,
			AutoCommit:      true,
			VerboseMode:     false,
			MaxContextLines: 2000,
			GitOnly:         true,
		},
		BalancedMode: SessionConfig{
			Mode:            CodeMode,
			Model:           capableModel,
			AutoCommit:      true,
			VerboseMode:     false,
			MaxContextLines: 4000,
			GitOnly:         true,
		},
		PrecisionMode: SessionConfig{
			Mode:            CodeMode,
			Model:           capableModel,
			AutoCommit:      true,
			VerboseMode:     false,
			MaxContextLines: 8000,
			GitOnly:         true,
		},
	}
}

// GetRandomModel returns a random model from available models
func (ai *AiderIntegration) GetRandomModel() (*ModelMetadata, error) {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	models := ai.registry.ListEnabledModels()
	if len(models) == 0 {
		return nil, NewOllamaError("INVALID_REQUEST", "no models available", nil)
	}

	index := rand.Intn(len(models))
	return models[index], nil
}

// ModelStats provides statistics about registered models
type ModelStats struct {
	// TotalModels is the count of registered models
	TotalModels int
	// AverageCapability is the mean capability score
	AverageCapability float64
	// AverageLatency is the mean latency in milliseconds
	AverageLatency float64
	// FastestModel is the name of the fastest model
	FastestModel string
	// MostCapableModel is the name of the most capable model
	MostCapableModel string
}

// GetModelStats returns statistics about registered models
func (ai *AiderIntegration) GetModelStats() ModelStats {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	models := ai.registry.ListEnabledModels()
	stats := ModelStats{
		TotalModels: len(models),
	}

	if stats.TotalModels == 0 {
		return stats
	}

	var totalCapability int64
	var totalLatency int64
	var fastestModel *ModelMetadata
	var fastestLatency int64 = int64(^uint64(0) >> 1)
	var mostCapableModel *ModelMetadata
	var maxCapability int

	for _, model := range models {
		metrics, ok := ai.modelMetrics[model.Name]
		if ok {
			totalCapability += int64(metrics.CapabilityScore)
			totalLatency += metrics.AverageLatencyMs

			if metrics.AverageLatencyMs < fastestLatency {
				fastestModel = model
				fastestLatency = metrics.AverageLatencyMs
			}
			if metrics.CapabilityScore > maxCapability {
				mostCapableModel = model
				maxCapability = metrics.CapabilityScore
			}
		}
	}

	stats.AverageCapability = float64(totalCapability) / float64(stats.TotalModels)
	stats.AverageLatency = float64(totalLatency) / float64(stats.TotalModels)

	if fastestModel != nil {
		stats.FastestModel = fastestModel.Name
	}
	if mostCapableModel != nil {
		stats.MostCapableModel = mostCapableModel.Name
	}

	return stats
}
