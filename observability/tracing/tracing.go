package tracing

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	gortexContext "github.com/yshengliao/gortex/http/context"
	"github.com/yshengliao/gortex/middleware"
)

// Span represents a trace span
type Span struct {
	TraceID   string
	SpanID    string
	ParentID  string
	Operation string
	StartTime time.Time
	EndTime   time.Time
	Tags      map[string]string
	Status    SpanStatus
}

// SpanStatus represents the status of a span
type SpanStatus int

const (
	SpanStatusUnset SpanStatus = iota
	SpanStatusOK
	SpanStatusError
)

// Tracer defines the interface for distributed tracing
type Tracer interface {
	// StartSpan starts a new span
	StartSpan(ctx context.Context, operation string) (context.Context, *Span)
	
	// FinishSpan finishes a span
	FinishSpan(span *Span)
	
	// AddTags adds tags to a span
	AddTags(span *Span, tags map[string]string)
	
	// SetStatus sets the status of a span
	SetStatus(span *Span, status SpanStatus)
}

// NoOpTracer is a no-op implementation of Tracer
type NoOpTracer struct{}

func (n *NoOpTracer) StartSpan(ctx context.Context, operation string) (context.Context, *Span) {
	return ctx, &Span{}
}

func (n *NoOpTracer) FinishSpan(span *Span) {}
func (n *NoOpTracer) AddTags(span *Span, tags map[string]string) {}
func (n *NoOpTracer) SetStatus(span *Span, status SpanStatus) {}

// SimpleTracer is a simple in-memory tracer
type SimpleTracer struct {
	spans []Span
}

// NewSimpleTracer creates a new simple tracer
func NewSimpleTracer() *SimpleTracer {
	return &SimpleTracer{
		spans: make([]Span, 0),
	}
}

func (t *SimpleTracer) StartSpan(ctx context.Context, operation string) (context.Context, *Span) {
	// Get parent span from context
	parentSpan := SpanFromContext(ctx)
	
	span := &Span{
		SpanID:    uuid.New().String(),
		Operation: operation,
		StartTime: time.Now(),
		Tags:      make(map[string]string),
		Status:    SpanStatusUnset,
	}
	
	if parentSpan != nil {
		span.TraceID = parentSpan.TraceID
		span.ParentID = parentSpan.SpanID
	} else {
		span.TraceID = uuid.New().String()
	}
	
	// Store span in context
	ctx = ContextWithSpan(ctx, span)
	
	return ctx, span
}

func (t *SimpleTracer) FinishSpan(span *Span) {
	if span == nil {
		return
	}
	
	span.EndTime = time.Now()
	t.spans = append(t.spans, *span)
}

func (t *SimpleTracer) AddTags(span *Span, tags map[string]string) {
	if span == nil {
		return
	}
	
	for k, v := range tags {
		span.Tags[k] = v
	}
}

func (t *SimpleTracer) SetStatus(span *Span, status SpanStatus) {
	if span == nil {
		return
	}
	
	span.Status = status
}

// Context keys for tracing
type contextKey string

const (
	spanContextKey contextKey = "span"
)

// ContextWithSpan returns a new context with the span
func ContextWithSpan(ctx context.Context, span *Span) context.Context {
	return context.WithValue(ctx, spanContextKey, span)
}

// SpanFromContext returns the span from the context
func SpanFromContext(ctx context.Context) *Span {
	if span, ok := ctx.Value(spanContextKey).(*Span); ok {
		return span
	}
	return nil
}

// TracingMiddleware creates a Gortex middleware for distributed tracing
func TracingMiddleware(tracer Tracer) middleware.MiddlewareFunc {
	return func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(c gortexContext.Context) error {
			// Extract trace context from headers
			traceID := c.Request().Header.Get("X-Trace-ID")
			if traceID == "" {
				traceID = uuid.New().String()
			}
			
			// Start span
			ctx, span := tracer.StartSpan(c.Request().Context(), fmt.Sprintf("%s %s", c.Request().Method, c.Path()))
			
			// Add tags
			tracer.AddTags(span, map[string]string{
				"http.method":     c.Request().Method,
				"http.path":       c.Path(),
				"http.url":        c.Request().URL.String(),
				"http.user_agent": c.Request().UserAgent(),
				"peer.address":    c.RealIP(),
			})
			
			// Set trace ID in response header
			c.Response().Header().Set("X-Trace-ID", span.TraceID)
			
			// Update request context
			c.SetRequest(c.Request().WithContext(ctx))
			
			// Process request
			err := next(c)
			
			// Set span status
			if err != nil {
				tracer.SetStatus(span, SpanStatusError)
				tracer.AddTags(span, map[string]string{
					"error": err.Error(),
				})
			} else {
				tracer.SetStatus(span, SpanStatusOK)
			}
			
			// Add response tags
			tracer.AddTags(span, map[string]string{
				"http.status_code": fmt.Sprintf("%d", c.Response().Status()),
			})
			
			// Finish span
			tracer.FinishSpan(span)
			
			return err
		}
	}
}

// StartSpanFromContext is a helper function to start a span from context
func StartSpanFromContext(ctx context.Context, tracer Tracer, operation string) (context.Context, *Span) {
	return tracer.StartSpan(ctx, operation)
}

// TraceFunction is a helper to trace a function execution
func TraceFunction(ctx context.Context, tracer Tracer, operation string, fn func(context.Context) error) error {
	ctx, span := tracer.StartSpan(ctx, operation)
	defer tracer.FinishSpan(span)
	
	err := fn(ctx)
	if err != nil {
		tracer.SetStatus(span, SpanStatusError)
		tracer.AddTags(span, map[string]string{
			"error": err.Error(),
		})
	} else {
		tracer.SetStatus(span, SpanStatusOK)
	}
	
	return err
}