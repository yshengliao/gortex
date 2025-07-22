package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/pkg/requestid"
	"github.com/yshengliao/gortex/response"
	"go.uber.org/zap"
)

// Define handlers that demonstrate request ID usage

type HandlersManager struct {
	API      *APIHandler      `url:"/api"`
	External *ExternalHandler `url:"/external"`
}

type APIHandler struct {
	Logger *zap.Logger
}

// GET /api - Simple endpoint that logs with request ID
func (h *APIHandler) GET(c echo.Context) error {
	// Get logger with request ID automatically included
	logger := requestid.LoggerFromEcho(h.Logger, c)
	
	// Log message will include request_id field
	logger.Info("Handling API request",
		zap.String("method", c.Request().Method),
		zap.String("path", c.Request().URL.Path),
	)
	
	// Response includes request ID automatically via SuccessWithMeta
	return response.SuccessWithMeta(c, http.StatusOK, 
		map[string]string{
			"message": "API endpoint successfully called",
			"time":    time.Now().Format(time.RFC3339),
		},
		map[string]interface{}{
			"version": "1.0.0",
		},
	)
}

// POST /api/process - Endpoint that processes data with request ID tracking
func (h *APIHandler) Process(c echo.Context) error {
	logger := requestid.LoggerFromEcho(h.Logger, c)
	
	var body map[string]interface{}
	if err := c.Bind(&body); err != nil {
		logger.Error("Failed to parse request body", zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
	}
	
	// Simulate processing with request ID tracking
	logger.Info("Processing request",
		zap.Any("payload", body),
		zap.String("processing_stage", "validation"),
	)
	
	// Simulate some work
	time.Sleep(100 * time.Millisecond)
	
	logger.Info("Processing complete",
		zap.String("processing_stage", "complete"),
	)
	
	return response.SuccessWithMeta(c, http.StatusOK,
		map[string]interface{}{
			"processed": true,
			"data":      body,
		},
		map[string]interface{}{
			"processing_time_ms": 100,
		},
	)
}

type ExternalHandler struct {
	Logger *zap.Logger
}

// GET /external/call - Demonstrates propagating request ID to external services
func (h *ExternalHandler) Call(c echo.Context) error {
	logger := requestid.LoggerFromEcho(h.Logger, c)
	
	// Create context with request ID for propagation
	ctx := requestid.WithEchoContext(context.Background(), c)
	
	// Create HTTP client that automatically propagates request ID
	client := requestid.NewHTTPClient(http.DefaultClient, ctx)
	
	// Make external call - request ID will be automatically included
	// For demo, we'll call our own API endpoint
	apiURL := fmt.Sprintf("http://localhost%s/api", c.Echo().Server.Addr)
	
	logger.Info("Making external API call",
		zap.String("url", apiURL),
	)
	
	resp, err := client.Get(apiURL)
	if err != nil {
		logger.Error("External call failed", zap.Error(err))
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "External service unavailable",
		})
	}
	defer resp.Body.Close()
	
	logger.Info("External call completed",
		zap.Int("status_code", resp.StatusCode),
		zap.String("received_request_id", resp.Header.Get(requestid.HeaderXRequestID)),
	)
	
	return response.SuccessWithMeta(c, http.StatusOK,
		map[string]interface{}{
			"external_call_status": resp.StatusCode,
			"request_propagated":   true,
		},
		map[string]interface{}{
			"external_request_id": resp.Header.Get(requestid.HeaderXRequestID),
		},
	)
}

// GET /external/trace - Shows the complete request ID flow
func (h *ExternalHandler) Trace(c echo.Context) error {
	// Extract request ID in different ways
	rid1 := requestid.FromEchoContext(c)
	rid2 := c.Get("request_id").(string)
	rid3 := c.Response().Header().Get(echo.HeaderXRequestID)
	
	// Create a standard context with request ID
	ctx := requestid.WithEchoContext(context.Background(), c)
	rid4 := requestid.FromContext(ctx)
	
	return c.JSON(http.StatusOK, map[string]interface{}{
		"request_id_sources": map[string]string{
			"from_echo_context":   rid1,
			"from_context_value":  rid2,
			"from_response_header": rid3,
			"from_standard_context": rid4,
		},
		"all_match": rid1 == rid2 && rid2 == rid3 && rid3 == rid4,
	})
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create configuration
	cfg := &app.Config{}
	cfg.Server.Address = ":8080"
	cfg.Server.Recovery = true
	cfg.Logger.Level = "debug" // Enable debug to show request details

	// Create handlers
	handlers := &HandlersManager{
		API: &APIHandler{
			Logger: logger,
		},
		External: &ExternalHandler{
			Logger: logger,
		},
	}

	// Create application with request ID middleware automatically configured
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
		logger.Info("Starting server with request ID tracking", 
			zap.String("address", cfg.Server.Address))
		logger.Info("Try these endpoints:",
			zap.String("simple", "curl http://localhost:8080/api"),
			zap.String("with_id", "curl -H 'X-Request-ID: my-custom-id' http://localhost:8080/api"),
			zap.String("post", "curl -X POST -H 'Content-Type: application/json' -d '{\"test\":\"data\"}' http://localhost:8080/api/process"),
			zap.String("trace", "curl http://localhost:8080/external/trace"),
		)
		
		if err := application.Run(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()

	// Graceful shutdown
	logger.Info("Shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := application.Shutdown(shutdownCtx); err != nil {
		logger.Error("Shutdown error", zap.Error(err))
	}

	logger.Info("Server stopped")
}