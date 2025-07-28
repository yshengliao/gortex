package assert

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// JSONResponse asserts that the HTTP response contains the expected JSON
func JSONResponse(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus int, expected interface{}) {
	t.Helper()
	
	// Check status code
	if rec.Code != expectedStatus {
		t.Errorf("Expected status code %d, got %d", expectedStatus, rec.Code)
		return
	}
	
	// Check content type
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" && contentType != "application/json; charset=utf-8" {
		t.Errorf("Expected JSON content type, got %s", contentType)
		return
	}
	
	// Parse response body
	var actual interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &actual); err != nil {
		t.Errorf("Failed to parse JSON response: %v", err)
		return
	}
	
	// Compare with expected
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Errorf("Failed to marshal expected value: %v", err)
		return
	}
	
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		t.Errorf("Failed to marshal actual value: %v", err)
		return
	}
	
	if string(expectedJSON) != string(actualJSON) {
		t.Errorf("JSON response mismatch\nExpected: %s\nActual: %s", expectedJSON, actualJSON)
	}
}

// StatusCode asserts that the HTTP response has the expected status code
func StatusCode(t *testing.T, rec *httptest.ResponseRecorder, expected int) {
	t.Helper()
	
	if rec.Code != expected {
		t.Errorf("Expected status code %d, got %d", expected, rec.Code)
	}
}

// Header asserts that the HTTP response contains the expected header
func Header(t *testing.T, rec *httptest.ResponseRecorder, name, expected string) {
	t.Helper()
	
	actual := rec.Header().Get(name)
	if actual != expected {
		t.Errorf("Expected header %s=%s, got %s", name, expected, actual)
	}
}

// NoError asserts that there is no error
func NoError(t *testing.T, err error) {
	t.Helper()
	
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// Error asserts that there is an error
func Error(t *testing.T, err error) {
	t.Helper()
	
	if err == nil {
		t.Error("Expected an error, got nil")
	}
}