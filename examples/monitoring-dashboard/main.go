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
	Metrics *MetricsHandlers `url:"/metrics"`
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

func (h *APIHandlers) Status(c echo.Context) error {
	// Simulate some processing
	h.Logger.Info("Checking application status")
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"status": "operational",
		"services": map[string]string{
			"database": "connected",
			"cache":    "healthy",
			"queue":    "active",
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// MetricsHandlers demonstrates metrics endpoints
type MetricsHandlers struct {
	Logger    *zap.Logger
	StartTime time.Time
}

func (h *MetricsHandlers) GET(c echo.Context) error {
	// Basic application metrics
	uptime := time.Since(h.StartTime)
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"application": map[string]interface{}{
			"name":           "monitoring-dashboard-example",
			"version":        "1.0.0",
			"uptime_seconds": uptime.Seconds(),
			"uptime_human":   uptime.String(),
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (h *MetricsHandlers) Health(c echo.Context) error {
	// Simple health check
	return response.Success(c, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// Simulate some background work to generate metrics
func simulateWork(logger *zap.Logger) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Simulate some work
			logger.Info("Performing background task",
				zap.Int("random_value", int(time.Now().Unix()%100)),
			)
			
			// Allocate some memory to see it in metrics
			data := make([]byte, 1024*1024) // 1MB
			_ = data
		}
	}
}

func main() {
	// Initialize logger in development mode
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create configuration with debug level to enable development mode
	cfg := &app.Config{}
	cfg.Server.Address = ":8080"
	cfg.Logger.Level = "debug" // This enables development mode and monitoring endpoints

	startTime := time.Now()

	// Create handlers
	handlers := &HandlersManager{
		API: &APIHandlers{
			Logger: logger,
		},
		Metrics: &MetricsHandlers{
			Logger:    logger,
			StartTime: startTime,
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

	// Start background work
	go simulateWork(logger)

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Start server
	go func() {
		logger.Info("Starting monitoring dashboard example", 
			zap.String("address", cfg.Server.Address),
		)
		
		fmt.Println("\n=== MONITORING DASHBOARD EXAMPLE ===")
		fmt.Println("\nApplication endpoints:")
		fmt.Println("  GET /api - API root")
		fmt.Println("  POST /api/status - Check application status")
		fmt.Println("  GET /metrics - Basic application metrics")
		fmt.Println("  POST /metrics/health - Health check")
		
		fmt.Println("\nDevelopment monitoring endpoints:")
		fmt.Println("  GET /_monitor - System monitoring dashboard (memory, goroutines, etc.)")
		fmt.Println("  GET /_routes - List all registered routes")
		
		fmt.Println("\nTry these commands:")
		fmt.Println("  # View system monitoring dashboard")
		fmt.Println("  curl http://localhost:8080/_monitor | jq")
		fmt.Println()
		fmt.Println("  # Watch system metrics in real-time")
		fmt.Println("  watch -n 1 'curl -s http://localhost:8080/_monitor | jq .memory'")
		fmt.Println()
		fmt.Println("  # Check application metrics")
		fmt.Println("  curl http://localhost:8080/metrics | jq")
		fmt.Println()
		fmt.Println("  # Monitor specific fields")
		fmt.Println("  curl -s http://localhost:8080/_monitor | jq '.system.goroutines'")
		fmt.Println("  curl -s http://localhost:8080/_monitor | jq '.memory.heap_alloc_mb'")
		fmt.Println()
		fmt.Println("Press Ctrl+C to stop...")
		fmt.Println()
		
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