package middleware

import (
	"net/http/httptest"
	"testing"

	httpctx "github.com/yshengliao/gortex/transport/http"
)

func TestRequestID(t *testing.T) {
	// Create a test handler
	handler := func(c Context) error {
		// Verify request ID is set
		rid := c.Get("request_id")
		if rid == nil || rid.(string) == "" {
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
	ctx := httpctx.NewDefaultContext(req, w)

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
	handler := func(c Context) error {
		rid := c.Get("request_id")
		if rid == nil || rid.(string) != existingID {
			t.Errorf("Expected request ID %s, got %v", existingID, rid)
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
	ctx := httpctx.NewDefaultContext(req, w)

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
		Header: customHeader,
		Generator: func() string {
			return "custom-id"
		},
	}

	// Create a test handler
	handler := func(c Context) error {
		rid := c.Get("request_id")
		if rid == nil || rid.(string) != "custom-id" {
			t.Errorf("Expected request ID 'custom-id', got %v", rid)
		}
		return c.String(200, "test")
	}

	// Create middleware
	middleware := RequestIDWithConfig(&config)
	wrappedHandler := middleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	ctx := httpctx.NewDefaultContext(req, w)

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
	// Skipper functionality test - currently RequestIDConfig doesn't support Skipper
	t.Skip("Skipper functionality not yet implemented in RequestIDConfig")
}
