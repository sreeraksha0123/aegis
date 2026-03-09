package ports

import "github.com/SilentPlaces/rate_limiter/internal/domain/config"

type ConfigService interface {
	GetConfig() config.Config
}
