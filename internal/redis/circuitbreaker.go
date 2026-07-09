package redis

import (
	"context"
	"sync"
	"time"
)

type breakerState int

const (
	closed breakerState = iota
	halfOpen
	open
)

// CircuitBreaker guards Redis calls: after failureThreshold consecutive
// failures it opens and fails fast for resetTimeout, then allows a single
// trial call (half-open) before fully closing again. This is what turns a
// Redis blip into a bounded-latency degradation instead of every goroutine
// piling up on a dead connection pool (the actual mechanism behind
// "connection-pool contention" under failure).
type CircuitBreaker struct {
	mu               sync.Mutex
	state            breakerState
	failures         int
	failureThreshold int
	resetTimeout     time.Duration
	openedAt         time.Time
}

func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{failureThreshold: failureThreshold, resetTimeout: resetTimeout}
}

func (cb *CircuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case open:
		if time.Since(cb.openedAt) >= cb.resetTimeout {
			cb.state = halfOpen
			return true
		}
		return false
	default:
		return true
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = closed
}

func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	if cb.state == halfOpen || cb.failures >= cb.failureThreshold {
		cb.state = open
		cb.openedAt = time.Now()
	}
}

// StateValue returns 0=closed, 1=half-open, 2=open, matching the
// aegis_circuit_breaker_state metric convention.
func (cb *CircuitBreaker) StateValue() float64 {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return float64(cb.state)
}

// Do runs fn if the breaker permits it, recording success/failure.
func (cb *CircuitBreaker) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if !cb.allow() {
		return ErrCircuitOpenLocal
	}
	err := fn(ctx)
	if err != nil {
		cb.recordFailure()
		return err
	}
	cb.recordSuccess()
	return nil
}

// ErrCircuitOpenLocal avoids an import cycle with pkg/errors; the gRPC
// handler layer maps this to the shared ErrCircuitOpen before returning
// a status to clients.
var ErrCircuitOpenLocal = &circuitOpenErr{}

type circuitOpenErr struct{}

func (*circuitOpenErr) Error() string { return "circuit breaker open" }
