package response

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/yshengliao/gortex/test"
)

func TestSuccess(t *testing.T) {
	ctx := test.NewMockContext("GET", "/").Build()
	mockCtx := ctx.(*test.MockContext)

	data := map[string]string{"message": "success"}
	err := Success(ctx, http.StatusOK, data)

	if err != nil {
		t.Errorf("Success() returned error: %v", err)
	}

	rec := mockCtx.ResponseRecorder()
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
	mockCtx := test.NewMockContext("GET", "/").
		WithHeader("X-Request-ID", "test-123")
	ctx := mockCtx.Build()

	data := map[string]string{"message": "success"}
	meta := map[string]interface{}{"version": "1.0"}

	err := SuccessWithMeta(ctx, http.StatusOK, data, meta)

	if err != nil {
		t.Errorf("SuccessWithMeta() returned error: %v", err)
	}

	rec := mockCtx.ResponseRecorder()
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}

	var resp SuccessResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected Success to be true")
	}

	if resp.Meta["version"] != "1.0" {
		t.Errorf("Expected meta version to be 1.0")
	}
}

func TestError(t *testing.T) {
	ctx := test.NewMockContext("GET", "/").Build()
	mockCtx := ctx.(*test.MockContext)

	err := Error(ctx, http.StatusBadRequest, "Bad request")

	if err != nil {
		t.Errorf("Error() returned error: %v", err)
	}

	rec := mockCtx.ResponseRecorder()
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp StandardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Errorf("Expected Success to be false")
	}

	if resp.Error != "Bad request" {
		t.Errorf("Expected error message 'Bad request', got '%s'", resp.Error)
	}
}

func TestBadRequest(t *testing.T) {
	ctx := test.NewMockContext("GET", "/").Build()
	mockCtx := ctx.(*test.MockContext)

	err := BadRequest(ctx, "Invalid input")

	if err != nil {
		t.Errorf("BadRequest() returned error: %v", err)
	}

	rec := mockCtx.ResponseRecorder()
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUnauthorized(t *testing.T) {
	ctx := test.NewMockContext("GET", "/").Build()
	mockCtx := ctx.(*test.MockContext)

	err := Unauthorized(ctx, "Authentication required")

	if err != nil {
		t.Errorf("Unauthorized() returned error: %v", err)
	}

	rec := mockCtx.ResponseRecorder()
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestForbidden(t *testing.T) {
	ctx := test.NewMockContext("GET", "/").Build()
	mockCtx := ctx.(*test.MockContext)

	err := Forbidden(ctx, "Access denied")

	if err != nil {
		t.Errorf("Forbidden() returned error: %v", err)
	}

	rec := mockCtx.ResponseRecorder()
	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status code %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestNotFound(t *testing.T) {
	ctx := test.NewMockContext("GET", "/").Build()
	mockCtx := ctx.(*test.MockContext)

	err := NotFound(ctx, "Resource not found")

	if err != nil {
		t.Errorf("NotFound() returned error: %v", err)
	}

	rec := mockCtx.ResponseRecorder()
	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestInternalServerError(t *testing.T) {
	ctx := test.NewMockContext("GET", "/").Build()
	mockCtx := ctx.(*test.MockContext)

	err := InternalServerError(ctx, "Server error")

	if err != nil {
		t.Errorf("InternalServerError() returned error: %v", err)
	}

	rec := mockCtx.ResponseRecorder()
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestCreated(t *testing.T) {
	ctx := test.NewMockContext("POST", "/").Build()
	mockCtx := ctx.(*test.MockContext)

	data := map[string]interface{}{"id": 123, "name": "Test"}
	err := Created(ctx, data)

	if err != nil {
		t.Errorf("Created() returned error: %v", err)
	}

	rec := mockCtx.ResponseRecorder()
	if rec.Code != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestErrorFromRegistered(t *testing.T) {
	// This test is no longer relevant since Error() function is deprecated
	// and errors should be handled through the errors package
	t.Skip("Error() function is deprecated - use pkg/errors instead")
}
