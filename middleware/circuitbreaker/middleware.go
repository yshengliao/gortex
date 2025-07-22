package circuitbreaker

import (
	"context"
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/yshengliao/gortex/pkg/circuitbreaker"
	"github.com/yshengliao/gortex/response"
)

// Config defines the config for CircuitBreaker middleware
type Config struct {
	// Skipper defines a function to skip middleware
	Skipper middleware.Skipper

	// CircuitBreakerConfig is the configuration for the circuit breaker
	CircuitBreakerConfig circuitbreaker.Config

	// GetCircuitBreakerName returns the circuit breaker name for a request
	// Default: returns the request path
	GetCircuitBreakerName func(c echo.Context) string

	// IsFailure determines if a response should be considered a failure
	// Default: status code >= 500
	IsFailure func(c echo.Context, err error) bool

	// ErrorHandler is called when the circuit is open
	// Default: returns 503 Service Unavailable
	ErrorHandler func(c echo.Context, err error) error
}

// DefaultConfig returns a default configuration for the circuit breaker middleware
func DefaultConfig() Config {
	return Config{
		Skipper: middleware.DefaultSkipper,
		CircuitBreakerConfig: circuitbreaker.DefaultConfig(),
		GetCircuitBreakerName: func(c echo.Context) string {
			return c.Path()
		},
		IsFailure: func(c echo.Context, err error) bool {
			if err != nil {
				return true
			}
			return c.Response().Status >= http.StatusInternalServerError
		},
		ErrorHandler: func(c echo.Context, err error) error {
			if err == circuitbreaker.ErrCircuitOpen {
				return response.Error(c, http.StatusServiceUnavailable, "Service temporarily unavailable")
			}
			if err == circuitbreaker.ErrTooManyRequests {
				return response.Error(c, http.StatusServiceUnavailable, "Too many requests, please retry later")
			}
			return err
		},
	}
}

// CircuitBreaker returns a circuit breaker middleware
func CircuitBreaker() echo.MiddlewareFunc {
	return CircuitBreakerWithConfig(DefaultConfig())
}

// CircuitBreakerWithConfig returns a circuit breaker middleware with config
func CircuitBreakerWithConfig(config Config) echo.MiddlewareFunc {
	// Defaults
	if config.Skipper == nil {
		config.Skipper = DefaultConfig().Skipper
	}
	if config.GetCircuitBreakerName == nil {
		config.GetCircuitBreakerName = DefaultConfig().GetCircuitBreakerName
	}
	if config.IsFailure == nil {
		config.IsFailure = DefaultConfig().IsFailure
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = DefaultConfig().ErrorHandler
	}

	// Circuit breakers per endpoint
	breakers := &sync.Map{}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			name := config.GetCircuitBreakerName(c)
			
			// Get or create circuit breaker
			value, _ := breakers.LoadOrStore(name, circuitbreaker.New(name, config.CircuitBreakerConfig))
			cb := value.(*circuitbreaker.CircuitBreaker)

			// Execute request through circuit breaker
			err := cb.Call(c.Request().Context(), func(ctx context.Context) error {
				// Execute the handler
				err := next(c)
				
				// Check if this should be considered a failure
				if config.IsFailure(c, err) {
					if err != nil {
						return err
					}
					return &statusError{code: c.Response().Status}
				}
				
				return nil
			})

			// Handle circuit breaker errors
			if err == circuitbreaker.ErrCircuitOpen || err == circuitbreaker.ErrTooManyRequests {
				return config.ErrorHandler(c, err)
			}

			// Check if it was a status error
			if se, ok := err.(*statusError); ok {
				// Already written to response
				if se.code != 0 {
					return nil
				}
			}

			return err
		}
	}
}

// statusError represents an HTTP status code error
type statusError struct {
	code int
}

func (e *statusError) Error() string {
	return http.StatusText(e.code)
}

// Manager manages multiple circuit breakers
type Manager struct {
	breakers *sync.Map
	config   circuitbreaker.Config
}

// Config returns the manager's circuit breaker configuration
func (m *Manager) Config() circuitbreaker.Config {
	return m.config
}

// NewManager creates a new circuit breaker manager
func NewManager(config circuitbreaker.Config) *Manager {
	return &Manager{
		breakers: &sync.Map{},
		config:   config,
	}
}

// Get returns a circuit breaker by name, creating it if necessary
func (m *Manager) Get(name string) *circuitbreaker.CircuitBreaker {
	value, _ := m.breakers.LoadOrStore(name, circuitbreaker.New(name, m.config))
	return value.(*circuitbreaker.CircuitBreaker)
}

// GetAll returns all circuit breakers
func (m *Manager) GetAll() map[string]*circuitbreaker.CircuitBreaker {
	result := make(map[string]*circuitbreaker.CircuitBreaker)
	m.breakers.Range(func(key, value interface{}) bool {
		result[key.(string)] = value.(*circuitbreaker.CircuitBreaker)
		return true
	})
	return result
}

// Reset resets a circuit breaker by name
func (m *Manager) Reset(name string) {
	m.breakers.Delete(name)
}

// ResetAll resets all circuit breakers
func (m *Manager) ResetAll() {
	m.breakers = &sync.Map{}
}

// Stats returns statistics for all circuit breakers
func (m *Manager) Stats() map[string]interface{} {
	stats := make(map[string]interface{})
	m.breakers.Range(func(key, value interface{}) bool {
		cb := value.(*circuitbreaker.CircuitBreaker)
		counts := cb.Counts()
		stats[key.(string)] = map[string]interface{}{
			"state":          cb.State().String(),
			"requests":       counts.Requests,
			"successes":      counts.TotalSuccesses,
			"failures":       counts.TotalFailures,
			"failure_ratio":  counts.FailureRatio(),
			"consecutive_successes": counts.ConsecutiveSuccesses,
			"consecutive_failures":  counts.ConsecutiveFailures,
		}
		return true
	})
	return stats
}