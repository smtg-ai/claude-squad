package ollama

import "fmt"

// OllamaError represents an error from the Ollama service
type OllamaError struct {
	Code    string
	Message string
	Err     error
}

// Error implements the error interface
func (e *OllamaError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error, enabling errors.Is and errors.As
func (e *OllamaError) Unwrap() error {
	return e.Err
}

// Common errors
var (
	ErrorTimeout = &OllamaError{
		Code:    "TIMEOUT",
		Message: "Request timeout",
	}

	ErrorConnectionFailed = &OllamaError{
		Code:    "CONNECTION_FAILED",
		Message: "Connection to Ollama service failed",
	}

	ErrorInvalidModel = &OllamaError{
		Code:    "INVALID_MODEL",
		Message: "Invalid or unknown model",
	}

	ErrorResourceExhausted = &OllamaError{
		Code:    "RESOURCE_EXHAUSTED",
		Message: "Ollama service resources exhausted",
	}

	ErrorModelNotLoaded = &OllamaError{
		Code:    "MODEL_NOT_LOADED",
		Message: "Model not loaded in memory",
	}

	ErrorInvalidRequest = &OllamaError{
		Code:    "INVALID_REQUEST",
		Message: "Invalid request parameters",
	}
)

// NewOllamaError creates a new OllamaError with the given code and message
func NewOllamaError(code, message string, err error) *OllamaError {
	return &OllamaError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}
