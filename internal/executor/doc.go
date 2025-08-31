// Package executor implements a breadth-first, batch-friendly GraphQL executor
// with explicit runtime hooks for synchronous resolution, depth-wise batching of
// asynchronous work, abstract-type resolution, and leaf serialization.
//
// # Overview
//
// The executor follows a level-by-level (BFS) execution model designed to:
//   - Expand synchronous ("physical") fields immediately without adding batch depth.
//   - Collect asynchronous ("remote") fields encountered at the current depth and
//     resolve them in a single call to Runtime.BatchResolveAsync.
//   - Complete values according to the GraphQL specification (lists, leafs,
//     objects, abstract types), including Non-Null null-propagation rules.
//   - Accumulate located errors while allowing partial success.
//
// # Preparation
//
// Before execution, the executor:
//  1. Validates the schema (assumed by the caller) and chooses the operation
//     (by name or by uniqueness when unnamed).
//  2. Coerces variables from the provided input against operation variable
//     definitions, producing a variableValues map. Errors here stop execution.
//  3. Builds an execution context: schema, document, operation, coerced
//     variables, root value, and the injected Runtime implementation.
//  4. Determines the root object type from the operation (Query/Mutation/Subscription)
//     and collects the root selection set.
//
// # Execution Model
//
// The executor models work in three conceptual sets:
//   - Frontier: the currently reachable synchronous work; it expands downward
//     immediately and does not increment depth.
//   - PendingTasks: asynchronous field resolutions discovered while expanding
//     this depth; they are batched and executed exactly once per depth.
//   - ResultStore: a mutable response tree where completed values are written at
//     their response paths.
//
// A field instance (Node) is characterized by its parent object type, field
// definition, AST field nodes, response path, source value, coerced arguments,
// and return type. The executor classifies a field as synchronous or
// asynchronous as follows:
//
//   - Synchronous ("physical") fields are resolvable without remote I/O and are
//     executed immediately via Runtime.ResolveSync.
//   - Asynchronous ("remote") fields require a resolver/loader call and are
//     queued to be resolved in batch via Runtime.BatchResolveAsync.
//
// In this implementation, the schema conveys this classification via
// schema.Field.Async. Schema construction should set Async=false for physical
// projection fields (e.g., fields backed directly by the source value without
// network calls) and Async=true for resolver/loader-backed fields (including
// root fields with resolvers). This mirrors the physical vs remote rules in the
// specification.
//
// BFS Loop (per depth)
//
// The executor repeats the following cycle until both the frontier and the
// pending async tasks are empty:
//
//	A. Sync expansion
//	   - For each field in the current selection set, compute argument values and
//	     determine its return type and async flag.
//	   - If sync: call Runtime.ResolveSync, then completeValue immediately.
//	     If the result is an object, collect its subfields and keep expanding
//	     synchronously (depth does not increase). If the result is a list/leaf/
//	     null, write it to the response tree.
//	   - If async: create an AsyncResolveTask and enqueue it in the current
//	     depth’s PendingTasks without executing yet.
//
//	B. Batch execution
//	   - If there are async tasks at this depth, call Runtime.BatchResolveAsync
//	     exactly once with all of them (after filtering out any paths nullified
//	     by prior Non-Null violations). The runtime must return one result per
//	     task, in the same order.
//	   - For each result, run completeValue. If it yields new object subfields,
//	     those subfields are collected for the next depth; their async children
//	     are queued only for the next batch (preserving depth boundaries).
//
//	C. Non-Null propagation and pruning
//	   - A Non-Null violation at path p sets the nearest nullable ancestor to
//	     null and marks that ancestor path as a tombstone. Any queued tasks
//	     under that path are dropped. Errors are recorded as located errors.
//
//	D. Advance depth
//	   - Move to the next depth with the subfield frontier gathered from object
//	     completions and the async tasks queued at that depth.
//
// A core invariant is preserved: for a graph with asynchronous depth d,
// BatchResolveAsync is invoked exactly d times. Purely synchronous descents do
// not increase d.
//
// # Value Completion
//
// The executor implements GraphQL value completion using runtime hooks:
//   - Non-Null: unwrap and complete the inner type. If the inner completion
//     produced null, record a Non-Null violation and propagate null upwards.
//   - Null: nil results produce GraphQL null.
//   - List: complete each element recursively with index-aware paths. A null
//     element for a Non-Null inner type nullifies the entire list value.
//   - Leaf (Scalar/Enum): defer to Runtime.SerializeLeafValue to produce a
//     JSON-safe Go value.
//   - Abstract (Interface/Union): defer to Runtime.ResolveType to determine the
//     concrete object type, validate it against the schema, then complete as an
//     object.
//   - Object: collect subfields; execute sync fields immediately and queue async
//     subfields for the current or next depth as defined above.
//
// # Errors and Partial Success
//
// Errors are accumulated as located GraphQL errors (message + path). For a
// Non-Null field, a null result or error triggers propagation to the nearest
// nullable ancestor; otherwise, the field value is set to null and execution
// continues. Batch results are independent, enabling partial success within a
// single batch call.
//
// # Runtime Contract
//
// The Runtime interface abstracts host integration:
//   - ResolveSync: resolve synchronous fields immediately.
//   - BatchResolveAsync: resolve one depth’s async fields in a single, ordered
//     batch, returning exactly one result per task.
//   - ResolveType: resolve concrete object type names for interface/union values.
//   - SerializeLeafValue: serialize scalars and enums to JSON-safe Go values.
//
// See runtime.go for detailed method contracts and guidance (ordering, partial
// success, cancellation via context, batching strategies).
//
// Notes and Alignment
//
//   - Async detection: This package relies on schema.Field.Async to indicate
//     whether a field is synchronous (physical) or asynchronous (remote). Schema
//     construction should apply the physical-vs-remote rule so that physical
//     projection fields are marked Async=false and resolver/loader-backed fields
//     are marked Async=true.
//   - Mutation: While the specification note suggests mutations may be modeled
//     as all-sync, the executor does not enforce this; async mutation fields are
//     supported if the schema marks them Async=true.
//   - Fragments on abstract types: field collection currently matches inline
//     fragment and fragment spread type conditions only when they equal the
//     concrete object type name. Support for interface/union subtype matching is
//     a known gap to address at collection time.
//   - Cancellation: The executor prunes queued tasks under paths nullified by
//     Non-Null propagation to avoid unnecessary runtime work.
package executor
