package router

import (
	"net/http/httptest"
	"testing"

	"github.com/yshengliao/gortex/context"
	"github.com/yshengliao/gortex/middleware"
)

func TestGortexRouter_Basic(t *testing.T) {
	router := NewGortexRouter()
	
	// Test basic route registration
	router.GET("/users", func(c context.Context) error {
		return c.String(200, "users")
	})
	
	router.POST("/users", func(c context.Context) error {
		return c.String(201, "created")
	})
	
	// Test route with parameter
	router.GET("/users/:id", func(c context.Context) error {
		id := c.Param("id")
		return c.String(200, "user "+id)
	})
	
	// Test GET /users
	req := httptest.NewRequest("GET", "/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// Test POST /users
	req = httptest.NewRequest("POST", "/users", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 201 {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
	
	// Test GET /users/123
	req = httptest.NewRequest("GET", "/users/123", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGortexRouter_Group(t *testing.T) {
	router := NewGortexRouter()
	
	// Test route group
	api := router.Group("/api/v1")
	api.GET("/users", func(c context.Context) error {
		return c.String(200, "api users")
	})
	
	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGortexRouter_Middleware(t *testing.T) {
	router := NewGortexRouter()
	
	var executed []string
	
	// Global middleware
	router.Use(func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(c context.Context) error {
			executed = append(executed, "global")
			return next(c)
		}
	})
	
	// Route with middleware
	router.GET("/test", func(c context.Context) error {
		executed = append(executed, "handler")
		return c.String(200, "test")
	}, func(next middleware.HandlerFunc) middleware.HandlerFunc {
		return func(c context.Context) error {
			executed = append(executed, "route")
			return next(c)
		}
	})
	
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// Check middleware execution order
	expectedOrder := []string{"global", "route", "handler"}
	if len(executed) != len(expectedOrder) {
		t.Errorf("Expected %d middleware executions, got %d", len(expectedOrder), len(executed))
	}
	
	for i, expected := range expectedOrder {
		if i >= len(executed) || executed[i] != expected {
			t.Errorf("Expected middleware order %v, got %v", expectedOrder, executed)
			break
		}
	}
}
