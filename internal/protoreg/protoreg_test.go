package protoreg_test

import (
	"context"
	"path"
	"testing"

	"github.com/hanpama/protograph/internal/grpcrt"
	"github.com/hanpama/protograph/internal/ir"
	"github.com/hanpama/protograph/internal/protoreg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildTestRegistry(t *testing.T) grpcrt.Registry {
	t.Helper()
	discovery, err := ir.NewFileSystemDiscovery(context.Background(), path.Join("testdata", "schema"), "testdata.proto")
	require.NoError(t, err)

	proj, err := ir.Build(context.Background(), discovery)
	require.NoError(t, err)

	reg, err := protoreg.Build(proj)
	require.NoError(t, err)

	return reg
}

func TestRender(t *testing.T) {
	discovery, err := ir.NewFileSystemDiscovery(t.Context(), path.Join("testdata", "schema"), "testdata.proto")
	if err != nil {
		t.Fatal(err)
	}
	proj, err := ir.Build(t.Context(), discovery)
	if err != nil {
		t.Fatal(err)
	}
	reg, err := protoreg.Build(proj)
	if err != nil {
		t.Fatal(err)
	}

	err = protoreg.Render(reg, ".")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetSourceFieldDescriptor(t *testing.T) {
	reg := buildTestRegistry(t)

	tests := []struct {
		name         string
		objectType   string
		graphqlField string
		shouldExist  bool
		fieldName    string // expected proto field name
	}{
		{
			name:         "User id field",
			objectType:   "User",
			graphqlField: "id",
			shouldExist:  true,
			fieldName:    "id",
		},
		{
			name:         "User name field",
			objectType:   "User",
			graphqlField: "name",
			shouldExist:  true,
			fieldName:    "name",
		},
		{
			name:         "User role field",
			objectType:   "User",
			graphqlField: "role",
			shouldExist:  true,
			fieldName:    "role",
		},
		{
			name:         "Post title field",
			objectType:   "Post",
			graphqlField: "title",
			shouldExist:  true,
			fieldName:    "title",
		},
		{
			name:         "Post content field",
			objectType:   "Post",
			graphqlField: "content",
			shouldExist:  true,
			fieldName:    "content",
		},
		{
			name:         "Post authorId internal field",
			objectType:   "Post",
			graphqlField: "authorId",
			shouldExist:  true,
			fieldName:    "author_id",
		},
		{
			name:         "Non-existent field",
			objectType:   "User",
			graphqlField: "nonExistent",
			shouldExist:  false,
		},
		{
			name:         "Non-existent type",
			objectType:   "NonExistent",
			graphqlField: "field",
			shouldExist:  false,
		},
		{
			name:         "Post author field (should not be in source)",
			objectType:   "Post",
			graphqlField: "author",
			shouldExist:  false, // @load fields are not in source
		},
		{
			name:         "Post likeCount field (should not be in source)",
			objectType:   "Post",
			graphqlField: "likeCount",
			shouldExist:  false, // @resolve fields are not in source
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := reg.GetSourceFieldDescriptor(tt.objectType, tt.graphqlField)

			if tt.shouldExist {
				require.NotNil(t, fd, "Field descriptor should exist for %s.%s", tt.objectType, tt.graphqlField)
				assert.Equal(t, tt.fieldName, string(fd.Name()), "Field name should match")
			} else {
				assert.Nil(t, fd, "Field descriptor should not exist for %s.%s", tt.objectType, tt.graphqlField)
			}
		})
	}
}

func TestGetSingleResolverDescriptor(t *testing.T) {
	reg := buildTestRegistry(t)

	tests := []struct {
		name        string
		objectType  string
		field       string
		shouldExist bool
		methodName  string // expected method name pattern
	}{
		{
			name:        "Query getUser resolver",
			objectType:  "Query",
			field:       "getUser",
			shouldExist: true,
			methodName:  "ResolveQueryGetUser",
		},
		{
			name:        "Mutation createUser resolver",
			objectType:  "Mutation",
			field:       "createUser",
			shouldExist: true,
			methodName:  "ResolveMutationCreateUser",
		},
		{
			name:        "Post likeCount resolver (batch=true, should not be single)",
			objectType:  "Post",
			field:       "likeCount",
			shouldExist: false, // This is a batch resolver
		},
		{
			name:        "Non-existent resolver",
			objectType:  "User",
			field:       "nonExistent",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := reg.GetSingleResolverDescriptor(tt.objectType, tt.field)

			if tt.shouldExist {
				require.NotNil(t, md, "Method descriptor should exist for %s.%s", tt.objectType, tt.field)
				assert.Equal(t, tt.methodName, string(md.Name()), "Method name should match")
			} else {
				assert.Nil(t, md, "Method descriptor should not exist for %s.%s", tt.objectType, tt.field)
			}
		})
	}
}

func TestGetBatchResolverDescriptor(t *testing.T) {
	reg := buildTestRegistry(t)

	tests := []struct {
		name        string
		objectType  string
		field       string
		shouldExist bool
		methodName  string // expected method name pattern
	}{
		{
			name:        "Post likeCount batch resolver",
			objectType:  "Post",
			field:       "likeCount",
			shouldExist: true,
			methodName:  "BatchResolvePostLikeCount",
		},
		{
			name:        "Query getUser (not batch)",
			objectType:  "Query",
			field:       "getUser",
			shouldExist: false, // This is a single resolver
		},
		{
			name:        "Non-existent batch resolver",
			objectType:  "User",
			field:       "nonExistent",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := reg.GetBatchResolverDescriptor(tt.objectType, tt.field)

			if tt.shouldExist {
				require.NotNil(t, md, "Method descriptor should exist for %s.%s", tt.objectType, tt.field)
				assert.Equal(t, tt.methodName, string(md.Name()), "Method name should match")
			} else {
				assert.Nil(t, md, "Method descriptor should not exist for %s.%s", tt.objectType, tt.field)
			}
		})
	}
}

func TestGetSingleLoaderDescriptor(t *testing.T) {
	reg := buildTestRegistry(t)

	tests := []struct {
		name        string
		objectType  string
		field       string
		shouldExist bool
		methodName  string // expected method name pattern
	}{
		{
			name:        "Post author single loader",
			objectType:  "Post",
			field:       "author",
			shouldExist: false, // Loaders are batch by default
		},
		{
			name:        "Non-existent single loader",
			objectType:  "User",
			field:       "nonExistent",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := reg.GetSingleLoaderDescriptor(tt.objectType, tt.field)

			if tt.shouldExist {
				require.NotNil(t, md, "Method descriptor should exist for %s.%s", tt.objectType, tt.field)
				assert.Equal(t, tt.methodName, string(md.Name()), "Method name should match")
			} else {
				assert.Nil(t, md, "Method descriptor should not exist for %s.%s", tt.objectType, tt.field)
			}
		})
	}
}

func TestGetBatchLoaderDescriptor(t *testing.T) {
	reg := buildTestRegistry(t)

	tests := []struct {
		name        string
		objectType  string
		field       string
		shouldExist bool
		methodName  string // expected method name pattern
	}{
		{
			name:        "Post author batch loader",
			objectType:  "Post",
			field:       "author",
			shouldExist: true,
			methodName:  "BatchLoadUserById",
		},
		{
			name:        "Non-existent batch loader",
			objectType:  "User",
			field:       "nonExistent",
			shouldExist: false,
		},
		{
			name:        "Post likeCount (resolver not loader)",
			objectType:  "Post",
			field:       "likeCount",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := reg.GetBatchLoaderDescriptor(tt.objectType, tt.field)

			if tt.shouldExist {
				require.NotNil(t, md, "Method descriptor should exist for %s.%s", tt.objectType, tt.field)
				assert.Equal(t, tt.methodName, string(md.Name()), "Method name should match")
			} else {
				assert.Nil(t, md, "Method descriptor should not exist for %s.%s", tt.objectType, tt.field)
			}
		})
	}
}

func TestRegistryInterfaceCompliance(t *testing.T) {
	// Ensure that the implementation returned by Build implements the Registry interface
	discovery, err := ir.NewFileSystemDiscovery(context.Background(), path.Join("testdata", "schema"), "testdata.proto")
	require.NoError(t, err)

	proj, err := ir.Build(context.Background(), discovery)
	require.NoError(t, err)

	reg, err := protoreg.Build(proj)
	require.NoError(t, err)

	// This will fail to compile if reg doesn't implement grpcrt.Registry
	var _ grpcrt.Registry = reg

	// Verify the registry is not nil
	assert.NotNil(t, reg, "Registry should not be nil")
}

func TestRegistryWithEmptyProject(t *testing.T) {
	// Test with minimal/empty project
	proj := &ir.Project{
		Services:    make(map[ir.ServiceID]*ir.Service),
		Definitions: make(map[string]*ir.Definition),
	}

	reg, err := protoreg.Build(proj)
	require.NoError(t, err)
	require.NotNil(t, reg)

	// Should return empty results but not panic
	files := reg.GetAllServiceFiles()
	assert.Empty(t, files, "Empty project should have no service files")

	fd := reg.GetSourceFieldDescriptor("AnyType", "anyField")
	assert.Nil(t, fd, "Empty project should return nil for any field descriptor")

	md := reg.GetSingleResolverDescriptor("AnyType", "anyField")
	assert.Nil(t, md, "Empty project should return nil for any resolver descriptor")
}

func TestRegistryMethodsReturnNilForInvalidInput(t *testing.T) {
	reg := buildTestRegistry(t)

	// Test with empty strings
	assert.Nil(t, reg.GetSourceFieldDescriptor("", ""))
	assert.Nil(t, reg.GetSingleResolverDescriptor("", ""))
	assert.Nil(t, reg.GetBatchResolverDescriptor("", ""))
	assert.Nil(t, reg.GetSingleLoaderDescriptor("", ""))
	assert.Nil(t, reg.GetBatchLoaderDescriptor("", ""))

	// Test with only one parameter empty
	assert.Nil(t, reg.GetSourceFieldDescriptor("User", ""))
	assert.Nil(t, reg.GetSourceFieldDescriptor("", "name"))

	// Test with special characters
	assert.Nil(t, reg.GetSourceFieldDescriptor("User!", "@field"))
	assert.Nil(t, reg.GetSingleResolverDescriptor("Type$", "field#"))
}

func TestGetRequestFieldSourceMapping(t *testing.T) {
	reg := buildTestRegistry(t)
	// Loader mapping from schema: Post.author @load(with: { id: "authorId" })
	mp := reg.GetRequestFieldSourceMapping("Post", "author")
	require.NotNil(t, mp, "loader mapping should be present for Post.author")
	require.Equal(t, "authorId", mp["id"])
	// Resolver without mapping should return nil or empty
	mp2 := reg.GetRequestFieldSourceMapping("Query", "getUser")
	// getUser has explicit arg id and no parent mapping; accept nil or empty
	if mp2 != nil {
		require.Len(t, mp2, 0)
	}
}
