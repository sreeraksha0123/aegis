# API Reference

Full definition: `api/proto/ratelimit.proto`. Generated Go stubs live in
`api/proto/ratelimit.pb.go` and `ratelimit_grpc.pb.go` (regenerate with
`make proto` if you change the `.proto` file).

## RateLimiter service

### `CheckLimit(RateLimitRequest) returns (RateLimitResponse)`

The main call. One round trip, one Redis EVALSHA.

Request:
| field | type | meaning |
|---|---|---|
| `key` | string | identifier: user ID, IP, API path, etc. Required. |
| `algorithm` | string | `"token_bucket"` or `"sliding_window"`. Empty = server default (`config.yaml`'s `rate_limiter.default_algorithm`). |
| `tenant` | string | tenant namespace, used for key isolation and per-tenant stats. Empty = `""` tenant (still isolated, just an implicit one). |
| `requests` | int64 | units to consume; `<= 0` is treated as `1`. |

Response:
| field | type | meaning |
|---|---|---|
| `allowed` | bool | whether this call was let through |
| `remaining` | int64 | tokens left (token bucket) or slots left in window (sliding window) |
| `reset_time` | int64 | unix ms — when the bucket/window state changes next |
| `limit` | int64 | the configured capacity/limit, echoed back |
| `retry_after` | int64 | ms to wait before retrying, only meaningful when `allowed=false` |

### `CheckLimitBatch(stream RateLimitRequest) returns (stream RateLimitResponse)`

Bidirectional streaming wrapper around `CheckLimit` for clients that want
to avoid one-RPC-per-check overhead over a long-lived stream. Each request
on the stream gets exactly one response, in order.

### `HealthCheck(HealthCheckRequest) returns (HealthCheckResponse)`

Pings Redis and reports `{healthy, redis_connected}`. Also exposed as
plain HTTP at `GET /healthz` on the metrics port for load balancer /
Kubernetes liveness probes that don't want to speak gRPC.

### `GetMetrics(GetMetricsRequest) returns (GetMetricsResponse)`

Per-tenant counters: total/allowed/denied requests and hit ratio, tracked
in-process (`internal/tenant/manager.go`). Resets on restart — this is a
live-process view, not a durable metrics store; use Prometheus
(`/metrics`) for anything you need to persist or alert on.

### `GetConfig` / `UpdateConfig`

**Stubbed, not fully implemented.** `GetConfig` always returns a
hardcoded `{algorithm: "token_bucket"}`; `UpdateConfig` always returns
`{success: false}`. The proto messages and factory's algorithm-dispatch
are real and working; a persistent per-tenant config store behind these
RPCs is not built. See `architecture.md`.

## Error codes

`pkg/errors/errors.go` maps internal errors to gRPC status codes:

| condition | gRPC code |
|---|---|
| unknown algorithm, missing `key` | `InvalidArgument` |
| tenant cap reached | `ResourceExhausted` |
| Redis unreachable / circuit open | `Unavailable` |
| anything else | `Internal` |

Clients should treat `Unavailable` as retryable (with backoff) and
`InvalidArgument` / `ResourceExhausted` as not.

## Metrics (Prometheus, `:9090/metrics`)

| metric | type | labels |
|---|---|---|
| `aegis_requests_total` | counter | `algorithm`, `tenant`, `outcome` |
| `aegis_request_duration_seconds` | histogram | `algorithm` (gRPC method via interceptor) |
| `aegis_redis_command_duration_seconds` | histogram | `command` |
| `aegis_redis_pool` | gauge | `stat` (hits/misses/timeouts/total_conns/idle_conns/stale_conns) |
| `aegis_rate_limit_hit_ratio` | gauge | `tenant` |
| `aegis_circuit_breaker_state` | gauge | `target` |
