package integration

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aegis/internal/config"
	"aegis/internal/limiter"
	aredis "aegis/internal/redis"
)

func testRedisAddr() string {
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		return v
	}
	return "localhost:6379"
}

func newTestClient(t *testing.T) *aredis.Client {
	t.Helper()
	cfg := &config.RedisConfig{
		Addresses:    []string{testRedisAddr()},
		PoolSize:     50,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
	client, err := aredis.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to connect to redis at %s: %v (start redis-server or set REDIS_ADDR)", testRedisAddr(), err)
	}
	return client
}

func TestTokenBucket_BurstThenDeny(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	tb := limiter.NewTokenBucket(client, 10, 5, 60) // capacity=10, refill=5/s
	ctx := context.Background()
	key := "test_burst_" + time.Now().Format("150405.000000")

	allowed := 0
	for i := 0; i < 15; i++ {
		res, err := tb.Check(ctx, "test_tenant", key, 1)
		if err != nil {
			t.Fatalf("check failed: %v", err)
		}
		if res.Allowed {
			allowed++
		}
	}

	if allowed != 10 {
		t.Errorf("expected exactly 10 of 15 rapid requests allowed (capacity=10), got %d", allowed)
	}
}

func TestTokenBucket_RefillOverTime(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	tb := limiter.NewTokenBucket(client, 2, 10, 60) // capacity=2, refill=10/s -> ~1 token per 100ms
	ctx := context.Background()
	key := "test_refill_" + time.Now().Format("150405.000000")

	// Drain the bucket
	for i := 0; i < 2; i++ {
		res, err := tb.Check(ctx, "test_tenant", key, 1)
		if err != nil || !res.Allowed {
			t.Fatalf("expected initial burst to be allowed, got allowed=%v err=%v", res.Allowed, err)
		}
	}

	res, err := tb.Check(ctx, "test_tenant", key, 1)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if res.Allowed {
		t.Errorf("expected bucket to be empty immediately after burst, but request was allowed")
	}

	time.Sleep(150 * time.Millisecond)

	res, err = tb.Check(ctx, "test_tenant", key, 1)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if !res.Allowed {
		t.Errorf("expected token to have refilled after 150ms at 10/s refill rate")
	}
}

// TestTokenBucket_NoRaceUnderConcurrency is the direct test of the "zero
// race conditions" claim: N goroutines hammer the same key concurrently;
// the total number of allowed requests must never exceed capacity, because
// the Lua script is atomic. If there were a race, allowed count would
// exceed capacity under concurrent load.
func TestTokenBucket_NoRaceUnderConcurrency(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	const capacity = 100
	tb := limiter.NewTokenBucket(client, capacity, 0, 60) // refill=0: pure capacity test
	ctx := context.Background()
	key := "test_race_" + time.Now().Format("150405.000000")

	const goroutines = 50
	const perGoroutine = 10 // 500 total attempts against a 100-token bucket

	var allowed int64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				res, err := tb.Check(ctx, "test_tenant", key, 1)
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

	if allowed != capacity {
		t.Errorf("race condition detected: expected exactly %d allowed under concurrency, got %d", capacity, allowed)
	}
}
