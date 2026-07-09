package limiter

import (
	"context"
	"fmt"
	"time"

	aredis "aegis/internal/redis"
)

type TokenBucket struct {
	client       *aredis.Client
	capacity     int64
	refillRate   int64 // tokens per second
	ttlSeconds   int64
	keyPrefix    string
}

func NewTokenBucket(client *aredis.Client, capacity, refillRate, ttlSeconds int64) *TokenBucket {
	return &TokenBucket{
		client:     client,
		capacity:   capacity,
		refillRate: refillRate,
		ttlSeconds: ttlSeconds,
		keyPrefix:  "ratelimit:token_bucket",
	}
}

func (t *TokenBucket) Name() string { return "token_bucket" }

func (t *TokenBucket) Check(ctx context.Context, tenant, key string, requested int64) (Result, error) {
	fullKey := fmt.Sprintf("%s:%s:%s", t.keyPrefix, tenant, key)
	now := time.Now().UnixMilli()

	res, err := t.client.EvalShaOrLoad(ctx, "token_bucket", TokenBucketScript,
		[]string{fullKey}, t.capacity, t.refillRate, now, requested, t.ttlSeconds)
	if err != nil {
		return Result{}, fmt.Errorf("token_bucket check: %w", err)
	}

	vals, ok := res.([]interface{})
	if !ok || len(vals) != 4 {
		return Result{}, fmt.Errorf("token_bucket: unexpected script result: %#v", res)
	}

	allowed := toInt64(vals[0]) == 1
	remaining := toInt64(vals[1])
	retryAfter := toInt64(vals[2])
	limit := toInt64(vals[3])

	return Result{
		Allowed:    allowed,
		Remaining:  remaining,
		ResetTime:  now + retryAfter,
		Limit:      limit,
		RetryAfter: retryAfter,
	}, nil
}

func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return 0
	}
}
