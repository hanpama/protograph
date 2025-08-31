package executor

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	schema "github.com/hanpama/protograph/internal/schema"
)

// Pattern: Result comparison
func TestOrdering_FieldOutput_Order_Result(t *testing.T) {
	sch := &schema.Schema{
		QueryType: "Query",
		Types: map[string]*schema.Type{
			"Query": {
				Name: "Query",
				Kind: schema.TypeKindObject,
				Fields: []*schema.Field{
					{Name: "a", Type: schema.NamedType("String"), Async: false},
					{Name: "b", Type: schema.NamedType("String"), Async: true},
					{Name: "c", Type: schema.NamedType("String"), Async: false},
				},
			},
			"String": {Name: "String", Kind: schema.TypeKindScalar},
		},
	}
	rt := NewMockRuntime(map[string]MockResolver{
		"Query.a": NewMockValueResolver("A"),
		"Query.b": NewMockValueResolver("B"),
		"Query.c": NewMockValueResolver("C"),
	})
	exec := NewExecutor(rt, sch)
	doc := mustParseQuery(t, "{ a b c }")

	gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
	gotCalls := rt.GetCalls()

	wantRes := &ExecutionResult{Data: map[string]any{"a": "A", "b": "B", "c": "C"}, Errors: []GraphQLError{}}
	if diff := cmp.Diff(wantRes, gotRes); diff != "" {
		t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
	}

	wantCalls := []Call{
		{Kind: "sync", ObjectType: "Query", Field: "a", Source: nil, Args: map[string]any{}, BatchID: 0},
		{Kind: "sync", ObjectType: "Query", Field: "c", Source: nil, Args: map[string]any{}, BatchID: 0},
		{Kind: "async", ObjectType: "Query", Field: "b", Source: nil, Args: map[string]any{}, BatchID: 1},
	}
	if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
		t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
	}
}

// Pattern: Result comparison
func TestOrdering_FragmentMerge_DuplicateFields_Result(t *testing.T) {
	sch := &schema.Schema{
		QueryType: "Query",
		Types: map[string]*schema.Type{
			"Query":  {Name: "Query", Kind: schema.TypeKindObject, Fields: []*schema.Field{{Name: "obj", Type: schema.NamedType("Obj")}}},
			"Obj":    {Name: "Obj", Kind: schema.TypeKindObject, Fields: []*schema.Field{{Name: "a", Type: schema.NamedType("Sub")}}},
			"Sub":    {Name: "Sub", Kind: schema.TypeKindObject, Fields: []*schema.Field{{Name: "x", Type: schema.NamedType("String")}, {Name: "y", Type: schema.NamedType("String")}}},
			"String": {Name: "String", Kind: schema.TypeKindScalar},
		},
	}
	rt := NewMockRuntime(map[string]MockResolver{
		"Query.obj": NewMockValueResolver(map[string]any{}),
		"Obj.a":     NewMockValueResolver(map[string]any{}),
		"Sub.x":     NewMockValueResolver("X"),
		"Sub.y":     NewMockValueResolver("Y"),
	})
	exec := NewExecutor(rt, sch)
	doc := mustParseQuery(t, "{ obj { a { x } a { y } } }")

	gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
	gotCalls := rt.GetCalls()

	wantRes := &ExecutionResult{Data: map[string]any{"obj": map[string]any{"a": map[string]any{"x": "X", "y": "Y"}}}, Errors: []GraphQLError{}}
	if diff := cmp.Diff(wantRes, gotRes); diff != "" {
		t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
	}

	wantCalls := []Call{
		{Kind: "sync", ObjectType: "Query", Field: "obj", Source: nil, Args: map[string]any{}, BatchID: 0},
		{Kind: "sync", ObjectType: "Obj", Field: "a", Source: map[string]any{}, Args: map[string]any{}, BatchID: 0},
		{Kind: "sync", ObjectType: "Sub", Field: "x", Source: map[string]any{}, Args: map[string]any{}, BatchID: 0},
		{Kind: "sync", ObjectType: "Sub", Field: "y", Source: map[string]any{}, Args: map[string]any{}, BatchID: 0},
	}
	if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
		t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
	}
}
