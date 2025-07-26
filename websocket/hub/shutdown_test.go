package hub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestHubShutdown(t *testing.T) {
	t.Run("graceful shutdown sends close messages", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		hub := NewHub(logger)
		
		// Start hub
		go hub.Run()
		time.Sleep(10 * time.Millisecond) // Let hub start
		
		// Create test WebSocket server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			upgrader := &websocket.Upgrader{
				CheckOrigin: func(r *http.Request) bool {
					return true
				},
			}
			conn, err := upgrader.Upgrade(w, r, nil)
			require.NoError(t, err)
			defer conn.Close()
			
			client := NewClient(hub, conn, "test-user", logger)
			hub.RegisterClient(client)
			
			// Start client pumps in goroutines
			go client.WritePump()
			client.ReadPump() // This blocks until connection closes
		}))
		defer server.Close()
		
		// Connect a client
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer conn.Close()
		
		// Wait for welcome message
		var welcomeMsg Message
		err = conn.ReadJSON(&welcomeMsg)
		require.NoError(t, err)
		assert.Equal(t, "welcome", welcomeMsg.Type)
		
		// Verify client is connected
		assert.Equal(t, 1, hub.GetConnectedClients())
		
		// Start shutdown with timeout
		done := make(chan error, 1)
		go func() {
			done <- hub.ShutdownWithTimeout(2 * time.Second)
		}()
		
		// Read messages until we get the close message
		var closeReceived bool
		for !closeReceived {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			
			if msgType == websocket.CloseMessage {
				closeReceived = true
				closeCode := websocket.CloseGoingAway
				closeText := "Server shutting down"
				
				// Parse close message
				if len(msg) >= 2 {
					closeCode = int(msg[0])<<8 | int(msg[1])
					if len(msg) > 2 {
						closeText = string(msg[2:])
					}
				}
				
				assert.Equal(t, websocket.CloseGoingAway, closeCode)
				assert.Contains(t, closeText, "shutting down")
			}
		}
		
		assert.True(t, closeReceived, "Should have received close message")
		
		// Wait for shutdown to complete
		err = <-done
		assert.NoError(t, err)
		
		// Verify no clients remain
		assert.Equal(t, 0, hub.GetConnectedClients())
	})
	
	t.Run("shutdown timeout works", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		hub := NewHub(logger)
		
		// Don't start the hub's Run method - this simulates a stuck hub
		
		start := time.Now()
		err := hub.ShutdownWithTimeout(100 * time.Millisecond)
		duration := time.Since(start)
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timed out")
		assert.Less(t, duration, 200*time.Millisecond)
	})
	
	t.Run("multiple clients receive shutdown message", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		hub := NewHub(logger)
		
		// Start hub
		go hub.Run()
		time.Sleep(10 * time.Millisecond)
		
		// Track close messages received
		var closeCount atomic.Int32
		var wg sync.WaitGroup
		
		// Create multiple clients
		numClients := 3
		for i := 0; i < numClients; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				// Create test WebSocket server for this client
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					upgrader := &websocket.Upgrader{
				CheckOrigin: func(r *http.Request) bool {
					return true
				},
			}
			conn, err := upgrader.Upgrade(w, r, nil)
					require.NoError(t, err)
					defer conn.Close()
					
					client := NewClient(hub, conn, "test-user", logger)
					hub.RegisterClient(client)
					
					go client.WritePump()
					client.ReadPump()
				}))
				defer server.Close()
				
				// Connect
				wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
				conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
				require.NoError(t, err)
				defer conn.Close()
				
				// Read messages
				for {
					msgType, _, err := conn.ReadMessage()
					if err != nil {
						break
					}
					
					if msgType == websocket.CloseMessage {
						closeCount.Add(1)
						break
					}
				}
			}(i)
		}
		
		// Wait for all clients to connect
		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, numClients, hub.GetConnectedClients())
		
		// Shutdown hub
		err := hub.ShutdownWithTimeout(2 * time.Second)
		assert.NoError(t, err)
		
		// Wait for all clients to finish
		wg.Wait()
		
		// All clients should have received close message
		assert.Equal(t, int32(numClients), closeCount.Load())
	})
	
	t.Run("broadcast shutdown notification", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		hub := NewHub(logger)
		
		// Start hub
		go hub.Run()
		time.Sleep(10 * time.Millisecond)
		
		// Create test WebSocket server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			upgrader := &websocket.Upgrader{
				CheckOrigin: func(r *http.Request) bool {
					return true
				},
			}
			conn, err := upgrader.Upgrade(w, r, nil)
			require.NoError(t, err)
			defer conn.Close()
			
			client := NewClient(hub, conn, "test-user", logger)
			hub.RegisterClient(client)
			
			go client.WritePump()
			client.ReadPump()
		}))
		defer server.Close()
		
		// Connect a client
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer conn.Close()
		
		// Start reading messages
		msgChan := make(chan Message, 10)
		go func() {
			for {
				var msg Message
				if err := conn.ReadJSON(&msg); err != nil {
					return
				}
				msgChan <- msg
			}
		}()
		
		// Wait for welcome message
		select {
		case msg := <-msgChan:
			assert.Equal(t, "welcome", msg.Type)
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for welcome message")
		}
		
		// Start shutdown
		go hub.ShutdownWithTimeout(2 * time.Second)
		
		// Should receive server_shutdown message
		select {
		case msg := <-msgChan:
			assert.Equal(t, "server_shutdown", msg.Type)
			assert.Equal(t, "Server is shutting down", msg.Data["message"])
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for shutdown message")
		}
	})
}

func TestHubShutdownWithContext(t *testing.T) {
	t.Run("hub shutdown can be integrated with app shutdown", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		hub := NewHub(logger)
		
		// Start hub
		go hub.Run()
		time.Sleep(10 * time.Millisecond)
		
		// Simulate app shutdown hook
		shutdownHook := func(ctx context.Context) error {
			// Use remaining context time for hub shutdown
			deadline, ok := ctx.Deadline()
			if !ok {
				return hub.ShutdownWithTimeout(5 * time.Second)
			}
			
			timeout := time.Until(deadline)
			if timeout <= 0 {
				timeout = 1 * time.Second
			}
			
			return hub.ShutdownWithTimeout(timeout)
		}
		
		// Execute hook with context
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		
		err := shutdownHook(ctx)
		assert.NoError(t, err)
		
		// Verify hub is shut down
		assert.Equal(t, 0, hub.GetConnectedClients())
	})
}