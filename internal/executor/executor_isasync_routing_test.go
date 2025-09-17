package executor_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	executor "github.com/hanpama/protograph/internal/executor"
	schema "github.com/hanpama/protograph/internal/schema"
)

// Pattern: Calls comparison + Result comparison via go-cmp snapshot workflow
func TestRouting_IsAsync_SyncVsAsync_Calls(t *testing.T) {
	// 1) 스키마 정의: Query { a: String (sync), b: String (async) }
	sch := &schema.Schema{
		QueryType: "Query",
		Types: map[string]*schema.Type{
			"Query": {
				Name: "Query",
				Kind: schema.TypeKindObject,
				Fields: schema.NewFieldMap(
					&schema.Field{Name: "a", Type: schema.NamedType("String"), Async: false},
					&schema.Field{Name: "b", Type: schema.NamedType("String"), Async: true},
				),
			},
			"String": {Name: "String", Kind: schema.TypeKindScalar},
		},
	}

	// 2) Mock Runtime 정의
	rt := executor.NewMockRuntime(map[string]executor.MockResolver{
		"Query.a": executor.NewMockValueResolver("A"),
		"Query.b": executor.NewMockValueResolver("B"),
	})

	// 3) Executor 생성
	exec := executor.NewExecutor(rt, sch)

	// 4) 쿼리 파싱
	doc := mustParseQuery(t, "{ a b }")

	// 5) 실행
	gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
	gotCalls := rt.GetCalls()

	// 6) 스냅샷 기대값
	wantRes := &executor.ExecutionResult{
		Data: map[string]any{
			"a": "A",
			"b": "B",
		},
		Errors: []executor.GraphQLError{},
	}
	if diff := cmp.Diff(wantRes, gotRes); diff != "" {
		t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
	}

	wantCalls := []executor.Call{
		{Kind: "sync", ObjectType: "Query", Field: "a", Source: nil, Args: map[string]any{}, BatchID: 0},
		{Kind: "async", ObjectType: "Query", Field: "b", Source: nil, Args: map[string]any{}, BatchID: 1},
	}
	if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
		t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
	}
}

// Pattern: Snapshot-first (then paste expectations)
func TestRouting_DepthWiseBatch_Invariants_Calls(t *testing.T) {
	// d=1: same-depth async fields aggregated into one batch
	// Schema: type Query { a: String @async, b: String @async }
	sch := &schema.Schema{
		QueryType: "Query",
		Types: map[string]*schema.Type{
			"Query": {
				Name: "Query",
				Kind: schema.TypeKindObject,
				Fields: schema.NewFieldMap(
					&schema.Field{Name: "a", Type: schema.NamedType("String"), Async: true},
					&schema.Field{Name: "b", Type: schema.NamedType("String"), Async: true},
				),
			},
			"String": {Name: "String", Kind: schema.TypeKindScalar},
		},
	}

	rt := executor.NewMockRuntime(map[string]executor.MockResolver{
		"Query.a": executor.NewMockValueResolver("A"),
		"Query.b": executor.NewMockValueResolver("B"),
	})
	exec := executor.NewExecutor(rt, sch)
	doc := mustParseQuery(t, "{ a b }")

	gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
	gotCalls := rt.GetCalls()

	// Pasted expectations from initial diff
	wantRes := &executor.ExecutionResult{
		Data:   map[string]any{"a": "A", "b": "B"},
		Errors: []executor.GraphQLError{},
	}
	if diff := cmp.Diff(wantRes, gotRes); diff != "" {
		t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
	}

	wantCalls := []executor.Call{
		{Kind: "async", ObjectType: "Query", Field: "a", Source: nil, Args: map[string]any{}, BatchID: 1},
		{Kind: "async", ObjectType: "Query", Field: "b", Source: nil, Args: map[string]any{}, BatchID: 1},
	}
	if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
		t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
	}

	// d=2: two async layers (root async -> nested async)
	// Schema:
	//   type Query { root: Node @async }
	//   type Node  { x: String @async }
	sch2 := &schema.Schema{
		QueryType: "Query",
		Types: map[string]*schema.Type{
			"Query":  {Name: "Query", Kind: schema.TypeKindObject, Fields: schema.NewFieldMap(&schema.Field{Name: "root", Type: schema.NamedType("Node"), Async: true})},
			"Node":   {Name: "Node", Kind: schema.TypeKindObject, Fields: schema.NewFieldMap(&schema.Field{Name: "x", Type: schema.NamedType("String"), Async: true})},
			"String": {Name: "String", Kind: schema.TypeKindScalar},
		},
	}
	rt2 := executor.NewMockRuntime(map[string]executor.MockResolver{
		"Query.root": func(ctx context.Context, src any, args map[string]any) (any, error) {
			return map[string]any{"id": "r"}, nil
		},
		"Node.x": executor.NewMockValueResolver("X"),
	})
	exec2 := executor.NewExecutor(rt2, sch2)
	doc2 := mustParseQuery(t, "{ root { x } }")

	gotRes2 := exec2.ExecuteRequest(context.Background(), doc2, "", nil, nil)
	gotCalls2 := rt2.GetCalls()

	wantRes2 := &executor.ExecutionResult{
		Data:   map[string]any{"root": map[string]any{"x": "X"}},
		Errors: []executor.GraphQLError{},
	}
	if diff := cmp.Diff(wantRes2, gotRes2); diff != "" {
		t.Fatalf("d2 result mismatch (-want +got):\n%s", diff)
	}
	wantCalls2 := []executor.Call{
		{Kind: "async", ObjectType: "Query", Field: "root", Source: nil, Args: map[string]any{}, BatchID: 1},
		{Kind: "async", ObjectType: "Node", Field: "x", Source: map[string]any{"id": "r"}, Args: map[string]any{}, BatchID: 2},
	}
	if diff := cmp.Diff(wantCalls2, gotCalls2); diff != "" {
		t.Fatalf("d2 calls mismatch (-want +got):\n%s", diff)
	}

	// d=3: three async layers (root async -> child async -> grandchild async)
	// Schema:
	//   type Query { root: Node @async }
	//   type Node  { child: Node @async, x: String @async }
	sch3 := &schema.Schema{
		QueryType: "Query",
		Types: map[string]*schema.Type{
			"Query": {Name: "Query", Kind: schema.TypeKindObject, Fields: schema.NewFieldMap(&schema.Field{Name: "root", Type: schema.NamedType("Node"), Async: true})},
			"Node": {Name: "Node", Kind: schema.TypeKindObject, Fields: schema.NewFieldMap(
				&schema.Field{Name: "child", Type: schema.NamedType("Node"), Async: true},
				&schema.Field{Name: "x", Type: schema.NamedType("String"), Async: true},
			)},
			"String": {Name: "String", Kind: schema.TypeKindScalar},
		},
	}
	rt3 := executor.NewMockRuntime(map[string]executor.MockResolver{
		"Query.root": func(ctx context.Context, src any, args map[string]any) (any, error) {
			return map[string]any{"id": "r"}, nil
		},
		"Node.child": func(ctx context.Context, src any, args map[string]any) (any, error) {
			return map[string]any{"id": "c"}, nil
		},
		"Node.x": executor.NewMockValueResolver("X"),
	})
	exec3 := executor.NewExecutor(rt3, sch3)
	doc3 := mustParseQuery(t, "{ root { child { x } } }")

	gotRes3 := exec3.ExecuteRequest(context.Background(), doc3, "", nil, nil)
	gotCalls3 := rt3.GetCalls()

	wantRes3 := &executor.ExecutionResult{
		Data: map[string]any{
			"root": map[string]any{
				"child": map[string]any{"x": "X"},
			},
		},
		Errors: []executor.GraphQLError{},
	}
	if diff := cmp.Diff(wantRes3, gotRes3); diff != "" {
		t.Fatalf("d3 result mismatch (-want +got):\n%s", diff)
	}
	wantCalls3 := []executor.Call{
		{Kind: "async", ObjectType: "Query", Field: "root", Source: nil, Args: map[string]any{}, BatchID: 1},
		{Kind: "async", ObjectType: "Node", Field: "child", Source: map[string]any{"id": "r"}, Args: map[string]any{}, BatchID: 2},
		{Kind: "async", ObjectType: "Node", Field: "x", Source: map[string]any{"id": "c"}, Args: map[string]any{}, BatchID: 3},
	}
	if diff := cmp.Diff(wantCalls3, gotCalls3); diff != "" {
		t.Fatalf("d3 calls mismatch (-want +got):\n%s", diff)
	}
}
