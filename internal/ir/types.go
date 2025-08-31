package ir

import (
	"sort"
	"strings"
)

type Project struct {
	Services    map[ServiceID]*Service             `json:"services"`
	Schema      *Schema                            `json:"schema,omitempty"`
	Definitions map[string]*Definition             `json:"definitions"`
	Directives  map[string]*DirectiveDefinition    `json:"directives"`
	Loaders     map[LoaderID]*LoaderDefinition     `json:"loaders"`
	Resolvers   map[ResolverID]*ResolverDefinition `json:"resolvers"`
}

type Schema struct {
	QueryType        string `json:"queryType,omitempty"`
	MutationType     string `json:"mutationType,omitempty"`
	SubscriptionType string `json:"subscriptionType,omitempty"`
}

type Service struct {
	ID          ServiceID `json:"id"`
	Name        string    `json:"name"`
	PackagePath []string  `json:"packagePath"`
	FilePath    string    `json:"filePath,omitempty"`

	Definitions  []string     `json:"sources"`
	Directives   []string     `json:"directives"`
	Loaders      []LoaderID   `json:"loaders"`
	Resolvers    []ResolverID `json:"resolvers"`
	Dependencies []ServiceID  `json:"dependencies"`
}

// ServiceID is a unique identifier for a service.
// ex. "com/example/myapp/User"
type ServiceID string

type Definition struct {
	Object    *ObjectDefinition    `json:"object,omitempty"`
	Interface *InterfaceDefinition `json:"interface,omitempty"`
	Union     *UnionDefinition     `json:"union,omitempty"`
	Input     *InputDefinition     `json:"input,omitempty"`
	Enum      *EnumDefinition      `json:"enum,omitempty"`
	Scalar    *ScalarDefinition    `json:"scalar,omitempty"`
}

type ObjectDefinition struct {
	Name        string                      `json:"name"`
	Description string                      `json:"description,omitempty"`
	Fields      map[string]*FieldDefinition `json:"fields"`
	Interfaces  map[string]*InterfaceImpl   `json:"interfaces"`
	IDFields    []string                    `json:"idFields"`
}

type InterfaceDefinition struct {
	Name          string                      `json:"name"`
	Description   string                      `json:"description,omitempty"`
	Fields        map[string]*FieldDefinition `json:"fields"`
	Interfaces    map[string]*InterfaceImpl   `json:"interfaces"`
	PossibleTypes []string                    `json:"possibleTypes"`
}

type UnionDefinition struct {
	Name        string                          `json:"name"`
	Description string                          `json:"description,omitempty"`
	Types       map[string]*UnionTypeDefinition `json:"types"`
}

type UnionTypeDefinition struct {
	Name  string `json:"name"`
	Index int    `json:"index"`
}

type InputDefinition struct {
	Name        string                           `json:"name"`
	Description string                           `json:"description,omitempty"`
	InputValues map[string]*InputValueDefinition `json:"inputValues"`
	OneOf       bool                             `json:"oneOf,omitempty"`
}

type EnumDefinition struct {
	Name        string                          `json:"name"`
	Description string                          `json:"description,omitempty"`
	Values      map[string]*EnumValueDefinition `json:"values"`
}

type EnumValueDefinition struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Index       int          `json:"index"`
	Deprecation *Deprecation `json:"deprecation,omitempty"`
}

type ScalarDefinition struct {
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	MappedToProtoType string `json:"mappedToProtoType,omitempty"`
	SpecifiedByURL    string `json:"specifiedByURL,omitempty"`
}

type DirectiveDefinition struct {
	Name        string                         `json:"name"`
	Description string                         `json:"description,omitempty"`
	Args        map[string]*ArgumentDefinition `json:"args"`
	Repeatable  bool                           `json:"repeatable,omitempty"`
	Locations   []string                       `json:"locations"`
}

type InterfaceImpl struct {
	Interface string `json:"interface"`
	Index     int    `json:"index"`
}

type FieldDefinition struct {
	Name              string                         `json:"name"`
	Description       string                         `json:"description,omitempty"`
	Index             int                            `json:"index"`
	Args              map[string]*ArgumentDefinition `json:"args"`
	Type              *TypeExpr                      `json:"fieldType"`
	IsInternal        bool                           `json:"isInternal,omitempty"`
	Deprecation       *Deprecation                   `json:"deprecation,omitempty"`
	ResolveBySource   *FieldResolveBySource          `json:"bySource,omitempty"`
	ResolveByResolver *FieldResolveByResolver        `json:"byResolver,omitempty"`
	ResolveByLoader   *FieldResolveByLoader          `json:"byLoader,omitempty"`
}

type FieldResolveBySource struct {
	SourceField string `json:"sourceField"`
}

type FieldResolveByResolver struct {
	ResolverID ResolverID        `json:"resolverId"`
	With       map[string]string `json:"with"`
}

type FieldResolveByLoader struct {
	LoaderID LoaderID          `json:"loaderId"`
	With     map[string]string `json:"with"`
}

type ArgumentDefinition struct {
	Name         string       `json:"name"`
	Description  string       `json:"description,omitempty"`
	Index        int          `json:"index"`
	DefaultValue Value        `json:"defaultValue,omitempty"`
	Type         *TypeExpr    `json:"type"`
	Deprecation  *Deprecation `json:"deprecation,omitempty"`
}

type InputValueDefinition struct {
	Name         string       `json:"name"`
	Description  string       `json:"description,omitempty"`
	Index        int          `json:"index"`
	DefaultValue Value        `json:"defaultValue,omitempty"`
	Type         *TypeExpr    `json:"type"`
	Deprecation  *Deprecation `json:"deprecation,omitempty"`
}

type Argument struct {
	Name  string `json:"name"`
	Value Value  `json:"value,omitempty"`
}

type Value = any

type Deprecation struct {
	Reason string `json:"reason,omitempty"`
}

type LoaderDefinition struct {
	ID         LoaderID              `json:"id"`
	TargetType string                `json:"targetType"`      // The type this loader loads (e.g., "User", "Post")
	KeyFields  []string              `json:"keyFields"`       // Field names used as keys (e.g., ["id"] or ["userId", "postId"])
	Batch      bool                  `json:"batch,omitempty"` // true to generate BatchLoad*, false for Load*
	Args       map[string]*MethodArg `json:"args"`            // Arguments for the loader
}

// LoaderID is a unique identifier for a loader.
// e.g. "User:id", "Like:postId:userId"
type LoaderID string

type ResolverDefinition struct {
	ID          ResolverID            `json:"id"`
	Parent      string                `json:"parent"`
	Field       string                `json:"field"`
	Args        map[string]*MethodArg `json:"args"`
	Batch       bool                  `json:"batch,omitempty"`
	ReturnType  *TypeExpr             `json:"returnType"`
	Description string                `json:"description,omitempty"`
}

type MethodArg struct {
	Name        string    `json:"name"`
	Type        *TypeExpr `json:"type"`
	Index       int       `json:"index"`
	Description string    `json:"description,omitempty"`
}

// ResolverID is a unique identifier for a resolver.
// e.g. "User:likes", "Post:author"
type ResolverID string

// TypeExpr represents a GraphQL type expression (e.g. String, [String!], String!).
type TypeExpr struct {
	Kind   TypeExprKind `json:"kind"`
	OfType *TypeExpr    `json:"ofType,omitempty"`
	Named  string       `json:"named,omitempty"`
}

type TypeExprKind string

const (
	TypeExprKindNamed   TypeExprKind = "NAMED"
	TypeExprKindList    TypeExprKind = "LIST"
	TypeExprKindNonNull TypeExprKind = "NON_NULL"
)

func (t *TypeExpr) unwrap() string {
	if t == nil {
		return ""
	}
	if t.Kind == TypeExprKindNamed {
		return t.Named
	}
	return t.OfType.unwrap()
}

func (t *TypeExpr) String() string {
	if t == nil {
		return "Unknown"
	}

	switch t.Kind {
	case TypeExprKindNamed:
		return t.Named
	case TypeExprKindList:
		return "[" + t.OfType.String() + "]"
	case TypeExprKindNonNull:
		inner := t.OfType.String()
		if strings.HasSuffix(inner, "!") {
			return inner
		}
		return inner + "!"
	default:
		return "Unknown"
	}
}

func (e *ObjectDefinition) OrderedFields() []*FieldDefinition {
	fields := make([]*FieldDefinition, 0, len(e.Fields))
	for _, field := range e.Fields {
		fields = append(fields, field)
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Index < fields[j].Index
	})
	return fields
}

func (e *EnumDefinition) OrderedValues() []*EnumValueDefinition {
	values := make([]*EnumValueDefinition, 0, len(e.Values))
	for _, val := range e.Values {
		values = append(values, val)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].Index < values[j].Index
	})
	return values
}

func (e *InputDefinition) OrderedInputValues() []*InputValueDefinition {
	values := make([]*InputValueDefinition, 0, len(e.InputValues))
	for _, val := range e.InputValues {
		values = append(values, val)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].Index < values[j].Index
	})
	return values
}

func (r *ResolverDefinition) OrderedArgs() []*MethodArg {
	args := make([]*MethodArg, 0, len(r.Args))
	for _, arg := range r.Args {
		args = append(args, arg)
	}
	sort.Slice(args, func(i, j int) bool {
		return args[i].Index < args[j].Index
	})
	return args
}

func (l *LoaderDefinition) OrderedArgs() []*MethodArg {
	args := make([]*MethodArg, 0, len(l.Args))
	for _, arg := range l.Args {
		args = append(args, arg)
	}
	sort.Slice(args, func(i, j int) bool {
		return args[i].Index < args[j].Index
	})
	return args
}
