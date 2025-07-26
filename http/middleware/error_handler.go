// Package middleware provides standard HTTP middleware for the Gortex framework
package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"
	"github.com/yshengliao/gortex/errors"
	"github.com/yshengliao/gortex/http/router"
)

// ErrorHandlerConfig contains configuration for the error handler middleware
type ErrorHandlerConfig struct {
	// Logger is used to log errors
	Logger *zap.Logger
	// HideInternalServerErrorDetails hides the actual error details in production
	HideInternalServerErrorDetails bool
	// DefaultMessage is the message used when hiding internal server error details
	DefaultMessage string
}

// DefaultErrorHandlerConfig returns the default configuration
func DefaultErrorHandlerConfig() *ErrorHandlerConfig {
	return &ErrorHandlerConfig{
		Logger:                         nil,
		HideInternalServerErrorDetails: true,
		DefaultMessage:                 "An internal error occurred",
	}
}

// ErrorHandler returns a standard HTTP middleware that handles errors
func ErrorHandler() router.Middleware {
	return ErrorHandlerWithConfig(DefaultErrorHandlerConfig())
}

// ErrorHandlerWithConfig returns a middleware with custom configuration
func ErrorHandlerWithConfig(config *ErrorHandlerConfig) router.Middleware {
	// Apply defaults
	if config == nil {
		config = DefaultErrorHandlerConfig()
	}
	if config.DefaultMessage == "" {
		config.DefaultMessage = "An internal error occurred"
	}

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			// Call the next handler
			err := next(c)
			if err == nil {
				return nil
			}

			// Get request ID if available
			requestID := ""
			if id := c.Get("request_id"); id != nil {
				requestID, _ = id.(string)
			}

			// Handle the error
			var code int
			var errCode string
			var message string
			var details map[string]interface{}

			switch e := err.(type) {
			case *errors.ErrorResponse:
				// Use our custom error type
				code = e.ErrorDetail.Code
				errCode = fmt.Sprintf("ERR_%d", code)
				message = e.ErrorDetail.Message
				if e.ErrorDetail.Details != nil {
					details = make(map[string]interface{})
					for k, v := range e.ErrorDetail.Details {
						details[k] = v
					}
				}
			case interface{ StatusCode() int }:
				// Handle errors with StatusCode method (Echo compatibility)
				code = e.StatusCode()
				errCode = fmt.Sprintf("HTTP_%d", code)
				message = fmt.Sprintf("%v", e)
			default:
				// Generic error - default to 500
				code = http.StatusInternalServerError
				errCode = "INTERNAL_ERROR"
				message = err.Error()
				
				// Hide details in production
				if config.HideInternalServerErrorDetails {
					message = config.DefaultMessage
				}
				
				// Log the actual error
				if config.Logger != nil {
					config.Logger.Error("Internal server error",
						zap.Error(err),
						zap.String("request_id", requestID),
						zap.String("path", getPath(c)))
				}
			}

			return writeErrorResponse(c, code, errCode, message, details, requestID, config)
		}
	}
}

// writeErrorResponse writes the error response in a consistent format
func writeErrorResponse(c router.Context, statusCode int, errCode, message string, details map[string]interface{}, requestID string, config *ErrorHandlerConfig) error {
	// Build error response
	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    errCode,
			"message": message,
		},
	}
	
	// Add request ID if available
	if requestID != "" {
		response["request_id"] = requestID
	}
	
	// Add details if available and not hidden
	if details != nil && (!config.HideInternalServerErrorDetails || statusCode < 500) {
		response["error"].(map[string]interface{})["details"] = details
	}
	
	// Set response header and write JSON
	if resp, ok := c.Response().(http.ResponseWriter); ok {
		resp.Header().Set("Content-Type", "application/json")
		resp.WriteHeader(statusCode)
		encoder := json.NewEncoder(resp)
		return encoder.Encode(response)
	}
	
	// Fallback to context JSON method
	return c.JSON(statusCode, response)
}

// getPath safely gets the request path
func getPath(c router.Context) string {
	if req, ok := c.Request().(*http.Request); ok {
		return req.URL.Path
	}
	return "unknown"
}

