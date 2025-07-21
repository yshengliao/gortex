package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/app"
	"go.uber.org/zap"
)

// TestSimpleExample tests the simple example application
func TestSimpleExample(t *testing.T) {
	// Create logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create handlers
	handlersManager := &HandlersManager{
		Default:   &DefaultHandler{},
		Health:    &HealthHandler{},
		WebSocket: &WebSocketHandler{},
		API:       &APIHandler{Logger: logger},
	}

	// Create application
	application, err := app.NewApp(
		app.WithLogger(logger),
		app.WithHandlers(handlersManager),
	)
	require.NoError(t, err)

	// Get the Echo instance for testing
	e := application.Echo()

	t.Run("Health Check", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var response map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "healthy", response["status"])
	})

	t.Run("Default Endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Welcome to STMP Framework", response["message"])
		assert.Equal(t, "1.0.0", response["version"])
	})

	t.Run("API Endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		rec := httptest.NewRecorder()
		
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "API endpoint", response["message"])
	})

	t.Run("API Echo", func(t *testing.T) {
		payload := map[string]interface{}{
			"message": "test echo",
			"number": 42,
		}
		body, _ := json.Marshal(payload)
		
		req := httptest.NewRequest(http.MethodPost, "/api/echo", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "test echo", response["message"])
		assert.Equal(t, float64(42), response["number"])
	})
}

// TestWebSocketConnection tests WebSocket functionality
func TestWebSocketConnection(t *testing.T) {
	// Skip in CI environment as WebSocket tests require running server
	if testing.Short() {
		t.Skip("Skipping WebSocket test in short mode")
	}

	// Create and start server in background
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	handlersManager := &HandlersManager{
		Default:   &DefaultHandler{},
		Health:    &HealthHandler{},
		WebSocket: &WebSocketHandler{},
		API:       &APIHandler{Logger: logger},
	}

	application, err := app.NewApp(
		app.WithLogger(logger),
		app.WithHandlers(handlersManager),
	)
	require.NoError(t, err)

	// Start server
	go func() {
		application.Run()
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Test WebSocket connection
	url := "ws://localhost:8080/ws"
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Skipf("Could not connect to WebSocket: %v", err)
		return
	}
	defer ws.Close()

	// Send a message
	message := map[string]string{"type": "ping"}
	err = ws.WriteJSON(message)
	assert.NoError(t, err)

	// Read response
	var response map[string]interface{}
	err = ws.ReadJSON(&response)
	assert.NoError(t, err)
	assert.Equal(t, "Welcome to Gortex WebSocket!", response["message"])
}

// BenchmarkHandlers benchmarks the handler performance
func BenchmarkHandlers(b *testing.B) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	handlersManager := &HandlersManager{
		Default:   &DefaultHandler{},
		Health:    &HealthHandler{},
		WebSocket: &WebSocketHandler{},
		API:       &APIHandler{Logger: logger},
	}

	application, _ := app.NewApp(
		app.WithLogger(logger),
		app.WithHandlers(handlersManager),
	)

	e := application.Echo()

	b.Run("Health", func(b *testing.B) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
		}
	})

	b.Run("API", func(b *testing.B) {
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
		}
	})
}