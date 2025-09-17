package executor

import (
	"context"
	"fmt"
	"strings"
	"sync"

	schema "github.com/hanpama/protograph/internal/schema"
)

// MockResolver resolves a single item; MockRuntime adapts it for batched calls in tests.
type MockResolver func(ctx context.Context, source any, args map[string]any) (any, error)

// CallKind identifies whether a call was from Resolve (sync) or Batch (async).
const (
	CallKindSync  = "sync"
	CallKindAsync = "async"
)

// NewMockValueResolver returns a MockResolver that always returns the provided value.
func NewMockValueResolver(val any) MockResolver {
	return func(ctx context.Context, source any, args map[string]any) (any, error) {
		return val, nil
	}
}

// NewMockErrorResolver returns a MockResolver that always returns the provided error.
func NewMockErrorResolver(err error) MockResolver {
	return func(ctx context.Context, source any, args map[string]any) (any, error) {
		return nil, err
	}
}

// Call represents a single task-level invocation record.
// Sync and async both record one Call per item. Async calls share the same BatchID within a flush.
type Call struct {
	Kind       string
	ObjectType string
	Field      string
	Source     any
	Args       map[string]any
	BatchID    int // >0 for async items in the same batch, 0 for sync
}

// MockRuntime implements Runtime with a single resolver registry and a single call log.
type MockRuntime struct {
	mu        sync.Mutex
	resolvers map[string]MockResolver
	calls     []Call
	batchSeq  int // increments per Batch call

	typeResolver func(value any) (string, error)
	serializer   func(val any, t schema.TypeRef) (any, error)
}

// NewMockRuntime creates a MockRuntime with the provided resolvers.
// The resolvers map keys are of the form "ObjectType.Field".
func NewMockRuntime(resolvers map[string]MockResolver) *MockRuntime {
	m := &MockRuntime{
		resolvers: make(map[string]MockResolver),
		typeResolver: func(value any) (string, error) {
			if m, ok := value.(map[string]any); ok {
				if typename, ok := m["__typename"].(string); ok {
					return typename, nil
				}
			}
			return "", fmt.Errorf("cannot resolve type")
		},
		serializer: func(val any, t schema.TypeRef) (any, error) {
			return val, nil
		},
	}
	m.mu.Lock()
	for k, v := range resolvers {
		m.resolvers[k] = v
	}
	m.mu.Unlock()
	return m
}

// SetResolver registers or updates a resolver for the given object type and field.
func (m *MockRuntime) SetResolver(objectType, field string, resolver MockResolver) {
	key := objectType + "." + field
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.resolvers == nil {
		m.resolvers = make(map[string]MockResolver)
	}
	m.resolvers[key] = resolver
}

func SetTypeResolver(r Runtime, f func(value any) (string, error)) {
	if mr, ok := r.(*MockRuntime); ok {
		mr.mu.Lock()
		mr.typeResolver = f
		mr.mu.Unlock()
	}
}

func SetSerializer(r Runtime, f func(val any, t schema.TypeRef) (any, error)) {
	if mr, ok := r.(*MockRuntime); ok {
		mr.mu.Lock()
		mr.serializer = f
		mr.mu.Unlock()
	}
}

// ResolveSync implements Runtime.Resolve by invoking the resolver with a single item.
func (m *MockRuntime) ResolveSync(ctx context.Context, objectType string, field string, source any, args map[string]any) (any, error) {
	key := objectType + "." + field

	m.mu.Lock()
	r := m.resolvers[key]
	m.mu.Unlock()

	var out AsyncResolveResult
	if r != nil {
		val, err := r(ctx, source, args)
		out = AsyncResolveResult{Value: val, Error: err}
	} else {
		out = AsyncResolveResult{}
	}

	// Log per-task call (sync => BatchID 0)
	m.mu.Lock()
	m.calls = append(m.calls, Call{
		Kind:       CallKindSync,
		ObjectType: objectType,
		Field:      field,
		Source:     source,
		Args:       args,
		BatchID:    0,
	})
	m.mu.Unlock()

	if out.Error != nil {
		return nil, out.Error
	}
	return out.Value, nil
}

// BatchResolveAsync implements Runtime.Batch with stable, order-preserving grouping by (objectType, field).
func (m *MockRuntime) BatchResolveAsync(ctx context.Context, tasks []AsyncResolveTask) []AsyncResolveResult {
	if len(tasks) == 0 {
		return nil
	}

	// Build order-preserving groups
	type group struct {
		key     string
		indices []int
	}
	groups := make([]group, 0)
	indexByKey := make(map[string]int)
	for i, t := range tasks {
		key := t.ObjectType + "." + t.Field
		if gi, ok := indexByKey[key]; ok {
			groups[gi].indices = append(groups[gi].indices, i)
		} else {
			indexByKey[key] = len(groups)
			groups = append(groups, group{key: key, indices: []int{i}})
		}
	}

	results := make([]AsyncResolveResult, len(tasks))

	m.mu.Lock()
	m.batchSeq++
	batchID := m.batchSeq
	m.mu.Unlock()

	// Execute groups in first-appearance order
	for _, g := range groups {
		// Gather inputs
		sources := make([]any, len(g.indices))
		args := make([]map[string]any, len(g.indices))
		for i, idx := range g.indices {
			sources[i] = tasks[idx].Source
			args[i] = tasks[idx].Args
		}

		m.mu.Lock()
		r := m.resolvers[g.key]
		m.mu.Unlock()

		groupResults := make([]AsyncResolveResult, len(g.indices))
		for i := range g.indices {
			if r != nil {
				val, err := r(ctx, sources[i], args[i])
				groupResults[i] = AsyncResolveResult{Value: val, Error: err}
			} else {
				groupResults[i] = AsyncResolveResult{}
			}
		}

		obj, fld := splitKey(g.key)

		// Map results back by index and log per item
		for i, idx := range g.indices {
			if i < len(groupResults) {
				results[idx] = groupResults[i]
			} else {
				results[idx] = AsyncResolveResult{}
			}

			// log each task as a separate Call
			m.mu.Lock()
			m.calls = append(m.calls, Call{
				Kind:       CallKindAsync,
				ObjectType: obj,
				Field:      fld,
				Source:     sources[i],
				Args:       args[i],
				BatchID:    batchID,
			})
			m.mu.Unlock()
		}
	}

	return results
}

// ResolveType implements Runtime.ResolveType
func (m *MockRuntime) ResolveType(ctx context.Context, abstractType string, value any) (string, error) {
	if m.typeResolver == nil {
		return "", fmt.Errorf("type resolver not configured")
	}
	return m.typeResolver(value)
}

// ResolveUnionConcreteValue implements Runtime.ResolveUnionConcreteValue.
func (m *MockRuntime) ResolveUnionConcreteValue(ctx context.Context, unionTypeName string, value any) (any, error) {
	return value, nil
}

// ResolveInterfaceConcreteValue implements Runtime.ResolveInterfaceConcreteValue.
func (m *MockRuntime) ResolveInterfaceConcreteValue(ctx context.Context, interfaceTypeName string, value any) (any, error) {
	return value, nil
}

// SerializeLeafValue implements Runtime.SerializeLeafValue
func (m *MockRuntime) SerializeLeafValue(ctx context.Context, scalarOrEnumTypeName string, value any) (any, error) {
	if m.serializer == nil {
		return value, nil
	}
	return m.serializer(value, *schema.NamedType(scalarOrEnumTypeName))
}

// GetCalls returns a copy of the recorded calls in order.
func (m *MockRuntime) GetCalls() []Call {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Call, len(m.calls))
	copy(out, m.calls)
	return out
}

// Reset clears recorded calls and counters (resolvers remain).
func (m *MockRuntime) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
	m.batchSeq = 0
}

// Helpers
func splitKey(key string) (string, string) {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return key, ""
}
