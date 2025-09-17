package executor_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	executor "github.com/hanpama/protograph/internal/executor"
	schema "github.com/hanpama/protograph/internal/schema"
)

// Pattern: Result comparison
func TestCompleteValue_NonNull_Propagation_Result(t *testing.T) {
	t.Run("Resolver error", func(t *testing.T) {
		// Schema: type Query { obj: Obj! }
		//         type Obj { a: String! b: String! @async }
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {
					Name:   "Query",
					Kind:   schema.TypeKindObject,
					Fields: schema.NewFieldMap(&schema.Field{Name: "obj", Type: schema.NonNullType(schema.NamedType("Obj"))}),
				},
				"Obj": {
					Name: "Obj",
					Kind: schema.TypeKindObject,
					Fields: schema.NewFieldMap(
						&schema.Field{Name: "a", Type: schema.NonNullType(schema.NamedType("String")), Async: false},
						&schema.Field{Name: "b", Type: schema.NonNullType(schema.NamedType("String")), Async: true},
					),
				},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}

		// Runtime: obj resolves to empty map; a errors; b would return "B" but must be cancelled.
		rt := executor.NewMockRuntime(map[string]executor.MockResolver{
			"Query.obj": executor.NewMockValueResolver(map[string]any{}),
			"Obj.a":     executor.NewMockErrorResolver(fmt.Errorf("boom")),
			"Obj.b":     executor.NewMockValueResolver("B"),
		})

		exec := executor.NewExecutor(rt, sch)
		doc := mustParseQuery(t, "{ obj { a b } }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
		gotCalls := rt.GetCalls()

		wantRes := &executor.ExecutionResult{
			Data: map[string]any{
				"obj": nil,
			},
			Errors: []executor.GraphQLError{
				{Message: "boom", Path: executor.Path{"obj", "a"}},
			},
		}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}

		wantCalls := []executor.Call{
			{Kind: "sync", ObjectType: "Query", Field: "obj", Source: nil, Args: map[string]any{}, BatchID: 0},
			{Kind: "sync", ObjectType: "Obj", Field: "a", Source: map[string]any{}, Args: map[string]any{}, BatchID: 0},
		}
		if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
			t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Resolver returns null", func(t *testing.T) {
		// Schema: type Query { obj: Obj! }
		//         type Obj { a: String! b: String! @async }
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {
					Name:   "Query",
					Kind:   schema.TypeKindObject,
					Fields: schema.NewFieldMap(&schema.Field{Name: "obj", Type: schema.NonNullType(schema.NamedType("Obj"))}),
				},
				"Obj": {
					Name: "Obj",
					Kind: schema.TypeKindObject,
					Fields: schema.NewFieldMap(
						&schema.Field{Name: "a", Type: schema.NonNullType(schema.NamedType("String")), Async: false},
						&schema.Field{Name: "b", Type: schema.NonNullType(schema.NamedType("String")), Async: true},
					),
				},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}

		// Runtime: obj resolves to empty map; a returns nil; b would return "B" but must be cancelled.
		rt := executor.NewMockRuntime(map[string]executor.MockResolver{
			"Query.obj": executor.NewMockValueResolver(map[string]any{}),
			"Obj.a":     executor.NewMockValueResolver(nil),
			"Obj.b":     executor.NewMockValueResolver("B"),
		})

		exec := executor.NewExecutor(rt, sch)
		doc := mustParseQuery(t, "{ obj { a b } }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
		gotCalls := rt.GetCalls()

		wantRes := &executor.ExecutionResult{
			Data: map[string]any{"obj": nil},
			Errors: []executor.GraphQLError{
				{Message: "Cannot return null for non-nullable field obj.a", Path: executor.Path{"obj", "a"}},
			},
		}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}

		wantCalls := []executor.Call{
			{Kind: "sync", ObjectType: "Query", Field: "obj", Source: nil, Args: map[string]any{}, BatchID: 0},
			{Kind: "sync", ObjectType: "Obj", Field: "a", Source: map[string]any{}, Args: map[string]any{}, BatchID: 0},
		}
		if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
			t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
		}
	})
}

// Pattern: Result comparison
func TestCompleteValue_List_Nullability_Result(t *testing.T) {
	t.Run("List contains values", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {
					Name:   "Query",
					Kind:   schema.TypeKindObject,
					Fields: schema.NewFieldMap(&schema.Field{Name: "list", Type: schema.ListType(schema.NamedType("String"))}),
				},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		rt := executor.NewMockRuntime(map[string]executor.MockResolver{
			"Query.list": executor.NewMockValueResolver([]any{"A", "B"}),
		})
		exec := executor.NewExecutor(rt, sch)
		doc := mustParseQuery(t, "{ list }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)

		wantRes := &executor.ExecutionResult{
			Data:   map[string]any{"list": []any{"A", "B"}},
			Errors: []executor.GraphQLError{},
		}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("List contains null", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {
					Name:   "Query",
					Kind:   schema.TypeKindObject,
					Fields: schema.NewFieldMap(&schema.Field{Name: "list", Type: schema.ListType(schema.NamedType("String"))}),
				},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		rt := executor.NewMockRuntime(map[string]executor.MockResolver{
			"Query.list": executor.NewMockValueResolver([]any{"A", nil, "B"}),
		})
		exec := executor.NewExecutor(rt, sch)
		doc := mustParseQuery(t, "{ list }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)

		wantRes := &executor.ExecutionResult{
			Data:   map[string]any{"list": []any{"A", nil, "B"}},
			Errors: []executor.GraphQLError{},
		}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("List is null", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {
					Name:   "Query",
					Kind:   schema.TypeKindObject,
					Fields: schema.NewFieldMap(&schema.Field{Name: "list", Type: schema.ListType(schema.NamedType("String"))}),
				},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		rt := executor.NewMockRuntime(map[string]executor.MockResolver{
			"Query.list": executor.NewMockValueResolver(nil),
		})
		exec := executor.NewExecutor(rt, sch)
		doc := mustParseQuery(t, "{ list }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)

		wantRes := &executor.ExecutionResult{
			Data:   map[string]any{"list": nil},
			Errors: []executor.GraphQLError{},
		}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Item non-null violation", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {
					Name:   "Query",
					Kind:   schema.TypeKindObject,
					Fields: schema.NewFieldMap(&schema.Field{Name: "list", Type: schema.ListType(schema.NonNullType(schema.NamedType("String")))}),
				},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		rt := executor.NewMockRuntime(map[string]executor.MockResolver{
			"Query.list": executor.NewMockValueResolver([]any{"A", nil, "B"}),
		})
		exec := executor.NewExecutor(rt, sch)
		doc := mustParseQuery(t, "{ list }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
		wantRes := &executor.ExecutionResult{
			Data: map[string]any{"list": nil},
			Errors: []executor.GraphQLError{
				{Message: "Cannot return null for non-nullable field list.[1]", Path: executor.Path{"list", 1}},
			},
		}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})
}

// Pattern: Result comparison
func TestCompleteValue_Leaf_Serialization_Result(t *testing.T) {
	t.Run("SerializeLeafValue success", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {
					Name:   "Query",
					Kind:   schema.TypeKindObject,
					Fields: schema.NewFieldMap(&schema.Field{Name: "a", Type: schema.NamedType("String")}),
				},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		rt := executor.NewMockRuntime(map[string]executor.MockResolver{
			"Query.a": executor.NewMockValueResolver("ok"),
		})
		executor.SetSerializer(rt, func(val any, t schema.TypeRef) (any, error) {
			if s, ok := val.(string); ok {
				return fmt.Sprintf("%s!", s), nil
			}
			return nil, fmt.Errorf("not string")
		})
		exec := executor.NewExecutor(rt, sch)
		doc := mustParseQuery(t, "{ a }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)

		wantRes := &executor.ExecutionResult{
			Data:   map[string]any{"a": "ok!"},
			Errors: []executor.GraphQLError{},
		}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("SerializeLeafValue error", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {
					Name:   "Query",
					Kind:   schema.TypeKindObject,
					Fields: schema.NewFieldMap(&schema.Field{Name: "a", Type: schema.NamedType("String")}),
				},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		rt := executor.NewMockRuntime(map[string]executor.MockResolver{
			"Query.a": executor.NewMockValueResolver("bad"),
		})
		executor.SetSerializer(rt, func(val any, t schema.TypeRef) (any, error) {
			return nil, fmt.Errorf("serialize error")
		})
		exec := executor.NewExecutor(rt, sch)
		doc := mustParseQuery(t, "{ a }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)

		wantRes := &executor.ExecutionResult{
			Data:   map[string]any{"a": nil},
			Errors: []executor.GraphQLError{{Message: "serialize error", Path: executor.Path{"a"}}},
		}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})
}

// Pattern: Result comparison
func TestCompleteValue_Object_And_MixedSyncAsync_Result(t *testing.T) {
	sch := &schema.Schema{
		QueryType: "Query",
		Types: map[string]*schema.Type{
			"Query": {
				Name:   "Query",
				Kind:   schema.TypeKindObject,
				Fields: schema.NewFieldMap(&schema.Field{Name: "obj", Type: schema.NamedType("Obj")}),
			},
			"Obj": {
				Name: "Obj",
				Kind: schema.TypeKindObject,
				Fields: schema.NewFieldMap(
					&schema.Field{Name: "a", Type: schema.NamedType("String"), Async: false},
					&schema.Field{Name: "b", Type: schema.NamedType("String"), Async: true},
				),
			},
			"String": {Name: "String", Kind: schema.TypeKindScalar},
		},
	}

	rt := executor.NewMockRuntime(map[string]executor.MockResolver{
		"Query.obj": executor.NewMockValueResolver(map[string]any{}),
		"Obj.a":     executor.NewMockValueResolver("A"),
		"Obj.b":     executor.NewMockValueResolver("B"),
	})

	exec := executor.NewExecutor(rt, sch)
	doc := mustParseQuery(t, "{ obj { a b } }")

	gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
	gotCalls := rt.GetCalls()

	wantRes := &executor.ExecutionResult{
		Data:   map[string]any{"obj": map[string]any{"a": "A", "b": "B"}},
		Errors: []executor.GraphQLError{},
	}
	if diff := cmp.Diff(wantRes, gotRes); diff != "" {
		t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
	}

	wantCalls := []executor.Call{
		{Kind: "sync", ObjectType: "Query", Field: "obj", Source: nil, Args: map[string]any{}, BatchID: 0},
		{Kind: "sync", ObjectType: "Obj", Field: "a", Source: map[string]any{}, Args: map[string]any{}, BatchID: 0},
		{Kind: "async", ObjectType: "Obj", Field: "b", Source: map[string]any{}, Args: map[string]any{}, BatchID: 1},
	}
	if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
		t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
	}
}

// Pattern: Result comparison
func TestCompleteValue_Abstract_ResolveType_Result(t *testing.T) {
	t.Run("ResolveType returns concrete subtype", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {
					Name:   "Query",
					Kind:   schema.TypeKindObject,
					Fields: schema.NewFieldMap(&schema.Field{Name: "iface", Type: schema.NamedType("Node")}),
				},
				"Node": {Name: "Node", Kind: schema.TypeKindInterface, PossibleTypes: []string{"Obj"}},
				"Obj": {
					Name:       "Obj",
					Kind:       schema.TypeKindObject,
					Interfaces: []string{"Node"},
					Fields:     schema.NewFieldMap(&schema.Field{Name: "a", Type: schema.NamedType("String")}),
				},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		rt := executor.NewMockRuntime(map[string]executor.MockResolver{
			"Query.iface": executor.NewMockValueResolver(map[string]any{"val": "A"}),
			"Obj.a":       executor.NewMockValueResolver("A"),
		})
		executor.SetTypeResolver(rt, func(value any) (string, error) { return "Obj", nil })
		exec := executor.NewExecutor(rt, sch)
		doc := mustParseQuery(t, "{ iface { a } }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
		gotCalls := rt.GetCalls()

		wantRes := &executor.ExecutionResult{
			Data:   map[string]any{"iface": map[string]any{"a": "A"}},
			Errors: []executor.GraphQLError{},
		}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}

		wantCalls := []executor.Call{
			{Kind: "sync", ObjectType: "Query", Field: "iface", Source: nil, Args: map[string]any{}, BatchID: 0},
			{Kind: "sync", ObjectType: "Obj", Field: "a", Source: map[string]any{"val": "A"}, Args: map[string]any{}, BatchID: 0},
		}
		if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
			t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("ResolveType error", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {
					Name:   "Query",
					Kind:   schema.TypeKindObject,
					Fields: schema.NewFieldMap(&schema.Field{Name: "iface", Type: schema.NamedType("Node")}),
				},
				"Node": {Name: "Node", Kind: schema.TypeKindInterface, PossibleTypes: []string{"Obj"}},
				"Obj": {
					Name:       "Obj",
					Kind:       schema.TypeKindObject,
					Interfaces: []string{"Node"},
					Fields:     schema.NewFieldMap(&schema.Field{Name: "a", Type: schema.NamedType("String")}),
				},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		rt := executor.NewMockRuntime(map[string]executor.MockResolver{
			"Query.iface": executor.NewMockValueResolver(map[string]any{}),
		})
		executor.SetTypeResolver(rt, func(value any) (string, error) { return "", fmt.Errorf("boom") })
		exec := executor.NewExecutor(rt, sch)
		doc := mustParseQuery(t, "{ iface { a } }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
		gotCalls := rt.GetCalls()

		wantRes := &executor.ExecutionResult{
			Data:   map[string]any{"iface": nil},
			Errors: []executor.GraphQLError{{Message: "boom", Path: executor.Path{"iface"}}},
		}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}

		wantCalls := []executor.Call{{Kind: "sync", ObjectType: "Query", Field: "iface", Source: nil, Args: map[string]any{}, BatchID: 0}}
		if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
			t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("ResolveType invalid type name", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {
					Name:   "Query",
					Kind:   schema.TypeKindObject,
					Fields: schema.NewFieldMap(&schema.Field{Name: "iface", Type: schema.NamedType("Node")}),
				},
				"Node": {Name: "Node", Kind: schema.TypeKindInterface, PossibleTypes: []string{"Obj"}},
				"Obj": {
					Name:       "Obj",
					Kind:       schema.TypeKindObject,
					Interfaces: []string{"Node"},
					Fields:     schema.NewFieldMap(&schema.Field{Name: "a", Type: schema.NamedType("String")}),
				},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		rt := executor.NewMockRuntime(map[string]executor.MockResolver{
			"Query.iface": executor.NewMockValueResolver(map[string]any{}),
		})
		executor.SetTypeResolver(rt, func(value any) (string, error) { return "Unknown", nil })
		exec := executor.NewExecutor(rt, sch)
		doc := mustParseQuery(t, "{ iface { a } }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
		gotCalls := rt.GetCalls()

		wantRes := &executor.ExecutionResult{
			Data:   map[string]any{"iface": nil},
			Errors: []executor.GraphQLError{{Message: "Abstract type Node must resolve to an Object type at runtime. Got: Unknown", Path: executor.Path{"iface"}}},
		}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}

		wantCalls := []executor.Call{{Kind: "sync", ObjectType: "Query", Field: "iface", Source: nil, Args: map[string]any{}, BatchID: 0}}
		if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
			t.Fatalf("Runtime calls mismatch (-want +got):\n%s", diff)
		}
	})
}
