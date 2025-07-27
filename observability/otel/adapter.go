package otel

import (
	"context"
	"fmt"

	"github.com/yshengliao/gortex/observability/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// OTelTracerAdapter adapts Gortex tracer to OpenTelemetry
type OTelTracerAdapter struct {
	tracer       tracing.EnhancedTracer
	otelTracer   oteltrace.Tracer
	severityMap  map[tracing.SpanStatus]attribute.KeyValue
}

// NewOTelTracerAdapter creates a new OpenTelemetry adapter
func NewOTelTracerAdapter(tracer tracing.EnhancedTracer, tracerName string) *OTelTracerAdapter {
	return &OTelTracerAdapter{
		tracer:     tracer,
		otelTracer: otel.Tracer(tracerName),
		severityMap: initSeverityMap(),
	}
}

// initSeverityMap initializes the severity level mapping
func initSeverityMap() map[tracing.SpanStatus]attribute.KeyValue {
	return map[tracing.SpanStatus]attribute.KeyValue{
		tracing.SpanStatusUnset:     attribute.String("severity", "UNSET"),
		tracing.SpanStatusOK:        attribute.String("severity", "OK"),
		tracing.SpanStatusError:     attribute.String("severity", "ERROR"),
		tracing.SpanStatusDEBUG:     attribute.String("severity", "DEBUG"),
		tracing.SpanStatusINFO:      attribute.String("severity", "INFO"),
		tracing.SpanStatusNOTICE:    attribute.String("severity", "NOTICE"),
		tracing.SpanStatusWARN:      attribute.String("severity", "WARN"),
		tracing.SpanStatusERROR:     attribute.String("severity", "ERROR"),
		tracing.SpanStatusCRITICAL:  attribute.String("severity", "CRITICAL"),
		tracing.SpanStatusALERT:     attribute.String("severity", "ALERT"),
		tracing.SpanStatusEMERGENCY: attribute.String("severity", "EMERGENCY"),
	}
}

// severityToNumber converts Gortex severity to OpenTelemetry severity number
func severityToNumber(status tracing.SpanStatus) int {
	severityNumbers := map[tracing.SpanStatus]int{
		tracing.SpanStatusUnset:     0,
		tracing.SpanStatusOK:        1,
		tracing.SpanStatusDEBUG:     5,
		tracing.SpanStatusINFO:      9,
		tracing.SpanStatusNOTICE:    10,
		tracing.SpanStatusWARN:      13,
		tracing.SpanStatusError:     17,
		tracing.SpanStatusERROR:     17,
		tracing.SpanStatusCRITICAL:  21,
		tracing.SpanStatusALERT:     22,
		tracing.SpanStatusEMERGENCY: 24,
	}
	
	if num, ok := severityNumbers[status]; ok {
		return num
	}
	return 0
}

// StartSpan starts a new span using both Gortex and OpenTelemetry
func (a *OTelTracerAdapter) StartSpan(ctx context.Context, operation string, opts ...oteltrace.SpanStartOption) (context.Context, *SpanAdapter) {
	// Start Gortex enhanced span
	ctx, gortexSpan := a.tracer.StartEnhancedSpan(ctx, operation)
	
	// Start OpenTelemetry span
	ctx, otelSpan := a.otelTracer.Start(ctx, operation, opts...)
	
	// Create adapter
	adapter := &SpanAdapter{
		gortexSpan: gortexSpan,
		otelSpan:   otelSpan,
		adapter:    a,
	}
	
	// Store adapter in context
	ctx = ContextWithSpanAdapter(ctx, adapter)
	
	return ctx, adapter
}

// SpanAdapter bridges Gortex enhanced span and OpenTelemetry span
type SpanAdapter struct {
	gortexSpan *tracing.EnhancedSpan
	otelSpan   oteltrace.Span
	adapter    *OTelTracerAdapter
}

// LogEvent logs an event to both Gortex and OpenTelemetry
func (s *SpanAdapter) LogEvent(severity tracing.SpanStatus, msg string, fields map[string]any) {
	// Log to Gortex
	s.gortexSpan.LogEvent(severity, msg, fields)
	
	// Convert to OpenTelemetry attributes
	attrs := []attribute.KeyValue{
		s.adapter.severityMap[severity],
		attribute.Int("severity.number", severityToNumber(severity)),
		attribute.String("event.name", msg),
	}
	
	// Add fields as attributes
	for k, v := range fields {
		switch val := v.(type) {
		case string:
			attrs = append(attrs, attribute.String(k, val))
		case int:
			attrs = append(attrs, attribute.Int(k, val))
		case int64:
			attrs = append(attrs, attribute.Int64(k, val))
		case float64:
			attrs = append(attrs, attribute.Float64(k, val))
		case bool:
			attrs = append(attrs, attribute.Bool(k, val))
		default:
			attrs = append(attrs, attribute.String(k, fmt.Sprintf("%v", val)))
		}
	}
	
	// Add event to OpenTelemetry span
	s.otelSpan.AddEvent(msg, oteltrace.WithAttributes(attrs...))
}

// SetError sets error on both spans
func (s *SpanAdapter) SetError(err error) {
	// Set error on Gortex span
	s.gortexSpan.SetError(err)
	
	// Set error on OpenTelemetry span
	s.otelSpan.RecordError(err)
	s.otelSpan.SetStatus(codes.Error, err.Error())
}

// AddTags adds tags/attributes to both spans
func (s *SpanAdapter) AddTags(tags map[string]string) {
	// Add to Gortex span
	s.gortexSpan.AddTags(tags)
	
	// Convert to OpenTelemetry attributes
	attrs := make([]attribute.KeyValue, 0, len(tags))
	for k, v := range tags {
		attrs = append(attrs, attribute.String(k, v))
	}
	
	s.otelSpan.SetAttributes(attrs...)
}

// SetStatus sets the status on both spans
func (s *SpanAdapter) SetStatus(status tracing.SpanStatus) {
	// Set on Gortex span
	s.gortexSpan.SetStatus(status)
	
	// Map to OpenTelemetry status
	var code codes.Code
	var description string
	
	switch status {
	case tracing.SpanStatusOK:
		code = codes.Ok
	case tracing.SpanStatusError, tracing.SpanStatusERROR, 
	     tracing.SpanStatusCRITICAL, tracing.SpanStatusALERT, 
	     tracing.SpanStatusEMERGENCY:
		code = codes.Error
		description = status.String()
	default:
		code = codes.Unset
	}
	
	s.otelSpan.SetStatus(code, description)
	
	// Also set severity as attribute
	s.otelSpan.SetAttributes(s.adapter.severityMap[status])
}

// End ends both spans
func (s *SpanAdapter) End() {
	// Finish Gortex span
	s.adapter.tracer.FinishSpan(s.gortexSpan.Span)
	
	// End OpenTelemetry span
	s.otelSpan.End()
}

// GortexSpan returns the underlying Gortex enhanced span
func (s *SpanAdapter) GortexSpan() *tracing.EnhancedSpan {
	return s.gortexSpan
}

// OTelSpan returns the underlying OpenTelemetry span
func (s *SpanAdapter) OTelSpan() oteltrace.Span {
	return s.otelSpan
}

// Context keys
type contextKey string

const spanAdapterKey contextKey = "span_adapter"

// ContextWithSpanAdapter stores span adapter in context
func ContextWithSpanAdapter(ctx context.Context, adapter *SpanAdapter) context.Context {
	return context.WithValue(ctx, spanAdapterKey, adapter)
}

// SpanAdapterFromContext retrieves span adapter from context
func SpanAdapterFromContext(ctx context.Context) *SpanAdapter {
	if adapter, ok := ctx.Value(spanAdapterKey).(*SpanAdapter); ok {
		return adapter
	}
	return nil
}

// HTTPMiddleware creates a middleware that integrates with OpenTelemetry
func (a *OTelTracerAdapter) HTTPMiddleware(next func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		// Extract trace context from headers if available
		// This would typically use the propagator from OpenTelemetry
		
		operation := "HTTP Request" // This should be extracted from the request
		ctx, span := a.StartSpan(ctx, operation,
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		)
		defer span.End()
		
		// Add HTTP semantic conventions
		span.AddTags(map[string]string{
			string(semconv.HTTPMethodKey):     "GET", // Should be extracted
			string(semconv.HTTPTargetKey):     "/",   // Should be extracted
			string(semconv.HTTPSchemeKey):     "http",
			string(semconv.HTTPStatusCodeKey): "200", // Should be set after
		})
		
		// Execute the handler
		err := next(ctx)
		if err != nil {
			span.SetError(err)
		}
		
		return err
	}
}