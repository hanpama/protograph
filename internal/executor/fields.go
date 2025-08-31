package executor

import (
	"fmt"

	language "github.com/hanpama/protograph/internal/language"
	schema "github.com/hanpama/protograph/internal/schema"
)

// collectedFieldMap preserves field order from the original query
type collectedFieldMap struct {
	fields []collectedField
	index  map[string]int
}

type collectedField struct {
	ResponseName string
	Fields       []*language.Field
}

func newCollectedFieldMap() *collectedFieldMap {
	return &collectedFieldMap{
		fields: make([]collectedField, 0),
		index:  make(map[string]int),
	}
}

func (cfm *collectedFieldMap) add(responseName string, field *language.Field) {
	if idx, exists := cfm.index[responseName]; exists {
		// Append to existing field group
		cfm.fields[idx].Fields = append(cfm.fields[idx].Fields, field)
	} else {
		// Create new field group
		cfm.index[responseName] = len(cfm.fields)
		cfm.fields = append(cfm.fields, collectedField{
			ResponseName: responseName,
			Fields:       []*language.Field{field},
		})
	}
}

func (cfm *collectedFieldMap) orderedFields() []collectedField {
	return cfm.fields
}

// collectFields collects fields from a selection set
func collectFields(state *executionState, objectType *schema.Type, selectionSet language.SelectionSet) *collectedFieldMap {
	groupedFields := newCollectedFieldMap()
	visitedFragments := make(map[string]bool)

	collectFieldsImpl(state, objectType, selectionSet, groupedFields, visitedFragments)

	return groupedFields
}

// collectFieldsImpl is the recursive implementation of field collection
func collectFieldsImpl(state *executionState, objectType *schema.Type, selectionSet language.SelectionSet, groupedFields *collectedFieldMap, visitedFragments map[string]bool) {
	for _, selection := range selectionSet {
		switch sel := selection.(type) {
		case *language.Field:
			// Check directives (@skip, @include)
			if !shouldIncludeNode(state, sel.Directives) {
				continue
			}

			responseName := sel.Alias
			if responseName == "" {
				responseName = sel.Name
			}

			groupedFields.add(responseName, sel)

		case *language.InlineFragment:
			// Check directives
			if !shouldIncludeNode(state, sel.Directives) {
				continue
			}

			// Check type condition
			if sel.TypeCondition != "" && sel.TypeCondition != objectType.Name {
				// TODO: Check if objectType implements the interface/union
				continue
			}

			// Collect fields from inline fragment
			collectFieldsImpl(state, objectType, sel.SelectionSet, groupedFields, visitedFragments)

		case *language.FragmentSpread:
			// Check directives
			if !shouldIncludeNode(state, sel.Directives) {
				continue
			}

			// Check if already visited
			if visitedFragments[sel.Name] {
				continue
			}
			visitedFragments[sel.Name] = true

			// Get fragment definition from document
			fragmentDef := getFragmentDefinition(state.document, sel.Name)
			if fragmentDef == nil {
				continue
			}

			// Check type condition
			if fragmentDef.TypeCondition != "" && fragmentDef.TypeCondition != objectType.Name {
				// TODO: Check if objectType implements the interface/union
				continue
			}

			// Check fragment definition directives (@skip, @include)
			if !shouldIncludeNode(state, fragmentDef.Directives) {
				continue
			}

			// Recursively collect fields from fragment
			collectFieldsImpl(state, objectType, fragmentDef.SelectionSet, groupedFields, visitedFragments)
		}
	}
}

// shouldIncludeNode checks if a node should be included based on directives
func shouldIncludeNode(state *executionState, directives language.DirectiveList) bool {
	// Check @skip directive
	if skip := directives.ForName("skip"); skip != nil {
		if skipIf, err := getDirectiveArgumentValue(state, skip, "if"); err == nil {
			if skipBool, ok := skipIf.(bool); ok && skipBool {
				return false
			}
		}
	}

	// Check @include directive
	if include := directives.ForName("include"); include != nil {
		if includeIf, err := getDirectiveArgumentValue(state, include, "if"); err == nil {
			if includeBool, ok := includeIf.(bool); ok && !includeBool {
				return false
			}
		}
	}

	return true
}

// getDirectiveArgumentValue gets the value of a directive argument
func getDirectiveArgumentValue(state *executionState, directive *language.Directive, argName string) (any, error) {
	for _, arg := range directive.Arguments {
		if arg.Name == argName {
			return valueFromAST(state, arg.Value), nil
		}
	}
	return nil, fmt.Errorf("argument %s not found", argName)
}

// valueFromAST converts an AST value to a runtime value
func valueFromAST(state *executionState, value *language.Value) any {
	if value == nil {
		return nil
	}

	switch value.Kind {
	case language.Variable:
		varName := value.Raw
		if val, ok := state.variableValues[varName]; ok {
			return val
		}
		return nil
	default:
		return astValueToGo(value)
	}
}

// getFragmentDefinition finds a fragment definition by name in the document
func getFragmentDefinition(document *language.QueryDocument, name string) *language.FragmentDefinition {
	if fd := document.Fragments.ForName(name); fd != nil {
		return fd
	}
	// Fallback: iterate
	for _, f := range document.Fragments {
		if f != nil && f.Name == name {
			return f
		}
	}
	return nil
}

func getFieldDefinition(objectType *schema.Type, fieldName string) *schema.Field {
	for _, field := range objectType.Fields {
		if field.Name == fieldName {
			return field
		}
	}
	return nil
}
