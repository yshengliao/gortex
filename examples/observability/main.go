package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/auth"
	"github.com/yshengliao/gortex/config"
	"github.com/yshengliao/gortex/middleware"
	"github.com/yshengliao/gortex/observability"
)

// Handlers
type HandlersManager struct {
	Health  *HealthHandler  `url:"/health"`
	Metrics *MetricsHandler `url:"/metrics"`
	API     *APIHandler     `url:"/api"`
}

type HealthHandler struct {
	Checker *observability.HealthChecker
}

func (h *HealthHandler) GET(c echo.Context) error {
	results := h.Checker.GetResults()
	overallStatus := h.Checker.GetOverallStatus()
	
	statusCode := http.StatusOK
	if overallStatus == observability.HealthStatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}
	
	return c.JSON(statusCode, map[string]interface{}{
		"status": overallStatus,
		"checks": results,
		"timestamp": time.Now().Unix(),
	})
}

type MetricsHandler struct {
	Collector observability.MetricsCollector
	Logger    *zap.Logger
}

func (h *MetricsHandler) GET(c echo.Context) error {
	// In a real implementation, this would expose metrics in Prometheus format
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	return c.JSON(200, map[string]interface{}{
		"goroutines": runtime.NumGoroutine(),
		"memory": map[string]interface{}{
			"alloc_mb":      m.Alloc / 1024 / 1024,
			"total_alloc_mb": m.TotalAlloc / 1024 / 1024,
			"sys_mb":        m.Sys / 1024 / 1024,
			"gc_count":      m.NumGC,
		},
		"timestamp": time.Now().Unix(),
	})
}

type APIHandler struct {
	Logger    *zap.Logger
	Collector observability.MetricsCollector
	Tracer    observability.Tracer
}

func (h *APIHandler) SlowEndpoint(c echo.Context) error {
	_, span := h.Tracer.StartSpan(c.Request().Context(), "SlowEndpoint")
	defer h.Tracer.FinishSpan(span)
	
	// Simulate slow operation
	time.Sleep(100 * time.Millisecond)
	
	// Record business metric
	h.Collector.RecordBusinessMetric("api.slow_endpoint.calls", 1, map[string]string{
		"endpoint": "slow",
		"status": "success",
	})
	
	h.Tracer.SetStatus(span, observability.SpanStatusOK)
	return c.JSON(200, map[string]interface{}{
		"message": "Slow operation completed",
		"trace_id": span.TraceID,
	})
}

func (h *APIHandler) FastEndpoint(c echo.Context) error {
	_, span := h.Tracer.StartSpan(c.Request().Context(), "FastEndpoint")
	defer h.Tracer.FinishSpan(span)
	
	// Fast operation
	time.Sleep(5 * time.Millisecond)
	
	h.Collector.RecordBusinessMetric("api.fast_endpoint.calls", 1, map[string]string{
		"endpoint": "fast",
		"status": "success",
	})
	
	h.Tracer.SetStatus(span, observability.SpanStatusOK)
	return c.JSON(200, map[string]interface{}{
		"message": "Fast operation completed",
		"trace_id": span.TraceID,
	})
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create configuration
	cfg := config.DefaultConfig()
	cfg.Server.Address = ":8080"

	// Create observability components
	metricsCollector := observability.NewSimpleCollector()
	tracer := observability.NewSimpleTracer()
	healthChecker := observability.NewHealthChecker(30*time.Second, 5*time.Second)
	
	// Register health checks
	healthChecker.Register("server", func(ctx context.Context) observability.HealthCheckResult {
		return observability.HealthCheckResult{
			Status:  observability.HealthStatusHealthy,
			Message: "Server is running",
		}
	})
	
	healthChecker.Register("database", observability.DatabaseHealthCheck(func(ctx context.Context) error {
		// Simulate database ping
		return nil
	}))
	
	healthChecker.Register("memory", observability.MemoryHealthCheck(1024))

	// Create handlers
	handlers := &HandlersManager{
		Health: &HealthHandler{
			Checker: healthChecker,
		},
		Metrics: &MetricsHandler{
			Collector: metricsCollector,
			Logger:    logger,
		},
		API: &APIHandler{
			Logger:    logger,
			Collector: metricsCollector,
			Tracer:    tracer,
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

	e := application.Echo()

	// Add observability middleware
	e.Use(observability.MetricsMiddleware(metricsCollector))
	e.Use(observability.TracingMiddleware(tracer))

	// Add rate limiting
	// Global rate limit: 100 requests per second
	e.Use(middleware.RateLimitByIP(100, 200))

	// Specific rate limits for API endpoints
	apiGroup := e.Group("/api")
	
	// Slow endpoint: 1 request per second
	apiGroup.POST("/slow", handlers.API.SlowEndpoint, middleware.RateLimitByIP(1, 2))
	
	// Fast endpoint: 10 requests per second
	apiGroup.POST("/fast", handlers.API.FastEndpoint, middleware.RateLimitByIP(10, 20))

	// Create JWT service for protected endpoints
	jwtService := auth.NewJWTService(
		"secret-key",
		time.Hour,
		24*time.Hour*7,
		"observability-example",
	)

	// Protected metrics endpoint
	metricsGroup := e.Group("/metrics")
	metricsGroup.Use(auth.Middleware(jwtService))

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server
	go func() {
		logger.Info("Starting observability example server",
			zap.String("address", cfg.Server.Address),
			zap.String("health", "/health"),
			zap.String("metrics", "/metrics (requires auth)"),
			zap.String("api_slow", "/api/slow (1 req/sec)"),
			zap.String("api_fast", "/api/fast (10 req/sec)"),
		)

		if err := application.Run(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	// Monitor metrics in background
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				metricsCollector.RecordGoroutines(runtime.NumGoroutine())
				
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				metricsCollector.RecordMemoryUsage(m.Alloc)
				
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for interrupt
	<-ctx.Done()

	// Shutdown
	logger.Info("Shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	healthChecker.Stop()
	if err := application.Shutdown(shutdownCtx); err != nil {
		logger.Error("Shutdown error", zap.Error(err))
	}

	logger.Info("Server stopped")
}