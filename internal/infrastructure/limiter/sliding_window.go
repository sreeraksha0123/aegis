package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/SilentPlaces/rate_limiter/internal/application/ports"
	"github.com/SilentPlaces/rate_limiter/internal/domain/config"
	"github.com/SilentPlaces/rate_limiter/internal/domain/errors"
	"github.com/google/uuid"
)

type SlidingWindowLimiter struct {
	score      ports.LimiterScore
	scriptSHA1 string
}

func NewSlidingWindowLimiter(score ports.LimiterScore, scriptSHA1 string) ports.RateLimiter {
	return &SlidingWindowLimiter{scriptSHA1: scriptSHA1, score: score}
}

func SlidingWindowLimiterFactory(score ports.LimiterScore, scriptSHA1 string) ports.RateLimiter {
	return NewSlidingWindowLimiter(score, scriptSHA1)
}

func (s *SlidingWindowLimiter) Allow(ctx context.Context, key string, cfg config.AlgorithmConfig) (ports.RateLimitInfo, error) {
	slidingConfig, ok := cfg.(config.SlidingWindowConfig)
	if !ok {
		return ports.RateLimitInfo{}, errors.NewRateLimiterError(errors.ErrInvalidConfig.Code,
			"invalid config type for SlidingWindowLimiter",
			fmt.Errorf("invalid config type for SlidingWindowLimiter, got %T", cfg))
	}

	now := time.Now().UnixMilli()
	requestID := uuid.New().String()
	windowMs := slidingConfig.Window * 1000

	res, err := s.score.EvalSha(ctx,
		s.scriptSHA1,
		[]string{key}, []interface{}{windowMs, slidingConfig.Limit, now, requestID},
	)
	if err != nil {
		return ports.RateLimitInfo{}, err
	}
	result, ok := res.([]interface{})
	if !ok || len(result) < 4 {
		return ports.RateLimitInfo{}, fmt.Errorf("unexpected lua script response")
	}

	allowed, _ := result[0].(int64)
	remaining, _ := result[2].(int64)
	resetTime, _ := result[3].(int64)

	return ports.RateLimitInfo{
		Allowed:   allowed == 1,
		Limit:     slidingConfig.Limit,
		Remaining: int(remaining),
		ResetTime: resetTime,
	}, nil
}
