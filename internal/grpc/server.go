package grpc

import (
	"fmt"
	"log/slog"
	"net"

	pb "aegis/api/proto"
	"aegis/internal/config"
	"aegis/internal/grpc/interceptor"
	"aegis/internal/limiter"
	aredis "aegis/internal/redis"
	"aegis/internal/tenant"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type Server struct {
	grpcServer *grpc.Server
	handler    *Handler
	port       int
}

func NewServer(cfg *config.Config, client *aredis.Client, factory *limiter.Factory, tm *tenant.Manager, logger *slog.Logger) *Server {
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			interceptor.UnaryLogging(logger),
			interceptor.UnaryMetrics(),
		),
		grpc.MaxConcurrentStreams(uint32(cfg.Server.MaxConnections)),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: cfg.Server.Timeout * 6,
		}),
	}

	gs := grpc.NewServer(opts...)
	h := NewHandler(client, factory, tm, logger)
	pb.RegisterRateLimiterServer(gs, h)

	return &Server{grpcServer: gs, handler: h, port: cfg.Server.Port}
}

func (s *Server) ListenAndServe() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("grpc: failed to listen: %w", err)
	}
	return s.grpcServer.Serve(lis)
}

func (s *Server) GracefulStop() {
	s.grpcServer.GracefulStop()
}
