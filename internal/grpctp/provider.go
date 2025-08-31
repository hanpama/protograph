package grpctp

import (
	"context"
	"sync"
)

// EndpointProvider provides a list of reachable endpoints (host:port) for a given
// fully-qualified gRPC service name (e.g. "graphql.UserService").
// Implementations may integrate with service discovery/registry systems.
// Return at least one endpoint or an error.
// Implementations should be safe for concurrent use.

type EndpointProvider interface {
	Endpoints(ctx context.Context, service string) ([]string, error)
}

// StaticEndpoints is a simple provider backed by an in-memory map.
// Key is fully-qualified service name; value is list of endpoints.

type StaticEndpoints struct {
	mu   sync.RWMutex
	data map[string][]string
}

func NewStaticEndpoints(m map[string][]string) *StaticEndpoints {
	cp := make(map[string][]string, len(m))
	for k, v := range m {
		vv := make([]string, len(v))
		copy(vv, v)
		cp[k] = vv
	}
	return &StaticEndpoints{data: cp}
}

func (s *StaticEndpoints) Endpoints(ctx context.Context, service string) ([]string, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	arr := s.data[service]
	if len(arr) == 0 {
		return nil, ErrNoEndpoints
	}
	out := make([]string, len(arr))
	copy(out, arr)
	return out, nil
}
