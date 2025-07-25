package errors

import (
	"errors"
	"net/http"
	"reflect"
	"sync"
)

// ErrorMapping represents a mapping from a business error to HTTP error details
type ErrorMapping struct {
	Code       ErrorCode
	HTTPStatus int
	Message    string
}

// ErrorRegistry manages mappings between business errors and HTTP responses
type ErrorRegistry struct {
	mu       sync.RWMutex
	mappings map[error]*ErrorMapping
	// For error types (not specific instances)
	typeMappings map[string]*ErrorMapping
}

// globalRegistry is the default registry instance
var globalRegistry = &ErrorRegistry{
	mappings:     make(map[error]*ErrorMapping),
	typeMappings: make(map[string]*ErrorMapping),
}

// Register registers a business error with its HTTP mapping
func Register(err error, code ErrorCode, httpStatus int, message string) {
	globalRegistry.Register(err, code, httpStatus, message)
}

// RegisterType registers an error type (by name) with its HTTP mapping
func RegisterType(errorTypeName string, code ErrorCode, httpStatus int, message string) {
	globalRegistry.RegisterType(errorTypeName, code, httpStatus, message)
}

// RegisterSimple registers a business error with just an error code (uses default HTTP status)
func RegisterSimple(err error, code ErrorCode) {
	globalRegistry.RegisterSimple(err, code)
}

// GetMapping retrieves the error mapping for a given error
func GetMapping(err error) (*ErrorMapping, bool) {
	return globalRegistry.GetMapping(err)
}

// Register registers a business error with its HTTP mapping
func (r *ErrorRegistry) Register(err error, code ErrorCode, httpStatus int, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if message == "" {
		message = code.Message()
	}
	
	r.mappings[err] = &ErrorMapping{
		Code:       code,
		HTTPStatus: httpStatus,
		Message:    message,
	}
}

// RegisterType registers an error type with its HTTP mapping
func (r *ErrorRegistry) RegisterType(errorTypeName string, code ErrorCode, httpStatus int, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if message == "" {
		message = code.Message()
	}
	
	r.typeMappings[errorTypeName] = &ErrorMapping{
		Code:       code,
		HTTPStatus: httpStatus,
		Message:    message,
	}
}

// RegisterSimple registers a business error with just an error code
func (r *ErrorRegistry) RegisterSimple(err error, code ErrorCode) {
	httpStatus := GetHTTPStatus(code)
	r.Register(err, code, httpStatus, "")
}

// GetMapping retrieves the error mapping for a given error
func (r *ErrorRegistry) GetMapping(err error) (*ErrorMapping, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// First, try exact error match
	if mapping, ok := r.mappings[err]; ok {
		return mapping, true
	}
	
	// Then, try error type match
	errType := getErrorTypeName(err)
	if mapping, ok := r.typeMappings[errType]; ok {
		return mapping, true
	}
	
	// Try to unwrap and check again
	unwrapped := errors.Unwrap(err)
	if unwrapped != nil && unwrapped != err {
		return r.GetMapping(unwrapped)
	}
	
	return nil, false
}

// Clear clears all registered mappings
func (r *ErrorRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.mappings = make(map[error]*ErrorMapping)
	r.typeMappings = make(map[string]*ErrorMapping)
}

// getErrorTypeName returns the type name of an error
func getErrorTypeName(err error) string {
	if err == nil {
		return ""
	}
	
	// Use reflection to get the type name
	t := reflect.TypeOf(err)
	if t == nil {
		return ""
	}
	
	// Handle pointer types
	if t.Kind() == reflect.Ptr && t.Elem() != nil {
		t = t.Elem()
	}
	
	// Return package path + type name
	if t.PkgPath() != "" && t.Name() != "" {
		return t.PkgPath() + "." + t.Name()
	}
	
	if t.Name() != "" {
		return t.Name()
	}
	
	// For unnamed types, return the string representation
	return t.String()
}

// HandleBusinessError converts a business error to an HTTP error response
func HandleBusinessError(err error) (int, *ErrorResponse) {
	if err == nil {
		return http.StatusOK, nil
	}
	
	// Check if error is already an ErrorResponse
	if errResp, ok := err.(*ErrorResponse); ok {
		code := ErrorCode(errResp.ErrorDetail.Code)
		return GetHTTPStatus(code), errResp
	}
	
	// Look up in registry
	if mapping, ok := GetMapping(err); ok {
		resp := New(mapping.Code, mapping.Message)
		// Add the original error message as detail
		resp.WithDetail("error", err.Error())
		return mapping.HTTPStatus, resp
	}
	
	// Default to internal server error
	resp := New(CodeInternalServerError, "An error occurred")
	resp.WithDetail("error", err.Error())
	return http.StatusInternalServerError, resp
}

// Common business errors can be pre-registered
func init() {
	// Example registrations - projects can add their own
	// Register(ErrUserNotFound, CodeResourceNotFound, http.StatusNotFound, "User not found")
	// Register(ErrInvalidCredentials, CodeInvalidCredentials, http.StatusUnauthorized, "Invalid username or password")
}