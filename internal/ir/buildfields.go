package ir

import (
	"strings"

	language "github.com/hanpama/protograph/internal/language"
)

func (b *builder) populateReferences() error {
	for _, doc := range b.serviceDocs {
		for _, node := range doc.Definitions {
			b.populateDefinitionReference(b.Definitions[node.Name], node)
		}
	}
	for _, doc := range b.serviceDocs {
		for _, node := range doc.Extensions {
			b.populateDefinitionReference(b.Definitions[node.Name], node)
		}
	}
	if len(b.violations) > 0 {
		return ValidationError(b.violations)
	}
	return nil
}

func (b *builder) populateDefinitionReference(def *Definition, node *language.Definition) {
	switch node.Kind {
	case language.Object:
		b.extendObjectDefinition(def.Object, node)
	case language.Interface:
		b.extendInterfaceDefinition(def.Interface, node)
	case language.Union:
		// NOOP
	case language.InputObject:
		b.extendInputDefinition(def.Input, node)
	case language.Enum:
		b.extendEnumDefinition(def.Enum, node)
	case language.Scalar:
		// NOOP
	default:
		panic("unreachable")
	}
}

func (b *builder) extendObjectDefinition(def *ObjectDefinition, node *language.Definition) {
	for _, fieldNode := range node.Fields {
		if strings.HasPrefix(fieldNode.Name, "__") {
			b.addViolation(violationReservedFieldPrefix("Field", fieldNode.Name, fieldNode.Position))
			continue
		}
		if _, ok := def.Fields[fieldNode.Name]; ok {
			b.addViolation(violationDuplicateField("object", fieldNode.Name, node.Name, fieldNode.Position))
		}
		def.Fields[fieldNode.Name] = b.populateFieldDefinition(len(def.Fields), fieldNode)

	}
}

func (b *builder) extendInterfaceDefinition(def *InterfaceDefinition, node *language.Definition) {
	for _, fieldNode := range node.Fields {
		if strings.HasPrefix(fieldNode.Name, "__") {
			b.addViolation(violationReservedFieldPrefix("Field", fieldNode.Name, fieldNode.Position))
			continue
		}
		if _, ok := def.Fields[fieldNode.Name]; ok {
			b.addViolation(violationDuplicateField("interface", fieldNode.Name, node.Name, fieldNode.Position))
		}
		def.Fields[fieldNode.Name] = b.populateFieldDefinition(len(def.Fields), fieldNode)
	}
}

func (b *builder) extendInputDefinition(def *InputDefinition, node *language.Definition) {
	for _, fieldNode := range node.Fields {
		if _, ok := def.InputValues[fieldNode.Name]; ok {
			b.addViolation(violationDuplicateInputValue(fieldNode.Name, node.Name, fieldNode.Position))
		}
		def.InputValues[fieldNode.Name] = b.projectInputValueDefinition(len(def.InputValues), fieldNode)
	}
}

func (b *builder) extendEnumDefinition(def *EnumDefinition, node *language.Definition) (verrs []*Violation) {
	for _, value := range node.EnumValues {
		if _, ok := def.Values[value.Name]; ok {
			b.addViolation(violationDuplicateEnumValue(value.Name, node.Name, value.Position))
		}
		def.Values[value.Name] = b.projectEnumValueDefinition(len(def.Values), value)
	}

	return
}

func (b *builder) populateFieldDefinition(index int, node *language.FieldDefinition) *FieldDefinition {
	def := &FieldDefinition{
		Name:              node.Name,
		Description:       node.Description,
		Index:             index,
		Type:              b.projectTypeExpr(node.Type, typeExprModeOutput),
		Args:              make(map[string]*ArgumentDefinition, len(node.Arguments)),
		IsInternal:        false,
		Deprecation:       nil,
		ResolveBySource:   nil,
		ResolveByResolver: nil,
		ResolveByLoader:   nil,
	}

	for _, argNode := range node.Arguments {
		if strings.HasPrefix(argNode.Name, "__") {
			b.addViolation(violationReservedFieldPrefix("Argument", argNode.Name, argNode.Position))
			continue
		}
		def.Args[argNode.Name] = b.projectArgumentDefinition(len(def.Args), argNode)

	}
	return def
}

func (b *builder) projectArgumentDefinition(index int, node *language.ArgumentDefinition) *ArgumentDefinition {
	def := &ArgumentDefinition{
		Name:         node.Name,
		Description:  node.Description,
		Index:        index,
		Type:         b.projectTypeExpr(node.Type, typeExprModeInput),
		DefaultValue: nil,
		Deprecation:  nil,
	}
	if node.DefaultValue != nil {
		defaultValue, derr := node.DefaultValue.Value(nil)
		if derr != nil {
			b.addViolation(violationWithPosition(derr.Error(), node.DefaultValue.Position))
		}
		def.DefaultValue = defaultValue
	}

	return def
}

func (b *builder) projectInputValueDefinition(index int, node *language.FieldDefinition) *InputValueDefinition {
	def := &InputValueDefinition{
		Name:         node.Name,
		Description:  node.Description,
		Index:        index,
		Type:         b.projectTypeExpr(node.Type, typeExprModeInput),
		DefaultValue: nil,
		Deprecation:  nil,
	}

	if node.DefaultValue != nil {
		defaultValue, derr := node.DefaultValue.Value(nil)
		if derr != nil {
			b.addViolation(violationWithPosition(derr.Error(), node.DefaultValue.Position))
			return nil
		}
		def.DefaultValue = defaultValue
	}

	return def
}

func (b *builder) projectEnumValueDefinition(index int, node *language.EnumValueDefinition) *EnumValueDefinition {
	return &EnumValueDefinition{
		Name:        node.Name,
		Description: node.Description,
		Index:       index,
		Deprecation: nil,
	}
}

func (b *builder) projectTypeExpr(node *language.Type, mode typeExprMode) *TypeExpr {
	if node.NonNull {
		return &TypeExpr{
			Kind: TypeExprKindNonNull,
			OfType: b.projectTypeExpr(&language.Type{
				NamedType: node.NamedType,
				Elem:      node.Elem,
				NonNull:   false,
				Position:  node.Position,
			}, mode),
		}
	} else if node.Elem != nil {
		return &TypeExpr{
			Kind:   TypeExprKindList,
			OfType: b.projectTypeExpr(node.Elem, mode),
		}
	} else {
		def, ok := b.Definitions[node.NamedType]
		if !ok {
			b.addViolation(violationTypeNotFound(node.NamedType, node.Position))
			return nil
		}
		if mode == typeExprModeInput {
			if def.Input == nil && def.Scalar == nil && def.Enum == nil {
				b.addViolation(violationTypeNotInput(node.NamedType, node.Position))
				return nil
			}
		}
		if mode == typeExprModeOutput {
			if def.Object == nil && def.Interface == nil && def.Union == nil && def.Scalar == nil && def.Enum == nil {
				b.addViolation(violationTypeNotOutput(node.NamedType, node.Position))
				return nil
			}
		}
		return &TypeExpr{
			Kind:  TypeExprKindNamed,
			Named: node.NamedType,
		}
	}
}

type typeExprMode int

const (
	typeExprModeInput typeExprMode = iota
	typeExprModeOutput
)
