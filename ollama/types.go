// Package ollama provides a framework for integrating Ollama language models
// with Claude Squad sessions. It includes interfaces for extensibility, error types,
// and common type definitions used across the framework.
//
// The package allows for flexible model management, client operations, and health
// monitoring of Ollama instances.
package ollama

import (
	"context"
	"fmt"
)

// ModelProvider defines the interface for different model sources (local, remote, etc.)
type ModelProvider interface {
	// FetchModels retrieves available models from the provider
	FetchModels(ctx context.Context) ([]*ModelMetadata, error)

	// GetModel retrieves a specific model by name
	GetModel(ctx context.Context, name string) (*ModelMetadata, error)

	// Name returns the name of the provider
	Name() string
}

// HealthChecker defines the interface for health checking capabilities
type HealthChecker interface {
	// CheckHealth performs a health check and returns true if healthy
	CheckHealth(ctx context.Context) (bool, error)

	// GetStatus returns a detailed status string
	GetStatus(ctx context.Context) (string, error)
}

// ClientConfig encapsulates Ollama client configuration
type ClientConfig struct {
	// BaseURL is the base URL of the Ollama API endpoint
	BaseURL string

	// Timeout is the request timeout duration in seconds
	Timeout int

	// RetryAttempts specifies how many times to retry failed requests
	RetryAttempts int

	// Headers contains any custom HTTP headers to include in requests
	Headers map[string]string
}

// RequestOptions encapsulates options for API requests
type RequestOptions struct {
	// Stream indicates if the response should be streamed
	Stream bool

	// Temperature controls randomness in model responses (0-1)
	Temperature float32

	// TopK limits vocabulary to top K most likely tokens
	TopK int

	// TopP uses nucleus sampling with this probability threshold
	TopP float32

	// NumPredict limits the number of tokens to predict
	NumPredict int
}

// FrameworkModelStatus represents the operational status of a model
type FrameworkModelStatus int

const (
	// FrameworkModelStatusUnknown indicates the model status is unknown
	FrameworkModelStatusUnknown FrameworkModelStatus = iota

	// FrameworkModelStatusAvailable indicates the model is available and ready to use
	FrameworkModelStatusAvailable

	// FrameworkModelStatusLoading indicates the model is currently being loaded
	FrameworkModelStatusLoading

	// FrameworkModelStatusUnavailable indicates the model is not available
	FrameworkModelStatusUnavailable

	// FrameworkModelStatusError indicates an error occurred with the model
	FrameworkModelStatusError
)

// String returns the string representation of FrameworkModelStatus
func (ms FrameworkModelStatus) String() string {
	switch ms {
	case FrameworkModelStatusAvailable:
		return "available"
	case FrameworkModelStatusLoading:
		return "loading"
	case FrameworkModelStatusUnavailable:
		return "unavailable"
	case FrameworkModelStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// FrameworkError represents errors specific to the Ollama framework
type FrameworkError struct {
	// Code is a machine-readable error code
	Code string

	// Message is a human-readable error message
	Message string

	// Cause is the underlying error, if any
	Cause error
}

// Error implements the error interface
func (fe *FrameworkError) Error() string {
	if fe.Cause != nil {
		return fmt.Sprintf("ollama: %s (%s): %v", fe.Code, fe.Message, fe.Cause)
	}
	return fmt.Sprintf("ollama: %s (%s)", fe.Code, fe.Message)
}

// Unwrap returns the underlying cause error
func (fe *FrameworkError) Unwrap() error {
	return fe.Cause
}

// NewFrameworkError creates a new FrameworkError
func NewFrameworkError(code, message string, cause error) *FrameworkError {
	return &FrameworkError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// Common error codes
const (
	ErrCodeConnectionFailed = "connection_failed"
	ErrCodeInvalidModel     = "invalid_model"
	ErrCodeInvalidRequest   = "invalid_request"
	ErrCodeNotFound         = "not_found"
	ErrCodeUnauthorized     = "unauthorized"
	ErrCodeTimeout          = "timeout"
	ErrCodeInternal         = "internal_error"
)

// ClientError represents HTTP client-related errors
type ClientError struct {
	StatusCode int
	Message    string
	Cause      error
}

// Error implements the error interface
func (ce *ClientError) Error() string {
	if ce.Cause != nil {
		return fmt.Sprintf("client error (%d): %s: %v", ce.StatusCode, ce.Message, ce.Cause)
	}
	return fmt.Sprintf("client error (%d): %s", ce.StatusCode, ce.Message)
}

// Unwrap returns the underlying cause error
func (ce *ClientError) Unwrap() error {
	return ce.Cause
}

// NewClientError creates a new ClientError
func NewClientError(statusCode int, message string, cause error) *ClientError {
	return &ClientError{
		StatusCode: statusCode,
		Message:    message,
		Cause:      cause,
	}
}

// ParseError represents errors during data parsing
type ParseError struct {
	Field  string
	Value  interface{}
	Reason string
	Cause  error
}

// Error implements the error interface
func (pe *ParseError) Error() string {
	if pe.Cause != nil {
		return fmt.Sprintf("parse error in field %q: %s (%v): %v", pe.Field, pe.Reason, pe.Value, pe.Cause)
	}
	return fmt.Sprintf("parse error in field %q: %s (%v)", pe.Field, pe.Reason, pe.Value)
}

// Unwrap returns the underlying cause error
func (pe *ParseError) Unwrap() error {
	return pe.Cause
}

// NewParseError creates a new ParseError
func NewParseError(field string, value interface{}, reason string, cause error) *ParseError {
	return &ParseError{
		Field:  field,
		Value:  value,
		Reason: reason,
		Cause:  cause,
	}
}
