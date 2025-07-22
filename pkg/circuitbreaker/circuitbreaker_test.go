package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var errTest = errors.New("test error")

func TestNewCircuitBreaker(t *testing.T) {
	cb := New("test", DefaultConfig())
	assert.NotNil(t, cb)
	assert.Equal(t, "test", cb.name)
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreakerStates(t *testing.T) {
	assert.Equal(t, "closed", StateClosed.String())
	assert.Equal(t, "open", StateOpen.String())
	assert.Equal(t, "half-open", StateHalfOpen.String())
	assert.Equal(t, "unknown", State(999).String())
}

func TestCircuitBreakerClosedState(t *testing.T) {
	var stateChanges []string
	config := DefaultConfig()
	config.OnStateChange = func(name string, from, to State) {
		stateChanges = append(stateChanges, fmt.Sprintf("%s: %s->%s", name, from, to))
	}
	config.ReadyToTrip = func(counts Counts) bool {
		// Trip after 3 failures and > 50% failure ratio
		return counts.Requests >= 3 && counts.FailureRatio() > 0.5
	}
	
	cb := New("test", config)
	
	// Successful calls should not open the circuit
	for i := 0; i < 5; i++ {
		err := cb.Call(context.Background(), func(ctx context.Context) error {
			return nil
		})
		assert.NoError(t, err)
	}
	
	assert.Equal(t, StateClosed, cb.State())
	counts := cb.Counts()
	assert.Equal(t, uint32(5), counts.Requests)
	assert.Equal(t, uint32(5), counts.TotalSuccesses)
	assert.Equal(t, uint32(0), counts.TotalFailures)
	
	// Make more failures than successes to trip the circuit
	// Need to start a new generation first
	cb.mu.Lock()
	cb.toNewGeneration(time.Now())
	cb.mu.Unlock()
	
	// Make 2 failures - not enough to trip
	for i := 0; i < 2; i++ {
		cb.Call(context.Background(), func(ctx context.Context) error {
			return errTest
		})
	}
	assert.Equal(t, StateClosed, cb.State())
	
	// Make one more failure - should trip now (3 requests, 100% failure)
	cb.Call(context.Background(), func(ctx context.Context) error {
		return errTest
	})
	
	assert.Equal(t, StateOpen, cb.State())
	assert.Contains(t, stateChanges, "test: closed->open")
}

func TestCircuitBreakerOpenState(t *testing.T) {
	config := DefaultConfig()
	config.Timeout = 100 * time.Millisecond
	
	cb := New("test", config)
	
	// Force open state
	cb.setState(StateOpen)
	cb.expiry = time.Now().Add(config.Timeout)
	
	// Calls should fail immediately
	err := cb.Call(context.Background(), func(ctx context.Context) error {
		t.Fatal("Should not be called")
		return nil
	})
	assert.Equal(t, ErrCircuitOpen, err)
	
	// Wait for timeout
	time.Sleep(config.Timeout + 10*time.Millisecond)
	
	// Should transition to half-open
	err = cb.Call(context.Background(), func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, StateHalfOpen, cb.State())
}

func TestCircuitBreakerHalfOpenState(t *testing.T) {
	config := DefaultConfig()
	config.MaxRequests = 3
	
	cb := New("test", config)
	
	// Force half-open state
	cb.setState(StateHalfOpen)
	cb.expiry = time.Now().Add(time.Hour)
	
	// First MaxRequests should succeed
	var wg sync.WaitGroup
	successCount := atomic.Int32{}
	
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := cb.Call(context.Background(), func(ctx context.Context) error {
				successCount.Add(1)
				time.Sleep(10 * time.Millisecond)
				return nil
			})
			if err == nil {
				assert.LessOrEqual(t, successCount.Load(), int32(config.MaxRequests))
			}
		}()
	}
	
	wg.Wait()
	assert.Equal(t, int32(config.MaxRequests), successCount.Load())
}

func TestCircuitBreakerHalfOpenToOpen(t *testing.T) {
	config := DefaultConfig()
	config.MaxRequests = 1
	
	cb := New("test", config)
	
	// Force half-open state
	cb.setState(StateHalfOpen)
	cb.expiry = time.Now().Add(time.Hour)
	
	// Failed request should open the circuit
	err := cb.Call(context.Background(), func(ctx context.Context) error {
		return errTest
	})
	assert.Equal(t, errTest, err)
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreakerHalfOpenToClosed(t *testing.T) {
	config := DefaultConfig()
	config.MaxRequests = 3
	
	cb := New("test", config)
	
	// Force half-open state
	cb.setState(StateHalfOpen)
	cb.expiry = time.Now().Add(time.Hour)
	
	// Successful requests should close the circuit
	for i := 0; i < int(config.MaxRequests); i++ {
		err := cb.Call(context.Background(), func(ctx context.Context) error {
			return nil
		})
		assert.NoError(t, err)
	}
	
	assert.Equal(t, StateClosed, cb.State())
}

func TestCustomReadyToTrip(t *testing.T) {
	tripAfter := uint32(3)
	config := DefaultConfig()
	config.ReadyToTrip = func(counts Counts) bool {
		return counts.ConsecutiveFailures >= tripAfter
	}
	
	cb := New("test", config)
	
	// First two failures should not trip
	for i := 0; i < 2; i++ {
		cb.Call(context.Background(), func(ctx context.Context) error {
			return errTest
		})
		assert.Equal(t, StateClosed, cb.State())
	}
	
	// Third consecutive failure should trip
	cb.Call(context.Background(), func(ctx context.Context) error {
		return errTest
	})
	assert.Equal(t, StateOpen, cb.State())
}

func TestCallAsync(t *testing.T) {
	cb := New("test", DefaultConfig())
	
	// Test successful async call
	errCh := cb.CallAsync(context.Background(), func(ctx context.Context) error {
		return nil
	})
	
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for async call")
	}
	
	// Test failed async call
	errCh = cb.CallAsync(context.Background(), func(ctx context.Context) error {
		return errTest
	})
	
	select {
	case err := <-errCh:
		assert.Equal(t, errTest, err)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for async call")
	}
}

func TestCallAsyncWithOpenCircuit(t *testing.T) {
	cb := New("test", DefaultConfig())
	
	// Force open state
	cb.setState(StateOpen)
	cb.expiry = time.Now().Add(time.Hour)
	
	errCh := cb.CallAsync(context.Background(), func(ctx context.Context) error {
		t.Fatal("Should not be called")
		return nil
	})
	
	select {
	case err := <-errCh:
		assert.Equal(t, ErrCircuitOpen, err)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for async call")
	}
}

func TestCountsFailureRatio(t *testing.T) {
	tests := []struct {
		counts   Counts
		expected float64
	}{
		{Counts{}, 0.0},
		{Counts{Requests: 10, TotalFailures: 0}, 0.0},
		{Counts{Requests: 10, TotalFailures: 5}, 0.5},
		{Counts{Requests: 10, TotalFailures: 10}, 1.0},
	}
	
	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.counts.FailureRatio())
	}
}

func TestGenerationHandling(t *testing.T) {
	config := DefaultConfig()
	config.Interval = 50 * time.Millisecond
	
	cb := New("test", config)
	
	// Make some successful calls
	for i := 0; i < 5; i++ {
		cb.Call(context.Background(), func(ctx context.Context) error {
			return nil
		})
	}
	
	counts1 := cb.Counts()
	assert.Equal(t, uint32(5), counts1.Requests)
	
	// Wait for new generation
	time.Sleep(config.Interval + 10*time.Millisecond)
	
	// Make another call
	cb.Call(context.Background(), func(ctx context.Context) error {
		return nil
	})
	
	counts2 := cb.Counts()
	assert.Equal(t, uint32(1), counts2.Requests) // New generation
}

func TestConcurrentCalls(t *testing.T) {
	cb := New("test", DefaultConfig())
	
	var wg sync.WaitGroup
	numGoroutines := 50
	callsPerGoroutine := 100
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				cb.Call(context.Background(), func(ctx context.Context) error {
					if id%2 == 0 {
						return nil
					}
					return errTest
				})
			}
		}(i)
	}
	
	wg.Wait()
	
	// Circuit should eventually open due to failures
	assert.Equal(t, StateOpen, cb.State())
}

func BenchmarkCircuitBreakerClosed(b *testing.B) {
	cb := New("bench", DefaultConfig())
	ctx := context.Background()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		cb.Call(ctx, func(ctx context.Context) error {
			return nil
		})
	}
}

func BenchmarkCircuitBreakerOpen(b *testing.B) {
	cb := New("bench", DefaultConfig())
	cb.setState(StateOpen)
	cb.expiry = time.Now().Add(time.Hour)
	ctx := context.Background()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		cb.Call(ctx, func(ctx context.Context) error {
			return nil
		})
	}
}

func BenchmarkCircuitBreakerConcurrent(b *testing.B) {
	cb := New("bench", DefaultConfig())
	ctx := context.Background()
	
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cb.Call(ctx, func(ctx context.Context) error {
				return nil
			})
		}
	})
}