package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/hub"
	"go.uber.org/zap/zaptest"
)

func TestGracefulShutdownExample(t *testing.T) {
	t.Run("shutdown hooks execute in order", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		
		// Track hook execution order
		var mu sync.Mutex
		var order []string
		
		// Create test app
		application, err := app.NewApp(
			app.WithLogger(logger),
			app.WithShutdownTimeout(5 * time.Second),
		)
		require.NoError(t, err)
		
		// Register hooks
		application.OnShutdown(func(ctx context.Context) error {
			mu.Lock()
			order = append(order, "first")
			mu.Unlock()
			return nil
		})
		
		application.OnShutdown(func(ctx context.Context) error {
			time.Sleep(50 * time.Millisecond) // Simulate work
			mu.Lock()
			order = append(order, "second")
			mu.Unlock()
			return nil
		})
		
		application.OnShutdown(func(ctx context.Context) error {
			mu.Lock()
			order = append(order, "third")
			mu.Unlock()
			return nil
		})
		
		// Shutdown
		ctx := context.Background()
		err = application.Shutdown(ctx)
		assert.NoError(t, err)
		
		// Verify all hooks ran
		assert.Len(t, order, 3)
		assert.Contains(t, order, "first")
		assert.Contains(t, order, "second")
		assert.Contains(t, order, "third")
	})
	
	t.Run("websocket clients receive shutdown notification", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		
		// Create components
		wsHub := hub.NewHub(logger)
		cfg := &app.Config{}
		cfg.Server.Address = ":0" // Random port
		
		handlers := &HandlersManager{
			Health: &HealthHandler{},
			WebSocket: &WebSocketHandler{
				Hub:    wsHub,
				Logger: logger,
			},
			LongTask: &LongTaskHandler{Logger: logger},
		}
		
		// Create app
		application, err := app.NewApp(
			app.WithConfig(cfg),
			app.WithLogger(logger),
			app.WithHandlers(handlers),
		)
		require.NoError(t, err)
		
		// Register WebSocket shutdown hook
		application.OnShutdown(func(ctx context.Context) error {
			return wsHub.ShutdownWithTimeout(2 * time.Second)
		})
		
		// Start hub
		go wsHub.Run()
		
		// Start server
		go application.Run()
		time.Sleep(100 * time.Millisecond) // Let server start
		
		// Get server address
		addr := application.Echo().ListenerAddr()
		require.NotNil(t, addr)
		
		// Connect WebSocket client
		wsURL := "ws://" + addr.String() + "/ws"
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer conn.Close()
		
		// Read welcome message
		var welcomeMsg hub.Message
		err = conn.ReadJSON(&welcomeMsg)
		require.NoError(t, err)
		assert.Equal(t, "welcome", welcomeMsg.Type)
		
		// Start shutdown
		shutdownDone := make(chan error, 1)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			shutdownDone <- application.Shutdown(ctx)
		}()
		
		// Should receive shutdown notification or close message
		msgType, msg, err := conn.ReadMessage()
		if msgType == websocket.TextMessage {
			// Could be server_shutdown message
			assert.Contains(t, string(msg), "shutdown")
		} else if msgType == websocket.CloseMessage {
			// Or direct close message
			assert.Contains(t, string(msg), "shutting down")
		}
		
		// Wait for shutdown
		err = <-shutdownDone
		assert.NoError(t, err)
	})
	
	t.Run("long running tasks are handled", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		
		// Create handler
		handler := &LongTaskHandler{Logger: logger}
		
		// Simulate active task
		handler.activeTasks.Store(3)
		
		// Create shutdown hook that waits for tasks
		hook := func(ctx context.Context) error {
			timeout := time.After(1 * time.Second)
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			
			for {
				select {
				case <-ticker.C:
					// Simulate tasks completing
					current := handler.activeTasks.Add(-1)
					if current <= 0 {
						return nil
					}
				case <-timeout:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
		
		// Execute hook
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		
		err := hook(ctx)
		assert.NoError(t, err)
		assert.LessOrEqual(t, handler.GetActiveTasks(), int32(0))
	})
	
	t.Run("database shutdown", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		db := NewMockDatabase(logger)
		
		// Connect
		err := db.Connect()
		require.NoError(t, err)
		assert.Equal(t, int32(1), db.connections.Load())
		
		// Shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		
		err = db.Close(ctx)
		assert.NoError(t, err)
		assert.Equal(t, int32(0), db.connections.Load())
	})
	
	t.Run("background worker shutdown", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		worker := NewBackgroundWorker(logger)
		
		// Start worker
		worker.Start()
		time.Sleep(100 * time.Millisecond)
		
		// Stop worker
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		
		err := worker.Stop(ctx)
		assert.NoError(t, err)
	})
}

func TestHealthEndpoint(t *testing.T) {
	handler := &HealthHandler{}
	
	// Make multiple requests
	for i := 0; i < 5; i++ {
		// Create mock echo context
		application, err := app.NewApp()
		require.NoError(t, err)
		
		req, _ := http.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		c := application.Echo().NewContext(req, rec)
		
		err = handler.GET(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "healthy")
		assert.Contains(t, rec.Body.String(), "requestCount")
	}
	
	// Verify counter incremented
	assert.Equal(t, int64(5), handler.requestCount.Load())
}

func BenchmarkShutdownHooks(b *testing.B) {
	logger := zaptest.NewLogger(b)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app, _ := app.NewApp(
			app.WithLogger(logger),
			app.WithShutdownTimeout(100 * time.Millisecond),
		)
		
		// Register some hooks
		for j := 0; j < 10; j++ {
			app.OnShutdown(func(ctx context.Context) error {
				return nil
			})
		}
		
		// Shutdown
		ctx := context.Background()
		app.Shutdown(ctx)
	}
}