package redis

import (
	"context"
	"time"

	"github.com/SilentPlaces/rate_limiter/internal/application/ports"
	"github.com/SilentPlaces/rate_limiter/internal/infrastructure/resilience"
)

type ResilientRedisAdapter struct {
	adapter        ports.LimiterScore
	circuitBreaker *resilience.CircuitBreaker
	logger         ports.Logger
}

func NewResilientRedisAdapter(adapter ports.LimiterScore, logger ports.Logger, maxFailures int, timeout time.Duration) ports.LimiterScore {
	return &ResilientRedisAdapter{
		adapter:        adapter,
		circuitBreaker: resilience.NewCircuitBreaker(maxFailures, timeout),
		logger:         logger,
	}
}

func (r *ResilientRedisAdapter) Get(ctx context.Context, key string) interface{} {
	result, err := r.circuitBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
		val := r.adapter.Get(ctx, key)
		if err, ok := val.(error); ok {
			return nil, err
		}
		return val, nil
	})

	if err != nil {
		r.logger.Error("ResilientRedisAdapter: Get failed", ports.Field{Key: "error", Val: err})
		return err
	}
	return result
}

func (r *ResilientRedisAdapter) Set(ctx context.Context, key string, value interface{}, ttlSeconds int) error {
	_, err := r.circuitBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
		return nil, r.adapter.Set(ctx, key, value, ttlSeconds)
	})
	return err
}

func (r *ResilientRedisAdapter) Incr(ctx context.Context, key string) error {
	_, err := r.circuitBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
		return nil, r.adapter.Incr(ctx, key)
	})
	return err
}

func (r *ResilientRedisAdapter) Eval(ctx context.Context, script string, keys []string, args ...[]interface{}) (interface{}, error) {
	result, err := r.circuitBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
		return r.adapter.Eval(ctx, script, keys, args...)
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *ResilientRedisAdapter) ScriptLoad(ctx context.Context, script string) (string, error) {
	result, err := r.circuitBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
		return r.adapter.ScriptLoad(ctx, script)
	})

	if err != nil {
		return "", err
	}
	return result.(string), nil
}

func (r *ResilientRedisAdapter) EvalSha(ctx context.Context, sha1 string, keys []string, args ...[]interface{}) (interface{}, error) {
	result, err := r.circuitBreaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
		return r.adapter.EvalSha(ctx, sha1, keys, args...)
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}
