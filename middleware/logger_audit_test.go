package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

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

// TestLogger_LogsRealStatusAndLevel is the regression test for the dead
// response-capture path: the logger used to install its own responseWriter via
// an optional SetResponse hook that no context type implemented, so the
// assertion always failed and every request logged a hardcoded status 200 at
// Info level. The logger now reads the real status from the context's tracked
// response writer and selects the log level from it. A handler that writes a
// 404/500 directly (no returned error) must be logged with that status and the
// matching level — both of which fail against the old code.
func TestLogger_LogsRealStatusAndLevel(t *testing.T) {
	tests := []struct {
		name      string
		write     func(c Context) error
		wantCode  int
		wantLevel zapcore.Level
		wantMsg   string
	}{
		{
			name:      "200 logged at info",
			write:     func(c Context) error { return c.JSON(http.StatusOK, map[string]string{"ok": "yes"}) },
			wantCode:  http.StatusOK,
			wantLevel: zapcore.InfoLevel,
			wantMsg:   "Request completed",
		},
		{
			name:      "404 written directly logged at warn",
			write:     func(c Context) error { return c.JSON(http.StatusNotFound, map[string]string{"err": "missing"}) },
			wantCode:  http.StatusNotFound,
			wantLevel: zapcore.WarnLevel,
			wantMsg:   "Request error",
		},
		{
			name:      "500 written directly logged at error",
			write:     func(c Context) error { return c.JSON(http.StatusInternalServerError, map[string]string{"err": "boom"}) },
			wantCode:  http.StatusInternalServerError,
			wantLevel: zapcore.ErrorLevel,
			wantMsg:   "Request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.DebugLevel)
			mw := LoggerWithConfig(&LoggerConfig{Logger: zap.New(core)})

			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			// Use the framework's real DefaultContext-style tracked writer so
			// the status genuinely reaches the wire (testResponseWriter mirrors
			// transport/http.responseWriter's Status()/Written() tracking).
			c := newTestContext(req, httptest.NewRecorder())

			require.NoError(t, mw(tt.write)(c))

			entries := logs.All()
			require.Len(t, entries, 1, "expected exactly one log line")
			entry := entries[0]
			assert.Equal(t, tt.wantLevel, entry.Level, "log level must follow the real status")
			assert.Equal(t, tt.wantMsg, entry.Message)

			status, ok := entry.ContextMap()["status"]
			require.True(t, ok, "status field must be present")
			assert.Equal(t, int64(tt.wantCode), status, "logged status must be the real status, not a hardcoded 200")
		})
	}
}

// TestLogger_ErrorWithoutWrite_LoggedAtError verifies that when a handler
// returns an error but nothing reaches the wire (no status written — e.g. the
// error handler middleware is not in the chain), the logger still surfaces the
// failure loudly at Error level rather than silently logging a 200.
func TestLogger_ErrorWithoutWrite_LoggedAtError(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	mw := LoggerWithConfig(&LoggerConfig{Logger: zap.New(core)})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	c := newTestContext(req, httptest.NewRecorder())

	err := mw(func(Context) error { return assert.AnError })(c)
	require.ErrorIs(t, err, assert.AnError, "logger must propagate the handler error")

	entries := logs.All()
	require.Len(t, entries, 1)
	assert.Equal(t, zapcore.ErrorLevel, entries[0].Level)
	assert.Equal(t, "Request failed", entries[0].Message)
}
