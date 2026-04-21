package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func runRateLimiter(t *testing.T, cfg *GortexRateLimitConfig) (allow func() *httptest.ResponseRecorder) {
	t.Helper()
	mw := GortexRateLimitWithConfig(cfg)
	h := mw(func(c Context) error {
		c.Response().WriteHeader(http.StatusOK)
		_, _ = c.Response().Write([]byte("ok"))
		return nil
	})
	return func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		rec := httptest.NewRecorder()
		ctx := newTestContext(req, rec)
		if err := h(ctx); err != nil {
			rec.Code = http.StatusInternalServerError
		}
		return rec
	}
}

func TestRateLimitHeadersPresentOnAllowedRequest(t *testing.T) {
	store := NewMemoryRateLimiter()
	store.SetRate(rate.Limit(10), 5) // 5 burst, 10/s refill
	cfg := &GortexRateLimitConfig{
		Rate:    10,
		Burst:   5,
		Store:   store,
		KeyFunc: func(c Context) string { return "testkey" },
	}
	rec := runRateLimiter(t, cfg)()

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "5", rec.Header().Get(HeaderRateLimitLimit))

	remaining, err := strconv.Atoi(rec.Header().Get(HeaderRateLimitRemaining))
	require.NoError(t, err)
	// After consuming one token out of 5-burst we expect 4 remaining —
	// but the clock-based Tokens computation can float to 3 under load.
	assert.GreaterOrEqual(t, remaining, 3)
	assert.LessOrEqual(t, remaining, 4)

	assert.NotEmpty(t, rec.Header().Get(HeaderRateLimitReset))
	assert.Empty(t, rec.Header().Get(HeaderRetryAfter))
}

func TestRateLimitHeadersOn429IncludeRetryAfter(t *testing.T) {
	store := NewMemoryRateLimiter()
	store.SetRate(rate.Limit(1), 1) // 1 token, 1/s refill
	cfg := &GortexRateLimitConfig{
		Rate:    1,
		Burst:   1,
		Store:   store,
		KeyFunc: func(c Context) string { return "single-bucket" },
	}

	fire := runRateLimiter(t, cfg)
	// First request uses the only token.
	rec := fire()
	require.Equal(t, http.StatusOK, rec.Code)

	// Second request is rate-limited.
	rec = fire()
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "1", rec.Header().Get(HeaderRateLimitLimit))
	assert.Equal(t, "0", rec.Header().Get(HeaderRateLimitRemaining))

	retry := rec.Header().Get(HeaderRetryAfter)
	require.NotEmpty(t, retry)
	seconds, err := strconv.Atoi(retry)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, seconds, 1)
}

func TestMemoryRateLimiterStatusReportsBurst(t *testing.T) {
	store := NewMemoryRateLimiter()
	store.SetRate(rate.Limit(5), 7)

	limit, remaining, reset := store.Status("fresh-key")
	assert.Equal(t, 7, limit)
	// A fresh bucket should report ~full capacity.
	assert.GreaterOrEqual(t, remaining, 6)
	assert.LessOrEqual(t, remaining, 7)
	assert.False(t, reset.IsZero())
}
