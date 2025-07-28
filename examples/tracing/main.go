package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yshengliao/gortex/pkg/config"
	"github.com/yshengliao/gortex/core/app"
	"github.com/yshengliao/gortex/observability/tracing"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Handler definitions with struct tags
type HandlersManager struct {
	Home      *HomeHandler      `url:"/"`
	Users     *UsersHandler     `url:"/users"`
	UserByID  *UserByIDHandler  `url:"/users/:id"`
	Products  *ProductsHandler  `url:"/products"`
	Order     *OrderHandler     `url:"/order"`
	Analytics *AnalyticsHandler `url:"/analytics"`
}

// HomeHandler demonstrates basic tracing
type HomeHandler struct{}

func (h *HomeHandler) GET(c httpctx.Context) error {
	// Get span from context
	if span := c.Span(); span != nil {
		if enhancedSpan, ok := span.(*tracing.EnhancedSpan); ok {
			// Log a debug event
			enhancedSpan.LogEvent(tracing.SpanStatusDEBUG, "Home page accessed", map[string]any{
				"user_agent": c.Request().UserAgent(),
				"ip":         c.RealIP(),
			})
		}
	}
	
	return c.JSON(200, map[string]string{
		"message": "Welcome to Gortex Tracing Example",
		"version": "0.4.0-alpha",
	})
}

// UsersHandler demonstrates child spans
type UsersHandler struct {
	tracer tracing.Tracer
}

func NewUsersHandler(tracer tracing.Tracer) *UsersHandler {
	return &UsersHandler{tracer: tracer}
}

func (h *UsersHandler) GET(c httpctx.Context) error {
	// Create a child span for database operation
	var dbSpan *tracing.Span
	var enhancedDbSpan *tracing.EnhancedSpan
	ctx := c.Request().Context()
	
	if enhancedTracer, ok := h.tracer.(tracing.EnhancedTracer); ok {
		ctx, enhancedDbSpan = enhancedTracer.StartEnhancedSpan(ctx, "database.query")
		dbSpan = enhancedDbSpan.Span
		defer h.tracer.FinishSpan(dbSpan)
	} else {
		ctx, dbSpan = h.tracer.StartSpan(ctx, "database.query")
		defer h.tracer.FinishSpan(dbSpan)
	}
	
	// Simulate database query
	users, err := h.fetchUsers(ctx)
	if err != nil {
		if enhancedDbSpan != nil {
			enhancedDbSpan.SetError(err)
		}
		return err
	}
	
	// Add tags to database span
	h.tracer.AddTags(dbSpan, map[string]string{
		"db.type":     "postgresql",
		"db.query":    "SELECT * FROM users",
		"result.size": fmt.Sprintf("%d", len(users)),
	})
	
	// Log an info event in parent span
	if span := c.Span(); span != nil {
		if enhancedSpan, ok := span.(*tracing.EnhancedSpan); ok {
			enhancedSpan.LogEvent(tracing.SpanStatusINFO, "Users fetched successfully", map[string]any{
				"count": len(users),
			})
		}
	}
	
	return c.JSON(200, users)
}

func (h *UsersHandler) fetchUsers(ctx context.Context) ([]map[string]any, error) {
	// Simulate database delay
	select {
	case <-time.After(50 * time.Millisecond):
		return []map[string]any{
			{"id": 1, "name": "Alice", "email": "alice@example.com"},
			{"id": 2, "name": "Bob", "email": "bob@example.com"},
			{"id": 3, "name": "Charlie", "email": "charlie@example.com"},
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// UserByIDHandler demonstrates error tracking
type UserByIDHandler struct{}

func (h *UserByIDHandler) GET(c httpctx.Context) error {
	userID := c.Param("id")
	
	// Simulate error for specific user ID
	if userID == "999" {
		err := fmt.Errorf("user not found: %s", userID)
		
		// Log error event in span
		if span := c.Span(); span != nil {
			if enhancedSpan, ok := span.(*tracing.EnhancedSpan); ok {
				enhancedSpan.SetError(err)
				enhancedSpan.LogEvent(tracing.SpanStatusWARN, "User lookup failed", map[string]any{
					"user_id": userID,
					"reason":  "not_found",
				})
			}
		}
		
		return httpctx.NewHTTPError(404, err.Error())
	}
	
	// Log successful lookup
	if span := c.Span(); span != nil {
		if enhancedSpan, ok := span.(*tracing.EnhancedSpan); ok {
			enhancedSpan.LogEvent(tracing.SpanStatusINFO, "User found", map[string]any{
				"user_id": userID,
			})
		}
	}
	
	return c.JSON(200, map[string]any{
		"id":    userID,
		"name":  "Test User",
		"email": fmt.Sprintf("user%s@example.com", userID),
	})
}

// ProductsHandler demonstrates different severity levels
type ProductsHandler struct{}

func (h *ProductsHandler) GET(c httpctx.Context) error {
	if span := c.Span(); span != nil {
		if enhancedSpan, ok := span.(*tracing.EnhancedSpan); ok {
			// Log various severity levels
			enhancedSpan.LogEvent(tracing.SpanStatusDEBUG, "Starting product fetch", nil)
			enhancedSpan.LogEvent(tracing.SpanStatusINFO, "Cache miss, fetching from DB", map[string]any{
				"cache_key": "products:all",
			})
			enhancedSpan.LogEvent(tracing.SpanStatusNOTICE, "Large dataset requested", map[string]any{
				"size": 1000,
			})
		}
	}
	
	// Simulate some processing
	time.Sleep(100 * time.Millisecond)
	
	return c.JSON(200, map[string]any{
		"products": []map[string]any{
			{"id": 1, "name": "Product A", "price": 99.99},
			{"id": 2, "name": "Product B", "price": 149.99},
		},
		"total": 2,
	})
}

// OrderHandler demonstrates critical errors
type OrderHandler struct{}

func (h *OrderHandler) POST(c httpctx.Context) error {
	// Simulate a critical error
	if span := c.Span(); span != nil {
		if enhancedSpan, ok := span.(*tracing.EnhancedSpan); ok {
			enhancedSpan.LogEvent(tracing.SpanStatusCRITICAL, "Payment gateway unavailable", map[string]any{
				"gateway": "stripe",
				"error":   "connection_timeout",
			})
			enhancedSpan.SetError(fmt.Errorf("payment processing failed"))
		}
	}
	
	return httpctx.NewHTTPError(503, "Service temporarily unavailable")
}

// AnalyticsHandler demonstrates all severity levels
type AnalyticsHandler struct{}

func (h *AnalyticsHandler) GET(c httpctx.Context) error {
	if span := c.Span(); span != nil {
		if enhancedSpan, ok := span.(*tracing.EnhancedSpan); ok {
			// Demonstrate all severity levels
			enhancedSpan.LogEvent(tracing.SpanStatusDEBUG, "Analytics query started", map[string]any{
				"query_id": "q123",
			})
			enhancedSpan.LogEvent(tracing.SpanStatusINFO, "Executing aggregation pipeline", map[string]any{
				"stages": 5,
			})
			enhancedSpan.LogEvent(tracing.SpanStatusNOTICE, "Large dataset processing", map[string]any{
				"records": 50000,
			})
			enhancedSpan.LogEvent(tracing.SpanStatusWARN, "Slow query detected", map[string]any{
				"duration_ms": 2500,
			})
			enhancedSpan.LogEvent(tracing.SpanStatusERROR, "Partial data failure", map[string]any{
				"failed_shards": 2,
				"total_shards":  10,
			})
			enhancedSpan.LogEvent(tracing.SpanStatusCRITICAL, "Memory pressure detected", map[string]any{
				"used_mb": 900,
				"limit_mb": 1024,
			})
			enhancedSpan.LogEvent(tracing.SpanStatusALERT, "System reaching limits", map[string]any{
				"cpu_percent": 95,
			})
			enhancedSpan.LogEvent(tracing.SpanStatusEMERGENCY, "Immediate action required", map[string]any{
				"action": "scale_up",
			})
		}
	}
	
	return c.JSON(200, map[string]any{
		"analytics": "demo_data",
		"status":    "completed_with_warnings",
	})
}

func main() {
	// Create logger
	logConfig := zap.NewProductionConfig()
	logConfig.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	logger, _ := logConfig.Build()
	
	// Create configuration
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:         "8084",
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		Logger: config.LoggerConfig{
			Level:    "debug",
			Encoding: "json",
		},
	}
	
	// Create tracer - in production, you would use OpenTelemetry
	tracer := tracing.NewSimpleTracer()
	
	// Create handlers
	handlers := &HandlersManager{
		Home:      &HomeHandler{},
		Users:     NewUsersHandler(tracer),
		UserByID:  &UserByIDHandler{},
		Products:  &ProductsHandler{},
		Order:     &OrderHandler{},
		Analytics: &AnalyticsHandler{},
	}
	
	// Create app with tracer
	gortexApp, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithHandlers(handlers),
		app.WithLogger(logger),
		app.WithTracer(tracer), // This auto-injects TracingMiddleware
	)
	if err != nil {
		log.Fatal("Failed to create app:", err)
	}
	
	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	fmt.Printf("Tracing example server starting on %s\n", addr)
	fmt.Println("\nTry these endpoints:")
	fmt.Println("  GET  http://localhost:8084/           - Basic tracing")
	fmt.Println("  GET  http://localhost:8084/users      - Child spans")
	fmt.Println("  GET  http://localhost:8084/users/123  - Success case")
	fmt.Println("  GET  http://localhost:8084/users/999  - Error case")
	fmt.Println("  GET  http://localhost:8084/products   - Multiple events")
	fmt.Println("  POST http://localhost:8084/order      - Critical error")
	fmt.Println("  GET  http://localhost:8084/analytics  - All severity levels")
	fmt.Println("\nPress Ctrl+C to stop")
	
	if err := gortexApp.Run(addr); err != nil {
		log.Fatal("Server failed:", err)
	}
}