package grpctp

import "errors"

var (
	// ErrNoEndpoints indicates the provider returned no endpoints for a service.
	ErrNoEndpoints = errors.New("grpctp: no endpoints available")
)
