package ir

import (
	"fmt"
	"sort"
	"strings"

	language "github.com/hanpama/protograph/internal/language"
)

func (b *builder) populateDirectiveUses() error {
	// 1st pass: Field-level directives
	for _, doc := range b.serviceDocs {
		for _, node := range doc.Definitions {
			def := b.Definitions[node.Name]
			switch node.Kind {
			case language.Object:
				b.processObjectFieldDirectives(def.Object, node)
			case language.Interface:
				b.processInterfaceFieldDirectives(def.Interface, node)
			}
		}
		for _, node := range doc.Extensions {
			def := b.Definitions[node.Name]
			switch node.Kind {
			case language.Object:
				b.processObjectFieldDirectives(def.Object, node)
			case language.Interface:
				b.processInterfaceFieldDirectives(def.Interface, node)
			}
		}
	}

	// 2nd pass: Definition-level directives
	for svcID, doc := range b.serviceDocs {
		svc := b.Services[svcID]
		for _, node := range doc.Definitions {
			def := b.Definitions[node.Name]

			switch node.Kind {
			case language.Object:
				b.processObjectTypeDirectives(svc, def.Object, node)
			case language.Interface:
				b.checkNoDefinitionDirectiveUses(node)
			case language.Union:
				b.checkNoDefinitionDirectiveUses(node)
			case language.Scalar:
				b.processScalarTypeDirectives(def.Scalar, node)
			case language.Enum:
				b.checkNoDefinitionDirectiveUses(node)
			case language.InputObject:
				b.checkNoDefinitionDirectiveUses(node)
			}
		}
		for _, node := range doc.Extensions {
			def := b.Definitions[node.Name]

			switch node.Kind {
			case language.Object:
				b.processObjectTypeDirectives(svc, def.Object, node)
			case language.Interface:
				b.checkNoDefinitionDirectiveUses(node)
			case language.Union:
				b.checkNoDefinitionDirectiveUses(node)
			case language.Scalar:
				b.processScalarTypeDirectives(def.Scalar, node)
			case language.Enum:
				b.checkNoDefinitionDirectiveUses(node)
			case language.InputObject:
				b.checkNoDefinitionDirectiveUses(node)
			}
		}
	}

	if len(b.violations) > 0 {
		return ValidationError(b.violations)
	}

	return nil
}

func (b *builder) processObjectFieldDirectives(obj *ObjectDefinition, node *language.Definition) {
	var idFields []string
	var hasExplicitId bool

	for _, fieldNode := range node.Fields {
		for _, dir := range fieldNode.Directives {
			switch dir.Name {
			case "id":
				b.checkNoDirectiveArguments(dir)
				idFields = append(idFields, fieldNode.Name)
				hasExplicitId = true
			case "internal":
				b.checkNoDirectiveArguments(dir)
				obj.Fields[fieldNode.Name].IsInternal = true
			case "deprecated":
				obj.Fields[fieldNode.Name].Deprecation = b.projectDeprecation(dir)
			case "load", "resolve":
				// skip here. These will be processed in the next pass
			default:
				b.addViolation(violationUnknownDirectiveOnField(dir.Name, fieldNode.Name, node.Name, dir.Position))
			}
		}
	}

	// Apply implicit @id behavior if no explicit @id fields
	if !hasExplicitId {
		if _, exists := obj.Fields["id"]; exists {
			idFields = append(idFields, "id")
		}
	}

	// Validate: all idFields must be Non-Null scalar types
	for _, idFn := range idFields {
		fd := obj.Fields[idFn]
		if fd == nil {
			continue
		}
		if !b.isNonNullType(fd.Type) {
			b.addViolation(violationIdFieldNotNonNull(idFn, obj.Name, node.Position))
			continue
		}
		base := fd.Type.unwrap()
		if !b.isScalarType(base) {
			b.addViolation(violationIdFieldNotScalar(idFn, obj.Name, node.Position))
		}
	}

	obj.IDFields = idFields
}

func (b *builder) processInterfaceFieldDirectives(iface *InterfaceDefinition, node *language.Definition) {
	for _, fieldNode := range node.Fields {
		for _, dir := range fieldNode.Directives {
			switch dir.Name {
			case "deprecated":
				// @deprecated is allowed on interface fields (standard GraphQL)
				iface.Fields[fieldNode.Name].Deprecation = b.projectDeprecation(dir)
			default:
				// Spec 5.4: Interface-declared fields cannot carry protograph directives
				b.addViolation(violationInterfaceDirectiveNotAllowed(dir.Name, fieldNode.Name, fieldNode.Position))
			}
		}
	}
}

func (b *builder) processObjectTypeDirectives(svc *Service, def *ObjectDefinition, node *language.Definition) {
	for _, dir := range node.Directives {
		switch dir.Name {
		case "loader":
			b.handleLoaderDirective(svc, def, dir, node)
		default:
			b.addViolation(violationUnknownDirectiveOnType(dir.Name, node.Kind, node.Name, dir.Position))
		}
	}
}

func (b *builder) handleLoaderDirective(svc *Service, obj *ObjectDefinition, dir *language.Directive, node *language.Definition) {
	var keyFields []string
	batch := true
	hasKey := false
	hasKeys := false
	args := make(map[string]*MethodArg)

	for _, arg := range dir.Arguments {
		switch arg.Name {
		case "key":
			hasKey = true
			keyField := b.getStringValue(arg.Value)
			keyFields = append(keyFields, keyField)
		case "keys":
			hasKeys = true
			keyFields = b.getStringListValue(arg.Value)
		case "batch":
			batch = b.getBoolValue(arg.Value)
		default:
			b.addViolation(violationUnknownDirectiveArgument("loader", arg.Name, arg.Position))
		}
	}
	if hasKey && hasKeys {
		b.addViolation(violationLoaderKeyConflict(dir.Position))
		return
	}

	if len(keyFields) == 0 {
		if len(obj.IDFields) > 0 {
			keyFields = append(keyFields, obj.IDFields...)
		} else {
			b.addViolation(violationMissingIdFields(obj.Name, node.Position))
			return
		}
	}
	sort.Strings(keyFields)

	for _, keyField := range keyFields {
		fieldDef, exists := obj.Fields[keyField]
		if !exists {
			b.addViolation(violationLoaderKeyFieldNotExist(keyField, obj.Name, dir.Position))
			return
		}

		if !b.isNonNullType(fieldDef.Type) {
			b.addViolation(violationLoaderKeyFieldNotNonNull(keyField, obj.Name, dir.Position))
			return
		}
		base := fieldDef.Type.unwrap()
		if !b.isScalarType(base) {
			b.addViolation(violationLoaderKeyFieldNotScalar(keyField, obj.Name, dir.Position))
			return
		}
		args[keyField] = &MethodArg{Name: keyField, Type: fieldDef.Type, Index: len(args), Description: fieldDef.Description}
	}

	loaderDef := &LoaderDefinition{
		ID:         LoaderID(fmt.Sprintf("%s:%s", obj.Name, strings.Join(keyFields, ":"))),
		TargetType: obj.Name,
		KeyFields:  keyFields,
		Batch:      batch,
		Args:       args,
	}

	if existing, exists := b.Loaders[loaderDef.ID]; exists {
		b.addViolation(violationLoaderDuplicateKeys(keyFields, existing.TargetType, dir.Position))
		return
	}

	b.Loaders[loaderDef.ID] = loaderDef
	svc.Loaders = append(svc.Loaders, loaderDef.ID)
}

func (b *builder) processScalarTypeDirectives(def *ScalarDefinition, node *language.Definition) {
	for _, dir := range node.Directives {
		switch dir.Name {
		case "mapScalar":
			def.MappedToProtoType = b.projectMapScalar(dir)
		default:
			b.addViolation(violationUnknownDirectiveOnType(dir.Name, node.Kind, node.Name, dir.Position))
		}
	}
}

func (b *builder) projectMapScalar(dir *language.Directive) string {
	var protoType string

	// Parse toProtobuf argument
	for _, arg := range dir.Arguments {
		switch arg.Name {
		case "toProtobuf":
			protoType = b.getStringValue(arg.Value)
		default:
			b.addViolation(violationUnknownDirectiveArgument("mapScalar", arg.Name, arg.Position))
		}
	}
	if protoType == "" {
		protoType = "string"
	}

	return protoType
}

func (b *builder) projectDeprecation(dir *language.Directive) *Deprecation {
	reason := "No longer supported"

	for _, arg := range dir.Arguments {
		switch arg.Name {
		case "reason":
			reason = b.getStringValue(arg.Value)
		default:
			b.addViolation(violationUnknownDirectiveArgument("deprecated", arg.Name, arg.Position))
		}
	}

	return &Deprecation{
		Reason: reason,
	}
}

func (b *builder) checkNoDefinitionDirectiveUses(node *language.Definition) {
	for _, dir := range node.Directives {
		violations := []*Violation{violationUnknownDirectiveOnType(dir.Name, node.Kind, node.Name, dir.Position)}
		b.addViolation(violations...)
	}
}

func (b *builder) checkNoDirectiveArguments(node *language.Directive) {
	for _, arg := range node.Arguments {
		violations := []*Violation{violationDirectiveNoArguments(node.Name, arg.Position)}
		b.addViolation(violations...)
	}
}

func (b *builder) isRootObject(name string) bool {
	if b.Schema == nil {
		return false
	}
	return (b.Schema.QueryType != "" && b.Schema.QueryType == name) ||
		(b.Schema.MutationType != "" && b.Schema.MutationType == name) ||
		(b.Schema.SubscriptionType != "" && b.Schema.SubscriptionType == name)
}

// isScalarType reports whether a named type is a scalar (built-in or custom scalar definition).
func (b *builder) isScalarType(name string) bool {
	return b.Definitions[name].Scalar != nil
}
