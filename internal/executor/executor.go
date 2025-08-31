package executor

import (
	"context"
	"fmt"
	"reflect"

	language "github.com/hanpama/protograph/internal/language"
	schema "github.com/hanpama/protograph/internal/schema"
)

type Path []PathElement

type PathElement any

type NodeID uint64

// executionState holds the state during query execution
type executionState struct {
	runtime        Runtime
	schema         *schema.Schema
	document       *language.QueryDocument
	variableValues map[string]any
	context        context.Context
	asyncTaskGroup []asyncTask
	errors         []GraphQLError
	// Store async tasks by ID for completion
	asyncTaskInfo map[NodeID]asyncTask
	// simple incremental id generator
	nextID uint64
	// prefixes of paths that have been nullified (tombstoned)
	nullifiedPrefix map[string]struct{}
}

// asyncTask represents a pending async field resolution
type asyncTask struct {
	ID           NodeID
	Task         AsyncResolveTask
	ResponsePath Path
	FieldType    *schema.TypeRef
	Fields       []*language.Field
}

type asyncPending struct{}

type Executor struct {
	runtime Runtime
	schema  *schema.Schema
}

func NewExecutor(runtime Runtime, schema *schema.Schema) *Executor {
	return &Executor{runtime: runtime, schema: schema}
}

func (e *Executor) ExecuteRequest(
	ctx context.Context,
	document *language.QueryDocument,
	operationName string,
	variableValues map[string]any,
	initialValue any,
) *ExecutionResult {
	operation := getOperation(document, operationName)
	if operation == nil {
		return &ExecutionResult{Errors: []GraphQLError{{Message: "operation not found"}}}
	}

	coercedVariableValues, err := coerceVariableValues(e.schema, operation, variableValues)
	if err != nil {
		return &ExecutionResult{Errors: []GraphQLError{{Message: err.Error()}}}
	}

	var rootType *schema.Type
	switch operation.Operation {
	case language.Query:
		rootType = e.schema.GetQueryType()
	case language.Mutation:
		rootType = e.schema.GetMutationType()
	case language.Subscription:
		rootType = e.schema.GetSubscriptionType()
	default:
		return &ExecutionResult{Errors: []GraphQLError{{Message: fmt.Sprintf("unsupported operation type: %s", operation.Operation)}}}
	}

	if rootType == nil {
		return &ExecutionResult{Errors: []GraphQLError{{Message: fmt.Sprintf("root type not found for %s operation", operation.Operation)}}}
	}

	state := &executionState{
		runtime:         e.runtime,
		schema:          e.schema,
		document:        document,
		variableValues:  coercedVariableValues,
		context:         ctx,
		asyncTaskGroup:  []asyncTask{},
		errors:          []GraphQLError{},
		asyncTaskInfo:   make(map[NodeID]asyncTask),
		nextID:          1,
		nullifiedPrefix: make(map[string]struct{}),
	}

	responseRoot := make(map[string]any)

	// Root selection set: sync immediate expansion, async queued
	rootResult := executeSelectionSet(state, rootType, operation.SelectionSet, initialValue, Path{})
	for k, v := range rootResult {
		responseRoot[k] = v
	}

	// Depth-wise batch loop
	for len(state.asyncTaskGroup) > 0 {
		filtered, results := flushAsyncTasks(state)
		for i, r := range results {
			completeAsyncField(state, filtered[i], r, responseRoot)
		}
	}

	return &ExecutionResult{Data: responseRoot, Errors: state.errors}
}

type Node struct {
	ObjectType   *schema.Type
	SelectionSet language.SelectionSet
	SourceValue  any
	ResponsePath Path
}

// executeSelectionSet executes a selection set without flushing
func executeSelectionSet(state *executionState, objectType *schema.Type, selectionSet language.SelectionSet, objectValue any, path Path) map[string]any {
	groupedFields := collectFields(state, objectType, selectionSet)
	resultMap := make(map[string]any)

	for _, collectedField := range groupedFields.orderedFields() {
		responseName := collectedField.ResponseName
		fields := collectedField.Fields
		fieldPath := appendPath(path, responseName)

		fieldResult := executeFieldGroup(state, objectType, objectValue, fields, fieldPath)

		// Handle __typename special case
		if fields[0].Name == "__typename" {
			resultMap[responseName] = fieldResult
			continue
		}

		fieldDef := getFieldDefinition(objectType, fields[0].Name)
		if fieldDef == nil {
			// Unknown field – error was already recorded in executeFieldGroup; do not include it
			continue
		}

		// Handle non-null child behavior with nullish detection
		if schema.IsNonNull(fieldDef.Type) && isNullish(fieldResult) {
			if len(path) > 0 {
				return nil
			}
			// Root level: keep going but write nil
			resultMap[responseName] = nil
			continue
		}

		// For nullable fields, coerce typed-nil to interface-nil
		if isNullish(fieldResult) {
			resultMap[responseName] = nil
		} else {
			resultMap[responseName] = fieldResult
		}
	}

	return resultMap
}

func executeFieldGroup(state *executionState, objectType *schema.Type, objectValue any, fields []*language.Field, path Path) any {
	field := fields[0]
	fieldName := field.Name

	// Handle __typename meta field
	if fieldName == "__typename" {
		return objectType.Name
	}

	fieldDef := getFieldDefinition(objectType, fieldName)
	if fieldDef == nil {
		state.errors = append(state.errors, GraphQLError{
			Message: fmt.Sprintf("Cannot query field '%s' on type '%s'", fieldName, objectType.Name),
			Path:    path,
		})
		return nil
	}

	argumentValues := coerceArgumentValues(fieldDef, field.Arguments, state.variableValues, state, path)

	async := fieldDef.Async
	if !async {
		resolvedValue := resolveSyncField(state, objectType.Name, fieldName, objectValue, argumentValues, path)
		completed := completeValue(state, fieldDef.Type, fields, resolvedValue, path)
		return completed
	} else {
		id := NodeID(state.nextID)
		state.nextID++
		at := asyncTask{
			ID: id,
			Task: AsyncResolveTask{
				ObjectType: objectType.Name,
				Field:      fieldName,
				Source:     objectValue,
				Args:       argumentValues,
			},
			ResponsePath: path,
			FieldType:    fieldDef.Type,
			Fields:       fields,
		}
		state.asyncTaskGroup = append(state.asyncTaskGroup, at)
		state.asyncTaskInfo[id] = at
		return asyncPending{}
	}
}

// flushAsyncTasks flushes tasks and returns results (filtered by tombstones)
func flushAsyncTasks(state *executionState) ([]asyncTask, []AsyncResolveResult) {
	// Filter out tasks under nullified prefixes
	filtered := make([]asyncTask, 0, len(state.asyncTaskGroup))
	for _, at := range state.asyncTaskGroup {
		if state.hasNullifiedPrefix(at.ResponsePath) {
			// Drop this task; also forget it for completion
			delete(state.asyncTaskInfo, at.ID)
			continue
		}
		filtered = append(filtered, at)
	}

	// Extract tasks
	tasks := make([]AsyncResolveTask, len(filtered))
	for i, at := range filtered {
		tasks[i] = at.Task
	}

	// Clear group before executing
	state.asyncTaskGroup = nil

	// Execute batch
	results := state.runtime.BatchResolveAsync(state.context, tasks)
	return filtered, results
}

// completeAsyncField completes a single async result, with non-null propagation and pruning
func completeAsyncField(state *executionState, at asyncTask, res AsyncResolveResult, responseRoot map[string]any) {
	delete(state.asyncTaskInfo, at.ID)

	path := at.ResponsePath
	// If this path is already nullified by an ancestor, ignore
	if state.hasNullifiedPrefix(path) {
		return
	}

	// Handle error case first
	if res.Error != nil {
		state.errors = append(state.errors, GraphQLError{Message: res.Error.Error(), Path: path})
		// If non-null field, propagate to top-level field
		if schema.IsNonNull(at.FieldType) {
			top := topLevelFieldPath(path)
			setValueAtPath(responseRoot, top, nil)
			state.markNullifiedPrefix(top)
			return
		}
		setValueAtPath(responseRoot, path, nil)
		return
	}

	completed := completeValue(state, at.FieldType, at.Fields, res.Value, path)

	// If non-null type but completion yielded nullish → propagate
	if schema.IsNonNull(at.FieldType) && isNullish(completed) {
		top := topLevelFieldPath(path)
		setValueAtPath(responseRoot, top, nil)
		state.markNullifiedPrefix(top)
		return
	}

	// Normal write; coerce typed-nil to interface nil
	if isNullish(completed) {
		setValueAtPath(responseRoot, path, nil)
	} else {
		setValueAtPath(responseRoot, path, completed)
	}
}

// completeValue completes a value
func completeValue(state *executionState, fieldType *schema.TypeRef, fields []*language.Field, result any, path Path) any {
	if schema.IsNonNull(fieldType) {
		if isNullish(result) {
			if !state.hasErrorAtPath(path) {
				state.errors = append(state.errors, GraphQLError{Message: fmt.Sprintf("Cannot return null for non-nullable field %s", pathToString(path)), Path: path})
			}
			return nil
		}
		inner := schema.Unwrap(fieldType)
		completed := completeValue(state, inner, fields, result, path)
		if isNullish(completed) {
			// Error already recorded at original path; propagate only
			return nil
		}
		return completed
	}

	if isNullish(result) {
		return nil
	}

	if schema.IsList(fieldType) {
		return completeListValue(state, fieldType, fields, result, path)
	}
	namedType := schema.GetNamedType(fieldType)
	typeObj := state.schema.Types[namedType]
	if typeObj == nil {
		state.errors = append(state.errors, GraphQLError{Message: fmt.Sprintf("Unknown type: %s", namedType), Path: path})
		return nil
	}

	switch typeObj.Kind {
	case schema.TypeKindScalar, schema.TypeKindEnum:
		serialized, err := state.runtime.SerializeLeafValue(state.context, namedType, result)
		if err != nil {
			state.errors = append(state.errors, GraphQLError{Message: err.Error(), Path: path})
			return nil
		}
		return serialized
	case schema.TypeKindObject:
		return completeObjectValue(state, typeObj, fields, result, path)
	case schema.TypeKindInterface, schema.TypeKindUnion:
		return completeAbstractValue(state, namedType, fields, result, path)
	default:
		state.errors = append(state.errors, GraphQLError{Message: fmt.Sprintf("Cannot complete value of unexpected type: %s", typeObj.Kind), Path: path})
		return nil
	}
}

// completeListValue completes a list value
func completeListValue(state *executionState, listType *schema.TypeRef, fields []*language.Field, result any, path Path) any {
	var items []any
	if direct, ok := result.([]any); ok {
		items = direct
	} else {
		rv := reflect.ValueOf(result)
		if rv.Kind() != reflect.Slice {
			state.errors = append(state.errors, GraphQLError{Message: fmt.Sprintf("Expected list value, got %T", result), Path: path})
			return nil
		}
		items = make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			items[i] = rv.Index(i).Interface()
		}
	}

	inner := schema.Unwrap(listType)
	completed := make([]any, len(items))
	for i, item := range items {
		p := appendPath(path, i)
		v := completeValue(state, inner, fields, item, p)
		if schema.IsNonNull(inner) && isNullish(v) {
			// Propagate null to the list field; error already recorded by inner completion
			return nil
		}
		completed[i] = v
	}
	return completed
}

func completeObjectValue(state *executionState, objectType *schema.Type, fields []*language.Field, result any, path Path) any {
	sub := mergeSelectionSets(fields)
	return executeSelectionSet(state, objectType, sub, result, path)
}

func completeAbstractValue(state *executionState, abstractTypeName string, fields []*language.Field, result any, path Path) any {
	typeName, err := state.runtime.ResolveType(state.context, abstractTypeName, result)
	if err != nil {
		state.addError(err.Error(), path)
		return nil
	}
	objectType := state.schema.Types[typeName]
	if objectType == nil || objectType.Kind != schema.TypeKindObject {
		state.addError(fmt.Sprintf("Abstract type %s must resolve to an Object type at runtime. Got: %s", abstractTypeName, typeName), path)
		return nil
	}
	return completeObjectValue(state, objectType, fields, result, path)
}

func pathToString(path Path) string {
	result := ""
	for i, elem := range path {
		if i > 0 {
			result += "."
		}
		switch v := elem.(type) {
		case string:
			result += v
		case int:
			result += fmt.Sprintf("[%d]", v)
		}
	}
	return result
}

func appendPath(path Path, elem PathElement) Path {
	newPath := make(Path, len(path)+1)
	copy(newPath, path)
	newPath[len(path)] = elem
	return newPath
}

// Prefix tombstone helpers
func (s *executionState) markNullifiedPrefix(p Path) {
	key := pathToString(p)
	if key != "" {
		s.nullifiedPrefix[key] = struct{}{}
	}
}

func (s *executionState) hasNullifiedPrefix(p Path) bool {
	if len(s.nullifiedPrefix) == 0 {
		return false
	}
	// Build prefixes progressively
	cur := Path{}
	for _, elem := range p {
		cur = append(cur, elem)
		key := pathToString(cur)
		if _, ok := s.nullifiedPrefix[key]; ok {
			return true
		}
	}
	return false
}

func topLevelFieldPath(p Path) Path {
	for _, elem := range p {
		if name, ok := elem.(string); ok {
			return Path{name}
		}
	}
	return Path{}
}

// getOperation retrieves the operation from the document
func getOperation(document *language.QueryDocument, operationName string) *language.OperationDefinition {
	if operationName == "" && len(document.Operations) == 1 {
		for _, op := range document.Operations {
			return op
		}
	}
	for _, op := range document.Operations {
		if op.Name == operationName {
			return op
		}
	}
	return nil
}

func typeRefFromAST(t *language.Type) *schema.TypeRef {
	if t == nil {
		return nil
	}
	if t.NonNull {
		return schema.NonNullType(typeRefFromAST(&language.Type{NamedType: t.NamedType, Elem: t.Elem}))
	}
	if t.NamedType != "" {
		return schema.NamedType(t.NamedType)
	}
	if t.Elem != nil {
		return schema.ListType(typeRefFromAST(t.Elem))
	}
	return nil
}

// Helper function to add an error to the execution state
func (state *executionState) addError(message string, path Path) {
	state.errors = append(state.errors, GraphQLError{Message: message, Path: path})
}

// hasErrorAtPath reports whether an error with the given path already exists.
func (state *executionState) hasErrorAtPath(path Path) bool {
	for _, err := range state.errors {
		if reflect.DeepEqual(err.Path, path) {
			return true
		}
	}
	return false
}

// resolveSyncField resolves a field synchronously
func resolveSyncField(state *executionState, objectType string, fieldName string, source any, args map[string]any, path Path) any {
	value, err := state.runtime.ResolveSync(state.context, objectType, fieldName, source, args)
	if err != nil {
		state.addError(err.Error(), path)
		return nil
	}
	return value
}

// Helper function to set value at a specific path in response tree
func setValueAtPath(responseRoot map[string]any, path Path, value any) {
	if len(path) == 0 {
		return
	}
	if len(path) == 1 {
		if key, ok := path[0].(string); ok {
			responseRoot[key] = value
			return
		}
	}
	current := any(responseRoot)
	for i, elem := range path[:len(path)-1] {
		switch e := elem.(type) {
		case string:
			m, ok := current.(map[string]any)
			if !ok {
				return
			}
			next, exists := m[e]
			if !exists {
				if i+1 < len(path)-1 {
					next = make(map[string]any)
				} else {
					next = make(map[string]any)
				}
				m[e] = next
			}
			current = next
		case int:
			slice, ok := current.([]any)
			if !ok {
				return
			}
			for len(slice) <= e {
				slice = append(slice, nil)
			}
			if slice[e] == nil {
				slice[e] = make(map[string]any)
			}
			current = slice[e]
		}
	}
	finalElem := path[len(path)-1]
	switch fe := finalElem.(type) {
	case string:
		if m, ok := current.(map[string]any); ok {
			m[fe] = value
		}
	case int:
		if slice, ok := current.([]any); ok {
			for len(slice) <= fe {
				slice = append(slice, nil)
			}
			slice[fe] = value
		}
	}
}

// mergeSelectionSets merges selection sets from multiple fields
func mergeSelectionSets(fields []*language.Field) language.SelectionSet {
	var merged language.SelectionSet
	for _, f := range fields {
		merged = append(merged, f.SelectionSet...)
	}
	return merged
}

// isNullish returns true for nil interfaces and typed nils (map, slice, ptr, interface)
func isNullish(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Interface, reflect.Ptr, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan:
		return rv.IsNil()
	default:
		return false
	}
}
