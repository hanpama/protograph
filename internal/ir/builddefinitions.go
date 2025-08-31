package ir

import (
	language "github.com/hanpama/protograph/internal/language"
)

func (b *builder) populateDefinitions() error {
	for svcId, doc := range b.serviceDocs {
		for _, node := range doc.Definitions {
			if _, ok := b.Definitions[node.Name]; ok {
				b.addViolation(violationDefinitionAlreadyExists(node.Name, node.Position))
				continue
			}

			def := &Definition{}

			switch node.Kind {
			case language.Object:
				def.Object = newObjectDefinition(node)
				b.validateObjectFieldsExist(node)
			case language.Interface:
				def.Interface = newInterfaceDefinition(node)
				b.validateInterfaceFieldsExist(node)
			case language.Union:
				def.Union = newUnionDefinition(node)
			case language.InputObject:
				def.Input = newInputDefinition(node)
			case language.Enum:
				def.Enum = newEnumDefinition(node)
			case language.Scalar:
				def.Scalar = newScalarDefinition(node)
			default:
				panic("unreachable")
			}

			b.Definitions[node.Name] = def
			b.Services[svcId].Definitions = append(b.Services[svcId].Definitions, node.Name)
		}
	}

	for _, doc := range b.serviceDocs {
		for _, node := range doc.Extensions {
			def := b.Definitions[node.Name]
			if def == nil {
				b.addViolation(violationDefinitionNotFoundForExtension(node.Name, node.Position))
				// Skip further checks to avoid nil pointer dereference when base definition is missing
				continue
			}

			switch node.Kind {
			case language.Object:
				if def.Object == nil {
					b.addViolation(violationUnexpectedTypeForExtension(node, "object"))
				}
			case language.Interface:
				if def.Interface == nil {
					b.addViolation(violationUnexpectedTypeForExtension(node, "interface"))
				}
			case language.Union:
				if def.Union == nil {
					b.addViolation(violationUnexpectedTypeForExtension(node, "union"))
				}
			case language.InputObject:
				if def.Input == nil {
					b.addViolation(violationUnexpectedTypeForExtension(node, "input"))
				}
			case language.Enum:
				if def.Enum == nil {
					b.addViolation(violationUnexpectedTypeForExtension(node, "enum"))
				}
			case language.Scalar:
				if def.Scalar == nil {
					b.addViolation(violationUnexpectedTypeForExtension(node, "scalar"))
				}
			default:
				panic("unreachable")
			}
		}
	}

	if len(b.violations) > 0 {
		return ValidationError(b.violations)
	}
	return nil
}

func newObjectDefinition(node *language.Definition) *ObjectDefinition {
	return &ObjectDefinition{
		Name:        node.Name,
		Description: node.Description,
		Fields:      make(map[string]*FieldDefinition, len(node.Fields)),
		Interfaces:  make(map[string]*InterfaceImpl, len(node.Interfaces)),
	}
}

func newInterfaceDefinition(node *language.Definition) *InterfaceDefinition {
	return &InterfaceDefinition{
		Name:        node.Name,
		Description: node.Description,
		Fields:      make(map[string]*FieldDefinition, len(node.Fields)),
		Interfaces:  make(map[string]*InterfaceImpl, len(node.Interfaces)),
	}
}

func newUnionDefinition(node *language.Definition) *UnionDefinition {
	return &UnionDefinition{
		Name:        node.Name,
		Description: node.Description,
		Types:       make(map[string]*UnionTypeDefinition, len(node.Types)),
	}
}

func newInputDefinition(node *language.Definition) *InputDefinition {
	return &InputDefinition{
		Name:        node.Name,
		Description: node.Description,
		InputValues: make(map[string]*InputValueDefinition, len(node.Fields)),
		OneOf:       false,
	}
}

func newEnumDefinition(node *language.Definition) *EnumDefinition {
	return &EnumDefinition{
		Name:        node.Name,
		Description: node.Description,
		Values:      make(map[string]*EnumValueDefinition, len(node.EnumValues)),
	}
}

func newScalarDefinition(node *language.Definition) *ScalarDefinition {
	return &ScalarDefinition{
		Name:              node.Name,
		Description:       node.Description,
		SpecifiedByURL:    "",
		MappedToProtoType: "",
	}
}

func (b *builder) validateObjectFieldsExist(node *language.Definition) {
	if len(node.Fields) == 0 {
		b.addViolation(violationObjectMustHaveField(node.Name, node.Position))
	}
}

func (b *builder) validateInterfaceFieldsExist(node *language.Definition) {
	if len(node.Fields) == 0 {
		b.addViolation(violationInterfaceMustHaveField(node.Name, node.Position))
	}
}
