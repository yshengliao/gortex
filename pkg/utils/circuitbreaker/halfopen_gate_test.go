package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCircuitBreaker_HalfOpenAdmissionGateConcurrent is a regression test for
// the half-open admission gate. The counter used to be incremented outside the
// lock guarding the state/expiry, so concurrent callers could slip past the
// MaxRequests cap. This test holds every admitted call in-flight and asserts
// that exactly MaxRequests are admitted no matter how many race in.
func TestCircuitBreaker_HalfOpenAdmissionGateConcurrent(t *testing.T) {
	config := DefaultConfig()
	config.MaxRequests = 3

	cb := New("gate", config)
	cb.setState(StateHalfOpen)
	cb.expiry = time.Now().Add(time.Hour)

	const callers = 64
	var admitted, rejected atomic.Int32
	release := make(chan struct{})

	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := cb.Call(context.Background(), func(context.Context) error {
				admitted.Add(1)
				<-release // hold the admitted slot so all admissions overlap
				return nil
			})
			if errors.Is(err, ErrTooManyRequests) {
				rejected.Add(1)
			}
		}()
	}

	// Every caller beyond MaxRequests must be rejected while the admitted ones
	// are still blocked.
	require.Eventually(t, func() bool {
		return rejected.Load() == int32(callers)-int32(config.MaxRequests)
	}, 2*time.Second, time.Millisecond)

	assert.Equal(t, int32(config.MaxRequests), admitted.Load(),
		"half-open gate must admit at most MaxRequests concurrently")

	close(release)
	wg.Wait()
}

// TestCircuitBreaker_HalfOpenClosesOnSuccessesNotAdmissions is a regression test
// for the half-open close gate. It used to close the breaker once MaxRequests
// requests had been *admitted* (cb.halfOpen), so with two probes in flight the
// first success would close the breaker even though the second had not yet
// returned. The gate must count successes instead.
func TestCircuitBreaker_HalfOpenClosesOnSuccessesNotAdmissions(t *testing.T) {
	config := DefaultConfig()
	config.MaxRequests = 2

	cb := New("successes", config)
	cb.setState(StateHalfOpen)
	cb.expiry = time.Now().Add(time.Hour)

	// Admit two probes up-front (both in flight) before either completes.
	gen1, err := cb.onBeforeRequestHalfOpen()
	require.NoError(t, err)
	gen2, err := cb.onBeforeRequestHalfOpen()
	require.NoError(t, err)

	// First probe succeeds. The old admission-counting gate would already close
	// here (halfOpen == MaxRequests); it must stay half-open since only one of
	// the two probes has actually succeeded.
	cb.onAfterRequestHalfOpen(gen1, nil)
	assert.Equal(t, StateHalfOpen, cb.State(),
		"breaker must stay half-open until MaxRequests successes, not admissions")

	// Second probe also succeeds → MaxRequests successes reached → closed.
	cb.onAfterRequestHalfOpen(gen2, nil)
	assert.Equal(t, StateClosed, cb.State())
}

// TestCircuitBreaker_HalfOpenReopensOnFailureAfterEarlySuccess verifies that a
// later failure still reopens the breaker even after an earlier probe in the
// same half-open batch succeeded (the early success must not have latched it
// closed).
func TestCircuitBreaker_HalfOpenReopensOnFailureAfterEarlySuccess(t *testing.T) {
	config := DefaultConfig()
	config.MaxRequests = 2

	cb := New("reopen", config)
	cb.setState(StateHalfOpen)
	cb.expiry = time.Now().Add(time.Hour)

	gen1, err := cb.onBeforeRequestHalfOpen()
	require.NoError(t, err)
	gen2, err := cb.onBeforeRequestHalfOpen()
	require.NoError(t, err)

	cb.onAfterRequestHalfOpen(gen1, nil)
	require.Equal(t, StateHalfOpen, cb.State())

	cb.onAfterRequestHalfOpen(gen2, errors.New("boom"))
	assert.Equal(t, StateOpen, cb.State())
}
