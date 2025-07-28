package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpctx "github.com/yshengliao/gortex/transport/http"
)

func TestDevErrorPage(t *testing.T) {
	tests := []struct {
		name           string
		config         GortexDevErrorPageConfig
		handler        HandlerFunc
		acceptHeader   string
		expectedStatus int
		expectHTML     bool
		expectJSON     bool
	}{
		{
			name:   "HTML error page with stack trace",
			config: DefaultGortexDevErrorPageConfig,
			handler: func(c Context) error {
				return errors.New("test error")
			},
			acceptHeader:   "text/html,application/xhtml+xml",
			expectedStatus: http.StatusInternalServerError,
			expectHTML:     true,
		},
		{
			name:   "JSON error response",
			config: DefaultGortexDevErrorPageConfig,
			handler: func(c Context) error {
				return errors.New("test error")
			},
			acceptHeader:   "application/json",
			expectedStatus: http.StatusInternalServerError,
			expectJSON:     true,
		},
		{
			name: "No stack trace config",
			config: GortexDevErrorPageConfig{
				ShowStackTrace:     false,
				ShowRequestDetails: true,
				StackTraceLimit:    10,
			},
			handler: func(c Context) error {
				return errors.New("test error")
			},
			acceptHeader:   "text/html",
			expectedStatus: http.StatusInternalServerError,
			expectHTML:     true,
		},
		{
			name:   "No error case",
			config: DefaultGortexDevErrorPageConfig,
			handler: func(c Context) error {
				return c.String(http.StatusOK, "success")
			},
			acceptHeader:   "text/html",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Accept", tt.acceptHeader)
			req.Header.Set("User-Agent", "Test-Agent")

			rec := httptest.NewRecorder()

			// Create Gortex context
			ctx := httpctx.NewDefaultContext(req, rec)

			// Create middleware
			middleware := GortexDevErrorPageWithConfig(tt.config)
			handler := middleware(tt.handler)

			// Execute
			err := handler(ctx)

			// Verify
			if tt.expectedStatus != 0 {
				if rec.Code != tt.expectedStatus {
					t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
				}
			}

			body := rec.Body.String()

			if tt.expectHTML {
				if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
					t.Error("Expected HTML content type")
				}
				if !strings.Contains(body, "Gortex Error") {
					t.Error("Expected HTML error page")
				}
				if tt.config.ShowStackTrace && !strings.Contains(body, "Stack Trace") {
					t.Error("Expected stack trace in HTML")
				}
				if !tt.config.ShowStackTrace && strings.Contains(body, "Stack Trace") {
					t.Error("Should not show stack trace")
				}
			}

			if tt.expectJSON {
				if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
					t.Error("Expected JSON content type")
				}
				if !strings.Contains(body, "error") {
					t.Error("Expected JSON error response")
				}
			}

			if err != nil && tt.expectedStatus == http.StatusOK {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestRecoverWithErrorPage(t *testing.T) {
	tests := []struct {
		name           string
		config         GortexDevErrorPageConfig
		handler        HandlerFunc
		acceptHeader   string
		expectedStatus int
		expectHTML     bool
		expectJSON     bool
	}{
		{
			name:   "Panic with HTML response",
			config: DefaultGortexDevErrorPageConfig,
			handler: func(c Context) error {
				panic("test panic")
			},
			acceptHeader:   "text/html",
			expectedStatus: http.StatusInternalServerError,
			expectHTML:     true,
		},
		{
			name:   "Panic with JSON response",
			config: DefaultGortexDevErrorPageConfig,
			handler: func(c Context) error {
				panic("test panic")
			},
			acceptHeader:   "application/json",
			expectedStatus: http.StatusInternalServerError,
			expectJSON:     true,
		},
		{
			name:   "Panic with error type",
			config: DefaultGortexDevErrorPageConfig,
			handler: func(c Context) error {
				panic(errors.New("panic error"))
			},
			acceptHeader:   "text/html",
			expectedStatus: http.StatusInternalServerError,
			expectHTML:     true,
		},
		{
			name:   "No panic case",
			config: DefaultGortexDevErrorPageConfig,
			handler: func(c Context) error {
				return c.String(http.StatusOK, "success")
			},
			acceptHeader:   "text/html",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Accept", tt.acceptHeader)
			req.Header.Set("User-Agent", "Test-Agent")

			rec := httptest.NewRecorder()

			// Create Gortex context
			ctx := httpctx.NewDefaultContext(req, rec)

			// Create recovery middleware
			middleware := RecoverWithErrorPageConfig(tt.config)
			handler := middleware(tt.handler)

			// Execute
			err := handler(ctx)

			// Verify
			if tt.expectedStatus != 0 {
				if rec.Code != tt.expectedStatus {
					t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
				}
			}

			body := rec.Body.String()

			if tt.expectHTML {
				if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
					t.Error("Expected HTML content type")
				}
				if !strings.Contains(body, "Gortex Error") {
					t.Error("Expected HTML error page")
				}
			}

			if tt.expectJSON {
				if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
					t.Error("Expected JSON content type")
				}
				if !strings.Contains(body, "error") {
					t.Error("Expected JSON error response")
				}
			}

			if err != nil && tt.expectedStatus == http.StatusOK {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestExtractErrorInfo(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test?param=value", nil)
	req.Header.Set("User-Agent", "Test-Agent")
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()

	ctx := httpctx.NewDefaultContext(req, rec)
	err := errors.New("test error")
	config := DefaultGortexDevErrorPageConfig

	errorInfo := extractErrorInfo(err, ctx, config)

	if errorInfo.Message != "test error" {
		t.Errorf("Expected message 'test error', got '%s'", errorInfo.Message)
	}

	if errorInfo.Status != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, errorInfo.Status)
	}

	if !config.ShowStackTrace && errorInfo.StackTrace != "" {
		t.Error("Should not include stack trace when disabled")
	}

	if config.ShowRequestDetails {
		if errorInfo.RequestDetails["method"] != "POST" {
			t.Errorf("Expected method POST, got %s", errorInfo.RequestDetails["method"])
		}

		if !strings.Contains(errorInfo.RequestDetails["url"], "/test?param=value") {
			t.Error("Expected URL to contain path and query")
		}

		if len(errorInfo.Headers) == 0 {
			t.Error("Expected headers to be populated")
		}
	}
}

func BenchmarkDevErrorPage(b *testing.B) {
	config := DefaultGortexDevErrorPageConfig
	middleware := GortexDevErrorPageWithConfig(config)

	handler := middleware(func(c Context) error {
		return errors.New("benchmark error")
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "text/html")
		rec := httptest.NewRecorder()
		ctx := httpctx.NewDefaultContext(req, rec)

		handler(ctx)
	}
}
