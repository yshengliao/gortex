package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkServeHTTP_StaticRoute benchmarks a static route (no params).
func BenchmarkServeHTTP_StaticRoute(b *testing.B) {
	router := NewGortexRouter()
	router.GET("/api/v1/health", func(c Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkServeHTTP_ParamRoute benchmarks a route with one path parameter.
func BenchmarkServeHTTP_ParamRoute(b *testing.B) {
	router := NewGortexRouter()
	router.GET("/users/:id", func(c Context) error {
		_ = c.Param("id")
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/users/12345", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkServeHTTP_DeepParamRoute benchmarks a deeply nested route with multiple params.
func BenchmarkServeHTTP_DeepParamRoute(b *testing.B) {
	router := NewGortexRouter()
	router.GET("/api/v1/users/:userId/posts/:postId/comments/:commentId", func(c Context) error {
		_ = c.Param("userId")
		_ = c.Param("postId")
		_ = c.Param("commentId")
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/42/posts/99/comments/7", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkServeHTTP_WildcardRoute benchmarks a wildcard route.
func BenchmarkServeHTTP_WildcardRoute(b *testing.B) {
	router := NewGortexRouter()
	router.GET("/static/*", func(c Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/static/css/main.css", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkServeHTTP_JSON benchmarks JSON response (includes serialisation allocs).
func BenchmarkServeHTTP_JSON(b *testing.B) {
	router := NewGortexRouter()
	router.GET("/api/data", func(c Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		router.ServeHTTP(w, req)
	}
}
