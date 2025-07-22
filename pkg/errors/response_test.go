package errors

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestNewErrorResponse(t *testing.T) {
	code := CodeValidationFailed
	message := "Custom validation error"
	
	err := New(code, message)
	
	if err.Success != false {
		t.Errorf("Expected Success to be false")
	}
	
	if err.ErrorDetail.Code != code.Int() {
		t.Errorf("Expected Code to be %d, got %d", code.Int(), err.ErrorDetail.Code)
	}
	
	if err.ErrorDetail.Message != message {
		t.Errorf("Expected Message to be %s, got %s", message, err.ErrorDetail.Message)
	}
	
	if err.Timestamp.IsZero() {
		t.Errorf("Expected Timestamp to be set")
	}
}

func TestNewWithDetails(t *testing.T) {
	code := CodeInvalidInput
	message := "Invalid user input"
	details := map[string]interface{}{
		"field": "email",
		"error": "invalid format",
	}
	
	err := NewWithDetails(code, message, details)
	
	if err.ErrorDetail.Details == nil {
		t.Errorf("Expected Details to be set")
	}
	
	if err.ErrorDetail.Details["field"] != "email" {
		t.Errorf("Expected field detail to be 'email'")
	}
}

func TestNewFromCode(t *testing.T) {
	code := CodeUnauthorized
	err := NewFromCode(code)
	
	if err.ErrorDetail.Code != code.Int() {
		t.Errorf("Expected Code to be %d, got %d", code.Int(), err.ErrorDetail.Code)
	}
	
	if err.ErrorDetail.Message != code.Message() {
		t.Errorf("Expected Message to be %s, got %s", code.Message(), err.ErrorDetail.Message)
	}
}

func TestErrorResponseChaining(t *testing.T) {
	err := New(CodeBusinessLogicError, "Business error").
		WithRequestID("req-123").
		WithMeta(map[string]interface{}{"version": "1.0"}).
		WithDetail("resource", "user").
		WithDetail("id", 123)
	
	if err.RequestID != "req-123" {
		t.Errorf("Expected RequestID to be 'req-123', got %s", err.RequestID)
	}
	
	if err.Meta["version"] != "1.0" {
		t.Errorf("Expected Meta version to be '1.0'")
	}
	
	if err.ErrorDetail.Details["resource"] != "user" {
		t.Errorf("Expected resource detail to be 'user'")
	}
	
	if err.ErrorDetail.Details["id"] != 123 {
		t.Errorf("Expected id detail to be 123")
	}
}

func TestErrorResponseSend(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	err := New(CodeValidationFailed, "Test error").
		WithRequestID("test-123")
	
	if sendErr := err.Send(c, http.StatusBadRequest); sendErr != nil {
		t.Errorf("Send() returned error: %v", sendErr)
	}
	
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestGetRequestID(t *testing.T) {
	e := echo.New()
	
	tests := []struct {
		name      string
		setupFunc func(*echo.Context)
		expected  string
	}{
		{
			name: "From Response Header",
			setupFunc: func(c *echo.Context) {
				(*c).Response().Header().Set(echo.HeaderXRequestID, "resp-123")
			},
			expected: "resp-123",
		},
		{
			name: "From Request Header",
			setupFunc: func(c *echo.Context) {
				(*c).Request().Header.Set(echo.HeaderXRequestID, "req-123")
			},
			expected: "req-123",
		},
		{
			name:      "No Request ID",
			setupFunc: func(c *echo.Context) {},
			expected:  "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			
			tt.setupFunc(&c)
			
			if got := GetRequestID(c); got != tt.expected {
				t.Errorf("GetRequestID() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrorInterface(t *testing.T) {
	err := New(CodeInternalServerError, "Server error")
	
	// Test that ErrorResponse implements error interface
	var _ error = err
	
	if err.Error() != "Server error" {
		t.Errorf("Error() = %v, want %v", err.Error(), "Server error")
	}
}

func TestWithDetails(t *testing.T) {
	details := map[string]interface{}{
		"field1": "value1",
		"field2": 123,
	}
	
	err := New(CodeValidationFailed, "Validation error").
		WithDetails(details)
	
	if len(err.ErrorDetail.Details) != 2 {
		t.Errorf("Expected 2 details, got %d", len(err.ErrorDetail.Details))
	}
	
	if err.ErrorDetail.Details["field1"] != "value1" {
		t.Errorf("Expected field1 to be 'value1'")
	}
}

func BenchmarkNewErrorResponse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = New(CodeValidationFailed, "Validation error")
	}
}

func BenchmarkErrorResponseWithDetails(b *testing.B) {
	details := map[string]interface{}{
		"field": "email",
		"error": "invalid",
	}
	
	for i := 0; i < b.N; i++ {
		_ = NewWithDetails(CodeValidationFailed, "Validation error", details).
			WithRequestID("req-123").
			WithMeta(map[string]interface{}{"version": "1.0"})
	}
}

func TestTimestampFormat(t *testing.T) {
	err := New(CodeValidationFailed, "Test error")
	
	// Ensure timestamp is in UTC
	if err.Timestamp.Location() != time.UTC {
		t.Errorf("Expected timestamp to be in UTC")
	}
	
	// Ensure timestamp is recent (within last second)
	if time.Since(err.Timestamp) > time.Second {
		t.Errorf("Timestamp is too old")
	}
}