package interceptor

import (
	"context"
	"time"

	"aegis/internal/metrics"

	"google.golang.org/grpc"
)

// UnaryMetrics records request latency per method into the Prometheus
// histogram used to derive p50/p90/p99 in Grafana.
func UnaryMetrics() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		metrics.RequestLatency.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
		return resp, err
	}
}
