package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/http/context"
	"go.uber.org/zap"
)

// MockHandlers for testing
type MockHandlers struct {
	Root *MockHandler `url:"/"`
}

type MockHandler struct{}

func (h *MockHandler) GET(c context.Context) error {
	return c.String(http.StatusOK, "OK")
}

func TestMonitoringEndpoint(t *testing.T) {
	// Create logger
	logger, _ := zap.NewDevelopment()

	// Create config with debug mode
	cfg := &app.Config{}
	cfg.Logger.Level = "debug"

	// Create app with debug mode
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(&MockHandlers{Root: &MockHandler{}}),
		app.WithDevelopmentMode(),
	)
	require.NoError(t, err)

	// Test monitoring endpoint
	req := httptest.NewRequest(http.MethodGet, "/_monitor", nil)
	rec := httptest.NewRecorder()
	application.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Parse response
	var response map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify structure
	assert.Equal(t, "healthy", response["status"])

	// Check system info
	system, ok := response["system"].(map[string]interface{})
	require.True(t, ok, "system info should be present")
	assert.NotNil(t, system["goroutines"])
	assert.NotNil(t, system["cpu_count"])
	assert.NotNil(t, system["go_version"])
	assert.NotNil(t, system["max_procs"])
	assert.NotNil(t, system["timestamp"])
	assert.NotNil(t, system["uptime_seconds"])

	// Check memory info
	memory, ok := response["memory"].(map[string]interface{})
	require.True(t, ok, "memory info should be present")
	assert.NotNil(t, memory["alloc_mb"])
	assert.NotNil(t, memory["total_alloc_mb"])
	assert.NotNil(t, memory["sys_mb"])
	assert.NotNil(t, memory["heap_alloc_mb"])
	assert.NotNil(t, memory["heap_objects"])
	assert.NotNil(t, memory["num_gc"])

	// Check gc_stats
	_, ok = response["gc_stats"].([]interface{})
	require.True(t, ok, "gc_stats should be present")

	// Check routes info
	routes, ok := response["routes"].(map[string]interface{})
	require.True(t, ok, "routes info should be present")
	assert.NotNil(t, routes["total_routes"])

	// Check server info
	serverInfo, ok := response["server_info"].(map[string]interface{})
	require.True(t, ok, "server_info should be present")
	assert.NotNil(t, serverInfo["debug_mode"])

	// Check compression info
	compression, ok := response["compression"].(map[string]interface{})
	require.True(t, ok, "compression info should be present")
	assert.NotNil(t, compression["gzip_enabled"])
	assert.NotNil(t, compression["compression_level"])
}

func TestMonitoringEndpointNotInProduction(t *testing.T) {
	// Create logger
	logger, _ := zap.NewProduction()

	// Create config with production mode (not debug)
	cfg := &app.Config{}
	cfg.Logger.Level = "info"

	// Create app without debug mode
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(&MockHandlers{Root: &MockHandler{}}),
	)
	require.NoError(t, err)

	// Test monitoring endpoint should not exist
	req := httptest.NewRequest(http.MethodGet, "/_monitor", nil)
	rec := httptest.NewRecorder()
	application.Router().ServeHTTP(rec, req)

	// Should return 404 in production mode
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestMonitoringMetricsValues(t *testing.T) {
	// Create logger
	logger, _ := zap.NewDevelopment()

	// Create config with debug mode
	cfg := &app.Config{}
	cfg.Logger.Level = "debug"

	// Create app with debug mode
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(&MockHandlers{Root: &MockHandler{}}),
		app.WithDevelopmentMode(),
	)
	require.NoError(t, err)

	// Make a request to generate some activity
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	application.Router().ServeHTTP(rec, req)

	// Sleep briefly to ensure uptime is measurable
	time.Sleep(10 * time.Millisecond)

	// Test monitoring endpoint
	req = httptest.NewRequest(http.MethodGet, "/_monitor", nil)
	rec = httptest.NewRecorder()
	application.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Parse response
	var response map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify numeric values
	system := response["system"].(map[string]interface{})
	goroutines := system["goroutines"].(float64)
	assert.Greater(t, goroutines, float64(0), "Should have at least 1 goroutine")

	cpuCount := system["cpu_count"].(float64)
	assert.Greater(t, cpuCount, float64(0), "Should have at least 1 CPU")

	uptimeSeconds := system["uptime_seconds"].(float64)
	assert.Greater(t, uptimeSeconds, float64(0), "Uptime should be positive")

	// Verify memory values
	memory := response["memory"].(map[string]interface{})
	allocMB := memory["alloc_mb"].(float64)
	assert.Greater(t, allocMB, float64(0), "Should have some memory allocated")

	heapObjects := memory["heap_objects"].(float64)
	assert.Greater(t, heapObjects, float64(0), "Should have some heap objects")
}

func TestMonitoringCompressionStatus(t *testing.T) {
	// Create logger
	logger, _ := zap.NewDevelopment()

	// Test with compression enabled
	t.Run("CompressionEnabled", func(t *testing.T) {
		cfg := &app.Config{}
		cfg.Logger.Level = "debug"
		cfg.Server.GZip = true

		application, err := app.NewApp(
			app.WithConfig(cfg),
			app.WithLogger(logger),
			app.WithHandlers(&MockHandlers{Root: &MockHandler{}}),
			app.WithDevelopmentMode(),
		)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/_monitor", nil)
		rec := httptest.NewRecorder()
		application.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		compression := response["compression"].(map[string]interface{})
		assert.Equal(t, true, compression["gzip_enabled"])
		assert.Contains(t, compression["compression_level"].(string), "default")

		contentTypes, ok := compression["content_types"].([]interface{})
		assert.True(t, ok)
		assert.Greater(t, len(contentTypes), 0)

		minSize, ok := compression["min_size_bytes"].(float64)
		assert.True(t, ok)
		assert.Equal(t, float64(1024), minSize)
	})

	// Test with compression disabled
	t.Run("CompressionDisabled", func(t *testing.T) {
		cfg := &app.Config{}
		cfg.Logger.Level = "debug"
		cfg.Server.GZip = false

		application, err := app.NewApp(
			app.WithConfig(cfg),
			app.WithLogger(logger),
			app.WithHandlers(&MockHandlers{Root: &MockHandler{}}),
			app.WithDevelopmentMode(),
		)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/_monitor", nil)
		rec := httptest.NewRecorder()
		application.Router().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		compression := response["compression"].(map[string]interface{})
		assert.Equal(t, false, compression["gzip_enabled"])
		assert.Equal(t, "not configured", compression["compression_level"])
	})
}

func BenchmarkMonitoringEndpoint(b *testing.B) {
	// Create logger
	logger, _ := zap.NewDevelopment()

	// Create config with debug mode
	cfg := &app.Config{}
	cfg.Logger.Level = "debug"

	// Create app
	application, _ := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(&MockHandlers{Root: &MockHandler{}}),
		app.WithDevelopmentMode(),
	)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/_monitor", nil)
		rec := httptest.NewRecorder()
		application.Router().ServeHTTP(rec, req)
	}
}
