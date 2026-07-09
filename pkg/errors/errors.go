package errors

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrUnknownAlgorithm = errors.New("unknown rate-limit algorithm")
	ErrTenantLimit      = errors.New("tenant limit reached")
	ErrRedisUnavailable = errors.New("redis unavailable")
	ErrCircuitOpen      = errors.New("circuit breaker open: redis calls suspended")
	ErrInvalidRequest   = errors.New("invalid request")
)

// ToGRPCStatus maps internal errors to appropriate gRPC status codes so
// clients get actionable signals instead of a generic Unknown/Internal.
func ToGRPCStatus(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrUnknownAlgorithm), errors.Is(err, ErrInvalidRequest):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ErrTenantLimit):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, ErrRedisUnavailable), errors.Is(err, ErrCircuitOpen):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Internal, fmt.Sprintf("internal error: %v", err))
	}
}
