package redis

import (
	"context"
	"time"

	"aegis/internal/metrics"
)

// RunPoolStatsCollector periodically samples the connection pool stats
// (hits, misses, timeouts, total/idle/stale conns) into Prometheus gauges,
// so pool contention shows up in Grafana/alerts rather than only being
// visible via ad-hoc pprof runs. Lives in this package (not internal/metrics)
// because internal/redis already needs to import internal/metrics for
// per-command latency recording, and metrics importing redis back would
// create an import cycle.
func RunPoolStatsCollector(ctx context.Context, client *Client, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := client.PoolStats()
			if stats == nil {
				continue
			}
			metrics.ConnectionPoolStats.WithLabelValues("hits").Set(float64(stats.Hits))
			metrics.ConnectionPoolStats.WithLabelValues("misses").Set(float64(stats.Misses))
			metrics.ConnectionPoolStats.WithLabelValues("timeouts").Set(float64(stats.Timeouts))
			metrics.ConnectionPoolStats.WithLabelValues("total_conns").Set(float64(stats.TotalConns))
			metrics.ConnectionPoolStats.WithLabelValues("idle_conns").Set(float64(stats.IdleConns))
			metrics.ConnectionPoolStats.WithLabelValues("stale_conns").Set(float64(stats.StaleConns))
		}
	}
}
