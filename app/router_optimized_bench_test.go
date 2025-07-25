package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Benchmark optimized routing
func BenchmarkOptimizedRouting(b *testing.B) {
	e := echo.New()
	logger, _ := zap.NewProduction()
	ctx := NewContext()
	Register(ctx, logger)

	handler := &BenchHandler{Logger: logger}
	manager := &BenchHandlersManager{API: handler}
	
	// Create app with optimized router
	app := &App{
		e:               e,
		ctx:             ctx,
		logger:          logger,
		runtimeMode:     ModeGortex,
		optimizedRouter: NewOptimizedRouter(e, ctx, logger),
	}
	
	// Register routes using optimized router
	if err := RegisterRoutesCompat(app, manager); err != nil {
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

func BenchmarkOptimizedHTTPRequest(b *testing.B) {
	e := echo.New()
	logger, _ := zap.NewProduction()
	ctx := NewContext()
	Register(ctx, logger)

	handler := &BenchHandler{Logger: logger}
	manager := &BenchHandlersManager{API: handler}
	
	// Create app with optimized router
	app := &App{
		e:               e,
		ctx:             ctx,
		logger:          logger,
		runtimeMode:     ModeGortex,
		optimizedRouter: NewOptimizedRouter(e, ctx, logger),
	}
	
	if err := RegisterRoutesCompat(app, manager); err != nil {
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

// Benchmark caching effectiveness
func BenchmarkCachingEffectiveness(b *testing.B) {
	// Clear cache before test
	ClearCache()
	
	e := echo.New()
	logger, _ := zap.NewProduction()
	ctx := NewContext()
	Register(ctx, logger)

	// Multiple handlers to test cache
	type TestHandlers struct {
		API1 *BenchHandler `url:"/api1"`
		API2 *BenchHandler `url:"/api2"`
		API3 *BenchHandler `url:"/api3"`
		API4 *BenchHandler `url:"/api4"`
		API5 *BenchHandler `url:"/api5"`
	}

	handlers := &TestHandlers{
		API1: &BenchHandler{Logger: logger},
		API2: &BenchHandler{Logger: logger},
		API3: &BenchHandler{Logger: logger},
		API4: &BenchHandler{Logger: logger},
		API5: &BenchHandler{Logger: logger},
	}
	
	app := &App{
		e:               e,
		ctx:             ctx,
		logger:          logger,
		runtimeMode:     ModeGortex,
		optimizedRouter: NewOptimizedRouter(e, ctx, logger),
	}
	
	b.ResetTimer()
	
	// First registration should be slower (building cache)
	b.Run("FirstRegistration", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ClearCache()
			e := echo.New()
			app := &App{
				e:               e,
				ctx:             ctx,
				logger:          logger,
				runtimeMode:     ModeGortex,
				optimizedRouter: NewOptimizedRouter(e, ctx, logger),
			}
			RegisterRoutesCompat(app, handlers)
		}
	})
	
	// Subsequent registrations should be faster (using cache)
	b.Run("CachedRegistration", func(b *testing.B) {
		// Prime the cache
		RegisterRoutesCompat(app, handlers)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			e := echo.New()
			app := &App{
				e:               e,
				ctx:             ctx,
				logger:          logger,
				runtimeMode:     ModeGortex,
				optimizedRouter: NewOptimizedRouter(e, ctx, logger),
			}
			RegisterRoutesCompat(app, handlers)
		}
	})
}

// Benchmark memory usage
func BenchmarkMemoryUsage(b *testing.B) {
	modes := []struct {
		name string
		mode RuntimeMode
	}{
		{"Reflection", ModeEcho},
		{"Optimized", ModeGortex},
	}
	
	for _, m := range modes {
		b.Run(m.name, func(b *testing.B) {
			b.ReportAllocs()
			
			e := echo.New()
			logger, _ := zap.NewProduction()
			ctx := NewContext()
			Register(ctx, logger)
			
			handler := &BenchHandler{Logger: logger}
			manager := &BenchHandlersManager{API: handler}
			
			app := &App{
				e:           e,
				ctx:         ctx,
				logger:      logger,
				runtimeMode: m.mode,
			}
			
			if m.mode == ModeGortex {
				app.optimizedRouter = NewOptimizedRouter(e, ctx, logger)
			}
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				RegisterRoutesCompat(app, manager)
			}
		})
	}
}