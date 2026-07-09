package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Redis       RedisConfig       `yaml:"redis"`
	RateLimiter RateLimiterConfig `yaml:"rate_limiter"`
	Tenant      TenantConfig      `yaml:"tenant"`
	Metrics     MetricsConfig     `yaml:"metrics"`
}

type ServerConfig struct {
	Port           int           `yaml:"port"`
	MaxConnections int           `yaml:"max_connections"`
	Timeout        time.Duration `yaml:"timeout"`
}

type RedisConfig struct {
	Addresses    []string      `yaml:"addresses"`
	Password     string        `yaml:"password"`
	DB           int           `yaml:"db"`
	PoolSize     int           `yaml:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns"`
	MaxConnAge   time.Duration `yaml:"max_conn_age"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
	DialTimeout  time.Duration `yaml:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	ClusterMode  bool          `yaml:"cluster_mode"`
}

type TokenBucketConfig struct {
	DefaultRate     int64 `yaml:"default_rate"`
	DefaultCapacity int64 `yaml:"default_capacity"`
}

type SlidingWindowConfig struct {
	DefaultWindow time.Duration `yaml:"default_window"`
	DefaultLimit  int64         `yaml:"default_limit"`
}

type RateLimiterConfig struct {
	DefaultAlgorithm string              `yaml:"default_algorithm"`
	TokenBucket      TokenBucketConfig   `yaml:"token_bucket"`
	SlidingWindow    SlidingWindowConfig `yaml:"sliding_window"`
	KeyTTLSeconds    int64               `yaml:"key_ttl_seconds"`
}

type TenantConfig struct {
	Isolation  bool `yaml:"isolation"`
	MaxTenants int  `yaml:"max_tenants"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{Port: 50051, MaxConnections: 1000, Timeout: 5 * time.Second},
		Redis: RedisConfig{
			Addresses:    []string{"localhost:6379"},
			DB:           0,
			PoolSize:     200,
			MinIdleConns: 50,
			MaxConnAge:   30 * time.Minute,
			IdleTimeout:  10 * time.Minute,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		},
		RateLimiter: RateLimiterConfig{
			DefaultAlgorithm: "token_bucket",
			TokenBucket:      TokenBucketConfig{DefaultRate: 100, DefaultCapacity: 200},
			SlidingWindow:    SlidingWindowConfig{DefaultWindow: 60 * time.Second, DefaultLimit: 100},
			KeyTTLSeconds:    3600,
		},
		Tenant:  TenantConfig{Isolation: true, MaxTenants: 1000},
		Metrics: MetricsConfig{Enabled: true, Port: 9090, Path: "/metrics"},
	}
}

// Load reads a YAML config file, falling back to defaults for anything unset.
func Load(path string) (*Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
