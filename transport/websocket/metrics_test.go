package websocket

import (
	"fmt"
	"sync"
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

		// Connect a mock client — RegisterClient now blocks until the hub
		// has recorded the client, so we can assert immediately afterwards.
		mockClient := &Client{
			ID:     "test-1",
			UserID: "user-1",
			send:   make(chan *Message, 256),
		}
		hub.RegisterClient(mockClient)

		metrics = hub.GetMetrics()
		assert.Equal(t, 1, metrics.CurrentConnections)
		assert.Equal(t, int64(1), metrics.TotalConnections)
		assert.Equal(t, int64(1), metrics.MessagesSent) // Welcome message
		assert.Equal(t, int64(1), metrics.MessageTypes["welcome"])

		// Synchronous broadcast: waits for the hub to process the message
		// before returning, so the subsequent GetMetrics observes the
		// updated counters without a time-based sleep.
		hub.broadcastSync(&Message{
			Type: "chat",
			Data: map[string]any{"text": "Hello, world!"},
		})

		metrics = hub.GetMetrics()
		assert.Equal(t, int64(1), metrics.MessagesReceived)
		assert.Equal(t, int64(2), metrics.MessagesSent) // Welcome + broadcast
		assert.Equal(t, int64(1), metrics.MessageTypes["chat"])
		assert.Equal(t, int64(1), metrics.MessageTypes["welcome"])

		hub.UnregisterClient(mockClient)

		metrics = hub.GetMetrics()
		assert.Equal(t, 0, metrics.CurrentConnections)
		assert.Equal(t, int64(1), metrics.TotalConnections) // Total doesn't decrease
	})

	t.Run("message type counting", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		hub := NewHub(logger)
		go hub.Run()
		defer hub.Shutdown()

		clients := make([]*Client, 3)
		for i := 0; i < 3; i++ {
			clients[i] = &Client{
				ID:     fmt.Sprintf("test-%d", i),
				UserID: fmt.Sprintf("user-%d", i),
				send:   make(chan *Message, 256),
			}
			hub.RegisterClient(clients[i])
		}

		messageTypes := []string{"chat", "status", "notification", "chat", "status", "chat"}
		for _, msgType := range messageTypes {
			hub.broadcastSync(&Message{
				Type: msgType,
				Data: map[string]any{"test": true},
			})
		}

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

		client := &Client{
			ID:     "test-1",
			UserID: "user-1",
			send:   make(chan *Message, 256),
		}
		hub.RegisterClient(client)

		for i := 0; i < 10; i++ {
			hub.broadcastSync(&Message{
				Type: "test",
				Data: map[string]any{"index": i},
			})
		}

		// Rate is "messages per second since hub start". We assert the
		// rate is positive (the hub is producing traffic) rather than
		// reconstructing it from a second GetMetrics call, since the
		// denominator (Uptime) keeps moving between calls.
		sentRate, receivedRate := hub.GetMessageRate()
		assert.Greater(t, sentRate, float64(0))
		assert.Greater(t, receivedRate, float64(0))

		metrics := hub.GetMetrics()
		assert.GreaterOrEqual(t, metrics.MessagesSent, int64(11)) // welcome + 10 broadcasts
		assert.Equal(t, int64(10), metrics.MessagesReceived)
	})

	t.Run("metrics during shutdown", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		hub := NewHub(logger)
		go hub.Run()

		metrics := hub.GetMetrics()
		require.NotNil(t, metrics)
		assert.GreaterOrEqual(t, metrics.Uptime, time.Duration(0))

		// Close shutdown channel without blocking on shutdownDone — we want
		// GetMetrics to observe the shutdown state whilst Run() is still
		// draining.
		done := make(chan struct{})
		go func() {
			hub.Shutdown()
			close(done)
		}()

		// GetMetrics must remain safe to call regardless of shutdown state.
		metrics = hub.GetMetrics()
		require.NotNil(t, metrics)
		assert.GreaterOrEqual(t, metrics.Uptime, time.Duration(0))
		<-done
	})

	t.Run("concurrent metrics access", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		hub := NewHub(logger)
		go hub.Run()
		defer hub.Shutdown()

		for i := 0; i < 5; i++ {
			client := &Client{
				ID:     fmt.Sprintf("test-%d", i),
				UserID: fmt.Sprintf("user-%d", i),
				send:   make(chan *Message, 256),
			}
			hub.RegisterClient(client)
		}

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				metrics := hub.GetMetrics()
				assert.NotNil(t, metrics)

				hub.broadcastSync(&Message{
					Type: fmt.Sprintf("type-%d", idx%3),
					Data: map[string]any{"goroutine": idx},
				})

				sent, received := hub.GetMessageRate()
				assert.GreaterOrEqual(t, sent, float64(0))
				assert.GreaterOrEqual(t, received, float64(0))
			}(i)
		}
		wg.Wait()

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

	for i := 0; i < 10; i++ {
		client := &Client{
			ID:     fmt.Sprintf("bench-%d", i),
			UserID: fmt.Sprintf("user-%d", i),
			send:   make(chan *Message, 256),
		}
		hub.RegisterClient(client)
	}

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
