package ir_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hanpama/protograph/internal/ir"
)

func TestGoodSnapshot(t *testing.T) {
	type testCase struct {
		name      string
		snapshot  string
		discovery ir.Discovery
	}

	for _, tc := range []testCase{
		{
			name:     "loader_basic",
			snapshot: "testdata/good/loader_basic.json",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/good/loader_basic.graphql"),
				},
			}),
		},
		{
			name:     "loader_compound",
			snapshot: "testdata/good/loader_compound.json",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/good/loader_compound.graphql"),
				},
			}),
		},
		{
			name:     "loader_default_ids",
			snapshot: "testdata/good/loader_default_ids.json",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/good/loader_default_ids.graphql"),
				},
			}),
		},
		{
			name:     "internal_field",
			snapshot: "testdata/good/internal_field.json",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/good/internal_field.graphql"),
				},
			}),
		},
		{
			name:     "load_field",
			snapshot: "testdata/good/load_field.json",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/good/load_field.graphql"),
				},
			}),
		},
		{
			name:     "resolve_field",
			snapshot: "testdata/good/resolve_field.json",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/good/resolve_field.graphql"),
				},
			}),
		},
		{
			name:     "multiple_loaders",
			snapshot: "testdata/good/multiple_loaders.json",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/good/multiple_loaders.graphql"),
				},
			}),
		},
		{
			name:     "map_scalar",
			snapshot: "testdata/good/map_scalar.json",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/good/map_scalar.graphql"),
				},
			}),
		},
		{
			name:     "mutation",
			snapshot: "testdata/good/mutation.json",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/good/mutation.graphql"),
				},
			}),
		},
		{
			name:     "types",
			snapshot: "testdata/good/types.json",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/good/types.graphql"),
				},
			}),
		},
		{
			name:     "types",
			snapshot: "testdata/good/deps.json",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "deps_a",
					Content: mustReadData("testdata/good/deps_a.graphql"),
				},
				{
					Package: "testpackage",
					Name:    "deps_b",
					Content: mustReadData("testdata/good/deps_b.graphql"),
				},
			}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project, err := ir.Build(t.Context(), tc.discovery)
			if err != nil {
				t.Fatalf("Build failed: %v", err)
			}
			// if snapshotg file does not exist, create it
			if _, err := os.Stat(tc.snapshot); os.IsNotExist(err) {
				file, err := os.Create(tc.snapshot)
				if err != nil {
					t.Fatalf("Failed to create snapshot file: %v", err)
				}
				defer file.Close()
				enc := json.NewEncoder(file)
				enc.SetIndent("", "  ")
				if err := enc.Encode(project); err != nil {
					t.Fatalf("Failed to write snapshot: %v", err)
				}
				t.Logf("Snapshot created: %s", tc.snapshot)
				return
			}

			file, err := os.Open(tc.snapshot)
			if err != nil {
				t.Fatalf("Failed to open snapshot file: %v", err)
			}
			defer file.Close()
			var expectedProject *ir.Project
			if err := json.NewDecoder(file).Decode(&expectedProject); err != nil {
				t.Fatalf("Failed to decode snapshot: %v", err)
			}

			if diff := cmp.Diff(expectedProject, project); diff != "" {
				t.Errorf("Project mismatch (-expected +got):\n%s", diff)
			}
		})
	}

}

func TestBadSnapshot(t *testing.T) {
	type testCase struct {
		name      string
		discovery ir.Discovery
		wantErr   string
	}

	for _, tc := range []testCase{
		{
			name: "loader_errors",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/bad/loader_errors.graphql"),
				},
			}),
			wantErr: "has no @id fields or 'id' field",
		},
		{
			name: "load_type_mismatch",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/bad/load_type_mismatch.graphql"),
				},
			}),
			wantErr: "type mismatch",
		},
		{
			name: "load_missing_mapping_field",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/bad/load_missing_mapping_field.graphql"),
				},
			}),
			wantErr: "references unknown parent field",
		},
		{
			name: "resolve_errors",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/bad/resolve_errors.graphql"),
				},
			}),
			wantErr: "must not have arguments",
		},
		{
			name: "load_conflict_load_resolve",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/bad/load_conflict_load_resolve.graphql"),
				},
			}),
			wantErr: "cannot have both @load and @resolve",
		},
		{
			name: "resolve_with_conflict",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/bad/resolve_with_conflict.graphql"),
				},
			}),
			wantErr: "conflicts with argument",
		},
		{
			name: "interface_directive_errors",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/bad/interface_directive_errors.graphql"),
				},
			}),
			wantErr: "interface",
		},
		{
			name: "load_with_arguments",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/bad/load_with_arguments.graphql"),
				},
			}),
			wantErr: "must not have arguments",
		},
		{
			name: "interface_and_union",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestService",
					Content: mustReadData("testdata/bad/interface_and_union.graphql"),
				},
			}),
			wantErr: "must also implement interface",
		},
		{
			name: "cyclic_services",
			discovery: ir.NewInMemoryDiscovery([]ir.InMemoryService{
				{
					Package: "testpackage",
					Name:    "TestServiceA",
					Content: mustReadData("testdata/bad/cyclic_services_a.graphql"),
				},
				{
					Package: "testpackage",
					Name:    "TestServiceB",
					Content: mustReadData("testdata/bad/cyclic_services_b.graphql"),
				},
			}),
			wantErr: "Service dependency cycle",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ir.Build(t.Context(), tc.discovery)
			if err == nil {
				t.Fatal("expected error but got none")
			}
			// Check if error message contains expected substring
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func mustReadData(filename string) string {
	data, err := os.ReadFile(filename)
	if err != nil {
		panic(fmt.Sprintf("failed to read test data file %s: %v", filename, err))
	}
	return string(data)
}
