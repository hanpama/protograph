package ir

import (
	"fmt"

	language "github.com/hanpama/protograph/internal/language"
)

func (b *builder) populateImplementations() error {
	for _, doc := range b.serviceDocs {
		for _, node := range doc.Definitions {
			b.populateDefinitionImplementation(b.Definitions[node.Name], node)
		}
	}
	for _, doc := range b.serviceDocs {
		for _, node := range doc.Extensions {
			b.populateDefinitionImplementation(b.Definitions[node.Name], node)
		}
	}
	if len(b.violations) > 0 {
		return ValidationError(b.violations)
	}
	return nil
}

func (b *builder) populateDefinitionImplementation(def *Definition, node *language.Definition) {
	switch node.Kind {
	case language.Object:
		b.populateObjectImplementations(def.Object, node)
	case language.Interface:
		b.populateInterfaceImplementations(def.Interface, node)
	case language.Union:
		b.populateUnionMembers(def.Union, node)
	default:
		// Other types don't have implementations
	}
}

func (b *builder) populateObjectImplementations(def *ObjectDefinition, node *language.Definition) {
	for i, interfaceName := range node.Interfaces {
		def.Interfaces[interfaceName] = &InterfaceImpl{
			Interface: interfaceName,
			Index:     i,
		}

		// Validate interface implementation
		interfaceDef, ok := b.Definitions[interfaceName]
		if !ok {
			b.addViolation(violationWithPosition(
				fmt.Sprintf("Interface %q not found for object %q", interfaceName, node.Name),
				node.Position,
			))
			continue
		}
		if interfaceDef.Interface == nil {
			b.addViolation(violationWithPosition(
				fmt.Sprintf("Type %q is not an interface", interfaceName),
				node.Position,
			))
			continue
		}

		b.validateInterfaceImplementation(def, interfaceDef.Interface, node.Name, node.Position)
		interfaceDef.Interface.PossibleTypes = append(interfaceDef.Interface.PossibleTypes, node.Name)
	}
}

func (b *builder) populateInterfaceImplementations(def *InterfaceDefinition, node *language.Definition) {
	for i, interfaceName := range node.Interfaces {
		def.Interfaces[interfaceName] = &InterfaceImpl{
			Interface: interfaceName,
			Index:     i,
		}

		// Validate interface implementation
		interfaceDef, ok := b.Definitions[interfaceName]
		if !ok {
			b.addViolation(violationWithPosition(
				fmt.Sprintf("Interface %q not found for interface %q", interfaceName, node.Name),
				node.Position,
			))
			continue
		}
		if interfaceDef.Interface == nil {
			b.addViolation(violationWithPosition(
				fmt.Sprintf("Type %q is not an interface", interfaceName),
				node.Position,
			))
			continue
		}

		// For interface-to-interface implementation
		b.validateInterfaceToInterfaceImplementation(def, interfaceDef.Interface, node.Name, node.Position)
	}
}

func (b *builder) populateUnionMembers(def *UnionDefinition, node *language.Definition) {
	for i, typeName := range node.Types {
		memberDef, ok := b.Definitions[typeName]
		if !ok {
			b.addViolation(violationWithPosition(
				fmt.Sprintf("Type %q not found for union %q", typeName, node.Name),
				node.Position,
			))
			continue
		}
		if memberDef.Object == nil {
			b.addViolation(violationWithPosition(
				fmt.Sprintf("Union member %q must be an Object type, but got %s", typeName, getTypeKind(memberDef)),
				node.Position,
			))
			continue
		}

		def.Types[typeName] = &UnionTypeDefinition{
			Name:  typeName,
			Index: i,
		}
	}
}

func (b *builder) validateInterfaceImplementation(objDef *ObjectDefinition, interfaceDef *InterfaceDefinition, objName string, pos *language.Position) {
	// Check that object implements all interfaces that the interface implements
	for implementedInterface := range interfaceDef.Interfaces {
		if _, ok := objDef.Interfaces[implementedInterface]; !ok {
			b.addViolation(violationWithPosition(
				fmt.Sprintf("Object %q must also implement interface %q (required by interface %q)",
					objName, implementedInterface, interfaceDef.Name),
				pos,
			))
		}
	}

	// Check that object has all fields from the interface
	for fieldName, interfaceField := range interfaceDef.Fields {
		objField, ok := objDef.Fields[fieldName]
		if !ok {
			b.addViolation(violationWithPosition(
				fmt.Sprintf("Object %q is missing field %q required by interface %q",
					objName, fieldName, interfaceDef.Name),
				pos,
			))
			continue
		}

		b.validateFieldImplementation(objField, interfaceField, objName, interfaceDef.Name, pos)
	}
}

func (b *builder) validateInterfaceToInterfaceImplementation(implDef *InterfaceDefinition, interfaceDef *InterfaceDefinition, implName string, pos *language.Position) {
	// Check that implementing interface has all interfaces that the base interface implements
	for implementedInterface := range interfaceDef.Interfaces {
		if _, ok := implDef.Interfaces[implementedInterface]; !ok {
			b.addViolation(violationWithPosition(
				fmt.Sprintf("Interface %q must also implement interface %q (required by interface %q)",
					implName, implementedInterface, interfaceDef.Name),
				pos,
			))
		}
	}

	// Check that implementing interface has all fields from the base interface
	for fieldName, interfaceField := range interfaceDef.Fields {
		implField, ok := implDef.Fields[fieldName]
		if !ok {
			b.addViolation(violationWithPosition(
				fmt.Sprintf("Interface %q is missing field %q required by interface %q",
					implName, fieldName, interfaceDef.Name),
				pos,
			))
			continue
		}

		b.validateFieldImplementation(implField, interfaceField, implName, interfaceDef.Name, pos)
	}
}

func (b *builder) validateFieldImplementation(field *FieldDefinition, interfaceField *FieldDefinition, typeName string, interfaceName string, pos *language.Position) {
	// Validate arguments (invariant)
	for argName, interfaceArg := range interfaceField.Args {
		fieldArg, ok := field.Args[argName]
		if !ok {
			b.addViolation(violationWithPosition(
				fmt.Sprintf("Field %q.%q is missing argument %q required by interface %q",
					typeName, field.Name, argName, interfaceName),
				pos,
			))
			continue
		}

		// Arguments must have exactly the same type (invariant)
		if !b.typesAreEqual(fieldArg.Type, interfaceArg.Type) {
			b.addViolation(violationWithPosition(
				fmt.Sprintf("Argument %q of field %q.%q has type %s but interface %q expects %s",
					argName, typeName, field.Name,
					fieldArg.Type.String(),
					interfaceName,
					interfaceArg.Type.String()),
				pos,
			))
		}
	}

	// Check for additional required arguments (must have default values)
	for argName, fieldArg := range field.Args {
		if _, ok := interfaceField.Args[argName]; !ok {
			// Additional argument must be nullable (not NonNull)
			if b.isNonNullType(fieldArg.Type) {
				b.addViolation(violationWithPosition(
					fmt.Sprintf("Additional argument %q of field %q.%q must be nullable (interface %q doesn't have this argument)",
						argName, typeName, field.Name, interfaceName),
					pos,
				))
			}
		}
	}

	// Validate return type (covariant)
	if !b.isValidImplementationFieldType(field.Type, interfaceField.Type) {
		b.addViolation(violationWithPosition(
			fmt.Sprintf("Field %q.%q has type %s but interface %q expects %s (or a subtype)",
				typeName, field.Name,
				field.Type.String(),
				interfaceName,
				interfaceField.Type.String()),
			pos,
		))
	}
}

func (b *builder) isValidImplementationFieldType(fieldType, implementedFieldType *TypeExpr) bool {
	if fieldType == nil || implementedFieldType == nil {
		return false
	}

	// Non-Null unwrapping rule
	if fieldType.Kind == TypeExprKindNonNull {
		nullableType := fieldType.OfType
		var implementedNullableType *TypeExpr
		if implementedFieldType.Kind == TypeExprKindNonNull {
			implementedNullableType = implementedFieldType.OfType
		} else {
			implementedNullableType = implementedFieldType
		}
		return b.isValidImplementationFieldType(nullableType, implementedNullableType)
	}

	// List covariance rule
	if fieldType.Kind == TypeExprKindList && implementedFieldType.Kind == TypeExprKindList {
		return b.isValidImplementationFieldType(fieldType.OfType, implementedFieldType.OfType)
	}

	// Same type
	if b.typesAreEqual(fieldType, implementedFieldType) {
		return true
	}

	// For named types, check covariance
	if fieldType.Kind == TypeExprKindNamed && implementedFieldType.Kind == TypeExprKindNamed {
		fieldDef, ok := b.Definitions[fieldType.Named]
		if !ok {
			return false
		}
		implementedDef, ok := b.Definitions[implementedFieldType.Named]
		if !ok {
			return false
		}

		// Object covariant to Union
		if fieldDef.Object != nil && implementedDef.Union != nil {
			// Check if object is a member of the union
			if _, ok := implementedDef.Union.Types[fieldType.Named]; ok {
				return true
			}
		}

		// Object/Interface covariant to Interface
		if implementedDef.Interface != nil {
			if fieldDef.Object != nil {
				// Check if object implements the interface
				if _, ok := fieldDef.Object.Interfaces[implementedFieldType.Named]; ok {
					return true
				}
			} else if fieldDef.Interface != nil {
				// Check if interface implements the interface
				if _, ok := fieldDef.Interface.Interfaces[implementedFieldType.Named]; ok {
					return true
				}
			}
		}
	}

	return false
}

func (b *builder) typesAreEqual(t1, t2 *TypeExpr) bool {
	if t1 == nil || t2 == nil {
		return t1 == t2
	}

	if t1.Kind != t2.Kind {
		return false
	}

	switch t1.Kind {
	case TypeExprKindNamed:
		return t1.Named == t2.Named
	case TypeExprKindList, TypeExprKindNonNull:
		return b.typesAreEqual(t1.OfType, t2.OfType)
	default:
		return false
	}
}

func (b *builder) isNonNullType(t *TypeExpr) bool {
	return t != nil && t.Kind == TypeExprKindNonNull
}

func getTypeKind(def *Definition) string {
	if def.Object != nil {
		return "Object"
	}
	if def.Interface != nil {
		return "Interface"
	}
	if def.Union != nil {
		return "Union"
	}
	if def.Input != nil {
		return "InputObject"
	}
	if def.Enum != nil {
		return "Enum"
	}
	if def.Scalar != nil {
		return "Scalar"
	}
	return "Unknown"
}
