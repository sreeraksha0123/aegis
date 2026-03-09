package service

import (
	"context"
	"sync/atomic"
	"time"

	appConfig "github.com/SilentPlaces/rate_limiter/config"
	"github.com/SilentPlaces/rate_limiter/internal/application/ports"
	"github.com/SilentPlaces/rate_limiter/internal/domain/config"
)

type ConfigService struct {
	provider  ports.ConfigProvider
	logger    ports.Logger
	config    atomic.Value // config.Config
	appConfig appConfig.LimiterAppConfig
}

func NewConfigService(
	provider ports.ConfigProvider,
	logger ports.Logger,
	cfg appConfig.LimiterAppConfig,
) *ConfigService {
	cs := &ConfigService{
		provider:  provider,
		logger:    logger,
		appConfig: cfg,
	}
	cs.config.Store(config.Config{Routes: make(map[string]config.RouteConfig)})
	return cs
}

func (c *ConfigService) LoadOnce(ctx context.Context, key string) error {
	cfg, err := c.provider.GetConfig(ctx, key)
	if err != nil {
		c.logger.Error("ConfigService: LoadOnce: Failed to get config from provider", ports.Field{Key: "err", Val: err})
		return err
	}

	c.config.Store(cfg)
	return nil
}

func (c *ConfigService) WatchConfig(ctx context.Context, key string) {
	fetchInterval := time.Duration(c.appConfig.FetchConfigPeriodSeconds) * time.Second

	go c.provider.WatchConfig(
		ctx,
		key,
		uint(fetchInterval),
		c.handleConfigUpdate,
		c.handleConfigError,
	)
}

func (c *ConfigService) handleConfigUpdate(cfg config.Config) {
	c.config.Store(cfg)
	c.logger.Info("ConfigService: handleConfigUpdate: Config updated via provider")
}

func (c *ConfigService) handleConfigError(err error) {
	if err != nil {
		c.logger.Error("ConfigService: handleConfigError: Watch config error", ports.Field{Key: "err", Val: err})
	}
}

func (c *ConfigService) GetConfig() config.Config {
	cfg := c.config.Load()
	if cfg == nil {
		c.logger.Error("ConfigService: GetConfig: config is nil - system misconfigured")
		panic("rate limiter configuration not initialized")
	}

	domainCfg, ok := cfg.(config.Config)
	if !ok {
		c.logger.Error("ConfigService: GetConfig: Invalid config type stored",
			ports.Field{Key: "type", Val: cfg})
		panic("invalid config type stored in ConfigService")
	}

	return domainCfg
}
