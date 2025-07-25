package router

import (
	"net/http/httptest"
	"testing"
)

func BenchmarkRouterStatic(b *testing.B) {
	r := NewRouter()
	r.GET("/user/home", func(c Context) error {
		return nil
	})
	
	req := httptest.NewRequest("GET", "/user/home", nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		r.(*router).ServeHTTP(rec, req)
	}
}

func BenchmarkRouterParam(b *testing.B) {
	r := NewRouter()
	r.GET("/user/:name", func(c Context) error {
		c.Param("name")
		return nil
	})
	
	req := httptest.NewRequest("GET", "/user/john", nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		r.(*router).ServeHTTP(rec, req)
	}
}

func BenchmarkRouterMultipleParams(b *testing.B) {
	r := NewRouter()
	r.GET("/user/:name/post/:id", func(c Context) error {
		c.Param("name")
		c.Param("id")
		return nil
	})
	
	req := httptest.NewRequest("GET", "/user/john/post/123", nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		r.(*router).ServeHTTP(rec, req)
	}
}

func BenchmarkRouterWildcard(b *testing.B) {
	r := NewRouter()
	r.GET("/static/*filepath", func(c Context) error {
		c.Param("filepath")
		return nil
	})
	
	req := httptest.NewRequest("GET", "/static/css/style.css", nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		r.(*router).ServeHTTP(rec, req)
	}
}

func BenchmarkRouterManyRoutes(b *testing.B) {
	r := NewRouter()
	
	// Add many routes to test tree performance
	routes := []string{
		"/",
		"/api",
		"/api/users",
		"/api/users/:id",
		"/api/users/:id/posts",
		"/api/users/:id/posts/:postId",
		"/api/posts",
		"/api/posts/:id",
		"/static/*filepath",
		"/health",
		"/metrics",
		"/admin",
		"/admin/users",
		"/admin/settings",
		"/blog",
		"/blog/:year/:month/:day/:slug",
		"/products",
		"/products/:category",
		"/products/:category/:id",
		"/cart",
		"/checkout",
		"/orders",
		"/orders/:id",
	}
	
	for _, route := range routes {
		r.GET(route, func(c Context) error {
			return nil
		})
	}
	
	req := httptest.NewRequest("GET", "/api/users/123/posts/456", nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		r.(*router).ServeHTTP(rec, req)
	}
}

// Benchmark just the route lookup without handler execution
func BenchmarkRouterLookup(b *testing.B) {
	r := NewRouter()
	r.GET("/user/:name/post/:id", func(c Context) error {
		return nil
	})
	
	root := r.(*router).trees["GET"]
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		params := make(map[string]string)
		root.search("/user/john/post/123", params)
	}
}