package protoreg

import (
	"github.com/hanpama/protograph/internal/grpcrt"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Registry implements grpcrt.Registry
type Registry struct {
	fileDescriptors           []protoreflect.FileDescriptor
	sourceFieldDescriptors    map[[2]string]protoreflect.FieldDescriptor
	singleResolverDescriptors map[[2]string]protoreflect.MethodDescriptor
	batchResolverDescriptors  map[[2]string]protoreflect.MethodDescriptor
	singleLoaderDescriptors   map[[2]string]protoreflect.MethodDescriptor
	batchLoaderDescriptors    map[[2]string]protoreflect.MethodDescriptor
	// requestFieldSourceMap optionally maps (objectType, field) -> request field name -> parent source field name
	requestFieldSourceMap map[[2]string]map[string]string
}

// GetAllServiceFiles implements grpcrt.Registry.
func (r *Registry) GetAllServiceFiles() []protoreflect.FileDescriptor {
	return r.fileDescriptors
}

// GetBatchLoaderDescriptor implements grpcrt.Registry.
func (r *Registry) GetBatchLoaderDescriptor(objectType string, field string) protoreflect.MethodDescriptor {
	return r.batchLoaderDescriptors[[2]string{objectType, field}]
}

// GetBatchResolverDescriptor implements grpcrt.Registry.
func (r *Registry) GetBatchResolverDescriptor(objectType string, field string) protoreflect.MethodDescriptor {
	return r.batchResolverDescriptors[[2]string{objectType, field}]
}

// GetSingleLoaderDescriptor implements grpcrt.Registry.
func (r *Registry) GetSingleLoaderDescriptor(objectType string, field string) protoreflect.MethodDescriptor {
	return r.singleLoaderDescriptors[[2]string{objectType, field}]
}

// GetSingleResolverDescriptor implements grpcrt.Registry.
func (r *Registry) GetSingleResolverDescriptor(objectType string, field string) protoreflect.MethodDescriptor {
	return r.singleResolverDescriptors[[2]string{objectType, field}]
}

// GetSourceFieldDescriptor implements grpcrt.Registry.
func (r *Registry) GetSourceFieldDescriptor(objectType string, graphqlField string) protoreflect.FieldDescriptor {
	return r.sourceFieldDescriptors[[2]string{objectType, graphqlField}]
}

// GetRequestFieldSourceMapping implements grpcrt.Registry.
// For now, return nil unless the builder populates this in the future.
func (r *Registry) GetRequestFieldSourceMapping(objectType, field string) map[string]string {
	if r.requestFieldSourceMap == nil {
		return nil
	}
	return r.requestFieldSourceMap[[2]string{objectType, field}]
}

var _ grpcrt.Registry = (*Registry)(nil)
