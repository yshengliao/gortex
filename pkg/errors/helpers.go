package errors

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Common HTTP status mappings for error codes
var codeToHTTPStatus = map[ErrorCode]int{
	// Validation errors -> 400
	CodeValidationFailed:      http.StatusBadRequest,
	CodeInvalidInput:          http.StatusBadRequest,
	CodeMissingRequiredField:  http.StatusBadRequest,
	CodeInvalidFormat:         http.StatusBadRequest,
	CodeValueOutOfRange:       http.StatusBadRequest,
	CodeDuplicateValue:        http.StatusBadRequest,
	CodeInvalidLength:         http.StatusBadRequest,
	CodeInvalidType:           http.StatusBadRequest,
	CodeInvalidJSON:           http.StatusBadRequest,
	CodeInvalidQueryParam:     http.StatusBadRequest,

	// Auth errors -> 401/403
	CodeUnauthorized:           http.StatusUnauthorized,
	CodeInvalidCredentials:     http.StatusUnauthorized,
	CodeTokenExpired:           http.StatusUnauthorized,
	CodeTokenInvalid:           http.StatusUnauthorized,
	CodeTokenMissing:           http.StatusUnauthorized,
	CodeForbidden:              http.StatusForbidden,
	CodeInsufficientPermissions: http.StatusForbidden,
	CodeAccountLocked:          http.StatusForbidden,
	CodeAccountNotFound:        http.StatusNotFound,
	CodeSessionExpired:         http.StatusUnauthorized,

	// System errors -> 500/503
	CodeInternalServerError:    http.StatusInternalServerError,
	CodeDatabaseError:          http.StatusInternalServerError,
	CodeServiceUnavailable:     http.StatusServiceUnavailable,
	CodeTimeout:                http.StatusRequestTimeout,
	CodeRateLimitExceeded:      http.StatusTooManyRequests,
	CodeResourceExhausted:      http.StatusServiceUnavailable,
	CodeNotImplemented:         http.StatusNotImplemented,
	CodeBadGateway:             http.StatusBadGateway,
	CodeCircuitBreakerOpen:     http.StatusServiceUnavailable,
	CodeConfigurationError:     http.StatusInternalServerError,

	// Business logic errors -> various
	CodeBusinessLogicError:     http.StatusUnprocessableEntity,
	CodeResourceNotFound:       http.StatusNotFound,
	CodeResourceAlreadyExists:  http.StatusConflict,
	CodeInvalidOperation:       http.StatusBadRequest,
	CodePreconditionFailed:     http.StatusPreconditionFailed,
	CodeConflict:               http.StatusConflict,
	CodeInsufficientBalance:    http.StatusPaymentRequired,
	CodeQuotaExceeded:          http.StatusPaymentRequired,
	CodeInvalidState:           http.StatusConflict,
	CodeDependencyFailed:       http.StatusFailedDependency,
}

// GetHTTPStatus returns the appropriate HTTP status code for an error code
func GetHTTPStatus(code ErrorCode) int {
	if status, ok := codeToHTTPStatus[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// ValidationError creates and sends a validation error response
func ValidationError(c echo.Context, message string, details map[string]any) error {
	err := NewWithDetails(CodeValidationFailed, message, details)
	return err.Send(c, http.StatusBadRequest)
}

// ValidationFieldError creates and sends a validation error for a specific field
func ValidationFieldError(c echo.Context, field, message string) error {
	details := map[string]any{
		"field": field,
		"error": message,
	}
	err := NewWithDetails(CodeInvalidInput, fmt.Sprintf("Validation failed for field: %s", field), details)
	return err.Send(c, http.StatusBadRequest)
}

// UnauthorizedError creates and sends an unauthorized error response
func UnauthorizedError(c echo.Context, message string) error {
	if message == "" {
		message = CodeUnauthorized.Message()
	}
	err := New(CodeUnauthorized, message)
	return err.Send(c, http.StatusUnauthorized)
}

// ForbiddenError creates and sends a forbidden error response
func ForbiddenError(c echo.Context, message string) error {
	if message == "" {
		message = CodeForbidden.Message()
	}
	err := New(CodeForbidden, message)
	return err.Send(c, http.StatusForbidden)
}

// NotFoundError creates and sends a not found error response
func NotFoundError(c echo.Context, resource string) error {
	message := fmt.Sprintf("%s not found", resource)
	err := New(CodeResourceNotFound, message).
		WithDetail("resource", resource)
	return err.Send(c, http.StatusNotFound)
}

// InternalServerError creates and sends an internal server error response
func InternalServerError(c echo.Context, err error) error {
	// In production, hide internal error details
	message := "An internal error occurred"
	details := map[string]any{
		"error": err.Error(),
	}
	
	// You might want to log the actual error here
	errResp := NewWithDetails(CodeInternalServerError, message, details)
	return errResp.Send(c, http.StatusInternalServerError)
}

// RateLimitError creates and sends a rate limit exceeded error response
func RateLimitError(c echo.Context, retryAfter int) error {
	err := New(CodeRateLimitExceeded, "Rate limit exceeded").
		WithDetail("retry_after", retryAfter)
	
	// Set Retry-After header
	c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
	
	return err.Send(c, http.StatusTooManyRequests)
}

// ConflictError creates and sends a conflict error response
func ConflictError(c echo.Context, resource string, reason string) error {
	message := fmt.Sprintf("Conflict with %s", resource)
	if reason != "" {
		message = fmt.Sprintf("%s: %s", message, reason)
	}
	err := New(CodeConflict, message).
		WithDetail("resource", resource).
		WithDetail("reason", reason)
	return err.Send(c, http.StatusConflict)
}

// TimeoutError creates and sends a timeout error response
func TimeoutError(c echo.Context, operation string) error {
	message := fmt.Sprintf("Operation timed out: %s", operation)
	err := New(CodeTimeout, message).
		WithDetail("operation", operation)
	return err.Send(c, http.StatusRequestTimeout)
}

// DatabaseError creates and sends a database error response
func DatabaseError(c echo.Context, operation string) error {
	message := fmt.Sprintf("Database operation failed: %s", operation)
	err := New(CodeDatabaseError, message).
		WithDetail("operation", operation)
	return err.Send(c, http.StatusInternalServerError)
}

// SendError is a generic function to send any error code with custom message
func SendError(c echo.Context, code ErrorCode, message string, details map[string]any) error {
	if message == "" {
		message = code.Message()
	}
	err := NewWithDetails(code, message, details)
	return err.Send(c, GetHTTPStatus(code))
}

// SendErrorCode sends an error using just the error code with default message
func SendErrorCode(c echo.Context, code ErrorCode) error {
	err := NewFromCode(code)
	return err.Send(c, GetHTTPStatus(code))
}

// BadRequest is a shorthand for validation errors
func BadRequest(c echo.Context, message string) error {
	return ValidationError(c, message, nil)
}