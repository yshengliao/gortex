package app

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	gorillaWS "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gortexContext "github.com/yshengliao/gortex/transport/http"
	"github.com/yshengliao/gortex/transport/websocket"
	// Hub is in websocket package, not a subpackage
	"go.uber.org/zap/zaptest"
)

type TestHandlers struct {
	Health    *TestHealthHandler    `url:"/health"`
	WebSocket *TestWebSocketHandler `url:"/ws" hijack:"ws"`
}

type TestHealthHandler struct{}

func (h *TestHealthHandler) GET(c gortexContext.Context) error {
	return c.JSON(200, map[string]string{"status": "ok"})
}

type TestWebSocketHandler struct {
	Hub    *websocket.Hub
	Logger *testing.T
}

func (h *TestWebSocketHandler) HandleConnection(c gortexContext.Context) error {
	upgrader := gorillaWS.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	testLogger := zaptest.NewLogger(h.Logger)
	client := websocket.NewClient(h.Hub, conn, "test-user", testLogger)
	h.Hub.RegisterClient(client)

	go client.WritePump()
	go client.ReadPump()

	return nil
}

func TestIntegrationGracefulShutdown(t *testing.T) {
	t.Run("full graceful shutdown flow", func(t *testing.T) {
		logger := zaptest.NewLogger(t)

		// Create WebSocket hub
		wsHub := websocket.NewHub(logger)
		go wsHub.Run()

		// Create config
		cfg := &Config{}
		cfg.Server.Address = ":18080" // Fixed port for testing

		// Create handlers
		handlers := &TestHandlers{
			Health: &TestHealthHandler{},
			WebSocket: &TestWebSocketHandler{
				Hub:    wsHub,
				Logger: t,
			},
		}

		// Create app with shutdown timeout
		app, err := NewApp(
			WithConfig(cfg),
			WithLogger(logger),
			WithHandlers(handlers),
			WithShutdownTimeout(5*time.Second),
		)
		require.NoError(t, err)

		// Register shutdown hooks
		var hubShutdown bool
		app.OnShutdown(func(ctx context.Context) error {
			hubShutdown = true
			return wsHub.ShutdownWithTimeout(2 * time.Second)
		})

		var cleanupDone bool
		app.OnShutdown(func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond)
			cleanupDone = true
			return nil
		})

		// Start server
		go func() {
			if err := app.Run(); err != nil && err != http.ErrServerClosed {
				t.Errorf("Server error: %v", err)
			}
		}()

		// Wait for server to start
		time.Sleep(100 * time.Millisecond)

		// Get server address
		// For Gortex, we need to get the address from the server after it starts
		// This is a temporary workaround - in production you'd use the configured address
		serverURL := "http://localhost" + cfg.Server.Address

		// Test health endpoint
		resp, err := http.Get(serverURL + "/health")
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		resp.Body.Close()

		// Connect WebSocket client
		wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + "/ws"
		conn, _, err := gorillaWS.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer conn.Close()

		// Read welcome message
		var msg websocket.Message
		err = conn.ReadJSON(&msg)
		require.NoError(t, err)
		assert.Equal(t, "welcome", msg.Type)

		// Verify client is connected
		assert.Equal(t, 1, wsHub.GetConnectedClients())

		// Start graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// Monitor for close message in background
		closeReceived := make(chan bool, 1)
		go func() {
			conn.SetCloseHandler(func(code int, text string) error {
				closeReceived <- true
				return nil
			})

			// Keep reading to process close frame
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					// Check if it's a close error
					if gorillaWS.IsCloseError(err, gorillaWS.CloseGoingAway, gorillaWS.CloseNormalClosure) {
						closeReceived <- true
					}
					return
				}
			}
		}()

		// Perform shutdown
		err = app.Shutdown(shutdownCtx)
		assert.NoError(t, err)

		// Verify shutdown hooks were called
		assert.True(t, hubShutdown, "Hub shutdown hook should have been called")
		assert.True(t, cleanupDone, "Cleanup hook should have been called")

		// Verify WebSocket client received close message
		select {
		case <-closeReceived:
			// Good, client was notified
		case <-time.After(1 * time.Second):
			t.Error("WebSocket client did not receive close message")
		}

		// Verify no clients remain
		assert.Equal(t, 0, wsHub.GetConnectedClients())

		// Verify server is stopped (connection should fail)
		_, err = http.Get(serverURL + "/health")
		assert.Error(t, err)
	})

	// Skip this complex test for now - method definitions inside functions are not allowed
	t.Run("shutdown timeout handling", func(t *testing.T) {
		logger := zaptest.NewLogger(t)

		// Create app with short timeout
		app, err := NewApp(
			WithLogger(logger),
			WithShutdownTimeout(100*time.Millisecond),
		)
		require.NoError(t, err)

		// Register a slow shutdown hook
		app.OnShutdown(func(ctx context.Context) error {
			select {
			case <-time.After(500 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		// Perform shutdown
		start := time.Now()
		err = app.Shutdown(context.Background())
		duration := time.Since(start)

		// Should timeout
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timed out")
		assert.Less(t, duration, 200*time.Millisecond)
	})
}
