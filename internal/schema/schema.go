package schema

import "sort"

// Schema represents the complete GraphQL schema
type Schema struct {
	QueryType        string
	MutationType     string
	SubscriptionType string
	Types            map[string]*Type // All named types keyed by name
	Directives       map[string]*Directive
	Description      string
}

// NewSchema constructs an empty schema with initialized maps.
func NewSchema(description string) *Schema {
	return &Schema{
		Types:       make(map[string]*Type),
		Directives:  make(map[string]*Directive),
		Description: description,
	}
}

// GetQueryType returns the root query type (may be nil if absent)
func (s *Schema) GetQueryType() *Type { return s.Types[s.QueryType] }

// GetMutationType returns the root mutation type (may be nil if absent)
func (s *Schema) GetMutationType() *Type { return s.Types[s.MutationType] }

// GetSubscriptionType returns the root subscription type (may be nil if absent)
func (s *Schema) GetSubscriptionType() *Type { return s.Types[s.SubscriptionType] }

// SetQueryType sets the schema's root query type name.
func (s *Schema) SetQueryType(name string) *Schema {
	s.QueryType = name
	return s
}

// SetMutationType sets the schema's root mutation type name.
func (s *Schema) SetMutationType(name string) *Schema {
	s.MutationType = name
	return s
}

// SetSubscriptionType sets the schema's root subscription type name.
func (s *Schema) SetSubscriptionType(name string) *Schema {
	s.SubscriptionType = name
	return s
}

// AddType registers the given type on the schema, overriding by name.
func (s *Schema) AddType(t *Type) *Schema {
	s.Types[t.Name] = t
	return s
}

// AddDirective registers the given directive on the schema, overriding by name.
func (s *Schema) AddDirective(d *Directive) *Schema {
	s.Directives[d.Name] = d
	return s
}

// Type is a named GraphQL type (object, interface, union, scalar, enum, input)
type Type struct {
	Name           string
	Kind           TypeKind
	Description    string
	Fields         map[string]*Field      // For OBJECT and INTERFACE
	Interfaces     []string               // For OBJECT and INTERFACE (implemented/extended)
	PossibleTypes  []string               // For INTERFACE and UNION
	EnumValues     []*EnumValue           // For ENUM
	InputFields    map[string]*InputValue // For INPUT_OBJECT
	SpecifiedByURL *string
	OneOf          bool
}

// NewType constructs a type with initialized field and input-field maps.
func NewType(name string, kind TypeKind, description string) *Type {
	return &Type{
		Name:        name,
		Kind:        kind,
		Description: description,
		Fields:      make(map[string]*Field),
		InputFields: make(map[string]*InputValue),
	}
}

// SetDescription sets the type description.
func (t *Type) SetDescription(description string) *Type {
	t.Description = description
	return t
}

// SetOneOf marks the type as a oneof input object.
func (t *Type) SetOneOf(oneOf bool) *Type {
	t.OneOf = oneOf
	return t
}

// SetSpecifiedByURL sets or clears the specifiedBy URL for scalar types.
func (t *Type) SetSpecifiedByURL(url string) *Type {
	if url == "" {
		t.SpecifiedByURL = nil
		return t
	}
	t.SpecifiedByURL = &url
	return t
}

// AddInterface records that the type implements the provided interface.
func (t *Type) AddInterface(name string) *Type {
	for _, existing := range t.Interfaces {
		if existing == name {
			return t
		}
	}
	t.Interfaces = append(t.Interfaces, name)
	return t
}

// AddPossibleType records a possible type for interfaces or unions.
func (t *Type) AddPossibleType(name string) *Type {
	for _, existing := range t.PossibleTypes {
		if existing == name {
			return t
		}
	}
	t.PossibleTypes = append(t.PossibleTypes, name)
	return t
}

// AddEnumValue appends an enum value definition.
func (t *Type) AddEnumValue(value *EnumValue) *Type {
	t.EnumValues = append(t.EnumValues, value)
	return t
}

// AddField registers a field on the type, auto-assigning an index when absent.
func (t *Type) AddField(field *Field) *Type {
	field.Index = nextFieldIndex(t.Fields)
	t.Fields[field.Name] = field
	return t
}

// AddInputField registers an input field on the type, auto-assigning an index when absent.
func (t *Type) AddInputField(input *InputValue) *Type {
	input.Index = nextInputValueIndex(t.InputFields)
	t.InputFields[input.Name] = input
	return t
}

// Field represents a field on an object or interface
type Field struct {
	Name              string
	Description       string
	Type              *TypeRef
	Arguments         map[string]*InputValue
	Async             bool
	IsDeprecated      bool
	DeprecationReason string
	Index             int
}

// NewField constructs a field definition with the provided name, description, and type reference.
func NewField(name, description string, typeRef *TypeRef) *Field {
	return &Field{
		Name:        name,
		Description: description,
		Type:        typeRef,
		Arguments:   make(map[string]*InputValue),
	}
}

// SetAsync marks whether the field resolves asynchronously.
func (f *Field) SetAsync(async bool) *Field {
	f.Async = async
	return f
}

// Deprecate marks the field as deprecated with an optional reason.
func (f *Field) Deprecate(reason string) *Field {
	f.IsDeprecated = true
	f.DeprecationReason = reason
	return f
}

// AddArgument registers an argument definition for the field, assigning an index when absent.
func (f *Field) AddArgument(arg *InputValue) *Field {
	arg.Index = nextArgumentIndex(f.Arguments)
	f.Arguments[arg.Name] = arg
	return f
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

// NewEnumValue constructs an enum value definition.
func NewEnumValue(name, description string) *EnumValue {
	return &EnumValue{Name: name, Description: description}
}

// Deprecate marks the enum value as deprecated with an optional reason.
func (e *EnumValue) Deprecate(reason string) *EnumValue {
	e.IsDeprecated = true
	e.DeprecationReason = reason
	return e
}

type InputValue struct {
	Name              string
	Description       string
	Type              *TypeRef
	DefaultValue      any
	IsDeprecated      bool
	DeprecationReason string
	Index             int
}

// NewInputValue constructs an input value definition with the provided name, description, and type.
func NewInputValue(name, description string, typeRef *TypeRef) *InputValue {
	return &InputValue{Name: name, Description: description, Type: typeRef}
}

// SetDefault assigns the default value.
func (v *InputValue) SetDefault(value any) *InputValue {
	v.DefaultValue = value
	return v
}

// SetIndex sets the input value order index.
func (v *InputValue) SetIndex(index int) *InputValue {
	v.Index = index
	return v
}

// Deprecate marks the input value as deprecated with an optional reason.
func (v *InputValue) Deprecate(reason string) *InputValue {
	v.IsDeprecated = true
	v.DeprecationReason = reason
	return v
}

type Directive struct {
	Name         string
	Description  string
	Locations    []string
	Arguments    []*InputValue // formerly ArgumentDefinitionMap
	IsRepeatable bool
}

// NewDirective constructs a directive definition with the provided name and description.
func NewDirective(name, description string) *Directive {
	return &Directive{Name: name, Description: description}
}

// SetRepeatable marks whether the directive is repeatable.
func (d *Directive) SetRepeatable(repeatable bool) *Directive {
	d.IsRepeatable = repeatable
	return d
}

// AddLocation appends a directive location if not already present.
func (d *Directive) AddLocation(location string) *Directive {
	for _, existing := range d.Locations {
		if existing == location {
			return d
		}
	}
	d.Locations = append(d.Locations, location)
	return d
}

// AddArgument appends an argument definition, maintaining insertion order.
func (d *Directive) AddArgument(arg *InputValue) *Directive {
	arg.Index = len(d.Arguments)
	d.Arguments = append(d.Arguments, arg)
	return d
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

// Field returns the named field for object or interface types.
func (t *Type) Field(name string) *Field {
	return t.Fields[name]
}

// GetOrderedFields returns fields sorted by their index for deterministic iteration.
func (t *Type) GetOrderedFields() []*Field {
	if len(t.Fields) == 0 {
		return nil
	}
	fields := make([]*Field, 0, len(t.Fields))
	for _, f := range t.Fields {
		fields = append(fields, f)
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Index < fields[j].Index })
	return fields
}

// InputField returns the named input field for input object types.
func (t *Type) InputField(name string) *InputValue {
	return t.InputFields[name]
}

// GetOrderedInputFields returns input fields sorted by their index for deterministic iteration.
func (t *Type) GetOrderedInputFields() []*InputValue {
	if len(t.InputFields) == 0 {
		return nil
	}
	fields := make([]*InputValue, 0, len(t.InputFields))
	for _, f := range t.InputFields {
		fields = append(fields, f)
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Index < fields[j].Index })
	return fields
}

// Argument returns the named argument for the field, if present.
func (f *Field) Argument(name string) *InputValue {
	return f.Arguments[name]
}

// GetOrderedArguments returns field arguments sorted by their index for deterministic iteration.
func (f *Field) GetOrderedArguments() []*InputValue {
	if len(f.Arguments) == 0 {
		return nil
	}
	args := make([]*InputValue, 0, len(f.Arguments))
	for _, a := range f.Arguments {
		args = append(args, a)
	}
	sort.Slice(args, func(i, j int) bool { return args[i].Index < args[j].Index })
	return args
}

func nextFieldIndex(fields map[string]*Field) int {
	max := -1
	for _, f := range fields {
		if f.Index > max {
			max = f.Index
		}
	}
	return max + 1
}

func nextInputValueIndex(values map[string]*InputValue) int {
	max := -1
	for _, v := range values {
		if v.Index > max {
			max = v.Index
		}
	}
	return max + 1
}

func nextArgumentIndex(values map[string]*InputValue) int {
	return nextInputValueIndex(values)
}

// NewFieldMap constructs a field map from the provided fields, assigning
// sequential indexes when they are zero-valued.
func NewFieldMap(fields ...*Field) map[string]*Field {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]*Field, len(fields))
	for i, field := range fields {
		if field == nil || field.Name == "" {
			continue
		}
		if field.Index == 0 && i > 0 {
			field.Index = i
		}
		out[field.Name] = field
	}
	return out
}

// NewInputValueMap constructs an input value map from the provided values,
// assigning sequential indexes when they are zero-valued.
func NewInputValueMap(values ...*InputValue) map[string]*InputValue {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]*InputValue, len(values))
	for i, value := range values {
		if value == nil || value.Name == "" {
			continue
		}
		if value.Index == 0 && i > 0 {
			value.Index = i
		}
		out[value.Name] = value
	}
	return out
}
