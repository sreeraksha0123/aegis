package ports

import (
	"context"

	"github.com/SilentPlaces/rate_limiter/internal/domain/config"
)

type ConfigProvider interface {
	GetConfig(ctx context.Context, key string) (config.Config, error)
	WatchConfig(
		ctx context.Context,
		key string,
		checkingSeconds uint,
		onChange func(data config.Config),
		onError func(error),
	)
}
