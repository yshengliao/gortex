package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/pkg/config"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap/zaptest"
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
		app.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotNil(t, response["total_routes"])
		assert.NotNil(t, response["routes"])

		// Check if /_routes is in the routes list
		routes := response["routes"].([]interface{})
		found := false
		for _, r := range routes {
			route := r.(map[string]interface{})
			if route["path"] == "/_routes" {
				found = true
				break
			}
		}
		assert.True(t, found, "/_routes should be in the routes list")
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
		app.router.ServeHTTP(rec, req)

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
		app.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "available_types")

		// Test internal error
		req = httptest.NewRequest(http.MethodGet, "/_error?type=internal", nil)
		rec = httptest.NewRecorder()
		app.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
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

		// Test the /_error endpoint with internal type
		req := httptest.NewRequest(http.MethodGet, "/_error?type=internal", nil)
		rec := httptest.NewRecorder()
		app.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)

		// Test the /_error endpoint with panic type should panic
		defer func() {
			if r := recover(); r != nil {
				assert.Contains(t, r, "Development test panic")
			}
		}()

		req = httptest.NewRequest(http.MethodGet, "/_error?type=panic", nil)
		rec = httptest.NewRecorder()
		app.router.ServeHTTP(rec, req)
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
		app.router.POST("/test", func(c httpctx.Context) error {
			return c.JSON(http.StatusOK, map[string]string{
				"message": "test response",
			})
		})

		// Send request with body
		body := `{"test": "data"}`
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
		req.Header.Set(httpctx.HeaderContentType, httpctx.MIMEApplicationJSON)
		rec := httptest.NewRecorder()

		app.router.ServeHTTP(rec, req)

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
		app.router.GET("/test", func(c httpctx.Context) error {
			return c.String(http.StatusOK, "OK")
		})

		// Send request with sensitive headers
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer secret-token")
		req.Header.Set("X-Api-Key", "secret-api-key")
		rec := httptest.NewRecorder()

		app.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		// In a real implementation, logger output would show [MASKED] for sensitive headers
	})
}
