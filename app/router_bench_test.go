package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Test handlers for benchmarking
type BenchHandler struct {
	Logger *zap.Logger
}

func (h *BenchHandler) GET(c echo.Context) error {
	return c.JSON(200, map[string]string{"message": "benchmark"})
}

func (h *BenchHandler) POST(c echo.Context) error {
	return c.JSON(200, map[string]string{"message": "post"})
}

func (h *BenchHandler) CustomMethod(c echo.Context) error {
	return c.JSON(200, map[string]string{"message": "custom"})
}

type BenchHandlersManager struct {
	API *BenchHandler `url:"/api"`
}

// Direct registration for comparison (what static generation should produce)
func registerDirectRoutes(e *echo.Echo, handler *BenchHandler) {
	e.GET("/api", handler.GET)
	e.POST("/api", handler.POST)
	e.POST("/api/custom-method", handler.CustomMethod)
}

func BenchmarkReflectionRouting(b *testing.B) {
	e := echo.New()
	logger, _ := zap.NewProduction()
	ctx := NewContext()
	Register(ctx, logger)

	handler := &BenchHandler{Logger: logger}
	manager := &BenchHandlersManager{API: handler}
	
	// Register routes using reflection (current implementation)
	if err := RegisterRoutes(e, manager, ctx); err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			e.Router().Find(http.MethodGet, "/api", c)
			if c.Handler() != nil {
				c.Handler()(c)
			}
		}
	})
}

func BenchmarkDirectRouting(b *testing.B) {
	e := echo.New()
	logger, _ := zap.NewProduction()
	handler := &BenchHandler{Logger: logger}
	
	// Register routes directly (what we want to achieve with code generation)
	registerDirectRoutes(e, handler)

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			e.Router().Find(http.MethodGet, "/api", c)
			if c.Handler() != nil {
				c.Handler()(c)
			}
		}
	})
}

// Benchmark full HTTP request processing
func BenchmarkReflectionHTTPRequest(b *testing.B) {
	e := echo.New()
	logger, _ := zap.NewProduction()
	ctx := NewContext()
	Register(ctx, logger)

	handler := &BenchHandler{Logger: logger}
	manager := &BenchHandlersManager{API: handler}
	
	if err := RegisterRoutes(e, manager, ctx); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/api", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
		}
	})
}

func BenchmarkDirectHTTPRequest(b *testing.B) {
	e := echo.New()
	logger, _ := zap.NewProduction()
	handler := &BenchHandler{Logger: logger}
	
	registerDirectRoutes(e, handler)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/api", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
		}
	})
}

// Test reflection overhead in method calling
func BenchmarkReflectionMethodCall(b *testing.B) {
	logger, _ := zap.NewProduction()
	handler := &BenchHandler{Logger: logger}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/api", nil)
			rec := httptest.NewRecorder()
			e := echo.New()
			c := e.NewContext(req, rec)
			
			// Direct method call
			handler.GET(c)
		}
	})
}