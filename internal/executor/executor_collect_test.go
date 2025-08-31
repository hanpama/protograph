package executor

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	language "github.com/hanpama/protograph/internal/language"
	schema "github.com/hanpama/protograph/internal/schema"
)

// Pattern: Result comparison
func TestCollectFields_And_Directives_Result(t *testing.T) {
	t.Run("Fragment merging and typename", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query":  {Name: "Query", Kind: schema.TypeKindObject, Fields: []*schema.Field{{Name: "a", Type: schema.NamedType("String")}}},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		doc := mustParseQuery(t, `{
                        a
                        ...F1
                        ...F2
                }
                fragment F1 on Query { a __typename }
                fragment F2 on Query { __typename }
                `)
		state := &executionState{schema: sch, document: doc, variableValues: map[string]any{}}
		got := collectFields(state, sch.Types["Query"], doc.Operations[0].SelectionSet).orderedFields()

		opSel := doc.Operations[0].SelectionSet
		frag1 := doc.Fragments.ForName("F1").SelectionSet
		frag2 := doc.Fragments.ForName("F2").SelectionSet
		want := []collectedField{
			{ResponseName: "a", Fields: []*language.Field{opSel[0].(*language.Field), frag1[0].(*language.Field)}},
			{ResponseName: "__typename", Fields: []*language.Field{frag1[1].(*language.Field), frag2[0].(*language.Field)}},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("collected fields mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Directives on scalar", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {Name: "Query", Kind: schema.TypeKindObject, Fields: []*schema.Field{
					{Name: "a", Type: schema.NamedType("String")},
					{Name: "b", Type: schema.NamedType("String")},
					{Name: "c", Type: schema.NamedType("String")},
				}},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		doc := mustParseQuery(t, `{ a b @skip(if: true) c @include(if: false) }`)
		state := &executionState{schema: sch, document: doc, variableValues: map[string]any{}}
		got := collectFields(state, sch.Types["Query"], doc.Operations[0].SelectionSet).orderedFields()

		opSel := doc.Operations[0].SelectionSet
		want := []collectedField{{ResponseName: "a", Fields: []*language.Field{opSel[0].(*language.Field)}}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("collected fields mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Directives on fragment spread", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {Name: "Query", Kind: schema.TypeKindObject, Fields: []*schema.Field{
					{Name: "a", Type: schema.NamedType("String")},
					{Name: "b", Type: schema.NamedType("String")},
					{Name: "c", Type: schema.NamedType("String")},
				}},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		doc := mustParseQuery(t, `{
                        a
                        ...Frag1 @include(if: true)
                        ...Frag2 @skip(if: true)
                }
                fragment Frag1 on Query { b }
                fragment Frag2 on Query { c }
                `)
		state := &executionState{schema: sch, document: doc, variableValues: map[string]any{}}
		got := collectFields(state, sch.Types["Query"], doc.Operations[0].SelectionSet).orderedFields()

		opSel := doc.Operations[0].SelectionSet
		frag1 := doc.Fragments.ForName("Frag1").SelectionSet
		want := []collectedField{
			{ResponseName: "a", Fields: []*language.Field{opSel[0].(*language.Field)}},
			{ResponseName: "b", Fields: []*language.Field{frag1[0].(*language.Field)}},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("collected fields mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Directives on inline fragment", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {Name: "Query", Kind: schema.TypeKindObject, Fields: []*schema.Field{
					{Name: "a", Type: schema.NamedType("String")},
					{Name: "b", Type: schema.NamedType("String")},
					{Name: "c", Type: schema.NamedType("String")},
				}},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		doc := mustParseQuery(t, `{
                        a
                        ... on Query @include(if: true) { b }
                        ... on Query @skip(if: true) { c }
                }`)
		state := &executionState{schema: sch, document: doc, variableValues: map[string]any{}}
		got := collectFields(state, sch.Types["Query"], doc.Operations[0].SelectionSet).orderedFields()

		opSel := doc.Operations[0].SelectionSet
		inline1 := opSel[1].(*language.InlineFragment)
		want := []collectedField{
			{ResponseName: "a", Fields: []*language.Field{opSel[0].(*language.Field)}},
			{ResponseName: "b", Fields: []*language.Field{inline1.SelectionSet[0].(*language.Field)}},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("collected fields mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Directives on anonymous inline fragment", func(t *testing.T) {
		sch := &schema.Schema{
			QueryType: "Query",
			Types: map[string]*schema.Type{
				"Query": {Name: "Query", Kind: schema.TypeKindObject, Fields: []*schema.Field{
					{Name: "a", Type: schema.NamedType("String")},
					{Name: "b", Type: schema.NamedType("String")},
					{Name: "c", Type: schema.NamedType("String")},
				}},
				"String": {Name: "String", Kind: schema.TypeKindScalar},
			},
		}
		doc := mustParseQuery(t, `{
                        a
                        ... @include(if: true) { b }
                        ... @skip(if: true) { c }
                }`)
		state := &executionState{schema: sch, document: doc, variableValues: map[string]any{}}
		got := collectFields(state, sch.Types["Query"], doc.Operations[0].SelectionSet).orderedFields()

		opSel := doc.Operations[0].SelectionSet
		inline1 := opSel[1].(*language.InlineFragment)
		want := []collectedField{
			{ResponseName: "a", Fields: []*language.Field{opSel[0].(*language.Field)}},
			{ResponseName: "b", Fields: []*language.Field{inline1.SelectionSet[0].(*language.Field)}},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("collected fields mismatch (-want +got):\n%s", diff)
		}
	})
}
