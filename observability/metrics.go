// Package observability provides metrics, tracing, and monitoring capabilities
package observability

import (
	"time"

	"github.com/yshengliao/gortex/context"
	"github.com/yshengliao/gortex/middleware"
)

// MetricsCollector defines the interface for collecting metrics
type MetricsCollector interface {
	// HTTP metrics
	RecordHTTPRequest(method, path string, statusCode int, duration time.Duration)
	RecordHTTPRequestSize(method, path string, size int64)
	RecordHTTPResponseSize(method, path string, size int64)

	// WebSocket metrics
	RecordWebSocketConnection(connected bool)
	RecordWebSocketMessage(direction string, messageType string, size int64)

	// Business metrics
	RecordBusinessMetric(name string, value float64, tags map[string]string)

	// System metrics
	RecordGoroutines(count int)
	RecordMemoryUsage(bytes uint64)
}

// NoOpCollector is a no-op implementation of MetricsCollector
type NoOpCollector struct{}

func (n *NoOpCollector) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {
}
func (n *NoOpCollector) RecordHTTPRequestSize(method, path string, size int64)                   {}
func (n *NoOpCollector) RecordHTTPResponseSize(method, path string, size int64)                  {}
func (n *NoOpCollector) RecordWebSocketConnection(connected bool)                                {}
func (n *NoOpCollector) RecordWebSocketMessage(direction string, messageType string, size int64) {}
func (n *NoOpCollector) RecordBusinessMetric(name string, value float64, tags map[string]string) {}
func (n *NoOpCollector) RecordGoroutines(count int)                                              {}
func (n *NoOpCollector) RecordMemoryUsage(bytes uint64)                                          {}

// NewCollector creates a new improved metrics collector (recommended)
func NewCollector() *ImprovedCollector {
	return NewImprovedCollector()
}

// MetricsMiddleware creates a Gortex middleware for collecting HTTP metrics
func MetricsMiddleware(collector MetricsCollector) middleware.MiddlewareFunc {
	return func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(c context.Context) error {
			start := time.Now()

			// Get request size
			reqSize := c.Request().ContentLength
			if reqSize > 0 {
				collector.RecordHTTPRequestSize(c.Request().Method, c.Path(), reqSize)
			}

			// Process request
			err := next(c)

			// Calculate duration
			duration := time.Since(start)

			// Get response status
			status := c.Response().Status()
			if err != nil {
				if he, ok := err.(*context.HTTPError); ok {
					status = he.Code
				} else {
					status = 500
				}
			}

			// Record metrics
			collector.RecordHTTPRequest(c.Request().Method, c.Path(), status, duration)

			// Get response size
			respSize := c.Response().Size()
			if respSize > 0 {
				collector.RecordHTTPResponseSize(c.Request().Method, c.Path(), respSize)
			}

			return err
		}
	}
}
