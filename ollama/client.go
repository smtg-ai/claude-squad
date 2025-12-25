package ollama

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// Client wraps the Ollama API and provides convenient methods for interaction
type Client struct {
	config     *ClientConfig
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new Ollama client with the provided configuration
func NewClient(config *ClientConfig) (*Client, error) {
	if config == nil {
		return nil, NewFrameworkError(ErrCodeInvalidRequest, "client config cannot be nil", nil)
	}

	if config.BaseURL == "" {
		return nil, NewFrameworkError(ErrCodeInvalidRequest, "base URL cannot be empty", nil)
	}

	timeout := time.Duration(config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	return &Client{
		config:     config,
		httpClient: httpClient,
		baseURL:    config.BaseURL,
	}, nil
}

// do performs an HTTP request to the Ollama API with retry logic
func (c *Client) do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return NewFrameworkError(ErrCodeInternal, "failed to marshal request body", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	url := c.baseURL + path
	var lastErr error

	for attempt := 0; attempt <= c.config.RetryAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return NewFrameworkError(ErrCodeInternal, "failed to create request", err)
		}

		// Set content type
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		// Add custom headers
		for key, value := range c.config.Headers {
			req.Header.Set(key, value)
		}

		// Reset body reader for retry
		if body != nil && attempt > 0 {
			bodyBytes, _ := json.Marshal(body)
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < c.config.RetryAttempts {
				baseDelay := time.Duration(100) * time.Millisecond
				backoff := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
				jitter := time.Duration(rand.Int63n(int64(baseDelay)))
				time.Sleep(backoff + jitter)
				continue
			}
			return NewFrameworkError(ErrCodeConnectionFailed, "failed to execute request", err)
		}

		defer resp.Body.Close()

		// Handle HTTP errors
		if resp.StatusCode >= 400 {
			respBody, _ := io.ReadAll(resp.Body)
			return NewClientError(resp.StatusCode, string(respBody), nil)
		}

		// Parse response
		if result != nil {
			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				return NewFrameworkError(ErrCodeInternal, "failed to read response", err)
			}

			if err := json.Unmarshal(respBody, result); err != nil {
				return NewParseError("response", string(respBody), "failed to unmarshal response", err)
			}
		}

		return nil
	}

	return NewFrameworkError(ErrCodeConnectionFailed, "failed after retries", lastErr)
}

// CheckHealth checks if the Ollama API is healthy and accessible
func (c *Client) CheckHealth(ctx context.Context) (bool, error) {
	// Use a short timeout for health check
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	status, err := c.GetStatus(healthCtx)
	if err != nil {
		return false, err
	}

	return status != "", nil
}

// GetStatus retrieves detailed status information from the Ollama API
func (c *Client) GetStatus(ctx context.Context) (string, error) {
	path := "/api/version"
	var result map[string]interface{}

	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return "", NewFrameworkError(ErrCodeConnectionFailed, "failed to get status", err)
	}

	if version, ok := result["version"].(string); ok {
		return version, nil
	}

	return "unknown", nil
}

// GenerateResponse represents a response from the generate endpoint
type GenerateResponse struct {
	Model     string    `json:"model"`
	Response  string    `json:"response"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

// Generate sends a generation request to the Ollama API
func (c *Client) Generate(ctx context.Context, model string, prompt string, opts *RequestOptions) (*GenerateResponse, error) {
	if model == "" {
		return nil, NewFrameworkError(ErrCodeInvalidModel, "model cannot be empty", nil)
	}

	if prompt == "" {
		return nil, NewFrameworkError(ErrCodeInvalidRequest, "prompt cannot be empty", nil)
	}

	reqBody := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	}

	// Apply request options if provided
	if opts != nil {
		if opts.Temperature > 0 {
			reqBody["temperature"] = opts.Temperature
		}
		if opts.TopK > 0 {
			reqBody["top_k"] = opts.TopK
		}
		if opts.TopP > 0 {
			reqBody["top_p"] = opts.TopP
		}
		if opts.NumPredict > 0 {
			reqBody["num_predict"] = opts.NumPredict
		}
	}

	var result GenerateResponse
	if err := c.do(ctx, http.MethodPost, "/api/generate", reqBody, &result); err != nil {
		return nil, NewFrameworkError(ErrCodeInternal, "generation failed", err)
	}

	return &result, nil
}

// ListModelsResponse represents the response from the list models endpoint
type ListModelsResponse struct {
	Models []ModelData `json:"models"`
}

// ModelData represents model information in the list response
type ModelData struct {
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	Digest    string    `json:"digest"`
	Modified  time.Time `json:"modified_at"`
	CreatedAt time.Time `json:"created_at"`
}

// ListModels retrieves all available models from the Ollama API
func (c *Client) ListModels(ctx context.Context) ([]*ModelMetadata, error) {
	var resp ListModelsResponse

	if err := c.do(ctx, http.MethodGet, "/api/tags", nil, &resp); err != nil {
		return nil, NewFrameworkError(ErrCodeInternal, "failed to list models", err)
	}

	models := make([]*ModelMetadata, len(resp.Models))
	for i, m := range resp.Models {
		models[i] = &ModelMetadata{
			Name:       m.Name,
			FullName:   m.Name,
			Size:       m.Size,
			Digest:     m.Digest,
			Modified:   m.Modified,
			CreatedAt:  m.CreatedAt,
			Status:     FrameworkModelStatusAvailable,
			Attributes: make(map[string]interface{}),
		}
	}

	return models, nil
}

// PullModelResponse represents the response from a pull operation
type PullModelResponse struct {
	Status    string `json:"status"`
	Digest    string `json:"digest"`
	Total     int64  `json:"total"`
	Completed int64  `json:"completed"`
}

// PullModel pulls (downloads) a model from Ollama
func (c *Client) PullModel(ctx context.Context, modelName string) error {
	if modelName == "" {
		return NewFrameworkError(ErrCodeInvalidModel, "model name cannot be empty", nil)
	}

	reqBody := map[string]interface{}{
		"name":   modelName,
		"stream": false,
	}

	var result PullModelResponse
	if err := c.do(ctx, http.MethodPost, "/api/pull", reqBody, &result); err != nil {
		return NewFrameworkError(ErrCodeInternal, fmt.Sprintf("failed to pull model %q", modelName), err)
	}

	return nil
}

// DeleteModelResponse represents the response from a delete operation
type DeleteModelResponse struct {
	Status string `json:"status"`
}

// DeleteModel removes a model from Ollama
func (c *Client) DeleteModel(ctx context.Context, modelName string) error {
	if modelName == "" {
		return NewFrameworkError(ErrCodeInvalidModel, "model name cannot be empty", nil)
	}

	reqBody := map[string]interface{}{
		"name": modelName,
	}

	var result DeleteModelResponse
	if err := c.do(ctx, http.MethodDelete, "/api/delete", reqBody, &result); err != nil {
		return NewFrameworkError(ErrCodeInternal, fmt.Sprintf("failed to delete model %q", modelName), err)
	}

	return nil
}

// EmbedRequest represents a request for embeddings
type EmbedRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// EmbedResponse represents the response from an embeddings request
type EmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// GenerateEmbedding generates embeddings for a given prompt
func (c *Client) GenerateEmbedding(ctx context.Context, model string, prompt string) ([]float32, error) {
	if model == "" {
		return nil, NewFrameworkError(ErrCodeInvalidModel, "model cannot be empty", nil)
	}

	if prompt == "" {
		return nil, NewFrameworkError(ErrCodeInvalidRequest, "prompt cannot be empty", nil)
	}

	req := EmbedRequest{
		Model:  model,
		Prompt: prompt,
	}

	var result EmbedResponse
	if err := c.do(ctx, http.MethodPost, "/api/embeddings", req, &result); err != nil {
		return nil, NewFrameworkError(ErrCodeInternal, "failed to generate embeddings", err)
	}

	return result.Embedding, nil
}

// GetConfig returns the client configuration
func (c *Client) GetConfig() *ClientConfig {
	return c.config
}

// SetTimeout updates the client timeout
func (c *Client) SetTimeout(seconds int) {
	c.config.Timeout = seconds
	c.httpClient.Timeout = time.Duration(seconds) * time.Second
}

// SetBaseURL updates the client's base URL
func (c *Client) SetBaseURL(baseURL string) error {
	if baseURL == "" {
		return NewFrameworkError(ErrCodeInvalidRequest, "base URL cannot be empty", nil)
	}
	c.baseURL = baseURL
	c.config.BaseURL = baseURL
	return nil
}
