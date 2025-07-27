package requestid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	gortexContext "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

func TestFromGortexContext(t *testing.T) {
	t.Run("from context value", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := gortexContext.NewDefaultContext(req, rec)
		
		expectedID := "test-request-id"
		c.Set("request_id", expectedID)
		
		assert.Equal(t, expectedID, FromGortexContext(c))
	})

	t.Run("from response header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := gortexContext.NewDefaultContext(req, rec)
		
		expectedID := "header-request-id"
		c.Response().Header().Set(HeaderXRequestID, expectedID)
		
		assert.Equal(t, expectedID, FromGortexContext(c))
	})

	t.Run("from request header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		expectedID := "request-header-id"
		req.Header.Set(HeaderXRequestID, expectedID)
		rec := httptest.NewRecorder()
		c := gortexContext.NewDefaultContext(req, rec)
		
		assert.Equal(t, expectedID, FromGortexContext(c))
	})

	t.Run("priority order", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(HeaderXRequestID, "request-id")
		rec := httptest.NewRecorder()
		c := gortexContext.NewDefaultContext(req, rec)
		
		c.Response().Header().Set(HeaderXRequestID, "response-id")
		c.Set("request_id", "context-id")
		
		// Context value should have highest priority
		assert.Equal(t, "context-id", FromGortexContext(c))
	})

	t.Run("no request ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := gortexContext.NewDefaultContext(req, rec)
		
		assert.Empty(t, FromGortexContext(c))
	})
}

func TestContextOperations(t *testing.T) {
	t.Run("WithContext and FromContext", func(t *testing.T) {
		ctx := context.Background()
		requestID := "test-123"
		
		// Add request ID to context
		ctx = WithContext(ctx, requestID)
		
		// Retrieve request ID
		assert.Equal(t, requestID, FromContext(ctx))
	})

	t.Run("FromContext with no ID", func(t *testing.T) {
		ctx := context.Background()
		assert.Empty(t, FromContext(ctx))
	})

	t.Run("WithGortexContext", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := gortexContext.NewDefaultContext(req, rec)
		
		requestID := "echo-request-id"
		c.Set("request_id", requestID)
		
		ctx := context.Background()
		ctx = WithGortexContext(ctx, c)
		
		assert.Equal(t, requestID, FromContext(ctx))
	})
}

func TestHTTPHeaderOperations(t *testing.T) {
	t.Run("SetHeader and GetHeader", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
		requestID := "header-id-123"
		
		SetHeader(req, requestID)
		assert.Equal(t, requestID, req.Header.Get(HeaderXRequestID))
		assert.Equal(t, requestID, GetHeader(req))
	})

	t.Run("SetHeader with empty ID", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
		
		SetHeader(req, "")
		assert.Empty(t, req.Header.Get(HeaderXRequestID))
	})
}

func TestPropagation(t *testing.T) {
	t.Run("PropagateToRequest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := gortexContext.NewDefaultContext(req, rec)
		
		requestID := "propagate-123"
		c.Set("request_id", requestID)
		
		outReq, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
		PropagateToRequest(c, outReq)
		
		assert.Equal(t, requestID, outReq.Header.Get(HeaderXRequestID))
	})

	t.Run("PropagateFromContext", func(t *testing.T) {
		ctx := context.Background()
		requestID := "ctx-propagate-456"
		ctx = WithContext(ctx, requestID)
		
		req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
		PropagateFromContext(ctx, req)
		
		assert.Equal(t, requestID, req.Header.Get(HeaderXRequestID))
	})
}

func TestLoggerIntegration(t *testing.T) {
	t.Run("Logger with request ID", func(t *testing.T) {
		logger := zap.NewNop()
		requestID := "logger-id-123"
		
		newLogger := Logger(logger, requestID)
		assert.NotNil(t, newLogger)
		// Note: We can't easily test that the field was added to a nop logger
		// In a real scenario, you'd use a test logger and verify the field
	})

	t.Run("Logger with empty request ID", func(t *testing.T) {
		logger := zap.NewNop()
		
		newLogger := Logger(logger, "")
		assert.Equal(t, logger, newLogger) // Should return same logger
	})

	t.Run("LoggerFromGortex", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := gortexContext.NewDefaultContext(req, rec)
		
		requestID := "echo-logger-id"
		c.Set("request_id", requestID)
		
		logger := zap.NewNop()
		newLogger := LoggerFromGortex(logger, c)
		assert.NotNil(t, newLogger)
	})

	t.Run("LoggerFromContext", func(t *testing.T) {
		ctx := context.Background()
		requestID := "ctx-logger-id"
		ctx = WithContext(ctx, requestID)
		
		logger := zap.NewNop()
		newLogger := LoggerFromContext(logger, ctx)
		assert.NotNil(t, newLogger)
	})
}

func TestHTTPClient(t *testing.T) {
	t.Run("NewHTTPClient", func(t *testing.T) {
		ctx := context.Background()
		requestID := "client-id-123"
		ctx = WithContext(ctx, requestID)
		
		client := NewHTTPClient(nil, ctx)
		assert.NotNil(t, client)
		assert.Equal(t, http.DefaultClient, client.client)
	})

	t.Run("HTTPClient.Do propagates request ID", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Echo back the request ID
			requestID := r.Header.Get(HeaderXRequestID)
			w.Write([]byte(requestID))
		}))
		defer server.Close()

		ctx := context.Background()
		requestID := "do-test-id"
		ctx = WithContext(ctx, requestID)
		
		client := NewHTTPClient(nil, ctx)
		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		
		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()
		
		// Verify request ID was propagated
		assert.Equal(t, requestID, req.Header.Get(HeaderXRequestID))
	})

	t.Run("HTTPClient.Get", func(t *testing.T) {
		receivedID := ""
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedID = r.Header.Get(HeaderXRequestID)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		ctx := context.Background()
		requestID := "get-test-id"
		ctx = WithContext(ctx, requestID)
		
		client := NewHTTPClient(nil, ctx)
		resp, err := client.Get(server.URL)
		assert.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, requestID, receivedID)
	})

	t.Run("HTTPClient.Post", func(t *testing.T) {
		receivedID := ""
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedID = r.Header.Get(HeaderXRequestID)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		ctx := context.Background()
		requestID := "post-test-id"
		ctx = WithContext(ctx, requestID)
		
		client := NewHTTPClient(nil, ctx)
		resp, err := client.Post(server.URL, "application/json", nil)
		assert.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, requestID, receivedID)
	})
}

// Benchmarks
func BenchmarkFromGortexContext(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := gortexContext.NewDefaultContext(req, rec)
	c.Set("request_id", "bench-id")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FromGortexContext(c)
	}
}

func BenchmarkWithContext(b *testing.B) {
	ctx := context.Background()
	requestID := "bench-id"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WithContext(ctx, requestID)
	}
}

func BenchmarkLogger(b *testing.B) {
	logger := zap.NewNop()
	requestID := "bench-logger-id"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Logger(logger, requestID)
	}
}