package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/yshengliao/gortex/app"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func setupTestApp(t *testing.T) *app.App {
	logger := zaptest.NewLogger(t)

	cfg := &app.Config{}
	cfg.Server.Recovery = true
	cfg.Logger.Level = "debug"

	handlers := &HandlersManager{
		API: &APIHandler{
			Logger: logger,
		},
		External: &ExternalHandler{
			Logger: logger,
		},
	}

	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	assert.NoError(t, err)

	return application
}

func TestAPIHandler_GET(t *testing.T) {
	app := setupTestApp(t)
	e := app.Echo()

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Check that request ID is in response header
	requestID := rec.Header().Get(echo.HeaderXRequestID)
	assert.NotEmpty(t, requestID)

	// Check response body
	var response map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))
	assert.Equal(t, requestID, response["request_id"])
}

func TestAPIHandler_GET_WithCustomRequestID(t *testing.T) {
	app := setupTestApp(t)
	e := app.Echo()

	customID := "my-custom-request-id-123"
	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req.Header.Set(echo.HeaderXRequestID, customID)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Check that custom request ID is preserved
	assert.Equal(t, customID, rec.Header().Get(echo.HeaderXRequestID))

	// Check response body
	var response map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, customID, response["request_id"])
}

func TestAPIHandler_Process(t *testing.T) {
	app := setupTestApp(t)
	e := app.Echo()

	payload := map[string]interface{}{
		"test":   "data",
		"number": 42,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/process", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Check request ID in header
	requestID := rec.Header().Get(echo.HeaderXRequestID)
	assert.NotEmpty(t, requestID)

	// Check response
	var response map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))
	assert.Equal(t, requestID, response["request_id"])

	// Check processed data
	data := response["data"].(map[string]interface{})
	assert.True(t, data["processed"].(bool))
	processedData := data["data"].(map[string]interface{})
	assert.Equal(t, "data", processedData["test"])
	assert.Equal(t, float64(42), processedData["number"])
}

func TestExternalHandler_Trace(t *testing.T) {
	app := setupTestApp(t)
	e := app.Echo()

	req := httptest.NewRequest(http.MethodPost, "/external/trace", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Check response
	var response map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify all request ID sources match
	assert.True(t, response["all_match"].(bool))

	sources := response["request_id_sources"].(map[string]interface{})
	requestID := sources["from_echo_context"].(string)
	assert.NotEmpty(t, requestID)
	assert.Equal(t, requestID, sources["from_context_value"])
	assert.Equal(t, requestID, sources["from_response_header"])
	assert.Equal(t, requestID, sources["from_standard_context"])
}

func TestRequestIDPropagation(t *testing.T) {
	// This test verifies that request IDs are properly propagated through the system
	app := setupTestApp(t)
	e := app.Echo()

	testCases := []struct {
		name          string
		customID      string
		expectCustom  bool
	}{
		{
			name:         "auto-generated ID",
			customID:     "",
			expectCustom: false,
		},
		{
			name:         "custom ID",
			customID:     "test-request-id-12345",
			expectCustom: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api", nil)
			if tc.customID != "" {
				req.Header.Set(echo.HeaderXRequestID, tc.customID)
			}
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			responseID := rec.Header().Get(echo.HeaderXRequestID)
			assert.NotEmpty(t, responseID)

			if tc.expectCustom {
				assert.Equal(t, tc.customID, responseID)
			} else {
				// Should be a valid UUID
				assert.Regexp(t, `^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`, responseID)
			}
		})
	}
}

// Benchmark request ID operations
func BenchmarkAPIHandler_GET(b *testing.B) {
	logger := zap.NewNop()
	cfg := &app.Config{}
	handlers := &HandlersManager{
		API: &APIHandler{Logger: logger},
		External: &ExternalHandler{Logger: logger},
	}

	application, _ := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	e := application.Echo()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/api", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
		}
	})
}

func BenchmarkAPIHandler_Process(b *testing.B) {
	logger := zap.NewNop()
	cfg := &app.Config{}
	handlers := &HandlersManager{
		API: &APIHandler{Logger: logger},
		External: &ExternalHandler{Logger: logger},
	}

	application, _ := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	e := application.Echo()

	payload := []byte(`{"test":"data","number":42}`)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodPost, "/api/process", bytes.NewReader(payload))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
		}
	})
}