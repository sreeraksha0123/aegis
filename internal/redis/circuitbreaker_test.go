package redis

import (
	"context"
	"errors"
	"testing"
	"time"
)

// This is a unit test of the breaker in isolation (no real Redis needed),
// proving the actual state machine, since the "connection-pool
// contention" claim depends on this opening under repeated failure
// rather than on the integration test suite.
func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)
	ctx := context.Background()
	failing := func(ctx context.Context) error { return errors.New("simulated redis failure") }

	for i := 0; i < 3; i++ {
		if err := cb.Do(ctx, failing); err == nil {
			t.Fatalf("expected failure on attempt %d", i)
		}
	}

	// Breaker should now be open: calls should fail fast without even
	// invoking fn, distinguishable by the ErrCircuitOpenLocal sentinel.
	called := false
	err := cb.Do(ctx, func(ctx context.Context) error {
		called = true
		return nil
	})
	if err != ErrCircuitOpenLocal {
		t.Fatalf("expected ErrCircuitOpenLocal once open, got %v", err)
	}
	if called {
		t.Errorf("breaker should fail fast without calling fn while open")
	}
}

func TestCircuitBreaker_RecoversAfterCooldown(t *testing.T) {
	cb := NewCircuitBreaker(2, 30*time.Millisecond)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		cb.Do(ctx, func(ctx context.Context) error { return errors.New("fail") })
	}
	if err := cb.Do(ctx, func(ctx context.Context) error { return nil }); err != ErrCircuitOpenLocal {
		t.Fatalf("expected breaker to be open immediately after threshold, got %v", err)
	}

	time.Sleep(40 * time.Millisecond) // past resetTimeout

	called := false
	err := cb.Do(ctx, func(ctx context.Context) error {
		called = true
		return nil // trial call succeeds
	})
	if err != nil {
		t.Fatalf("expected trial call to succeed after cooldown, got %v", err)
	}
	if !called {
		t.Errorf("expected breaker to allow one trial call after cooldown (half-open)")
	}

	// And it should now be fully closed again.
	if err := cb.Do(ctx, func(ctx context.Context) error { return nil }); err != nil {
		t.Errorf("expected breaker to be closed after a successful trial, got %v", err)
	}
}
