package introspection

import (
	schema "github.com/hanpama/protograph/internal/schema"
)

// extendSchemaWithIntrospection creates a copy of the schema and adds introspection types and fields
func extendSchemaWithIntrospection(original *schema.Schema) *schema.Schema {
	extended := schema.NewSchema(original.Description).
		SetQueryType(original.QueryType).
		SetMutationType(original.MutationType).
		SetSubscriptionType(original.SubscriptionType)

	// Share existing directives snapshot (immutable in practice)
	extended.Directives = original.Directives

	for _, typ := range original.Types {
		extended.AddType(typ)
	}

	addIntrospectionTypes(extended)

	if queryType := extended.GetQueryType(); queryType != nil {
		queryTypeCopy := schema.NewType(queryType.Name, queryType.Kind, queryType.Description)
		for _, iface := range queryType.Interfaces {
			queryTypeCopy.AddInterface(iface)
		}
		for _, field := range queryType.GetOrderedFields() {
			queryTypeCopy.AddField(cloneField(field))
		}

		queryTypeCopy.AddField(schema.NewField(
			"__schema",
			"Access the current type schema of this server.",
			schema.NonNullType(schema.NamedType("__Schema")),
		))

		typeField := schema.NewField(
			"__type",
			"Request the type information of a single type.",
			schema.NamedType("__Type"),
		)
		typeField.AddArgument(
			schema.NewInputValue(
				"name",
				"The name of the type to look up.",
				schema.NonNullType(schema.NamedType("String")),
			),
		)
		queryTypeCopy.AddField(typeField)

		extended.AddType(queryTypeCopy)
	}

	return extended
}

// addIntrospectionTypes adds the introspection types to the schema
func addIntrospectionTypes(sch *schema.Schema) {
	sch.AddType(schemaType()).
		AddType(typeType()).
		AddType(fieldType()).
		AddType(inputValueType()).
		AddType(enumValueType()).
		AddType(directiveType()).
		AddType(typeKindEnum()).
		AddType(directiveLocationEnum())
}

// schemaType returns the __Schema introspection type definition
func schemaType() *schema.Type {
	t := schema.NewType(
		"__Schema",
		schema.TypeKindObject,
		"A GraphQL Schema defines the capabilities of a GraphQL server. It exposes all available types and directives on the server, as well as the entry points for query, mutation, and subscription operations.",
	)
	t.AddField(schema.NewField(
		"types",
		"A list of all types supported by this server.",
		schema.NonNullType(schema.ListType(schema.NonNullType(schema.NamedType("__Type")))),
	))
	t.AddField(schema.NewField(
		"queryType",
		"The type that query operations will be rooted at.",
		schema.NonNullType(schema.NamedType("__Type")),
	))
	t.AddField(schema.NewField(
		"mutationType",
		"If this server supports mutation, the type that mutation operations will be rooted at.",
		schema.NamedType("__Type"),
	))
	t.AddField(schema.NewField(
		"subscriptionType",
		"If this server supports subscription, the type that subscription operations will be rooted at.",
		schema.NamedType("__Type"),
	))
	t.AddField(schema.NewField(
		"directives",
		"A list of all directives supported by this server.",
		schema.NonNullType(schema.ListType(schema.NonNullType(schema.NamedType("__Directive")))),
	))
	t.AddField(schema.NewField(
		"description",
		"A description of this schema.",
		schema.NamedType("String"),
	))
	return t
}

// typeType returns the __Type introspection type definition
func typeType() *schema.Type {
	t := schema.NewType(
		"__Type",
		schema.TypeKindObject,
		"The fundamental unit of any GraphQL Schema is the type. There are many kinds of types in GraphQL as represented by the `__TypeKind` enum.\n\nDepending on the kind of a type, certain fields describe information about that type. Scalar types provide no information beyond a name, description and optional `specifiedByURL`, while Enum types provide their values. Object and Interface types provide the fields they describe. Abstract types, Union and Interface, provide the Object types possible at runtime. List and NonNull types compose other types.",
	)
	t.AddField(schema.NewField(
		"kind",
		"The kind of type.",
		schema.NonNullType(schema.NamedType("__TypeKind")),
	))
	t.AddField(schema.NewField(
		"name",
		"The name of the type.",
		schema.NamedType("String"),
	))
	t.AddField(schema.NewField(
		"description",
		"The description of the type.",
		schema.NamedType("String"),
	))
	fieldsField := schema.NewField(
		"fields",
		"Fields on this type, including deprecated fields if `includeDeprecated` is true.",
		schema.ListType(schema.NonNullType(schema.NamedType("__Field"))),
	)
	fieldsField.AddArgument(schema.NewInputValue(
		"includeDeprecated",
		"Include deprecated fields in the results.",
		schema.NamedType("Boolean"),
	).SetDefault(false))
	t.AddField(fieldsField)
	t.AddField(schema.NewField(
		"interfaces",
		"Interfaces implemented by this type.",
		schema.ListType(schema.NonNullType(schema.NamedType("__Type"))),
	))
	t.AddField(schema.NewField(
		"possibleTypes",
		"Possible concrete types for this abstract type.",
		schema.ListType(schema.NonNullType(schema.NamedType("__Type"))),
	))
	enumValuesField := schema.NewField(
		"enumValues",
		"Values for this enum type, including deprecated values if `includeDeprecated` is true.",
		schema.ListType(schema.NonNullType(schema.NamedType("__EnumValue"))),
	)
	enumValuesField.AddArgument(schema.NewInputValue(
		"includeDeprecated",
		"Include deprecated enum values in the results.",
		schema.NamedType("Boolean"),
	).SetDefault(false))
	t.AddField(enumValuesField)
	inputFieldsField := schema.NewField(
		"inputFields",
		"Input fields for this input object, including deprecated input fields if `includeDeprecated` is true.",
		schema.ListType(schema.NonNullType(schema.NamedType("__InputValue"))),
	)
	inputFieldsField.AddArgument(schema.NewInputValue(
		"includeDeprecated",
		"Include deprecated input fields in the results.",
		schema.NamedType("Boolean"),
	).SetDefault(false))
	t.AddField(inputFieldsField)
	t.AddField(schema.NewField(
		"ofType",
		"Wrapped type if this is a list or non-null.",
		schema.NamedType("__Type"),
	))
	t.AddField(schema.NewField(
		"specifiedByURL",
		"URL that specifies the behaviour of this scalar.",
		schema.NamedType("String"),
	))
	t.AddField(schema.NewField(
		"isOneOf",
		"Whether this input object represents a @oneOf specification.",
		schema.NamedType("Boolean"),
	))
	return t
}

// fieldType returns the __Field introspection type definition
func fieldType() *schema.Type {
	t := schema.NewType(
		"__Field",
		schema.TypeKindObject,
		"Object and Interface types are described by a list of Fields, each of which has a name, potentially a list of arguments, and a return type.",
	)
	t.AddField(schema.NewField(
		"name",
		"The name of the field.",
		schema.NonNullType(schema.NamedType("String")),
	))
	t.AddField(schema.NewField(
		"description",
		"The description of the field.",
		schema.NamedType("String"),
	))
	argsField := schema.NewField(
		"args",
		"Arguments accepted by this field.",
		schema.NonNullType(schema.ListType(schema.NonNullType(schema.NamedType("__InputValue")))),
	)
	argsField.AddArgument(schema.NewInputValue(
		"includeDeprecated",
		"Include deprecated arguments in the results.",
		schema.NamedType("Boolean"),
	).SetDefault(false))
	t.AddField(argsField)
	t.AddField(schema.NewField(
		"type",
		"The return type of the field.",
		schema.NonNullType(schema.NamedType("__Type")),
	))
	t.AddField(schema.NewField(
		"isDeprecated",
		"Whether this field is deprecated.",
		schema.NonNullType(schema.NamedType("Boolean")),
	))
	t.AddField(schema.NewField(
		"deprecationReason",
		"Reason the field is deprecated, if any.",
		schema.NamedType("String"),
	))
	return t
}

// inputValueType returns the __InputValue introspection type definition
func inputValueType() *schema.Type {
	t := schema.NewType(
		"__InputValue",
		schema.TypeKindObject,
		"Arguments provided to Fields or Directives and the input fields of an InputObject are represented as Input Values which describe their type and optionally a default value.",
	)
	t.AddField(schema.NewField(
		"name",
		"The name of the input value.",
		schema.NonNullType(schema.NamedType("String")),
	))
	t.AddField(schema.NewField(
		"description",
		"The description of the input value.",
		schema.NamedType("String"),
	))
	t.AddField(schema.NewField(
		"type",
		"The expected type of the input value.",
		schema.NonNullType(schema.NamedType("__Type")),
	))
	t.AddField(schema.NewField(
		"defaultValue",
		"A GraphQL-formatted string representing the default value for this input value.",
		schema.NamedType("String"),
	))
	t.AddField(schema.NewField(
		"isDeprecated",
		"Whether this input value is deprecated.",
		schema.NonNullType(schema.NamedType("Boolean")),
	))
	t.AddField(schema.NewField(
		"deprecationReason",
		"Reason the input value is deprecated, if any.",
		schema.NamedType("String"),
	))
	return t
}

// enumValueType returns the __EnumValue introspection type definition
func enumValueType() *schema.Type {
	t := schema.NewType(
		"__EnumValue",
		schema.TypeKindObject,
		"One possible value for a given Enum. Enum values are unique values, not a placeholder for a string or numeric value. However an Enum value is returned in a JSON response as a string.",
	)
	t.AddField(schema.NewField(
		"name",
		"The name of the enum value.",
		schema.NonNullType(schema.NamedType("String")),
	))
	t.AddField(schema.NewField(
		"description",
		"The description of the enum value.",
		schema.NamedType("String"),
	))
	t.AddField(schema.NewField(
		"isDeprecated",
		"Whether this enum value is deprecated.",
		schema.NonNullType(schema.NamedType("Boolean")),
	))
	t.AddField(schema.NewField(
		"deprecationReason",
		"Reason the enum value is deprecated, if any.",
		schema.NamedType("String"),
	))
	return t
}

// directiveType returns the __Directive introspection type definition
func directiveType() *schema.Type {
	t := schema.NewType(
		"__Directive",
		schema.TypeKindObject,
		"A Directive provides a way to describe alternate runtime execution and type validation behavior in a GraphQL document.\n\nIn some cases, you need to provide options to alter GraphQL's execution behavior in ways field arguments will not suffice, such as conditionally including or skipping a field. Directives provide this by describing additional information to the executor.",
	)
	t.AddField(schema.NewField(
		"name",
		"The name of the directive.",
		schema.NonNullType(schema.NamedType("String")),
	))
	t.AddField(schema.NewField(
		"description",
		"The description of the directive.",
		schema.NamedType("String"),
	))
	t.AddField(schema.NewField(
		"isRepeatable",
		"Whether this directive may be used more than once at a single location.",
		schema.NonNullType(schema.NamedType("Boolean")),
	))
	t.AddField(schema.NewField(
		"locations",
		"Locations this directive may be used in.",
		schema.NonNullType(schema.ListType(schema.NonNullType(schema.NamedType("__DirectiveLocation")))),
	))
	argsField := schema.NewField(
		"args",
		"Arguments accepted by this directive.",
		schema.NonNullType(schema.ListType(schema.NonNullType(schema.NamedType("__InputValue")))),
	)
	argsField.AddArgument(schema.NewInputValue(
		"includeDeprecated",
		"Include deprecated directive arguments in the results.",
		schema.NamedType("Boolean"),
	).SetDefault(false))
	t.AddField(argsField)
	return t
}

// typeKindEnum returns the __TypeKind enum type definition
func typeKindEnum() *schema.Type {
	t := schema.NewType(
		"__TypeKind",
		schema.TypeKindEnum,
		"An enum describing what kind of type a given `__Type` is.",
	)
	t.AddEnumValue(schema.NewEnumValue("SCALAR", "Indicates this type is a scalar."))
	t.AddEnumValue(schema.NewEnumValue("OBJECT", "Indicates this type is an object. `fields` and `interfaces` are valid fields."))
	t.AddEnumValue(schema.NewEnumValue("INTERFACE", "Indicates this type is an interface. `fields`, `interfaces`, and `possibleTypes` are valid fields."))
	t.AddEnumValue(schema.NewEnumValue("UNION", "Indicates this type is a union. `possibleTypes` is a valid field."))
	t.AddEnumValue(schema.NewEnumValue("ENUM", "Indicates this type is an enum. `enumValues` is a valid field."))
	t.AddEnumValue(schema.NewEnumValue("INPUT_OBJECT", "Indicates this type is an input object. `inputFields` is a valid field."))
	t.AddEnumValue(schema.NewEnumValue("LIST", "Indicates this type is a list. `ofType` is a valid field."))
	t.AddEnumValue(schema.NewEnumValue("NON_NULL", "Indicates this type is a non-null. `ofType` is a valid field."))
	return t
}

// directiveLocationEnum returns the __DirectiveLocation enum type definition
func directiveLocationEnum() *schema.Type {
	t := schema.NewType(
		"__DirectiveLocation",
		schema.TypeKindEnum,
		"A Directive can be adjacent to many parts of the GraphQL language, a __DirectiveLocation describes one such possible adjacency.",
	)
	locations := []struct {
		Name, Description string
	}{
		{"QUERY", "Location adjacent to a query operation."},
		{"MUTATION", "Location adjacent to a mutation operation."},
		{"SUBSCRIPTION", "Location adjacent to a subscription operation."},
		{"FIELD", "Location adjacent to a field."},
		{"FRAGMENT_DEFINITION", "Location adjacent to a fragment definition."},
		{"FRAGMENT_SPREAD", "Location adjacent to a fragment spread."},
		{"INLINE_FRAGMENT", "Location adjacent to an inline fragment."},
		{"VARIABLE_DEFINITION", "Location adjacent to a variable definition."},
		{"SCHEMA", "Location adjacent to a schema definition."},
		{"SCALAR", "Location adjacent to a scalar definition."},
		{"OBJECT", "Location adjacent to an object type definition."},
		{"FIELD_DEFINITION", "Location adjacent to a field definition."},
		{"ARGUMENT_DEFINITION", "Location adjacent to an argument definition."},
		{"INTERFACE", "Location adjacent to an interface definition."},
		{"UNION", "Location adjacent to a union definition."},
		{"ENUM", "Location adjacent to an enum definition."},
		{"ENUM_VALUE", "Location adjacent to an enum value definition."},
		{"INPUT_OBJECT", "Location adjacent to an input object type definition."},
		{"INPUT_FIELD_DEFINITION", "Location adjacent to an input object field definition."},
	}
	for _, loc := range locations {
		t.AddEnumValue(schema.NewEnumValue(loc.Name, loc.Description))
	}
	return t
}

func cloneField(src *schema.Field) *schema.Field {
	if src == nil {
		return nil
	}
	cloned := schema.NewField(src.Name, src.Description, src.Type).
		SetAsync(src.Async)
	if src.IsDeprecated {
		cloned.Deprecate(src.DeprecationReason)
	}
	for _, arg := range src.GetOrderedArguments() {
		cloned.AddArgument(cloneInputValue(arg))
	}
	cloned.Index = src.Index
	return cloned
}

func cloneInputValue(src *schema.InputValue) *schema.InputValue {
	cloned := schema.NewInputValue(src.Name, src.Description, src.Type).
		SetDefault(src.DefaultValue)
	if src.IsDeprecated {
		cloned.Deprecate(src.DeprecationReason)
	}
	cloned.Index = src.Index
	return cloned
}
