package middleware

import (
	"errors"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactHeadersMasksSensitive(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer leaked")
	req.Header.Set("Cookie", "session=leaked")
	req.Header.Set("X-Api-Key", "sk_live_leaked")
	req.Header.Set("X-Custom-Business-Header", "safe")

	out := redactHeaders(req.Header)
	assert.Equal(t, redactedPlaceholder, out.Get("Authorization"))
	assert.Equal(t, redactedPlaceholder, out.Get("Cookie"))
	assert.Equal(t, redactedPlaceholder, out.Get("X-Api-Key"))
	assert.Equal(t, "safe", out.Get("X-Custom-Business-Header"))
}

func TestRedactURLMasksSensitiveQueryParams(t *testing.T) {
	u, err := url.Parse("/api/users?token=sk_live_leaked&id=42&password=leaked&page=3")
	require.NoError(t, err)
	out := redactURL(u)

	parsed, err := url.Parse(out)
	require.NoError(t, err)

	q := parsed.Query()
	assert.Equal(t, redactedPlaceholder, q.Get("token"))
	assert.Equal(t, redactedPlaceholder, q.Get("password"))
	assert.Equal(t, "42", q.Get("id"))
	assert.Equal(t, "3", q.Get("page"))
}

func TestExtractErrorInfoRedactsRequestFields(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/login?token=leaked&next=/dashboard", nil)
	req.Header.Set("Authorization", "Bearer leaked")
	req.Header.Set("User-Agent", "unit-test")
	rec := httptest.NewRecorder()
	ctx := newMockContext(req, rec)

	info := extractErrorInfo(errors.New("boom"), ctx, DefaultGortexDevErrorPageConfig)

	assert.NotContains(t, info.RequestDetails["url"], "leaked")
	parsed, err := url.Parse(info.RequestDetails["url"])
	require.NoError(t, err)
	assert.Equal(t, redactedPlaceholder, parsed.Query().Get("token"))
	assert.Equal(t, []string{redactedPlaceholder}, info.Headers["Authorization"])
	assert.Equal(t, []string{"unit-test"}, info.Headers["User-Agent"])
}
