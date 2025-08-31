package grpctp

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	eventbus "github.com/hanpama/protograph/internal/eventbus"
	events "github.com/hanpama/protograph/internal/events"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hanpama/protograph/internal/grpcrt"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Transport is a real gRPC transport with connection pooling and deadline
// propagation. It integrates with an EndpointProvider for service discovery.

type Transport struct {
	opts *Options

	mu     sync.RWMutex
	pools  map[string]*connPool // key: endpoint
	closed atomic.Bool
}

func New(opts ...Option) *Transport {
	o := defaultOptions()
	for _, f := range opts {
		f(o)
	}
	if len(o.DialOptions) == 0 {
		o.DialOptions = []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithConnectParams(grpc.ConnectParams{Backoff: backoff.DefaultConfig}),
		}
	}
	return &Transport{
		opts:  o,
		pools: make(map[string]*connPool),
	}
}

// Ensure we satisfy grpcrt.Transport
var _ grpcrt.Transport = (*Transport)(nil)

func (t *Transport) Call(ctx context.Context, method protoreflect.MethodDescriptor, request protoreflect.Message) (resp protoreflect.Message, err error) {
	if t.closed.Load() {
		err = fmt.Errorf("grpctp: closed")
		return
	}
	if t.opts.Provider == nil {
		err = fmt.Errorf("grpctp: provider not configured")
		return
	}
	service := string(method.Parent().FullName())
	mthFull := fmt.Sprintf("/%s/%s", service, method.Name())

	// Determine deadline
	if _, ok := ctx.Deadline(); !ok {
		// apply default if provided
		if t.opts.RPCTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, t.opts.RPCTimeout)
			defer cancel()
		}
	}

	// simple metadata for tracing (optional)
	ctx = metadata.AppendToOutgoingContext(ctx, "x-protograph-service", service)

	// get endpoints from provider
	endpoints, err := t.opts.Provider.Endpoints(ctx, service)
	if err != nil {
		return
	}
	// pick one with shuffle
	idx := rand.Intn(len(endpoints))
	endpoint := endpoints[idx]

	cc, err := t.getConn(ctx, endpoint)
	if err != nil {
		return
	}
	defer t.returnConn(endpoint, cc)

	start := time.Now()
	eventbus.Publish(ctx, events.GRPCClientStart{Service: service, Method: string(method.Name()), Target: endpoint})
	resp, err = t.invoke(ctx, cc, mthFull, request, method)
	eventbus.Publish(ctx, events.GRPCClientFinish{
		Service:  service,
		Method:   string(method.Name()),
		Target:   endpoint,
		Code:     status.Code(err),
		Err:      err,
		Duration: time.Since(start),
	})
	return
}

func (t *Transport) Close() error {
	if t.closed.Swap(true) {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, p := range t.pools {
		p.close()
	}
	t.pools = map[string]*connPool{}
	return nil
}

// ---------------- internals ----------------

type connPool struct {
	endpoint string
	opts     *Options
	conns    chan *grpc.ClientConn
	once     sync.Once
	closed   atomic.Bool
}

func newConnPool(endpoint string, opts *Options) *connPool {
	n := opts.MaxConnsPerEndpoint
	if n <= 0 {
		n = 2
	}
	return &connPool{
		endpoint: endpoint,
		opts:     opts,
		conns:    make(chan *grpc.ClientConn, n),
	}
}

func (p *connPool) get(ctx context.Context) (*grpc.ClientConn, error) {
	if p.closed.Load() {
		return nil, fmt.Errorf("grpctp: pool closed")
	}
	select {
	case cc := <-p.conns:
		return cc, nil
	default:
		// create new
		cc, err := grpc.DialContext(ctx, p.endpoint, p.opts.DialOptions...)
		if err != nil {
			return nil, err
		}
		return cc, nil
	}
}

func (p *connPool) put(cc *grpc.ClientConn) {
	if cc == nil || p.closed.Load() {
		if cc != nil {
			_ = cc.Close()
		}
		return
	}
	select {
	case p.conns <- cc:
	default:
		_ = cc.Close()
	}
}

func (p *connPool) close() {
	if p.closed.Swap(true) {
		return
	}
	close(p.conns)
	for cc := range p.conns {
		_ = cc.Close()
	}
}

func (t *Transport) getConn(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	t.mu.RLock()
	pool := t.pools[endpoint]
	t.mu.RUnlock()
	if pool == nil {
		t.mu.Lock()
		pool = t.pools[endpoint]
		if pool == nil {
			pool = newConnPool(endpoint, t.opts)
			t.pools[endpoint] = pool
		}
		t.mu.Unlock()
	}
	return pool.get(ctx)
}

func (t *Transport) returnConn(endpoint string, cc *grpc.ClientConn) {
	t.mu.RLock()
	pool := t.pools[endpoint]
	t.mu.RUnlock()
	if pool != nil {
		pool.put(cc)
		return
	}
	_ = cc.Close()
}

func (t *Transport) invoke(ctx context.Context, cc *grpc.ClientConn, fullMethod string, req protoreflect.Message, md protoreflect.MethodDescriptor) (protoreflect.Message, error) {
	// Use dynamicpb to construct response
	resp := dynamicpb.NewMessage(md.Output())
	// We can use the low-level ClientConn.Invoke
	if err := cc.Invoke(ctx, fullMethod, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}
