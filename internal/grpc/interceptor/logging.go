package interceptor

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
)

// UnaryLogging logs each unary RPC with method, duration, and error status
// using structured logging (slog), as required for production log parsing.
func UnaryLogging(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		dur := time.Since(start)

		attrs := []any{
			slog.String("method", info.FullMethod),
			slog.Duration("duration", dur),
		}
		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
			logger.Error("rpc completed with error", attrs...)
		} else {
			logger.Info("rpc completed", attrs...)
		}
		return resp, err
	}
}
