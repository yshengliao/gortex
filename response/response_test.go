package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/pkg/errors"
)

func TestSuccess(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	data := map[string]string{"message": "success"}
	err := Success(c, http.StatusOK, data)

	if err != nil {
		t.Errorf("Success() returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}

	var resp StandardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected Success to be true")
	}
}

func TestSuccessWithMeta(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(echo.HeaderXRequestID, "test-123")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	data := map[string]string{"message": "success"}
	meta := map[string]interface{}{"version": "1.0"}
	
	err := SuccessWithMeta(c, http.StatusOK, data, meta)

	if err != nil {
		t.Errorf("SuccessWithMeta() returned error: %v", err)
	}

	var resp SuccessResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected Success to be true")
	}

	if resp.Meta["version"] != "1.0" {
		t.Errorf("Expected meta version to be '1.0'")
	}

	if resp.RequestID != "test-123" {
		t.Errorf("Expected request ID to be 'test-123', got %s", resp.RequestID)
	}
}

func TestBackwardCompatibility(t *testing.T) {
	e := echo.New()
	
	tests := []struct {
		name     string
		fn       func(echo.Context, string) error
		message  string
		expected int
	}{
		{
			name:     "BadRequest",
			fn:       BadRequest,
			message:  "Bad request",
			expected: http.StatusBadRequest,
		},
		{
			name:     "Unauthorized",
			fn:       Unauthorized,
			message:  "Unauthorized",
			expected: http.StatusUnauthorized,
		},
		{
			name:     "Forbidden",
			fn:       Forbidden,
			message:  "Forbidden",
			expected: http.StatusForbidden,
		},
		{
			name:     "NotFound",
			fn:       NotFound,
			message:  "Not found",
			expected: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := tt.fn(c, tt.message)
			if err != nil {
				t.Errorf("%s() returned error: %v", tt.name, err)
			}

			if rec.Code != tt.expected {
				t.Errorf("Expected status code %d, got %d", tt.expected, rec.Code)
			}

			// Verify it returns new error format
			var resp errors.ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if resp.Success != false {
				t.Errorf("Expected Success to be false")
			}
		})
	}
}

func TestCreated(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	data := map[string]string{"id": "123"}
	err := Created(c, data)

	if err != nil {
		t.Errorf("Created() returned error: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestNoContent(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := NoContent(c)

	if err != nil {
		t.Errorf("NoContent() returned error: %v", err)
	}

	if rec.Code != http.StatusNoContent {
		t.Errorf("Expected status code %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestAccepted(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	data := map[string]string{"status": "processing"}
	err := Accepted(c, data)

	if err != nil {
		t.Errorf("Accepted() returned error: %v", err)
	}

	if rec.Code != http.StatusAccepted {
		t.Errorf("Expected status code %d, got %d", http.StatusAccepted, rec.Code)
	}
}

func TestDeprecatedError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Test the deprecated Error function still works
	err := Error(c, http.StatusBadRequest, "Test error")

	if err != nil {
		t.Errorf("Error() returned error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp StandardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Success != false {
		t.Errorf("Expected Success to be false")
	}

	if resp.Error != "Test error" {
		t.Errorf("Expected error message 'Test error', got %s", resp.Error)
	}
}