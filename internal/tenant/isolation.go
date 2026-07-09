package tenant

import "strings"

// Namespace builds an isolated key namespace for a tenant. Used anywhere a
// key needs the tenant baked in beyond what the limiter already does (e.g.
// per-tenant config lookups, per-tenant metrics).
func Namespace(tenant string) string {
	if tenant == "" {
		tenant = "default"
	}
	return "t:" + sanitize(tenant)
}

// sanitize strips characters that would break Redis key parsing or allow
// one tenant to construct a key colliding with another tenant's namespace.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r == ':' || r == ' ' || r == '\n' || r == '\t' {
			return '_'
		}
		return r
	}, s)
}
