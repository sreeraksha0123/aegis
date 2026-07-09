package redis

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// RetryWithBackoff retries fn up to maxAttempts times with exponential
// backoff + jitter, honoring ctx cancellation between attempts. Intended
// for transient Redis errors (network blips), not for logic errors.
func RetryWithBackoff(ctx context.Context, maxAttempts int, base time.Duration, fn func(ctx context.Context) error) error {
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err = fn(ctx)
		if err == nil {
			return nil
		}
		if attempt == maxAttempts-1 {
			break
		}
		backoff := time.Duration(float64(base) * math.Pow(2, float64(attempt)))
		jitter := time.Duration(rand.Int63n(int64(backoff) / 2))
		select {
		case <-time.After(backoff + jitter):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}
