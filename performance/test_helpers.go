// Package performance provides test helpers for benchmarking
package performance

import (
	"bytes"
	"net/http"
	"net/http/httptest"
)

// TestRequest creates a new test HTTP request
func NewTestRequest(method, path string, body []byte) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	return req
}

// TestResponseRecorder creates a new test response recorder
func NewTestResponseRecorder() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}