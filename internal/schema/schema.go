package schema

// Schema represents the complete GraphQL schema
type Schema struct {
	QueryType        string
	MutationType     string
	SubscriptionType string
	Types            map[string]*Type // All named types keyed by name
	Directives       map[string]*Directive
	Description      string
}

// GetQueryType returns the root query type (may be nil if absent)
func (s *Schema) GetQueryType() *Type { return s.Types[s.QueryType] }

// GetMutationType returns the root mutation type (may be nil if absent)
func (s *Schema) GetMutationType() *Type { return s.Types[s.MutationType] }

// GetSubscriptionType returns the root subscription type (may be nil if absent)
func (s *Schema) GetSubscriptionType() *Type { return s.Types[s.SubscriptionType] }

// Type is a named GraphQL type (object, interface, union, scalar, enum, input)
type Type struct {
	Name           string
	Kind           TypeKind
	Description    string
	Fields         []*Field      // For OBJECT and INTERFACE
	Interfaces     []string      // For OBJECT and INTERFACE (implemented/extended)
	PossibleTypes  []string      // For INTERFACE and UNION
	EnumValues     []*EnumValue  // For ENUM
	InputFields    []*InputValue // For INPUT_OBJECT
	SpecifiedByURL *string
	OneOf          bool
}

// Field represents a field on an object or interface
type Field struct {
	Name              string
	Description       string
	Type              *TypeRef
	Arguments         []*InputValue // formerly ArgumentDefinitionMap
	Async             bool
	IsDeprecated      bool
	DeprecationReason string
}

// TypeKind represents the kind of GraphQL type
type TypeKind string

const (
	TypeKindScalar      TypeKind = "SCALAR"
	TypeKindObject      TypeKind = "OBJECT"
	TypeKindInterface   TypeKind = "INTERFACE"
	TypeKindUnion       TypeKind = "UNION"
	TypeKindEnum        TypeKind = "ENUM"
	TypeKindInputObject TypeKind = "INPUT_OBJECT"
)

// TypeRef represents a reference to a type (can be wrapped)
type TypeRef struct {
	Kind   TypeRefKind
	OfType *TypeRef // For List and NonNull
	Named  string   // For named types
}

type TypeRefKind string

const (
	TypeRefKindNamed   TypeRefKind = "NAMED"
	TypeRefKindList    TypeRefKind = "LIST"
	TypeRefKindNonNull TypeRefKind = "NON_NULL"
)

// Helper functions for TypeRef
func (t *TypeRef) IsNonNull() bool {
	return t != nil && t.Kind == TypeRefKindNonNull
}

func (t *TypeRef) IsList() bool {
	if t.Kind == TypeRefKindList {
		return true
	}
	if t.Kind == TypeRefKindNonNull && t.OfType != nil {
		return t.OfType.Kind == TypeRefKindList
	}
	return false
}

func (t *TypeRef) Unwrap() *TypeRef {
	if t.Kind == TypeRefKindNonNull || t.Kind == TypeRefKindList {
		return t.OfType
	}
	return t
}

func (t *TypeRef) GetNamedType() string {
	current := t
	for current != nil {
		if current.Named != "" {
			return current.Named
		}
		current = current.OfType
	}
	return ""
}

type EnumValue struct {
	Name              string
	Description       string
	IsDeprecated      bool
	DeprecationReason string
}

type InputValue struct {
	Name              string
	Description       string
	Type              *TypeRef
	DefaultValue      any
	IsDeprecated      bool
	DeprecationReason string
}

type Directive struct {
	Name         string
	Description  string
	Locations    []string
	Arguments    []*InputValue // formerly ArgumentDefinitionMap
	IsRepeatable bool
}

func NonNullType(t *TypeRef) *TypeRef { return &TypeRef{Kind: TypeRefKindNonNull, OfType: t} }
func ListType(t *TypeRef) *TypeRef    { return &TypeRef{Kind: TypeRefKindList, OfType: t} }
func NamedType(name string) *TypeRef  { return &TypeRef{Kind: TypeRefKindNamed, Named: name} }

// IsNonNull reports whether the type is wrapped with Non-Null.
func IsNonNull(t *TypeRef) bool { return t != nil && t.IsNonNull() }

// IsList reports whether the type is (or is wrapped by) a list type.
func IsList(t *TypeRef) bool { return t != nil && t.IsList() }

// Unwrap removes one layer of Non-Null or List wrapping and returns the inner type.
func Unwrap(t *TypeRef) *TypeRef { return t.Unwrap() }

// GetNamedType returns the innermost named type for the given reference.
func GetNamedType(t *TypeRef) string { return t.GetNamedType() }
