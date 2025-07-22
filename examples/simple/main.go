package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/app"
	"go.uber.org/zap"
)

// HandlersManager defines all handlers with declarative routing
type HandlersManager struct {
	Home   *HomeHandler   `url:"/"`
	Health *HealthHandler `url:"/health"`
	API    *APIHandler    `url:"/api"`
}

// HomeHandler serves the root endpoint
type HomeHandler struct{}

func (h *HomeHandler) GET(c echo.Context) error {
	return c.JSON(200, map[string]string{
		"message": "Welcome to Gortex Framework",
		"version": "1.0.0",
	})
}

// HealthHandler provides health check endpoint
type HealthHandler struct{}

func (h *HealthHandler) GET(c echo.Context) error {
	return c.JSON(200, map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// APIHandler demonstrates API endpoints
type APIHandler struct {
	Logger *zap.Logger
}

// GET /api
func (h *APIHandler) GET(c echo.Context) error {
	h.Logger.Info("API endpoint called")
	return c.JSON(200, map[string]string{
		"message": "API endpoint",
		"method":  "GET",
	})
}

// POST /api/echo
func (h *APIHandler) Echo(c echo.Context) error {
	var body map[string]any
	if err := c.Bind(&body); err != nil {
		return c.JSON(400, map[string]string{"error": "Invalid JSON"})
	}
	h.Logger.Info("Echo endpoint called", zap.Any("body", body))
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

	// Create handlers
	handlers := &HandlersManager{
		Home:   &HomeHandler{},
		Health: &HealthHandler{},
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

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server
	go func() {
		logger.Info("Starting server", zap.String("address", cfg.Server.Address))
		if err := application.Run(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()

	// Graceful shutdown
	logger.Info("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := application.Shutdown(shutdownCtx); err != nil {
		logger.Error("Shutdown error", zap.Error(err))
	}

	logger.Info("Server stopped")
}