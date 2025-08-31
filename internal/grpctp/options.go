package grpctp

import (
	"time"

	"google.golang.org/grpc"
)

// Options configures the gRPC transport behavior.
//
// Defaults:
// - MaxConnsPerEndpoint: 2
// - RPCTimeout:          3s (used only if incoming context has no deadline)
// - DialOptions:         insecure credentials
//
// EndpointProvider must be provided (use StaticEndpoints or a custom implementation).
// If Provider is nil, the transport will error on calls.
//
// All options are safe to leave zero-valued to use defaults.

type Options struct {
	Provider EndpointProvider

	MaxConnsPerEndpoint int
	RPCTimeout          time.Duration

	DialOptions []grpc.DialOption
}

// Option mutates Options
//
// Use WithX helpers below.

type Option func(*Options)

func defaultOptions() *Options {
	return &Options{
		MaxConnsPerEndpoint: 2,
		RPCTimeout:          3 * time.Second,
	}
}

func WithProvider(p EndpointProvider) Option { return func(o *Options) { o.Provider = p } }
func WithMaxConnsPerEndpoint(n int) Option   { return func(o *Options) { o.MaxConnsPerEndpoint = n } }
func WithRPCTimeout(d time.Duration) Option  { return func(o *Options) { o.RPCTimeout = d } }
func WithDialOptions(opts ...grpc.DialOption) Option {
	return func(o *Options) { o.DialOptions = opts }
}
