package grpcrt

import (
    "context"
    "testing"

    executor "github.com/hanpama/protograph/internal/executor"
    "google.golang.org/protobuf/reflect/protodesc"
    "google.golang.org/protobuf/reflect/protoreflect"
    "google.golang.org/protobuf/types/descriptorpb"
    "google.golang.org/protobuf/types/dynamicpb"
)

func TestBoundary_NoRegisteredMethod_Panics(t *testing.T) {
    reg := NewMockRegistry() // no methods registered
    rt := NewRuntime(reg, NewMockTransport())
    defer func() {
        if r := recover(); r == nil {
            t.Fatalf("expected panic when no resolver/loader is registered for a group")
        }
    }()
    _ = rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "f"}})
}

func TestBoundary_ResolveSync_NoFieldDescriptor_Panics(t *testing.T) {
    reg := NewMockRegistry() // no source field registered
    rt := NewRuntime(reg, nil)
    // Build a dummy message to pass as source
    md := buildSimpleMessage(t, "UserSource", "title")
    msg := dynamicpb.NewMessage(md)
    defer func() {
        if r := recover(); r == nil {
            t.Fatalf("expected panic when FieldDescriptor is missing for a physical field")
        }
    }()
    _, _ = rt.ResolveSync(context.Background(), "User", "title", msg, nil)
}

// buildSimpleMessage builds a proto3 message with one string field by jsonName
func buildSimpleMessage(t *testing.T, name, field string) protoreflect.MessageDescriptor {
    t.Helper()
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("bnd.proto"),
        Package: protoString("bnd"),
        MessageType: []*descriptorpb.DescriptorProto{{
            Name: protoString(name),
            Field: []*descriptorpb.FieldDescriptorProto{{
                Name:     protoString(field),
                JsonName: protoString(field),
                Number:   protoInt32(1),
                Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
                Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
            }},
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    if err != nil {
        t.Fatalf("protodesc.NewFiles: %v", err)
    }
    fd, err := files.FindFileByPath("bnd.proto")
    if err != nil {
        t.Fatalf("FindFileByPath: %v", err)
    }
    return fd.Messages().ByName(protoreflect.Name(name))
}
