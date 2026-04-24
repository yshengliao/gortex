package http_test

import (
	"net/http/httptest"
	"testing"

	ghttp "github.com/yshengliao/gortex/transport/http"
)

// BenchmarkGortexRouterStatic measures static route lookup performance.
func BenchmarkGortexRouterStatic(b *testing.B) {
	r := ghttp.NewGortexRouter()
	r.GET("/user/home", func(c ghttp.Context) error { return nil })

	req := httptest.NewRequest("GET", "/user/home", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

// BenchmarkGortexRouterParam measures parameterised route lookup performance.
func BenchmarkGortexRouterParam(b *testing.B) {
	r := ghttp.NewGortexRouter()
	r.GET("/user/:name", func(c ghttp.Context) error {
		c.Param("name")
		return nil
	})

	req := httptest.NewRequest("GET", "/user/john", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

// BenchmarkGortexRouterManyRoutes measures performance with many registered routes.
func BenchmarkGortexRouterManyRoutes(b *testing.B) {
	r := ghttp.NewGortexRouter()

	routes := []string{
		"/", "/api", "/api/users", "/api/users/:id", "/api/users/:id/posts",
		"/api/users/:id/posts/:postId", "/api/posts", "/api/posts/:id",
		"/static/*filepath", "/health", "/metrics", "/admin", "/admin/users",
		"/admin/settings", "/blog", "/blog/:year/:month/:day/:slug",
		"/products", "/products/:category", "/products/:category/:id",
		"/cart", "/checkout", "/orders", "/orders/:id",
	}
	for _, route := range routes {
		r.GET(route, func(c ghttp.Context) error { return nil })
	}

	req := httptest.NewRequest("GET", "/api/users/123/posts/456", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}
