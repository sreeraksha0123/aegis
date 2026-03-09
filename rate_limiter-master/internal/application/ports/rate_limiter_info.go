package ports

import (
	"context"

	"github.com/SilentPlaces/rate_limiter/internal/domain/config"
)

type RateLimiterWithInfo interface {
	AllowWithInfo(ctx context.Context, key string, cfg config.AlgorithmConfig) (RateLimitInfo, error)
}

type RateLimitInfo struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetTime int64
}
