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
