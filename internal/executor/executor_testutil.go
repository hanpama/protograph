package executor

import (
	"testing"

	language "github.com/hanpama/protograph/internal/language"
)

// mustParseQuery parses a GraphQL query and fails the test on error.
func mustParseQuery(t *testing.T, q string) *language.QueryDocument {
	t.Helper()
	d, err := language.ParseQuery(q)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return d
}
