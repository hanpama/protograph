package introspection

import (
	"context"
	"testing"

	executor "github.com/hanpama/protograph/internal/executor"
	language "github.com/hanpama/protograph/internal/language"
	schema "github.com/hanpama/protograph/internal/schema"
)

// noopRuntime implements executor.Runtime with no behaviour.
type noopRuntime struct{}

func (noopRuntime) ResolveSync(context.Context, string, string, any, map[string]any) (any, error) {
	return nil, nil
}

func (noopRuntime) BatchResolveAsync(context.Context, []executor.AsyncResolveTask) []executor.AsyncResolveResult {
	return nil
}

func (noopRuntime) ResolveType(context.Context, string, any) (string, error) {
	return "", nil
}

func (noopRuntime) SerializeLeafValue(_ context.Context, _ string, value any) (any, error) {
	return value, nil
}

func buildSchema(t *testing.T) *schema.Schema {
	t.Helper()
	sdl := `type Query { hello: String }`
	sch, err := schema.BuildFromSDL(sdl)
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	return sch
}

func TestIntrospectionEnabled(t *testing.T) {
	sch := buildSchema(t)
	// Wrap with introspection enabled
	wrapper := Wrap(noopRuntime{}, sch)
	exec := executor.NewExecutor(wrapper.Runtime, wrapper.Schema)
	doc, err := language.ParseQuery("{__schema{queryType{name}}}")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
	if len(res.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	data := res.Data.(map[string]any)
	schData := data["__schema"].(map[string]any)
	qt := schData["queryType"].(map[string]any)
	if qt["name"].(string) != "Query" {
		t.Fatalf("queryType.name = %v", qt["name"])
	}
}

func TestTypenameField(t *testing.T) {
	sch := buildSchema(t)
	// __typename should work without introspection wrapper
	rt := noopRuntime{}
	exec := executor.NewExecutor(rt, sch)
	doc, err := language.ParseQuery("{__typename}")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res := exec.ExecuteRequest(context.Background(), doc, "", nil, nil)
	if len(res.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	data := res.Data.(map[string]any)
	if data["__typename"] != "Query" {
		t.Fatalf("expected __typename to be Query, got %v", data["__typename"])
	}
}
