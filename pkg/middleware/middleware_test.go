package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"github.com/yshengliao/gortex/pkg/errors"
	"github.com/yshengliao/gortex/pkg/router"
)

// mockContext implements router.Context for testing
type mockContext struct {
	request  *http.Request
	response http.ResponseWriter
	values   map[string]interface{}
	params   map[string]string
}

func newMockContext(req *http.Request, resp http.ResponseWriter) *mockContext {
	return &mockContext{
		request:  req,
		response: resp,
		values:   make(map[string]interface{}),
		params:   make(map[string]string),
	}
}

func (m *mockContext) Param(name string) string {
	return m.params[name]
}

func (m *mockContext) QueryParam(name string) string {
	return m.request.URL.Query().Get(name)
}

func (m *mockContext) Bind(i interface{}) error {
	decoder := json.NewDecoder(m.request.Body)
	return decoder.Decode(i)
}

func (m *mockContext) JSON(code int, i interface{}) error {
	m.response.Header().Set("Content-Type", "application/json")
	m.response.WriteHeader(code)
	encoder := json.NewEncoder(m.response)
	return encoder.Encode(i)
}

func (m *mockContext) String(code int, s string) error {
	m.response.Header().Set("Content-Type", "text/plain")
	m.response.WriteHeader(code)
	_, err := m.response.Write([]byte(s))
	return err
}

func (m *mockContext) Get(key string) interface{} {
	return m.values[key]
}

func (m *mockContext) Set(key string, val interface{}) {
	m.values[key] = val
}

func (m *mockContext) Request() interface{} {
	return m.request
}

func (m *mockContext) Response() interface{} {
	return m.response
}

// Tests

func TestErrorHandler(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	middleware := ErrorHandlerWithConfig(&ErrorHandlerConfig{
		Logger:                         logger,
		HideInternalServerErrorDetails: true,
		DefaultMessage:                 "Something went wrong",
	})

	tests := []struct {
		name           string
		handler        router.HandlerFunc
		expectedStatus int
		expectedCode   string
		expectedMsg    string
	}{
		{
			name: "no error",
			handler: func(c router.Context) error {
				return c.String(200, "OK")
			},
			expectedStatus: 200,
		},
		{
			name: "custom error",
			handler: func(c router.Context) error {
				return &errors.ErrorResponse{
					Success: false,
					ErrorDetail: errors.ErrorDetail{
						Code:    404,
						Message: "User not found",
					},
				}
			},
			expectedStatus: 404,
			expectedCode:   "ERR_404",
			expectedMsg:    "User not found",
		},
		{
			name: "generic error",
			handler: func(c router.Context) error {
				return fmt.Errorf("database connection failed")
			},
			expectedStatus: 500,
			expectedCode:   "INTERNAL_ERROR",
			expectedMsg:    "Something went wrong", // Hidden in production
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()
			ctx := newMockContext(req, rec)

			handler := middleware(tt.handler)
			handler(ctx)

			if tt.expectedStatus > 0 && rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			if tt.expectedCode != "" {
				var response map[string]interface{}
				json.NewDecoder(rec.Body).Decode(&response)
				
				if errMap, ok := response["error"].(map[string]interface{}); ok {
					if code := errMap["code"]; code != tt.expectedCode {
						t.Errorf("Expected error code %s, got %v", tt.expectedCode, code)
					}
					if msg := errMap["message"]; msg != tt.expectedMsg {
						t.Errorf("Expected error message %s, got %v", tt.expectedMsg, msg)
					}
				}
			}
		})
	}
}

func TestRequestID(t *testing.T) {
	middleware := RequestID()

	t.Run("generates new ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		ctx := newMockContext(req, rec)

		var capturedID string
		handler := middleware(func(c router.Context) error {
			capturedID = c.Get("request_id").(string)
			return c.String(200, "OK")
		})

		handler(ctx)

		// Check ID was generated
		if capturedID == "" {
			t.Error("Expected request ID to be generated")
		}

		// Check ID is valid UUID
		if _, err := uuid.Parse(capturedID); err != nil {
			t.Errorf("Invalid UUID: %s", capturedID)
		}

		// Check response header
		if rec.Header().Get("X-Request-ID") != capturedID {
			t.Error("Expected request ID in response header")
		}
	})

	t.Run("preserves existing ID", func(t *testing.T) {
		existingID := "test-request-id"
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", existingID)
		rec := httptest.NewRecorder()
		ctx := newMockContext(req, rec)

		var capturedID string
		handler := middleware(func(c router.Context) error {
			capturedID = c.Get("request_id").(string)
			return c.String(200, "OK")
		})

		handler(ctx)

		if capturedID != existingID {
			t.Errorf("Expected %s, got %s", existingID, capturedID)
		}
	})
}

func TestRecovery(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	middleware := RecoveryWithConfig(&RecoveryConfig{
		Logger:            logger,
		DisablePrintStack: true,
	})

	t.Run("recovers from panic", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		ctx := newMockContext(req, rec)

		handler := middleware(func(c router.Context) error {
			panic("test panic")
		})

		handler(ctx)

		if rec.Code != 500 {
			t.Errorf("Expected status 500, got %d", rec.Code)
		}

		var response map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&response)
		
		if errMap, ok := response["error"].(map[string]interface{}); ok {
			if code := errMap["code"]; code != "PANIC" {
				t.Errorf("Expected error code PANIC, got %v", code)
			}
		}
	})

	t.Run("normal request passes through", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		ctx := newMockContext(req, rec)

		handler := middleware(func(c router.Context) error {
			return c.String(200, "OK")
		})

		handler(ctx)

		if rec.Code != 200 {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		if rec.Body.String() != "OK" {
			t.Errorf("Expected body OK, got %s", rec.Body.String())
		}
	})
}

func TestCORS(t *testing.T) {
	middleware := CORSWithConfig(&CORSConfig{
		AllowOrigins:     []string{"https://example.com"},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           3600,
	})

	t.Run("simple request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()
		ctx := newMockContext(req, rec)

		handler := middleware(func(c router.Context) error {
			return c.String(200, "OK")
		})

		handler(ctx)

		if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
			t.Error("Expected CORS origin header")
		}

		if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Error("Expected CORS credentials header")
		}
	})

	t.Run("preflight request", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Content-Type")
		rec := httptest.NewRecorder()
		ctx := newMockContext(req, rec)

		handler := middleware(func(c router.Context) error {
			return c.String(200, "OK")
		})

		handler(ctx)

		if rec.Code != 204 {
			t.Errorf("Expected status 204, got %d", rec.Code)
		}

		expectedHeaders := map[string]string{
			"Access-Control-Allow-Origin":      "https://example.com",
			"Access-Control-Allow-Methods":     "GET, POST",
			"Access-Control-Allow-Headers":     "Content-Type, Authorization",
			"Access-Control-Allow-Credentials": "true",
			"Access-Control-Max-Age":           "3600",
		}

		for header, expected := range expectedHeaders {
			if got := rec.Header().Get(header); got != expected {
				t.Errorf("Header %s: expected %s, got %s", header, expected, got)
			}
		}
	})

	t.Run("disallowed origin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://evil.com")
		rec := httptest.NewRecorder()
		ctx := newMockContext(req, rec)

		handler := middleware(func(c router.Context) error {
			return c.String(200, "OK")
		})

		handler(ctx)

		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Error("Should not set CORS headers for disallowed origin")
		}
	})
}

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(&buf),
		zap.InfoLevel,
	))

	middleware := LoggerWithConfig(&LoggerConfig{
		Logger:          logger,
		SkipPaths:       []string{"/health"},
		LogRequestBody:  true,
		LogResponseBody: true,
		BodyLogLimit:    100,
	})

	t.Run("logs request", func(t *testing.T) {
		buf.Reset()
		
		body := `{"test": "data"}`
		req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		ctx := newMockContext(req, rec)
		ctx.Set("request_id", "test-123")

		handler := middleware(func(c router.Context) error {
			return c.JSON(200, map[string]string{"status": "ok"})
		})

		handler(ctx)

		// Check log was written
		if buf.Len() == 0 {
			t.Error("Expected log output")
		}

		// Parse log
		var log map[string]interface{}
		if err := json.NewDecoder(&buf).Decode(&log); err != nil {
			t.Fatalf("Failed to parse log: %v", err)
		}

		// Check fields
		if log["method"] != "POST" {
			t.Errorf("Expected method POST, got %v", log["method"])
		}
		if log["path"] != "/test" {
			t.Errorf("Expected path /test, got %v", log["path"])
		}
		if log["status"] != float64(200) {
			t.Errorf("Expected status 200, got %v", log["status"])
		}
		if log["request_id"] != "test-123" {
			t.Errorf("Expected request_id test-123, got %v", log["request_id"])
		}
	})

	t.Run("skips configured paths", func(t *testing.T) {
		buf.Reset()
		
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()
		ctx := newMockContext(req, rec)

		handler := middleware(func(c router.Context) error {
			return c.String(200, "OK")
		})

		handler(ctx)

		// Should not log
		if buf.Len() > 0 {
			t.Error("Should not log skipped paths")
		}
	})
}