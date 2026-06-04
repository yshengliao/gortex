package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// State represents the state of the circuit breaker
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

var (
	// ErrCircuitOpen is returned when the circuit breaker is open
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrTooManyRequests is returned when half-open state limit is reached
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// Config represents circuit breaker configuration
type Config struct {
	// MaxRequests is the maximum number of requests allowed to pass through
	// when the circuit breaker is half-open
	MaxRequests uint32

	// Interval is the cyclic period of the closed state
	Interval time.Duration

	// Timeout is the timeout for the circuit breaker to stay in open state
	Timeout time.Duration

	// ReadyToTrip is called with a copy of Counts whenever a request fails
	// in the closed state. If ReadyToTrip returns true, the circuit breaker
	// will be placed into the open state
	ReadyToTrip func(counts Counts) bool

	// OnStateChange is called whenever the state of the circuit breaker changes
	OnStateChange func(name string, from State, to State)
}

// DefaultConfig returns a default configuration for the circuit breaker
func DefaultConfig() Config {
	return Config{
		MaxRequests: 1,
		Interval:    time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			return counts.Requests > 10 && counts.FailureRatio() > 0.5
		},
		OnStateChange: nil,
	}
}

// Counts holds the count of requests and their successes/failures
type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

// FailureRatio returns the failure ratio
func (c Counts) FailureRatio() float64 {
	if c.Requests == 0 {
		return 0
	}
	return float64(c.TotalFailures) / float64(c.Requests)
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name   string
	config Config
	state  atomic.Value // State
	mu     sync.Mutex
	counts Counts
	expiry time.Time
	// halfOpen counts in-flight requests admitted while half-open. It is
	// guarded by mu (not atomics) so the MaxRequests gate stays consistent
	// with the state/expiry transitions that also run under mu.
	halfOpen uint32
	// halfOpenSuccesses counts requests that have *succeeded* while half-open.
	// The circuit only closes once this reaches MaxRequests, so a single early
	// success cannot close the breaker while sibling probes are still in flight
	// (or about to fail) when MaxRequests > 1. Also guarded by mu.
	halfOpenSuccesses uint32
}

// New creates a new circuit breaker
func New(name string, config Config) *CircuitBreaker {
	cb := &CircuitBreaker{
		name:   name,
		config: config,
		expiry: time.Now(),
	}
	cb.state.Store(StateClosed)
	return cb
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() State {
	return cb.state.Load().(State)
}

// Counts returns the current counts
func (cb *CircuitBreaker) Counts() Counts {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.counts
}

// Call executes the given function if the circuit breaker allows it
func (cb *CircuitBreaker) Call(ctx context.Context, fn func(ctx context.Context) error) error {
	generation, err := cb.beforeRequest()
	if err != nil {
		return err
	}

	err = fn(ctx)
	cb.afterRequest(generation, err)
	return err
}

// CallAsync executes the given function asynchronously if the circuit breaker allows it
func (cb *CircuitBreaker) CallAsync(ctx context.Context, fn func(ctx context.Context) error) <-chan error {
	errCh := make(chan error, 1)

	go func() {
		generation, err := cb.beforeRequest()
		if err != nil {
			errCh <- err
			return
		}

		err = fn(ctx)
		cb.afterRequest(generation, err)
		errCh <- err
	}()

	return errCh
}

// beforeRequest is called before a request is made
func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	state := cb.State()

	switch state {
	case StateClosed:
		return cb.onBeforeRequestClosed()
	case StateOpen:
		return cb.onBeforeRequestOpen()
	case StateHalfOpen:
		return cb.onBeforeRequestHalfOpen()
	default:
		panic("unknown state")
	}
}

// afterRequest is called after a request is made
func (cb *CircuitBreaker) afterRequest(generation uint64, err error) {
	state := cb.State()

	switch state {
	case StateClosed:
		cb.onAfterRequestClosed(err)
	case StateHalfOpen:
		cb.onAfterRequestHalfOpen(generation, err)
	}
}

// onBeforeRequestClosed handles before request logic for closed state
func (cb *CircuitBreaker) onBeforeRequestClosed() (uint64, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	if cb.expiry.Before(now) {
		cb.toNewGeneration(now)
	}

	return 0, nil
}

// onAfterRequestClosed handles after request logic for closed state
func (cb *CircuitBreaker) onAfterRequestClosed(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.counts.Requests++
	if err != nil {
		cb.counts.TotalFailures++
		cb.counts.ConsecutiveSuccesses = 0
		cb.counts.ConsecutiveFailures++

		if cb.config.ReadyToTrip(cb.counts) {
			cb.setState(StateOpen)
			cb.expiry = time.Now().Add(cb.config.Timeout)
		}
	} else {
		cb.counts.TotalSuccesses++
		cb.counts.ConsecutiveFailures = 0
		cb.counts.ConsecutiveSuccesses++
	}
}

// onBeforeRequestOpen handles before request logic for open state
func (cb *CircuitBreaker) onBeforeRequestOpen() (uint64, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	if cb.expiry.Before(now) {
		cb.setState(StateHalfOpen)
		// The request that triggers this transition is the first half-open
		// probe, so count it (halfOpen=1). Otherwise half-open would admit
		// MaxRequests *more* requests on top of it (MaxRequests+1 total).
		cb.halfOpen = 1
		cb.halfOpenSuccesses = 0
		cb.expiry = now.Add(cb.config.Interval)
		return uint64(cb.expiry.UnixNano()), nil
	}

	return 0, ErrCircuitOpen
}

// onBeforeRequestHalfOpen handles before request logic for half-open state.
// The admission gate and the generation read happen under the same lock so
// the MaxRequests cap cannot be exceeded by requests racing in concurrently.
func (cb *CircuitBreaker) onBeforeRequestHalfOpen() (uint64, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.halfOpen >= cb.config.MaxRequests {
		return 0, ErrTooManyRequests
	}
	cb.halfOpen++

	return uint64(cb.expiry.UnixNano()), nil
}

// onAfterRequestHalfOpen handles after request logic for half-open state
func (cb *CircuitBreaker) onAfterRequestHalfOpen(generation uint64, err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check if this request belongs to current generation
	if generation != uint64(cb.expiry.UnixNano()) {
		return
	}

	if err != nil {
		cb.setState(StateOpen)
		cb.expiry = time.Now().Add(cb.config.Timeout)
		cb.counts = Counts{}
		cb.halfOpenSuccesses = 0
	} else {
		// Close only after MaxRequests *successful* probes, not merely
		// MaxRequests admitted requests: with several probes in flight the
		// first success must not close the breaker while siblings may still
		// fail (matters when MaxRequests > 1). The transition counts its own
		// probe via halfOpen=1, so every admitted probe is counted here.
		cb.halfOpenSuccesses++
		if cb.halfOpenSuccesses >= cb.config.MaxRequests {
			cb.setState(StateClosed)
			cb.toNewGeneration(time.Now())
		}
	}
}

// setState changes the state of the circuit breaker
func (cb *CircuitBreaker) setState(state State) {
	prevState := cb.State()
	cb.state.Store(state)

	if cb.config.OnStateChange != nil && prevState != state {
		cb.config.OnStateChange(cb.name, prevState, state)
	}
}

// toNewGeneration starts a new generation
func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.counts = Counts{}
	cb.expiry = now.Add(cb.config.Interval)
}

// String returns a string representation of the state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}
