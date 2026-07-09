package integration

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aegis/internal/limiter"
)

func TestSlidingWindow_LimitEnforced(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	sw := limiter.NewSlidingWindow(client, 1*time.Second, 5, 60)
	ctx := context.Background()
	key := "test_sw_limit_" + time.Now().Format("150405.000000")

	allowed := 0
	for i := 0; i < 8; i++ {
		res, err := sw.Check(ctx, "test_tenant", key, 1)
		if err != nil {
			t.Fatalf("check failed: %v", err)
		}
		if res.Allowed {
			allowed++
		}
	}

	if allowed != 5 {
		t.Errorf("expected exactly 5 of 8 requests allowed within a 1s/5-req window, got %d", allowed)
	}
}

func TestSlidingWindow_SlidesOverTime(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	sw := limiter.NewSlidingWindow(client, 300*time.Millisecond, 2, 60)
	ctx := context.Background()
	key := "test_sw_slide_" + time.Now().Format("150405.000000")

	for i := 0; i < 2; i++ {
		res, err := sw.Check(ctx, "test_tenant", key, 1)
		if err != nil || !res.Allowed {
			t.Fatalf("expected initial requests to be allowed, got allowed=%v err=%v", res.Allowed, err)
		}
	}

	res, err := sw.Check(ctx, "test_tenant", key, 1)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if res.Allowed {
		t.Errorf("expected 3rd request within window to be denied")
	}

	time.Sleep(350 * time.Millisecond)

	res, err = sw.Check(ctx, "test_tenant", key, 1)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if !res.Allowed {
		t.Errorf("expected request to be allowed after the window fully slid past")
	}
}

// TestSlidingWindow_NoRaceUnderConcurrency mirrors the token bucket race
// test: concurrent hammering of one key must never let more than `limit`
// requests through, since ZCARD+ZADD happen atomically inside one EVAL.
func TestSlidingWindow_NoRaceUnderConcurrency(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	const limit = 100
	sw := limiter.NewSlidingWindow(client, 10*time.Second, limit, 60)
	ctx := context.Background()
	key := "test_sw_race_" + time.Now().Format("150405.000000")

	const goroutines = 50
	const perGoroutine = 10

	var allowed int64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				res, err := sw.Check(ctx, "test_tenant", key, 1)
				if err != nil {
					t.Errorf("check failed: %v", err)
					return
				}
				if res.Allowed {
					atomic.AddInt64(&allowed, 1)
				}
			}
		}()
	}
	wg.Wait()

	if allowed != limit {
		t.Errorf("race condition detected: expected exactly %d allowed under concurrency, got %d", limit, allowed)
	}
}
