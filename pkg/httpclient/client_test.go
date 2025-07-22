package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	config := DefaultConfig()
	client := New(config)
	
	assert.NotNil(t, client)
	assert.NotNil(t, client.Client)
	assert.Equal(t, config.Timeout, client.Client.Timeout)
	
	// Check transport configuration
	transport, ok := client.Client.Transport.(*metricsTransport)
	require.True(t, ok)
	
	baseTransport, ok := transport.base.(*http.Transport)
	require.True(t, ok)
	
	assert.Equal(t, config.MaxIdleConns, baseTransport.MaxIdleConns)
	assert.Equal(t, config.MaxIdleConnsPerHost, baseTransport.MaxIdleConnsPerHost)
}

func TestClientDo(t *testing.T) {
	// Create test server
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()
	
	client := NewDefault()
	
	// Make requests
	for i := 0; i < 5; i++ {
		req, err := http.NewRequest("GET", server.URL, nil)
		require.NoError(t, err)
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}
	
	// Check metrics
	metrics := client.GetMetrics()
	assert.Equal(t, int64(5), metrics.TotalRequests)
	assert.Equal(t, int64(5), metrics.TotalResponses)
	assert.Equal(t, int64(0), metrics.TotalErrors)
	assert.Equal(t, int64(5), metrics.StatusCodes[200])
}

func TestClientDoWithContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	client := NewDefault()
	
	// Test with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	req, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)
	
	_, err = client.DoWithContext(ctx, req)
	assert.Error(t, err) // Should timeout
	
	// Check error metrics
	metrics := client.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalRequests)
	assert.Equal(t, int64(1), metrics.TotalErrors)
}

func TestClientMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusOK
		if r.URL.Path == "/error" {
			status = http.StatusInternalServerError
		} else if r.URL.Path == "/notfound" {
			status = http.StatusNotFound
		}
		w.WriteHeader(status)
	}))
	defer server.Close()
	
	client := NewDefault()
	
	// Make various requests
	paths := []string{"/", "/", "/error", "/notfound", "/"}
	for _, path := range paths {
		req, _ := http.NewRequest("GET", server.URL+path, nil)
		resp, _ := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	}
	
	metrics := client.GetMetrics()
	assert.Equal(t, int64(5), metrics.TotalRequests)
	assert.Equal(t, int64(5), metrics.TotalResponses)
	assert.Equal(t, int64(3), metrics.StatusCodes[200])
	assert.Equal(t, int64(1), metrics.StatusCodes[500])
	assert.Equal(t, int64(1), metrics.StatusCodes[404])
}

func TestClientConcurrency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	client := NewDefault()
	
	// Concurrent requests
	var wg sync.WaitGroup
	concurrency := 10
	requestsPerGoroutine := 10
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				req, _ := http.NewRequest("GET", server.URL, nil)
				resp, err := client.Do(req)
				if err == nil && resp != nil {
					resp.Body.Close()
				}
			}
		}()
	}
	
	wg.Wait()
	
	metrics := client.GetMetrics()
	expectedRequests := int64(concurrency * requestsPerGoroutine)
	assert.Equal(t, expectedRequests, metrics.TotalRequests)
	assert.Equal(t, expectedRequests, metrics.TotalResponses)
}

func TestClientConnectionReuse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	config := DefaultConfig()
	config.MaxIdleConnsPerHost = 5
	client := New(config)
	
	// Make multiple requests to the same host
	for i := 0; i < 10; i++ {
		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, err := client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		
		// Small delay to allow connection reuse
		time.Sleep(10 * time.Millisecond)
	}
	
	metrics := client.GetMetrics()
	// Most connections should be reused
	assert.True(t, metrics.ConnectionReuse > 5)
}

func TestClientClose(t *testing.T) {
	client := NewDefault()
	
	// Make a request to establish connections
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, _ := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	
	// Close should not panic
	assert.NotPanics(t, func() {
		client.Close()
	})
}

func TestClientWithoutMetrics(t *testing.T) {
	config := DefaultConfig()
	config.EnableMetrics = false
	client := New(config)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	
	// Metrics should be empty
	metrics := client.GetMetrics()
	assert.Equal(t, int64(0), metrics.TotalRequests)
	assert.Equal(t, int64(0), metrics.TotalResponses)
}

func BenchmarkClient(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()
	
	client := NewDefault()
	req, _ := http.NewRequest("GET", server.URL, nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
	}
}

func BenchmarkClientConcurrent(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	client := NewDefault()
	
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