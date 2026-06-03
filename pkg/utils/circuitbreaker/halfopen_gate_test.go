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
