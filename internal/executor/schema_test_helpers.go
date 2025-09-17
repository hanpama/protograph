package executor

import schema "github.com/hanpama/protograph/internal/schema"

func newSchemaWithQueryType(query *schema.Type, additional ...*schema.Type) *schema.Schema {
	sch := schema.NewSchema("")
	if query != nil {
		sch.SetQueryType(query.Name)
		sch.AddType(query)
	}
	for _, t := range additional {
		sch.AddType(t)
	}
	return sch
}

func newObjectType(name string, fields ...*schema.Field) *schema.Type {
	t := schema.NewType(name, schema.TypeKindObject, "")
	for _, field := range fields {
		t.AddField(field)
	}
	return t
}

func newScalarType(name string) *schema.Type {
	return schema.NewType(name, schema.TypeKindScalar, "")
}
