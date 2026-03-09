package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/SilentPlaces/rate_limiter/internal/application/ports"
	"github.com/SilentPlaces/rate_limiter/internal/domain/config"
)

type FixedWindowLimiter struct {
	score      ports.LimiterScore
	scriptSHA1 string
}

func NewFixedWindowLimiter(score ports.LimiterScore, scriptSHA1 string) ports.RateLimiter {
	return &FixedWindowLimiter{
		score:      score,
		scriptSHA1: scriptSHA1,
	}
}

func FixedWindowLimiterFactory(score ports.LimiterScore, scriptSHA1 string) ports.RateLimiter {
	return NewFixedWindowLimiter(score, scriptSHA1)
}

func (f *FixedWindowLimiter) Allow(ctx context.Context, key string, cfg config.AlgorithmConfig) (ports.RateLimitInfo, error) {
	fixedCfg, ok := cfg.(config.FixedWindowConfig)
	if !ok {
		return ports.RateLimitInfo{}, fmt.Errorf("invalid config type for FixedWindowLimiter, got %T", cfg)
	}

	res, err := f.score.EvalSha(ctx, f.scriptSHA1, []string{key}, []interface{}{fixedCfg.Window, fixedCfg.Limit})
	if err != nil {
		return ports.RateLimitInfo{}, err
	}

	result, ok := res.([]interface{})
	if !ok || len(result) < 4 {
		return ports.RateLimitInfo{}, fmt.Errorf("unexpected lua script response")
	}

	allowed, _ := result[0].(int64)
	remaining, _ := result[2].(int64)
	ttl, _ := result[3].(int64)

	var resetTime int64
	if ttl > 0 {
		resetTime = time.Now().Unix() + ttl
	}

	return ports.RateLimitInfo{
		Allowed:   allowed == 1,
		Limit:     fixedCfg.Limit,
		Remaining: int(remaining),
		ResetTime: resetTime,
	}, nil
}
