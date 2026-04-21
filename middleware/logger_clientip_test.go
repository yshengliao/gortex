package middleware

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustCIDR(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, cidr, err := net.ParseCIDR(s)
	require.NoError(t, err)
	return cidr
}

func TestClientIPFromRequestIgnoresForwardingHeadersWhenNoProxiesTrusted(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:41234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7")
	req.Header.Set("X-Real-IP", "198.51.100.7")

	assert.Equal(t, "203.0.113.9:41234", clientIPFromRequest(req, nil))
	assert.Equal(t, "203.0.113.9:41234", clientIPFromRequest(req, []*net.IPNet{}))
}

func TestClientIPFromRequestIgnoresSpoofedHeaderFromUntrustedPeer(t *testing.T) {
	trusted := []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:41234" // public peer
	req.Header.Set("X-Forwarded-For", "198.51.100.7")

	assert.Equal(t, "203.0.113.9:41234", clientIPFromRequest(req, trusted))
}

func TestClientIPFromRequestHonoursTrustedProxyXRealIP(t *testing.T) {
	trusted := []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.42:51234"
	req.Header.Set("X-Real-IP", "198.51.100.7")

	assert.Equal(t, "198.51.100.7", clientIPFromRequest(req, trusted))
}

func TestClientIPFromRequestHonoursTrustedProxyXForwardedFor(t *testing.T) {
	trusted := []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.42:51234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 10.0.0.42")

	assert.Equal(t, "198.51.100.7", clientIPFromRequest(req, trusted))
}

func TestClientIPFromRequestFallsBackToRemoteAddrWithoutHeaders(t *testing.T) {
	trusted := []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.42:51234"

	assert.Equal(t, "10.0.0.42:51234", clientIPFromRequest(req, trusted))
}

func TestClientIPFromRequestHandlesPortlessRemoteAddr(t *testing.T) {
	trusted := []*net.IPNet{mustCIDR(t, "127.0.0.0/8")}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1" // no port
	req.Header.Set("X-Real-IP", "198.51.100.7")

	assert.Equal(t, "198.51.100.7", clientIPFromRequest(req, trusted))
}

func TestPeerIsTrustedRejectsMalformedRemoteAddr(t *testing.T) {
	trusted := []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}
	assert.False(t, peerIsTrusted("not-an-ip", trusted))
	assert.False(t, peerIsTrusted("", trusted))
}
