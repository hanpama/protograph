package executor

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	schema "github.com/hanpama/protograph/internal/schema"
)

// Pattern: Calls comparison
func TestRuntimeContract_Routing_SyncVsBatch_Calls(t *testing.T) {
	sch := newSchemaWithQueryType(
		newObjectType(
			"Query",
			schema.NewField("a", "", schema.NamedType("String")),
			schema.NewField("b", "", schema.NamedType("String")).SetAsync(true),
		),
		newScalarType("String"),
	)
	rt := NewMockRuntime(map[string]MockResolver{
		"Query.a": NewMockValueResolver("A"),
		"Query.b": NewMockValueResolver("B"),
	})
	exec := NewExecutor(rt, sch)
	doc := mustParseQuery(t, "{ a b }")

	_ = exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
	gotCalls := rt.GetCalls()

	wantCalls := []Call{
		{Kind: "sync", ObjectType: "Query", Field: "a", Source: nil, Args: map[string]any{}, BatchID: 0},
		{Kind: "async", ObjectType: "Query", Field: "b", Source: nil, Args: map[string]any{}, BatchID: 1},
	}
	if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
		t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
	}
}

// Pattern: Calls comparison
func TestRuntimeContract_PayloadTransparency_Calls(t *testing.T) {
	sch := newSchemaWithQueryType(
		newObjectType("Query", schema.NewField("obj", "", schema.NamedType("Obj"))),
		newObjectType(
			"Obj",
			schema.NewField("a", "", schema.NamedType("String")).
				AddArgument(schema.NewInputValue("arg", "", schema.NamedType("String"))),
		),
		newScalarType("String"),
	)
	rt := NewMockRuntime(map[string]MockResolver{
		"Query.obj": NewMockValueResolver(map[string]any{"token": "root"}),
		"Obj.a":     func(ctx context.Context, src any, args map[string]any) (any, error) { return "A", nil },
	})
	exec := NewExecutor(rt, sch)
	doc := mustParseQuery(t, "{ obj { a(arg: \"val\") } }")

	_ = exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
	gotCalls := rt.GetCalls()

	wantCalls := []Call{
		{Kind: "sync", ObjectType: "Query", Field: "obj", Source: nil, Args: map[string]any{}, BatchID: 0},
		{Kind: "sync", ObjectType: "Obj", Field: "a", Source: map[string]any{"token": "root"}, Args: map[string]any{"arg": "val"}, BatchID: 0},
	}
	if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
		t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
	}
}

// Pattern: Calls comparison
func TestRuntimeContract_BatchBoundary_SingleBatchPerDepth_Calls(t *testing.T) {
	sch := newSchemaWithQueryType(
		newObjectType(
			"Query",
			schema.NewField("a", "", schema.NamedType("String")).SetAsync(true),
			schema.NewField("b", "", schema.NamedType("String")).SetAsync(true),
		),
		newScalarType("String"),
	)
	rt := NewMockRuntime(map[string]MockResolver{
		"Query.a": NewMockValueResolver("A"),
		"Query.b": NewMockValueResolver("B"),
	})
	exec := NewExecutor(rt, sch)
	doc := mustParseQuery(t, "{ a b }")

	_ = exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
	gotCalls := rt.GetCalls()

	wantCalls := []Call{
		{Kind: "async", ObjectType: "Query", Field: "a", Source: nil, Args: map[string]any{}, BatchID: 1},
		{Kind: "async", ObjectType: "Query", Field: "b", Source: nil, Args: map[string]any{}, BatchID: 1},
	}
	if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
		t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
	}
}

// Pattern: Calls + Result comparison
func TestRuntimeContract_HookInvocation_Serialize_ResolveType_CallsAndResult(t *testing.T) {
	nodeType := schema.NewType("Node", schema.TypeKindInterface, "").AddPossibleType("Obj")
	objType := newObjectType("Obj", schema.NewField("a", "", schema.NamedType("String")))
	objType.AddInterface("Node")

	sch := newSchemaWithQueryType(
		newObjectType("Query", schema.NewField("iface", "", schema.NamedType("Node"))),
		nodeType,
		objType,
		newScalarType("String"),
	)
	rt := NewMockRuntime(map[string]MockResolver{
		"Query.iface": NewMockValueResolver(map[string]any{}),
		"Obj.a":       NewMockValueResolver("A"),
	})

	typeCount := 0
	serializerCount := 0
	SetTypeResolver(rt, func(value any) (string, error) { typeCount++; return "Obj", nil })
	SetSerializer(rt, func(val any, t schema.TypeRef) (any, error) { serializerCount++; return val.(string) + "!", nil })

	exec := NewExecutor(rt, sch)
	doc := mustParseQuery(t, "{ iface { a } }")

	gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
	gotCalls := rt.GetCalls()

	wantRes := &ExecutionResult{Data: map[string]any{"iface": map[string]any{"a": "A!"}}, Errors: []GraphQLError{}}
	if diff := cmp.Diff(wantRes, gotRes); diff != "" {
		t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
	}
	wantCalls := []Call{
		{Kind: "sync", ObjectType: "Query", Field: "iface", Source: nil, Args: map[string]any{}, BatchID: 0},
		{Kind: "sync", ObjectType: "Obj", Field: "a", Source: map[string]any{}, Args: map[string]any{}, BatchID: 0},
	}
	if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
		t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
	}
	if typeCount != 1 || serializerCount != 1 {
		t.Fatalf("hook counts wrong type=%d serializer=%d", typeCount, serializerCount)
	}
}

// Pattern: Calls + Result comparison
func TestRuntimeContract_CancellationTimeouts_PartialFailure_CallsAndResult(t *testing.T) {
	sch := newSchemaWithQueryType(
		newObjectType(
			"Query",
			schema.NewField("a", "", schema.NamedType("String")).SetAsync(true),
			schema.NewField("b", "", schema.NamedType("String")).SetAsync(true),
		),
		newScalarType("String"),
	)
	rt := NewMockRuntime(map[string]MockResolver{
		"Query.a": NewMockErrorResolver(fmt.Errorf("boom")),
		"Query.b": NewMockValueResolver("B"),
	})
	exec := NewExecutor(rt, sch)
	doc := mustParseQuery(t, "{ a b }")

	gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
	gotCalls := rt.GetCalls()

	wantRes := &ExecutionResult{Data: map[string]any{"a": nil, "b": "B"}, Errors: []GraphQLError{{Message: "boom", Path: Path{"a"}}}}
	if diff := cmp.Diff(wantRes, gotRes); diff != "" {
		t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
	}
	wantCalls := []Call{
		{Kind: "async", ObjectType: "Query", Field: "a", Source: nil, Args: map[string]any{}, BatchID: 1},
		{Kind: "async", ObjectType: "Query", Field: "b", Source: nil, Args: map[string]any{}, BatchID: 1},
	}
	if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
		t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
	}
}
