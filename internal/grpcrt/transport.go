package grpcrt

import (
	"context"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// Transport handles the actual gRPC communication.
// This interface allows for different transport implementations (real gRPC, mock, etc.).
// Implementations MUST be safe for concurrent use: grpcrt may invoke Call from
// multiple goroutines when executing independent groups in parallel.
//
// Provided implementations:
// - internal/grpctp.Transport: production-ready client with pooling and timeouts
// - internal/grpcrt/transport_test.go: test fakes/mocks
type Transport interface {
	// Call executes a single gRPC method call.
	Call(ctx context.Context, method protoreflect.MethodDescriptor, request protoreflect.Message) (protoreflect.Message, error)
}
