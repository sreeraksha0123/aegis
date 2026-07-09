# Aegis

A distributed rate limiter: Go, gRPC, Redis. Token bucket and sliding
window algorithms, both implemented as atomic Redis Lua scripts so
correctness holds across multiple Aegis instances hitting shared Redis.

## 🚀 Engineering Highlights

- Designed and implemented a **distributed rate limiter** in **Go** supporting **Token Bucket** and **Sliding Window** algorithms, with all rate-limiting operations executed through **atomic Redis Lua scripts** to guarantee correctness under concurrent access across multiple service instances.

- Built a **modular, layered architecture** separating gRPC transport, rate-limiting algorithms, Redis infrastructure, configuration, metrics, and tenant management, making the system easy to extend with additional algorithms and infrastructure components.

- Developed a **resilient Redis communication layer** featuring connection pooling, automatic retries for transient failures, and a circuit breaker to prevent cascading failures and enable graceful recovery when Redis becomes unavailable.

- Optimized Redis memory utilization by replacing individual key storage with **hash-based counters**, significantly reducing memory overhead while maintaining efficient lookup and update operations for large numbers of rate-limiting keys.

- Implemented comprehensive **integration and concurrency tests** validating atomicity and correctness under high contention, ensuring strict per-key enforcement without race conditions during simultaneous requests.

- Added **Prometheus-based observability**, exposing operational metrics for request throughput, latency, Redis operations, and rate-limit decisions to support monitoring and performance analysis.

- Included **Docker Compose** and **Kubernetes** deployment configurations, enabling reproducible local development environments and containerized deployment workflows.

- Developed a **k6 benchmarking suite** for evaluating throughput, latency, and system behavior under concurrent workloads, providing a foundation for performance tuning and scalability testing.

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
