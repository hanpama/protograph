package otel

import (
	"context"
	"sync"

	eventbus "github.com/hanpama/protograph/internal/eventbus"
	events "github.com/hanpama/protograph/internal/events"
	reqid "github.com/hanpama/protograph/internal/reqid"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// Setup configures OpenTelemetry and attaches eventbus subscribers.
// If endpoint is empty, no telemetry is configured.
func Setup(endpoint, service string) (func(context.Context) error, error) {
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	exp, err := otlptracegrpc.New(context.Background(),
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithDialOption(grpc.WithInsecure()))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(service),
		)),
	)
	otel.SetTracerProvider(tp)

	sub := &subscriber{tracer: otel.Tracer("protograph")}
	sub.register()

	return tp.Shutdown, nil
}

type subscriber struct {
	tracer    trace.Tracer
	httpSpans sync.Map // rid -> trace.Span
	gqlSpans  sync.Map // rid -> trace.Span
	grpcSpans sync.Map // rid -> trace.Span
}

func (s *subscriber) register() {
	eventbus.Subscribe(func(ctx context.Context, e events.HTTPStart) {
		rid, _ := reqid.FromContext(ctx)
		_, span := s.tracer.Start(ctx, "http.request")
		span.SetAttributes(
			semconv.HTTPMethodKey.String(e.Request.Method),
			attribute.String("http.target", e.Request.URL.Path),
		)
		s.httpSpans.Store(rid, span)
	})

	eventbus.Subscribe(func(ctx context.Context, e events.HTTPFinish) {
		rid, _ := reqid.FromContext(ctx)
		v, ok := s.httpSpans.LoadAndDelete(rid)
		if !ok {
			return
		}
		span := v.(trace.Span)
		span.SetAttributes(semconv.HTTPStatusCodeKey.Int(e.Status))
		span.End()
	})

	eventbus.Subscribe(func(ctx context.Context, e events.GraphQLStart) {
		rid, _ := reqid.FromContext(ctx)
		parent := ctx
		if v, ok := s.httpSpans.Load(rid); ok {
			parent = trace.ContextWithSpan(ctx, v.(trace.Span))
		}
		_, span := s.tracer.Start(parent, "graphql.operation")
		span.SetAttributes(
			attribute.String("graphql.operation.name", e.OperationName),
			attribute.String("graphql.operation.type", e.OperationType),
		)
		s.gqlSpans.Store(rid, span)
	})

	eventbus.Subscribe(func(ctx context.Context, e events.GraphQLFinish) {
		rid, _ := reqid.FromContext(ctx)
		v, ok := s.gqlSpans.LoadAndDelete(rid)
		if !ok {
			return
		}
		span := v.(trace.Span)
		span.SetAttributes(attribute.Int("graphql.error_count", len(e.Errors)))
		span.End()
	})

	eventbus.Subscribe(func(ctx context.Context, e events.GRPCClientStart) {
		rid, _ := reqid.FromContext(ctx)
		parent := ctx
		if v, ok := s.gqlSpans.Load(rid); ok {
			parent = trace.ContextWithSpan(ctx, v.(trace.Span))
		} else if v, ok := s.httpSpans.Load(rid); ok {
			parent = trace.ContextWithSpan(ctx, v.(trace.Span))
		}
		_, span := s.tracer.Start(parent, "grpc.client")
		span.SetAttributes(
			semconv.RPCServiceKey.String(e.Service),
			semconv.RPCMethodKey.String(e.Method),
			attribute.String("net.peer.name", e.Target),
		)
		s.grpcSpans.Store(rid, span)
	})

	eventbus.Subscribe(func(ctx context.Context, e events.GRPCClientFinish) {
		rid, _ := reqid.FromContext(ctx)
		v, ok := s.grpcSpans.LoadAndDelete(rid)
		if !ok {
			return
		}
		span := v.(trace.Span)
		span.SetAttributes(attribute.String("grpc.code", e.Code.String()))
		if e.Err != nil {
			span.RecordError(e.Err)
		}
		span.End()
	})
}
