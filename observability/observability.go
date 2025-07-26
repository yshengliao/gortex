// Package observability provides a unified interface for metrics, health checks, and tracing
package observability

import (
	"github.com/yshengliao/gortex/middleware"
	"github.com/yshengliao/gortex/observability/health"
	"github.com/yshengliao/gortex/observability/metrics"
	"github.com/yshengliao/gortex/observability/tracing"
)

// Re-export types and functions from sub-packages for backward compatibility

// Metrics types
type (
	MetricsCollector = metrics.MetricsCollector
	ImprovedCollector = metrics.ImprovedCollector
	NoOpCollector = metrics.NoOpCollector
)

// Health types
type (
	HealthChecker = health.HealthChecker
	SafeHealthChecker = health.SafeHealthChecker
	HealthStatus = health.HealthStatus
	HealthCheck = health.HealthCheck
	HealthCheckResult = health.HealthCheckResult
)

// Tracing types
type (
	Tracer = tracing.Tracer
	Span = tracing.Span
	SpanStatus = tracing.SpanStatus
	NoOpTracer = tracing.NoOpTracer
	SimpleTracer = tracing.SimpleTracer
)

// Metrics functions
var (
	NewCollector = metrics.NewCollector
	NewImprovedCollector = metrics.NewImprovedCollector
	MetricsMiddleware = metrics.MetricsMiddleware
)

// Health functions
var (
	NewHealthChecker = health.NewHealthChecker
	NewSafeHealthChecker = health.NewSafeHealthChecker
)

// Tracing functions
var (
	NewSimpleTracer = tracing.NewSimpleTracer
	TracingMiddleware = tracing.TracingMiddleware
)

// Tracing constants
const (
	SpanStatusUnset = tracing.SpanStatusUnset
	SpanStatusOK = tracing.SpanStatusOK
	SpanStatusError = tracing.SpanStatusError
)

// CreateMiddleware creates middleware for all observability components
func CreateMiddleware(collector MetricsCollector, tracer Tracer) []middleware.MiddlewareFunc {
	middlewares := []middleware.MiddlewareFunc{}
	
	if tracer != nil {
		middlewares = append(middlewares, TracingMiddleware(tracer))
	}
	
	if collector != nil {
		middlewares = append(middlewares, MetricsMiddleware(collector))
	}
	
	return middlewares
}