package errors

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Define test errors
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidInput      = errors.New("invalid input")
	ErrDatabaseTimeout   = errors.New("database timeout")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

// Custom error type for testing
type BusinessError struct {
	Code    string
	Message string
}

func (e *BusinessError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func TestErrorRegistry(t *testing.T) {
	// Create a new registry for testing
	registry := &ErrorRegistry{
		mappings:     make(map[error]*ErrorMapping),
		typeMappings: make(map[string]*ErrorMapping),
	}

	t.Run("Register and retrieve exact error", func(t *testing.T) {
		registry.Register(ErrUserNotFound, CodeResourceNotFound, http.StatusNotFound, "User not found")
		
		mapping, ok := registry.GetMapping(ErrUserNotFound)
		assert.True(t, ok)
		assert.Equal(t, CodeResourceNotFound, mapping.Code)
		assert.Equal(t, http.StatusNotFound, mapping.HTTPStatus)
		assert.Equal(t, "User not found", mapping.Message)
	})

	t.Run("Register with default message", func(t *testing.T) {
		registry.RegisterSimple(ErrInvalidInput, CodeInvalidInput)
		
		mapping, ok := registry.GetMapping(ErrInvalidInput)
		assert.True(t, ok)
		assert.Equal(t, CodeInvalidInput, mapping.Code)
		assert.Equal(t, http.StatusBadRequest, mapping.HTTPStatus)
		assert.Equal(t, CodeInvalidInput.Message(), mapping.Message)
	})

	t.Run("Register error type", func(t *testing.T) {
		// Get the actual type name
		testErr := &BusinessError{Code: "TEST", Message: "test"}
		typeName := getErrorTypeName(testErr)
		
		registry.RegisterType(typeName, CodeBusinessLogicError, http.StatusUnprocessableEntity, "Business rule violation")
		
		bizErr := &BusinessError{Code: "BIZ001", Message: "Some business error"}
		mapping, ok := registry.GetMapping(bizErr)
		assert.True(t, ok)
		assert.NotNil(t, mapping)
		if mapping != nil {
			assert.Equal(t, CodeBusinessLogicError, mapping.Code)
			assert.Equal(t, http.StatusUnprocessableEntity, mapping.HTTPStatus)
			assert.Equal(t, "Business rule violation", mapping.Message)
		}
	})

	t.Run("Get mapping for unregistered error", func(t *testing.T) {
		unknownErr := errors.New("unknown error")
		mapping, ok := registry.GetMapping(unknownErr)
		assert.False(t, ok)
		assert.Nil(t, mapping)
	})

	t.Run("Clear registry", func(t *testing.T) {
		registry.Clear()
		mapping, ok := registry.GetMapping(ErrUserNotFound)
		assert.False(t, ok)
		assert.Nil(t, mapping)
	})
}

func TestGlobalRegistry(t *testing.T) {
	// Clear global registry before tests
	globalRegistry.Clear()

	t.Run("Global register functions", func(t *testing.T) {
		Register(ErrDatabaseTimeout, CodeTimeout, http.StatusRequestTimeout, "Database operation timed out")
		
		// Get the actual type name for BusinessError
		testErr := &BusinessError{Code: "TEST", Message: "test"}
		typeName := getErrorTypeName(testErr)
		RegisterType(typeName, CodeBusinessLogicError, http.StatusUnprocessableEntity, "")
		
		RegisterSimple(ErrInsufficientFunds, CodeInsufficientBalance)

		// Test exact error
		mapping, ok := GetMapping(ErrDatabaseTimeout)
		assert.True(t, ok)
		assert.Equal(t, CodeTimeout, mapping.Code)
		assert.Equal(t, http.StatusRequestTimeout, mapping.HTTPStatus)

		// Test error type
		bizErr := &BusinessError{Code: "BIZ002", Message: "Test"}
		mapping, ok = GetMapping(bizErr)
		assert.True(t, ok)
		assert.Equal(t, CodeBusinessLogicError, mapping.Code)

		// Test simple registration
		mapping, ok = GetMapping(ErrInsufficientFunds)
		assert.True(t, ok)
		assert.Equal(t, CodeInsufficientBalance, mapping.Code)
		assert.Equal(t, http.StatusPaymentRequired, mapping.HTTPStatus)
	})
}

func TestHandleBusinessError(t *testing.T) {
	// Clear and setup global registry
	globalRegistry.Clear()
	Register(ErrUserNotFound, CodeResourceNotFound, http.StatusNotFound, "User not found")
	RegisterSimple(ErrInvalidInput, CodeInvalidInput)

	t.Run("Handle nil error", func(t *testing.T) {
		status, resp := HandleBusinessError(nil)
		assert.Equal(t, http.StatusOK, status)
		assert.Nil(t, resp)
	})

	t.Run("Handle registered error", func(t *testing.T) {
		status, resp := HandleBusinessError(ErrUserNotFound)
		assert.Equal(t, http.StatusNotFound, status)
		assert.NotNil(t, resp)
		assert.Equal(t, CodeResourceNotFound.Int(), resp.ErrorDetail.Code)
		assert.Equal(t, "User not found", resp.ErrorDetail.Message)
		assert.Equal(t, "user not found", resp.ErrorDetail.Details["error"])
	})

	t.Run("Handle ErrorResponse directly", func(t *testing.T) {
		errResp := New(CodeForbidden, "Access denied")
		status, resp := HandleBusinessError(errResp)
		assert.Equal(t, http.StatusForbidden, status)
		assert.Equal(t, errResp, resp)
	})

	t.Run("Handle unregistered error", func(t *testing.T) {
		unknownErr := errors.New("some unknown error")
		status, resp := HandleBusinessError(unknownErr)
		assert.Equal(t, http.StatusInternalServerError, status)
		assert.NotNil(t, resp)
		assert.Equal(t, CodeInternalServerError.Int(), resp.ErrorDetail.Code)
		assert.Equal(t, "An error occurred", resp.ErrorDetail.Message)
		assert.Equal(t, "some unknown error", resp.ErrorDetail.Details["error"])
	})

	t.Run("Handle wrapped error", func(t *testing.T) {
		wrappedErr := fmt.Errorf("operation failed: %w", ErrUserNotFound)
		status, resp := HandleBusinessError(wrappedErr)
		assert.Equal(t, http.StatusNotFound, status)
		assert.NotNil(t, resp)
		assert.Equal(t, CodeResourceNotFound.Int(), resp.ErrorDetail.Code)
		assert.Contains(t, resp.ErrorDetail.Details["error"], "operation failed")
	})
}

func TestErrorTypeName(t *testing.T) {
	t.Run("Standard error", func(t *testing.T) {
		err := errors.New("test")
		typeName := getErrorTypeName(err)
		assert.Contains(t, typeName, "errorString")
	})

	t.Run("Custom error type", func(t *testing.T) {
		err := &BusinessError{Code: "TEST", Message: "test"}
		typeName := getErrorTypeName(err)
		assert.Contains(t, typeName, "BusinessError")
		assert.Contains(t, typeName, "errors")
	})

	t.Run("Nil error", func(t *testing.T) {
		typeName := getErrorTypeName(nil)
		assert.Equal(t, "", typeName)
	})
}