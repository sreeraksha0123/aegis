package dep

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/SilentPlaces/rate_limiter/config"
	"github.com/SilentPlaces/rate_limiter/internal/application/ports"
	"github.com/SilentPlaces/rate_limiter/internal/application/service"
	domainConfig "github.com/SilentPlaces/rate_limiter/internal/domain/config"
	domainLimiter "github.com/SilentPlaces/rate_limiter/internal/domain/limiter"
	infraConfig "github.com/SilentPlaces/rate_limiter/internal/infrastructure/config"
	"github.com/SilentPlaces/rate_limiter/internal/infrastructure/consul"
	"github.com/SilentPlaces/rate_limiter/internal/infrastructure/limiter"
	redis2 "github.com/SilentPlaces/rate_limiter/internal/infrastructure/redis"
	handler "github.com/SilentPlaces/rate_limiter/internal/interfaces/http"
	"github.com/hashicorp/consul/api"
	"github.com/redis/go-redis/v9"
)

// Container holds all initialized application services and their dependencies.
type Container struct {
	Log                ports.Logger
	Config             *config.Config
	RedisClient        *redis.Client
	ConsulClient       *api.Client
	ConfigService      *service.ConfigService
	RateLimiterService *service.LimiterService
	HTTPHandler        http.Handler
}

func (c *Container) Close() {
	if c.RedisClient != nil {
		c.Log.Info("Closing Redis connection...")
		if err := c.RedisClient.Close(); err != nil {
			c.Log.Error("failed to close redis client", ports.Field{Key: "err", Val: err.Error()})
		}
	}
}

// New builds dependencies using already created logger and loaded config.
func New(ctx context.Context, log ports.Logger, cfg *config.Config) (*Container, error) {
	// Initialize Redis
	rc, err := newRedisClient(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}

	// Initialize Consul
	cc, err := newConsulClient(cfg.Consul)
	if err != nil {
		_ = rc.Close()
		return nil, fmt.Errorf("consul: %w", err)
	}

	// Adapters
	configParser := infraConfig.NewParser()
	consulAdapter := consul.NewConsulAdapter(cc, configParser, log)
	baseRedisAdapter := redis2.NewRedisAdapter(rc, log, cfg.Redis.OperationTimeoutSeconds)
	redisAdapter := redis2.NewResilientRedisAdapter(
		baseRedisAdapter,
		log,
		cfg.Redis.CircuitBreakerMaxFailures,
		time.Duration(cfg.Redis.CircuitBreakerTimeoutSeconds)*time.Second,
	)

	// Create Config Service
	cfgSvc := service.NewConfigService(consulAdapter, log, cfg.App)
	if err := cfgSvc.LoadOnce(ctx, cfg.App.ConfigKey); err != nil {
		_ = rc.Close()
		return nil, fmt.Errorf("config load: %w", err)
	}
	cfgSvc.WatchConfig(ctx, cfg.App.ConfigKey)
	log.Info("ConfigService initialized")

	// Load lua files
	luaFiles, err := loadLuaFiles([]string{
		fmt.Sprintf("scripts/lua/%s.lua", domainConfig.AlgorithmFixedWindow),
		fmt.Sprintf("scripts/lua/%s.lua", domainConfig.AlgorithmTokenBucket),
		fmt.Sprintf("scripts/lua/%s.lua", domainConfig.AlgorithmSlidingWindow),
	}, log)
	if err != nil {
		_ = rc.Close()
		return nil, fmt.Errorf("lua files: %w", err)
	}
	log.Info("Lua files loaded from disk", ports.Field{Key: "count", Val: len(luaFiles)})

	// Load scripts into Redis and get SHA1 hashes
	scriptSHA1s, err := loadScriptsIntoRedis(ctx, redisAdapter, luaFiles, log)
	if err != nil {
		_ = rc.Close()
		return nil, fmt.Errorf("load scripts into redis: %w", err)
	}
	log.Info("Lua scripts loaded into Redis", ports.Field{Key: "count", Val: len(scriptSHA1s)})

	// Register rate limiter algorithms
	registry := domainLimiter.NewRegistry()
	_ = registry.Register(domainConfig.AlgorithmFixedWindow, limiter.FixedWindowLimiterFactory)
	_ = registry.Register(domainConfig.AlgorithmTokenBucket, limiter.TokenBucketLimiterFactory)
	_ = registry.Register(domainConfig.AlgorithmSlidingWindow, limiter.SlidingWindowLimiterFactory)

	// Load limiter algorithms with SHA1 hashes
	limiters := make(map[string]ports.RateLimiter)
	for algo, scriptPath := range map[string]string{
		domainConfig.AlgorithmFixedWindow:   fmt.Sprintf("scripts/lua/%s.lua", domainConfig.AlgorithmFixedWindow),
		domainConfig.AlgorithmTokenBucket:   fmt.Sprintf("scripts/lua/%s.lua", domainConfig.AlgorithmTokenBucket),
		domainConfig.AlgorithmSlidingWindow: fmt.Sprintf("scripts/lua/%s.lua", domainConfig.AlgorithmSlidingWindow),
	} {
		// Create instance of each registered algorithm with SHA1 hash
		limiterInstance, err := registry.Create(algo, redisAdapter, scriptSHA1s[scriptPath])
		if err != nil {
			_ = rc.Close()
			return nil, fmt.Errorf("create limiter '%s': %w", algo, err)
		}
		limiters[algo] = limiterInstance
	}

	log.Info("Rate limiters initialized", ports.Field{Key: "algorithms", Val: registry.GetRegisteredAlgorithms()})

	// Create limiter policy for whitelisted IPs
	policy, err := domainLimiter.NewPolicy(cfg.App.WhitelistedIPs)
	if err != nil {
		_ = rc.Close()
		return nil, fmt.Errorf("policy creation: %w", err)
	}

	// Rate limiter service
	limiterSvc := service.NewLimiterService(log, cfgSvc, limiters, policy)
	log.Info("LimiterService initialized", ports.Field{Key: "whitelisted_ips", Val: policy.WhitelistedIPsCount()})

	// HTTP
	h, err := handler.NewHTTPHandler(limiterSvc, log, cfg.App.BackendNginxAddr)
	if err != nil {
		_ = rc.Close()
		return nil, fmt.Errorf("http handler: %w", err)
	}
	log.Info("HTTPHandler initialized")

	return &Container{
		Log:                log,
		Config:             cfg,
		RedisClient:        rc,
		ConsulClient:       cc,
		ConfigService:      cfgSvc,
		RateLimiterService: limiterSvc,
		HTTPHandler:        h,
	}, nil
}

func newRedisClient(cfg config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", cfg.Addr, cfg.Port),
		DB:   cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ConnectionTimeoutSeconds)*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connect to Redis: %w", err)
	}
	return client, nil
}

func newConsulClient(cfg config.ConsulConfig) (*api.Client, error) {
	consulCfg := api.DefaultConfig()
	consulCfg.Address = cfg.Addr
	return api.NewClient(consulCfg)
}

func loadLuaFiles(paths []string, log ports.Logger) (map[string]string, error) {
	contents := make(map[string]string)
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			log.Error("Failed to read lua file",
				ports.Field{Key: "path", Val: p},
				ports.Field{Key: "error", Val: err},
			)
			continue
		}
		contents[p] = string(data)
	}
	return contents, nil
}

func loadScriptsIntoRedis(ctx context.Context, redisAdapter ports.LimiterScore, luaScripts map[string]string, log ports.Logger) (map[string]string, error) {
	scriptSHA1s := make(map[string]string)
	for path, script := range luaScripts {
		sha1, err := redisAdapter.ScriptLoad(ctx, script)
		if err != nil {
			log.Error("Failed to load script into Redis",
				ports.Field{Key: "path", Val: path},
				ports.Field{Key: "error", Val: err},
			)
			return nil, fmt.Errorf("load script %s: %w", path, err)
		}
		scriptSHA1s[path] = sha1
		log.Info("Script loaded into Redis",
			ports.Field{Key: "path", Val: path},
			ports.Field{Key: "sha1", Val: sha1},
		)
	}
	return scriptSHA1s, nil
}
