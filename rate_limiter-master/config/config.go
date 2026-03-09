package config

import (
	"strings"

	lgr "github.com/SilentPlaces/rate_limiter/internal/application/ports"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
)

var K = koanf.New(".")

type Config struct {
	Redis  RedisConfig      `koanf:"redis"`
	Consul ConsulConfig     `koanf:"consul"`
	Server ServerConfig     `koanf:"server"`
	App    LimiterAppConfig `koanf:"app"`
}

type RedisConfig struct {
	Addr                         string `koanf:"addr"`
	Port                         int    `koanf:"port"`
	DB                           int    `koanf:"db"`
	ConnectionTimeoutSeconds     int    `koanf:"connection_timeout_seconds"`
	OperationTimeoutSeconds      int    `koanf:"operation_timeout_seconds"`
	CircuitBreakerMaxFailures    int    `koanf:"circuit_breaker_max_failures"`
	CircuitBreakerTimeoutSeconds int    `koanf:"circuit_breaker_timeout_seconds"`
}

type ConsulConfig struct {
	Addr string `koanf:"addr"`
}

type ServerConfig struct {
	Port                   int    `koanf:"port"`
	Address                string `koanf:"address"`
	ShutdownTimeoutSeconds int    `koanf:"shutdown_timeout_seconds"`
}

type LimiterAppConfig struct {
	FetchConfigPeriodSeconds int      `koanf:"fetch_config_period_seconds"`
	ConfigKey                string   `koanf:"config_key"`
	BackendNginxAddr         string   `koanf:"backend_nginx_addr"`
	WhitelistedIPs           []string `koanf:"whitelisted_ips"`
}

func LoadConfig(path string, logger lgr.Logger) (*Config, error) {
	// load config YAML file
	if err := K.Load(file.Provider(path), yaml.Parser()); err != nil {
		logger.Error("Failed to load YAML, fallback to env only", lgr.Field{Key: "error", Val: err})
	}

	// override from environment variables
	if err := K.Load(env.Provider("", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(s), "_", ".")
	}), nil); err != nil {
		logger.Error("Failed to load env variables", lgr.Field{Key: "error", Val: err})
	}

	var cfg Config
	if err := K.Unmarshal("", &cfg); err != nil {
		return nil, err
	}

	logger.Info(
		"Configuration loaded",
		lgr.Field{Key: "redis", Val: cfg.Redis},
		lgr.Field{Key: "consul", Val: cfg.Consul},
		lgr.Field{Key: "server", Val: cfg.Server},
		lgr.Field{Key: "app", Val: cfg.App},
	)
	return &cfg, nil
}
