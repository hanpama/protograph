package ir

var StringType = &ScalarDefinition{
	Name:              "String",
	Description:       "The String scalar type represents textual data, represented as UTF-8 character sequences.",
	MappedToProtoType: "string",
	SpecifiedByURL:    "https://spec.graphql.org/October2021/#sec-String",
}

var IntType = &ScalarDefinition{
	Name:              "Int",
	Description:       "The Int scalar type represents non-fractional signed whole numeric values.",
	MappedToProtoType: "int32",
	SpecifiedByURL:    "https://spec.graphql.org/October2021/#sec-Int",
}

var FloatType = &ScalarDefinition{
	Name:              "Float",
	Description:       "The Float scalar type represents signed double-precision fractional values.",
	MappedToProtoType: "double",
	SpecifiedByURL:    "https://spec.graphql.org/October2021/#sec-Float",
}

var BooleanType = &ScalarDefinition{
	Name:              "Boolean",
	Description:       "The Boolean scalar type represents true or false.",
	MappedToProtoType: "bool",
	SpecifiedByURL:    "https://spec.graphql.org/October2021/#sec-Boolean",
}

var IDType = &ScalarDefinition{
	Name:              "ID",
	Description:       "The ID scalar type represents a unique identifier, often used to refetch an object or as a key for caching.",
	MappedToProtoType: "string",
	SpecifiedByURL:    "https://spec.graphql.org/October2021/#sec-ID",
}
