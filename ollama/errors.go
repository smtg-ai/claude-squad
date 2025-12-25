package ollama

import (
	"fmt"
)

// OllamaError represents an error from the Ollama service or framework operations.
//
// This error type is used throughout the Ollama framework to provide structured error
// information with machine-readable error codes and optional error wrapping.
//
// # Fields
//
// Code: A machine-readable error code (e.g., "TIMEOUT", "INVALID_MODEL")
// Message: A human-readable error message describing what went wrong
// Err: An optional wrapped error providing the underlying cause
//
// # When This Error Is Returned
//
// OllamaError is returned in the following scenarios:
//   - Service timeouts (ErrorTimeout)
//   - Connection failures to Ollama service (ErrorConnectionFailed)
//   - Invalid or unknown model names (ErrorInvalidModel)
//   - Resource exhaustion (ErrorResourceExhausted)
//   - Model not loaded in memory (ErrorModelNotLoaded)
//   - Invalid request parameters (ErrorInvalidRequest)
//   - Pool management errors (ErrorPoolClosed, ErrorPoolAlreadyClosed)
//   - Registry errors (ErrorNoModels, ErrorModelNotFound)
//   - Storage errors (ErrorStorageNotConfigured)
//   - Task dispatch errors (ErrorTaskNotFound, ErrorAgentNotFound)
//
// # Checking for Specific Errors
//
// Use errors.Is to check if an error is a specific sentinel error:
//
//	if errors.Is(err, ollama.ErrorTimeout) {
//	    // Handle timeout
//	}
//
// Use errors.As to extract the OllamaError and access fields:
//
//	var ollamaErr *ollama.OllamaError
//	if errors.As(err, &ollamaErr) {
//	    log.Printf("Error code: %s, message: %s", ollamaErr.Code, ollamaErr.Message)
//	    if ollamaErr.Err != nil {
//	        log.Printf("Underlying cause: %v", ollamaErr.Err)
//	    }
//	}
//
// # Error Wrapping Pattern
//
// When wrapping OllamaError with additional context, use fmt.Errorf with %w:
//
//	if err := someOperation(); err != nil {
//	    return fmt.Errorf("failed to process request: %w", err)
//	}
//
// This preserves the error chain for errors.Is and errors.As:
//
//	err := fmt.Errorf("operation failed: %w", ollama.ErrorTimeout)
//	errors.Is(err, ollama.ErrorTimeout) // true
//
// # Example Usage
//
//	// Creating a new error
//	err := ollama.NewOllamaError("CUSTOM_ERROR", "operation failed", nil)
//
//	// Creating an error with a wrapped cause
//	err := ollama.NewOllamaError("NETWORK_ERROR", "failed to connect", netErr)
//
//	// Checking for specific errors
//	if errors.Is(err, ollama.ErrorModelNotFound) {
//	    // Model doesn't exist, try discovery
//	}
//
//	// Extracting error details
//	var ollamaErr *ollama.OllamaError
//	if errors.As(err, &ollamaErr) {
//	    switch ollamaErr.Code {
//	    case "TIMEOUT":
//	        // Retry with backoff
//	    case "INVALID_MODEL":
//	        // Prompt user for valid model
//	    default:
//	        // Generic error handling
//	    }
//	}
type OllamaError struct {
	// Code is a machine-readable error code (e.g., "TIMEOUT", "INVALID_MODEL")
	Code string

	// Message is a human-readable error message
	Message string

	// Err is the wrapped underlying error, if any
	Err error
}

// Error implements the error interface and formats the error message.
//
// If Err is not nil, the output includes the wrapped error:
//
//	"ERROR_CODE: error message (wrapped error)"
//
// Otherwise:
//
//	"ERROR_CODE: error message"
func (e *OllamaError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error, enabling errors.Is and errors.As.
//
// This allows error chain traversal:
//
//	err := NewOllamaError("TIMEOUT", "request timeout", context.DeadlineExceeded)
//	errors.Is(err, context.DeadlineExceeded) // true
func (e *OllamaError) Unwrap() error {
	return e.Err
}

// Sentinel Errors
//
// These are predefined error values that can be used with errors.Is for error checking.
// All sentinel errors are pointers to OllamaError to ensure identity-based comparison.
//
// # Usage Pattern
//
// Check for specific errors using errors.Is:
//
//	if errors.Is(err, ollama.ErrorTimeout) {
//	    // Retry with backoff
//	}
//
// You can also wrap sentinel errors with additional context:
//
//	return fmt.Errorf("model initialization failed: %w", ollama.ErrorModelNotLoaded)
//
// The wrapped error can still be identified:
//
//	errors.Is(err, ollama.ErrorModelNotLoaded) // true
var (
	// ErrorTimeout indicates a request exceeded the configured timeout duration.
	//
	// Returned by:
	//   - Client.doRequest when context deadline is exceeded
	//   - AgentPool.AcquireAgent when waiting for available agent times out
	//   - Discovery operations when model discovery takes too long
	//
	// Example:
	//   if errors.Is(err, ollama.ErrorTimeout) {
	//       // Implement exponential backoff retry
	//   }
	ErrorTimeout = &OllamaError{
		Code:    "TIMEOUT",
		Message: "Request timeout",
	}

	// ErrorConnectionFailed indicates the Ollama service is unreachable.
	//
	// Returned by:
	//   - Client.doRequest when HTTP request fails
	//   - Discovery.testConnection when service health check fails
	//   - Framework.HealthCheck when connection cannot be established
	//
	// Example:
	//   if errors.Is(err, ollama.ErrorConnectionFailed) {
	//       // Check if Ollama service is running
	//       // Verify network connectivity
	//   }
	ErrorConnectionFailed = &OllamaError{
		Code:    "CONNECTION_FAILED",
		Message: "Connection to Ollama service failed",
	}

	// ErrorInvalidModel indicates a model name is invalid or not recognized.
	//
	// Returned by:
	//   - Client.Generate when model parameter is empty or malformed
	//   - ModelRegistry.GetModel when model doesn't exist
	//   - Router.SelectModel when no matching model is found
	//
	// Example:
	//   if errors.Is(err, ollama.ErrorInvalidModel) {
	//       // Prompt user to select from available models
	//       // Trigger model discovery
	//   }
	ErrorInvalidModel = &OllamaError{
		Code:    "INVALID_MODEL",
		Message: "Invalid or unknown model",
	}

	// ErrorResourceExhausted indicates the Ollama service has no available resources.
	//
	// Returned by:
	//   - AgentPool.AcquireAgent when all agents are busy
	//   - Client when service returns 429 or 503 status
	//   - TaskDispatcher when queue is full
	//
	// Example:
	//   if errors.Is(err, ollama.ErrorResourceExhausted) {
	//       // Wait and retry
	//       // Scale up pool size
	//   }
	ErrorResourceExhausted = &OllamaError{
		Code:    "RESOURCE_EXHAUSTED",
		Message: "Ollama service resources exhausted",
	}

	// ErrorModelNotLoaded indicates a model exists but is not loaded in memory.
	//
	// Returned by:
	//   - Client when Ollama returns model not loaded error
	//   - ModelRegistry when model metadata exists but model is not ready
	//
	// Example:
	//   if errors.Is(err, ollama.ErrorModelNotLoaded) {
	//       // Wait for model to load
	//       // Trigger model loading
	//   }
	ErrorModelNotLoaded = &OllamaError{
		Code:    "MODEL_NOT_LOADED",
		Message: "Model not loaded in memory",
	}

	// ErrorInvalidRequest indicates request parameters are invalid or malformed.
	//
	// Returned by:
	//   - Client.Generate when prompt is empty
	//   - Framework.NewFramework when config is nil
	//   - Aider when no models are available for selection
	//
	// Example:
	//   if errors.Is(err, ollama.ErrorInvalidRequest) {
	//       // Validate input parameters
	//       // Provide user feedback
	//   }
	ErrorInvalidRequest = &OllamaError{
		Code:    "INVALID_REQUEST",
		Message: "Invalid request parameters",
	}

	// ErrorNoModels indicates no models are registered in the registry.
	//
	// Returned by:
	//   - ModelOrchestrator.SelectModel when registry is empty
	//   - Router.SelectModel when no models are available
	//   - Aider.SelectModel when model list is empty
	//
	// Example:
	//   if errors.Is(err, ollama.ErrorNoModels) {
	//       // Trigger model discovery
	//       // Guide user to pull models
	//   }
	ErrorNoModels = &OllamaError{
		Code:    "NO_MODELS",
		Message: "No models registered",
	}

	// ErrorModelNotFound indicates a specific model name was not found in the registry.
	//
	// Returned by:
	//   - ModelRegistry.GetModel when model doesn't exist
	//   - ModelRegistry.GetModelConfig when config doesn't exist
	//   - Metrics when querying stats for non-existent model
	//   - ModelOrchestrator.GetModel when model name is invalid
	//
	// Example:
	//   if errors.Is(err, ollama.ErrorModelNotFound) {
	//       // List available models
	//       // Suggest similar model names
	//   }
	ErrorModelNotFound = &OllamaError{
		Code:    "MODEL_NOT_FOUND",
		Message: "Model not found in registry",
	}

	// ErrorPoolClosed indicates an operation was attempted on a closed agent pool.
	//
	// Returned by:
	//   - AgentPool.AcquireAgent when pool is closed
	//   - AgentPool.WarmPool when pool is closed during initialization
	//
	// Example:
	//   if errors.Is(err, ollama.ErrorPoolClosed) {
	//       // Recreate pool
	//       // Handle graceful shutdown
	//   }
	ErrorPoolClosed = &OllamaError{
		Code:    "POOL_CLOSED",
		Message: "Agent pool is closed",
	}

	// ErrorPoolAlreadyClosed indicates Close() was called on an already closed pool.
	//
	// Returned by:
	//   - AgentPool.Close when called multiple times
	//
	// Example:
	//   if errors.Is(err, ollama.ErrorPoolAlreadyClosed) {
	//       // This is typically safe to ignore
	//   }
	ErrorPoolAlreadyClosed = &OllamaError{
		Code:    "POOL_ALREADY_CLOSED",
		Message: "Pool is already closed",
	}

	// ErrorAgentNotFound indicates a specific agent ID was not found in the pool.
	//
	// Returned by:
	//   - AgentPool.GetAgent when agent doesn't exist
	//
	// Example:
	//   if errors.Is(err, ollama.ErrorAgentNotFound) {
	//       // Agent may have been removed
	//       // Acquire new agent
	//   }
	ErrorAgentNotFound = &OllamaError{
		Code:    "AGENT_NOT_FOUND",
		Message: "Agent not found in pool",
	}

	// ErrorTaskNotFound indicates a specific task ID was not found in the dispatcher.
	//
	// Returned by:
	//   - TaskDispatcher.GetTaskStatus when task doesn't exist
	//   - TaskDispatcher.GetTaskResult when task doesn't exist
	//   - TaskDispatcher.CancelTask when task doesn't exist
	//
	// Example:
	//   if errors.Is(err, ollama.ErrorTaskNotFound) {
	//       // Task may have completed and been cleaned up
	//       // Check if task was submitted successfully
	//   }
	ErrorTaskNotFound = &OllamaError{
		Code:    "TASK_NOT_FOUND",
		Message: "Task not found in dispatcher",
	}

	// ErrorStorageNotConfigured indicates storage operations require configuration.
	//
	// Returned by:
	//   - AgentPool.SaveState when storage config is nil
	//   - AgentPool.LoadState when storage config is nil
	//
	// Example:
	//   if errors.Is(err, ollama.ErrorStorageNotConfigured) {
	//       // Configure storage in PoolConfig
	//       // Disable persistence features
	//   }
	ErrorStorageNotConfigured = &OllamaError{
		Code:    "STORAGE_NOT_CONFIGURED",
		Message: "Storage configuration is required for this operation",
	}
)

// NewOllamaError creates a new OllamaError with the given code, message, and optional wrapped error.
//
// Parameters:
//   - code: A machine-readable error code (e.g., "TIMEOUT", "INVALID_MODEL")
//   - message: A human-readable error message describing the error
//   - err: An optional underlying error to wrap (can be nil)
//
// # Usage Examples
//
// Creating a simple error without wrapping:
//
//	return NewOllamaError("INVALID_INPUT", "model name cannot be empty", nil)
//
// Creating an error that wraps another error:
//
//	if err := client.Connect(); err != nil {
//	    return NewOllamaError("CONNECTION_FAILED", "failed to connect to Ollama", err)
//	}
//
// The wrapped error can be extracted using errors.Unwrap, errors.Is, or errors.As:
//
//	err := NewOllamaError("TIMEOUT", "request timeout", context.DeadlineExceeded)
//	errors.Is(err, context.DeadlineExceeded) // true
//
// # Error Wrapping Best Practices
//
// Throughout the codebase, errors are wrapped using these patterns:
//
// 1. Wrapping with fmt.Errorf and %w (preserves error chain):
//
//	if err := operation(); err != nil {
//	    return fmt.Errorf("failed to execute operation: %w", err)
//	}
//
// 2. Wrapping with NewOllamaError (adds structured error code):
//
//	if err := httpClient.Do(req); err != nil {
//	    return NewOllamaError("CONNECTION_FAILED", "HTTP request failed", err)
//	}
//
// 3. Using sentinel errors directly:
//
//	if len(models) == 0 {
//	    return ErrorNoModels
//	}
//
// 4. Wrapping sentinel errors with context:
//
//	if pool.closed {
//	    return fmt.Errorf("cannot acquire agent: %w", ErrorPoolClosed)
//	}
//
// # Checking Wrapped Errors
//
// Use errors.Is to check if an error or any error in its chain matches a sentinel:
//
//	err := fmt.Errorf("operation failed: %w", ErrorTimeout)
//	if errors.Is(err, ErrorTimeout) {
//	    // Handle timeout (even though it's wrapped)
//	}
//
// Use errors.As to extract a specific error type from the chain:
//
//	var ollamaErr *OllamaError
//	if errors.As(err, &ollamaErr) {
//	    log.Printf("Error code: %s", ollamaErr.Code)
//	}
//
// # Common Error Wrapping Patterns in the Codebase
//
// From pool.go:
//
//	if err := agent.Start(); err != nil {
//	    return fmt.Errorf("failed to start instance: %w", err)
//	}
//
// From discovery.go:
//
//	if err := url.Parse(apiURL); err != nil {
//	    return fmt.Errorf("invalid URL format: %w", err)
//	}
//
// From client.go:
//
//	return NewFrameworkError(ErrCodeConnectionFailed, "failed to execute request", err)
//
// From router.go:
//
//	if len(models) == 0 {
//	    return "", fmt.Errorf("no models registered")
//	}
func NewOllamaError(code, message string, err error) *OllamaError {
	return &OllamaError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}
