package websocket

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestHubMetrics(t *testing.T) {
	t.Run("basic metrics tracking", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		hub := NewHub(logger)
		go hub.Run()
		defer hub.Shutdown()

		// Initial metrics
		metrics := hub.GetMetrics()
		assert.Equal(t, 0, metrics.CurrentConnections)
		assert.Equal(t, int64(0), metrics.TotalConnections)
		assert.Equal(t, int64(0), metrics.MessagesSent)
		assert.Equal(t, int64(0), metrics.MessagesReceived)
		assert.Empty(t, metrics.MessageTypes)

		// Connect a mock client
		mockClient := &Client{
			ID:     "test-1",
			UserID: "user-1",
			send:   make(chan *Message, 256),
		}
		hub.RegisterClient(mockClient)
		time.Sleep(50 * time.Millisecond)

		// Check metrics after connection
		metrics = hub.GetMetrics()
		assert.Equal(t, 1, metrics.CurrentConnections)
		assert.Equal(t, int64(1), metrics.TotalConnections)
		assert.Equal(t, int64(1), metrics.MessagesSent) // Welcome message
		assert.Equal(t, int64(1), metrics.MessageTypes["welcome"])

		// Send a broadcast message
		testMsg := &Message{
			Type: "chat",
			Data: map[string]interface{}{
				"text": "Hello, world!",
			},
		}
		hub.Broadcast(testMsg)
		time.Sleep(50 * time.Millisecond)

		// Check metrics after broadcast
		metrics = hub.GetMetrics()
		assert.Equal(t, int64(1), metrics.MessagesReceived)
		assert.Equal(t, int64(2), metrics.MessagesSent) // Welcome + broadcast
		assert.Equal(t, int64(1), metrics.MessageTypes["chat"])
		assert.Equal(t, int64(1), metrics.MessageTypes["welcome"])

		// Disconnect client
		hub.unregister <- mockClient
		time.Sleep(50 * time.Millisecond)

		// Check metrics after disconnection
		metrics = hub.GetMetrics()
		assert.Equal(t, 0, metrics.CurrentConnections)
		assert.Equal(t, int64(1), metrics.TotalConnections) // Total doesn't decrease
	})

	t.Run("message type counting", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		hub := NewHub(logger)
		go hub.Run()
		defer hub.Shutdown()

		// Wait for hub to start
		time.Sleep(50 * time.Millisecond)

		// Connect mock clients
		clients := make([]*Client, 3)
		for i := 0; i < 3; i++ {
			clients[i] = &Client{
				ID:     fmt.Sprintf("test-%d", i),
				UserID: fmt.Sprintf("user-%d", i),
				send:   make(chan *Message, 256),
			}
			hub.RegisterClient(clients[i])
			time.Sleep(20 * time.Millisecond) // Give time for each registration
		}
		time.Sleep(100 * time.Millisecond)

		// Send various message types
		messageTypes := []string{"chat", "status", "notification", "chat", "status", "chat"}
		for _, msgType := range messageTypes {
			hub.Broadcast(&Message{
				Type: msgType,
				Data: map[string]interface{}{"test": true},
			})
		}
		time.Sleep(100 * time.Millisecond)

		// Check message type counts
		metrics := hub.GetMetrics()
		assert.Equal(t, int64(3), metrics.MessageTypes["chat"])
		assert.Equal(t, int64(2), metrics.MessageTypes["status"])
		assert.Equal(t, int64(1), metrics.MessageTypes["notification"])
		assert.Equal(t, int64(3), metrics.MessageTypes["welcome"]) // 3 clients connected
	})

	t.Run("message rate calculation", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		hub := NewHub(logger)
		go hub.Run()
		defer hub.Shutdown()

		// Wait for hub to start
		time.Sleep(50 * time.Millisecond)

		// Connect a client
		client := &Client{
			ID:     "test-1",
			UserID: "user-1",
			send:   make(chan *Message, 256),
		}
		hub.RegisterClient(client)
		time.Sleep(50 * time.Millisecond)

		// Send messages over time
		for i := 0; i < 10; i++ {
			hub.Broadcast(&Message{
				Type: "test",
				Data: map[string]interface{}{"index": i},
			})
			time.Sleep(100 * time.Millisecond)
		}

		// Calculate message rates
		sentRate, receivedRate := hub.GetMessageRate()
		assert.Greater(t, sentRate, float64(0))
		assert.Greater(t, receivedRate, float64(0))

		// Rates should be roughly equal to messages/second
		metrics := hub.GetMetrics()
		expectedSentRate := float64(metrics.MessagesSent) / metrics.Uptime.Seconds()
		expectedReceivedRate := float64(metrics.MessagesReceived) / metrics.Uptime.Seconds()
		
		assert.InDelta(t, expectedSentRate, sentRate, 0.1)
		assert.InDelta(t, expectedReceivedRate, receivedRate, 0.1)
	})

	t.Run("metrics during shutdown", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		hub := NewHub(logger)
		go hub.Run()

		// Get initial metrics
		metrics := hub.GetMetrics()
		require.NotNil(t, metrics)
		assert.GreaterOrEqual(t, metrics.Uptime, time.Duration(0))

		// Start shutdown
		go hub.Shutdown()
		time.Sleep(50 * time.Millisecond)

		// Metrics should still be accessible during shutdown
		metrics = hub.GetMetrics()
		require.NotNil(t, metrics)
		assert.GreaterOrEqual(t, metrics.Uptime, time.Duration(0))
	})

	t.Run("concurrent metrics access", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		hub := NewHub(logger)
		go hub.Run()
		defer hub.Shutdown()

		// Wait for hub to start
		time.Sleep(50 * time.Millisecond)

		// Connect multiple clients
		for i := 0; i < 5; i++ {
			client := &Client{
				ID:     fmt.Sprintf("test-%d", i),
				UserID: fmt.Sprintf("user-%d", i),
				send:   make(chan *Message, 256),
			}
			hub.RegisterClient(client)
			time.Sleep(10 * time.Millisecond) // Small delay between registrations
		}
		time.Sleep(50 * time.Millisecond)

		// Concurrently access metrics and send messages
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func(idx int) {
				defer func() { done <- true }()
				
				// Get metrics
				metrics := hub.GetMetrics()
				assert.NotNil(t, metrics)
				
				// Send a message
				hub.Broadcast(&Message{
					Type: fmt.Sprintf("type-%d", idx%3),
					Data: map[string]interface{}{"goroutine": idx},
				})
				
				// Get message rates
				sent, received := hub.GetMessageRate()
				assert.GreaterOrEqual(t, sent, float64(0))
				assert.GreaterOrEqual(t, received, float64(0))
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Final metrics check
		metrics := hub.GetMetrics()
		assert.Greater(t, metrics.MessagesSent, int64(0))
		assert.Greater(t, metrics.MessagesReceived, int64(0))
		assert.NotEmpty(t, metrics.MessageTypes)
	})
}

func BenchmarkHubMetrics(b *testing.B) {
	logger := zaptest.NewLogger(b)
	hub := NewHub(logger)
	go hub.Run()
	defer hub.Shutdown()

	// Connect some clients
	for i := 0; i < 10; i++ {
		client := &Client{
			ID:     fmt.Sprintf("bench-%d", i),
			UserID: fmt.Sprintf("user-%d", i),
			send:   make(chan *Message, 256),
		}
		hub.RegisterClient(client)
	}
	time.Sleep(50 * time.Millisecond)

	b.ResetTimer()
	b.Run("GetMetrics", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			metrics := hub.GetMetrics()
			_ = metrics
		}
	})

	b.Run("GetMessageRate", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sent, received := hub.GetMessageRate()
			_ = sent
			_ = received
		}
	})
}