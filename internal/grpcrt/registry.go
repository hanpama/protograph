package grpcrt

import "google.golang.org/protobuf/reflect/protoreflect"

type Registry interface {
	// GetSourceFieldDescriptor returns the proto field descriptor for a GraphQL field in the given object type
	// Returns nil if not found
	GetSourceFieldDescriptor(objectType, graphqlField string) protoreflect.FieldDescriptor

	// Resolver methods
	// GetSingleResolverDescriptor returns the method descriptor for a single resolver field
	GetSingleResolverDescriptor(objectType, field string) protoreflect.MethodDescriptor
	// GetBatchResolverDescriptor returns the method descriptor for a batch resolver field
	GetBatchResolverDescriptor(objectType, field string) protoreflect.MethodDescriptor

    // Loader methods
    // GetSingleLoaderDescriptor returns the method descriptor for a single loader field
    GetSingleLoaderDescriptor(objectType, field string) protoreflect.MethodDescriptor
    // GetBatchLoaderDescriptor returns the method descriptor for a batch loader field
    GetBatchLoaderDescriptor(objectType, field string) protoreflect.MethodDescriptor

    // GetRequestFieldSourceMapping returns a mapping for a resolver/loader input field name
    // (destination) to the parent source GraphQL field name (source). This is used to populate
    // request fields from the parent object (e.g., explicit @resolve(with: { authorId: "id" })).
    // When nil, no additional mapping is applied beyond provided args.
    GetRequestFieldSourceMapping(objectType, field string) map[string]string
}
