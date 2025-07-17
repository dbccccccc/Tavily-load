package errors

import (
	"fmt"
	"net/http"
)

// ErrorType represents the type of error
type ErrorType string

const (
	// Permanent errors that should blacklist the key permanently
	ErrorTypeUnauthorized ErrorType = "unauthorized"
	ErrorTypeInvalidKey   ErrorType = "invalid_key"
	ErrorTypeForbidden    ErrorType = "forbidden"
	ErrorTypeNotFound     ErrorType = "not_found"
	ErrorTypeBadRequest   ErrorType = "bad_request"

	// Temporary errors that should blacklist the key temporarily
	ErrorTypeRateLimit     ErrorType = "rate_limit"
	ErrorTypeQuotaExceeded ErrorType = "quota_exceeded"
	ErrorTypeServerError   ErrorType = "server_error"
	ErrorTypeTimeout       ErrorType = "timeout"
	ErrorTypeNetworkError  ErrorType = "network_error"

	// System errors
	ErrorTypeNoKeysAvailable ErrorType = "no_keys_available"
	ErrorTypeConfigError     ErrorType = "config_error"
	ErrorTypeInternalError   ErrorType = "internal_error"
)

// TavilyError represents an error from the Tavily API or proxy
type TavilyError struct {
	Type       ErrorType `json:"type"`
	Message    string    `json:"message"`
	StatusCode int       `json:"status_code"`
	Key        string    `json:"key,omitempty"`
	Permanent  bool      `json:"permanent"`
	Retryable  bool      `json:"retryable"`
	Details    string    `json:"details,omitempty"`
}

// Error implements the error interface
func (e *TavilyError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("[%s] %s (key: %s...)", e.Type, e.Message, e.Key[:8])
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// IsPermanent returns true if the error should permanently blacklist the key
func (e *TavilyError) IsPermanent() bool {
	return e.Permanent
}

// IsRetryable returns true if the request can be retried with a different key
func (e *TavilyError) IsRetryable() bool {
	return e.Retryable
}

// NewTavilyError creates a new TavilyError
func NewTavilyError(errorType ErrorType, message string, statusCode int) *TavilyError {
	permanent, retryable := classifyError(errorType, statusCode)

	return &TavilyError{
		Type:       errorType,
		Message:    message,
		StatusCode: statusCode,
		Permanent:  permanent,
		Retryable:  retryable,
	}
}

// NewTavilyErrorWithKey creates a new TavilyError with a key
func NewTavilyErrorWithKey(errorType ErrorType, message string, statusCode int, key string) *TavilyError {
	err := NewTavilyError(errorType, message, statusCode)
	err.Key = key
	return err
}

// classifyError determines if an error is permanent and retryable
func classifyError(errorType ErrorType, statusCode int) (permanent bool, retryable bool) {
	switch errorType {
	case ErrorTypeUnauthorized, ErrorTypeInvalidKey, ErrorTypeForbidden:
		return true, true // Permanent error, but retryable with different key
	case ErrorTypeNotFound, ErrorTypeBadRequest:
		return false, false // Not permanent, but not retryable (client error)
	case ErrorTypeRateLimit, ErrorTypeQuotaExceeded:
		return false, true // Temporary error, retryable with different key
	case ErrorTypeServerError, ErrorTypeTimeout, ErrorTypeNetworkError:
		return false, true // Temporary error, retryable
	case ErrorTypeNoKeysAvailable:
		return false, false // System error, not retryable
	default:
		return false, true // Default: temporary and retryable
	}
}

// ParseHTTPError parses an HTTP response and creates a TavilyError
func ParseHTTPError(statusCode int, body []byte, key string) *TavilyError {
	var errorType ErrorType
	var message string

	switch statusCode {
	case http.StatusUnauthorized:
		errorType = ErrorTypeUnauthorized
		message = "Invalid or expired API key"
	case http.StatusForbidden:
		errorType = ErrorTypeForbidden
		message = "Access forbidden"
	case http.StatusNotFound:
		errorType = ErrorTypeNotFound
		message = "Endpoint not found"
	case http.StatusBadRequest:
		errorType = ErrorTypeBadRequest
		message = "Bad request"
	case http.StatusTooManyRequests:
		errorType = ErrorTypeRateLimit
		message = "Rate limit exceeded"
	case 432:
		errorType = ErrorTypeQuotaExceeded
		message = "API quota exceeded"
	case 433:
		errorType = ErrorTypeQuotaExceeded
		message = "Monthly quota exceeded"
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		errorType = ErrorTypeServerError
		message = "Server error"
	default:
		errorType = ErrorTypeInternalError
		message = fmt.Sprintf("HTTP %d error", statusCode)
	}

	if len(body) > 0 && len(body) < 500 {
		message = fmt.Sprintf("%s: %s", message, string(body))
	}

	return NewTavilyErrorWithKey(errorType, message, statusCode, key)
}

// IsTemporaryError checks if an error is temporary
func IsTemporaryError(err error) bool {
	if tavilyErr, ok := err.(*TavilyError); ok {
		return !tavilyErr.IsPermanent()
	}
	return true // Default to temporary for unknown errors
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	if tavilyErr, ok := err.(*TavilyError); ok {
		return tavilyErr.IsRetryable()
	}
	return true // Default to retryable for unknown errors
}
