# Aegis

A distributed rate limiter: Go, gRPC, Redis. Token bucket and sliding
window algorithms, both implemented as atomic Redis Lua scripts so
correctness holds across multiple Aegis instances hitting shared Redis.

## Status — what's real vs. what's scoped out

This was built and genuinely tested end-to-end (not just written and
assumed to work):

| Claim | Status |
|---|---|
| Token bucket + sliding window, atomic Lua, per-key isolation | **Done.** Both algorithms implemented, race-condition tests pass (500 concurrent requests against a 100-capacity limit → exactly 100 allowed, both algorithms). |
| 12K+ req/s @ p99 < 2ms | **Not validated here.** Built and load-tested on a single-vCPU sandbox where server + Redis + load generator share one core — real numbers from that run are in `docs/performance-tuning.md`, but that hardware can't prove or disprove this claim. Re-run `scripts/benchmark/` on real multi-core infra with a separate load-gen host. |
| 40% Redis memory reduction via hash counters | **Done, measured at 79.5%** on 100K entries / 256 shards, real Redis `INFO memory` delta — see `docs/performance-tuning.md`. |
| Connection-pool contention / circuit breaker | **Done and unit-tested** (`internal/redis/circuitbreaker_test.go`): breaker opens after repeated failures, fails fast, recovers after cooldown. Not chaos-tested against a real live outage of the running server in this pass. |
| Redis Cluster support | **Not implemented.** Config has a placeholder flag; client is always single-node. |
| OpenTelemetry tracing | **Not implemented.** |
| Per-tenant persistent config store (`GetConfig`/`UpdateConfig`) | **Stubbed.** RPCs exist and compile; no backing store. |
| pprof profiling | **Not wired in.** Prometheus metrics are; pprof isn't. |

Full detail on all of the above: `docs/architecture.md`.

## Quick start

```bash
go mod tidy   # normal internet access needed — see docs/deployment-guide.md
redis-server &
go run ./cmd/aegis -config config.yaml
```

```bash
grpcurl -plaintext -d '{"key":"user_123","algorithm":"token_bucket","tenant":"t1","requests":1}' \
  localhost:50051 ratelimit.RateLimiter/CheckLimit
```

## Repo layout

```
cmd/aegis/            entrypoint
internal/limiter/     token bucket + sliding window + factory + lua scripts
internal/redis/       pool, circuit breaker, retry, EVALSHA wrapper
internal/grpc/        server, handlers, interceptors
internal/tenant/       tenant isolation + per-tenant stats
internal/metrics/      Prometheus metric definitions
internal/config/       config.yaml loader
api/proto/             ratelimit.proto + generated Go stubs
scripts/benchmark/     k6 load tests (native gRPC client)
scripts/memory/        real memory comparison tool
deployments/docker/    Dockerfile + docker-compose (Redis/Prometheus/Grafana)
deployments/kubernetes/ Deployment/Service/ConfigMap/HPA + reference Redis
test/integration/      real tests against Redis, including race-condition tests
docs/                   architecture, API reference, performance, deployment
```

## Testing

```bash
make test-integration   # needs redis-server running on localhost:6379
```

Ran here: 6/6 pass, plus 2/2 circuit breaker unit tests (`go test ./internal/redis/...`).

## Docs

- `docs/architecture.md` — how it works, and the direct explanation of why
  Lua scripts are what actually make the "no race conditions" claim true.
- `docs/performance-tuning.md` — real measured numbers, with the honest
  caveats about what this sandbox can and can't prove.
- `docs/deployment-guide.md` — Docker Compose and Kubernetes.
- `docs/api.md` — full RPC/metric reference.
