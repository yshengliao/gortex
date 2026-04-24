package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// TestMemoryRateLimiter_TTLCleanup verifies that entries idle beyond the TTL
// are automatically removed by the background cleanup goroutine.
func TestMemoryRateLimiter_TTLCleanup(t *testing.T) {
	// Use a very short TTL and interval so the test doesn't take long.
	limiter := NewMemoryRateLimiterWithOptions(MemoryRateLimiterOptions{
		Rate:            rate.Limit(100),
		Burst:           100,
		TTL:             50 * time.Millisecond,
		CleanupInterval: 20 * time.Millisecond,
	})
	defer limiter.Stop()

	key := "cleanup-key"

	// Populate an entry.
	require.True(t, limiter.Allow(key), "first Allow must succeed")

	// Confirm the entry exists internally.
	limiter.mu.RLock()
	_, exists := limiter.entries[key]
	limiter.mu.RUnlock()
	require.True(t, exists, "entry must exist after Allow()")

	// Wait long enough for the entry to expire and cleanup to run.
	time.Sleep(200 * time.Millisecond)

	// The entry should have been removed.
	limiter.mu.RLock()
	_, existsAfter := limiter.entries[key]
	limiter.mu.RUnlock()
	assert.False(t, existsAfter, "expired entry must be removed by background cleanup")
}

// TestMemoryRateLimiter_Stop verifies that Stop() causes the background
// goroutine to exit cleanly (no panic, no goroutine leak).
func TestMemoryRateLimiter_Stop(t *testing.T) {
	limiter := NewMemoryRateLimiterWithOptions(MemoryRateLimiterOptions{
		Rate:            rate.Limit(10),
		Burst:           10,
		TTL:             1 * time.Minute,
		CleanupInterval: 100 * time.Millisecond,
	})

	_ = limiter.Allow("k")

	// Stop must not panic and should be idempotent.
	limiter.Stop()
	limiter.Stop() // second call must also be safe
}

// TestMemoryRateLimiter_CleanupPartial verifies that Cleanup() only removes
// entries older than TTL, not recently active ones.
func TestMemoryRateLimiter_CleanupPartial(t *testing.T) {
	limiter := NewMemoryRateLimiterWithOptions(MemoryRateLimiterOptions{
		Rate:            rate.Limit(100),
		Burst:           100,
		TTL:             200 * time.Millisecond,
		CleanupInterval: 10 * time.Minute, // disable auto-cleanup for this test
	})
	defer limiter.Stop()

	oldKey := "old"
	newKey := "new"

	// Populate old entry and wait for it to age past TTL.
	require.True(t, limiter.Allow(oldKey))
	time.Sleep(250 * time.Millisecond)

	// Populate new entry (fresh lastSeen).
	require.True(t, limiter.Allow(newKey))

	// Manually trigger cleanup.
	limiter.Cleanup()

	limiter.mu.RLock()
	_, oldExists := limiter.entries[oldKey]
	_, newExists := limiter.entries[newKey]
	limiter.mu.RUnlock()

	assert.False(t, oldExists, "old entry must be removed by Cleanup()")
	assert.True(t, newExists, "new entry must be kept by Cleanup()")
}
