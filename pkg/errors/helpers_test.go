package errors

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestGetHTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		expected int
	}{
		// Validation errors
		{"ValidationFailed", CodeValidationFailed, http.StatusBadRequest},
		{"InvalidInput", CodeInvalidInput, http.StatusBadRequest},
		
		// Auth errors
		{"Unauthorized", CodeUnauthorized, http.StatusUnauthorized},
		{"Forbidden", CodeForbidden, http.StatusForbidden},
		
		// System errors
		{"InternalServerError", CodeInternalServerError, http.StatusInternalServerError},
		{"ServiceUnavailable", CodeServiceUnavailable, http.StatusServiceUnavailable},
		{"RateLimitExceeded", CodeRateLimitExceeded, http.StatusTooManyRequests},
		{"Timeout", CodeTimeout, http.StatusRequestTimeout},
		
		// Business errors
		{"ResourceNotFound", CodeResourceNotFound, http.StatusNotFound},
		{"Conflict", CodeConflict, http.StatusConflict},
		{"PreconditionFailed", CodePreconditionFailed, http.StatusPreconditionFailed},
		
		// Unknown code defaults to 500
		{"Unknown", ErrorCode(9999), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetHTTPStatus(tt.code); got != tt.expected {
				t.Errorf("GetHTTPStatus() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	details := map[string]interface{}{
		"field": "email",
		"error": "invalid format",
	}
	
	err := ValidationError(c, "Validation failed", details)
	if err != nil {
		t.Errorf("ValidationError() returned error: %v", err)
	}
	
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, rec.Code)
	}
	
	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if resp.ErrorDetail.Code != CodeValidationFailed.Int() {
		t.Errorf("Expected error code %d, got %d", CodeValidationFailed.Int(), resp.ErrorDetail.Code)
	}
}

func TestValidationFieldError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	err := ValidationFieldError(c, "email", "Invalid email format")
	if err != nil {
		t.Errorf("ValidationFieldError() returned error: %v", err)
	}
	
	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if resp.ErrorDetail.Details["field"] != "email" {
		t.Errorf("Expected field to be 'email', got %v", resp.ErrorDetail.Details["field"])
	}
}

func TestUnauthorizedError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	// Test with custom message
	err := UnauthorizedError(c, "Custom unauthorized message")
	if err != nil {
		t.Errorf("UnauthorizedError() returned error: %v", err)
	}
	
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	
	// Test with default message
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	
	err = UnauthorizedError(c, "")
	if err != nil {
		t.Errorf("UnauthorizedError() returned error: %v", err)
	}
	
	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if resp.ErrorDetail.Message != CodeUnauthorized.Message() {
		t.Errorf("Expected default message, got %s", resp.ErrorDetail.Message)
	}
}

func TestNotFoundError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	err := NotFoundError(c, "User")
	if err != nil {
		t.Errorf("NotFoundError() returned error: %v", err)
	}
	
	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, rec.Code)
	}
	
	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if resp.ErrorDetail.Details["resource"] != "User" {
		t.Errorf("Expected resource to be 'User', got %v", resp.ErrorDetail.Details["resource"])
	}
}

func TestInternalServerError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	originalErr := errors.New("database connection failed")
	err := InternalServerError(c, originalErr)
	if err != nil {
		t.Errorf("InternalServerError() returned error: %v", err)
	}
	
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	
	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	// Should hide internal error details in message
	if resp.ErrorDetail.Message != "An internal error occurred" {
		t.Errorf("Expected generic error message, got %s", resp.ErrorDetail.Message)
	}
	
	// But include in details
	if resp.ErrorDetail.Details["error"] != originalErr.Error() {
		t.Errorf("Expected error detail to contain original error")
	}
}

func TestRateLimitError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	retryAfter := 60
	err := RateLimitError(c, retryAfter)
	if err != nil {
		t.Errorf("RateLimitError() returned error: %v", err)
	}
	
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status code %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
	
	// Check Retry-After header
	if rec.Header().Get("Retry-After") != "60" {
		t.Errorf("Expected Retry-After header to be '60', got %s", rec.Header().Get("Retry-After"))
	}
	
	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if resp.ErrorDetail.Details["retry_after"] != float64(retryAfter) {
		t.Errorf("Expected retry_after to be %d, got %v", retryAfter, resp.ErrorDetail.Details["retry_after"])
	}
}

func TestConflictError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	err := ConflictError(c, "user", "email already exists")
	if err != nil {
		t.Errorf("ConflictError() returned error: %v", err)
	}
	
	if rec.Code != http.StatusConflict {
		t.Errorf("Expected status code %d, got %d", http.StatusConflict, rec.Code)
	}
	
	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if resp.ErrorDetail.Details["resource"] != "user" {
		t.Errorf("Expected resource to be 'user', got %v", resp.ErrorDetail.Details["resource"])
	}
	
	if resp.ErrorDetail.Details["reason"] != "email already exists" {
		t.Errorf("Expected reason in details")
	}
}

func TestTimeoutError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	err := TimeoutError(c, "database query")
	if err != nil {
		t.Errorf("TimeoutError() returned error: %v", err)
	}
	
	if rec.Code != http.StatusRequestTimeout {
		t.Errorf("Expected status code %d, got %d", http.StatusRequestTimeout, rec.Code)
	}
}

func TestDatabaseError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	err := DatabaseError(c, "insert user")
	if err != nil {
		t.Errorf("DatabaseError() returned error: %v", err)
	}
	
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	
	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if resp.ErrorDetail.Details["operation"] != "insert user" {
		t.Errorf("Expected operation in details")
	}
}

func TestSendError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	details := map[string]interface{}{
		"custom": "detail",
	}
	
	err := SendError(c, CodeBusinessLogicError, "Custom error message", details)
	if err != nil {
		t.Errorf("SendError() returned error: %v", err)
	}
	
	expectedStatus := GetHTTPStatus(CodeBusinessLogicError)
	if rec.Code != expectedStatus {
		t.Errorf("Expected status code %d, got %d", expectedStatus, rec.Code)
	}
}

func TestSendErrorCode(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	err := SendErrorCode(c, CodeResourceNotFound)
	if err != nil {
		t.Errorf("SendErrorCode() returned error: %v", err)
	}
	
	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, rec.Code)
	}
	
	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if resp.ErrorDetail.Message != CodeResourceNotFound.Message() {
		t.Errorf("Expected default message, got %s", resp.ErrorDetail.Message)
	}
}

func BenchmarkValidationError(b *testing.B) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	details := map[string]interface{}{
		"field": "email",
		"error": "invalid",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		_ = ValidationError(c, "Validation failed", details)
	}
}

func BenchmarkSendError(b *testing.B) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		_ = SendErrorCode(c, CodeValidationFailed)
	}
}