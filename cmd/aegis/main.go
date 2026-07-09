package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aegis/internal/config"
	agrpc "aegis/internal/grpc"
	"aegis/internal/limiter"
	"aegis/internal/metrics"
	aredis "aegis/internal/redis"
	"aegis/internal/tenant"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config.yaml")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	redisClient, err := aredis.NewClient(&cfg.Redis)
	if err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	// Precompute script SHAs at startup so the first real request already
	// hits the EVALSHA fast path instead of paying an EVAL+cache-miss.
	ctx := context.Background()
	for name, src := range map[string]string{
		"token_bucket":    limiter.TokenBucketScript,
		"sliding_window":  limiter.SlidingWindowScript,
		"hash_operations": limiter.HashOperationsScript,
	} {
		if _, err := redisClient.LoadScript(ctx, name, src); err != nil {
			logger.Error("failed to preload script", "script", name, "error", err)
			os.Exit(1)
		}
	}

	factory := limiter.NewFactory(redisClient, &cfg.RateLimiter)
	tenantMgr := tenant.NewManager(cfg.Tenant.Isolation, cfg.Tenant.MaxTenants)

	metrics.MustRegister()

	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go aredis.RunPoolStatsCollector(appCtx, redisClient, 5*time.Second)

	if cfg.Metrics.Enabled {
		mux := http.NewServeMux()
		mux.Handle(cfg.Metrics.Path, promhttp.Handler())
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			if redisClient.HealthCheck(r.Context()) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
				return
			}
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("redis unavailable"))
		})
		go func() {
			addr := fmt.Sprintf(":%d", cfg.Metrics.Port)
			logger.Info("metrics server listening", "addr", addr, "path", cfg.Metrics.Path)
			if err := http.ListenAndServe(addr, mux); err != nil {
				logger.Error("metrics server stopped", "error", err)
			}
		}()
	}

	server := agrpc.NewServer(cfg, redisClient, factory, tenantMgr, logger)

	go func() {
		logger.Info("aegis grpc server listening", "port", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil {
			logger.Error("grpc server stopped", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown on SIGTERM/SIGINT: stop accepting new RPCs, let
	// in-flight ones finish, then close the Redis pool cleanly.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Info("shutdown signal received, draining")
	server.GracefulStop()
	logger.Info("shutdown complete")
}
