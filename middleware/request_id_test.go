package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/yshengliao/gortex/context"
)

func TestRequestID(t *testing.T) {
	// Create a test handler
	handler := func(c context.Context) error {
		// Verify request ID is set
		rid := GetRequestID(c)
		if rid == "" {
			t.Error("Request ID should be set")
		}
		return c.String(200, "test")
	}

	// Create middleware
	middleware := RequestID()
	wrappedHandler := middleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := context.NewContext(req, w)

	// Execute
	err := wrappedHandler(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check response header
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header should be set")
	}
}

func TestRequestIDWithExistingID(t *testing.T) {
	existingID := "existing-request-id"

	// Create a test handler
	handler := func(c context.Context) error {
		rid := GetRequestID(c)
		if rid != existingID {
			t.Errorf("Expected request ID %s, got %s", existingID, rid)
		}
		return c.String(200, "test")
	}

	// Create middleware
	middleware := RequestID()
	wrappedHandler := middleware(handler)

	// Create test request with existing request ID
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", existingID)
	w := httptest.NewRecorder()
	ctx := context.NewContext(req, w)

	// Execute
	err := wrappedHandler(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check response header matches existing ID
	if w.Header().Get("X-Request-ID") != existingID {
		t.Errorf("Expected X-Request-ID header %s, got %s", existingID, w.Header().Get("X-Request-ID"))
	}
}

func TestRequestIDWithConfig(t *testing.T) {
	customHeader := "X-Custom-Request-ID"

	config := RequestIDConfig{
		TargetHeader: customHeader,
		Generator: func() string {
			return "custom-id"
		},
	}

	// Create a test handler
	handler := func(c context.Context) error {
		rid := GetRequestID(c)
		if rid != "custom-id" {
			t.Errorf("Expected request ID 'custom-id', got %s", rid)
		}
		return c.String(200, "test")
	}

	// Create middleware
	middleware := RequestIDWithConfig(config)
	wrappedHandler := middleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := context.NewContext(req, w)

	// Execute
	err := wrappedHandler(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check response header
	if w.Header().Get(customHeader) != "custom-id" {
		t.Errorf("Expected %s header 'custom-id', got %s", customHeader, w.Header().Get(customHeader))
	}
}

func TestRequestIDSkipper(t *testing.T) {
	config := RequestIDConfig{
		Skipper: func(c context.Context) bool {
			return c.Request().URL.Path == "/skip"
		},
	}

	// Create a test handler
	handler := func(c context.Context) error {
		return c.String(200, "test")
	}

	// Create middleware
	middleware := RequestIDWithConfig(config)
	wrappedHandler := middleware(handler)

	// Test skipped request
	req := httptest.NewRequest("GET", "/skip", nil)
	w := httptest.NewRecorder()
	ctx := context.NewContext(req, w)

	err := wrappedHandler(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check that request ID header is not set
	if w.Header().Get("X-Request-ID") != "" {
		t.Error("X-Request-ID header should not be set for skipped requests")
	}

	// Test non-skipped request
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	ctx = context.NewContext(req, w)

	err = wrappedHandler(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check that request ID header is set
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header should be set for non-skipped requests")
	}
}
