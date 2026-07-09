package limiter

import (
	"context"
	"fmt"
	"time"

	aredis "aegis/internal/redis"
)

type SlidingWindow struct {
	client     *aredis.Client
	windowMs   int64
	limit      int64
	ttlSeconds int64
	keyPrefix  string
}

func NewSlidingWindow(client *aredis.Client, window time.Duration, limit, ttlSeconds int64) *SlidingWindow {
	return &SlidingWindow{
		client:     client,
		windowMs:   window.Milliseconds(),
		limit:      limit,
		ttlSeconds: ttlSeconds,
		keyPrefix:  "ratelimit:sliding_window",
	}
}

func (s *SlidingWindow) Name() string { return "sliding_window" }

func (s *SlidingWindow) Check(ctx context.Context, tenant, key string, requested int64) (Result, error) {
	fullKey := fmt.Sprintf("%s:%s:%s", s.keyPrefix, tenant, key)
	now := time.Now().UnixMilli()

	res, err := s.client.EvalShaOrLoad(ctx, "sliding_window", SlidingWindowScript,
		[]string{fullKey}, s.windowMs, s.limit, now, requested, s.ttlSeconds)
	if err != nil {
		return Result{}, fmt.Errorf("sliding_window check: %w", err)
	}

	vals, ok := res.([]interface{})
	if !ok || len(vals) != 4 {
		return Result{}, fmt.Errorf("sliding_window: unexpected script result: %#v", res)
	}

	allowed := toInt64(vals[0]) == 1
	remaining := toInt64(vals[1])
	resetTime := toInt64(vals[2])
	limit := toInt64(vals[3])

	var retryAfter int64
	if !allowed {
		retryAfter = resetTime - now
		if retryAfter < 0 {
			retryAfter = 0
		}
	}

	return Result{
		Allowed:    allowed,
		Remaining:  remaining,
		ResetTime:  resetTime,
		Limit:      limit,
		RetryAfter: retryAfter,
	}, nil
}
