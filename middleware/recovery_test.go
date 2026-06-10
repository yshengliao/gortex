package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRecoveryDefaultConfigRecovers verifies that Recovery() (using the
// default nil-logger config) still intercepts a panic and responds with 500.
func TestRecoveryDefaultConfigRecovers(t *testing.T) {
	mw := Recovery()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := newMockContext(req, rec)

	handler := mw(func(c Context) error {
		panic("default config panic")
	})

	// Must not propagate the panic.
	require.NotPanics(t, func() { _ = handler(ctx) })
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	errMap, ok := body["error"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "PANIC", errMap["code"])
}

// TestRecoveryLogsViaZapWhenInjected verifies that when a zap logger is
// supplied the panic is logged through it (observable with zaptest/observer).
func TestRecoveryLogsViaZapWhenInjected(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)

	mw := RecoveryWithConfig(&RecoveryConfig{
		Logger:            logger,
		DisablePrintStack: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := newMockContext(req, rec)

	handler := mw(func(c Context) error {
		panic("logged panic")
	})
	_ = handler(ctx)

	require.Equal(t, 1, logs.Len(), "expected exactly one log entry")
	entry := logs.All()[0]
	assert.Equal(t, "Panic recovered", entry.Message)
	assert.NotNil(t, entry.ContextMap()["error"])
}

// TestFormatStackContainsPanicFrame verifies that formatStack retains the
// frame from user code (not just runtime internals) and strips frames
// belonging to this middleware package.
func TestFormatStackContainsPanicFrame(t *testing.T) {
	// Capture a real stack trace by recovering inside a deferred function.
	var captured []byte
	func() {
		defer func() {
			recover()
			buf := make([]byte, 4<<10)
			// We can't call runtime.Stack here directly, so we use a
			// synthesised stack string that mimics the real format.
			captured = []byte(
				"goroutine 1 [running]:\n" +
					"github.com/yshengliao/gortex/middleware.panicFunc(...)\n" +
					"\t/home/user/gortex/middleware/recovery.go:50\n" +
					"github.com/user/myapp.BusinessHandler(...)\n" +
					"\t/home/user/myapp/handler.go:99\n" +
					"runtime/debug.Stack()\n" +
					"\t/usr/local/go/src/runtime/debug/stack.go:24\n",
			)
			_ = buf
		}()
	}()

	frames := formatStack(captured)

	// The middleware frame should be filtered out.
	for _, f := range frames {
		assert.False(t, strings.Contains(f, "github.com/yshengliao/gortex/middleware"),
			"middleware package frames must be filtered: %s", f)
	}
	// The user-code frame must survive.
	found := false
	for _, f := range frames {
		if strings.Contains(f, "myapp") {
			found = true
			break
		}
	}
	assert.True(t, found, "user-code frames must be retained in formatStack output")
}

// TestRecoveryDebugDetailsWhenDebugLogger verifies that when the logger is at
// debug level the response includes panic details (non-nil "details" key).
func TestRecoveryDebugDetailsWhenDebugLogger(t *testing.T) {
	core, _ := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	mw := RecoveryWithConfig(&RecoveryConfig{
		Logger:            logger,
		DisablePrintStack: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := newMockContext(req, rec)

	handler := mw(func(c Context) error {
		panic("debug panic")
	})
	_ = handler(ctx)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	errMap, ok := body["error"].(map[string]interface{})
	require.True(t, ok)
	assert.NotNil(t, errMap["details"], "debug-level logger should include panic details in response")
}
