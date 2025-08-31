package ir

import (
	"fmt"

	language "github.com/hanpama/protograph/internal/language"
)

// Common reusable violation constructors (template helpers)
// NOTE: Keep messages stable to avoid breaking snapshot tests.

func violationUnknownDirectiveArgument(directive, arg string, pos *language.Position) *Violation {
	return violationWithPosition(
		"Unknown argument '"+arg+"' in @"+directive+" directive",
		pos,
	)
}

func violationUnknownDirectiveOnField(directive, fieldName, typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		"Unknown directive @"+directive+" on field "+fieldName+" of type "+typeName,
		pos,
	)
}

func violationUnknownDirectiveOnType(directive string, kind language.DefinitionKind, typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		"Unknown directive @"+directive+" on "+string(kind)+" type "+typeName,
		pos,
	)
}

// Generic helpers replacing scattered inline strings
func violationReservedFieldPrefix(kind, fieldName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("%s name %q cannot start with '__' (reserved prefix)", kind, fieldName),
		pos,
	)
}

func violationDuplicateField(kind, fieldName, typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Duplicate field %q found in %s %q", fieldName, kind, typeName),
		pos,
	)
}

func violationDuplicateInputValue(fieldName, typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Duplicate input value %q found in input %q", fieldName, typeName),
		pos,
	)
}

func violationDuplicateEnumValue(valueName, enumName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Duplicate enum value %q found in enum %q", valueName, enumName),
		pos,
	)
}

func violationTypeNotFound(typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Type %q not found in definitions", typeName),
		pos,
	)
}

func violationTypeNotInput(typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Type %q is not an input type", typeName),
		pos,
	)
}

func violationTypeNotOutput(typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Type %q is not an output type", typeName),
		pos,
	)
}

func violationObjectMustHaveField(typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Object type %q must have at least one field", typeName),
		pos,
	)
}

func violationInterfaceMustHaveField(typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Interface type %q must have at least one field", typeName),
		pos,
	)
}

func violationDirectiveAlreadyDefined(name string, pos *language.Position) *Violation {
	return violationWithPosition(
		"Directive "+name+" is already defined",
		pos,
	)
}

func violationDefinitionNotFoundForExtension(name string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("definition %q not found for extension", name),
		pos,
	)
}

func violationFieldArgsNotAllowedWithLoad(pos *language.Position) *Violation {
	return violationWithPosition(
		"Fields with @load directive must not have arguments",
		pos,
	)
}

func violationLoadResolveConflict(typeName, fieldName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Field %s on type %s cannot have both @load and @resolve directives", fieldName, typeName),
		pos,
	)
}

func violationLoaderDuplicateKeys(keyFields []string, existingTarget string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Duplicate @loader with identical keys: %v (already defined for %s)", keyFields, existingTarget),
		pos,
	)
}

func violationLoaderKeyFieldNotExist(keyField, typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("@loader key field '%s' does not exist on type %s", keyField, typeName),
		pos,
	)
}

func violationLoaderKeyFieldNotNonNull(keyField, typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("@loader key field '%s' on type %s must be Non-Null", keyField, typeName),
		pos,
	)
}

func violationLoaderKeyFieldNotScalar(keyField, typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("@loader key field '%s' on type %s must be a scalar type", keyField, typeName),
		pos,
	)
}

func violationIdFieldNotNonNull(fieldName, typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("@id field '%s' on type %s must be Non-Null", fieldName, typeName),
		pos,
	)
}

func violationIdFieldNotScalar(fieldName, typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("@id field '%s' on type %s must be a scalar type", fieldName, typeName),
		pos,
	)
}

func violationLoaderKeyConflict(pos *language.Position) *Violation {
	return violationWithPosition(
		"@loader cannot have both 'key' and 'keys' arguments",
		pos,
	)
}

func violationLoaderMappingKeysMismatch(pos *language.Position) *Violation {
	return violationWithPosition(
		"@load mapping keys do not match loader key set",
		pos,
	)
}

func violationLoadMappingUnknownParentField(parentField, typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("@load mapping references unknown parent field '%s' on type %s", parentField, typeName),
		pos,
	)
}

func violationLoaderNotFound(loaderID, fieldName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Loader with ID '%s' not found for field %s", loaderID, fieldName),
		pos,
	)
}

func violationLoadTypeMismatch(objName, parentField, srcType, targetType, key string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("@load type mismatch: parent field '%s.%s' (%s) is not assignable to target key '%s.%s' (%s)", objName, parentField, srcType, targetType, key, key),
		pos,
	)
}

func violationResolveWithKeyConflictsArg(reqField string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("@resolve 'with' key '%s' conflicts with argument name '%s'", reqField, reqField),
		pos,
	)
}

func violationResolveMappingUnknownParentField(parentField, typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("@resolve mapping references unknown parent field '%s' on type %s", parentField, typeName),
		pos,
	)
}

func violationDirectiveNoArguments(name string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Directive @%s does not accept arguments", name),
		pos,
	)
}

func violationExpectedString(pos *language.Position) *Violation {
	return violationWithPosition(
		"Expected a string value",
		pos,
	)
}

func violationExpectedList(pos *language.Position) *Violation {
	return violationWithPosition(
		"Expected a list value",
		pos,
	)
}

func violationExpectedObject(pos *language.Position) *Violation {
	return violationWithPosition(
		"Expected an object value",
		pos,
	)
}

func violationExpectedBoolean(pos *language.Position) *Violation {
	return violationWithPosition(
		"Expected a boolean value",
		pos,
	)
}

// Existing moved helpers from original file remain below
func violationSchemaAlreadyDefined(pos *language.Position) *Violation {
	return &Violation{
		Message: "Schema is already defined",
		File:    pos.Src.Name,
		Line:    pos.Start,
		Column:  pos.End,
	}
}

func violationDefinitionAlreadyExists(name string, pos *language.Position) *Violation {
	return &Violation{
		Message: fmt.Sprintf("Definition %q already exists", name),
		File:    pos.Src.Name,
		Line:    pos.Start,
		Column:  pos.End,
	}
}

func violationUnexpectedTypeForExtension(node *language.Definition, expectedType string) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Unexpected type for extension %s, expected %s", node.Name, expectedType),
		node.Position,
	)
}

func violationMissingIdFields(typeName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Type %q has no @id fields or 'id' field for @loader default key", typeName),
		pos,
	)
}

func violationArgumentConflictWithId(argName, fieldName, idField string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Argument %q in field %q conflicts with @id field %q in implicit resolver", argName, fieldName, idField),
		pos,
	)
}

func violationInterfaceDirectiveNotAllowed(directiveName, interfaceField string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Directive @%s is not allowed on interface field %q. Only concrete implementors may use protograph directives", directiveName, interfaceField),
		pos,
	)
}

func violationMissingWithArgument(directiveName string, pos *language.Position) *Violation {
	return violationWithPosition(
		fmt.Sprintf("Directive @%s requires 'with' parameter. Use 'with: {}' when no parent fields are needed", directiveName),
		pos,
	)
}

func violationSchemaDefinitionRequired() *Violation {
	return &Violation{
		Message: "Schema definition is required",
	}
}

func violationRootTypeNotFound(kind, typeName string) *Violation {
	return &Violation{
		Message: fmt.Sprintf("%s type %q not found in definitions", kind, typeName),
	}
}

func violationRootTypeNotObject(kind, typeName string) *Violation {
	return &Violation{
		Message: fmt.Sprintf("%s type %q must be an Object type", kind, typeName),
	}
}
