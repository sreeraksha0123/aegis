# Deployment Guide

## Local (no Docker)

```bash
go mod tidy          # needs normal internet access — see note below
redis-server &
go run ./cmd/aegis -config config.yaml
```

Health check: `curl localhost:9090/healthz`
Metrics: `curl localhost:9090/metrics`

### A note on `go mod tidy` / dependency fetching

This project was built and fully tested (build + integration tests +
benchmarks) inside a sandboxed environment with restricted network
egress — it could reach GitHub directly but not `proxy.golang.org`,
`golang.org`, or `gopkg.in`, which the normal Go module resolution flow
needs. To get around that *for testing purposes only*, local git clones
and `replace` directives pointing at local filesystem paths were used
temporarily. **The `go.mod` shipped in this zip is the clean version**
— standard module paths and versions, no local-path replaces — because
your machine has normal internet access and `go mod tidy && go build ./...`
should resolve everything the ordinary way. If you hit a network-egress
restriction of your own (corporate proxy, firewalled CI runner), the same
GitHub-mirror-and-replace trick that was used here will work for you too.

## Docker Compose

```bash
cd deployments/docker
docker compose up --build
```

Brings up: Aegis (`:50051` gRPC, `:9090` metrics), Redis (`:6379`),
Prometheus (`:9091`), Grafana (`:3000`, admin/admin — change this before
using anywhere but local dev).

Point Grafana at Prometheus (`http://prometheus:9090` from inside the
compose network) and build dashboards off `aegis_requests_total`,
`aegis_request_duration_seconds`, `aegis_redis_pool`,
`aegis_redis_command_duration_seconds`. No pre-built Grafana dashboard
JSON is included — build one against your actual traffic shape rather
than importing a generic one that may not match your label cardinality.

## Kubernetes

```bash
kubectl apply -f deployments/kubernetes/redis/redis-deployment.yaml
kubectl apply -f deployments/kubernetes/configmap.yaml
kubectl apply -f deployments/kubernetes/deployment.yaml
kubectl apply -f deployments/kubernetes/service.yaml
kubectl apply -f deployments/kubernetes/hpa.yaml
```

Notes:
- `deployment.yaml`'s `readinessProbe.grpc` needs Kubernetes 1.24+
  (feature graduated to stable in 1.27). On older clusters, swap it for
  an `exec` probe calling the `HealthCheck` RPC via `grpcurl`, or expose
  a plain HTTP readiness endpoint alongside `/healthz`.
- `redis-deployment.yaml` is a **single Redis instance**, not a cluster —
  fine for a demo/reference deployment, not for production HA. See
  `architecture.md`'s "What's stubbed" section: real Redis Cluster support
  (cluster-aware client + hash-tag key design) isn't implemented in this
  codebase yet.
- Build and push your own image before deploying:
  `docker build -f deployments/docker/Dockerfile -t <your-registry>/aegis:latest .`
  then update `image:` in `deployment.yaml`.

## Configuration reference

See `config.yaml` at the repo root for the full set of tunables (matches
`internal/config/config.go`). Override the path with `-config <path>`.
