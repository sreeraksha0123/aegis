package redis

import (
	"context"
	"errors"
	"strings"
	"time"

	"aegis/internal/metrics"

	goredis "github.com/redis/go-redis/v9"
)

// EvalShaOrLoad runs a script by its cached SHA. If the server responds
// NOSCRIPT (e.g. after a FLUSHALL / restart wiped the script cache), it
// reloads the source and retries once. This keeps the hot path on EVALSHA
// (cheap: just a SHA lookup server-side) while staying correct if the
// cache is ever cold.
//
// The actual Redis call is guarded by a circuit breaker: after repeated
// failures (real connectivity/timeout errors, not logic errors like
// NOSCRIPT) it fails fast instead of letting every caller queue up on a
// dead connection pool — that's the mechanism behind Claim 4.
func (c *Client) EvalShaOrLoad(ctx context.Context, name, source string, keys []string, args ...interface{}) (interface{}, error) {
	sha, ok := c.ScriptSHA(name)
	if !ok {
		var err error
		sha, err = c.LoadScript(ctx, name, source)
		if err != nil {
			return nil, err
		}
	}

	var res interface{}
	breakerErr := c.breaker.Do(ctx, func(ctx context.Context) error {
		start := time.Now()
		var evalErr error
		res, evalErr = c.rdb.EvalSha(ctx, sha, keys, args...).Result()
		metrics.RedisCommandDuration.WithLabelValues("evalsha:" + name).Observe(time.Since(start).Seconds())

		if evalErr != nil && isNoScript(evalErr) {
			// Logic-level cache miss, not a Redis health problem: reload
			// and retry once, without counting it as a breaker failure.
			if _, loadErr := c.LoadScript(ctx, name, source); loadErr != nil {
				return loadErr
			}
			reloadedSHA, _ := c.ScriptSHA(name)
			start = time.Now()
			res, evalErr = c.rdb.EvalSha(ctx, reloadedSHA, keys, args...).Result()
			metrics.RedisCommandDuration.WithLabelValues("evalsha:" + name + ":retry").Observe(time.Since(start).Seconds())
		}
		return evalErr
	})
	metrics.CircuitBreakerState.WithLabelValues("redis").Set(c.breaker.StateValue())

	if breakerErr == ErrCircuitOpenLocal {
		return nil, ErrCircuitOpenLocal
	}
	return res, breakerErr
}

func isNoScript(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "NOSCRIPT")
}

// IsRedisNil reports whether err is go-redis's sentinel "no results" error,
// distinct from real failures, so callers can branch cleanly.
func IsRedisNil(err error) bool {
	return errors.Is(err, goredis.Nil)
}
