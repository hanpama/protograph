package executor

import (
	"fmt"
	"strconv"
	"strings"

	language "github.com/hanpama/protograph/internal/language"
	schema "github.com/hanpama/protograph/internal/schema"
)

// coerceVariableValues coerces variable values according to their types
func coerceVariableValues(
	schema *schema.Schema,
	operation *language.OperationDefinition,
	variableValues map[string]any,
) (map[string]any, error) {
	if variableValues == nil {
		variableValues = make(map[string]any)
	}
	coerced := make(map[string]any)
	for _, varDef := range operation.VariableDefinitions {
		name := varDef.Variable
		t := varDef.Type
		val, ok := variableValues[name]
		if !ok {
			if v2, ok2 := variableValues[strings.TrimPrefix(name, "$")]; ok2 {
				val = v2
				ok = true
			}
		}
		if !ok {
			if varDef.DefaultValue != nil {
				val = astValueToGo(varDef.DefaultValue)
			} else if t.NonNull {
				return nil, fmt.Errorf("variable $%s of required type %s was not provided", name, t.String())
			} else {
				continue
			}
		}
		if val == nil && t.NonNull {
			return nil, fmt.Errorf("variable $%s of type %s cannot be null", name, t.String())
		}
		cv, err := coerceValue(val, typeRefFromAST(t))
		if err != nil {
			return nil, fmt.Errorf("variable $%s of type %s cannot be coerced: %v", name, t.String(), err)
		}
		coerced[name] = cv
	}
	return coerced, nil
}

// coerceArgumentValues coerces argument values for a field
func coerceArgumentValues(
	fieldDef *schema.Field,
	arguments language.ArgumentList,
	variableValues map[string]any,
	state *executionState,
	path Path,
) map[string]any {
	coerced := make(map[string]any)
	for _, arg := range arguments {
		var argDef *schema.InputValue
		for _, a := range fieldDef.Arguments {
			if a.Name == arg.Name {
				argDef = a
				break
			}
		}
		if argDef == nil {
			continue
		}
		val := valueFromASTWithVars(arg.Value, variableValues)
		cv, err := coerceValue(val, argDef.Type)
		if err != nil {
			state.addError(fmt.Sprintf("argument '%s' cannot be coerced: %v", arg.Name, err), path)
			continue
		}
		coerced[arg.Name] = cv
	}
	for _, argDef := range fieldDef.Arguments {
		name := argDef.Name
		if _, ok := coerced[name]; !ok {
			if argDef.DefaultValue != nil {
				coerced[name] = argDef.DefaultValue
			} else if schema.IsNonNull(argDef.Type) {
				state.addError(fmt.Sprintf("argument '%s' of required type was not provided", name), path)
			}
		}
	}
	return coerced
}

// valueFromASTWithVars converts an AST value to a runtime value with variable substitution
func valueFromASTWithVars(value *language.Value, variableValues map[string]any) any {
	if value == nil {
		return nil
	}
	switch value.Kind {
	case language.Variable:
		name := value.Raw
		if v, ok := variableValues[name]; ok {
			return v
		}
		if v, ok := variableValues[strings.TrimPrefix(name, "$")]; ok {
			return v
		}
		return nil
	default:
		return astValueToGo(value)
	}
}

// astValueToGo converts an AST value to a Go value
func astValueToGo(value *language.Value) any {
	if value == nil {
		return nil
	}
	switch value.Kind {
	case language.IntValue:
		iv, _ := strconv.Atoi(value.Raw)
		return iv
	case language.FloatValue:
		fv, _ := strconv.ParseFloat(value.Raw, 64)
		return fv
	case language.StringValue, language.BlockValue:
		return value.Raw
	case language.BooleanValue:
		return value.Raw == "true"
	case language.NullValue:
		return nil
	case language.EnumValue:
		return value.Raw
	case language.ListValue:
		out := make([]any, len(value.Children))
		for i, c := range value.Children {
			out[i] = astValueToGo(c.Value)
		}
		return out
	case language.ObjectValue:
		m := make(map[string]any)
		for _, f := range value.Children {
			m[f.Name] = astValueToGo(f.Value)
		}
		return m
	default:
		return nil
	}
}

// coerceValue coerces a value to the specified GraphQL type
func coerceValue(value any, targetType *schema.TypeRef) (any, error) {
	// Handle Non-Null wrapper
	if schema.IsNonNull(targetType) {
		if value == nil {
			return nil, fmt.Errorf("cannot provide null for non-null type")
		}
		return coerceValue(value, schema.Unwrap(targetType))
	}

	// Handle null for nullable types
	if value == nil {
		return nil, nil
	}

	// Handle List wrapper
	if schema.IsList(targetType) {
		return coerceListValue(value, targetType)
	}

	// Get the named type for scalar coercion
	namedType := schema.GetNamedType(targetType)

	// Coerce based on target scalar type
	switch namedType {
	case "Int":
		return coerceToInt(value)
	case "Float":
		return coerceToFloat(value)
	case "String":
		return coerceToString(value)
	case "Boolean":
		return coerceToBoolean(value)
	case "ID":
		return coerceToID(value)
	default:
		// For custom scalars and other types, return as-is
		return value, nil
	}
}

// coerceListValue coerces a value to a list
func coerceListValue(value any, listType *schema.TypeRef) (any, error) {
	// If already a slice, coerce each item
	if slice, ok := value.([]any); ok {
		innerType := schema.Unwrap(listType)
		coercedSlice := make([]any, len(slice))
		for i, item := range slice {
			coercedItem, err := coerceValue(item, innerType)
			if err != nil {
				return nil, err
			}
			coercedSlice[i] = coercedItem
		}
		return coercedSlice, nil
	}

	// Single value becomes a list of one
	innerType := schema.Unwrap(listType)
	coercedItem, err := coerceValue(value, innerType)
	if err != nil {
		return nil, err
	}
	return []any{coercedItem}, nil
}

// Basic scalar coercion functions - improved
func coerceToInt(value any) (any, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case float32:
		return int(v), nil
	case string:
		if intVal, err := strconv.Atoi(v); err == nil {
			return intVal, nil
		}
	}
	return nil, fmt.Errorf("cannot coerce %v (%T) to int", value, value)
}

func coerceToFloat(value any) (any, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		if floatVal, err := strconv.ParseFloat(v, 64); err == nil {
			return floatVal, nil
		}
	}
	return nil, fmt.Errorf("cannot coerce %v (%T) to float", value, value)
}

func coerceToString(value any) (any, error) {
	if v, ok := value.(string); ok {
		return v, nil
	}
	return fmt.Sprintf("%v", value), nil
}

func coerceToBoolean(value any) (any, error) {
	if v, ok := value.(bool); ok {
		return v, nil
	}
	return nil, fmt.Errorf("cannot coerce %v (%T) to boolean", value, value)
}

func coerceToID(value any) (any, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case int:
		return strconv.Itoa(v), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	default:
		return fmt.Sprintf("%v", value), nil
	}
}
