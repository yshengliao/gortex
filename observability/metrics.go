// Package observability provides metrics, tracing, and monitoring capabilities
package observability

import (
	"fmt"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
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

func (n *NoOpCollector) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {}
func (n *NoOpCollector) RecordHTTPRequestSize(method, path string, size int64) {}
func (n *NoOpCollector) RecordHTTPResponseSize(method, path string, size int64) {}
func (n *NoOpCollector) RecordWebSocketConnection(connected bool) {}
func (n *NoOpCollector) RecordWebSocketMessage(direction string, messageType string, size int64) {}
func (n *NoOpCollector) RecordBusinessMetric(name string, value float64, tags map[string]string) {}
func (n *NoOpCollector) RecordGoroutines(count int) {}
func (n *NoOpCollector) RecordMemoryUsage(bytes uint64) {}

// SimpleCollector is a simple in-memory metrics collector
type SimpleCollector struct {
	httpRequests    []HTTPRequestMetric
	httpRequestSizes map[string]int64
	httpResponseSizes map[string]int64
	wsConnections   int64
	wsMessages      []WebSocketMessageMetric
	businessMetrics []BusinessMetric
	goroutineCount  int
	memoryUsage     uint64
	mu              sync.RWMutex
}

// HTTPRequestMetric represents an HTTP request metric
type HTTPRequestMetric struct {
	Method     string
	Path       string
	StatusCode int
	Duration   time.Duration
	Timestamp  time.Time
}

// WebSocketMessageMetric represents a WebSocket message metric
type WebSocketMessageMetric struct {
	Direction   string // "inbound" or "outbound"
	MessageType string
	Size        int64
	Timestamp   time.Time
}

// BusinessMetric represents a custom business metric
type BusinessMetric struct {
	Name      string
	Value     float64
	Tags      map[string]string
	Timestamp time.Time
}

// NewSimpleCollector creates a new simple metrics collector
// Deprecated: Use NewImprovedCollector instead for better performance
func NewSimpleCollector() *SimpleCollector {
	return &SimpleCollector{
		httpRequests:      make([]HTTPRequestMetric, 0),
		httpRequestSizes:  make(map[string]int64),
		httpResponseSizes: make(map[string]int64),
		wsMessages:        make([]WebSocketMessageMetric, 0),
		businessMetrics:   make([]BusinessMetric, 0),
	}
}

// NewCollector creates a new improved metrics collector (recommended)
func NewCollector() *ImprovedCollector {
	return NewImprovedCollector()
}

func (s *SimpleCollector) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.httpRequests = append(s.httpRequests, HTTPRequestMetric{
		Method:     method,
		Path:       path,
		StatusCode: statusCode,
		Duration:   duration,
		Timestamp:  time.Now(),
	})
}

func (s *SimpleCollector) RecordHTTPRequestSize(method, path string, size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	key := fmt.Sprintf("%s:%s", method, path)
	s.httpRequestSizes[key] = size
}

func (s *SimpleCollector) RecordHTTPResponseSize(method, path string, size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	key := fmt.Sprintf("%s:%s", method, path)
	s.httpResponseSizes[key] = size
}

func (s *SimpleCollector) RecordWebSocketConnection(connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if connected {
		s.wsConnections++
	} else {
		s.wsConnections--
	}
}

func (s *SimpleCollector) RecordWebSocketMessage(direction string, messageType string, size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.wsMessages = append(s.wsMessages, WebSocketMessageMetric{
		Direction:   direction,
		MessageType: messageType,
		Size:        size,
		Timestamp:   time.Now(),
	})
}

func (s *SimpleCollector) RecordBusinessMetric(name string, value float64, tags map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.businessMetrics = append(s.businessMetrics, BusinessMetric{
		Name:      name,
		Value:     value,
		Tags:      tags,
		Timestamp: time.Now(),
	})
}

func (s *SimpleCollector) RecordGoroutines(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.goroutineCount = count
}

func (s *SimpleCollector) RecordMemoryUsage(bytes uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.memoryUsage = bytes
}

// MetricsMiddleware creates an Echo middleware for collecting HTTP metrics
func MetricsMiddleware(collector MetricsCollector) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
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
			status := c.Response().Status
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					status = he.Code
				} else {
					status = 500
				}
			}
			
			// Record metrics
			collector.RecordHTTPRequest(c.Request().Method, c.Path(), status, duration)
			
			// Get response size
			respSize := c.Response().Size
			if respSize > 0 {
				collector.RecordHTTPResponseSize(c.Request().Method, c.Path(), respSize)
			}
			
			return err
		}
	}
}