package language

import "github.com/vektah/gqlparser/v2/ast"

type (
	QueryDocument       = ast.QueryDocument
	SchemaDocument      = ast.SchemaDocument
	OperationDefinition = ast.OperationDefinition
	SelectionSet        = ast.SelectionSet
	Selection           = ast.Selection
	Field               = ast.Field
	InlineFragment      = ast.InlineFragment
	FragmentDefinition  = ast.FragmentDefinition
	FragmentSpread      = ast.FragmentSpread
	Directive           = ast.Directive
	DirectiveList       = ast.DirectiveList
	ArgumentList        = ast.ArgumentList
	Argument            = ast.Argument
	Value               = ast.Value
	FieldDefinition     = ast.FieldDefinition
	ArgumentDefinition  = ast.ArgumentDefinition
	EnumValueDefinition = ast.EnumValueDefinition
	Type                = ast.Type
	Definition          = ast.Definition
	DefinitionList      = ast.DefinitionList
	Position            = ast.Position
)

type DefinitionKind = ast.DefinitionKind

type Operation = ast.Operation

type ValueKind = ast.ValueKind

const (
	Query        Operation = ast.Query
	Mutation     Operation = ast.Mutation
	Subscription Operation = ast.Subscription

	Object      DefinitionKind = ast.Object
	Interface   DefinitionKind = ast.Interface
	Union       DefinitionKind = ast.Union
	Scalar      DefinitionKind = ast.Scalar
	Enum        DefinitionKind = ast.Enum
	InputObject DefinitionKind = ast.InputObject

	Variable     ValueKind = ast.Variable
	IntValue     ValueKind = ast.IntValue
	FloatValue   ValueKind = ast.FloatValue
	StringValue  ValueKind = ast.StringValue
	BlockValue   ValueKind = ast.BlockValue
	BooleanValue ValueKind = ast.BooleanValue
	NullValue    ValueKind = ast.NullValue
	EnumValue    ValueKind = ast.EnumValue
	ListValue    ValueKind = ast.ListValue
	ObjectValue  ValueKind = ast.ObjectValue
)
