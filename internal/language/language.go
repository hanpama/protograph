package language

import (
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

func ParseQuery(source string) (*QueryDocument, error) {
	doc, err := parser.ParseQuery(&ast.Source{Input: source})
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func ParseSchema(name, source string) (*SchemaDocument, error) {
	doc, err := parser.ParseSchema(&ast.Source{Name: name, Input: source})
	if err != nil {
		return nil, err
	}
	return doc, nil
}
