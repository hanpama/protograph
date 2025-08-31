package schema

var stringType = &Type{
	Name:        "String",
	Kind:        TypeKindScalar,
	Description: "The `String` scalar type represents textual data, represented as UTF-8 character sequences.",
}

var intType = &Type{
	Name:        "Int",
	Kind:        TypeKindScalar,
	Description: "The `Int` scalar type represents non-fractional signed whole numeric values.",
}

var floatType = &Type{
	Name:        "Float",
	Kind:        TypeKindScalar,
	Description: "The `Float` scalar type represents signed double-precision fractional values.",
}

var booleanType = &Type{
	Name:        "Boolean",
	Kind:        TypeKindScalar,
	Description: "The `Boolean` scalar type represents `true` or `false`.",
}

var idType = &Type{
	Name:        "ID",
	Kind:        TypeKindScalar,
	Description: "The `ID` scalar type represents a unique identifier, often used to refetch an object or as a key for caching.",
}

var includeDirective = &Directive{
	Name:        "include",
	Description: "Directs the executor to include this field or fragment only when the `if` argument is true.",
	Arguments: []*InputValue{
		{
			Name:        "if",
			Description: "Included when true.",
			Type:        &TypeRef{Kind: TypeRefKindNonNull, OfType: &TypeRef{Kind: TypeRefKindNamed, Named: "Boolean"}},
		},
	},
	Locations:    []string{"FIELD", "FRAGMENT_SPREAD", "INLINE_FRAGMENT"},
	IsRepeatable: false,
}

var skipDirective = &Directive{
	Name:        "skip",
	Description: "Directs the executor to skip this field or fragment when the `if` argument is true.",
	Arguments: []*InputValue{
		{
			Name:        "if",
			Description: "Skipped when true.",
			Type:        &TypeRef{Kind: TypeRefKindNonNull, OfType: &TypeRef{Kind: TypeRefKindNamed, Named: "Boolean"}},
		},
	},
	Locations:    []string{"FIELD", "FRAGMENT_SPREAD", "INLINE_FRAGMENT"},
	IsRepeatable: false,
}
