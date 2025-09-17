package executor

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	schema "github.com/hanpama/protograph/internal/schema"
)

// Pattern: Result comparison
func TestErrors_LocatedPaths_Result(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query":  {Name: "Query", Kind: schema.TypeKindObject, Fields: schema.NewFieldMap(&schema.Field{Name: "a", Type: schema.NamedType("String")})},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		rt := NewMockRuntime(map[string]MockResolver{
			"Query.a": NewMockErrorResolver(fmt.Errorf("boom")),
		})
		exec := NewExecutor(rt, sch)
		doc := mustParseQuery(t, "{ a }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)

		wantRes := &ExecutionResult{
			Data:   map[string]any{"a": nil},
			Errors: []GraphQLError{{Message: "boom", Path: Path{"a"}}},
		}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Nested", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query":  {Name: "Query", Kind: schema.TypeKindObject, Fields: schema.NewFieldMap(&schema.Field{Name: "obj", Type: schema.NamedType("Obj")})},
				"Obj":    {Name: "Obj", Kind: schema.TypeKindObject, Fields: schema.NewFieldMap(&schema.Field{Name: "a", Type: schema.NamedType("String")})},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		rt := NewMockRuntime(map[string]MockResolver{
			"Query.obj": NewMockValueResolver(map[string]any{}),
			"Obj.a":     NewMockErrorResolver(fmt.Errorf("boom")),
		})
		exec := NewExecutor(rt, sch)
		doc := mustParseQuery(t, "{ obj { a } }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)

		wantRes := &ExecutionResult{
			Data:   map[string]any{"obj": map[string]any{"a": nil}},
			Errors: []GraphQLError{{Message: "boom", Path: Path{"obj", "a"}}},
		}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("List index in path", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query":  {Name: "Query", Kind: schema.TypeKindObject, Fields: schema.NewFieldMap(&schema.Field{Name: "objs", Type: schema.ListType(schema.NamedType("Obj"))})},
				"Obj":    {Name: "Obj", Kind: schema.TypeKindObject, Fields: schema.NewFieldMap(&schema.Field{Name: "a", Type: schema.NamedType("String")})},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		rt := NewMockRuntime(map[string]MockResolver{
			"Query.objs": NewMockValueResolver([]any{map[string]any{"idx": 0}, map[string]any{"idx": 1}}),
			"Obj.a": func(ctx context.Context, src any, args map[string]any) (any, error) {
				if src.(map[string]any)["idx"].(int) == 1 {
					return nil, fmt.Errorf("boom")
				}
				return "A", nil
			},
		})
		exec := NewExecutor(rt, sch)
		doc := mustParseQuery(t, "{ objs { a } }")

		gotRes := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)

		wantRes := &ExecutionResult{
			Data:   map[string]any{"objs": []any{map[string]any{"a": "A"}, map[string]any{"a": nil}}},
			Errors: []GraphQLError{{Message: "boom", Path: Path{"objs", 1, "a"}}},
		}
		if diff := cmp.Diff(wantRes, gotRes); diff != "" {
			t.Fatalf("ExecutionResult mismatch (-want +got):\n%s", diff)
		}
	})
}
