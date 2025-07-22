package main

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/app"
	cbmiddleware "github.com/yshengliao/gortex/middleware/circuitbreaker"
	"github.com/yshengliao/gortex/pkg/circuitbreaker"
	"github.com/yshengliao/gortex/response"
	"go.uber.org/zap"
)

type HandlersManager struct {
	Service *ServiceHandler `url:"/service"`
	Status  *StatusHandler  `url:"/status"`
}

type ServiceHandler struct {
	Logger        *zap.Logger
	failureRate   atomic.Value  // stores float64
	requestCount  atomic.Int64
	cb            *circuitbreaker.CircuitBreaker
}

// Simulate an unreliable external service
func (h *ServiceHandler) External(c echo.Context) error {
	h.requestCount.Add(1)
	
	// Call external service through circuit breaker
	err := h.cb.Call(c.Request().Context(), func(ctx context.Context) error {
		// Simulate network delay
		time.Sleep(time.Duration(10+rand.Intn(40)) * time.Millisecond)
		
		// Simulate failures based on current failure rate
		rate := h.failureRate.Load().(float64)
		if rand.Float64() < rate {
			return errors.New("external service error")
		}
		
		return nil
	})
	
	if err == circuitbreaker.ErrCircuitOpen {
		h.Logger.Warn("Circuit breaker is OPEN", zap.String("path", c.Path()))
		return response.Error(c, http.StatusServiceUnavailable, "Service temporarily unavailable")
	}
	
	if err == circuitbreaker.ErrTooManyRequests {
		h.Logger.Warn("Circuit breaker is HALF-OPEN with too many requests", zap.String("path", c.Path()))
		return response.Error(c, http.StatusServiceUnavailable, "Too many requests, please retry")
	}
	
	if err != nil {
		h.Logger.Error("External service failed", zap.Error(err))
		return response.Error(c, http.StatusBadGateway, "External service error")
	}
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"message": "External service call successful",
		"request_number": h.requestCount.Load(),
		"circuit_state": h.cb.State().String(),
	})
}

// Simulate another endpoint that might fail
func (h *ServiceHandler) Database(c echo.Context) error {
	// This endpoint is protected by middleware-level circuit breaker
	
	// Simulate database operation
	time.Sleep(time.Duration(5+rand.Intn(15)) * time.Millisecond)
	
	// Simulate failures
	rate := h.failureRate.Load().(float64)
	if rand.Float64() < rate {
		return response.Error(c, http.StatusInternalServerError, "Database connection failed")
	}
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"message": "Database query successful",
		"data": map[string]interface{}{
			"users": 100,
			"active": 85,
		},
	})
}

// Control endpoint to adjust failure rate
func (h *ServiceHandler) FailureRate(c echo.Context) error {
	var req struct {
		Rate float64 `json:"rate" validate:"min=0,max=1"`
	}
	
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request")
	}
	
	h.failureRate.Store(req.Rate)
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"message": "Failure rate updated",
		"rate": req.Rate,
	})
}

type StatusHandler struct {
	Manager *cbmiddleware.Manager
	ServiceCB *circuitbreaker.CircuitBreaker
}

func (h *StatusHandler) GET(c echo.Context) error {
	// Get middleware circuit breaker stats
	middlewareStats := h.Manager.Stats()
	
	// Get service circuit breaker stats
	serviceCounts := h.ServiceCB.Counts()
	serviceStats := map[string]interface{}{
		"state": h.ServiceCB.State().String(),
		"requests": serviceCounts.Requests,
		"successes": serviceCounts.TotalSuccesses,
		"failures": serviceCounts.TotalFailures,
		"failure_ratio": serviceCounts.FailureRatio(),
		"consecutive_successes": serviceCounts.ConsecutiveSuccesses,
		"consecutive_failures": serviceCounts.ConsecutiveFailures,
	}
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"circuit_breakers": map[string]interface{}{
			"middleware": middlewareStats,
			"service": map[string]interface{}{
				"/service/external": serviceStats,
			},
		},
		"info": "Circuit breakers protect against cascading failures",
	})
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	
	// Create circuit breaker for external service
	cbConfig := circuitbreaker.Config{
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     5 * time.Second,
		ReadyToTrip: func(counts circuitbreaker.Counts) bool {
			// Trip if failure ratio > 50% and at least 5 requests
			return counts.Requests >= 5 && counts.FailureRatio() > 0.5
		},
		OnStateChange: func(name string, from, to circuitbreaker.State) {
			logger.Info("Circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()))
		},
	}
	
	serviceCB := circuitbreaker.New("external-service", cbConfig)
	
	// Create circuit breaker manager for middleware
	cbManager := cbmiddleware.NewManager(circuitbreaker.Config{
		MaxRequests: 2,
		Interval:    10 * time.Second,
		Timeout:     3 * time.Second,
		ReadyToTrip: func(counts circuitbreaker.Counts) bool {
			// Trip if 3 consecutive failures or failure ratio > 60%
			return counts.ConsecutiveFailures >= 3 || 
				(counts.Requests >= 5 && counts.FailureRatio() > 0.6)
		},
	})
	
	// Create configuration
	cfg := &app.Config{}
	cfg.Server.Address = ":8080"
	cfg.Logger.Level = "debug"
	
	// Create handlers
	serviceHandler := &ServiceHandler{
		Logger: logger,
		cb:     serviceCB,
	}
	serviceHandler.failureRate.Store(0.1) // 10% failure rate initially
	
	handlers := &HandlersManager{
		Service: serviceHandler,
		Status: &StatusHandler{
			Manager:   cbManager,
			ServiceCB: serviceCB,
		},
	}
	
	// Create application
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	if err != nil {
		logger.Fatal("Failed to create application", zap.Error(err))
	}
	
	// Add circuit breaker middleware
	e := application.Echo()
	e.Use(cbmiddleware.CircuitBreakerWithConfig(cbmiddleware.Config{
		CircuitBreakerConfig: cbManager.Config(),
		GetCircuitBreakerName: func(c echo.Context) string {
			// Use path as circuit breaker name
			return c.Path()
		},
		IsFailure: func(c echo.Context, err error) bool {
			// Consider 5xx errors and actual errors as failures
			if err != nil {
				return true
			}
			return c.Response().Status >= 500
		},
	}))
	
	logger.Info("Circuit breaker example started", 
		zap.String("address", cfg.Server.Address))
	logger.Info("Try these endpoints:",
		zap.String("external", "GET http://localhost:8080/service/external"),
		zap.String("database", "POST http://localhost:8080/service/database"),
		zap.String("status", "GET http://localhost:8080/status"),
		zap.String("failure_rate", "POST http://localhost:8080/service/failure-rate"))
	logger.Info("Adjust failure rate to trigger circuit breaker:")
	logger.Info(`curl -X POST http://localhost:8080/service/failure-rate -H "Content-Type: application/json" -d '{"rate": 0.8}'`)
	
	// Simulate load in background
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		for {
			time.Sleep(500 * time.Millisecond)
			
			// Call external service
			resp, err := client.Get("http://localhost:8080/service/external")
			if err == nil {
				resp.Body.Close()
			}
			
			// Call database service
			resp, err = client.Post("http://localhost:8080/service/database", "application/json", nil)
			if err == nil {
				resp.Body.Close()
			}
		}
	}()
	
	// Start server
	if err := application.Run(); err != nil {
		logger.Fatal("Server failed", zap.Error(err))
	}
}