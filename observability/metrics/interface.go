// Package metrics provides metrics collection interfaces and implementations
package metrics

import (
	"time"
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