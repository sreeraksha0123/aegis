package benchmark

import (
	"context"
	"os"
	"testing"
	"time"

	"aegis/internal/config"
	"aegis/internal/limiter"
	aredis "aegis/internal/redis"
)

// BenchmarkTokenBucket_Check and BenchmarkSlidingWindow_Check measure the
// actual per-call latency of a single CheckLimit-equivalent call against a
// real Redis instance (one EVALSHA round trip each). Run with:
//
//	REDIS_ADDR=localhost:6379 go test ./test/benchmark/... -bench=. -benchtime=3s
//
// These are single-connection, single-goroutine benchmarks measuring pure
// call latency — they are NOT a substitute for the k6 concurrent-load
// tests in scripts/benchmark/, which exercise the full gRPC + connection
// pool + concurrency path. Use both: this isolates Redis round-trip cost,
// k6 measures end-to-end system throughput under concurrency.

func redisAddr() string {
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		return v
	}
	return "localhost:6379"
}

func setupClient(b *testing.B) *aredis.Client {
	b.Helper()
	cfg := &config.RedisConfig{
		Addresses: []string{redisAddr()}, PoolSize: 50, MinIdleConns: 10,
		DialTimeout: 5 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second,
	}
	client, err := aredis.NewClient(cfg)
	if err != nil {
		b.Skipf("skipping: redis not reachable at %s (%v)", redisAddr(), err)
	}
	return client
}

func BenchmarkTokenBucket_Check(b *testing.B) {
	client := setupClient(b)
	defer client.Close()
	tb := limiter.NewTokenBucket(client, 1_000_000, 1_000_000, 60) // effectively unlimited, isolates round-trip cost
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := tb.Check(ctx, "bench_tenant", "bench_key", 1); err != nil {
			b.Fatalf("check failed: %v", err)
		}
	}
}

func BenchmarkSlidingWindow_Check(b *testing.B) {
	client := setupClient(b)
	defer client.Close()
	sw := limiter.NewSlidingWindow(client, time.Hour, 10_000_000, 60)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sw.Check(ctx, "bench_tenant", "bench_key_sw", 1); err != nil {
			b.Fatalf("check failed: %v", err)
		}
	}
}
