package ports

import (
	"context"

	"github.com/SilentPlaces/rate_limiter/internal/domain/config"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string, cfg config.AlgorithmConfig) (RateLimitInfo, error)
}
