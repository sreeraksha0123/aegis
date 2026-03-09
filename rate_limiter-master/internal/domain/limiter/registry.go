package limiter

import (
	"fmt"
	"sync"

	"github.com/SilentPlaces/rate_limiter/internal/application/ports"
)

type AlgorithmFactory func(score ports.LimiterScore, luaScript string) ports.RateLimiter

type Registry struct {
	factories map[string]AlgorithmFactory
	mu        sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]AlgorithmFactory),
	}
}

func (r *Registry) Register(name string, factory AlgorithmFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("algorithm '%s' is already registered", name)
	}

	r.factories[name] = factory
	return nil
}

func (r *Registry) Create(name string, score ports.LimiterScore, luaScript string) (ports.RateLimiter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("algorithm is not registered: %s", name)
	}

	return factory(score, luaScript), nil
}

func (r *Registry) GetRegisteredAlgorithms() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	algorithms := make([]string, 0, len(r.factories))
	for name := range r.factories {
		algorithms = append(algorithms, name)
	}
	return algorithms
}
