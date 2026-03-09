package consul

import (
	"context"
	"fmt"
	"time"

	"github.com/SilentPlaces/rate_limiter/internal/application/ports"
	"github.com/SilentPlaces/rate_limiter/internal/domain/config"
	"github.com/hashicorp/consul/api"
)

type Adapter struct {
	client *api.Client
	logger ports.Logger
	parser ports.ConfigParser
}

func NewConsulAdapter(client *api.Client, parser ports.ConfigParser, logger ports.Logger) ports.ConfigProvider {
	return &Adapter{
		client: client,
		parser: parser,
		logger: logger,
	}
}

func (c *Adapter) GetConfig(ctx context.Context, key string) (config.Config, error) {
	qo := api.QueryOptions{}
	pair, _, err := c.client.KV().Get(key, qo.WithContext(ctx))
	if err != nil {
		c.logger.Error(fmt.Sprintf("ConsulAdapter: Failed to get config from consul, key is %s", key),
			ports.Field{Key: "error", Val: err})
		return config.Config{}, err
	}
	if pair == nil {
		c.logger.Info(fmt.Sprintf("ConsulAdapter: %s is nil", key))
		return config.Config{}, nil
	}

	cfg, err := c.parser.Parse(pair.Value)
	if err != nil {
		c.logger.Error(fmt.Sprintf("ConsulAdapter: Failed to parse config from consul, key is %s", key),
			ports.Field{Key: "error", Val: err})
		return config.Config{}, err
	}

	return cfg, nil
}

func (c *Adapter) WatchConfig(ctx context.Context, key string, checkingSeconds uint,
	onChange func(data config.Config), onError func(error)) {
	if onChange == nil || onError == nil {
		c.logger.Error("ConsulWatchConfig: consul watch error")
		return
	}

	kv := c.client.KV()

	pair, meta, err := kv.Get(key, nil)
	if err != nil {
		c.logger.Error("ConsulWatchConfig: consul initial fetch error",
			ports.Field{Key: "key", Val: key},
			ports.Field{Key: "err", Val: err})
		return
	}

	var lastIndex uint64
	if meta != nil {
		lastIndex = meta.LastIndex
	}

	if pair != nil {
		c.logger.Info("ConsulWatchConfig: consul initial config loaded",
			ports.Field{Key: "key", Val: key},
			ports.Field{Key: "value", Val: string(pair.Value)},
		)
		cfg, err := c.parser.Parse(pair.Value)
		if err != nil {
			c.logger.Error("ConsulWatchConfig: consul initial config parse error",
				ports.Field{Key: "key", Val: key},
				ports.Field{Key: "err", Val: err})
			onError(err)
			return
		}
		onChange(cfg)
	}

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("ConsulWatchConfig: consul watch stopped",
				ports.Field{Key: "key", Val: key})
			return
		default:
		}

		queryOpts := &api.QueryOptions{
			WaitTime:  time.Duration(checkingSeconds) * time.Second,
			WaitIndex: lastIndex,
		}

		pair, meta, err := kv.Get(key, queryOpts.WithContext(ctx))

		if err != nil {
			select {
			case <-ctx.Done():
				c.logger.Info("ConsulWatchConfig: consul watch stopped during error",
					ports.Field{Key: "key", Val: key})
				return
			default:
			}

			c.logger.Error("ConsulWatchConfig: consul watch error",
				ports.Field{Key: "key", Val: key},
				ports.Field{Key: "err", Val: err})
			onError(err)

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(checkingSeconds) * time.Second):
			}
			continue
		}

		if meta != nil {
			lastIndex = meta.LastIndex
		}

		if pair != nil {
			c.logger.Info("ConsulWatchConfig: consul config updated",
				ports.Field{Key: "key", Val: key})
			cfg, err := c.parser.Parse(pair.Value)
			if err != nil {
				c.logger.Error("ConsulWatchConfig: consul config parse error",
					ports.Field{Key: "key", Val: key},
					ports.Field{Key: "err", Val: err})
				onError(err)
				continue
			}
			onChange(cfg)
		} else {
			c.logger.Info("ConsulWatchConfig: consul config key deleted",
				ports.Field{Key: "key", Val: key})
		}
	}
}
