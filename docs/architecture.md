# Aegis — Architecture

## What this is

A distributed rate limiter: Go service, gRPC API, Redis for shared state,
two interchangeable algorithms (token bucket, sliding window) implemented
as atomic Lua scripts so correctness holds under concurrent access from
multiple Aegis instances hitting the same Redis.

## Request flow

```
client --gRPC--> Handler.CheckLimit
                     |
                     v
                 tenant.Manager.Admit()      (tenant cap enforcement)
                     |
                     v
                 limiter.Factory.Get(algorithm)
                     |
                     v
        TokenBucket.Check() / SlidingWindow.Check()
                     |
                     v
        redis.Client.EvalShaOrLoad()  -->  Redis (EVALSHA, one round trip)
                     |
                     v
        metrics + tenant.Manager.RecordResult()
                     |
                     v
                 RateLimitResponse
```

Every check is **one Redis round trip** (one EVALSHA), because the whole
refill-and-consume (or count-and-add) operation runs server-side inside
the Lua script. That's what makes the atomicity claim ("zero race
conditions across concurrent nodes") true — there's no read, decide,
write sequence split across multiple round trips for two Aegis instances
to interleave into a race.

## Why Lua scripts, specifically

Redis executes a Lua script as a single atomic operation relative to all
other commands, including from other clients. Two Aegis pods calling
`CheckLimit` for the same key at the same instant will have their EVALSHA
calls serialized by Redis itself — one fully completes before the other
starts. That's the actual mechanism behind the "no race conditions" claim,
not something enforced by our Go code. `test/integration/*_test.go` has
tests that fire 500 concurrent requests at a bucket sized for exactly 100,
from 50 goroutines, and assert **exactly** 100 get through — that's the
race condition test, and it passes (see the repo's test run notes in
`README.md`).

## Token bucket (`internal/limiter/lua/token_bucket.lua`)

State per key: a Redis Hash with `tokens` and `ts` (last-touch timestamp,
ms). On each call: refill based on elapsed time × rate, cap at capacity,
consume if enough tokens are available. `now` is passed in from the Go
client rather than read via Redis `TIME`, so refill math is deterministic
and doesn't depend on clock behavior inside the Lua sandbox.

## Sliding window (`internal/limiter/lua/sliding_window.lua`)

State per key: a Redis Sorted Set, one member per accepted request, scored
by its timestamp. Each call evicts everything older than `now - window`,
counts what's left, and admits the new request only if it fits under the
limit. This is the standard "sliding window log" approach — precise, at
the cost of one entry per accepted request rather than a single counter
(the fixed-window-counter approximation trades precision for that).

## Memory optimization (`internal/limiter/lua/hash_operations.lua`)

Claim 3 targets a 40% memory reduction versus one Redis STRING per
identifier. The approach implemented: shard identifiers across N grouped
Hash keys (`ratelimit:{algo}:shard:{bucket}`), so many identifiers share
one key's fixed overhead instead of each paying it individually. Redis
keeps small hashes in a compact listpack encoding, so this is cheaper than
one dict entry + one expire-table entry per identifier.

**Real Redis 7.4+ `HEXPIRE` is not available in this reference build**
(tested against Redis 7.0.15). The script implements equivalent
behavior by packing an explicit expiry timestamp into each field's value
and treating stale fields as logically expired on read. If you deploy on
Redis 7.4+, swap in real `HEXPIRE`/`HTTL` and this script becomes
unnecessary — noted as a TODO, not done in this reference implementation.

Measured result in this environment (see `scripts/memory/`,
100,000 entries, 256 shards): **79.5% reduction** in `used_memory` delta,
well past the 40% target — see `performance-tuning.md` for the exact
numbers and how they were produced. Your own reduction will depend on
shard count and payload size; the script is written so you can re-run it
against your actual key/value shapes.

## Connection pool hardening (Claim 4)

- `internal/redis/pool.go`: pool configured per the spec (`PoolSize: 200`,
  `MinIdleConns: 50`, etc.), pool stats exposed via `PoolStats()`.
- `internal/redis/circuitbreaker.go`: a small circuit breaker — opens after
  N consecutive failures, fails fast for a cooldown window, then allows one
  trial call before fully closing. This is the actual fix for "connection
  pool contention": without it, a Redis outage means every goroutine keeps
  retrying against a dead pool, holding connections and queuing behind
  each other; the breaker turns that into a bounded, fast failure instead.
- `internal/redis/retry.go`: exponential backoff + jitter for transient
  errors, separate from the breaker (breaker decides *whether* to try at
  all; retry decides how to retry once you've decided to).
- `internal/tenant/manager.go`: caps tracked tenants so one tenant can't
  cause unbounded key/goroutine growth.

**Not load-tested to failure in this reference run** — the circuit breaker
and retry logic are implemented and unit-testable, but this repo does not
include a chaos test that actually kills Redis mid-load and measures pool
behavior. That's a good next addition; see `performance-tuning.md`.

## What's stubbed, not fully implemented

Being direct about scope, per the "defensibility over completeness" rule:

- **`GetConfig` / `UpdateConfig` RPCs** return a hardcoded default and
  `success: false` respectively. There's no persistent per-tenant config
  store wired up. The `RateLimitConfig` proto message and the factory's
  algorithm-selection logic are real; a config store behind them is not.
- **Redis Cluster support**: `config.RedisConfig` has a `cluster_mode`
  field but `NewClient` always builds a single-node `*redis.Client`, never
  `*redis.ClusterClient`. True cluster support needs hash-tag key design
  (`{tenant}:key` so all keys for one Lua script land on one slot) and a
  cluster-aware client — not implemented here.
- **OpenTelemetry distributed tracing**: not implemented. The gRPC
  interceptor chain has logging and metrics interceptors; no tracing
  interceptor or span propagation.
- **Batch/pipelining beyond EVALSHA**: MGET/MSET-style batching is not
  needed for the current design (one EVALSHA already does the whole
  operation atomically), so it's not implemented — this was a means to an
  end in the original spec, not an end in itself.

## Deployment

See `deployment-guide.md` for Docker Compose and Kubernetes instructions.
