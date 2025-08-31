package ir

import (
	"fmt"
	"sort"
	"strings"

	language "github.com/hanpama/protograph/internal/language"
)

func (b *builder) setFieldResolution() error {

	// 3rd pass: Field resolution directives
	for svcId, doc := range b.serviceDocs {
		svc := b.Services[svcId]
		for _, node := range doc.Definitions {
			switch node.Kind {
			case language.Object:
				obj := b.Definitions[node.Name].Object
				for _, fieldNode := range node.Fields {
					field := obj.Fields[fieldNode.Name]
					b.processFieldResolution(svc, field, fieldNode, obj, false)
				}
			}
		}
		for _, node := range doc.Extensions {
			switch node.Kind {
			case language.Object:
				obj := b.Definitions[node.Name].Object
				for _, fieldNode := range node.Fields {
					field := obj.Fields[fieldNode.Name]
					b.processFieldResolution(svc, field, fieldNode, obj, true)
				}
			}
		}
	}
	if len(b.violations) > 0 {
		return ValidationError(b.violations)
	}
	return nil
}

func (b *builder) processFieldResolution(svc *Service, field *FieldDefinition, fieldNode *language.FieldDefinition, obj *ObjectDefinition, isExt bool) {
	// Pre-scan for conflicting directives (@load + @resolve together)
	hasLoad := false
	hasResolve := false
	for _, dir := range fieldNode.Directives {
		if dir.Name == "load" {
			hasLoad = true
		}
		if dir.Name == "resolve" {
			hasResolve = true
		}
	}
	if hasLoad && hasResolve {
		b.addViolation(violationLoadResolveConflict(obj.Name, fieldNode.Name, fieldNode.Position))
		return // abort further processing to avoid ambiguous resolution fallback
	}

	// Check for @load and @resolve directives
	for _, dir := range fieldNode.Directives {
		switch dir.Name {
		case "load":
			b.handleLoadDirective(field, dir, fieldNode, obj)
		case "resolve":
			b.handleResolveDirective(svc, obj, field, dir, fieldNode)
		}
	}

	isRoot := b.isRootObject(obj.Name)
	// Implicit resolver conditions:
	// 1. No explicit @resolve / @load already applied
	// 2. Either field has arguments OR parent object is a root (schema-configured)
	if field.ResolveByResolver == nil && field.ResolveByLoader == nil && (len(fieldNode.Arguments) > 0 || isRoot) || isExt {
		b.handleImplicitResolver(svc, obj, fieldNode, field)
	}

	// If still unresolved and not a root object, resolve by source
	if field.ResolveByResolver == nil && field.ResolveByLoader == nil && !isRoot {
		field.ResolveBySource = &FieldResolveBySource{SourceField: fieldNode.Name}
	}
}

func (b *builder) handleLoadDirective(field *FieldDefinition, dir *language.Directive, fieldNode *language.FieldDefinition, obj *ObjectDefinition) {
	// Spec: @load fields must not define arguments
	if len(fieldNode.Arguments) > 0 {
		b.addViolation(violationFieldArgsNotAllowedWithLoad(fieldNode.Position))
		return
	}

	var withMapping map[string]string
	var hasWithArg bool
	for _, arg := range dir.Arguments {
		switch arg.Name {
		case "with":
			hasWithArg = true
			withMapping = b.getStringMapValue(arg.Value)
		default:
			b.addViolation(violationUnknownDirectiveArgument("load", arg.Name, arg.Position))
		}
	}
	if !hasWithArg || len(withMapping) == 0 {
		b.addViolation(violationMissingWithArgument("load", dir.Position))
		return
	}

	// Mapping orientation: key = target loader key field, value = parent field name
	// Collect target key fields
	var keyFields []string
	for loaderKey := range withMapping {
		keyFields = append(keyFields, loaderKey)
	}
	sort.Strings(keyFields)

	// Determine target type (base named type of the field return type)
	targetType := field.Type.unwrap()
	loaderID := LoaderID(fmt.Sprintf("%s:%s", targetType, strings.Join(keyFields, ":")))

	loaderDef, found := b.Loaders[loaderID]
	if !found {
		b.addViolation(violationLoaderNotFound(string(loaderID), fieldNode.Name, fieldNode.Position))
		return
	}

	// Validate each parent source field exists and types are (optionally) compatible
	for _, parentFieldName := range withMapping {
		if _, exists := obj.Fields[parentFieldName]; !exists {
			b.addViolation(violationLoadMappingUnknownParentField(parentFieldName, obj.Name, fieldNode.Position))
			return
		}
	}

	// Ensure mapping keys match exactly the loader key set (already sorted)
	if len(loaderDef.KeyFields) != len(keyFields) {
		b.addViolation(violationLoaderMappingKeysMismatch(dir.Position))
		return
	}
	for i, k := range keyFields {
		if loaderDef.KeyFields[i] != k {
			b.addViolation(violationLoaderMappingKeysMismatch(dir.Position))
			return
		}
	}

	// Type compatibility: each parent source field type must be assignable to the target key field type
	// Determine target type definition (OBJECT)
	def := b.Definitions[targetType]
	targetObj := def.Object
	for _, k := range keyFields {
		parentFieldName := withMapping[k]
		srcField := obj.Fields[parentFieldName]
		tgtField := targetObj.Fields[k]
		if !b.areTypesAssignableForLoad(srcField.Type, tgtField.Type) {
			b.addViolation(violationLoadTypeMismatch(obj.Name, parentFieldName, srcField.Type.String(), targetType, k, fieldNode.Position))
			return
		}
	}

	field.ResolveByLoader = &FieldResolveByLoader{LoaderID: loaderID, With: withMapping}
}

func (b *builder) handleResolveDirective(svc *Service, def *ObjectDefinition, field *FieldDefinition, dir *language.Directive, fieldNode *language.FieldDefinition) {
	var violations []*Violation
	var withMapping map[string]string
	batch := false // default
	var hasWithArg bool

	for _, arg := range dir.Arguments {
		switch arg.Name {
		case "with":
			hasWithArg = true
			withMapping = b.getStringMapValue(arg.Value)
		case "batch":
			batch = b.getBoolValue(arg.Value)
		default:
			violations = append(violations, violationUnknownDirectiveArgument("resolve", arg.Name, arg.Position))
		}
	}

	// Build args: start with declared GraphQL arguments
	args := make(map[string]*MethodArg)
	for _, arg := range field.Args {
		args[arg.Name] = &MethodArg{Name: arg.Name, Type: arg.Type, Index: len(args), Description: arg.Description}
	}

	// Default mapping when `with` is omitted: include all @id fields (reqField == parentField)
	if !hasWithArg {
		withMapping = make(map[string]string)
		for _, idFn := range def.IDFields {
			withMapping[idFn] = idFn
		}
	}

	// Validate mapping: key=request field name, value=parent field name
	for reqField, parentField := range withMapping {
		if _, exists := args[reqField]; exists {
			violations = append(violations, violationResolveWithKeyConflictsArg(reqField, fieldNode.Position))
		}
		parentObjField, ok := def.Fields[parentField]
		if !ok {
			violations = append(violations, violationResolveMappingUnknownParentField(parentField, def.Name, fieldNode.Position))
			continue
		}
		args[reqField] = &MethodArg{Name: reqField, Type: parentObjField.Type, Index: len(args), Description: parentObjField.Description}
	}

	if len(violations) > 0 {
		b.addViolation(violations...)
		return
	}

	resolverID := ResolverID(fmt.Sprintf("%s:%s", def.Name, fieldNode.Name))
	resolverDef := &ResolverDefinition{
		ID:          resolverID,
		Parent:      def.Name,
		Description: field.Description,
		Field:       fieldNode.Name,
		Args:        args,
		Batch:       batch,
		ReturnType:  field.Type,
	}
	resolverUse := &FieldResolveByResolver{ResolverID: resolverDef.ID, With: withMapping}

	b.Resolvers[resolverDef.ID] = resolverDef
	svc.Resolvers = append(svc.Resolvers, resolverDef.ID)
	field.ResolveByResolver = resolverUse
}

func (b *builder) handleImplicitResolver(svc *Service, obj *ObjectDefinition, fieldNode *language.FieldDefinition, field *FieldDefinition) {
	var violations []*Violation
	resolverID := ResolverID(fmt.Sprintf("%s:%s", obj.Name, fieldNode.Name))

	args := make(map[string]*MethodArg)
	for _, arg := range field.Args { // existing GraphQL args
		args[arg.Name] = &MethodArg{Name: arg.Name, Type: arg.Type, Index: len(args), Description: arg.Description}
	}

	withMapping := make(map[string]string)
	for _, idFn := range obj.IDFields { // add all @id fields to args & mapping
		if existingArgType, conflict := args[idFn]; conflict {
			_ = existingArgType
			violations = append(violations, violationArgumentConflictWithId(idFn, fieldNode.Name, idFn, fieldNode.Position))
			continue
		}
		idField := obj.Fields[idFn]
		if idField != nil {
			args[idFn] = &MethodArg{Name: idFn, Type: idField.Type, Index: len(args), Description: idField.Description}
			withMapping[idFn] = idFn // request arg name == parent field name
		}
	}

	if len(violations) > 0 {
		b.addViolation(violations...)
		return
	}

	resolverDef := &ResolverDefinition{
		ID:          resolverID,
		Parent:      obj.Name,
		Description: field.Description,
		Field:       fieldNode.Name,
		Args:        args,
		Batch:       false,
		ReturnType:  field.Type,
	}
	resolverUse := &FieldResolveByResolver{ResolverID: resolverDef.ID, With: withMapping}

	b.Resolvers[resolverDef.ID] = resolverDef
	svc.Resolvers = append(svc.Resolvers, resolverDef.ID)
	field.ResolveByResolver = resolverUse
}

// areTypesAssignableForLoad checks if a source value type can be assigned to a target key type for @load.
// Current rule: unwrap Non-Null on both sides; both must be NAMED types with identical base names.
func (b *builder) areTypesAssignableForLoad(src, tgt *TypeExpr) bool {
	// unwrap Non-Null
	if src.Kind == TypeExprKindNonNull && src.OfType != nil {
		src = src.OfType
	}
	if tgt.Kind == TypeExprKindNonNull && tgt.OfType != nil {
		tgt = tgt.OfType
	}
	if src.Kind != TypeExprKindNamed || tgt.Kind != TypeExprKindNamed {
		return false
	}
	return src.Named == tgt.Named
}
