package config

import (
	"fmt"

	"github.com/SilentPlaces/rate_limiter/internal/domain/errors"
)

// Algorithm type constants
const (
	AlgorithmFixedWindow   = "fixed_window"
	AlgorithmTokenBucket   = "token_bucket"
	AlgorithmSlidingWindow = "sliding_window"
)

type Config struct {
	Routes map[string]RouteConfig
}

type RouteConfig struct {
	Algorithm string
	Config    AlgorithmConfig
}

type AlgorithmConfig interface {
	AlgorithmName() string
	Validate() error
}

type FixedWindowConfig struct {
	Limit  int
	Window int
}

func (f FixedWindowConfig) AlgorithmName() string {
	return AlgorithmFixedWindow
}

func (f FixedWindowConfig) Validate() error {
	if f.Limit <= 0 {
		return errors.NewRateLimiterError(errors.ErrInvalidConfig.Code,
			"limit must be positive",
			fmt.Errorf("limit must be positive, got %d", f.Limit))
	}
	if f.Window <= 0 {
		return errors.NewRateLimiterError(errors.ErrInvalidConfig.Code,
			"window must be positive",
			fmt.Errorf("window must be positive, got %d", f.Window))
	}
	if f.Window > 86400 {
		return errors.NewRateLimiterError(errors.ErrInvalidConfig.Code,
			"window too large",
			fmt.Errorf("window too large: %d seconds (max 24 hours)", f.Window))
	}
	return nil
}

type TokenBucketConfig struct {
	Capacity   int
	RefillRate int
	BucketTTL  int
}

func (t TokenBucketConfig) AlgorithmName() string {
	return AlgorithmTokenBucket
}

func (t TokenBucketConfig) Validate() error {
	if t.Capacity <= 0 {
		return errors.NewRateLimiterError(errors.ErrInvalidConfig.Code,
			"capacity must be positive",
			fmt.Errorf("capacity must be positive, got %d", t.Capacity))
	}
	if t.RefillRate <= 0 {
		return errors.NewRateLimiterError(errors.ErrInvalidConfig.Code,
			"refill_rate must be positive",
			fmt.Errorf("refill_rate must be positive, got %d", t.RefillRate))
	}
	if t.BucketTTL <= 0 {
		return errors.NewRateLimiterError(errors.ErrInvalidConfig.Code,
			"bucket_ttl must be positive",
			fmt.Errorf("bucket_ttl must be positive, got %d", t.BucketTTL))
	}
	if t.BucketTTL > 86400 {
		return errors.NewRateLimiterError(errors.ErrInvalidConfig.Code,
			"bucket_ttl too large",
			fmt.Errorf("bucket_ttl too large: %d seconds (max 24 hours)", t.BucketTTL))
	}
	return nil
}

type SlidingWindowConfig struct {
	Limit  int
	Window int
}

func (s SlidingWindowConfig) AlgorithmName() string {
	return AlgorithmSlidingWindow
}

func (s SlidingWindowConfig) Validate() error {
	if s.Limit <= 0 {
		return errors.NewRateLimiterError(errors.ErrInvalidConfig.Code,
			"limit must be positive",
			fmt.Errorf("limit must be positive, got %d", s.Limit))
	}
	if s.Window <= 0 {
		return errors.NewRateLimiterError(errors.ErrInvalidConfig.Code,
			"window must be positive",
			fmt.Errorf("window must be positive, got %d", s.Window))
	}
	if s.Window > 86400 {
		return errors.NewRateLimiterError(errors.ErrInvalidConfig.Code,
			"window too large",
			fmt.Errorf("window too large: %d seconds (max 24 hours)", s.Window))
	}
	return nil
}
