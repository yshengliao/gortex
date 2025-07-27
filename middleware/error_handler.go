// Package middleware provides standard HTTP middleware for the Gortex framework
package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"
	"github.com/yshengliao/gortex/pkg/errors"
	httpctx "github.com/yshengliao/gortex/transport/http"
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
func ErrorHandler() MiddlewareFunc {
	return ErrorHandlerWithConfig(DefaultErrorHandlerConfig())
}

// ErrorHandlerWithConfig returns a middleware with custom configuration
func ErrorHandlerWithConfig(config *ErrorHandlerConfig) MiddlewareFunc {
	// Apply defaults
	if config == nil {
		config = DefaultErrorHandlerConfig()
	}
	if config.DefaultMessage == "" {
		config.DefaultMessage = "An internal error occurred"
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(c Context) error {
			// Call the next handler
			err := next(c)
			if err == nil {
				return nil
			}

			// Check if response has already been written
			if rw, ok := c.Response().(interface{ Written() bool }); ok && rw.Written() {
				// Response already written, return the error as-is
				return err
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
				errorCode := errors.ErrorCode(e.ErrorDetail.Code)
				code = errors.GetHTTPStatus(errorCode)
				errCode = fmt.Sprintf("ERR_%d", e.ErrorDetail.Code)
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
				// For HTTPError specifically, get the message field
				if httpErr, ok := e.(*httpctx.HTTPError); ok && httpErr.Message != nil {
					message = fmt.Sprintf("%v", httpErr.Message)
				} else {
					message = fmt.Sprintf("%v", e)
				}
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
func writeErrorResponse(c Context, statusCode int, errCode, message string, details map[string]interface{}, requestID string, config *ErrorHandlerConfig) error {
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
	
	// For development mode, add the error to details if it's a standard error
	if !config.HideInternalServerErrorDetails && statusCode >= 500 && details == nil && message != config.DefaultMessage {
		response["error"].(map[string]interface{})["details"] = map[string]interface{}{
			"error": message,
		}
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
func getPath(c Context) string {
	req := c.Request()
	return req.URL.Path
}

