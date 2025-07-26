package httpclient

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPool(t *testing.T) {
	pool := NewPool()
	assert.NotNil(t, pool)
	assert.Equal(t, 0, pool.Size())
}

func TestPoolGet(t *testing.T) {
	pool := NewPool()
	
	// Get should create new client
	client1 := pool.Get("test")
	assert.NotNil(t, client1)
	assert.Equal(t, 1, pool.Size())
	
	// Second get should return same client
	client2 := pool.Get("test")
	assert.Equal(t, client1, client2)
	assert.Equal(t, 1, pool.Size())
	
	// Different name should create new client
	client3 := pool.Get("other")
	assert.NotNil(t, client3)
	assert.NotEqual(t, client1, client3)
	assert.Equal(t, 2, pool.Size())
}

func TestPoolGetDefault(t *testing.T) {
	pool := NewPool()
	
	client := pool.GetDefault()
	assert.NotNil(t, client)
	
	// Should be same as Get("default")
	client2 := pool.Get("default")
	assert.Equal(t, client, client2)
}

func TestPoolSet(t *testing.T) {
	pool := NewPool()
	
	// Create custom client
	config := DefaultConfig()
	config.Timeout = 5 * time.Second
	customClient := New(config)
	
	// Set in pool
	pool.Set("custom", customClient)
	assert.Equal(t, 1, pool.Size())
	
	// Get should return the custom client
	retrieved := pool.Get("custom")
	assert.Equal(t, customClient, retrieved)
	
	// Replace existing client
	newClient := New(config)
	pool.Set("custom", newClient)
	retrieved2 := pool.Get("custom")
	assert.Equal(t, newClient, retrieved2)
	assert.NotEqual(t, customClient, retrieved2)
}

func TestPoolRemove(t *testing.T) {
	pool := NewPool()
	
	// Add clients
	pool.Get("client1")
	pool.Get("client2")
	assert.Equal(t, 2, pool.Size())
	
	// Remove one
	pool.Remove("client1")
	assert.Equal(t, 1, pool.Size())
	
	// Remove non-existent (should not panic)
	pool.Remove("non-existent")
	assert.Equal(t, 1, pool.Size())
	
	// Verify removed client is gone
	names := pool.Names()
	assert.Equal(t, []string{"client2"}, names)
}

func TestPoolClose(t *testing.T) {
	pool := NewPool()
	
	// Add some clients
	pool.Get("client1")
	pool.Get("client2")
	pool.Get("client3")
	assert.Equal(t, 3, pool.Size())
	
	// Close all
	pool.Close()
	assert.Equal(t, 0, pool.Size())
}

func TestPoolGetMetrics(t *testing.T) {
	pool := NewPool()
	
	// Create test server
	server := httptest.NewServer(nil)
	defer server.Close()
	
	// Get client and make request
	client := pool.Get("test")
	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, _ := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	
	// Get metrics
	metrics, err := pool.GetMetrics("test")
	require.NoError(t, err)
	assert.Equal(t, int64(1), metrics.TotalRequests)
	
	// Non-existent client
	_, err = pool.GetMetrics("non-existent")
	assert.Error(t, err)
}

func TestPoolGetAllMetrics(t *testing.T) {
	pool := NewPool()
	
	// Create test server
	server := httptest.NewServer(nil)
	defer server.Close()
	
	// Create multiple clients and make requests
	clients := []string{"client1", "client2", "client3"}
	for _, name := range clients {
		client := pool.Get(name)
		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, _ := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	}
	
	// Get all metrics
	allMetrics := pool.GetAllMetrics()
	assert.Len(t, allMetrics, 3)
	
	for _, name := range clients {
		metrics, exists := allMetrics[name]
		assert.True(t, exists)
		assert.Equal(t, int64(1), metrics.TotalRequests)
	}
}

func TestPoolNames(t *testing.T) {
	pool := NewPool()
	
	// Empty pool
	names := pool.Names()
	assert.Empty(t, names)
	
	// Add clients
	pool.Get("alpha")
	pool.Get("beta")
	pool.Get("gamma")
	
	names = pool.Names()
	assert.Len(t, names, 3)
	assert.Contains(t, names, "alpha")
	assert.Contains(t, names, "beta")
	assert.Contains(t, names, "gamma")
}

func TestPoolConcurrency(t *testing.T) {
	pool := NewPool()
	
	// Concurrent access
	var wg sync.WaitGroup
	clientNames := []string{"client1", "client2", "client3", "client4", "client5"}
	
	for i := 0; i < 10; i++ {
		for _, name := range clientNames {
			wg.Add(1)
			go func(n string) {
				defer wg.Done()
				client := pool.Get(n)
				assert.NotNil(t, client)
			}(name)
		}
	}
	
	wg.Wait()
	
	// Should have exactly 5 clients
	assert.Equal(t, len(clientNames), pool.Size())
}

func TestPoolCustomFactory(t *testing.T) {
	// Custom factory that creates clients with specific timeouts
	customFactory := func(name string) *Client {
		config := DefaultConfig()
		config.Timeout = time.Duration(len(name)) * time.Second
		return New(config)
	}
	
	pool := NewPoolWithFactory(customFactory)
	
	// Get clients
	shortClient := pool.Get("abc") // 3 second timeout
	longClient := pool.Get("verylongname") // 12 second timeout
	
	assert.Equal(t, 3*time.Second, shortClient.Client.Timeout)
	assert.Equal(t, 12*time.Second, longClient.Client.Timeout)
}

func TestPoolDefaultFactory(t *testing.T) {
	pool := NewPool()
	
	// Test predefined client types
	internal := pool.Get("internal")
	external := pool.Get("external")
	longPoll := pool.Get("long-poll")
	custom := pool.Get("custom")
	
	// Check timeouts
	assert.Equal(t, 5*time.Second, internal.Client.Timeout)
	assert.Equal(t, 30*time.Second, external.Client.Timeout)
	assert.Equal(t, 5*time.Minute, longPoll.Client.Timeout)
	assert.Equal(t, 30*time.Second, custom.Client.Timeout) // Default
}

func BenchmarkPoolGet(b *testing.B) {
	pool := NewPool()
	
	// Pre-create client
	pool.Get("test")
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = pool.Get("test")
	}
}

func BenchmarkPoolGetConcurrent(b *testing.B) {
	pool := NewPool()
	
	// Pre-create clients
	for i := 0; i < 10; i++ {
		pool.Get(string(rune('a' + i)))
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			name := string(rune('a' + (i % 10)))
			_ = pool.Get(name)
			i++
		}
	})
}