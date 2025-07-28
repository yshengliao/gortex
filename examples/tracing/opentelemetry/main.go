package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yshengliao/gortex/pkg/config"
	"github.com/yshengliao/gortex/core/app"
	"github.com/yshengliao/gortex/observability/otel"
	"github.com/yshengliao/gortex/observability/tracing"
	httpctx "github.com/yshengliao/gortex/transport/http"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Handler definitions
type HandlersManager struct {
	Home *HomeHandler `url:"/"`
	API  *APIHandler  `url:"/api/data"`
}

type HomeHandler struct{}

func (h *HomeHandler) GET(c httpctx.Context) error {
	// Access enhanced span
	if span := c.Span(); span != nil {
		if enhancedSpan, ok := span.(*tracing.EnhancedSpan); ok {
			enhancedSpan.LogEvent(tracing.SpanStatusINFO, "Home page accessed via OpenTelemetry", map[string]any{
				"exporter": "otlp",
			})
		}
	}
	
	return c.JSON(200, map[string]string{
		"message": "Gortex with OpenTelemetry",
		"status":  "operational",
	})
}

type APIHandler struct{}

func (h *APIHandler) GET(c httpctx.Context) error {
	// Simulate some processing with events
	if span := c.Span(); span != nil {
		if enhancedSpan, ok := span.(*tracing.EnhancedSpan); ok {
			enhancedSpan.LogEvent(tracing.SpanStatusDEBUG, "Starting data fetch", nil)
			
			// Simulate processing
			time.Sleep(50 * time.Millisecond)
			
			enhancedSpan.LogEvent(tracing.SpanStatusINFO, "Data processed", map[string]any{
				"records": 100,
				"duration_ms": 50,
			})
		}
	}
	
	return c.JSON(200, map[string]any{
		"data": []string{"item1", "item2", "item3"},
		"timestamp": time.Now().Unix(),
	})
}

// initTracer initializes OpenTelemetry with OTLP exporter
func initTracer() (*otel.OTelTracerAdapter, func(), error) {
	ctx := context.Background()
	
	// Create OTLP exporter
	client := otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint("localhost:4317"),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithDialOption(grpc.WithBlock()),
	)
	
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create exporter: %w", err)
	}
	
	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("gortex-otel-example"),
			semconv.ServiceVersionKey.String("0.4.0-alpha"),
			semconv.DeploymentEnvironmentKey.String("development"),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource: %w", err)
	}
	
	// Create tracer provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	
	// Set global provider
	otelapi.SetTracerProvider(provider)
	
	// Create Gortex tracer adapter
	gortexTracer := tracing.NewSimpleTracer()
	adapter := otel.NewOTelTracerAdapter(gortexTracer, "gortex")
	
	// Return cleanup function
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := provider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}
	
	return adapter, cleanup, nil
}

func main() {
	// Initialize OpenTelemetry
	tracer, cleanup, err := initTracer()
	if err != nil {
		log.Fatal("Failed to initialize tracer:", err)
	}
	defer cleanup()
	
	// Create logger
	logger, _ := zap.NewProduction()
	
	// Create configuration
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:         "8085",
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		Logger: config.LoggerConfig{
			Level:    "info",
			Encoding: "json",
		},
	}
	
	// Create handlers
	handlers := &HandlersManager{
		Home: &HomeHandler{},
		API:  &APIHandler{},
	}
	
	// Create app with OpenTelemetry tracer
	gortexApp, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithHandlers(handlers),
		app.WithLogger(logger),
		app.WithTracer(tracer), // Uses OTel adapter
	)
	if err != nil {
		log.Fatal("Failed to create app:", err)
	}
	
	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	fmt.Printf("OpenTelemetry example server starting on %s\n", addr)
	fmt.Println("\nMake sure OpenTelemetry Collector is running:")
	fmt.Println("  docker-compose up -d otel-collector")
	fmt.Println("\nTry these endpoints:")
	fmt.Println("  GET http://localhost:8085/")
	fmt.Println("  GET http://localhost:8085/api/data")
	fmt.Println("\nView traces in Jaeger UI:")
	fmt.Println("  http://localhost:16686")
	fmt.Println("\nPress Ctrl+C to stop")
	
	if err := gortexApp.Run(addr); err != nil {
		log.Fatal("Server failed:", err)
	}
}