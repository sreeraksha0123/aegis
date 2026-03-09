package errors

import "fmt"

type RateLimiterError struct {
	Code    string
	Message string
	Err     error
}

func (e *RateLimiterError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *RateLimiterError) Unwrap() error {
	return e.Err
}

func NewRateLimiterError(code, message string, err error) *RateLimiterError {
	return &RateLimiterError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

var (
	ErrUnknownAlgorithm = &RateLimiterError{
		Code:    "UNKNOWN_ALGORITHM",
		Message: "rate limiting algorithm not found",
	}
	ErrConfigNotFound = &RateLimiterError{
		Code:    "CONFIG_NOT_FOUND",
		Message: "route configuration not found",
	}
	ErrRedisOperation = &RateLimiterError{
		Code:    "REDIS_ERROR",
		Message: "redis operation failed",
	}
	ErrInvalidConfig = &RateLimiterError{
		Code:    "INVALID_CONFIG",
		Message: "configuration validation failed",
	}
	ErrConsulOperation = &RateLimiterError{
		Code:    "CONSUL_ERROR",
		Message: "consul operation failed",
	}
)
