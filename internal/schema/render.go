package schema

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Render produces SDL from the Schema.
// Deterministic ordering: type/directive names sorted lexicographically.
func Render(s *Schema) string {
	if s == nil {
		return ""
	}
	var b strings.Builder

	// Collect and sort type names, excluding built-in scalars
	typeNames := make([]string, 0, len(s.Types))

	for name, typ := range s.Types {
		switch typ {
		case stringType, intType, floatType, booleanType, idType:
			continue
		default:
			typeNames = append(typeNames, name)
		}
	}
	sort.Strings(typeNames)

	for _, name := range typeNames {
		typ := s.Types[name]
		switch typ.Kind {
		case TypeKindScalar:
			renderScalar(&b, typ)
		case TypeKindEnum:
			renderEnum(&b, typ)
		case TypeKindInputObject:
			renderInputObject(&b, typ)
		case TypeKindObject:
			renderObject(&b, typ)
		case TypeKindInterface:
			renderInterface(&b, typ)
		case TypeKindUnion:
			renderUnion(&b, typ)
		}
	}

	// Render directives
	directiveNames := make([]string, 0, len(s.Directives))
	for name, directive := range s.Directives {
		switch directive {
		case includeDirective, skipDirective:
			continue
		default:
			directiveNames = append(directiveNames, name)
		}
	}
	sort.Strings(directiveNames)
	for _, name := range directiveNames {
		renderDirective(&b, s.Directives[name])
	}

	out := strings.TrimRight(b.String(), "\n") + "\n"
	return out
}

// ----- render helpers -----

func renderDescription(b *strings.Builder, desc string) {
	if desc == "" {
		return
	}
	b.WriteString("\"\"\"\n")
	// Escape quotes in description
	escaped := strings.ReplaceAll(desc, "\"", "\\\"")
	b.WriteString(escaped)
	b.WriteString("\n\"\"\"\n")
}

func renderScalar(b *strings.Builder, typ *Type) {
	renderDescription(b, typ.Description)
	b.WriteString("scalar ")
	b.WriteString(typ.Name)
	if typ.SpecifiedByURL != nil {
		b.WriteString(" @specifiedBy(url: \"")
		b.WriteString(*typ.SpecifiedByURL)
		b.WriteString("\")")
	}
	b.WriteString("\n\n")
}

func renderEnum(b *strings.Builder, typ *Type) {
	renderDescription(b, typ.Description)
	b.WriteString("enum ")
	b.WriteString(typ.Name)
	b.WriteString(" {\n")
	for _, val := range typ.EnumValues {
		renderDescription(b, val.Description)
		b.WriteString("  ")
		b.WriteString(val.Name)
		if val.IsDeprecated {
			b.WriteString(" @deprecated")
			if val.DeprecationReason != "" {
				b.WriteString("(reason: \"")
				b.WriteString(val.DeprecationReason)
				b.WriteString("\")")
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n\n")
}

func renderInputObject(b *strings.Builder, typ *Type) {
	renderDescription(b, typ.Description)
	b.WriteString("input ")
	b.WriteString(typ.Name)
	if typ.OneOf {
		b.WriteString(" @oneOf")
	}
	b.WriteString(" {\n")
	for _, field := range typ.InputFields {
		renderDescription(b, field.Description)
		b.WriteString("  ")
		b.WriteString(field.Name)
		b.WriteString(": ")
		b.WriteString(renderTypeRef(field.Type))
		if field.DefaultValue != nil {
			b.WriteString(" = ")
			b.WriteString(renderValue(field.DefaultValue))
		}
		if field.IsDeprecated {
			b.WriteString(" @deprecated")
			if field.DeprecationReason != "" {
				b.WriteString("(reason: \"")
				b.WriteString(field.DeprecationReason)
				b.WriteString("\")")
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n\n")
}

func renderObject(b *strings.Builder, typ *Type) {
	renderDescription(b, typ.Description)
	b.WriteString("type ")
	b.WriteString(typ.Name)
	if len(typ.Interfaces) > 0 {
		b.WriteString(" implements ")
		for i, iface := range typ.Interfaces {
			if i > 0 {
				b.WriteString(" & ")
			}
			b.WriteString(iface)
		}
	}
	b.WriteString(" {\n")
	for _, field := range typ.Fields {
		renderField(b, field)
	}
	b.WriteString("}\n\n")
}

func renderInterface(b *strings.Builder, typ *Type) {
	renderDescription(b, typ.Description)
	b.WriteString("interface ")
	b.WriteString(typ.Name)
	if len(typ.Interfaces) > 0 {
		b.WriteString(" implements ")
		for i, iface := range typ.Interfaces {
			if i > 0 {
				b.WriteString(" & ")
			}
			b.WriteString(iface)
		}
	}
	b.WriteString(" {\n")
	for _, field := range typ.Fields {
		renderField(b, field)
	}
	b.WriteString("}\n\n")
}

func renderUnion(b *strings.Builder, typ *Type) {
	renderDescription(b, typ.Description)
	b.WriteString("union ")
	b.WriteString(typ.Name)
	b.WriteString(" = ")
	for i, possibleType := range typ.PossibleTypes {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(possibleType)
	}
	b.WriteString("\n\n")
}

func renderField(b *strings.Builder, field *Field) {
	renderDescription(b, field.Description)
	b.WriteString("  ")
	b.WriteString(field.Name)
	if len(field.Arguments) > 0 {
		b.WriteString("(")
		for i, arg := range field.Arguments {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(arg.Name)
			b.WriteString(": ")
			b.WriteString(renderTypeRef(arg.Type))
			if arg.DefaultValue != nil {
				b.WriteString(" = ")
				b.WriteString(renderValue(arg.DefaultValue))
			}
		}
		b.WriteString(")")
	}
	b.WriteString(": ")
	b.WriteString(renderTypeRef(field.Type))

	// Skip @resolve directive as it's protograph-internal
	if field.IsDeprecated {
		b.WriteString(" @deprecated")
		if field.DeprecationReason != "" {
			b.WriteString("(reason: \"")
			b.WriteString(field.DeprecationReason)
			b.WriteString("\")")
		}
	}

	b.WriteString("\n")
}

func renderDirective(b *strings.Builder, directive *Directive) {
	renderDescription(b, directive.Description)
	b.WriteString("directive @")
	b.WriteString(directive.Name)
	if len(directive.Arguments) > 0 {
		b.WriteString("(")
		for i, arg := range directive.Arguments {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(arg.Name)
			b.WriteString(": ")
			b.WriteString(renderTypeRef(arg.Type))
			if arg.DefaultValue != nil {
				b.WriteString(" = ")
				b.WriteString(renderValue(arg.DefaultValue))
			}
		}
		b.WriteString(")")
	}
	if directive.IsRepeatable {
		b.WriteString(" repeatable")
	}
	b.WriteString(" on ")
	for i, location := range directive.Locations {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(string(location))
	}
	b.WriteString("\n\n")
}

func renderTypeRef(typeRef *TypeRef) string {
	if typeRef == nil {
		return ""
	}

	switch typeRef.Kind {
	case TypeRefKindNamed:
		return typeRef.Named
	case TypeRefKindList:
		return "[" + renderTypeRef(typeRef.OfType) + "]"
	case TypeRefKindNonNull:
		return renderTypeRef(typeRef.OfType) + "!"
	default:
		return ""
	}
}

// renderValue renders a GraphQL value (for default values, directive arguments, etc.)
func renderValue(value any) string {
	if value == nil {
		return "null"
	}

	switch v := value.(type) {
	case string:
		return strconv.Quote(v)
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case []any:
		var parts []string
		for _, item := range v {
			parts = append(parts, renderValue(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		var parts []string
		for k, val := range v {
			parts = append(parts, k+": "+renderValue(val))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		// For enum values and other unquoted strings
		return fmt.Sprint(v)
	}
}
