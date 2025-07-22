package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/response"
	"go.uber.org/zap"
)

// HandlersManager defines application handlers
type HandlersManager struct {
	API     *APIHandlers     `url:"/api"`
	Example *ExampleHandlers `url:"/example"`
}

// APIHandlers provides API endpoints
type APIHandlers struct {
	Logger *zap.Logger
}

func (h *APIHandlers) GET(c echo.Context) error {
	return response.Success(c, http.StatusOK, map[string]string{
		"message": "API root endpoint",
		"version": "1.0.0",
	})
}

func (h *APIHandlers) Users(c echo.Context) error {
	// Simulate getting users
	users := []map[string]interface{}{
		{"id": 1, "name": "Alice", "email": "alice@example.com"},
		{"id": 2, "name": "Bob", "email": "bob@example.com"},
	}

	h.Logger.Info("Fetching users", zap.Int("count", len(users)))
	
	return response.Success(c, http.StatusOK, users)
}

// ExampleHandlers demonstrates different response types
type ExampleHandlers struct {
	Logger *zap.Logger
}

func (h *ExampleHandlers) Error(c echo.Context) error {
	// This will trigger an error response
	return fmt.Errorf("example error for development testing")
}

func (h *ExampleHandlers) Validation(c echo.Context) error {
	// Simulate validation error
	return response.Error(c, http.StatusBadRequest, "Validation failed", map[string]interface{}{
		"fields": map[string]string{
			"username": "required",
			"email":    "invalid format",
		},
	})
}

func (h *ExampleHandlers) Panic(c echo.Context) error {
	// This will trigger panic recovery
	panic("example panic for development testing")
}

func (h *ExampleHandlers) Slow(c echo.Context) error {
	// Simulate slow endpoint
	duration := c.QueryParam("duration")
	if duration == "" {
		duration = "2s"
	}

	d, err := time.ParseDuration(duration)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "Invalid duration format", nil)
	}

	h.Logger.Info("Starting slow operation", zap.Duration("duration", d))
	time.Sleep(d)

	return response.Success(c, http.StatusOK, map[string]interface{}{
		"message":  "Slow operation completed",
		"duration": d.String(),
	})
}

func main() {
	// Initialize logger in development mode
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create configuration with debug level to enable development mode
	cfg := &app.Config{}
	cfg.Server.Address = ":8080"
	cfg.Logger.Level = "debug" // This enables development mode

	// Create handlers
	handlers := &HandlersManager{
		API: &APIHandlers{
			Logger: logger,
		},
		Example: &ExampleHandlers{
			Logger: logger,
		},
	}

	// Create application with development mode
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
		app.WithDevelopmentMode(true),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Start server
	go func() {
		logger.Info("Starting development mode example", 
			zap.String("address", cfg.Server.Address),
		)
		
		logger.Info("Development endpoints available:",
			zap.String("routes", "GET /_routes - List all registered routes"),
			zap.String("error", "GET /_error - Test error pages"),
			zap.String("config", "GET /_config - View configuration"),
			zap.String("monitor", "GET /_monitor - System monitoring dashboard"),
		)
		
		logger.Info("Example endpoints:",
			zap.String("api", "GET /api - API root"),
			zap.String("users", "POST /api/users - Get users list"),
			zap.String("error", "POST /example/error - Trigger error"),
			zap.String("validation", "POST /example/validation - Trigger validation error"),
			zap.String("panic", "POST /example/panic - Trigger panic"),
			zap.String("slow", "POST /example/slow?duration=5s - Slow endpoint"),
		)

		logger.Info("Try these commands:",
			zap.String("cmd1", "curl http://localhost:8080/_routes | jq"),
			zap.String("cmd2", "curl -X POST http://localhost:8080/api/users | jq"),
			zap.String("cmd3", "curl -X POST http://localhost:8080/example/error"),
			zap.String("cmd4", "curl 'http://localhost:8080/_error?type=panic'"),
			zap.String("cmd5", "curl http://localhost:8080/_monitor | jq"),
			zap.String("cmd6", "Open http://localhost:8080/example/error in browser for HTML error page"),
		)
		
		if err := application.Run(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
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