package schema

import (
	"context"
	"sort"
	"strings"

	"github.com/hanpama/protograph/internal/ir"
)

// BuildFromIR builds an executable GraphQL schema from the ir project.
// It merges all extensions into their base definitions and strips protograph-specific
// directives.
func BuildFromIR(p *ir.Project) (*Schema, error) {
	s := &Schema{
		QueryType:        p.Schema.QueryType,
		MutationType:     p.Schema.MutationType,
		SubscriptionType: p.Schema.SubscriptionType,
		Description:      "",
		Types:            map[string]*Type{},
		Directives:       map[string]*Directive{},
	}
	// Builtins
	s.Types[stringType.Name] = stringType
	s.Types[intType.Name] = intType
	s.Types[floatType.Name] = floatType
	s.Types[booleanType.Name] = booleanType
	s.Types[idType.Name] = idType
	s.Directives[includeDirective.Name] = includeDirective
	s.Directives[skipDirective.Name] = skipDirective

	for name, def := range p.Definitions {
		if def.Object != nil {
			s.Types[name] = buildObject(def.Object)
		} else if def.Interface != nil {
			s.Types[name] = buildInterface(def.Interface)
		} else if def.Enum != nil {
			s.Types[name] = buildEnum(def.Enum)
		} else if def.Input != nil {
			s.Types[name] = buildInput(def.Input)
		} else if def.Union != nil {
			s.Types[name] = buildUnion(def.Union)
		} else if def.Scalar != nil {
			s.Types[name] = buildScalar(def.Scalar)
		}
	}
	for _, dir := range p.Directives {
		b := buildDirective(dir)
		s.Directives[b.Name] = b
	}
	return s, nil
}

func buildObject(def *ir.ObjectDefinition) *Type {
	t := &Type{
		Name:        def.Name,
		Kind:        TypeKindObject,
		Description: def.Description,
	}
	// Collect interface names
	for name := range def.Interfaces {
		t.Interfaces = append(t.Interfaces, name)
	}
	// Sort interfaces for deterministic output
	sort.Strings(t.Interfaces)

	// Build fields - skip internal fields
	var fieldNames []string
	for name := range def.Fields {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)

	for _, name := range fieldNames {
		f := def.Fields[name]
		if !f.IsInternal {
			t.Fields = append(t.Fields, buildField(f))
		}
	}
	return t
}

func buildInterface(def *ir.InterfaceDefinition) *Type {
	t := &Type{
		Name:        def.Name,
		Kind:        TypeKindInterface,
		Description: def.Description,
	}
	// Collect interface names
	for name := range def.Interfaces {
		t.Interfaces = append(t.Interfaces, name)
	}
	// Sort interfaces for deterministic output
	sort.Strings(t.Interfaces)

	// Build fields
	var fieldNames []string
	for name := range def.Fields {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)

	for _, name := range fieldNames {
		f := def.Fields[name]
		if !f.IsInternal {
			t.Fields = append(t.Fields, buildField(f))
		}
	}
	return t
}

func buildField(def *ir.FieldDefinition) *Field {
	f := &Field{
		Name:              def.Name,
		Description:       def.Description,
		Type:              buildTypeRef(def.Type),
		IsDeprecated:      def.Deprecation != nil,
		DeprecationReason: "",
		Async:             def.ResolveBySource == nil,
	}
	if def.Deprecation != nil {
		f.DeprecationReason = def.Deprecation.Reason
	}

	// Sort argument names for deterministic output
	var argNames []string
	for name := range def.Args {
		argNames = append(argNames, name)
	}
	sort.Strings(argNames)

	for _, name := range argNames {
		arg := def.Args[name]
		f.Arguments = append(f.Arguments, buildArgumentAsInputValue(arg))
	}
	return f
}

func buildEnum(def *ir.EnumDefinition) *Type {
	t := &Type{
		Name:        def.Name,
		Kind:        TypeKindEnum,
		Description: def.Description,
	}

	// Sort enum value names for deterministic output
	var valueNames []string
	for name := range def.Values {
		valueNames = append(valueNames, name)
	}
	sort.Strings(valueNames)

	for _, name := range valueNames {
		v := def.Values[name]
		t.EnumValues = append(t.EnumValues, buildEnumValue(v))
	}
	return t
}

func buildEnumValue(v *ir.EnumValueDefinition) *EnumValue {
	e := &EnumValue{
		Name:              v.Name,
		Description:       v.Description,
		IsDeprecated:      v.Deprecation != nil,
		DeprecationReason: "",
	}
	if v.Deprecation != nil {
		e.DeprecationReason = v.Deprecation.Reason
	}
	return e
}

func buildTypeRef(t *ir.TypeExpr) *TypeRef {
	switch t.Kind {
	case ir.TypeExprKindNamed:
		return &TypeRef{Kind: TypeRefKindNamed, Named: t.Named}
	case ir.TypeExprKindNonNull:
		return &TypeRef{Kind: TypeRefKindNonNull, OfType: buildTypeRef(t.OfType)}
	case ir.TypeExprKindList:
		return &TypeRef{Kind: TypeRefKindList, OfType: buildTypeRef(t.OfType)}
	}
	panic("unreachable")
}

func buildInputValue(v *ir.InputValueDefinition) *InputValue {
	in := &InputValue{
		Name:              v.Name,
		Description:       v.Description,
		Type:              buildTypeRef(v.Type),
		DefaultValue:      v.DefaultValue,
		IsDeprecated:      v.Deprecation != nil,
		DeprecationReason: "",
	}
	if v.Deprecation != nil {
		in.DeprecationReason = v.Deprecation.Reason
	}
	return in
}

func buildArgumentAsInputValue(a *ir.ArgumentDefinition) *InputValue {
	in := &InputValue{
		Name:              a.Name,
		Description:       a.Description,
		Type:              buildTypeRef(a.Type),
		DefaultValue:      a.DefaultValue,
		IsDeprecated:      a.Deprecation != nil,
		DeprecationReason: "",
	}
	if a.Deprecation != nil {
		in.DeprecationReason = a.Deprecation.Reason
	}
	return in
}

func buildInput(def *ir.InputDefinition) *Type {
	t := &Type{
		Name:        def.Name,
		Kind:        TypeKindInputObject,
		Description: def.Description,
		OneOf:       def.OneOf,
	}

	// Sort input value names for deterministic output
	var valueNames []string
	for name := range def.InputValues {
		valueNames = append(valueNames, name)
	}
	sort.Strings(valueNames)

	for _, name := range valueNames {
		v := def.InputValues[name]
		t.InputFields = append(t.InputFields, buildInputValue(v))
	}
	return t
}

func buildUnion(def *ir.UnionDefinition) *Type {
	t := &Type{
		Name:        def.Name,
		Kind:        TypeKindUnion,
		Description: def.Description,
	}

	// Sort union type names for deterministic output
	var typeNames []string
	for name := range def.Types {
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)

	for _, name := range typeNames {
		t.PossibleTypes = append(t.PossibleTypes, name)
	}
	return t
}

func buildScalar(def *ir.ScalarDefinition) *Type {
	t := &Type{
		Name:        def.Name,
		Kind:        TypeKindScalar,
		Description: def.Description,
	}
	return t
}

func buildDirective(dir *ir.DirectiveDefinition) *Directive {
	locations := make([]string, 0, len(dir.Locations))
	locations = append(locations, dir.Locations...)

	// Sort argument names for deterministic output
	var argNames []string
	for name := range dir.Args {
		argNames = append(argNames, name)
	}
	sort.Strings(argNames)

	var arguments []*InputValue
	for _, name := range argNames {
		arg := dir.Args[name]
		arguments = append(arguments, buildArgumentAsInputValue(arg))
	}

	return &Directive{
		Name:         dir.Name,
		Description:  dir.Description,
		Locations:    locations,
		Arguments:    arguments,
		IsRepeatable: dir.Repeatable,
	}
}

// BuildFromSDL parses SDL string and returns the corresponding Schema.
func BuildFromSDL(sdl string) (*Schema, error) {
	// Add schema definition if missing
	if !strings.Contains(sdl, "schema {") {
		sdl = "schema { query: Query }\n" + sdl
	}

	disc := ir.NewInMemoryDiscovery([]ir.InMemoryService{
		{
			Package: "test",
			Name:    "test",
			Content: sdl,
		},
	})
	proj, err := ir.Build(context.Background(), disc)
	if err != nil {
		return nil, err
	}
	schema, err := BuildFromIR(proj)
	if err != nil {
		return nil, err
	}
	return schema, nil
}
