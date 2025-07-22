package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/hub"
	"go.uber.org/zap"
)

// Define your handlers using struct tags for declarative routing

type HandlersManager struct {
	Default   *DefaultHandler   `url:"/"`
	Health    *HealthHandler    `url:"/health"`
	WebSocket *WebSocketHandler `url:"/ws" hijack:"ws"`
	API       *APIHandler       `url:"/api"`
}

// Simple handlers

type DefaultHandler struct{}

func (h *DefaultHandler) GET(c echo.Context) error {
	return c.JSON(200, map[string]string{
		"message": "Welcome to Gortex Framework",
		"version": "1.0.0",
	})
}

type HealthHandler struct{}

func (h *HealthHandler) GET(c echo.Context) error {
	return c.JSON(200, map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

type WebSocketHandler struct {
	Hub    *hub.Hub
	Logger *zap.Logger
}

func (h *WebSocketHandler) HandleConnection(c echo.Context) error {
	// Create WebSocket upgrader
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Configure appropriately for production
		},
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		h.Logger.Error("Failed to upgrade connection", zap.Error(err))
		return err
	}

	// Create client
	client := hub.NewClient(h.Hub, conn, "anonymous", h.Logger)
	h.Hub.RegisterClient(client)

	// Start client message pumps
	go client.WritePump()
	go client.ReadPump()

	return nil
}

type APIHandler struct {
	Logger *zap.Logger
}

// GET /api
func (h *APIHandler) GET(c echo.Context) error {
	return c.JSON(200, map[string]string{
		"message": "API endpoint",
	})
}

// POST /api/echo
func (h *APIHandler) Echo(c echo.Context) error {
	var body map[string]interface{}
	if err := c.Bind(&body); err != nil {
		return c.JSON(400, map[string]string{"error": "Invalid JSON"})
	}
	return c.JSON(200, body)
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

	// Create WebSocket hub
	wsHub := hub.NewHub(logger)

	// Create handlers
	handlers := &HandlersManager{
		Default: &DefaultHandler{},
		Health:  &HealthHandler{},
		WebSocket: &WebSocketHandler{
			Hub:    wsHub,
			Logger: logger,
		},
		API: &APIHandler{
			Logger: logger,
		},
	}

	// Create application
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Register services in DI container
	app.Register(application.Context(), logger)
	app.Register(application.Context(), wsHub)

	// Start WebSocket hub
	go wsHub.Run()

	// Register shutdown hooks
	application.OnShutdown(func(ctx context.Context) error {
		logger.Info("Running WebSocket hub shutdown hook")
		return wsHub.ShutdownWithTimeout(5 * time.Second)
	})

	application.OnShutdown(func(ctx context.Context) error {
		logger.Info("Running cleanup tasks")
		// Add any cleanup tasks here (e.g., closing database connections)
		time.Sleep(100 * time.Millisecond) // Simulate cleanup work
		logger.Info("Cleanup completed")
		return nil
	})

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server in goroutine
	go func() {
		logger.Info("Starting server", zap.String("address", cfg.Server.Address))
		if err := application.Run(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()

	// Shutdown with timeout
	logger.Info("Shutdown signal received, starting graceful shutdown...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := application.Shutdown(shutdownCtx); err != nil {
		logger.Error("Shutdown error", zap.Error(err))
	}

	logger.Info("Server stopped gracefully")
}
