package schema

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hanpama/protograph/internal/ir"
	"github.com/stretchr/testify/require"
)

func TestSchemaSnapshot(t *testing.T) {
	// Create in-memory discovery with two services to test extensions
	disc := ir.NewInMemoryDiscovery([]ir.InMemoryService{
		{
			Package: "test",
			Name:    "base",
			Content: mustReadFile(t, "testdata/base.graphql"),
		},
		{
			Package: "test",
			Name:    "extensions",
			Content: mustReadFile(t, "testdata/extensions.graphql"),
		},
	})

	// Build ir project
	proj, err := ir.Build(context.Background(), disc)
	require.NoError(t, err, "failed to build ir project")

	// Build schema from ir
	schema, err := BuildFromIR(proj)
	require.NoError(t, err, "failed to build schema from IR")

	// Convert to JSON for snapshot comparison
	actual, err := json.MarshalIndent(schema, "", "  ")
	require.NoError(t, err, "failed to marshal schema to JSON")

	// Snapshot file path
	snapshotPath := filepath.Join("testdata", "schema_snapshot.json")

	// If snapshot doesn't exist, create it
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		err := os.WriteFile(snapshotPath, actual, 0644)
		require.NoError(t, err, "failed to write snapshot file")
		t.Logf("Created snapshot file: %s", snapshotPath)
		return
	}

	// Read existing snapshot
	expected, err := os.ReadFile(snapshotPath)
	require.NoError(t, err, "failed to read snapshot file")

	// Compare snapshots
	if diff := cmp.Diff(string(expected), string(actual)); diff != "" {
		t.Errorf("Schema snapshot mismatch (-want +got):\n%s", diff)
	}
}

func TestSchemaRenderSnapshot(t *testing.T) {
	// Create in-memory discovery with two services to test extensions
	disc := ir.NewInMemoryDiscovery([]ir.InMemoryService{
		{
			Package: "test",
			Name:    "base",
			Content: mustReadFile(t, "testdata/base.graphql"),
		},
		{
			Package: "test",
			Name:    "extensions",
			Content: mustReadFile(t, "testdata/extensions.graphql"),
		},
	})

	// Build ir project
	proj, err := ir.Build(context.Background(), disc)
	require.NoError(t, err, "failed to build ir project")

	// Build schema from ir
	schema, err := BuildFromIR(proj)
	require.NoError(t, err, "failed to build schema from IR")

	// Render schema to SDL
	actual := Render(schema)

	// Snapshot file path
	snapshotPath := filepath.Join("testdata", "schema_rendered.graphql")

	// If snapshot doesn't exist, create it
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		err := os.WriteFile(snapshotPath, []byte(actual), 0644)
		require.NoError(t, err, "failed to write snapshot file")
		t.Logf("Created snapshot file: %s", snapshotPath)
		return
	}

	// Read existing snapshot
	expected, err := os.ReadFile(snapshotPath)
	require.NoError(t, err, "failed to read snapshot file")

	// Compare snapshots
	if diff := cmp.Diff(string(expected), actual); diff != "" {
		t.Errorf("Rendered schema snapshot mismatch (-want +got):\n%s", diff)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read file: %s", path)
	return string(content)
}
