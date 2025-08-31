package executor

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	schema "github.com/hanpama/protograph/internal/schema"
)

// Pattern: Result comparison
func TestContext_OperationSelection_Result(t *testing.T) {
	t.Run("Inline operation", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query":  {Name: "Query", Kind: schema.TypeKindObject, Fields: []*schema.Field{{Name: "a", Type: schema.NamedType("String")}}},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		rt := NewMockRuntime(map[string]MockResolver{
			"Query.a": NewMockValueResolver("A"),
		})
		exec := NewExecutor(rt, sch)
		doc := mustParseQuery(t, "{ a }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
		wantRes := &ExecutionResult{Data: map[string]any{"a": "A"}, Errors: []GraphQLError{}}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Single named operation without name", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query":  {Name: "Query", Kind: schema.TypeKindObject, Fields: []*schema.Field{{Name: "a", Type: schema.NamedType("String")}}},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		rt := NewMockRuntime(map[string]MockResolver{"Query.a": NewMockValueResolver("A")})
		exec := NewExecutor(rt, sch)
		doc := mustParseQuery(t, "query Foo { a }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
		wantRes := &ExecutionResult{Data: map[string]any{"a": "A"}, Errors: []GraphQLError{}}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Named operation provided", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query":  {Name: "Query", Kind: schema.TypeKindObject, Fields: []*schema.Field{{Name: "a", Type: schema.NamedType("String")}, {Name: "b", Type: schema.NamedType("String")}}},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		rt := NewMockRuntime(map[string]MockResolver{
			"Query.a": NewMockValueResolver("A"),
			"Query.b": NewMockValueResolver("B"),
		})
		exec := NewExecutor(rt, sch)
		doc := mustParseQuery(t, "query Foo { a } query Bar { b }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "Bar", nil, nil)
		wantRes := &ExecutionResult{Data: map[string]any{"b": "B"}, Errors: []GraphQLError{}}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Error no operation provided", func(t *testing.T) {
		sch := &schema.Schema{}
		rt := NewMockRuntime(nil)
		exec := NewExecutor(rt, sch)
		doc := mustParseQuery(t, "fragment F on Query { a }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
		wantRes := &ExecutionResult{Data: nil, Errors: []GraphQLError{{Message: "operation not found"}}}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Error no name with multiple operations", func(t *testing.T) {
		sch := &schema.Schema{}
		rt := NewMockRuntime(nil)
		exec := NewExecutor(rt, sch)
		doc := mustParseQuery(t, "query Foo { a } query Bar { b }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
		wantRes := &ExecutionResult{Data: nil, Errors: []GraphQLError{{Message: "operation not found"}}}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Error unknown operation name", func(t *testing.T) {
		sch := &schema.Schema{}
		rt := NewMockRuntime(nil)
		exec := NewExecutor(rt, sch)
		doc := mustParseQuery(t, "query Foo { a } query Bar { b }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "Baz", nil, nil)
		wantRes := &ExecutionResult{Data: nil, Errors: []GraphQLError{{Message: "operation not found"}}}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})
}

// Pattern: Result comparison
func TestContext_VariableCoercion_Result(t *testing.T) {
	t.Run("Provided variable", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {Name: "Query", Kind: schema.TypeKindObject, Fields: []*schema.Field{{Name: "echo", Type: schema.NamedType("Int"), Arguments: []*schema.InputValue{{Name: "v", Type: schema.NamedType("Int")}}}}},
				"Int":   {Name: "Int", Kind: schema.TypeKindScalar},
			},
		}
		rt := NewMockRuntime(map[string]MockResolver{
			"Query.echo": func(ctx context.Context, src any, args map[string]any) (any, error) { return args["v"], nil },
		})
		exec := NewExecutor(rt, sch)
		doc := mustParseQuery(t, "query($v: Int!){ echo(v:$v) }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", map[string]any{"v": 3}, nil)
		wantRes := &ExecutionResult{Data: map[string]any{"echo": 3}, Errors: []GraphQLError{}}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Use default", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {Name: "Query", Kind: schema.TypeKindObject, Fields: []*schema.Field{{Name: "echo", Type: schema.NamedType("Int"), Arguments: []*schema.InputValue{{Name: "v", Type: schema.NamedType("Int")}}}}},
				"Int":   {Name: "Int", Kind: schema.TypeKindScalar},
			},
		}
		rt := NewMockRuntime(map[string]MockResolver{
			"Query.echo": func(ctx context.Context, src any, args map[string]any) (any, error) { return args["v"], nil },
		})
		exec := NewExecutor(rt, sch)
		doc := mustParseQuery(t, "query($v: Int = 5){ echo(v:$v) }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
		wantRes := &ExecutionResult{Data: map[string]any{"echo": 5}, Errors: []GraphQLError{}}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Missing required variable", func(t *testing.T) {
		sch := &schema.Schema{}
		rt := NewMockRuntime(nil)
		exec := NewExecutor(rt, sch)
		doc := mustParseQuery(t, "query($v: Int!){ echo(v:$v) }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
		wantRes := &ExecutionResult{Errors: []GraphQLError{{Message: "variable $v of required type Int! was not provided"}}}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Null for NonNull variable", func(t *testing.T) {
		sch := &schema.Schema{}
		rt := NewMockRuntime(nil)
		exec := NewExecutor(rt, sch)
		doc := mustParseQuery(t, "query($v: Int!){ echo(v:$v) }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", map[string]any{"v": nil}, nil)
		wantRes := &ExecutionResult{Errors: []GraphQLError{{Message: "variable $v of type Int! cannot be null"}}}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})
}
