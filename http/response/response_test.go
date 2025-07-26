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

