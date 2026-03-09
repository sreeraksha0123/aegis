package config

import (
	"encoding/json"
	"fmt"

	"github.com/SilentPlaces/rate_limiter/internal/application/ports"
	domainConfig "github.com/SilentPlaces/rate_limiter/internal/domain/config"
)

type Parser struct{}

func NewParser() ports.ConfigParser {
	return &Parser{}
}

func (p *Parser) Parse(data []byte) (domainConfig.Config, error) {
	var dto limiterConfigDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return domainConfig.Config{}, fmt.Errorf("failed to unmarshal limiter config: %w", err)
	}

	return dtoToDomain(dto), nil
}

type limiterConfigDTO struct {
	Routes map[string]routeConfigDTO `json:"routes"`
}

type routeConfigDTO struct {
	Algorithm string          `json:"algorithm"`
	ConfigRaw json.RawMessage `json:"-"`
	Config    interface{}     `json:"config,omitempty"`
}

type fixedWindowConfigDTO struct {
	Limit  int `json:"limit"`
	Window int `json:"window"`
}

type tokenBucketConfigDTO struct {
	Capacity   int `json:"capacity"`
	RefillRate int `json:"refill_rate"`
	BucketTTL  int `json:"bucket_ttl"`
}

type slidingWindowConfigDTO struct {
	Limit  int `json:"limit"`
	Window int `json:"window"`
}

func (r *routeConfigDTO) UnmarshalJSON(data []byte) error {
	type Alias routeConfigDTO
	aux := struct {
		Algorithm string          `json:"algorithm"`
		Raw       json.RawMessage `json:"-"`
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	r.Algorithm = aux.Algorithm
	r.ConfigRaw = data

	switch aux.Algorithm {
	case domainConfig.AlgorithmFixedWindow:
		var cfg fixedWindowConfigDTO
		if err := json.Unmarshal(data, &cfg); err != nil {
			return err
		}
		r.Config = cfg
	case domainConfig.AlgorithmTokenBucket:
		var cfg tokenBucketConfigDTO
		if err := json.Unmarshal(data, &cfg); err != nil {
			return err
		}
		r.Config = cfg
	case domainConfig.AlgorithmSlidingWindow:
		var cfg slidingWindowConfigDTO
		if err := json.Unmarshal(data, &cfg); err != nil {
			return err
		}
		r.Config = cfg
	default:
		r.Config = nil
	}
	return nil
}

func dtoToDomain(dto limiterConfigDTO) domainConfig.Config {
	cfg := domainConfig.Config{
		Routes: make(map[string]domainConfig.RouteConfig),
	}

	for route, routeDTO := range dto.Routes {
		domainRoute := domainConfig.RouteConfig{
			Algorithm: routeDTO.Algorithm,
		}

		switch c := routeDTO.Config.(type) {
		case fixedWindowConfigDTO:
			domainRoute.Config = domainConfig.FixedWindowConfig{
				Limit:  c.Limit,
				Window: c.Window,
			}
		case tokenBucketConfigDTO:
			domainRoute.Config = domainConfig.TokenBucketConfig{
				Capacity:   c.Capacity,
				RefillRate: c.RefillRate,
				BucketTTL:  c.BucketTTL,
			}
		case slidingWindowConfigDTO:
			domainRoute.Config = domainConfig.SlidingWindowConfig{
				Limit:  c.Limit,
				Window: c.Window,
			}
		default:
			domainRoute.Config = nil
		}

		cfg.Routes[route] = domainRoute
	}

	return cfg
}
