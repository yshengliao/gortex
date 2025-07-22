package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/pkg/errors"
	"github.com/yshengliao/gortex/pkg/httpclient"
	"github.com/yshengliao/gortex/response"
	"go.uber.org/zap"
)

type HandlersManager struct {
	API     *APIHandler     `url:"/api"`
	Metrics *MetricsHandler `url:"/metrics"`
}

type APIHandler struct {
	Logger *zap.Logger
	Pool   *httpclient.Pool
}

func (h *APIHandler) External(c echo.Context) error {
	// Use external client for third-party API calls
	client := h.Pool.Get("external")
	
	req, err := http.NewRequest("GET", "https://api.github.com/users/github", nil)
	if err != nil {
		return errors.InternalServerError(c, err)
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return errors.SendErrorCode(c, errors.CodeServiceUnavailable)
	}
	defer resp.Body.Close()
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"status": resp.StatusCode,
		"source": "external",
	})
}

func (h *APIHandler) Internal(c echo.Context) error {
	// Use internal client for microservice calls
	client := h.Pool.Get("internal")
	
	// Simulate internal service call
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
	defer cancel()
	
	req, err := http.NewRequest("GET", "http://localhost:8081/health", nil)
	if err != nil {
		return errors.InternalServerError(c, err)
	}
	
	resp, err := client.DoWithContext(ctx, req)
	if err != nil {
		h.Logger.Warn("Internal service unavailable", zap.Error(err))
		return response.Success(c, http.StatusOK, map[string]interface{}{
			"status": "degraded",
			"source": "internal",
			"error":  err.Error(),
		})
	}
	defer resp.Body.Close()
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"status": resp.StatusCode,
		"source": "internal",
	})
}

func (h *APIHandler) Batch(c echo.Context) error {
	// Demonstrate concurrent requests with connection pooling
	urls := []string{
		"https://httpbin.org/delay/1",
		"https://httpbin.org/delay/2",
		"https://httpbin.org/delay/1",
	}
	
	client := h.Pool.Get("external")
	results := make(chan map[string]interface{}, len(urls))
	
	// Make concurrent requests
	for i, url := range urls {
		go func(idx int, u string) {
			start := time.Now()
			req, _ := http.NewRequest("GET", u, nil)
			resp, err := client.Do(req)
			
			result := map[string]interface{}{
				"index":    idx,
				"url":      u,
				"duration": time.Since(start).Milliseconds(),
			}
			
			if err != nil {
				result["error"] = err.Error()
			} else {
				result["status"] = resp.StatusCode
				resp.Body.Close()
			}
			
			results <- result
		}(i, url)
	}
	
	// Collect results
	var responses []map[string]interface{}
	for i := 0; i < len(urls); i++ {
		responses = append(responses, <-results)
	}
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"requests": responses,
		"info":     "Connection pooling enables efficient concurrent requests",
	})
}

type MetricsHandler struct {
	Pool *httpclient.Pool
}

func (h *MetricsHandler) GET(c echo.Context) error {
	allMetrics := h.Pool.GetAllMetrics()
	
	// Format metrics for display
	formattedMetrics := make(map[string]interface{})
	for name, metrics := range allMetrics {
		formattedMetrics[name] = map[string]interface{}{
			"connections": map[string]interface{}{
				"active":         metrics.ActiveConnections,
				"idle":           metrics.IdleConnections,
				"total_created":  metrics.TotalConnections,
				"reuse_count":    metrics.ConnectionReuse,
			},
			"requests": map[string]interface{}{
				"total":          metrics.TotalRequests,
				"successful":     metrics.TotalResponses,
				"errors":         metrics.TotalErrors,
				"avg_time_ms":    metrics.AverageResponseTime.Milliseconds(),
			},
			"status_codes": metrics.StatusCodes,
		}
	}
	
	return response.Success(c, http.StatusOK, map[string]interface{}{
		"pool_size": h.Pool.Size(),
		"clients":   h.Pool.Names(),
		"metrics":   formattedMetrics,
	})
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	
	// Create HTTP client pool with custom factory
	pool := httpclient.NewPoolWithFactory(func(name string) *httpclient.Client {
		config := httpclient.DefaultConfig()
		
		switch name {
		case "internal":
			// Fast timeout for internal services
			config.Timeout = 5 * time.Second
			config.MaxIdleConnsPerHost = 20
			config.DialTimeout = 2 * time.Second
		case "external":
			// Longer timeout for external APIs
			config.Timeout = 30 * time.Second
			config.MaxIdleConnsPerHost = 10
			config.IdleConnTimeout = 2 * time.Minute
		default:
			// Default configuration
		}
		
		return httpclient.New(config)
	})
	defer pool.Close()
	
	// Create configuration
	cfg := &app.Config{}
	cfg.Server.Address = ":8080"
	cfg.Logger.Level = "debug"
	
	// Create handlers
	handlers := &HandlersManager{
		API: &APIHandler{
			Logger: logger,
			Pool:   pool,
		},
		Metrics: &MetricsHandler{
			Pool: pool,
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
	
	// Start internal test server
	go startInternalServer()
	
	logger.Info("HTTP client pool example started", 
		zap.String("address", cfg.Server.Address))
	logger.Info("Try these endpoints:",
		zap.String("external", "http://localhost:8080/api/external"),
		zap.String("internal", "http://localhost:8080/api/internal"),
		zap.String("batch", "http://localhost:8080/api/batch"),
		zap.String("metrics", "http://localhost:8080/metrics"))
	
	// Start server
	if err := application.Run(); err != nil {
		logger.Fatal("Server failed", zap.Error(err))
	}
}

// startInternalServer simulates an internal microservice
func startInternalServer() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"healthy"}`)
	})
	http.ListenAndServe(":8081", nil)
}