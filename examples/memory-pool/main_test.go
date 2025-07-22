package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/pkg/pool"
	"go.uber.org/zap"
	"github.com/labstack/echo/v4"
)

func TestMemoryPoolExample(t *testing.T) {
	// Initialize components
	logger, _ := zap.NewDevelopment()
	bufferPool := pool.NewBufferPool()
	byteSlicePool := pool.NewByteSlicePool()
	responsePool := pool.NewObjectPool(
		func() *ProcessResponse {
			return &ProcessResponse{
				Data: make(map[string]interface{}),
			}
		},
		func(resp **ProcessResponse) {
			(*resp).ID = ""
			(*resp).Data = make(map[string]interface{})
			(*resp).Buffer = nil
		},
	)
	
	// Create configuration
	cfg := &app.Config{}
	cfg.Server.Address = ":0"
	cfg.Logger.Level = "debug"
	
	// Create handlers
	handlers := &HandlersManager{
		Process: &ProcessHandler{
			Logger:        logger,
			BufferPool:    bufferPool,
			ByteSlicePool: byteSlicePool,
			ResponsePool:  responsePool,
		},
		Metrics: &MetricsHandler{
			BufferPool:    bufferPool,
			ByteSlicePool: byteSlicePool,
			ResponsePool:  responsePool,
		},
	}
	
	// Create application
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	require.NoError(t, err)
	
	e := application.Echo()
	
	// Test JSON processing endpoint
	t.Run("JSONProcessing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/process/json", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var result map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, "Processing with buffer pool", result["message"])
		assert.Equal(t, true, result["reused"])
	})
	
	// Test binary processing endpoint
	t.Run("BinaryProcessing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/process/binary", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/octet-stream", rec.Header().Get("Content-Type"))
		assert.Equal(t, 4096, rec.Body.Len())
	})
	
	// Test batch processing endpoint
	t.Run("BatchProcessing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/process/batch", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"count":10`)
		assert.Contains(t, rec.Body.String(), `"results"`)
	})
	
	// Test metrics endpoint
	t.Run("Metrics", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"buffer_pool"`)
		assert.Contains(t, rec.Body.String(), `"byte_slice_pool"`)
		assert.Contains(t, rec.Body.String(), `"response_pool"`)
		assert.Contains(t, rec.Body.String(), `"reuse_rate"`)
	})
}

func TestPoolEfficiency(t *testing.T) {
	bufferPool := pool.NewBufferPool()
	
	// Simulate multiple operations
	for i := 0; i < 100; i++ {
		buf := bufferPool.Get()
		buf.WriteString("test data")
		bufferPool.Put(buf)
	}
	
	metrics := bufferPool.GetMetrics()
	assert.Equal(t, int64(100), metrics.TotalGet)
	assert.Equal(t, int64(100), metrics.TotalPut)
	// Should have high reuse rate
	assert.True(t, metrics.ReuseRate > 0.9, "Expected high reuse rate, got %f", metrics.ReuseRate)
	assert.True(t, metrics.TotalNew < 10, "Expected few new allocations, got %d", metrics.TotalNew)
}

func BenchmarkJSONProcessing(b *testing.B) {
	logger, _ := zap.NewProduction()
	bufferPool := pool.NewBufferPool()
	byteSlicePool := pool.NewByteSlicePool()
	responsePool := pool.NewObjectPool(
		func() *ProcessResponse {
			return &ProcessResponse{
				Data: make(map[string]interface{}),
			}
		},
		nil,
	)
	
	handler := &ProcessHandler{
		Logger:        logger,
		BufferPool:    bufferPool,
		ByteSlicePool: byteSlicePool,
		ResponsePool:  responsePool,
	}
	
	e := echo.New()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/process/json", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		
		handler.Json(c)
	}
}

func BenchmarkBinaryProcessing(b *testing.B) {
	logger, _ := zap.NewProduction()
	bufferPool := pool.NewBufferPool()
	byteSlicePool := pool.NewByteSlicePool()
	responsePool := pool.NewObjectPool(
		func() *ProcessResponse {
			return &ProcessResponse{
				Data: make(map[string]interface{}),
			}
		},
		nil,
	)
	
	handler := &ProcessHandler{
		Logger:        logger,
		BufferPool:    bufferPool,
		ByteSlicePool: byteSlicePool,
		ResponsePool:  responsePool,
	}
	
	e := echo.New()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/process/binary", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		
		handler.Binary(c)
	}
}