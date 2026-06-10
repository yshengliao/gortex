package middleware

import (
	"net"
	"net/http"
	"strings"
)

// clientIPFromRequest resolves the logical client IP for a request.
// Forwarding headers are only honoured when req.RemoteAddr is in one of
// the configured trustedProxies CIDRs; otherwise the direct peer address
// is returned unchanged, preventing a malicious client from spoofing an
// IP by simply sending X-Forwarded-For.
//
// Shared by the logger and rate-limit middleware so both apply the same
// trusted-proxy policy to forwarding headers.
func clientIPFromRequest(req *http.Request, trustedProxies []*net.IPNet) string {
	remoteAddr := req.RemoteAddr
	if !peerIsTrusted(remoteAddr, trustedProxies) {
		return remoteAddr
	}

	if ip := strings.TrimSpace(req.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	if fwd := req.Header.Get("X-Forwarded-For"); fwd != "" {
		// The left-most entry is the originating client; trailing
		// entries are the proxy chain and should be discarded.
		if idx := strings.IndexByte(fwd, ','); idx >= 0 {
			return strings.TrimSpace(fwd[:idx])
		}
		return strings.TrimSpace(fwd)
	}
	return remoteAddr
}

// peerIsTrusted reports whether the network peer behind remoteAddr falls
// within one of the trustedProxies CIDRs.
func peerIsTrusted(remoteAddr string, trustedProxies []*net.IPNet) bool {
	if len(trustedProxies) == 0 {
		return false
	}
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, cidr := range trustedProxies {
		if cidr == nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}
