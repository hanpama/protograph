package introspection

import (
	"context"
	"fmt"
	"sort"

	executor "github.com/hanpama/protograph/internal/executor"
	schema "github.com/hanpama/protograph/internal/schema"
)

// IntrospectionWrapper holds both the runtime and extended schema
type IntrospectionWrapper struct {
	Runtime executor.Runtime
	Schema  *schema.Schema
}

// Wrap returns a Runtime that handles GraphQL introspection fields.
// It extends the schema with introspection types and fields.
func Wrap(base executor.Runtime, sch *schema.Schema) *IntrospectionWrapper {
	// Create a copy of the schema to avoid modifying the original
	extendedSchema := extendSchemaWithIntrospection(sch)
	runtime := &runtime{
		base:           base,
		schema:         extendedSchema,
		originalSchema: sch,
	}
	return &IntrospectionWrapper{
		Runtime: runtime,
		Schema:  extendedSchema,
	}
}

type runtime struct {
	base           executor.Runtime
	schema         *schema.Schema // Extended schema with introspection types
	originalSchema *schema.Schema // Original schema for introspection queries
}

func (r *runtime) ResolveSync(ctx context.Context, objectType, field string, source any, args map[string]any) (any, error) {

	switch src := source.(type) {
	case *schema.Schema:
		if v, ok := resolveSchemaField(src, field); ok {
			return v, nil
		}
	case *schema.Type:
		if v, ok := resolveTypeField(r.originalSchema, src, field, args); ok {
			return v, nil
		}
	case *schema.TypeRef:
		if v, ok := resolveTypeRefField(r.originalSchema, src, field, args); ok {
			return v, nil
		}
	case *schema.Field:
		if v, ok := resolveFieldField(src, field, args); ok {
			return v, nil
		}
	case *schema.InputValue:
		if v, ok := resolveInputValueField(src, field); ok {
			return v, nil
		}
	case *schema.EnumValue:
		if v, ok := resolveEnumValueField(src, field); ok {
			return v, nil
		}
	case *schema.Directive:
		if v, ok := resolveDirectiveField(src, field, args); ok {
			return v, nil
		}
	}

	if objectType == "Query" {
		switch field {
		case "__schema":
			return r.originalSchema, nil
		case "__type":
			return r.resolveTypeQuery(args), nil
		}
	}

	return r.base.ResolveSync(ctx, objectType, field, source, args)
}

func (r *runtime) BatchResolveAsync(ctx context.Context, tasks []executor.AsyncResolveTask) []executor.AsyncResolveResult {
	return r.base.BatchResolveAsync(ctx, tasks)
}

func (r *runtime) ResolveType(ctx context.Context, abstractType string, value any) (string, error) {
	return r.base.ResolveType(ctx, abstractType, value)
}

func (r *runtime) ResolveUnionConcreteValue(ctx context.Context, unionTypeName string, value any) (any, error) {
	return r.base.ResolveUnionConcreteValue(ctx, unionTypeName, value)
}

func (r *runtime) ResolveInterfaceConcreteValue(ctx context.Context, interfaceTypeName string, value any) (any, error) {
	return r.base.ResolveInterfaceConcreteValue(ctx, interfaceTypeName, value)
}

func (r *runtime) SerializeLeafValue(ctx context.Context, typ string, value any) (any, error) {
	return r.base.SerializeLeafValue(ctx, typ, value)
}

// --- helpers ---

func (r *runtime) resolveTypeQuery(args map[string]any) *schema.Type {
	name, _ := args["name"].(string)
	if name == "" {
		return nil
	}
	return r.originalSchema.Types[name]
}

func resolveSchemaTypes(sch *schema.Schema) []*schema.Type {
	if sch.Types == nil {
		return []*schema.Type{}
	}
	out := make([]*schema.Type, 0, len(sch.Types))
	for _, t := range sch.Types {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func resolveSchemaDirectives(sch *schema.Schema) []*schema.Directive {
	if sch.Directives == nil {
		return []*schema.Directive{}
	}
	dirs := make([]*schema.Directive, 0, len(sch.Directives))
	for _, d := range sch.Directives {
		dirs = append(dirs, d)
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	return dirs
}

func resolveTypeFields(t *schema.Type, args map[string]any) []*schema.Field {
	if t.Kind != schema.TypeKindObject && t.Kind != schema.TypeKindInterface {
		return nil
	}
	includeDeprecated := boolArg(args, "includeDeprecated", false)
	out := []*schema.Field{}
	for _, f := range t.GetOrderedFields() {
		if !includeDeprecated && f.IsDeprecated {
			continue
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func resolveTypeInterfaces(sch *schema.Schema, t *schema.Type) []*schema.Type {
	if t.Kind != schema.TypeKindObject && t.Kind != schema.TypeKindInterface {
		return nil
	}
	out := make([]*schema.Type, 0, len(t.Interfaces))
	for _, name := range t.Interfaces {
		if def := sch.Types[name]; def != nil {
			out = append(out, def)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func resolveTypePossibleTypes(sch *schema.Schema, t *schema.Type) []*schema.Type {
	if t.Kind != schema.TypeKindInterface && t.Kind != schema.TypeKindUnion {
		return nil
	}
	pts := []*schema.Type{}
	for _, name := range t.PossibleTypes {
		if def := sch.Types[name]; def != nil {
			pts = append(pts, def)
		}
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].Name < pts[j].Name })
	return pts
}

func resolveTypeEnumValues(t *schema.Type, args map[string]any) []*schema.EnumValue {
	if t.Kind != schema.TypeKindEnum {
		return nil
	}
	includeDeprecated := boolArg(args, "includeDeprecated", false)
	out := []*schema.EnumValue{}
	for _, ev := range t.EnumValues {
		if !includeDeprecated && ev.IsDeprecated {
			continue
		}
		out = append(out, ev)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func resolveTypeInputFields(t *schema.Type, args map[string]any) []*schema.InputValue {
	if t.Kind != schema.TypeKindInputObject {
		return nil
	}
	includeDeprecated := boolArg(args, "includeDeprecated", false)
	out := []*schema.InputValue{}
	for _, iv := range t.GetOrderedInputFields() {
		if !includeDeprecated && iv.IsDeprecated {
			continue
		}
		out = append(out, iv)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func resolveFieldArgs(f *schema.Field, args map[string]any) []*schema.InputValue {
	includeDeprecated := boolArg(args, "includeDeprecated", false)
	out := []*schema.InputValue{}
	for _, a := range f.GetOrderedArguments() {
		if !includeDeprecated && a.IsDeprecated {
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func resolveFieldDeprecationReason(f *schema.Field) *string {
	if f.IsDeprecated {
		return &f.DeprecationReason
	}
	return nil
}

func resolveInputValueDefaultValue(a *schema.InputValue) *string {
	if a.DefaultValue != nil {
		value := fmt.Sprintf("%v", a.DefaultValue)
		return &value
	}
	return nil
}

func resolveInputValueDeprecationReason(a *schema.InputValue) *string {
	if a.IsDeprecated {
		return &a.DeprecationReason
	}
	return nil
}

func resolveEnumValueDeprecationReason(ev *schema.EnumValue) *string {
	if ev.IsDeprecated {
		return &ev.DeprecationReason
	}
	return nil
}

func resolveDirectiveLocations(d *schema.Directive) []string {
	locs := make([]string, len(d.Locations))
	for i, l := range d.Locations {
		locs[i] = string(l)
	}
	sort.Strings(locs)
	return locs
}

func resolveDirectiveArgs(d *schema.Directive, args map[string]any) []*schema.InputValue {
	includeDeprecated := boolArg(args, "includeDeprecated", false)
	out := []*schema.InputValue{}
	for _, a := range d.Arguments {
		if !includeDeprecated && a.IsDeprecated {
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func resolveSchemaField(sch *schema.Schema, field string) (any, bool) {
	switch field {
	case "types":
		return resolveSchemaTypes(sch), true
	case "queryType":
		return sch.GetQueryType(), true
	case "mutationType":
		return sch.GetMutationType(), true
	case "subscriptionType":
		return sch.GetSubscriptionType(), true
	case "directives":
		return resolveSchemaDirectives(sch), true
	case "description":
		return sch.Description, true
	}
	return nil, false
}

func resolveTypeField(sch *schema.Schema, t *schema.Type, field string, args map[string]any) (any, bool) {
	switch field {
	case "kind":
		return string(t.Kind), true
	case "name":
		return t.Name, true
	case "description":
		return t.Description, true
	case "specifiedByURL":
		return t.SpecifiedByURL, true
	case "fields":
		return resolveTypeFields(t, args), true
	case "interfaces":
		return resolveTypeInterfaces(sch, t), true
	case "possibleTypes":
		return resolveTypePossibleTypes(sch, t), true
	case "enumValues":
		return resolveTypeEnumValues(t, args), true
	case "inputFields":
		return resolveTypeInputFields(t, args), true
	case "isOneOf":
		return t.OneOf, true
	case "ofType":
		// Wrapper types (LIST/NON_NULL) are represented as TypeRef nodes, so named types never expose ofType.
		return nil, true
	}
	return nil, false
}

func resolveTypeRefField(sch *schema.Schema, tr *schema.TypeRef, field string, args map[string]any) (any, bool) {
	switch field {
	case "kind":
		return tr.Kind, true
	case "name":
		if schema.IsNonNull(tr) || schema.IsList(tr) {
			return nil, true
		}
		return tr.Named, true
	case "ofType":
		if tr.Kind == schema.TypeRefKindNonNull || tr.Kind == schema.TypeRefKindList {
			return tr.OfType, true
		}
		return nil, true
	default:
		if name := schema.GetNamedType(tr); name != "" {
			if def := sch.Types[name]; def != nil {
				return resolveTypeField(sch, def, field, args)
			}
		}
		return nil, true
	}
}

func resolveFieldField(f *schema.Field, field string, args map[string]any) (any, bool) {
	switch field {
	case "name":
		return f.Name, true
	case "description":
		return f.Description, true
	case "args":
		return resolveFieldArgs(f, args), true
	case "type":
		return f.Type, true
	case "isDeprecated":
		return f.IsDeprecated, true
	case "deprecationReason":
		return resolveFieldDeprecationReason(f), true
	}
	return nil, false
}

func resolveInputValueField(a *schema.InputValue, field string) (any, bool) {
	switch field {
	case "name":
		return a.Name, true
	case "description":
		return a.Description, true
	case "type":
		return a.Type, true
	case "defaultValue":
		return resolveInputValueDefaultValue(a), true
	case "isDeprecated":
		return a.IsDeprecated, true
	case "deprecationReason":
		return resolveInputValueDeprecationReason(a), true
	}
	return nil, false
}

func resolveEnumValueField(ev *schema.EnumValue, field string) (any, bool) {
	switch field {
	case "name":
		return ev.Name, true
	case "description":
		return ev.Description, true
	case "isDeprecated":
		return ev.IsDeprecated, true
	case "deprecationReason":
		return resolveEnumValueDeprecationReason(ev), true
	}
	return nil, false
}

func resolveDirectiveField(d *schema.Directive, field string, args map[string]any) (any, bool) {
	switch field {
	case "name":
		return d.Name, true
	case "description":
		return d.Description, true
	case "isRepeatable":
		return d.IsRepeatable, true
	case "locations":
		return resolveDirectiveLocations(d), true
	case "args":
		return resolveDirectiveArgs(d, args), true
	}
	return nil, false
}

func boolArg(args map[string]any, name string, def bool) bool {
	if args == nil {
		return def
	}
	if v, ok := args[name]; ok {
		if b, ok2 := v.(bool); ok2 {
			return b
		}
	}
	return def
}
