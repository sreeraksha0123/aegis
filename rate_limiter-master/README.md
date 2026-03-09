# Distributed Rate Limiter Service

A distributed rate limiting service built with Go, Redis, and Consul.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.23.0-blue)](https://golang.org/)
[![Redis](https://img.shields.io/badge/Redis-8.2.2-red)](https://redis.io/)
[![Consul](https://img.shields.io/badge/Consul-1.15.4-purple)](https://www.consul.io/)

## 🎯 Overview

This rate limiter acts as a reverse proxy that enforces request rate limits before forwarding traffic to backend services. It uses Redis for distributed state management and Consul for dynamic configuration, allowing you to update rate limiting rules without service restarts.

**Key Design Goals:**
- Clean, testable architecture following SOLID principles
- Hot-reloadable configuration via Consul
- Atomic rate limiting operations using Lua scripts
- Resilience patterns (circuit breakers) for fault tolerance
- Multiple algorithm support with extensible design
- Production-ready logging and graceful shutdown

## ✨ Features

### Rate Limiting Algorithms

- ✅ **Fixed Window** - Simple, efficient time-window based limiting
- ✅ **Token Bucket** - Smooth rate limiting with burst support
- ✅ **Sliding Window** - More accurate rate limiting
- 🔜 **Leaky Bucket** - Constant request rate (planned)

### Architecture & Design

- **Clean Architecture** - Clear separation of concerns with well-defined boundaries
- **Dependency Inversion** - All dependencies point inward toward domain logic
- **Hexagonal Architecture (Ports & Adapters)** - Infrastructure is pluggable and replaceable

### Infrastructure Features

- **Dynamic Configuration** - Hot-reload rules from Consul without restarts
- **Distributed State** - Redis-backed rate limiting across multiple service instances
- **Atomic Operations** - Lua scripts ensure race-condition-free updates
- **Script Caching** - Redis EVALSHA with preloaded scripts for optimal performance
- **Circuit Breaker** - Resilient Redis adapter prevents cascading failures
- **IP Whitelisting** - Bypass rate limits for trusted IPs
- **Route-Based Limiting** - Different limits for different API endpoints

### Production Ready

- Structured logging with zerolog
- Docker and Docker Compose deployment
- Health check endpoints
- Rate limit response headers (`X-RateLimit-*`)
- Environment variable configuration override
- Comprehensive error handling

## 🏗️ Architecture

### Project Structure

```
rate_limiter/
├── cmd/server/              # Application entry point
│   ├── main.go              # Server initialization & graceful shutdown
│   └── dep/                 # Dependency injection container
│       └── di_container.go  # Wire all dependencies
├── config/                  # Bootstrap configuration
│   ├── config.go            # Config loader with env override
│   └── config.yml           # YAML configuration
├── internal/
│   ├── domain/              # Pure business logic (no external dependencies)
│   │   ├── config/          # Rate limit configuration models & algorithm constants
│   │   │   └── config.go    # Limiter Algorithm name constants
│   │   ├── errors/          # Domain error types
│   │   └── limiter/         # Domain services
│   │       ├── policy.go    # IP whitelisting policy
│   │       └── registry.go  # Algorithm factory registry
│   ├── application/         # Use cases & orchestration
│   │   ├── ports/           # Interface definitions (abstractions)
│   │   └── service/         # Business logic implementations
│   │       ├── config_service.go   # Hot-reload config management
│   │       └── limiter_service.go  # Rate limiting orchestration
│   ├── infrastructure/      # Technical implementations
│   │   ├── config/          # JSON parser implementation
│   │   ├── consul/          # Consul client adapter
│   │   ├── limiter/         # Rate limiting algorithms
│   │   │   ├── fixed_window.go    # Fixed window implementation
│   │   │   └── token_bucket.go    # Token bucket implementation
│   │   ├── logger/          # Structured logging adapter
│   │   ├── redis/           # Redis adapters
│   │   │   ├── redis_adapter.go      # Base Redis client
│   │   │   └── resilient_adapter.go  # Circuit breaker wrapper
│   │   └── resilience/      # Resilience patterns
│   │       └── circuit_breaker.go   # Circuit breaker implementation
│   └── interfaces/          # External interfaces
│       └── http/            # HTTP handlers & reverse proxy
├── scripts/lua/             # Lua scripts for atomic Redis operations
│   ├── fixed_window.lua     # Fixed window algorithm
│   └── token_bucket.lua     # Token bucket algorithm
├── nginx/                   # Nginx configurations
│   ├── frontend.conf        # Frontend proxy (adds X-Rate-Limit-Rule headers)
│   └── backend.conf         # Backend service
├── docker-compose.yml       # Full stack deployment
└── Dockerfile              # Rate limiter service image
```

### Dependency Flow

The architecture follows **Clean Architecture** principles with strict dependency rules:

```
┌─────────────────────────────────────────────────────┐
│                    Interfaces                       │
│              (HTTP Handlers, CLI)                   │
└─────────────────────────────────────────────────────┘
                        ↓ depends on
┌─────────────────────────────────────────────────────┐
│                 Infrastructure                      │
│     (Redis, Consul, Logging, Algorithms)            │
└─────────────────────────────────────────────────────┘
                        ↓ depends on
┌─────────────────────────────────────────────────────┐
│                   Application                       │
│         (Services, Use Cases, Ports)                │
└─────────────────────────────────────────────────────┘
                        ↓ depends on
┌─────────────────────────────────────────────────────┐
│                     Domain                          │
│    (Business Logic, Entities, Constants)            │
│              (Zero Dependencies)                    │
└─────────────────────────────────────────────────────┘
```

### How It Works

1. **Startup** - Lua scripts are loaded into Redis via SCRIPT LOAD, compiled and cached with SHA1 hashes
2. **Frontend Nginx** receives client requests and adds `X-Rate-Limit-Rule` header
3. **Rate Limiter Service** extracts client IP and route key (key is `X-Rate-Limit-Rule` header), checks against Consul config
4. **Redis** executes cached Lua scripts via EVALSHA for atomic, race-free rate limit checks
5. **Circuit Breaker** protects against Redis failures
6. **Reverse Proxy** forwards allowed requests to backend services
7. **Backend Nginx** processes the request and returns response

## 📋 Prerequisites

- **Go** 1.23.0 or higher
- **Docker** and **Docker Compose** (for running dependencies)
- **Redis** 8.2.2
- **Consul** 1.15.4

## 🚀 Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/SilentPlaces/rate_limiter.git
cd rate_limiter
```

### 2. Start with Docker Compose

```bash
docker-compose up -d
```

This starts:
- **Redis** on `localhost:6379`
- **Consul** on `localhost:8500`
- **Rate Limiter** on `localhost:8080`
- **Frontend Nginx** on `localhost:80`
- **Backend Nginx** (internal)

The initial configuration is automatically loaded into Consul.

### 3. Test the Rate Limiter

```bash
# Make requests to test fixed window rate limiting (10 requests per 60 seconds)
for i in {1..15}; do
  curl -s -o /dev/null -w "Request $i: %{http_code}\n" http://localhost/
  sleep 0.5
done

# Test token bucket endpoint (100 capacity, 10 tokens/second)
curl http://localhost:8080/api/v1/test
```

### 4. Access Consul UI

Open `http://localhost:8500` to view and modify the rate limiter configuration in real-time.

## ⚙️ Configuration

### Application Configuration (`config/config.yml`)

```yaml
server:
  port: 8080
  address: "0.0.0.0"
  shutdown_timeout_seconds: 5

redis:
  addr: "redis"
  port: 6379
  db: 0
  connection_timeout_seconds: 60
  operation_timeout_seconds: 10
  circuit_breaker_max_failures: 5        # Open circuit after 5 failures
  circuit_breaker_timeout_seconds: 30    # Try half-open after 30 seconds

consul:
  addr: "http://consul:8500"

app:
  fetch_config_period_seconds: 300       # Poll Consul every 5 minutes
  config_key: "rate_limiter_config"      # Consul KV key
  backend_nginx_addr: "http://backend_nginx:80"
  whitelisted_ips:                       # IPs that bypass rate limiting
    - "127.0.0.1"
    - "::1"
    - "10.0.0.1"
```

**Environment Variable Override:** Any config value can be overridden using environment variables with dot notation (e.g., `REDIS.ADDR=localhost`).

### Rate Limiting Rules (Consul KV)

The rate limiting rules are stored in Consul under the key `rate_limiter_config` and support hot-reloading.

> **Algorithm Constants:** Algorithm types (`"fixed_window"`, `"token_bucket"`) are defined as constants in `internal/domain/config/config.go`:
> ```go
> const (
>     AlgorithmFixedWindow = "fixed_window"
>     AlgorithmTokenBucket = "token_bucket"
> )
> ```

#### Fixed Window Algorithm

```json
{
  "routes": {
    "api-users": {
      "algorithm": "fixed_window",
      "limit": 100,
      "window": 60
    }
  }
}
```

**Parameters:**
- `algorithm`: `"fixed_window"`
- `limit`: Maximum requests allowed in the window
- `window`: Time window in seconds

**How it works:** Counts requests in fixed time windows. Simple and efficient, but can allow bursts at window boundaries.

#### Token Bucket Algorithm

```json
{
  "routes": {
    "api-upload": {
      "algorithm": "token_bucket",
      "capacity": 50,
      "refill_rate": 10,
      "bucket_ttl": 300
    }
  }
}
```

**Parameters:**
- `algorithm`: `"token_bucket"`
- `capacity`: Maximum tokens in the bucket
- `refill_rate`: Tokens added per second
- `bucket_ttl`: Redis key TTL in seconds

**How it works:** Allows bursts up to capacity while maintaining average rate. Tokens refill at a constant rate.

### Dynamic Configuration Updates

Update rate limits without restarting:

```bash
# Using Consul CLI
consul kv put rate_limiter_config '{
  "routes": {
    "api-users": {
      "algorithm": "fixed_window",
      "limit": 200,
      "window": 60
    }
  }
}'

# Or use the Consul UI at http://localhost:8500
```

The service polls Consul every 5 minutes (configurable via `fetch_config_period_seconds`).

## 🔌 API Usage

The rate limiter acts as a reverse proxy. Requests must include the `X-Rate-Limit-Rule` header (typically added by your frontend proxy/load balancer) to specify which route configuration to apply.

### Request Flow

```
Client → Frontend Nginx → Rate Limiter → Backend Service
         (adds header)    (checks limit)  (processes request)
```

### Example with curl

```bash
# Test root route (configured in Consul)
curl http://localhost:8080/

# Test API endpoint
curl http://localhost:8080/api/v1/test
```

### Response Headers

When rate limiting is active, the service returns:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1609459200
```

### Rate Limit Exceeded Response

**Status:** `429 Too Many Requests`  
**Body:** `rate limit exceeded`

## 🛠️ Development

### Running Locally

```bash
# Build and run the service
docker compose build && docker compose up -d

```

### Building

```bash
# Build binary
go build -o rate-limiter ./cmd/server


# Build Docker image
docker build -t rate-limiter:latest .
```

### Adding a New Rate Limiting Algorithm

The project uses a centralized constants approach for algorithm types, making it easy to add new algorithms:

#### 1. Define Algorithm Constant

In `internal/domain/config/config.go`:

```go
const (
    AlgorithmFixedWindow   = "fixed_window"
    AlgorithmTokenBucket   = "token_bucket"
    AlgorithmSlidingWindow = "sliding_window"  // ← New!
)
```

#### 2. Create Domain Config Model

In `internal/domain/config/config.go`:

```go
type SlidingWindowConfig struct {
    Limit      int
    WindowSize int
}

func (s SlidingWindowConfig) AlgorithmName() string {
    return AlgorithmSlidingWindow  // Use the constant
}

func (s SlidingWindowConfig) Validate() error {
    if s.Limit <= 0 {
        return fmt.Errorf("limit must be positive")
    }
    if s.WindowSize <= 0 {
        return fmt.Errorf("window_size must be positive")
    }
    return nil
}
```

#### 3. Create Lua Script

Create `scripts/lua/sliding_window.lua` with your algorithm logic.

#### 4. Implement Algorithm

In `internal/infrastructure/limiter/sliding_window.go`:

```go
package limiter

import (
    "context"
    "github.com/SilentPlaces/rate_limiter/internal/application/ports"
    "github.com/SilentPlaces/rate_limiter/internal/domain/config"
)

type SlidingWindowLimiter struct {
    score      ports.LimiterScore
    scriptSHA1 string
}

func SlidingWindowLimiterFactory(score ports.LimiterScore, scriptSHA1 string) ports.RateLimiter {
    return &SlidingWindowLimiter{score: score, scriptSHA1: scriptSHA1}
}

func (s *SlidingWindowLimiter) Allow(ctx context.Context, key string, cfg config.AlgorithmConfig) (ports.RateLimitInfo, error) {
    slidingCfg, ok := cfg.(config.SlidingWindowConfig)
    if !ok {
        return ports.RateLimitInfo{}, fmt.Errorf("invalid config type")
    }
    
    // Execute cached Lua script using SHA1 hash (loaded at startup)
    res, err := s.score.EvalSha(ctx, s.scriptSHA1, []string{key}, []interface{}{
        slidingCfg.WindowSize, slidingCfg.Limit,
    })
    if err != nil {
        return ports.RateLimitInfo{}, err
    }
    
    // Parse and return result
    return parseResult(res, slidingCfg.Limit), nil
}
```

#### 5. Update Parser

In `internal/infrastructure/config/parser.go`, add a case to the switch statement:

```go
switch aux.Algorithm {
case domainConfig.AlgorithmFixedWindow:
    // existing code
case domainConfig.AlgorithmTokenBucket:
    // existing code
case domainConfig.AlgorithmSlidingWindow:  // ← New!
    var cfg slidingWindowConfigDTO
    if err := json.Unmarshal(data, &cfg); err != nil {
        return err
    }
    r.Config = cfg
}
```

And update the DTO conversion in `dtoToDomain()`.

#### 6. Register in DI Container

In `cmd/server/dep/di_container.go`:

```go
registry := domainLimiter.NewRegistry()
_ = registry.Register(domainConfig.AlgorithmFixedWindow, limiter.FixedWindowLimiterFactory)
_ = registry.Register(domainConfig.AlgorithmTokenBucket, limiter.TokenBucketLimiterFactory)
_ = registry.Register(domainConfig.AlgorithmSlidingWindow, limiter.SlidingWindowLimiterFactory)  // ← New!

// Add to lua files loading
luaFiles, err := loadLuaFiles([]string{
    fmt.Sprintf("scripts/lua/%s.lua", domainConfig.AlgorithmFixedWindow),
    fmt.Sprintf("scripts/lua/%s.lua", domainConfig.AlgorithmTokenBucket),
    fmt.Sprintf("scripts/lua/%s.lua", domainConfig.AlgorithmSlidingWindow),  // ← New!
}, log)

// Load scripts into Redis and get SHA1 hashes
scriptSHA1s, err := loadScriptsIntoRedis(ctx, redisAdapter, luaFiles, log)

// Create limiters with SHA1 hashes
limiters := make(map[string]ports.RateLimiter)
for algo, scriptPath := range map[string]string{
    domainConfig.AlgorithmFixedWindow:   fmt.Sprintf("scripts/lua/%s.lua", domainConfig.AlgorithmFixedWindow),
    domainConfig.AlgorithmTokenBucket:   fmt.Sprintf("scripts/lua/%s.lua", domainConfig.AlgorithmTokenBucket),
    domainConfig.AlgorithmSlidingWindow: fmt.Sprintf("scripts/lua/%s.lua", domainConfig.AlgorithmSlidingWindow),  // ← New!
} {
    limiterInstance, err := registry.Create(algo, redisAdapter, scriptSHA1s[scriptPath])
    if err != nil {
        return nil, fmt.Errorf("create limiter '%s': %w", algo, err)
    }
    limiters[algo] = limiterInstance
}
```

**That's it!** The new algorithm is now available for use in Consul configurations.

## 🧪 Testing

### Redis Monitoring

```bash
# Connect to Redis and monitor keys
docker exec -it redis redis-cli

# View rate limit keys
KEYS rl:*

# Check a specific key's value
GET rl:fixed_window:root:192.168.1.100

# View loaded Lua scripts (cached at startup)
SCRIPT LIST

# Check if scripts are loaded
SCRIPT EXISTS <sha1_hash>

# Monitor real-time commands (you'll see EVALSHA instead of EVAL)
MONITOR
```

## 📦 Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/redis/go-redis/v9` | v9.14.1 | Redis client for distributed state |
| `github.com/hashicorp/consul/api` | v1.13.0 | Consul client for dynamic config |
| `github.com/rs/zerolog` | v1.34.0 | Structured logging |
| `github.com/knadh/koanf` | v1.5.0 | Configuration management |

## 🤝 Contributing

**I'm open to contributions of all kinds!** Whether you're fixing bugs, adding features, improving documentation, or suggesting new ideas, your contributions are welcome and appreciated.

### How to Contribute

1. **Open an Issue**
   - Report bugs with detailed reproduction steps
   - Suggest features or improvements
   - Ask questions about the implementation

2. **Submit a Pull Request**
   - Fork the repository
   - Create a feature branch (`git checkout -b feature/amazing-feature`)
   - Make your changes following the existing code style
   - Add tests if applicable
   - Commit your changes (`git commit -m 'Add amazing feature'`)
   - Push to your branch (`git push origin feature/amazing-feature`)
   - Open a Pull Request with a clear description

3. **Improve Documentation**
   - Fix typos or unclear explanations
   - Add examples or use cases
   - Translate documentation

### Contribution Guidelines

- **Code Style:** Follow standard Go conventions and existing patterns
- **Commit Messages:** Write clear, descriptive commit messages
- **Keep Changes Focused:** One feature/fix per PR when possible
- **Add Context:** Explain *why* a change is needed, not just *what* changed
- **Be Respectful:** All contributors are expected to follow the code of conduct

### Areas for Contribution

- 🔧 Implement new rate limiting algorithms (Leaky Bucket)
- 📊 Add Prometheus metrics and monitoring
- ✅ Expand test coverage
- 🔐 Add authentication/API key based rate limiting
- 📚 Improve documentation with more examples
- 🐛 Fix bugs and improve error handling
- 🚀 Performance optimizations

**All contributions, no matter how small, are valuable and appreciated!**

## 📄 License

This project is licensed under the **MIT License** - see the [LICENSE](LICENSE) file for details.

```
MIT License

Copyright (c) 2025 Armin

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
```

## 🗺️ Roadmap

- [x] Fixed Window algorithm
- [x] Token Bucket algorithm
- [x] IP whitelisting
- [x] Circuit breaker pattern
- [x] Clean architecture refactoring
- [x] Docker deployment
- [x] Algorithm type constants for maintainability
- [x] Sliding Window algorithm
- [x] Redis script caching with EVALSHA for performance
- [ ] Leaky Bucket algorithm
- [ ] Prometheus metrics and monitoring
- [ ] Comprehensive test suite (unit + integration)
- [ ] User/API key based rate limiting
- [ ] OpenTelemetry distributed tracing
- [ ] Admin API for runtime management
- [ ] Grafana dashboards

## 📞 Contact & Support

- **Issues:** [GitHub Issues](https://github.com/SilentPlaces/rate_limiter/issues)
- **Discussions:** [GitHub Discussions](https://github.com/SilentPlaces/rate_limiter/discussions)

## 🙏 Acknowledgments

This project was built to demonstrate:
- Clean Architecture and SOLID principles in Go
- Distributed systems patterns (rate limiting, circuit breakers)
- Production-ready infrastructure with Redis and Consul
- Extensible design for adding new algorithms

Special thanks to the Go community and all contributors!

---

**⭐ If you find this project useful, please consider giving it a star!**

*Built with ❤️ by Armin*
