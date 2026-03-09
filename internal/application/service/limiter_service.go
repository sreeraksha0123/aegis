package service

import (
	"context"
	"fmt"

	"github.com/SilentPlaces/rate_limiter/internal/application/ports"
	"github.com/SilentPlaces/rate_limiter/internal/domain/errors"
	"github.com/SilentPlaces/rate_limiter/internal/domain/limiter"
)

const rateLimitKeyPrefix = "rl:%s:%s:%s"

type LimiterService struct {
	logger        ports.Logger
	configService ports.ConfigService
	limiters      map[string]ports.RateLimiter
	policy        *limiter.Policy
}

func NewLimiterService(
	logger ports.Logger,
	configService ports.ConfigService,
	limiters map[string]ports.RateLimiter,
	policy *limiter.Policy,
) *LimiterService {
	return &LimiterService{
		logger:        logger,
		configService: configService,
		limiters:      limiters,
		policy:        policy,
	}
}

func (l *LimiterService) AllowWithInfo(ctx context.Context, ip, route string) (ports.RateLimitInfo, error) {
	if l.policy.ShouldBypassRateLimit(ip) {
		l.logger.Info("LimiterService: Allow: IP whitelisted, bypassing rate limit",
			ports.Field{Key: "ip", Val: ip},
			ports.Field{Key: "route", Val: route})
		return ports.RateLimitInfo{Allowed: true, Limit: -1, Remaining: -1, ResetTime: 0}, nil
	}

	cfg := l.configService.GetConfig()

	routeConfig, ok := cfg.Routes[route]
	if !ok {
		l.logger.Info("LimiterService: Allow: route not found",
			ports.Field{Key: "route", Val: route},
			ports.Field{Key: "ip", Val: ip})
		return ports.RateLimitInfo{Allowed: true, Limit: -1, Remaining: -1, ResetTime: 0}, nil
	}

	limiter, ok := l.limiters[routeConfig.Algorithm]
	if !ok {
		l.logger.Error("LimiterService: Allow: limiter not found for algorithm",
			ports.Field{Key: "algorithm", Val: routeConfig.Algorithm},
			ports.Field{Key: "route", Val: route})
		return ports.RateLimitInfo{}, errors.NewRateLimiterError(
			"UNKNOWN_ALGORITHM",
			fmt.Sprintf("algorithm '%s' not found", routeConfig.Algorithm),
			nil,
		)
	}

	if err := routeConfig.Config.Validate(); err != nil {
		l.logger.Error("LimiterService: Allow: invalid configuration",
			ports.Field{Key: "algorithm", Val: routeConfig.Algorithm},
			ports.Field{Key: "route", Val: route},
			ports.Field{Key: "error", Val: err})
		return ports.RateLimitInfo{}, errors.NewRateLimiterError("INVALID_CONFIG", err.Error(), err)
	}

	key := l.buildRateLimitKey(routeConfig.Algorithm, route, ip)

	l.logger.Info("LimiterService: Allow: checking rate limit",
		ports.Field{Key: "key", Val: key},
		ports.Field{Key: "route", Val: route},
		ports.Field{Key: "ip", Val: ip},
		ports.Field{Key: "algorithm", Val: routeConfig.Algorithm})

	info, err := limiter.Allow(ctx, key, routeConfig.Config)
	if err != nil {
		return ports.RateLimitInfo{}, err
	}

	return info, nil
}

func (l *LimiterService) buildRateLimitKey(algorithm, route, ip string) string {
	return fmt.Sprintf(rateLimitKeyPrefix, algorithm, route, ip)
}
