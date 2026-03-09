package limiter

import (
	"fmt"
	"net"
	"strings"
)

type Policy struct {
	whitelistedIPs map[string]struct{}
}

func NewPolicy(whitelistedIPs []string) (*Policy, error) {
	if len(whitelistedIPs) == 0 {
		return &Policy{whitelistedIPs: make(map[string]struct{})}, nil
	}

	whitelist := make(map[string]struct{}, len(whitelistedIPs))
	for _, ip := range whitelistedIPs {
		normalized := strings.TrimSpace(ip)
		if normalized == "" {
			continue
		}
		if err := validateIP(normalized); err != nil {
			return nil, fmt.Errorf("invalid whitelisted IP '%s': %w", ip, err)
		}
		whitelist[normalized] = struct{}{}
	}

	return &Policy{whitelistedIPs: whitelist}, nil
}

func (p *Policy) ShouldBypassRateLimit(ip string) bool {
	_, exists := p.whitelistedIPs[ip]
	return exists
}

func (p *Policy) WhitelistedIPsCount() int {
	return len(p.whitelistedIPs)
}

func validateIP(ip string) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address format")
	}
	return nil
}
