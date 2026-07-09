package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	RequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aegis_requests_total",
		Help: "Total rate-limit check requests, labeled by algorithm, tenant, and outcome.",
	}, []string{"algorithm", "tenant", "outcome"})

	RequestLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "aegis_request_duration_seconds",
		Help: "Latency of rate-limit check requests (includes Redis round trip).",
		// Buckets tuned for sub-millisecond to a few ms, since that's the
		// claimed operating range (p99 < 2ms).
		Buckets: []float64{0.0001, 0.00025, 0.0005, 0.001, 0.0015, 0.002, 0.003, 0.005, 0.01, 0.05, 0.1},
	}, []string{"algorithm"})

	RedisCommandDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "aegis_redis_command_duration_seconds",
		Help:    "Latency of individual Redis commands issued by Aegis.",
		Buckets: prometheus.DefBuckets,
	}, []string{"command"})

	ConnectionPoolStats = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aegis_redis_pool",
		Help: "Live Redis connection pool stats (hits, misses, timeouts, total/idle/stale conns).",
	}, []string{"stat"})

	RateLimitHitRatio = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aegis_rate_limit_hit_ratio",
		Help: "Ratio of allowed to total requests per tenant.",
	}, []string{"tenant"})

	CircuitBreakerState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "aegis_circuit_breaker_state",
		Help: "Circuit breaker state: 0=closed, 1=half-open, 2=open.",
	}, []string{"target"})
)

func MustRegister() {
	prometheus.MustRegister(
		RequestsTotal,
		RequestLatency,
		RedisCommandDuration,
		ConnectionPoolStats,
		RateLimitHitRatio,
		CircuitBreakerState,
	)
}
