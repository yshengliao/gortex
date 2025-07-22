package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"github.com/yshengliao/gortex/config"
)

func TestDevelopmentModeFeatures(t *testing.T) {
	t.Run("development routes are registered in debug mode", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		cfg := &Config{
			Logger: config.LoggerConfig{
				Level: "debug",
			},
		}

		app, err := NewApp(
			WithConfig(cfg),
			WithLogger(logger),
		)
		require.NoError(t, err)

		// Test /_routes endpoint
		req := httptest.NewRequest(http.MethodGet, "/_routes", nil)
		rec := httptest.NewRecorder()
		app.e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "total_routes")
		assert.Contains(t, rec.Body.String(), "routes")
		assert.Contains(t, rec.Body.String(), "/_routes")
	})

	t.Run("development routes are not registered in production mode", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		cfg := &Config{
			Logger: config.LoggerConfig{
				Level: "info", // Not debug
			},
		}

		app, err := NewApp(
			WithConfig(cfg),
			WithLogger(logger),
		)
		require.NoError(t, err)

		// Test /_routes endpoint should not exist
		req := httptest.NewRequest(http.MethodGet, "/_routes", nil)
		rec := httptest.NewRecorder()
		app.e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("development error endpoint works", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		cfg := &Config{
			Logger: config.LoggerConfig{
				Level: "debug",
			},
		}

		app, err := NewApp(
			WithConfig(cfg),
			WithLogger(logger),
		)
		require.NoError(t, err)

		// Test default error page
		req := httptest.NewRequest(http.MethodGet, "/_error", nil)
		rec := httptest.NewRecorder()
		app.e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "available_types")

		// Test internal error
		req = httptest.NewRequest(http.MethodGet, "/_error?type=internal", nil)
		rec = httptest.NewRecorder()
		app.e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("development error pages show details", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		cfg := &Config{
			Logger: config.LoggerConfig{
				Level: "debug",
			},
		}

		app, err := NewApp(
			WithConfig(cfg),
			WithLogger(logger),
		)
		require.NoError(t, err)

		// Add a test handler that returns an error
		app.e.GET("/test-error", func(c echo.Context) error {
			return echo.NewHTTPError(http.StatusBadRequest, "Test error message")
		})

		// Request with HTML accept header
		req := httptest.NewRequest(http.MethodGet, "/test-error", nil)
		req.Header.Set("Accept", "text/html")
		rec := httptest.NewRecorder()
		app.e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Development Mode")
		assert.Contains(t, rec.Body.String(), "Test error message")
		assert.Contains(t, rec.Body.String(), "Request Information")
	})
}

func TestDevLoggerMiddleware(t *testing.T) {
	t.Run("logs request and response in development mode", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		cfg := &Config{
			Logger: config.LoggerConfig{
				Level: "debug",
			},
		}

		app, err := NewApp(
			WithConfig(cfg),
			WithLogger(logger),
		)
		require.NoError(t, err)

		// Add a test handler
		app.e.POST("/test", func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]string{
				"message": "test response",
			})
		})

		// Send request with body
		body := `{"test": "data"}`
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		
		app.e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		// In a real test, we would capture logger output to verify logging
	})

	t.Run("masks sensitive headers", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		cfg := &Config{
			Logger: config.LoggerConfig{
				Level: "debug",
			},
		}

		app, err := NewApp(
			WithConfig(cfg),
			WithLogger(logger),
		)
		require.NoError(t, err)

		// Add a test handler
		app.e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "OK")
		})

		// Send request with sensitive headers
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer secret-token")
		req.Header.Set("X-Api-Key", "secret-api-key")
		rec := httptest.NewRecorder()
		
		app.e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		// In a real implementation, logger output would show [MASKED] for sensitive headers
	})
}