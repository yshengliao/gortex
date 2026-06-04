package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestLoggerResponseWriter_HonoursBodyLimit is a regression test for the
// hardcoded 1 KB response-body capture. The wrapper used to stop capturing at a
// literal 1024 bytes regardless of the configured BodyLogLimit, silently
// truncating logged response bodies whenever BodyLogLimit was raised.
func TestLoggerResponseWriter_HonoursBodyLimit(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK, bodyLimit: 4096}

	payload := bytes.Repeat([]byte("x"), 2048)
	n, err := rw.Write(payload)
	require.NoError(t, err)
	require.Equal(t, 2048, n)

	assert.Len(t, rw.body, 2048, "capture must honour bodyLimit, not a hardcoded 1024")
	assert.Equal(t, 2048, rec.Body.Len(), "full body must still reach the client")
}

// TestLoggerResponseWriter_CapsExactlyAtBodyLimit verifies the capture stops
// exactly at bodyLimit (no overshoot) while the full body is still written
// downstream.
func TestLoggerResponseWriter_CapsExactlyAtBodyLimit(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK, bodyLimit: 1000}

	_, err := rw.Write(bytes.Repeat([]byte("y"), 2048))
	require.NoError(t, err)

	assert.Len(t, rw.body, 1000, "capture must stop exactly at bodyLimit")
	assert.Equal(t, 2048, rec.Body.Len(), "full body must still reach the client")
}

// TestLoggerResponseWriter_ZeroLimitDisablesCapture verifies that a zero limit
// (response-body logging disabled) captures nothing, avoiding wasted memory.
func TestLoggerResponseWriter_ZeroLimitDisablesCapture(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK} // bodyLimit 0

	_, err := rw.Write([]byte("hello"))
	require.NoError(t, err)

	assert.Empty(t, rw.body, "bodyLimit 0 must disable body capture")
	assert.Equal(t, "hello", rec.Body.String())
}

// TestLogger_NonStringRequestID_NoPanic is a regression test for an unchecked
// type assertion: requestID.(string) used to run without comma-ok, so a
// non-string value stored under "request_id" by other code would panic the
// logger middleware (and the request).
func TestLogger_NonStringRequestID_NoPanic(t *testing.T) {
	mw := LoggerWithConfig(&LoggerConfig{Logger: zaptest.NewLogger(t)})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := newTestContext(req, httptest.NewRecorder())
	c.Set("request_id", 12345) // non-string under the well-known key

	require.NotPanics(t, func() {
		_ = mw(func(Context) error { return nil })(c)
	})
}
