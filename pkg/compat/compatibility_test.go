package compat

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	
	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/pkg/router"
)

// TestEchoHandlerWrapping tests wrapping Echo handlers to Gortex
func TestEchoHandlerWrapping(t *testing.T) {
	// Echo handler
	echoHandler := func(c echo.Context) error {
		name := c.QueryParam("name")
		return c.String(http.StatusOK, "Hello, "+name)
	}
	
	// Wrap to Gortex handler
	gortexHandler := WrapEchoHandler(echoHandler)
	
	// Create test request
	req := httptest.NewRequest("GET", "/?name=World", nil)
	rec := httptest.NewRecorder()
	
	// Create context
	ctx := router.NewContext(rec, req)
	
	// Execute handler
	err := gortexHandler(ctx)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	// Check response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
	
	expected := "Hello, World"
	if strings.TrimSpace(rec.Body.String()) != expected {
		t.Errorf("Expected body %q, got %q", expected, rec.Body.String())
	}
}

// TestGortexHandlerWrapping tests wrapping Gortex handlers to Echo
func TestGortexHandlerWrapping(t *testing.T) {
	// Gortex handler
	gortexHandler := func(c router.Context) error {
		name := c.QueryParam("name")
		return c.String(http.StatusOK, "Hello, "+name)
	}
	
	// Wrap to Echo handler
	echoHandler := WrapGortexHandler(gortexHandler)
	
	// Create Echo instance and context
	e := echo.New()
	req := httptest.NewRequest("GET", "/?name=Gortex", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	// Execute handler
	err := echoHandler(c)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	// Check response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
	
	expected := "Hello, Gortex"
	if strings.TrimSpace(rec.Body.String()) != expected {
		t.Errorf("Expected body %q, got %q", expected, rec.Body.String())
	}
}

// TestContextValueSharing tests context value sharing between wrappers
func TestContextValueSharing(t *testing.T) {
	// Echo handler that sets and gets context values
	echoHandler := func(c echo.Context) error {
		c.Set("user", "john")
		c.Set("role", "admin")
		
		user := c.Get("user").(string)
		role := c.Get("role").(string)
		
		return c.String(http.StatusOK, user+":"+role)
	}
	
	// Wrap and test
	gortexHandler := WrapEchoHandler(echoHandler)
	
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	ctx := router.NewContext(rec, req)
	
	err := gortexHandler(ctx)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	expected := "john:admin"
	if strings.TrimSpace(rec.Body.String()) != expected {
		t.Errorf("Expected body %q, got %q", expected, rec.Body.String())
	}
}