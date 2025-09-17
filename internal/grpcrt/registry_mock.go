package grpcrt

import (
	"google.golang.org/protobuf/reflect/protoreflect"
)

// MockRegistry is a test helper that allows tests to register
// field/method descriptors per (objectType, field) key and returns
// them via the grpcrt.Registry interface.
type MockRegistry struct {
	sourceFields    map[[2]string]protoreflect.FieldDescriptor
	singleResolvers map[[2]string]protoreflect.MethodDescriptor
	batchResolvers  map[[2]string]protoreflect.MethodDescriptor
	singleLoaders   map[[2]string]protoreflect.MethodDescriptor
	batchLoaders    map[[2]string]protoreflect.MethodDescriptor
	requestMap      map[[2]string]map[string]string
	sourceMessages  map[string]protoreflect.MessageDescriptor
}

// NewMockRegistry creates an empty MockRegistry.
func NewMockRegistry() *MockRegistry {
	return &MockRegistry{
		sourceFields:    map[[2]string]protoreflect.FieldDescriptor{},
		singleResolvers: map[[2]string]protoreflect.MethodDescriptor{},
		batchResolvers:  map[[2]string]protoreflect.MethodDescriptor{},
		singleLoaders:   map[[2]string]protoreflect.MethodDescriptor{},
		batchLoaders:    map[[2]string]protoreflect.MethodDescriptor{},
		requestMap:      map[[2]string]map[string]string{},
		sourceMessages:  map[string]protoreflect.MessageDescriptor{},
	}
}

// RegisterSourceField maps (objectType, graphqlField) to a field descriptor.
func (m *MockRegistry) RegisterSourceField(objectType, graphqlField string, fd protoreflect.FieldDescriptor) *MockRegistry {
	m.sourceFields[[2]string{objectType, graphqlField}] = fd
	return m
}

// RegisterSingleResolver maps (objectType, field) to a single resolver method.
func (m *MockRegistry) RegisterSingleResolver(objectType, field string, md protoreflect.MethodDescriptor) *MockRegistry {
	m.singleResolvers[[2]string{objectType, field}] = md
	return m
}

// RegisterBatchResolver maps (objectType, field) to a batch resolver method.
func (m *MockRegistry) RegisterBatchResolver(objectType, field string, md protoreflect.MethodDescriptor) *MockRegistry {
	m.batchResolvers[[2]string{objectType, field}] = md
	return m
}

// RegisterSingleLoader maps (objectType, field) to a single loader method.
func (m *MockRegistry) RegisterSingleLoader(objectType, field string, md protoreflect.MethodDescriptor) *MockRegistry {
	m.singleLoaders[[2]string{objectType, field}] = md
	return m
}

// RegisterBatchLoader maps (objectType, field) to a batch loader method.
func (m *MockRegistry) RegisterBatchLoader(objectType, field string, md protoreflect.MethodDescriptor) *MockRegistry {
	m.batchLoaders[[2]string{objectType, field}] = md
	return m
}

// RegisterSourceMessage maps a GraphQL object type to its proto message descriptor.
func (m *MockRegistry) RegisterSourceMessage(objectType string, md protoreflect.MessageDescriptor) *MockRegistry {
	m.sourceMessages[objectType] = md
	return m
}

// RegisterRequestSourceMap maps (objectType, field) to a request field -> parent source field mapping.
// Example: { "authorId": "id" } to copy parent.id into request.authorId when not provided via args.
func (m *MockRegistry) RegisterRequestSourceMap(objectType, field string, mp map[string]string) *MockRegistry {
	m.requestMap[[2]string{objectType, field}] = mp
	return m
}

// ---- grpcrt.Registry implementation ----

func (m *MockRegistry) GetSourceFieldDescriptor(objectType, graphqlField string) protoreflect.FieldDescriptor {
	return m.sourceFields[[2]string{objectType, graphqlField}]
}

func (m *MockRegistry) GetSingleResolverDescriptor(objectType, field string) protoreflect.MethodDescriptor {
	return m.singleResolvers[[2]string{objectType, field}]
}

func (m *MockRegistry) GetBatchResolverDescriptor(objectType, field string) protoreflect.MethodDescriptor {
	return m.batchResolvers[[2]string{objectType, field}]
}

func (m *MockRegistry) GetSingleLoaderDescriptor(objectType, field string) protoreflect.MethodDescriptor {
	return m.singleLoaders[[2]string{objectType, field}]
}

func (m *MockRegistry) GetBatchLoaderDescriptor(objectType, field string) protoreflect.MethodDescriptor {
	return m.batchLoaders[[2]string{objectType, field}]
}

func (m *MockRegistry) GetRequestFieldSourceMapping(objectType, field string) map[string]string {
	return m.requestMap[[2]string{objectType, field}]
}

func (m *MockRegistry) GetSourceMessageDescriptor(objectType string) protoreflect.MessageDescriptor {
	return m.sourceMessages[objectType]
}

var _ Registry = (*MockRegistry)(nil)
