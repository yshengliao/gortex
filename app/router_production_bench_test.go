//go:build production
// +build production

package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Production benchmarks to compare with reflection-based routing
func BenchmarkProductionRouting(b *testing.B) {
	e := echo.New()
	logger, _ := zap.NewProduction()
	ctx := NewContext()
	Register(ctx, logger)

	handler := &TestCodegenHandler{Logger: logger}
	manager := &HandlersManager{API: handler}
	
	// Register routes using static registration (production mode)
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

func BenchmarkProductionHTTPRequest(b *testing.B) {
	e := echo.New()
	logger, _ := zap.NewProduction()
	ctx := NewContext()
	Register(ctx, logger)

	handler := &TestCodegenHandler{Logger: logger}
	manager := &HandlersManager{API: handler}
	
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