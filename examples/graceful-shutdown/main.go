package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/hub"
	"go.uber.org/zap"
)

// HandlersManager defines all application handlers
type HandlersManager struct {
	Health    *HealthHandler    `url:"/health"`
	WebSocket *WebSocketHandler `url:"/ws" hijack:"ws"`
	LongTask  *LongTaskHandler  `url:"/api/long-task"`
}

// HealthHandler provides health check endpoint
type HealthHandler struct {
	requestCount atomic.Int64
}

func (h *HealthHandler) GET(c echo.Context) error {
	count := h.requestCount.Add(1)
	return c.JSON(200, map[string]interface{}{
		"status":        "healthy",
		"time":          time.Now().Format(time.RFC3339),
		"requestCount":  count,
	})
}

// WebSocketHandler manages WebSocket connections
type WebSocketHandler struct {
	Hub    *hub.Hub
	Logger *zap.Logger
}

func (h *WebSocketHandler) HandleConnection(c echo.Context) error {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		h.Logger.Error("Failed to upgrade connection", zap.Error(err))
		return err
	}

	// Extract user ID from query params (demo purposes)
	userID := c.QueryParam("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	client := hub.NewClient(h.Hub, conn, userID, h.Logger)
	h.Hub.RegisterClient(client)

	go client.WritePump()
	go client.ReadPump()

	return nil
}

// LongTaskHandler simulates a long-running task
type LongTaskHandler struct {
	Logger       *zap.Logger
	activeTasks  atomic.Int32
}

func (h *LongTaskHandler) POST(c echo.Context) error {
	h.activeTasks.Add(1)
	defer h.activeTasks.Add(-1)

	h.Logger.Info("Starting long task", zap.Int32("active_tasks", h.activeTasks.Load()))

	// Simulate long-running work that respects context cancellation
	ctx := c.Request().Context()
	
	select {
	case <-time.After(5 * time.Second):
		h.Logger.Info("Long task completed")
		return c.JSON(200, map[string]string{
			"status": "completed",
			"result": "Task finished successfully",
		})
	case <-ctx.Done():
		h.Logger.Warn("Long task cancelled")
		return c.JSON(499, map[string]string{
			"status": "cancelled",
			"error":  "Client disconnected or server shutting down",
		})
	}
}

func (h *LongTaskHandler) GetActiveTasks() int32 {
	return h.activeTasks.Load()
}

// MockDatabase simulates a database connection that needs graceful shutdown
type MockDatabase struct {
	logger      *zap.Logger
	connections atomic.Int32
}

func NewMockDatabase(logger *zap.Logger) *MockDatabase {
	return &MockDatabase{logger: logger}
}

func (db *MockDatabase) Connect() error {
	db.connections.Add(1)
	db.logger.Info("Database connected", zap.Int32("connections", db.connections.Load()))
	return nil
}

func (db *MockDatabase) Close(ctx context.Context) error {
	db.logger.Info("Closing database connections...")
	
	// Simulate graceful connection closure
	select {
	case <-time.After(500 * time.Millisecond):
		db.connections.Store(0)
		db.logger.Info("Database connections closed")
		return nil
	case <-ctx.Done():
		db.logger.Error("Database shutdown timeout")
		return ctx.Err()
	}
}

// BackgroundWorker simulates a background job processor
type BackgroundWorker struct {
	logger   *zap.Logger
	stopChan chan struct{}
	done     chan struct{}
}

func NewBackgroundWorker(logger *zap.Logger) *BackgroundWorker {
	return &BackgroundWorker{
		logger:   logger,
		stopChan: make(chan struct{}),
		done:     make(chan struct{}),
	}
}

func (w *BackgroundWorker) Start() {
	go func() {
		defer close(w.done)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.logger.Info("Background worker processing...")
			case <-w.stopChan:
				w.logger.Info("Background worker stopping...")
				return
			}
		}
	}()
}

func (w *BackgroundWorker) Stop(ctx context.Context) error {
	close(w.stopChan)
	
	select {
	case <-w.done:
		w.logger.Info("Background worker stopped gracefully")
		return nil
	case <-ctx.Done():
		w.logger.Error("Background worker shutdown timeout")
		return ctx.Err()
	}
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create configuration
	cfg := &app.Config{}
	cfg.Server.Address = ":8080"
	cfg.Server.Recovery = true
	cfg.Server.CORS = true

	// Initialize components
	wsHub := hub.NewHub(logger)
	database := NewMockDatabase(logger)
	worker := NewBackgroundWorker(logger)
	
	// Create handlers
	longTaskHandler := &LongTaskHandler{Logger: logger}
	handlers := &HandlersManager{
		Health: &HealthHandler{},
		WebSocket: &WebSocketHandler{
			Hub:    wsHub,
			Logger: logger,
		},
		LongTask: longTaskHandler,
	}

	// Create application with custom shutdown timeout
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
		app.WithShutdownTimeout(15 * time.Second), // Extended timeout for demo
	)
	if err != nil {
		log.Fatal(err)
	}

	// Register services in DI container
	app.Register(application.Context(), logger)
	app.Register(application.Context(), wsHub)
	app.Register(application.Context(), database)

	// Start services
	if err := database.Connect(); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	
	go wsHub.Run()
	worker.Start()

	// Register shutdown hooks in priority order
	
	// 1. Stop accepting new requests (implicit in Echo shutdown)
	
	// 2. Wait for active requests to complete
	application.OnShutdown(func(ctx context.Context) error {
		logger.Info("Waiting for active tasks to complete...")
		
		// Wait up to 5 seconds for active tasks
		timeout := time.After(5 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				if longTaskHandler.GetActiveTasks() == 0 {
					logger.Info("All active tasks completed")
					return nil
				}
			case <-timeout:
				count := longTaskHandler.GetActiveTasks()
				logger.Warn("Timeout waiting for tasks", zap.Int32("remaining", count))
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})
	
	// 3. Shutdown WebSocket connections gracefully
	application.OnShutdown(func(ctx context.Context) error {
		logger.Info("Shutting down WebSocket hub...")
		
		// Calculate remaining time from context
		deadline, ok := ctx.Deadline()
		if !ok {
			return wsHub.ShutdownWithTimeout(5 * time.Second)
		}
		
		timeout := time.Until(deadline)
		if timeout < time.Second {
			timeout = time.Second
		}
		
		return wsHub.ShutdownWithTimeout(timeout)
	})
	
	// 4. Stop background workers
	application.OnShutdown(func(ctx context.Context) error {
		logger.Info("Stopping background worker...")
		return worker.Stop(ctx)
	})
	
	// 5. Close database connections
	application.OnShutdown(func(ctx context.Context) error {
		logger.Info("Closing database connections...")
		return database.Close(ctx)
	})
	
	// 6. Final cleanup
	application.OnShutdown(func(ctx context.Context) error {
		logger.Info("Performing final cleanup...")
		// Add any final cleanup tasks here
		time.Sleep(200 * time.Millisecond) // Simulate cleanup
		logger.Info("Cleanup completed")
		return nil
	})

	// Setup graceful shutdown signals
	ctx, stop := signal.NotifyContext(context.Background(), 
		os.Interrupt,    // Ctrl+C
		syscall.SIGTERM, // Kubernetes/Docker stop
		syscall.SIGQUIT, // Quit
	)
	defer stop()

	// Start server
	go func() {
		logger.Info("Starting server", 
			zap.String("address", cfg.Server.Address),
			zap.Duration("shutdown_timeout", 15 * time.Second),
		)
		if err := application.Run(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	// Periodic status broadcaster (demo feature)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				status := &hub.Message{
					Type: "status",
					Data: map[string]interface{}{
						"connected_clients": wsHub.GetConnectedClients(),
						"server_time":       time.Now().Format(time.RFC3339),
						"uptime":            time.Since(time.Now()).String(),
					},
				}
				wsHub.Broadcast(status)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	// Log shutdown reason
	logger.Info("Shutdown signal received", zap.String("signal", ctx.Err().Error()))

	// Perform graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	start := time.Now()
	if err := application.Shutdown(shutdownCtx); err != nil {
		logger.Error("Shutdown error", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("Server stopped gracefully", 
		zap.Duration("shutdown_duration", time.Since(start)),
	)
}