package http

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSmartParams_TruncateClearsOverflow is a regression test for the router
// backtracking bug: smartParams.truncate used to reset only the count and
// leave the overflow map populated, so a backtracked branch that filled 5+
// params would leak stale params into names()/values() of whatever route
// matched next.
func TestSmartParams_TruncateClearsOverflow(t *testing.T) {
	sp := newSmartParams()
	for _, kv := range [][2]string{
		{"a", "1"}, {"b", "2"}, {"c", "3"}, {"d", "4"}, // inline array
		{"e", "5"}, {"f", "6"}, // overflow map
	} {
		sp.set(kv[0], kv[1])
	}
	require.Len(t, sp.names(), 6)
	require.Len(t, sp.values(), 6)

	// Simulate the router backtracking to two params.
	sp.truncate(2)

	assert.Len(t, sp.names(), 2, "overflow params must be dropped on truncate")
	assert.Len(t, sp.values(), 2, "overflow values must be dropped on truncate")
	assert.Empty(t, sp.get("e"), "stale overflow param must be gone after truncate")
	assert.Empty(t, sp.get("f"), "stale overflow param must be gone after truncate")
}

// hijackableWriter is a ResponseWriter that also satisfies http.Hijacker,
// simulating a successful connection takeover (e.g. a WebSocket upgrade).
type hijackableWriter struct {
	http.ResponseWriter
}

func (h *hijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, nil
}

// TestResponseWriter_HijackMarksWritten is a regression test ensuring that a
// successful Hijack marks the wrapper as written (status 101) so downstream
// logging/metrics do not record a phantom 200/written=false for a hijacked
// (e.g. WebSocket) response.
func TestResponseWriter_HijackMarksWritten(t *testing.T) {
	rw := &responseWriter{ResponseWriter: &hijackableWriter{httptest.NewRecorder()}, status: http.StatusOK}
	require.False(t, rw.Written())

	_, _, err := rw.Hijack()
	require.NoError(t, err)

	assert.True(t, rw.Written(), "hijacked response should be marked written")
	assert.Equal(t, http.StatusSwitchingProtocols, rw.Status())
}
