package tracing

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	gortexContext "github.com/yshengliao/gortex/transport/http"
	"github.com/yshengliao/gortex/middleware"
)

// Event represents an event within a span
type Event struct {
	Timestamp time.Time
	Severity  SpanStatus
	Message   string
	Fields    map[string]any
}

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
	Events    []Event // New field for events
	Error     error   // New field for error tracking
}

// SpanStatus represents the status/severity level of a span
type SpanStatus int

const (
	// Original statuses for backward compatibility
	SpanStatusUnset SpanStatus = iota
	SpanStatusOK
	SpanStatusError
	
	// Extended severity levels (8 levels)
	SpanStatusDEBUG     SpanStatus = 10
	SpanStatusINFO      SpanStatus = 20
	SpanStatusNOTICE    SpanStatus = 30
	SpanStatusWARN      SpanStatus = 40
	SpanStatusERROR     SpanStatus = 50
	SpanStatusCRITICAL  SpanStatus = 60
	SpanStatusALERT     SpanStatus = 70
	SpanStatusEMERGENCY SpanStatus = 80
)

// String returns the string representation of the SpanStatus
func (s SpanStatus) String() string {
	switch s {
	case SpanStatusUnset:
		return "UNSET"
	case SpanStatusOK:
		return "OK"
	case SpanStatusError:
		return "ERROR"
	case SpanStatusDEBUG:
		return "DEBUG"
	case SpanStatusINFO:
		return "INFO"
	case SpanStatusNOTICE:
		return "NOTICE"
	case SpanStatusWARN:
		return "WARN"
	case SpanStatusERROR:
		return "ERROR"
	case SpanStatusCRITICAL:
		return "CRITICAL"
	case SpanStatusALERT:
		return "ALERT"
	case SpanStatusEMERGENCY:
		return "EMERGENCY"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

// IsMoreSevere returns true if this status is more severe than the other
func (s SpanStatus) IsMoreSevere(other SpanStatus) bool {
	// Map old statuses to severity levels
	sSeverity := s.severityLevel()
	otherSeverity := other.severityLevel()
	return sSeverity > otherSeverity
}

// severityLevel returns the numeric severity level for comparison
func (s SpanStatus) severityLevel() int {
	switch s {
	case SpanStatusUnset:
		return 0
	case SpanStatusOK:
		return 5 // Between UNSET and DEBUG
	case SpanStatusError:
		return 50 // Same as SpanStatusERROR
	default:
		return int(s)
	}
}

// SpanInterface represents the basic span operations
type SpanInterface interface {
	// LogEvent logs an event with severity level
	LogEvent(severity SpanStatus, msg string, fields map[string]any)
	
	// SetError sets an error on the span
	SetError(err error)
	
	// AddTags adds tags to the span
	AddTags(tags map[string]string)
	
	// SetStatus sets the status of the span
	SetStatus(status SpanStatus)
}

// EnhancedSpan implements SpanInterface with extended functionality
type EnhancedSpan struct {
	*Span
}

// LogEvent logs an event with the specified severity
func (s *EnhancedSpan) LogEvent(severity SpanStatus, msg string, fields map[string]any) {
	if s.Span == nil {
		return
	}
	
	event := Event{
		Timestamp: time.Now(),
		Severity:  severity,
		Message:   msg,
		Fields:    fields,
	}
	
	s.Events = append(s.Events, event)
}

// SetError sets an error on the span and updates status
func (s *EnhancedSpan) SetError(err error) {
	if s.Span == nil || err == nil {
		return
	}
	
	s.Error = err
	s.Status = SpanStatusERROR
	
	// Log error as an event
	s.LogEvent(SpanStatusERROR, "Error occurred", map[string]any{
		"error": err.Error(),
	})
}

// AddTags adds tags to the span
func (s *EnhancedSpan) AddTags(tags map[string]string) {
	if s.Span == nil {
		return
	}
	
	for k, v := range tags {
		s.Tags[k] = v
	}
}

// SetStatus sets the status of the span
func (s *EnhancedSpan) SetStatus(status SpanStatus) {
	if s.Span == nil {
		return
	}
	
	s.Status = status
}

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

// EnhancedTracer extends Tracer with enhanced span support
type EnhancedTracer interface {
	Tracer
	
	// StartEnhancedSpan starts a new enhanced span
	StartEnhancedSpan(ctx context.Context, operation string) (context.Context, *EnhancedSpan)
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
		Events:    make([]Event, 0),
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

// StartEnhancedSpan starts a new enhanced span
func (t *SimpleTracer) StartEnhancedSpan(ctx context.Context, operation string) (context.Context, *EnhancedSpan) {
	ctx, span := t.StartSpan(ctx, operation)
	enhancedSpan := &EnhancedSpan{Span: span}
	
	// Update context with enhanced span
	ctx = ContextWithEnhancedSpan(ctx, enhancedSpan)
	
	return ctx, enhancedSpan
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
	spanContextKey         contextKey = "span"
	enhancedSpanContextKey contextKey = "enhanced_span"
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

// ContextWithEnhancedSpan returns a new context with the enhanced span
func ContextWithEnhancedSpan(ctx context.Context, span *EnhancedSpan) context.Context {
	// Also store as regular span for backward compatibility
	ctx = ContextWithSpan(ctx, span.Span)
	return context.WithValue(ctx, enhancedSpanContextKey, span)
}

// EnhancedSpanFromContext returns the enhanced span from the context
func EnhancedSpanFromContext(ctx context.Context) *EnhancedSpan {
	if span, ok := ctx.Value(enhancedSpanContextKey).(*EnhancedSpan); ok {
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
			
			// Start span - use EnhancedTracer if available
			var ctx context.Context
			var span *Span
			var enhancedSpan *EnhancedSpan
			
			if enhancedTracer, ok := tracer.(EnhancedTracer); ok {
				ctx, enhancedSpan = enhancedTracer.StartEnhancedSpan(c.Request().Context(), fmt.Sprintf("%s %s", c.Request().Method, c.Path()))
				span = enhancedSpan.Span
				// Store enhanced span in Gortex context
				c.Set("enhanced_span", enhancedSpan)
			} else {
				ctx, span = tracer.StartSpan(c.Request().Context(), fmt.Sprintf("%s %s", c.Request().Method, c.Path()))
			}
			
			// Store span in Gortex context for easy access
			c.Set("span", span)
			
			// Add tags
			tags := map[string]string{
				"http.method":     c.Request().Method,
				"http.path":       c.Path(),
				"http.url":        c.Request().URL.String(),
				"http.user_agent": c.Request().UserAgent(),
				"peer.address":    c.RealIP(),
			}
			
			if enhancedSpan != nil {
				enhancedSpan.AddTags(tags)
			} else {
				tracer.AddTags(span, tags)
			}
			
			// Set trace ID in response header
			c.Response().Header().Set("X-Trace-ID", span.TraceID)
			
			// Update request context
			c.SetRequest(c.Request().WithContext(ctx))
			
			// Process request
			err := next(c)
			
			// Set span status and handle errors
			if err != nil {
				if enhancedSpan != nil {
					enhancedSpan.SetError(err)
				} else {
					tracer.SetStatus(span, SpanStatusError)
					tracer.AddTags(span, map[string]string{
						"error": err.Error(),
					})
				}
			} else {
				if enhancedSpan != nil {
					enhancedSpan.SetStatus(SpanStatusOK)
				} else {
					tracer.SetStatus(span, SpanStatusOK)
				}
			}
			
			// Add response tags
			responseTags := map[string]string{
				"http.status_code": fmt.Sprintf("%d", c.Response().Status()),
			}
			
			if enhancedSpan != nil {
				enhancedSpan.AddTags(responseTags)
			} else {
				tracer.AddTags(span, responseTags)
			}
			
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