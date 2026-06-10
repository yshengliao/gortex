package middleware

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fireRL sends one request through the middleware with the given RemoteAddr
// and optional X-Forwarded-For, returning the resulting status code.
func fireRL(t *testing.T, h HandlerFunc, remoteAddr, xff string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	rec := httptest.NewRecorder()
	ctx := newTestContext(req, rec)
	if err := h(ctx); err != nil {
		return http.StatusInternalServerError
	}
	return rec.Code
}

// With no TrustedProxies configured, the default KeyFunc must ignore
// X-Forwarded-For: two requests from the same peer but with different forged
// XFF headers share a bucket, so the second (over a 1-burst limit) is 429.
func TestDefaultKeyFuncIgnoresForgedXFF(t *testing.T) {
	cfg := &GortexRateLimitConfig{Rate: 1, Burst: 1}
	mw := GortexRateLimitWithConfig(cfg)
	require.NotNil(t, cfg.Store, "store must be written back")
	defer cfg.Store.(*MemoryRateLimiter).Stop()

	h := mw(func(c Context) error { return c.NoContent(http.StatusOK) })

	// Same peer, different forged client IPs in XFF.
	first := fireRL(t, h, "198.51.100.7:5000", "1.1.1.1")
	second := fireRL(t, h, "198.51.100.7:5001", "2.2.2.2")

	assert.Equal(t, http.StatusOK, first)
	assert.Equal(t, http.StatusTooManyRequests, second,
		"forged XFF must not let a client escape into a fresh bucket")
}

// When TrustedProxies includes the peer, the default KeyFunc honours
// X-Forwarded-For, so two distinct upstream clients get independent buckets.
func TestDefaultKeyFuncHonoursXFFFromTrustedPeer(t *testing.T) {
	_, cidr, err := net.ParseCIDR("10.0.0.0/8")
	require.NoError(t, err)

	cfg := &GortexRateLimitConfig{
		Rate:           1,
		Burst:          1,
		TrustedProxies: []*net.IPNet{cidr},
	}
	mw := GortexRateLimitWithConfig(cfg)
	defer cfg.Store.(*MemoryRateLimiter).Stop()

	h := mw(func(c Context) error { return c.NoContent(http.StatusOK) })

	// Trusted proxy forwarding two different real clients → two buckets.
	clientA := fireRL(t, h, "10.0.0.1:6000", "203.0.113.1")
	clientB := fireRL(t, h, "10.0.0.1:6001", "203.0.113.2")
	assert.Equal(t, http.StatusOK, clientA)
	assert.Equal(t, http.StatusOK, clientB,
		"distinct forwarded clients via a trusted proxy must not share a bucket")

	// A second request for client A (same forwarded IP) is limited.
	clientAAgain := fireRL(t, h, "10.0.0.1:6002", "203.0.113.1")
	assert.Equal(t, http.StatusTooManyRequests, clientAAgain)
}

// The store created lazily by GortexRateLimitWithConfig must be written back to
// config.Store so the caller holds a handle to Stop() its cleanup goroutine.
func TestLazyStoreWrittenBackIsStoppable(t *testing.T) {
	cfg := &GortexRateLimitConfig{Rate: 5, Burst: 5}
	_ = GortexRateLimitWithConfig(cfg)

	require.NotNil(t, cfg.Store)
	store, ok := cfg.Store.(*MemoryRateLimiter)
	require.True(t, ok, "lazily-created store should be a *MemoryRateLimiter")

	// Stop must not panic and must be idempotent.
	store.Stop()
	store.Stop()
}
