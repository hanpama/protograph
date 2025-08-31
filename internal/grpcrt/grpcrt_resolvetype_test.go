package grpcrt

import (
    "context"
    "testing"

    "github.com/stretchr/testify/require"
    "google.golang.org/protobuf/reflect/protodesc"
    "google.golang.org/protobuf/types/descriptorpb"
    "google.golang.org/protobuf/types/dynamicpb"
)

// docs §8.1
func Test_8_1_ResolveType_StripsSourceSuffix(t *testing.T) {
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("rt.proto"),
        Package: protoString("rsvc"),
        MessageType: []*descriptorpb.DescriptorProto{{
            Name: protoString("UserSource"),
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("rt.proto")
    require.NoError(t, err)
    md := fd.Messages().ByName("UserSource")
    msg := dynamicpb.NewMessage(md)

    rt := NewRuntime(nil, nil)
    typ, err := rt.ResolveType(context.Background(), "Any", msg)
    require.NoError(t, err)
    require.Equal(t, "User", typ)
}

// docs §8.2
func Test_8_2_ResolveType_NoSourceSuffix_Error(t *testing.T) {
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("rt2.proto"),
        Package: protoString("rsvc"),
        MessageType: []*descriptorpb.DescriptorProto{{
            Name: protoString("Unknown"),
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("rt2.proto")
    require.NoError(t, err)
    md := fd.Messages().ByName("Unknown")
    msg := dynamicpb.NewMessage(md)

    rt := NewRuntime(nil, nil)
    _, err = rt.ResolveType(context.Background(), "Any", msg)
    require.Error(t, err)
}

// docs §8.2
func Test_8_2_ResolveType_ValueNotMessage_Error(t *testing.T) {
    rt := NewRuntime(nil, nil)
    _, err := rt.ResolveType(context.Background(), "Any", 123)
    require.Error(t, err)
}
