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
	s := NewSchema("")
	s.SetQueryType(p.Schema.QueryType).
		SetMutationType(p.Schema.MutationType).
		SetSubscriptionType(p.Schema.SubscriptionType)
	// Builtins
	s.AddType(stringType).
		AddType(intType).
		AddType(floatType).
		AddType(booleanType).
		AddType(idType)
	s.AddDirective(includeDirective).
		AddDirective(skipDirective)

	for _, def := range p.Definitions {
		if def.Object != nil {
			s.AddType(buildObject(def.Object))
		} else if def.Interface != nil {
			s.AddType(buildInterface(def.Interface))
		} else if def.Enum != nil {
			s.AddType(buildEnum(def.Enum))
		} else if def.Input != nil {
			s.AddType(buildInput(def.Input))
		} else if def.Union != nil {
			s.AddType(buildUnion(def.Union))
		} else if def.Scalar != nil {
			s.AddType(buildScalar(def.Scalar))
		}
	}
	for _, dir := range p.Directives {
		s.AddDirective(buildDirective(dir))
	}
	return s, nil
}

func buildObject(def *ir.ObjectDefinition) *Type {
	t := NewType(def.Name, TypeKindObject, def.Description)

	var interfaceNames []string
	for name := range def.Interfaces {
		interfaceNames = append(interfaceNames, name)
	}
	sort.Strings(interfaceNames)
	for _, name := range interfaceNames {
		t.AddInterface(name)
	}

	fieldDefs := make([]*ir.FieldDefinition, 0, len(def.Fields))
	for _, fieldDef := range def.Fields {
		if fieldDef.IsInternal {
			continue
		}
		fieldDefs = append(fieldDefs, fieldDef)
	}
	sort.Slice(fieldDefs, func(i, j int) bool { return fieldDefs[i].Index < fieldDefs[j].Index })
	for _, fieldDef := range fieldDefs {
		t.AddField(buildField(fieldDef))
	}
	return t
}

func buildInterface(def *ir.InterfaceDefinition) *Type {
	t := NewType(def.Name, TypeKindInterface, def.Description)

	var interfaceNames []string
	for name := range def.Interfaces {
		interfaceNames = append(interfaceNames, name)
	}
	sort.Strings(interfaceNames)
	for _, name := range interfaceNames {
		t.AddInterface(name)
	}

	fieldDefs := make([]*ir.FieldDefinition, 0, len(def.Fields))
	for _, fieldDef := range def.Fields {
		if fieldDef.IsInternal {
			continue
		}
		fieldDefs = append(fieldDefs, fieldDef)
	}
	sort.Slice(fieldDefs, func(i, j int) bool { return fieldDefs[i].Index < fieldDefs[j].Index })
	for _, fieldDef := range fieldDefs {
		t.AddField(buildField(fieldDef))
	}
	return t
}

func buildField(def *ir.FieldDefinition) *Field {
	f := NewField(def.Name, def.Description, buildTypeRef(def.Type)).
		SetAsync(def.ResolveBySource == nil)
	if def.Deprecation != nil {
		f.Deprecate(def.Deprecation.Reason)
	}
	args := make([]*ir.ArgumentDefinition, 0, len(def.Args))
	for _, arg := range def.Args {
		args = append(args, arg)
	}
	sort.Slice(args, func(i, j int) bool { return args[i].Index < args[j].Index })
	for _, arg := range args {
		f.AddArgument(buildArgumentAsInputValue(arg))
	}
	return f
}

func buildEnum(def *ir.EnumDefinition) *Type {
	t := NewType(def.Name, TypeKindEnum, def.Description)

	var valueNames []string
	for name := range def.Values {
		valueNames = append(valueNames, name)
	}
	sort.Strings(valueNames)
	for _, name := range valueNames {
		t.AddEnumValue(buildEnumValue(def.Values[name]))
	}
	return t
}

func buildEnumValue(v *ir.EnumValueDefinition) *EnumValue {
	e := NewEnumValue(v.Name, v.Description)
	if v.Deprecation != nil {
		e.Deprecate(v.Deprecation.Reason)
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
	in := NewInputValue(v.Name, v.Description, buildTypeRef(v.Type)).SetDefault(v.DefaultValue)
	if v.Deprecation != nil {
		in.Deprecate(v.Deprecation.Reason)
	}
	return in
}

func buildArgumentAsInputValue(a *ir.ArgumentDefinition) *InputValue {
	in := NewInputValue(a.Name, a.Description, buildTypeRef(a.Type)).SetDefault(a.DefaultValue)
	if a.Deprecation != nil {
		in.Deprecate(a.Deprecation.Reason)
	}
	return in
}

func buildInput(def *ir.InputDefinition) *Type {
	t := NewType(def.Name, TypeKindInputObject, def.Description).SetOneOf(def.OneOf)
	values := make([]*ir.InputValueDefinition, 0, len(def.InputValues))
	for _, v := range def.InputValues {
		values = append(values, v)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Index < values[j].Index })
	for _, v := range values {
		t.AddInputField(buildInputValue(v))
	}
	return t
}

func buildUnion(def *ir.UnionDefinition) *Type {
	t := NewType(def.Name, TypeKindUnion, def.Description)

	// Sort union type names for deterministic output
	var typeNames []string
	for name := range def.Types {
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)

	for _, name := range typeNames {
		t.AddPossibleType(name)
	}
	return t
}

func buildScalar(def *ir.ScalarDefinition) *Type {
	t := NewType(def.Name, TypeKindScalar, def.Description)
	return t
}

func buildDirective(dir *ir.DirectiveDefinition) *Directive {
	d := NewDirective(dir.Name, dir.Description).SetRepeatable(dir.Repeatable)
	d.Locations = append(d.Locations, dir.Locations...)
	args := make([]*ir.ArgumentDefinition, 0, len(dir.Args))
	for _, arg := range dir.Args {
		args = append(args, arg)
	}
	sort.Slice(args, func(i, j int) bool { return args[i].Index < args[j].Index })
	for _, arg := range args {
		d.AddArgument(buildArgumentAsInputValue(arg))
	}
	return d
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
