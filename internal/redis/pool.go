package redis

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"aegis/internal/config"

	goredis "github.com/redis/go-redis/v9"
)

// Client wraps a go-redis client, exposing pool stats and precomputed
// script SHAs so hot-path calls can use EVALSHA instead of EVAL.
type Client struct {
	rdb        *goredis.Client
	scriptSHAs map[string]string
	mu         sync.RWMutex
	breaker    *CircuitBreaker
}

func NewClient(cfg *config.RedisConfig) (*Client, error) {
	if len(cfg.Addresses) == 0 {
		return nil, fmt.Errorf("redis: no addresses configured")
	}

	rdb := goredis.NewClient(&goredis.Options{
		Addr:            cfg.Addresses[0],
		Password:        cfg.Password,
		DB:              cfg.DB,
		PoolSize:        cfg.PoolSize,
		MinIdleConns:    cfg.MinIdleConns,
		ConnMaxLifetime: cfg.MaxConnAge,
		ConnMaxIdleTime: cfg.IdleTimeout,
		DialTimeout:     cfg.DialTimeout,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping failed: %w", err)
	}

	return &Client{
		rdb:        rdb,
		scriptSHAs: make(map[string]string),
		breaker:    NewCircuitBreaker(5, 5*time.Second), // open after 5 consecutive failures, 5s cooldown
	}, nil
}

func (c *Client) Raw() *goredis.Client { return c.rdb }

// LoadScript loads a Lua script via SCRIPT LOAD and caches its SHA so
// subsequent calls can use EVALSHA. Falls back transparently to EVAL
// (via go-redis's Script.Run, which retries with EVAL on NOSCRIPT) if the
// script has been flushed from the server's cache.
func (c *Client) LoadScript(ctx context.Context, name, source string) (string, error) {
	sum := sha1.Sum([]byte(source))
	sha := hex.EncodeToString(sum[:])

	loaded, err := c.rdb.ScriptExists(ctx, sha).Result()
	if err != nil {
		return "", err
	}
	if len(loaded) == 0 || !loaded[0] {
		got, err := c.rdb.ScriptLoad(ctx, source).Result()
		if err != nil {
			return "", err
		}
		sha = got
	}

	c.mu.Lock()
	c.scriptSHAs[name] = sha
	c.mu.Unlock()
	return sha, nil
}

func (c *Client) ScriptSHA(name string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	sha, ok := c.scriptSHAs[name]
	return sha, ok
}

// PoolStats exposes go-redis's live pool stats (hits, misses, timeouts,
// total/idle/stale conns) for the Prometheus collector.
func (c *Client) PoolStats() *goredis.PoolStats {
	return c.rdb.PoolStats()
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func (c *Client) HealthCheck(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return c.rdb.Ping(ctx).Err() == nil
}
