# Performance Tuning & Benchmark Results

## Honesty note on the numbers below

Everything in this document was **actually run** in the sandbox this
project was built in — a single-vCPU container running Redis, the Aegis
server, and the load generator all on that one core simultaneously. That
setup cannot demonstrate 12K req/s at sub-2ms p99; it doesn't have the
cores to. The numbers below are what was genuinely measured there, not
numbers edited to match the claim. Re-run `scripts/benchmark/mixed_workload.js`
against your actual target deployment (server on its own multi-core
host/pod, load generator on a separate machine, real network hop) to get
numbers that mean something for the 12K/p99<2ms claim.

## Correctness (this you can trust as-is)

- `go test ./test/integration/... -v` — 6/6 tests pass, including:
  - `TestTokenBucket_NoRaceUnderConcurrency`: 50 goroutines x 10 requests
    each (500 total) against a 100-token bucket -> **exactly 100** allowed.
  - `TestSlidingWindow_NoRaceUnderConcurrency`: same shape, sliding window
    -> **exactly 100** allowed.
  These are the actual "zero race conditions" test — if the Lua atomicity
  didn't hold, these counts would be flaky and >100.

## Memory reduction (Claim 3) — actually measured

Ran `scripts/memory/compare_memory_usage.go` against local Redis 7.0.15,
100,000 entries each approach, 256 shards for the grouped-hash approach:

```
baseline (string keys)         before=1416216 after=16912568 delta=15496352 bytes
optimized (grouped hash)       before=1465800 after=4637992  delta=3172192  bytes
Measured reduction: 79.5%
```

That's a real INFO memory delta before/after, not computed from a formula
— comfortably past the 40% target. Re-run this against your production
key/value shapes; the reduction depends on shard count and payload size
(bigger payloads -> per-key overhead matters less -> smaller % reduction).

## Throughput/latency (Claim 2) — measured, with honest caveats

Ran `k6 run scripts/benchmark/mixed_workload.js` with a k6-native gRPC
client (not HTTP — the original spec's k6 example posted JSON over HTTP
to the gRPC port, which doesn't work against a real gRPC server; this was
corrected to use `k6/net/grpc`) targeting the local server:

```
K6_RATE=2000 K6_DURATION=15s K6_VUS=200 K6_MAX_VUS=400
```

Result: **100% of requests succeeded** (status OK, correct allow/deny
decisions), but **p99 latency was ~102ms**, not <2ms — because this one
CPU core is time-slicing between Redis, the Go server, and the k6 process
itself. That's expected contention, not a defect in the implementation.
It tells you the logic and wiring work end-to-end; it does not tell you
whether the 12K req/s @ <2ms p99 target is met, because this hardware
can't test that claim.

**To actually validate Claim 2**, on real infrastructure:
1. Run `aegis` on its own multi-core host with Redis reachable over a fast
   local network (same AZ/rack), Redis on its own cores too.
2. Run k6 from a *separate* machine so the load generator isn't competing
   with the server for CPU.
3. `K6_RATE=12000 K6_DURATION=5m K6_VUS=1000 K6_MAX_VUS=2000`.
4. Check `p(99)` in the k6 summary output directly.

### Additional runs (isolated algorithms, same sandbox, same caveat)

`token_bucket_test.js`, 500 req/s target, 5s: **3296 req/s actual**
throughput (k6 ran ahead of the nominal target since the server kept up),
100% success, p90=23.9ms/p95=28.2ms (again, single-core contention, not a
correctness issue).

`sliding_window_test.js`, same parameters: **3346 req/s actual**, 100%
success, p90=23.1ms/p95=26.1ms — sliding window's ZADD/ZREMRANGEBYSCORE
per call costs about the same as token bucket's HSET/HMGET per call, as
expected for two O(log N)-ish Redis operations.

## Single-connection round-trip cost (`go test -bench`)

`test/benchmark/performance_test.go` isolates pure Redis round-trip cost
per `Check()` call, one connection, no concurrency (unlike the k6 tests,
which exercise the pool under concurrent load). Actually run here:

```
BenchmarkTokenBucket_Check      58903    42264 ns/op   (~42.3µs/call)
BenchmarkSlidingWindow_Check    46939    51302 ns/op   (~51.3µs/call)
```

Sliding window costs more per call than token bucket, consistent with the
scripts: token bucket does one HMGET + one HSET, sliding window does
ZREMRANGEBYSCORE + ZCARD + (conditionally) ZADD + EXPIRE + ZRANGE — more
Redis-side work per EVALSHA. Both are well under 1ms per call in isolation;
the ~14-100ms latencies seen in the k6 runs above come from concurrent
contention on this sandbox's single core, not from the scripts themselves.

## Connection pool / circuit breaker (Claim 4)

Implemented (`internal/redis/circuitbreaker.go`, `retry.go`) and covered by
the pool config in `config.yaml` matching the spec's numbers
(`PoolSize: 200`, `MinIdleConns: 50`, `MaxConnAge: 30m`, `IdleTimeout: 10m`).
**Not chaos-tested here** — there's no test that kills Redis mid-load and
confirms the breaker opens/recovers correctly. Recommended next step:
add a test that runs load, kills the Redis container, confirms requests
fail fast (not hang) during the outage, then confirms recovery once Redis
is back.

## Profiling

`net/http/pprof` is not yet wired into `cmd/aegis/main.go` — the spec asks
for pprof-based profiling but this reference implementation only ships
Prometheus metrics (latency histograms, pool stats). Adding
`import _ "net/http/pprof"` and mounting it on the metrics HTTP server is
a five-minute addition if you need CPU/heap profiles; it wasn't done here
to avoid claiming a profiling workflow that wasn't actually exercised.
