package events

import (
	"time"

	"google.golang.org/grpc/codes"
)

// GRPCClientStart is emitted before a gRPC client call.
type GRPCClientStart struct {
	Service string
	Method  string
	Target  string
}

// GRPCClientFinish is emitted after a gRPC client call completes.
type GRPCClientFinish struct {
	Service  string
	Method   string
	Target   string
	Code     codes.Code
	Err      error
	Duration time.Duration
}
