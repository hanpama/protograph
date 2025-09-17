package executor

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	schema "github.com/hanpama/protograph/internal/schema"
)

// Pattern: Calls comparison (and result where trivial)
func TestRoot_Query_IsAsync_Depth0Batch_Calls(t *testing.T) {
	sch := &schema.Schema{
		QueryType: "Query",
		Types: map[string]*schema.Type{
			"Query":  {Name: "Query", Kind: schema.TypeKindObject, Fields: schema.NewFieldMap(&schema.Field{Name: "a", Type: schema.NamedType("String"), Async: true})},
			"String": {Name: "String", Kind: schema.TypeKindScalar},
		},
	}
	rt := NewMockRuntime(map[string]MockResolver{"Query.a": NewMockValueResolver("A")})
	exec := NewExecutor(rt, sch)
	doc := mustParseQuery(t, "{ a }")

	gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
	gotCalls := rt.GetCalls()

	wantRes := &ExecutionResult{Data: map[string]any{"a": "A"}, Errors: []GraphQLError{}}
	if diff := cmp.Diff(wantRes, gotRes); diff != "" {
		t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
	}
	wantCalls := []Call{{Kind: "async", ObjectType: "Query", Field: "a", Source: nil, Args: map[string]any{}, BatchID: 1}}
	if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
		t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
	}
}

// Pattern: Result comparison
func TestMutation_Serial_Evaluation_Order_Result(t *testing.T) {
	sch := &schema.Schema{
		QueryType:    "Query",
		MutationType: "Mutation",
		Types: map[string]*schema.Type{
			"Query": {Name: "Query", Kind: schema.TypeKindObject},
			"Mutation": {Name: "Mutation", Kind: schema.TypeKindObject, Fields: schema.NewFieldMap(
				&schema.Field{Name: "m1", Type: schema.NamedType("String")},
				&schema.Field{Name: "m2", Type: schema.NamedType("String")},
				&schema.Field{Name: "m3", Type: schema.NamedType("String")},
			)},
			"String": {Name: "String", Kind: schema.TypeKindScalar},
		},
	}
	rt := NewMockRuntime(map[string]MockResolver{
		"Mutation.m1": NewMockValueResolver("1"),
		"Mutation.m2": NewMockErrorResolver(fmt.Errorf("boom")),
		"Mutation.m3": NewMockValueResolver("3"),
	})
	exec := NewExecutor(rt, sch)
	doc := mustParseQuery(t, "mutation { m1 m2 m3 }")

	gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
	gotCalls := rt.GetCalls()

	wantRes := &ExecutionResult{Data: map[string]any{"m1": "1", "m2": nil, "m3": "3"}, Errors: []GraphQLError{{Message: "boom", Path: Path{"m2"}}}}
	if diff := cmp.Diff(wantRes, gotRes); diff != "" {
		t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
	}

	wantCalls := []Call{
		{Kind: "sync", ObjectType: "Mutation", Field: "m1", Source: nil, Args: map[string]any{}, BatchID: 0},
		{Kind: "sync", ObjectType: "Mutation", Field: "m2", Source: nil, Args: map[string]any{}, BatchID: 0},
		{Kind: "sync", ObjectType: "Mutation", Field: "m3", Source: nil, Args: map[string]any{}, BatchID: 0},
	}
	if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
		t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
	}
}
