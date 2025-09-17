package executor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"

	language "github.com/hanpama/protograph/internal/language"
	schema "github.com/hanpama/protograph/internal/schema"
)

func TestCoerceVariableValues_InputObjectValidation(t *testing.T) {
	sch := schema.NewSchema("")

	input := schema.NewType("FilterInput", schema.TypeKindInputObject, "")
	input.AddInputField(schema.NewInputValue("required", "", schema.NonNullType(schema.NamedType("String"))))
	input.AddInputField(schema.NewInputValue("optional", "", schema.NamedType("Int")))
	sch.AddType(input)

	op := &language.OperationDefinition{
		Operation: language.Query,
		VariableDefinitions: ast.VariableDefinitionList{
			&ast.VariableDefinition{
				Variable: "input",
				Type:     &ast.Type{NamedType: "FilterInput", NonNull: true},
			},
		},
	}

	_, err := coerceVariableValues(sch, op, map[string]any{
		"input": map[string]any{
			"optional": 10,
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "required field 'required'")
}

func TestCoerceVariableValues_ScalarTypeMismatch(t *testing.T) {
	sch := schema.NewSchema("")

	op := &language.OperationDefinition{
		Operation: language.Query,
		VariableDefinitions: ast.VariableDefinitionList{
			&ast.VariableDefinition{
				Variable: "count",
				Type:     &ast.Type{NamedType: "Int", NonNull: true},
			},
		},
	}

	_, err := coerceVariableValues(sch, op, map[string]any{
		"count": "42",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot coerce")
}
