package limiter

import (
	"context"
	_ "embed"
)

//go:embed lua/token_bucket.lua
var TokenBucketScript string

//go:embed lua/sliding_window.lua
var SlidingWindowScript string

//go:embed lua/hash_operations.lua
var HashOperationsScript string

// Result is the outcome of a single rate-limit check, algorithm-agnostic.
type Result struct {
	Allowed    bool
	Remaining  int64
	ResetTime  int64 // unix ms
	Limit      int64
	RetryAfter int64 // ms; 0 if allowed
}

// Limiter is implemented by TokenBucket and SlidingWindow so the gRPC
// handler and factory can treat both algorithms uniformly.
type Limiter interface {
	// Check evaluates whether `requested` units are allowed for `key`
	// under `tenant`, consuming them atomically if allowed.
	Check(ctx context.Context, tenant, key string, requested int64) (Result, error)
	Name() string
}
