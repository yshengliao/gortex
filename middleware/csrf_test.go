package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runCSRF(t *testing.T, cfg CSRFConfig, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	mw := CSRFWithConfig(cfg)
	handler := mw(func(c Context) error {
		c.Response().WriteHeader(http.StatusOK)
		_, _ = c.Response().Write([]byte("ok"))
		return nil
	})
	rec := httptest.NewRecorder()
	ctx := newTestContext(req, rec)
	err := handler(ctx)
	if err != nil {
		// Surface HTTPError status code the way the framework would.
		if httpErr, ok := err.(interface{ StatusCode() int }); ok {
			rec.Code = httpErr.StatusCode()
		} else {
			rec.Code = http.StatusInternalServerError
		}
	}
	return rec
}

func TestCSRFIssuesTokenOnSafeMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := runCSRF(t, CSRFConfig{}, req)

	require.Equal(t, http.StatusOK, rec.Code)

	setCookie := rec.Header().Get("Set-Cookie")
	require.NotEmpty(t, setCookie, "middleware must set a CSRF cookie on safe methods")
	assert.Contains(t, setCookie, "_csrf=")
	assert.Contains(t, setCookie, "HttpOnly")
	assert.Contains(t, setCookie, "SameSite=Lax")

	assert.NotEmpty(t, rec.Header().Get("X-CSRF-Token"), "middleware must expose the token via response header")
}

func TestCSRFReusesExistingCookieOnSafeMethod(t *testing.T) {
	token := "existing-token-value"
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: token})

	rec := runCSRF(t, CSRFConfig{}, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// No rotation: we didn't issue a new cookie.
	assert.Empty(t, rec.Header().Get("Set-Cookie"))
	// But we still echo the token so the client can read it.
	assert.Equal(t, token, rec.Header().Get("X-CSRF-Token"))
}

func TestCSRFRejectsMissingTokenOnUnsafeMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := runCSRF(t, CSRFConfig{}, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCSRFRejectsCookieWithoutHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: "abc"})

	rec := runCSRF(t, CSRFConfig{}, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCSRFRejectsMismatchedToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: "cookie-token"})
	req.Header.Set("X-CSRF-Token", "different-token")

	rec := runCSRF(t, CSRFConfig{}, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCSRFAcceptsMatchingHeaderToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: "t0k3n"})
	req.Header.Set("X-CSRF-Token", "t0k3n")

	rec := runCSRF(t, CSRFConfig{}, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCSRFAcceptsMatchingFormToken(t *testing.T) {
	form := url.Values{}
	form.Set("csrf_token", "f0rm-t0k3n")
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: "f0rm-t0k3n"})

	rec := runCSRF(t, CSRFConfig{}, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCSRFSkipperBypassesValidation(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", nil)
	rec := runCSRF(t, CSRFConfig{
		Skipper: func(c Context) bool {
			return strings.HasPrefix(c.Request().URL.Path, "/api/webhook")
		},
	}, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCSRFTokensAreUnique(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := runCSRF(t, CSRFConfig{}, req)
		tok := rec.Header().Get("X-CSRF-Token")
		require.NotEmpty(t, tok)
		if _, dup := seen[tok]; dup {
			t.Fatalf("duplicate CSRF token after %d requests: %s", i, tok)
		}
		seen[tok] = struct{}{}
	}
}
