package ports

import "github.com/SilentPlaces/rate_limiter/internal/domain/config"

type ConfigParser interface {
	Parse(data []byte) (config.Config, error)
}
