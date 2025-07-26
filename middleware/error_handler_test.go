package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/http/context"
	"github.com/yshengliao/gortex/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

func TestErrorHandler(t *testing.T) {
	tests := []struct {
		name           string
		handler        HandlerFunc
		requestID      string
		expectedStatus int
		expectedCode   int
		validateBody   func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "Success - no error",
			handler: func(c context.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				assert.Equal(t, "ok", body["status"])
			},
		},
		{
			name: "ErrorResponse - validation error",
			handler: func(c context.Context) error {
				return errors.NewWithDetails(errors.CodeValidationFailed, "Validation failed", map[string]interface{}{
					"field": "email",
					"error": "invalid format",
				})
			},
			requestID:      "test-request-123",
			expectedStatus: http.StatusBadRequest,
			expectedCode:   errors.CodeValidationFailed.Int(),
			validateBody: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))
				assert.Equal(t, "test-request-123", body["request_id"])

				errorDetail := body["error"].(map[string]interface{})
				assert.Equal(t, float64(errors.CodeValidationFailed.Int()), errorDetail["code"])
				assert.Equal(t, "Validation failed", errorDetail["message"])

				details := errorDetail["details"].(map[string]interface{})
				assert.Equal(t, "email", details["field"])
				assert.Equal(t, "invalid format", details["error"])
			},
		},
		{
			name: "ErrorResponse - unauthorized",
			handler: func(c context.Context) error {
				return errors.New(errors.CodeTokenExpired, "Token has expired")
			},
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   errors.CodeTokenExpired.Int(),
			validateBody: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))

				errorDetail := body["error"].(map[string]interface{})
				assert.Equal(t, float64(errors.CodeTokenExpired.Int()), errorDetail["code"])
				assert.Equal(t, "Token has expired", errorDetail["message"])
			},
		},
		{
			name: "Echo HTTPError - 404",
			handler: func(c context.Context) error {
				return context.NewHTTPError(http.StatusNotFound, "Resource not found")
			},
			expectedStatus: http.StatusNotFound,
			expectedCode:   errors.CodeResourceNotFound.Int(),
			validateBody: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))

				errorDetail := body["error"].(map[string]interface{})
				assert.Equal(t, float64(errors.CodeResourceNotFound.Int()), errorDetail["code"])
				assert.Equal(t, "Resource not found", errorDetail["message"])
			},
		},
		{
			name: "Echo HTTPError - 500 with internal error",
			handler: func(c context.Context) error {
				err := context.NewHTTPError(http.StatusInternalServerError, "Something went wrong")
				err.Internal = fmt.Errorf("database connection failed")
				return err
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   errors.CodeInternalServerError.Int(),
			validateBody: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))

				errorDetail := body["error"].(map[string]interface{})
				assert.Equal(t, float64(errors.CodeInternalServerError.Int()), errorDetail["code"])
				assert.Equal(t, "Something went wrong", errorDetail["message"])

				// Internal error should be in details
				details := errorDetail["details"].(map[string]interface{})
				assert.Contains(t, details["internal"], "database connection failed")
			},
		},
		{
			name: "Standard Go error - hidden in production",
			handler: func(c context.Context) error {
				return fmt.Errorf("database connection timeout")
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   errors.CodeInternalServerError.Int(),
			validateBody: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))

				errorDetail := body["error"].(map[string]interface{})
				assert.Equal(t, float64(errors.CodeInternalServerError.Int()), errorDetail["code"])
				// In production mode, actual error is hidden
				assert.Equal(t, "An internal error occurred", errorDetail["message"])

				// No details should be exposed
				assert.Nil(t, errorDetail["details"])
			},
		},
		{
			name: "Echo HTTPError - rate limit",
			handler: func(c context.Context) error {
				return context.NewHTTPError(http.StatusTooManyRequests, "Rate limit exceeded")
			},
			expectedStatus: http.StatusTooManyRequests,
			expectedCode:   errors.CodeRateLimitExceeded.Int(),
			validateBody: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))

				errorDetail := body["error"].(map[string]interface{})
				assert.Equal(t, float64(errors.CodeRateLimitExceeded.Int()), errorDetail["code"])
				assert.Equal(t, "Rate limit exceeded", errorDetail["message"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.requestID != "" {
				req.Header.Set(context.HeaderXRequestID, tt.requestID)
			}
			rec := httptest.NewRecorder()
			c := context.NewContext(req, rec)

			// Add request ID middleware
			RequestID()(func(c context.Context) error {
				// Apply error handler middleware
				return ErrorHandler()(tt.handler)(c)
			})(c)

			// Assertions
			if tt.expectedStatus > 0 {
				assert.Equal(t, tt.expectedStatus, rec.Code)
			}

			// Parse response body
			var body map[string]interface{}
			err := json.Unmarshal(rec.Body.Bytes(), &body)
			require.NoError(t, err)

			// Validate body
			if tt.validateBody != nil {
				tt.validateBody(t, body)
			}

			// Check timestamp exists for error responses
			if tt.expectedCode > 0 {
				assert.NotEmpty(t, body["timestamp"])
				// Verify it's a valid timestamp
				timestampStr := body["timestamp"].(string)
				_, err := time.Parse(time.RFC3339, timestampStr)
				assert.NoError(t, err)
			}
		})
	}
}

func TestErrorHandlerWithConfig(t *testing.T) {
	t.Run("Development mode - show error details", func(t *testing.T) {
		// Setup with development config
		logger := zaptest.NewLogger(t)
		config := &ErrorHandlerConfig{
			Logger:                         logger,
			HideInternalServerErrorDetails: false,
			DefaultMessage:                 "Internal error",
		}

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		c := context.NewContext(req, rec)

		handler := func(c context.Context) error {
			return fmt.Errorf("sensitive database error: connection timeout at 192.168.1.1")
		}

		// Apply middleware
		ErrorHandlerWithConfig(config)(handler)(c)

		// Parse response
		var body map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &body)
		require.NoError(t, err)

		// In development mode, actual error is shown
		errorDetail := body["error"].(map[string]interface{})
		assert.Equal(t, "sensitive database error: connection timeout at 192.168.1.1", errorDetail["message"])

		// Error details should be included
		details := errorDetail["details"].(map[string]interface{})
		assert.Equal(t, "sensitive database error: connection timeout at 192.168.1.1", details["error"])
	})

	t.Run("Production mode - hide error details", func(t *testing.T) {
		// Setup with production config
		config := &ErrorHandlerConfig{
			Logger:                         nil,
			HideInternalServerErrorDetails: true,
			DefaultMessage:                 "Something went wrong",
		}

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		c := context.NewContext(req, rec)

		handler := func(c context.Context) error {
			return fmt.Errorf("sensitive database error: connection timeout at 192.168.1.1")
		}

		// Apply middleware
		ErrorHandlerWithConfig(config)(handler)(c)

		// Parse response
		var body map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &body)
		require.NoError(t, err)

		// In production mode, actual error is hidden
		errorDetail := body["error"].(map[string]interface{})
		assert.Equal(t, "Something went wrong", errorDetail["message"])

		// No details should be exposed
		assert.Nil(t, errorDetail["details"])
	})

	t.Run("Nil config uses defaults", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		c := context.NewContext(req, rec)

		handler := func(c context.Context) error {
			return fmt.Errorf("test error")
		}

		// Apply middleware with nil config
		ErrorHandlerWithConfig(nil)(handler)(c)

		// Should use default config
		assert.Equal(t, http.StatusInternalServerError, rec.Code)

		var body map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &body)
		require.NoError(t, err)

		errorDetail := body["error"].(map[string]interface{})
		assert.Equal(t, "An internal error occurred", errorDetail["message"])
	})
}

func TestErrorHandlerCommittedResponse(t *testing.T) {
	// Test that middleware doesn't modify already committed responses
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := context.NewContext(req, rec)

	handler := func(c context.Context) error {
		// Write response
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		// Note: In Gortex, check if response is already written instead
		rec.Flushed = true
		// Return error after response is committed
		return fmt.Errorf("error after commit")
	}

	// Apply middleware
	err := ErrorHandler()(handler)(c)

	// The error should be returned as-is
	assert.Error(t, err)
	assert.Equal(t, "error after commit", err.Error())

	// Response should not be modified
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
}

func TestMapHTTPErrorToCode(t *testing.T) {
	tests := []struct {
		httpStatus   int
		expectedCode errors.ErrorCode
	}{
		{http.StatusBadRequest, errors.CodeInvalidInput},
		{http.StatusUnauthorized, errors.CodeUnauthorized},
		{http.StatusForbidden, errors.CodeForbidden},
		{http.StatusNotFound, errors.CodeResourceNotFound},
		{http.StatusMethodNotAllowed, errors.CodeInvalidOperation},
		{http.StatusNotAcceptable, errors.CodeInvalidFormat},
		{http.StatusRequestTimeout, errors.CodeTimeout},
		{http.StatusConflict, errors.CodeConflict},
		{http.StatusPreconditionFailed, errors.CodePreconditionFailed},
		{http.StatusRequestEntityTooLarge, errors.CodeValueOutOfRange},
		{http.StatusUnprocessableEntity, errors.CodeValidationFailed},
		{http.StatusTooManyRequests, errors.CodeRateLimitExceeded},
		{http.StatusInternalServerError, errors.CodeInternalServerError},
		{http.StatusNotImplemented, errors.CodeNotImplemented},
		{http.StatusBadGateway, errors.CodeBadGateway},
		{http.StatusServiceUnavailable, errors.CodeServiceUnavailable},
		{http.StatusGatewayTimeout, errors.CodeTimeout},
		// Unknown codes
		{599, errors.CodeInternalServerError}, // Unknown 5xx
		{499, errors.CodeInvalidInput},        // Unknown 4xx
		{399, errors.CodeInternalServerError}, // Unknown 3xx
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("HTTP %d", tt.httpStatus), func(t *testing.T) {
			result := mapHTTPErrorToCode(tt.httpStatus)
			assert.Equal(t, tt.expectedCode, result)
		})
	}
}

func TestErrorHandlerLogging(t *testing.T) {
	// Create a test logger
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.Hooks(func(entry zapcore.Entry) error {
		// We can't easily capture fields in tests, but we can verify the log was called
		return nil
	})))

	config := &ErrorHandlerConfig{
		Logger:                         logger,
		HideInternalServerErrorDetails: true,
		DefaultMessage:                 "Internal error",
	}

	tests := []struct {
		name    string
		handler HandlerFunc
	}{
		{
			name: "Log ErrorResponse",
			handler: func(c context.Context) error {
				return errors.New(errors.CodeValidationFailed, "Test validation error")
			},
		},
		{
			name: "Log HTTPError",
			handler: func(c context.Context) error {
				return context.NewHTTPError(http.StatusNotFound, "Not found")
			},
		},
		{
			name: "Log standard error",
			handler: func(c context.Context) error {
				return fmt.Errorf("standard error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()
			c := context.NewContext(req, rec)

			// Apply middleware
			ErrorHandlerWithConfig(config)(tt.handler)(c)

			// Verify response was sent
			assert.NotEqual(t, http.StatusOK, rec.Code)
		})
	}
}
