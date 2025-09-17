package executor

import (
	"context"
)

// Runtime defines the host integration surface for field resolution, batching,
// abstract type resolution, and leaf-value serialization used by the Executor.
//
// General contract
//   - The Executor performs a breadth-first execution. At each depth it drains all
//     synchronous fields first via ResolveSync, then calls BatchResolveAsync ONCE
//     with all async tasks collected at that depth. The next depth does not begin
//     until BatchResolveAsync returns and those results are completed.
//   - The Executor guarantees that ResolveSync is never invoked for fields marked
//     async, and BatchResolveAsync is only invoked when there is at least one
//     async field at the current depth.
//   - Errors returned from any method are converted into located GraphQL errors.
//     If the field’s return type is Non-Null, the Executor will propagate the
//     null up to the nearest nullable ancestor per GraphQL spec.
//   - Implementations should be stateless or otherwise concurrency-safe. The
//     Executor may call these methods concurrently for different operations.
//   - Implementations must not mutate source or args values.
//
// Object/field identifiers
// - objectType is the GraphQL type name (e.g. "User").
// - field is the GraphQL field name on that type (e.g. "posts").
// - For root fields, objectType is the root type name (e.g. "Query").
// - source is the parent object value (nil for root).
// - args is the map of argument names to already-coerced Go values.
//
// Abstract types and leaf values
//   - ResolveType must return the concrete type name for interface/union values.
//   - SerializeLeafValue must coerce/serialize scalars and enums into JSON-safe
//     Go values (string, float64, int32/int64, bool, []byte as base64 string, etc.).
//     For enums, return the enum name as string.
//
// Partial success and determinism
//   - BatchResolveAsync must return one AsyncResolveResult per task. Each result
//     is independent; failures in one do not affect others. The Executor supports
//     partial success.
//   - Results MUST be returned in the same order as the input tasks. The result at
//     index i corresponds to the task at index i (results[i] corresponds to tasks[i]).
//
// Cancellation
//   - The Executor filters out tasks whose response paths were nullified by a
//     Non-Null violation, so BatchResolveAsync receives only live tasks. The
//     runtime implementation does not need to handle cancellation tokens beyond
//     respecting ctx.
//
// Performance guidance
//   - Implementations should batch by (objectType, field) and backend affinity.
//   - For highly parallel backends, you can fan out internally but still return a
//     single []AsyncResolveResult to the Executor.
//
// Idempotency
//   - Calls should be idempotent where practical; the Executor will not retry on
//     its own, but client-side retries may occur at higher layers.
//
// Transport coupling
//   - The Runtime interface is transport-agnostic. A gRPC-backed runtime should
//     translate these contracts into the appropriate protobuf RPCs and messages.
type Runtime interface {
	// ResolveSync resolves a synchronous field value immediately.
	//
	// Called only for fields declared as sync (Async == false). This should
	// perform any required computation synchronously and return the raw value
	// to be completed by the Executor (including nested selection sets).
	// Return (nil, nil) to produce a GraphQL null for nullable fields.
	ResolveSync(ctx context.Context, objectType string, field string, source any, args map[string]any) (any, error)

	// BatchResolveAsync resolves one execution depth of async field tasks.
	//
	// The Executor calls this exactly once per depth with all async tasks
	// collected at that depth (after draining sync paths). Implementations may
	// further batch/group by (objectType, field) or backend-specific keys.
	//
	// Requirements:
	// - Return len(results) == len(tasks).
	// - Results MUST maintain the same order as tasks (results[i] corresponds to tasks[i]).
	// - Return independent errors per element without failing the whole batch.
	BatchResolveAsync(ctx context.Context, tasks []AsyncResolveTask) []AsyncResolveResult

	// ResolveType determines the concrete runtime type name for a value of an
	// abstract GraphQL type (interface or union).
	//
	// Must return a type name that is a possible type of the abstractType in the
	// provided schema; otherwise return an error.
	ResolveType(ctx context.Context, abstractType string, value any) (string, error)

	// ResolveUnionConcreteValue converts a union envelope value into its concrete
	// representation prior to completion.
	ResolveUnionConcreteValue(ctx context.Context, unionTypeName string, value any) (any, error)

	// ResolveInterfaceConcreteValue converts an interface envelope value into its
	// concrete representation prior to completion.
	ResolveInterfaceConcreteValue(ctx context.Context, interfaceTypeName string, value any) (any, error)

	// SerializeLeafValue serializes a scalar or enum value to a JSON-safe Go
	// value according to the GraphQL schema and custom scalar mappings.
	//
	// For enums, return the symbolic name as string. For scalars, return the
	// appropriate Go type (e.g. float64 for Float, int32 for Int unless mapped,
	// string for String/ID, bool for Boolean). For custom scalars, apply any
	// runtime-defined encoding; bytes should be base64-encoded strings.
	SerializeLeafValue(ctx context.Context, scalarOrEnumTypeName string, value any) (any, error)
}

type AsyncResolveTask struct {
	// ObjectType is the parent GraphQL object type name for the field.
	ObjectType string
	// Field is the GraphQL field name to resolve.
	Field string
	// Source is the parent object value (nil for root fields).
	Source any
	// Args are the field arguments, coerced to Go values per the schema.
	Args map[string]any
}

type AsyncResolveResult struct {
	// Value is the resolved raw value prior to completion, or nil on error.
	Value any
	// Error contains a failure specific to this element; other elements in the
	// same batch are unaffected.
	Error error
}
