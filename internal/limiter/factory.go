package limiter

import (
	"fmt"

	"aegis/internal/config"
	aredis "aegis/internal/redis"
)

// Factory builds/holds one Limiter per algorithm and dispatches Check calls
// by name, so both algorithms can run simultaneously — selected either
// globally (server default) or per-request (client passes "algorithm").
type Factory struct {
	limiters map[string]Limiter
	def      string
}

func NewFactory(client *aredis.Client, cfg *config.RateLimiterConfig) *Factory {
	f := &Factory{limiters: make(map[string]Limiter), def: cfg.DefaultAlgorithm}

	f.limiters["token_bucket"] = NewTokenBucket(
		client, cfg.TokenBucket.DefaultCapacity, cfg.TokenBucket.DefaultRate, cfg.KeyTTLSeconds,
	)
	f.limiters["sliding_window"] = NewSlidingWindow(
		client, cfg.SlidingWindow.DefaultWindow, cfg.SlidingWindow.DefaultLimit, cfg.KeyTTLSeconds,
	)

	if cfg.KeyTTLSeconds == 0 {
		f.limiters["token_bucket"] = NewTokenBucket(client, cfg.TokenBucket.DefaultCapacity, cfg.TokenBucket.DefaultRate, 3600)
	}

	return f
}

func (f *Factory) Get(algorithm string) (Limiter, error) {
	if algorithm == "" {
		algorithm = f.def
	}
	l, ok := f.limiters[algorithm]
	if !ok {
		return nil, fmt.Errorf("unknown algorithm %q (want token_bucket or sliding_window)", algorithm)
	}
	return l, nil
}
