package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	httpctx "github.com/yshengliao/gortex/transport/http"
)

// DefaultCSRFTokenBytes is the amount of random entropy behind each CSRF
// token. 32 bytes / 256 bits lines up with modern session-token practice
// and is well above what base64 length heuristics can meaningfully
// brute-force.
const DefaultCSRFTokenBytes = 32

// ErrCSRFTokenMismatch is returned (as an HTTP 403) when the submitted
// token does not match the one bound to the session cookie.
var ErrCSRFTokenMismatch = httpctx.NewHTTPError(http.StatusForbidden, "csrf token mismatch")

// ErrCSRFTokenMissing is returned when the request carries no token at
// all on a method that requires one.
var ErrCSRFTokenMissing = httpctx.NewHTTPError(http.StatusForbidden, "csrf token missing")

// CSRFConfig tunes the CSRF middleware. The zero value is usable via
// CSRFWithConfig — missing fields fall back to sensible defaults at
// construction time.
type CSRFConfig struct {
	// CookieName is the name of the cookie that holds the token.
	CookieName string
	// HeaderName is the HTTP header clients use to echo the token on
	// unsafe requests. Echoed back on safe-method responses so SPA
	// clients can read it off a preflight request.
	HeaderName string
	// FormFieldName is the form field inspected when no header is set.
	FormFieldName string
	// CookiePath scopes the cookie to a URL path. Default "/".
	CookiePath string
	// CookieDomain, when non-empty, sets the cookie's Domain attribute.
	CookieDomain string
	// CookieMaxAge controls how long the cookie lives; default 24h.
	CookieMaxAge time.Duration
	// CookieSecure marks the cookie as Secure. Default true — clear it
	// only if you explicitly run over plain HTTP (local development).
	CookieSecure bool
	// CookieSameSite controls the SameSite attribute. Default Lax.
	CookieSameSite http.SameSite
	// TokenBytes is the amount of random bytes generated per token.
	// Defaults to DefaultCSRFTokenBytes.
	TokenBytes int
	// Skipper, when non-nil and returning true, bypasses both token
	// issuance and validation for a request.
	Skipper func(Context) bool
}

func (c *CSRFConfig) applyDefaults() {
	if c.CookieName == "" {
		c.CookieName = "_csrf"
	}
	if c.HeaderName == "" {
		c.HeaderName = "X-CSRF-Token"
	}
	if c.FormFieldName == "" {
		c.FormFieldName = "csrf_token"
	}
	if c.CookiePath == "" {
		c.CookiePath = "/"
	}
	if c.CookieMaxAge == 0 {
		c.CookieMaxAge = 24 * time.Hour
	}
	if c.CookieSameSite == 0 {
		c.CookieSameSite = http.SameSiteLaxMode
	}
	if c.TokenBytes <= 0 {
		c.TokenBytes = DefaultCSRFTokenBytes
	}
}

// CSRF returns the middleware with defaults (HttpOnly + Secure + Lax,
// 32-byte tokens, 24h cookie).
func CSRF() MiddlewareFunc {
	return CSRFWithConfig(CSRFConfig{CookieSecure: true})
}

// CSRFWithConfig returns a CSRF middleware wired from the supplied
// config.
func CSRFWithConfig(cfg CSRFConfig) MiddlewareFunc {
	cfg.applyDefaults()

	return func(next HandlerFunc) HandlerFunc {
		return func(c Context) error {
			if cfg.Skipper != nil && cfg.Skipper(c) {
				return next(c)
			}

			req := c.Request()
			method := req.Method
			safe := isSafeCSRFMethod(method)

			// Load the existing token, if any. We never trust it
			// without re-validating below, but we re-use it to avoid
			// churning cookies on every safe request.
			existing := ""
			if cookie, err := c.Cookie(cfg.CookieName); err == nil && cookie != nil {
				existing = cookie.Value
			}

			if !safe {
				submitted := req.Header.Get(cfg.HeaderName)
				if submitted == "" {
					submitted = c.FormValue(cfg.FormFieldName)
				}
				if existing == "" || submitted == "" {
					return ErrCSRFTokenMissing
				}
				if subtle.ConstantTimeCompare([]byte(existing), []byte(submitted)) != 1 {
					return ErrCSRFTokenMismatch
				}
				// Valid: fall through to the handler. The cookie stays
				// as-is so concurrent tabs don't trip over rotated
				// tokens.
				return next(c)
			}

			token := existing
			if token == "" {
				generated, err := generateCSRFToken(cfg.TokenBytes)
				if err != nil {
					return err
				}
				token = generated
				c.SetCookie(&http.Cookie{
					Name:     cfg.CookieName,
					Value:    token,
					Path:     cfg.CookiePath,
					Domain:   cfg.CookieDomain,
					Expires:  time.Now().Add(cfg.CookieMaxAge),
					MaxAge:   int(cfg.CookieMaxAge.Seconds()),
					Secure:   cfg.CookieSecure,
					HttpOnly: true,
					SameSite: cfg.CookieSameSite,
				})
			}
			// Expose the token so SPA clients can read it on their
			// bootstrap request and echo it on state-changing calls.
			c.Response().Header().Set(cfg.HeaderName, token)

			return next(c)
		}
	}
}

// isSafeCSRFMethod reports whether method is one of the RFC 7231
// "safe" methods, which by definition do not mutate server state and
// therefore don't require a CSRF token on the request.
func isSafeCSRFMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}
	return false
}

// generateCSRFToken returns a base64-URL token built from the configured
// amount of random entropy. We use URL-safe base64 so tokens can be
// embedded in headers, form fields, and URLs without further escaping.
func generateCSRFToken(size int) (string, error) {
	if size <= 0 {
		return "", errors.New("csrf: token size must be positive")
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
