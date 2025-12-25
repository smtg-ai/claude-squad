package ollama

import (
	"bytes"
	"claude-squad/log"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ModelInfo contains information about an Ollama model
type ModelInfo struct {
	Name            string       `json:"name"`
	ContextWindow   int          `json:"context_window,omitempty"`
	ParameterSize   string       `json:"parameter_size,omitempty"`
	Modified        time.Time    `json:"modified_at,omitempty"`
	Size            int64        `json:"size,omitempty"`
	Digest          string       `json:"digest,omitempty"`
	Quantization    string       `json:"quantization_level,omitempty"`
	Available       bool         `json:"available"`
	LastHealthCheck time.Time    `json:"-"`
	HealthStatus    HealthStatus `json:"-"`
	CachedAt        time.Time    `json:"-"`
}

// HealthStatus represents the health status of a model
type HealthStatus string

const (
	HealthUnknown   HealthStatus = "unknown"
	HealthHealthy   HealthStatus = "healthy"
	HealthUnhealthy HealthStatus = "unhealthy"
)

// ModelChangeEvent represents a change in model availability
type ModelChangeEvent struct {
	Type      ModelChangeType
	Model     *ModelInfo
	Timestamp time.Time
	Error     error
}

// ModelChangeType represents the type of change
type ModelChangeType string

const (
	ModelAdded     ModelChangeType = "added"
	ModelRemoved   ModelChangeType = "removed"
	ModelUpdated   ModelChangeType = "updated"
	ModelHealthy   ModelChangeType = "healthy"
	ModelUnhealthy ModelChangeType = "unhealthy"
)

// OllamaResponse represents the response from Ollama API
type OllamaResponse struct {
	Models []struct {
		Name     string    `json:"name"`
		Modified time.Time `json:"modified_at"`
		Size     int64     `json:"size"`
		Digest   string    `json:"digest"`
		Details  struct {
			ParentModel       string   `json:"parent_model"`
			Format            string   `json:"format"`
			Family            string   `json:"family"`
			Families          []string `json:"families"`
			ParameterSize     string   `json:"parameter_size"`
			QuantizationLevel string   `json:"quantization_level"`
		} `json:"details"`
	} `json:"models"`
}

// ModelDiscovery manages Ollama model discovery and health checking
type ModelDiscovery struct {
	// Configuration
	apiURL        string
	pollInterval  time.Duration
	cacheTTL      time.Duration
	contextWindow int
	httpClient    *http.Client

	// State
	mu           sync.RWMutex
	models       map[string]*ModelInfo
	lastPollTime time.Time
	isHealthy    bool
	pollTicker   *time.Ticker
	stopCh       chan struct{}
	wg           sync.WaitGroup

	// Event notifications
	eventCh        chan ModelChangeEvent
	eventListeners map[string]chan<- ModelChangeEvent
	listenerMu     sync.RWMutex

	// Tracking for change detection
	knownModels map[string]*ModelInfo
}

// validateAPIURL validates that the API URL is safe to use
func validateAPIURL(apiURL string) error {
	if apiURL == "" {
		return nil // Will be set to default
	}

	// Parse the URL
	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check scheme - only allow http and https
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("invalid URL scheme: only http:// and https:// are allowed")
	}

	// Reject URLs with user credentials (user@host)
	if parsedURL.User != nil {
		return fmt.Errorf("URLs with user credentials are not allowed")
	}

	// Validate hostname
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a valid hostname")
	}

	return nil
}

// NewModelDiscovery creates a new ModelDiscovery instance
func NewModelDiscovery(apiURL string, pollInterval time.Duration, cacheTTL time.Duration) *ModelDiscovery {
	if apiURL == "" {
		apiURL = "http://localhost:11434"
	}

	// Validate API URL for security
	if err := validateAPIURL(apiURL); err != nil {
		log.ErrorLog.Printf("invalid API URL %q: %v, using default", apiURL, err)
		apiURL = "http://localhost:11434"
	}

	if pollInterval == 0 {
		pollInterval = 30 * time.Second
	}
	if cacheTTL == 0 {
		cacheTTL = 2 * time.Minute
	}

	return &ModelDiscovery{
		apiURL:        apiURL,
		pollInterval:  pollInterval,
		cacheTTL:      cacheTTL,
		contextWindow: 4096,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		models:         make(map[string]*ModelInfo),
		eventCh:        make(chan ModelChangeEvent, 100),
		eventListeners: make(map[string]chan<- ModelChangeEvent),
		knownModels:    make(map[string]*ModelInfo),
		stopCh:         make(chan struct{}),
	}
}

// Start begins the periodic discovery and health checking process
func (md *ModelDiscovery) Start() error {
	log.InfoLog.Printf("starting Ollama model discovery with API URL: %s", md.apiURL)

	// Initial discovery
	if err := md.discoverModels(); err != nil {
		log.WarningLog.Printf("initial model discovery failed: %v", err)
	}

	md.wg.Add(1)
	go md.pollWorker()

	log.InfoLog.Printf("Ollama model discovery started")
	return nil
}

// Stop gracefully stops the discovery process
func (md *ModelDiscovery) Stop() {
	log.InfoLog.Printf("stopping Ollama model discovery")
	close(md.stopCh)
	md.wg.Wait()
	log.InfoLog.Printf("Ollama model discovery stopped")
}

// pollWorker runs the periodic polling loop
func (md *ModelDiscovery) pollWorker() {
	defer md.wg.Done()

	ticker := time.NewTimer(md.pollInterval)
	everyN := log.NewEvery(60 * time.Second)

	for {
		select {
		case <-md.stopCh:
			ticker.Stop()
			return
		default:
		}

		if err := md.discoverModels(); err != nil {
			if everyN.ShouldLog() {
				log.WarningLog.Printf("model discovery error: %v", err)
			}
		}

		if err := md.checkHealth(); err != nil {
			if everyN.ShouldLog() {
				log.WarningLog.Printf("health check error: %v", err)
			}
		}

		select {
		case <-md.stopCh:
			ticker.Stop()
			return
		case <-ticker.C:
			ticker.Reset(md.pollInterval)
		}
	}
}

// discoverModels queries the Ollama API for available models
func (md *ModelDiscovery) discoverModels() error {
	resp, err := md.httpClient.Get(md.apiURL + "/api/tags")
	if err != nil {
		md.mu.Lock()
		md.isHealthy = false
		md.mu.Unlock()
		return fmt.Errorf("failed to query Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		md.mu.Lock()
		md.isHealthy = false
		md.mu.Unlock()
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Ollama API returned status %d: %s", resp.StatusCode, string(body))
	}

	var olmResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&olmResp); err != nil {
		md.mu.Lock()
		md.isHealthy = false
		md.mu.Unlock()
		return fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	md.mu.Lock()
	defer md.mu.Unlock()

	md.isHealthy = true
	md.lastPollTime = time.Now()

	// Build current models
	currentModels := make(map[string]*ModelInfo)
	for _, model := range olmResp.Models {
		modelInfo := &ModelInfo{
			Name:            model.Name,
			Modified:        model.Modified,
			Size:            model.Size,
			Digest:          model.Digest,
			ParameterSize:   model.Details.ParameterSize,
			Quantization:    model.Details.QuantizationLevel,
			Available:       true,
			LastHealthCheck: time.Now(),
			HealthStatus:    HealthHealthy,
			CachedAt:        time.Now(),
		}

		// Extract context window from model capabilities
		md.detectCapabilities(modelInfo)
		currentModels[model.Name] = modelInfo
	}

	// Detect changes
	md.detectChanges(currentModels)

	// Update cache
	md.models = currentModels

	return nil
}

// detectChanges identifies new, removed, and updated models
func (md *ModelDiscovery) detectChanges(currentModels map[string]*ModelInfo) {
	// Check for new or updated models
	for name, newModel := range currentModels {
		oldModel, exists := md.knownModels[name]
		if !exists {
			// New model
			md.notifyEvent(ModelChangeEvent{
				Type:      ModelAdded,
				Model:     newModel,
				Timestamp: time.Now(),
			})
			log.InfoLog.Printf("new model discovered: %s", name)
		} else if oldModel.Digest != newModel.Digest {
			// Updated model
			md.notifyEvent(ModelChangeEvent{
				Type:      ModelUpdated,
				Model:     newModel,
				Timestamp: time.Now(),
			})
			log.InfoLog.Printf("model updated: %s", name)
		}
	}

	// Check for removed models
	for name := range md.knownModels {
		if _, exists := currentModels[name]; !exists {
			removedModel := md.knownModels[name]
			md.notifyEvent(ModelChangeEvent{
				Type:      ModelRemoved,
				Model:     removedModel,
				Timestamp: time.Now(),
			})
			log.InfoLog.Printf("model removed: %s", name)
		}
	}

	// Update known models
	for name, model := range currentModels {
		md.knownModels[name] = model
	}
}

// checkHealth performs health checks on available models
func (md *ModelDiscovery) checkHealth() error {
	md.mu.RLock()
	models := make([]*ModelInfo, 0, len(md.models))
	for _, model := range md.models {
		models = append(models, model)
	}
	md.mu.RUnlock()

	for _, model := range models {
		// Check if model cache is still valid
		if time.Since(model.CachedAt) > md.cacheTTL {
			if err := md.checkModelHealth(model); err != nil {
				md.updateModelHealth(model, HealthUnhealthy, err)
			} else {
				md.updateModelHealth(model, HealthHealthy, nil)
			}
		}
	}

	return nil
}

// checkModelHealth performs a health check on a specific model
func (md *ModelDiscovery) checkModelHealth(model *ModelInfo) error {
	// Create a simple prompt to test the model
	payload := map[string]interface{}{
		"model":  model.Name,
		"prompt": "test",
		"stream": false,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", md.apiURL+"/api/generate", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := md.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// updateModelHealth updates a model's health status
func (md *ModelDiscovery) updateModelHealth(model *ModelInfo, status HealthStatus, err error) {
	md.mu.Lock()
	defer md.mu.Unlock()

	oldStatus := model.HealthStatus
	model.HealthStatus = status
	model.LastHealthCheck = time.Now()

	// Only notify if status changed
	if oldStatus != status {
		if status == HealthHealthy {
			md.notifyEvent(ModelChangeEvent{
				Type:      ModelHealthy,
				Model:     model,
				Timestamp: time.Now(),
			})
			log.InfoLog.Printf("model healthy: %s", model.Name)
		} else {
			md.notifyEvent(ModelChangeEvent{
				Type:      ModelUnhealthy,
				Model:     model,
				Timestamp: time.Now(),
				Error:     err,
			})
			log.WarningLog.Printf("model unhealthy: %s (error: %v)", model.Name, err)
		}
	}
}

// detectCapabilities detects model capabilities like context window
func (md *ModelDiscovery) detectCapabilities(model *ModelInfo) {
	// Default context window
	model.ContextWindow = md.contextWindow

	// Parse quantization to estimate context
	// This is a heuristic based on common model patterns
	switch model.ParameterSize {
	case "7B":
		model.ContextWindow = 4096
	case "13B":
		model.ContextWindow = 4096
	case "34B":
		model.ContextWindow = 8192
	case "70B":
		model.ContextWindow = 8192
	default:
		model.ContextWindow = md.contextWindow
	}

	// Adjust based on quantization
	if model.Quantization == "q2" || model.Quantization == "q3" {
		if model.ContextWindow > 2048 {
			model.ContextWindow = 2048
		}
	}
}

// notifyEvent sends an event to all registered listeners
func (md *ModelDiscovery) notifyEvent(event ModelChangeEvent) {
	select {
	case md.eventCh <- event:
	default:
		log.WarningLog.Printf("event channel full, dropping event: %v", event.Type)
	}

	md.listenerMu.RLock()
	defer md.listenerMu.RUnlock()

	for _, listener := range md.eventListeners {
		select {
		case listener <- event:
		default:
			log.WarningLog.Printf("listener channel full, dropping event")
		}
	}
}

// Subscribe adds a listener for model change events
func (md *ModelDiscovery) Subscribe(listenerID string) <-chan ModelChangeEvent {
	ch := make(chan ModelChangeEvent, 50)

	md.listenerMu.Lock()
	defer md.listenerMu.Unlock()

	md.eventListeners[listenerID] = ch
	log.InfoLog.Printf("listener subscribed: %s", listenerID)

	return ch
}

// Unsubscribe removes a listener
func (md *ModelDiscovery) Unsubscribe(listenerID string) {
	md.listenerMu.Lock()
	defer md.listenerMu.Unlock()

	if ch, exists := md.eventListeners[listenerID]; exists {
		close(ch)
		delete(md.eventListeners, listenerID)
		log.InfoLog.Printf("listener unsubscribed: %s", listenerID)
	}
}

// GetModels returns a copy of all currently known models
func (md *ModelDiscovery) GetModels() []*ModelInfo {
	md.mu.RLock()
	defer md.mu.RUnlock()

	models := make([]*ModelInfo, 0, len(md.models))
	for _, model := range md.models {
		// Create a copy to prevent external modifications
		modelCopy := *model
		models = append(models, &modelCopy)
	}
	return models
}

// GetModel returns information about a specific model
func (md *ModelDiscovery) GetModel(name string) *ModelInfo {
	md.mu.RLock()
	defer md.mu.RUnlock()

	model, exists := md.models[name]
	if !exists {
		return nil
	}

	// Return a copy
	modelCopy := *model
	return &modelCopy
}

// IsHealthy returns whether the Ollama API is currently healthy
func (md *ModelDiscovery) IsHealthy() bool {
	md.mu.RLock()
	defer md.mu.RUnlock()

	return md.isHealthy
}

// GetLastPollTime returns the timestamp of the last successful poll
func (md *ModelDiscovery) GetLastPollTime() time.Time {
	md.mu.RLock()
	defer md.mu.RUnlock()

	return md.lastPollTime
}

// ModelCount returns the number of available models
func (md *ModelDiscovery) ModelCount() int {
	md.mu.RLock()
	defer md.mu.RUnlock()

	return len(md.models)
}

// HasModel checks if a model exists
func (md *ModelDiscovery) HasModel(name string) bool {
	md.mu.RLock()
	defer md.mu.RUnlock()

	_, exists := md.models[name]
	return exists
}

// GetHealthyModels returns only models with healthy status
func (md *ModelDiscovery) GetHealthyModels() []*ModelInfo {
	md.mu.RLock()
	defer md.mu.RUnlock()

	models := make([]*ModelInfo, 0)
	for _, model := range md.models {
		if model.HealthStatus == HealthHealthy {
			modelCopy := *model
			models = append(models, &modelCopy)
		}
	}
	return models
}

// GetUnhealthyModels returns only models with unhealthy status
func (md *ModelDiscovery) GetUnhealthyModels() []*ModelInfo {
	md.mu.RLock()
	defer md.mu.RUnlock()

	models := make([]*ModelInfo, 0)
	for _, model := range md.models {
		if model.HealthStatus == HealthUnhealthy {
			modelCopy := *model
			models = append(models, &modelCopy)
		}
	}
	return models
}

// SetContextWindow sets the default context window for models without explicit values
func (md *ModelDiscovery) SetContextWindow(contextWindow int) {
	md.mu.Lock()
	defer md.mu.Unlock()

	md.contextWindow = contextWindow
	log.InfoLog.Printf("context window set to: %d", contextWindow)
}

// RefreshModels forces an immediate model discovery
func (md *ModelDiscovery) RefreshModels() error {
	return md.discoverModels()
}

// Events returns the main event channel
func (md *ModelDiscovery) Events() <-chan ModelChangeEvent {
	return md.eventCh
}

// WaitForModels waits until at least one model is discovered or timeout
func (md *ModelDiscovery) WaitForModels(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		md.mu.RLock()
		if len(md.models) > 0 {
			md.mu.RUnlock()
			return nil
		}
		md.mu.RUnlock()

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for models to be discovered")
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// GetStats returns statistics about the discovery state
func (md *ModelDiscovery) GetStats() map[string]interface{} {
	md.mu.RLock()
	defer md.mu.RUnlock()

	healthyCount := 0
	unhealthyCount := 0

	for _, model := range md.models {
		if model.HealthStatus == HealthHealthy {
			healthyCount++
		} else if model.HealthStatus == HealthUnhealthy {
			unhealthyCount++
		}
	}

	return map[string]interface{}{
		"total_models":     len(md.models),
		"healthy_models":   healthyCount,
		"unhealthy_models": unhealthyCount,
		"api_healthy":      md.isHealthy,
		"last_poll":        md.lastPollTime,
		"poll_interval":    md.pollInterval.String(),
		"cache_ttl":        md.cacheTTL.String(),
		"active_listeners": len(md.eventListeners),
	}
}
