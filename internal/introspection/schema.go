package introspection

import (
	schema "github.com/hanpama/protograph/internal/schema"
)

// extendSchemaWithIntrospection creates a copy of the schema and adds introspection types and fields
func extendSchemaWithIntrospection(original *schema.Schema) *schema.Schema {
	// Create a shallow copy of the schema
	extended := &schema.Schema{
		QueryType:        original.QueryType,
		MutationType:     original.MutationType,
		SubscriptionType: original.SubscriptionType,
		Types:            make(map[string]*schema.Type),
		Directives:       original.Directives,
		Description:      original.Description,
	}
	
	// Copy all original types
	for name, typ := range original.Types {
		extended.Types[name] = typ
	}
	
	// Add introspection types
	addIntrospectionTypes(extended)
	
	// Add __schema and __type fields to Query type
	if queryType := extended.GetQueryType(); queryType != nil {
		// Create a copy of the Query type to add fields
		queryTypeCopy := &schema.Type{
			Name:        queryType.Name,
			Kind:        queryType.Kind,
			Description: queryType.Description,
			Fields:      make([]*schema.Field, len(queryType.Fields)),
			Interfaces:  queryType.Interfaces,
		}
		
		// Copy existing fields
		copy(queryTypeCopy.Fields, queryType.Fields)
		
		// Add introspection fields
		queryTypeCopy.Fields = append(queryTypeCopy.Fields,
			&schema.Field{
				Name:        "__schema",
				Description: "Access the current type schema of this server.",
				Type:        schema.NonNullType(schema.NamedType("__Schema")),
				Async:       false,
			},
			&schema.Field{
				Name:        "__type",
				Description: "Request the type information of a single type.",
				Arguments: []*schema.InputValue{
					{
						Name:        "name",
						Description: "The name of the type to look up.",
						Type:        schema.NonNullType(schema.NamedType("String")),
					},
				},
				Type:  schema.NamedType("__Type"),
				Async: false,
			},
		)
		
		extended.Types["Query"] = queryTypeCopy
	}
	
	return extended
}

// addIntrospectionTypes adds the introspection types to the schema
func addIntrospectionTypes(sch *schema.Schema) {
	sch.Types["__Schema"] = schemaType()
	sch.Types["__Type"] = typeType()
	sch.Types["__Field"] = fieldType()
	sch.Types["__InputValue"] = inputValueType()
	sch.Types["__EnumValue"] = enumValueType()
	sch.Types["__Directive"] = directiveType()
	sch.Types["__TypeKind"] = typeKindEnum()
	sch.Types["__DirectiveLocation"] = directiveLocationEnum()
}

// schemaType returns the __Schema introspection type definition
func schemaType() *schema.Type {
	return &schema.Type{
		Name:        "__Schema",
		Kind:        schema.TypeKindObject,
		Description: "A GraphQL Schema defines the capabilities of a GraphQL server.",
		Fields: []*schema.Field{
			{
				Name:        "types",
				Description: "A list of all types supported by this server.",
				Type:        schema.NonNullType(schema.ListType(schema.NonNullType(schema.NamedType("__Type")))),
			},
			{
				Name:        "queryType",
				Description: "The type that query operations will be rooted at.",
				Type:        schema.NonNullType(schema.NamedType("__Type")),
			},
			{
				Name:        "mutationType",
				Description: "If this server supports mutation, the type that mutation operations will be rooted at.",
				Type:        schema.NamedType("__Type"),
			},
			{
				Name:        "subscriptionType",
				Description: "If this server support subscription, the type that subscription operations will be rooted at.",
				Type:        schema.NamedType("__Type"),
			},
			{
				Name:        "directives",
				Description: "A list of all directives supported by this server.",
				Type:        schema.NonNullType(schema.ListType(schema.NonNullType(schema.NamedType("__Directive")))),
			},
			{
				Name:        "description",
				Description: "A description of the schema.",
				Type:        schema.NamedType("String"),
			},
		},
	}
}

// typeType returns the __Type introspection type definition
func typeType() *schema.Type {
	return &schema.Type{
		Name:        "__Type",
		Kind:        schema.TypeKindObject,
		Description: "The fundamental unit of any GraphQL Schema is the type.",
		Fields: []*schema.Field{
			{
				Name:        "kind",
				Description: "The kind of type.",
				Type:        schema.NonNullType(schema.NamedType("__TypeKind")),
			},
			{
				Name:        "name",
				Description: "The name of the type.",
				Type:        schema.NamedType("String"),
			},
			{
				Name:        "description",
				Description: "The description of the type.",
				Type:        schema.NamedType("String"),
			},
			{
				Name: "fields",
				Arguments: []*schema.InputValue{
					{
						Name:         "includeDeprecated",
						Type:         schema.NamedType("Boolean"),
						DefaultValue: false,
					},
				},
				Type: schema.ListType(schema.NonNullType(schema.NamedType("__Field"))),
			},
			{
				Name: "interfaces",
				Type: schema.ListType(schema.NonNullType(schema.NamedType("__Type"))),
			},
			{
				Name: "possibleTypes",
				Type: schema.ListType(schema.NonNullType(schema.NamedType("__Type"))),
			},
			{
				Name: "enumValues",
				Arguments: []*schema.InputValue{
					{
						Name:         "includeDeprecated",
						Type:         schema.NamedType("Boolean"),
						DefaultValue: false,
					},
				},
				Type: schema.ListType(schema.NonNullType(schema.NamedType("__EnumValue"))),
			},
			{
				Name: "inputFields",
				Arguments: []*schema.InputValue{
					{
						Name:         "includeDeprecated",
						Type:         schema.NamedType("Boolean"),
						DefaultValue: false,
					},
				},
				Type: schema.ListType(schema.NonNullType(schema.NamedType("__InputValue"))),
			},
			{
				Name: "ofType",
				Type: schema.NamedType("__Type"),
			},
			{
				Name: "specifiedByURL",
				Type: schema.NamedType("String"),
			},
			{
				Name: "isOneOf",
				Type: schema.NamedType("Boolean"),
			},
		},
	}
}

// fieldType returns the __Field introspection type definition
func fieldType() *schema.Type {
	return &schema.Type{
		Name: "__Field",
		Kind: schema.TypeKindObject,
		Fields: []*schema.Field{
			{Name: "name", Type: schema.NonNullType(schema.NamedType("String"))},
			{Name: "description", Type: schema.NamedType("String")},
			{
				Name: "args",
				Arguments: []*schema.InputValue{
					{Name: "includeDeprecated", Type: schema.NamedType("Boolean"), DefaultValue: false},
				},
				Type: schema.NonNullType(schema.ListType(schema.NonNullType(schema.NamedType("__InputValue")))),
			},
			{Name: "type", Type: schema.NonNullType(schema.NamedType("__Type"))},
			{Name: "isDeprecated", Type: schema.NonNullType(schema.NamedType("Boolean"))},
			{Name: "deprecationReason", Type: schema.NamedType("String")},
		},
	}
}

// inputValueType returns the __InputValue introspection type definition
func inputValueType() *schema.Type {
	return &schema.Type{
		Name: "__InputValue",
		Kind: schema.TypeKindObject,
		Fields: []*schema.Field{
			{Name: "name", Type: schema.NonNullType(schema.NamedType("String"))},
			{Name: "description", Type: schema.NamedType("String")},
			{Name: "type", Type: schema.NonNullType(schema.NamedType("__Type"))},
			{Name: "defaultValue", Type: schema.NamedType("String")},
			{Name: "isDeprecated", Type: schema.NonNullType(schema.NamedType("Boolean"))},
			{Name: "deprecationReason", Type: schema.NamedType("String")},
		},
	}
}

// enumValueType returns the __EnumValue introspection type definition
func enumValueType() *schema.Type {
	return &schema.Type{
		Name: "__EnumValue",
		Kind: schema.TypeKindObject,
		Fields: []*schema.Field{
			{Name: "name", Type: schema.NonNullType(schema.NamedType("String"))},
			{Name: "description", Type: schema.NamedType("String")},
			{Name: "isDeprecated", Type: schema.NonNullType(schema.NamedType("Boolean"))},
			{Name: "deprecationReason", Type: schema.NamedType("String")},
		},
	}
}

// directiveType returns the __Directive introspection type definition
func directiveType() *schema.Type {
	return &schema.Type{
		Name: "__Directive",
		Kind: schema.TypeKindObject,
		Fields: []*schema.Field{
			{Name: "name", Type: schema.NonNullType(schema.NamedType("String"))},
			{Name: "description", Type: schema.NamedType("String")},
			{Name: "isRepeatable", Type: schema.NonNullType(schema.NamedType("Boolean"))},
			{Name: "locations", Type: schema.NonNullType(schema.ListType(schema.NonNullType(schema.NamedType("__DirectiveLocation"))))},
			{
				Name: "args",
				Arguments: []*schema.InputValue{
					{Name: "includeDeprecated", Type: schema.NamedType("Boolean"), DefaultValue: false},
				},
				Type: schema.NonNullType(schema.ListType(schema.NonNullType(schema.NamedType("__InputValue")))),
			},
		},
	}
}

// typeKindEnum returns the __TypeKind enum type definition
func typeKindEnum() *schema.Type {
	return &schema.Type{
		Name: "__TypeKind",
		Kind: schema.TypeKindEnum,
		EnumValues: []*schema.EnumValue{
			{Name: "SCALAR"},
			{Name: "OBJECT"},
			{Name: "INTERFACE"},
			{Name: "UNION"},
			{Name: "ENUM"},
			{Name: "INPUT_OBJECT"},
			{Name: "LIST"},
			{Name: "NON_NULL"},
		},
	}
}

// directiveLocationEnum returns the __DirectiveLocation enum type definition
func directiveLocationEnum() *schema.Type {
	return &schema.Type{
		Name: "__DirectiveLocation",
		Kind: schema.TypeKindEnum,
		EnumValues: []*schema.EnumValue{
			{Name: "QUERY"},
			{Name: "MUTATION"},
			{Name: "SUBSCRIPTION"},
			{Name: "FIELD"},
			{Name: "FRAGMENT_DEFINITION"},
			{Name: "FRAGMENT_SPREAD"},
			{Name: "INLINE_FRAGMENT"},
			{Name: "VARIABLE_DEFINITION"},
			{Name: "SCHEMA"},
			{Name: "SCALAR"},
			{Name: "OBJECT"},
			{Name: "FIELD_DEFINITION"},
			{Name: "ARGUMENT_DEFINITION"},
			{Name: "INTERFACE"},
			{Name: "UNION"},
			{Name: "ENUM"},
			{Name: "ENUM_VALUE"},
			{Name: "INPUT_OBJECT"},
			{Name: "INPUT_FIELD_DEFINITION"},
		},
	}
}