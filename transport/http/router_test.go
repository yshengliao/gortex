package http

import (
	"net/http/httptest"
	"testing"
)

func TestRouterBasic(t *testing.T) {
	r := NewRouter()
	
	// Test basic route
	r.GET("/hello", func(c Context) error {
		return c.String(200, "Hello World")
	})
	
	req := httptest.NewRequest("GET", "/hello", nil)
	rec := httptest.NewRecorder()
	
	r.(*router).ServeHTTP(rec, req)
	
	if rec.Code != 200 {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	if rec.Body.String() != "Hello World" {
		t.Errorf("Expected body 'Hello World', got %s", rec.Body.String())
	}
}

func TestRouterParams(t *testing.T) {
	r := NewRouter()
	
	// Test param route
	r.GET("/users/:id", func(c Context) error {
		id := c.Param("id")
		return c.String(200, "User: "+id)
	})
	
	req := httptest.NewRequest("GET", "/users/123", nil)
	rec := httptest.NewRecorder()
	
	r.(*router).ServeHTTP(rec, req)
	
	if rec.Code != 200 {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	if rec.Body.String() != "User: 123" {
		t.Errorf("Expected body 'User: 123', got %s", rec.Body.String())
	}
}

func TestRouterMultipleParams(t *testing.T) {
	r := NewRouter()
	
	// Test multiple params
	r.GET("/users/:id/posts/:postId", func(c Context) error {
		userId := c.Param("id")
		postId := c.Param("postId")
		return c.String(200, "User: "+userId+", Post: "+postId)
	})
	
	req := httptest.NewRequest("GET", "/users/123/posts/456", nil)
	rec := httptest.NewRecorder()
	
	r.(*router).ServeHTTP(rec, req)
	
	if rec.Code != 200 {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	expected := "User: 123, Post: 456"
	if rec.Body.String() != expected {
		t.Errorf("Expected body '%s', got %s", expected, rec.Body.String())
	}
}

func TestRouterWildcard(t *testing.T) {
	r := NewRouter()
	
	// Test wildcard route
	r.GET("/static/*filepath", func(c Context) error {
		path := c.Param("filepath")
		return c.String(200, "File: "+path)
	})
	
	req := httptest.NewRequest("GET", "/static/css/style.css", nil)
	rec := httptest.NewRecorder()
	
	r.(*router).ServeHTTP(rec, req)
	
	if rec.Code != 200 {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	if rec.Body.String() != "File: css/style.css" {
		t.Errorf("Expected body 'File: css/style.css', got %s", rec.Body.String())
	}
}

func TestRouterNotFound(t *testing.T) {
	r := NewRouter()
	
	r.GET("/exists", func(c Context) error {
		return c.String(200, "Found")
	})
	
	req := httptest.NewRequest("GET", "/notfound", nil)
	rec := httptest.NewRecorder()
	
	r.(*router).ServeHTTP(rec, req)
	
	if rec.Code != 404 {
		t.Errorf("Expected status 404, got %d", rec.Code)
	}
}

func TestRouterMethods(t *testing.T) {
	r := NewRouter()
	
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	
	for _, method := range methods {
		switch method {
		case "GET":
			r.GET("/test", func(c Context) error {
				return c.String(200, method)
			})
		case "POST":
			r.POST("/test", func(c Context) error {
				return c.String(200, method)
			})
		case "PUT":
			r.PUT("/test", func(c Context) error {
				return c.String(200, method)
			})
		case "DELETE":
			r.DELETE("/test", func(c Context) error {
				return c.String(200, method)
			})
		case "PATCH":
			r.PATCH("/test", func(c Context) error {
				return c.String(200, method)
			})
		case "HEAD":
			r.HEAD("/test", func(c Context) error {
				return c.String(200, method)
			})
		case "OPTIONS":
			r.OPTIONS("/test", func(c Context) error {
				return c.String(200, method)
			})
		}
		
		req := httptest.NewRequest(method, "/test", nil)
		rec := httptest.NewRecorder()
		
		r.(*router).ServeHTTP(rec, req)
		
		if rec.Code != 200 {
			t.Errorf("%s: Expected status 200, got %d", method, rec.Code)
		}
		
		if rec.Body.String() != method {
			t.Errorf("%s: Expected body '%s', got %s", method, method, rec.Body.String())
		}
	}
}

func TestRouterGroups(t *testing.T) {
	r := NewRouter()
	
	// Create API group
	api := r.Group("/api")
	api.GET("/users", func(c Context) error {
		return c.String(200, "API Users")
	})
	
	// Create v1 group
	v1 := api.Group("/v1")
	v1.GET("/products", func(c Context) error {
		return c.String(200, "API V1 Products")
	})
	
	tests := []struct {
		path     string
		expected string
		code     int
	}{
		{"/api/users", "API Users", 200},
		{"/api/v1/products", "API V1 Products", 200},
		{"/api/v1/notfound", "", 404},
	}
	
	for _, test := range tests {
		req := httptest.NewRequest("GET", test.path, nil)
		rec := httptest.NewRecorder()
		
		r.(*router).ServeHTTP(rec, req)
		
		if rec.Code != test.code {
			t.Errorf("%s: Expected status %d, got %d", test.path, test.code, rec.Code)
		}
		
		if test.code == 200 && rec.Body.String() != test.expected {
			t.Errorf("%s: Expected body '%s', got %s", test.path, test.expected, rec.Body.String())
		}
	}
}