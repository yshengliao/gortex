// Package middleware provides common middleware for the Gortex framework
package middleware

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"github.com/yshengliao/gortex/pkg/errors"
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

// ErrorHandler returns a middleware that handles errors in a consistent format
func ErrorHandler() echo.MiddlewareFunc {
	return ErrorHandlerWithConfig(DefaultErrorHandlerConfig())
}

// ErrorHandlerWithConfig returns a middleware with custom configuration
func ErrorHandlerWithConfig(config *ErrorHandlerConfig) echo.MiddlewareFunc {
	// Apply defaults
	if config == nil {
		config = DefaultErrorHandlerConfig()
	}
	if config.DefaultMessage == "" {
		config.DefaultMessage = "An internal error occurred"
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Call the next handler
			err := next(c)
			if err == nil {
				return nil
			}

			// If response was already committed, we can't modify it
			if c.Response().Committed {
				return err
			}

			// Extract request ID for all error responses
			requestID := errors.GetRequestID(c)

			// Check if it's already an ErrorResponse - if so, just ensure request ID is set
			if errResp, ok := err.(*errors.ErrorResponse); ok {
				if errResp.RequestID == "" {
					errResp.RequestID = requestID
				}
				
				// Log the error
				if config.Logger != nil {
					config.Logger.Error("Request failed",
						zap.Int("code", errResp.ErrorDetail.Code),
						zap.String("message", errResp.ErrorDetail.Message),
						zap.String("request_id", errResp.RequestID),
						zap.String("path", c.Request().URL.Path),
						zap.String("method", c.Request().Method),
						zap.Any("details", errResp.ErrorDetail.Details),
					)
				}
				
				// Get the appropriate HTTP status
				httpStatus := errors.GetHTTPStatus(errors.ErrorCode(errResp.ErrorDetail.Code))
				return c.JSON(httpStatus, errResp)
			}

			// Check if it's an Echo HTTPError
			if he, ok := err.(*echo.HTTPError); ok {
				// Map Echo HTTP errors to our error codes
				code := mapHTTPErrorToCode(he.Code)
				
				// Extract message
				message := fmt.Sprintf("%v", he.Message)
				if message == "" {
					message = http.StatusText(he.Code)
				}

				// Create error response
				errResp := errors.New(code, message).WithRequestID(requestID)
				
				// Add internal details if available
				if he.Internal != nil {
					errResp.WithDetail("internal", fmt.Sprintf("%v", he.Internal))
				}

				// Log the error
				if config.Logger != nil {
					config.Logger.Error("HTTP error",
						zap.Int("http_status", he.Code),
						zap.Int("error_code", code.Int()),
						zap.String("message", message),
						zap.String("request_id", requestID),
						zap.String("path", c.Request().URL.Path),
						zap.String("method", c.Request().Method),
						zap.Error(he.Internal),
					)
				}

				return errResp.Send(c, he.Code)
			}

			// Handle standard Go errors
			// For production, hide internal error details
			message := err.Error()
			if config.HideInternalServerErrorDetails {
				message = config.DefaultMessage
			}

			// Create error response
			errResp := errors.New(errors.CodeInternalServerError, message).
				WithRequestID(requestID)

			// In development mode, add the actual error as a detail
			if !config.HideInternalServerErrorDetails {
				errResp.WithDetail("error", err.Error())
			}

			// Log the actual error
			if config.Logger != nil {
				config.Logger.Error("Unhandled error",
					zap.Error(err),
					zap.String("request_id", requestID),
					zap.String("path", c.Request().URL.Path),
					zap.String("method", c.Request().Method),
				)
			}

			return errResp.Send(c, http.StatusInternalServerError)
		}
	}
}

// mapHTTPErrorToCode maps HTTP status codes to our error codes
func mapHTTPErrorToCode(httpStatus int) errors.ErrorCode {
	switch httpStatus {
	case http.StatusBadRequest:
		return errors.CodeInvalidInput
	case http.StatusUnauthorized:
		return errors.CodeUnauthorized
	case http.StatusForbidden:
		return errors.CodeForbidden
	case http.StatusNotFound:
		return errors.CodeResourceNotFound
	case http.StatusMethodNotAllowed:
		return errors.CodeInvalidOperation
	case http.StatusNotAcceptable:
		return errors.CodeInvalidFormat
	case http.StatusRequestTimeout:
		return errors.CodeTimeout
	case http.StatusConflict:
		return errors.CodeConflict
	case http.StatusPreconditionFailed:
		return errors.CodePreconditionFailed
	case http.StatusRequestEntityTooLarge:
		return errors.CodeValueOutOfRange
	case http.StatusUnprocessableEntity:
		return errors.CodeValidationFailed
	case http.StatusTooManyRequests:
		return errors.CodeRateLimitExceeded
	case http.StatusInternalServerError:
		return errors.CodeInternalServerError
	case http.StatusNotImplemented:
		return errors.CodeNotImplemented
	case http.StatusBadGateway:
		return errors.CodeBadGateway
	case http.StatusServiceUnavailable:
		return errors.CodeServiceUnavailable
	case http.StatusGatewayTimeout:
		return errors.CodeTimeout
	default:
		// Default to internal server error for unknown status codes
		if httpStatus >= 500 {
			return errors.CodeInternalServerError
		} else if httpStatus >= 400 {
			return errors.CodeInvalidInput
		}
		return errors.CodeInternalServerError
	}
}