package observability_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yshengliao/gortex/context"
	"github.com/yshengliao/gortex/observability"
)

func TestSimpleCollector(t *testing.T) {
	collector := observability.NewSimpleCollector()

	t.Run("RecordHTTPRequest", func(t *testing.T) {
		collector.RecordHTTPRequest("GET", "/api/users", 200, 100*time.Millisecond)
		collector.RecordHTTPRequest("POST", "/api/users", 201, 150*time.Millisecond)

		// In a real implementation, we'd have getters to verify
		// For now, we just ensure no panic
		assert.NotNil(t, collector)
	})

	t.Run("RecordWebSocketConnection", func(t *testing.T) {
		collector.RecordWebSocketConnection(true)
		collector.RecordWebSocketConnection(true)
		collector.RecordWebSocketConnection(false)

		assert.NotNil(t, collector)
	})

	t.Run("RecordBusinessMetric", func(t *testing.T) {
		collector.RecordBusinessMetric("user.login", 1.0, map[string]string{
			"method": "password",
			"status": "success",
		})

		assert.NotNil(t, collector)
	})
}

func TestMetricsMiddleware(t *testing.T) {
	collector := observability.NewSimpleCollector()

	// Create a test handler
	handler := func(c context.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	}

	// Wrap with metrics middleware
	wrappedHandler := observability.MetricsMiddleware(collector)(handler)

	// Create test context
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	ctx := context.NewContext(req, rec)

	// Execute handler
	err := wrappedHandler(ctx)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestNoOpCollector(t *testing.T) {
	collector := &observability.NoOpCollector{}

	// Ensure all methods can be called without panic
	collector.RecordHTTPRequest("GET", "/", 200, time.Second)
	collector.RecordHTTPRequestSize("GET", "/", 1024)
	collector.RecordHTTPResponseSize("GET", "/", 2048)
	collector.RecordWebSocketConnection(true)
	collector.RecordWebSocketMessage("inbound", "text", 512)
	collector.RecordBusinessMetric("test", 1.0, nil)
	collector.RecordGoroutines(10)
	collector.RecordMemoryUsage(1024 * 1024)

	assert.NotNil(t, collector)
}
