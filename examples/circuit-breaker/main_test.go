package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yshengliao/gortex/app"
	cbmiddleware "github.com/yshengliao/gortex/middleware/circuitbreaker"
	"github.com/yshengliao/gortex/pkg/circuitbreaker"
	"go.uber.org/zap"
)

func TestCircuitBreakerExample(t *testing.T) {
	// Initialize components
	logger, _ := zap.NewDevelopment()
	
	// Create circuit breaker
	cbConfig := circuitbreaker.Config{
		MaxRequests: 2,
		Interval:    1 * time.Second,
		Timeout:     1 * time.Second,
		ReadyToTrip: func(counts circuitbreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
	}
	
	serviceCB := circuitbreaker.New("test-service", cbConfig)
	cbManager := cbmiddleware.NewManager(cbConfig)
	
	// Create configuration
	cfg := &app.Config{}
	cfg.Server.Address = ":0"
	cfg.Logger.Level = "debug"
	
	// Create handlers
	serviceHandler := &ServiceHandler{
		Logger: logger,
		cb:     serviceCB,
	}
	serviceHandler.failureRate.Store(0.0) // No failures initially
	
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
	require.NoError(t, err)
	
	e := application.Echo()
	
	// Add circuit breaker middleware
	e.Use(cbmiddleware.CircuitBreakerWithConfig(cbmiddleware.Config{
		CircuitBreakerConfig: cbManager.Config(),
	}))
	
	// Test successful external service call
	t.Run("SuccessfulExternalCall", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/service/external", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "External service call successful")
		assert.Contains(t, rec.Body.String(), `"circuit_state":"closed"`)
	})
	
	// Test database endpoint
	t.Run("DatabaseEndpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/service/database", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "Database query successful")
	})
	
	// Test failure rate adjustment
	t.Run("AdjustFailureRate", func(t *testing.T) {
		body := `{"rate": 1.0}`
		req := httptest.NewRequest(http.MethodPost, "/service/failure-rate", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "Failure rate updated")
	})
	
	// Test circuit breaker opening
	t.Run("CircuitBreakerOpens", func(t *testing.T) {
		// Make requests that will fail
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodPost, "/service/external", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			
			if i < 2 {
				assert.Equal(t, http.StatusBadGateway, rec.Code)
			} else {
				// Circuit should be open now
				assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
				assert.Contains(t, rec.Body.String(), "Service temporarily unavailable")
			}
		}
	})
	
	// Test status endpoint
	t.Run("StatusEndpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var result map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &result)
		require.NoError(t, err)
		
		data := result["data"].(map[string]interface{})
		breakers := data["circuit_breakers"].(map[string]interface{})
		assert.NotNil(t, breakers["service"])
		assert.NotNil(t, breakers["middleware"])
	})
}

func TestCircuitBreakerRecovery(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Fast timeout for testing
	cbConfig := circuitbreaker.Config{
		MaxRequests: 1,
		Interval:    100 * time.Millisecond,
		Timeout:     200 * time.Millisecond,
		ReadyToTrip: func(counts circuitbreaker.Counts) bool {
			return counts.TotalFailures >= 1
		},
	}
	
	serviceCB := circuitbreaker.New("test-recovery", cbConfig)
	
	serviceHandler := &ServiceHandler{
		Logger: logger,
		cb:     serviceCB,
	}
	
	cfg := &app.Config{}
	cfg.Server.Address = ":0"
	
	handlers := &HandlersManager{
		Service: serviceHandler,
		Status:  &StatusHandler{ServiceCB: serviceCB},
	}
	
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	require.NoError(t, err)
	
	e := application.Echo()
	
	// Set failure rate to 100%
	serviceHandler.failureRate.Store(1.0)
	
	// First request fails and opens circuit
	req := httptest.NewRequest(http.MethodPost, "/service/external", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadGateway, rec.Code)
	
	// Circuit is now open
	req = httptest.NewRequest(http.MethodPost, "/service/external", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	
	// Set failure rate to 0% for recovery
	serviceHandler.failureRate.Store(0.0)
	
	// Wait for timeout
	time.Sleep(250 * time.Millisecond)
	
	// Circuit should be half-open, request should succeed
	req = httptest.NewRequest(http.MethodPost, "/service/external", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	
	// Circuit should be closed now
	req = httptest.NewRequest(http.MethodPost, "/service/external", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"circuit_state":"closed"`)
}

func BenchmarkCircuitBreakerMiddleware(b *testing.B) {
	logger, _ := zap.NewProduction()
	
	serviceCB := circuitbreaker.New("bench", circuitbreaker.DefaultConfig())
	
	serviceHandler := &ServiceHandler{
		Logger: logger,
		cb:     serviceCB,
	}
	serviceHandler.failureRate.Store(0.0)
	
	cfg := &app.Config{}
	cfg.Server.Address = ":0"
	
	handlers := &HandlersManager{
		Service: serviceHandler,
	}
	
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	require.NoError(b, err)
	
	e := application.Echo()
	
	req := httptest.NewRequest(http.MethodPost, "/service/external", nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
	}
}