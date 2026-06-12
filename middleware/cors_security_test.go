package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCORSWildcardWithCredentialsRejected(t *testing.T) {
	_, err := CORSWithConfig(&CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowCredentials: true,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCORSWildcardWithCredentials))
}

func TestCORSWildcardWithoutCredentialsAccepted(t *testing.T) {
	_, err := CORSWithConfig(&CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowCredentials: false,
	})
	require.NoError(t, err)
}

func TestCORSDefaultPanicsOnlyIfMisconfigured(t *testing.T) {
	// Default config has no credentials so this must not panic.
	mw := CORS()
	require.NotNil(t, mw)
}

func TestCORSWildcardHeadersWithCredentialsRejected(t *testing.T) {
	// AllowHeaders ["*"] + AllowCredentials=true is unsafe: the Fetch spec
	// does not treat literal "*" as a wildcard when credentials are sent.
	_, err := CORSWithConfig(&CORSConfig{
		AllowOrigins:     []string{"https://example.com"},
		AllowHeaders:     []string{"*"},
		AllowCredentials: true,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCORSWildcardHeadersWithCredentials))
}

func TestCORSWildcardHeadersWithCredentialsPanicsViaHandler(t *testing.T) {
	// CORSHandlerWithConfig panics on unsafe configs (mirrors the origin case).
	assert.Panics(t, func() {
		CORSHandlerWithConfig(&CORSConfig{
			AllowOrigins:     []string{"https://example.com"},
			AllowHeaders:     []string{"*"},
			AllowCredentials: true,
		}, http.NotFoundHandler())
	})
}

func TestCORSWildcardHeadersWithoutCredentialsAllowed(t *testing.T) {
	// AllowHeaders ["*"] without credentials must still work (current behaviour).
	_, err := CORSWithConfig(&CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowCredentials: false,
		AllowHeaders:     []string{"*"},
	})
	require.NoError(t, err)
}

func TestCORSHeaderReflectionWithoutCredentials(t *testing.T) {
	// When AllowHeaders is ["*"] and there are no credentials, the preflight
	// handler should reflect the client's requested headers (existing behaviour).
	mw, err := CORSWithConfig(&CORSConfig{
		AllowOrigins:     []string{"https://example.com"},
		AllowHeaders:     []string{"*"},
		AllowCredentials: false,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "X-Custom-Header, Content-Type")
	rec := httptest.NewRecorder()
	ctx := newMockContext(req, rec)

	err = mw(func(c Context) error { return nil })(ctx)
	require.NoError(t, err)

	assert.Equal(t, "X-Custom-Header, Content-Type",
		rec.Header().Get("Access-Control-Allow-Headers"),
		"reflected headers should match client's request when no credentials")
}

func TestCORSEchoesConcreteOriginWhenCredentials(t *testing.T) {
	mw, err := CORSWithConfig(&CORSConfig{
		AllowOrigins:     []string{"https://good.example"},
		AllowCredentials: true,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://good.example")
	rec := httptest.NewRecorder()
	ctx := newMockContext(req, rec)

	err = mw(func(c Context) error { return nil })(ctx)
	require.NoError(t, err)

	assert.Equal(t, "https://good.example", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
}
