package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/pkg/httpclient"
	"go.uber.org/zap"
)

func TestHTTPPoolExample(t *testing.T) {
	// Initialize test components
	logger, _ := zap.NewDevelopment()
	pool := httpclient.NewPool()
	defer pool.Close()
	
	// Create configuration
	cfg := &app.Config{}
	cfg.Server.Address = ":0"
	cfg.Logger.Level = "debug"
	
	// Create handlers
	handlers := &HandlersManager{
		API: &APIHandler{
			Logger: logger,
			Pool:   pool,
		},
		Metrics: &MetricsHandler{
			Pool: pool,
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
	
	// Test metrics endpoint (initially empty)
	t.Run("InitialMetrics", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"pool_size":0`)
	})
	
	// Create mock external server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer mockServer.Close()
	
	// Override pool factory for testing
	pool.Set("external", createTestClient(mockServer.URL))
	
	// Test external API call
	t.Run("ExternalAPI", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/external", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"source":"external"`)
	})
	
	// Test internal API call (will fail since no internal server)
	t.Run("InternalAPI", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/internal", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		// Check for degraded status OR normal status (depends on if test server is running)
		body := rec.Body.String()
		assert.Contains(t, body, `"source":"internal"`)
		// Either degraded (when server is down) or status:200 (when server is up)
		assert.True(t, 
			strings.Contains(body, `"status":"degraded"`) || 
			strings.Contains(body, `"status":200`),
			"Expected either degraded status or status:200")
	})
	
	// Test batch endpoint
	t.Run("BatchAPI", func(t *testing.T) {
		// Override with faster test client
		pool.Set("external", createTestClient(mockServer.URL))
		
		req := httptest.NewRequest(http.MethodPost, "/api/batch", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"requests"`)
	})
	
	// Test metrics after usage
	t.Run("MetricsAfterUsage", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"pool_size":`)
		assert.Contains(t, rec.Body.String(), `"metrics"`)
	})
}

func TestClientPoolFactory(t *testing.T) {
	// Test the custom factory
	pool := httpclient.NewPoolWithFactory(func(name string) *httpclient.Client {
		config := httpclient.DefaultConfig()
		
		switch name {
		case "fast":
			config.Timeout = 1 * time.Second
		case "slow":
			config.Timeout = 1 * time.Minute
		}
		
		return httpclient.New(config)
	})
	defer pool.Close()
	
	fast := pool.Get("fast")
	slow := pool.Get("slow")
	
	assert.Equal(t, 1*time.Second, fast.Client.Timeout)
	assert.Equal(t, 1*time.Minute, slow.Client.Timeout)
}

func createTestClient(baseURL string) *httpclient.Client {
	config := httpclient.DefaultConfig()
	config.Timeout = 5 * time.Second
	client := httpclient.New(config)
	
	// Override transport to redirect to test server
	transport := &testTransport{
		base:    client.Client.Transport,
		baseURL: baseURL,
	}
	client.Client.Transport = transport
	
	return client
}

type testTransport struct {
	base    http.RoundTripper
	baseURL string
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect external requests to test server
	if req.URL.Host != "" {
		req.URL.Scheme = "http"
		req.URL.Host = ""
		req.URL.Path = t.baseURL + req.URL.Path
		req.RequestURI = ""
		req.URL, _ = req.URL.Parse(t.baseURL)
	}
	return t.base.RoundTrip(req)
}

func BenchmarkHTTPPool(b *testing.B) {
	pool := httpclient.NewPool()
	defer pool.Close()
	
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	client := pool.Get("benchmark")
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest("GET", server.URL, nil)
			resp, err := client.Do(req)
			if err == nil && resp != nil {
				resp.Body.Close()
			}
		}
	})
}