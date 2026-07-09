package tenant

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Manager enforces tenant isolation by namespacing every rate-limit key
// with the tenant ID (already baked into limiter key construction) and by
// capping the number of distinct tenants tracked, so one runaway tenant
// can't cause unbounded key growth in a shared Redis instance.
type Manager struct {
	maxTenants int
	isolation  bool

	mu      sync.RWMutex
	seen    map[string]struct{}
	stats   map[string]*Stats
}

type Stats struct {
	TotalRequests   int64
	AllowedRequests int64
	DeniedRequests  int64
}

func NewManager(isolation bool, maxTenants int) *Manager {
	return &Manager{
		isolation:  isolation,
		maxTenants: maxTenants,
		seen:       make(map[string]struct{}),
		stats:      make(map[string]*Stats),
	}
}

// Admit checks whether a new tenant may be onboarded, returning an error
// if maxTenants has already been reached. Known tenants are always admitted.
func (m *Manager) Admit(tenantID string) error {
	if !m.isolation {
		return nil
	}
	m.mu.RLock()
	_, known := m.seen[tenantID]
	count := len(m.seen)
	m.mu.RUnlock()
	if known {
		return nil
	}
	if count >= m.maxTenants {
		return fmt.Errorf("tenant limit reached (%d); rejecting new tenant %q", m.maxTenants, tenantID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.seen[tenantID] = struct{}{}
	m.stats[tenantID] = &Stats{}
	return nil
}

func (m *Manager) RecordResult(tenantID string, allowed bool) {
	m.mu.RLock()
	s, ok := m.stats[tenantID]
	m.mu.RUnlock()
	if !ok {
		m.mu.Lock()
		s = &Stats{}
		m.stats[tenantID] = s
		m.mu.Unlock()
	}
	atomic.AddInt64(&s.TotalRequests, 1)
	if allowed {
		atomic.AddInt64(&s.AllowedRequests, 1)
	} else {
		atomic.AddInt64(&s.DeniedRequests, 1)
	}
}

func (m *Manager) GetStats(tenantID string) Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.stats[tenantID]
	if !ok {
		return Stats{}
	}
	return Stats{
		TotalRequests:   atomic.LoadInt64(&s.TotalRequests),
		AllowedRequests: atomic.LoadInt64(&s.AllowedRequests),
		DeniedRequests:  atomic.LoadInt64(&s.DeniedRequests),
	}
}

func (m *Manager) TenantCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.seen)
}
