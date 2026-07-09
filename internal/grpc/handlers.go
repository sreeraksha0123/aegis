package grpc

import (
	"context"
	"log/slog"

	pb "aegis/api/proto"
	"aegis/internal/limiter"
	aerrors "aegis/pkg/errors"
	aredis "aegis/internal/redis"
	"aegis/internal/metrics"
	"aegis/internal/tenant"
)

type Handler struct {
	pb.UnimplementedRateLimiterServer
	client  *aredis.Client
	factory *limiter.Factory
	tenants *tenant.Manager
	logger  *slog.Logger
}

func NewHandler(client *aredis.Client, factory *limiter.Factory, tm *tenant.Manager, logger *slog.Logger) *Handler {
	return &Handler{client: client, factory: factory, tenants: tm, logger: logger}
}

func (h *Handler) CheckLimit(ctx context.Context, req *pb.RateLimitRequest) (*pb.RateLimitResponse, error) {
	if req.Key == "" {
		return nil, aerrors.ToGRPCStatus(aerrors.ErrInvalidRequest)
	}

	if err := h.tenants.Admit(req.Tenant); err != nil {
		return nil, aerrors.ToGRPCStatus(aerrors.ErrTenantLimit)
	}

	lim, err := h.factory.Get(req.Algorithm)
	if err != nil {
		return nil, aerrors.ToGRPCStatus(aerrors.ErrUnknownAlgorithm)
	}

	requested := req.Requests
	if requested <= 0 {
		requested = 1
	}

	result, err := lim.Check(ctx, req.Tenant, req.Key, requested)
	if err != nil {
		return nil, aerrors.ToGRPCStatus(aerrors.ErrRedisUnavailable)
	}

	outcome := "denied"
	if result.Allowed {
		outcome = "allowed"
	}
	metrics.RequestsTotal.WithLabelValues(lim.Name(), req.Tenant, outcome).Inc()
	h.tenants.RecordResult(req.Tenant, result.Allowed)

	return &pb.RateLimitResponse{
		Allowed:    result.Allowed,
		Remaining:  result.Remaining,
		ResetTime:  result.ResetTime,
		Limit:      result.Limit,
		RetryAfter: result.RetryAfter,
	}, nil
}

func (h *Handler) CheckLimitBatch(stream pb.RateLimiter_CheckLimitBatchServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err // io.EOF ends the stream cleanly for the client
		}
		resp, err := h.CheckLimit(stream.Context(), req)
		if err != nil {
			return err
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func (h *Handler) HealthCheck(ctx context.Context, _ *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	connected := h.client.HealthCheck(ctx)
	return &pb.HealthCheckResponse{Healthy: connected, RedisConnected: connected}, nil
}

func (h *Handler) GetMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	stats := h.tenants.GetStats(req.Tenant)
	var ratio float64
	if stats.TotalRequests > 0 {
		ratio = float64(stats.AllowedRequests) / float64(stats.TotalRequests)
	}
	return &pb.GetMetricsResponse{
		TotalRequests:   stats.TotalRequests,
		AllowedRequests: stats.AllowedRequests,
		DeniedRequests:  stats.DeniedRequests,
		HitRatio:        ratio,
	}, nil
}

// GetConfig / UpdateConfig are intentionally minimal: this reference
// implementation applies one global config (loaded from config.yaml) per
// algorithm rather than a full per-tenant override store. Wiring these to
// a Redis-backed or DB-backed config store is a natural next step — see
// docs/architecture.md.
func (h *Handler) GetConfig(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	return &pb.GetConfigResponse{Config: &pb.RateLimitConfig{Algorithm: "token_bucket"}}, nil
}

func (h *Handler) UpdateConfig(ctx context.Context, req *pb.UpdateConfigRequest) (*pb.UpdateConfigResponse, error) {
	return &pb.UpdateConfigResponse{Success: false}, nil
}
