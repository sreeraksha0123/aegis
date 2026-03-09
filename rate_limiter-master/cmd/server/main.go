package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SilentPlaces/rate_limiter/cmd/server/dep"
	"github.com/SilentPlaces/rate_limiter/config"
	"github.com/SilentPlaces/rate_limiter/internal/application/ports"
	"github.com/SilentPlaces/rate_limiter/internal/infrastructure/logger"
)

const configFilePath = "config/config.yml"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create logger
	log := logger.NewZeroLogger()

	// Load config
	cfg, err := config.LoadConfig(configFilePath, log)
	if err != nil {
		log.Error("failed to load config", ports.Field{Key: "err", Val: err.Error()})
		return
	}

	sigs := setupSignalHandler()

	// DI Container initialization
	c, err := dep.New(ctx, log, cfg)
	if err != nil {
		log.Error("failed to initialize dependencies", ports.Field{Key: "err", Val: err})
		return
	}
	defer c.Close()

	addr := fmt.Sprintf("%s:%d", c.Config.Server.Address, c.Config.Server.Port)
	server := &http.Server{Addr: addr, Handler: c.HTTPHandler}
	errCh := make(chan error, 1)

	go func() {
		log.Info("HTTP server starting on " + addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-sigs:
		log.Info("Received shutdown signal")
		cancel()
	case err := <-errCh:
		log.Error("HTTP server failed", ports.Field{Key: "err", Val: err.Error()})
		cancel()
		return
	case <-ctx.Done():
		log.Info("Context cancelled")
	}

	shutdownServer(server, log, time.Duration(c.Config.Server.ShutdownTimeoutSeconds)*time.Second)
}

func setupSignalHandler() chan os.Signal {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	return sigs
}

func shutdownServer(server *http.Server, log ports.Logger, timeout time.Duration) {
	log.Info("Shutting down server gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), timeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Server shutdown failed", ports.Field{Key: "err", Val: err.Error()})
	} else {
		log.Info("Server shutdown completed successfully")
	}
}
