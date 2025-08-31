package grpcrt

import (
    "context"
    "testing"

    "github.com/stretchr/testify/require"
    "google.golang.org/protobuf/reflect/protodesc"
    "google.golang.org/protobuf/reflect/protoreflect"
    "google.golang.org/protobuf/types/descriptorpb"
    "google.golang.org/protobuf/types/dynamicpb"

    executor "github.com/hanpama/protograph/internal/executor"
    "errors"
)

// helpers to build simple single/batch methods with recognizable names
func buildMethod(t *testing.T, svc, name string, batch bool) protoreflect.MethodDescriptor {
    t.Helper()
    var in, out *descriptorpb.DescriptorProto
    if batch {
        in = &descriptorpb.DescriptorProto{
            Name: protoString(name + "Request"),
            Field: []*descriptorpb.FieldDescriptorProto{{
                Name:     protoString("batches"),
                JsonName: protoString("batches"),
                Number:   protoInt32(1),
                Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
                Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
                TypeName: protoString(".q.Item"),
            }},
        }
        out = &descriptorpb.DescriptorProto{
            Name: protoString(name + "Response"),
            Field: []*descriptorpb.FieldDescriptorProto{{
                Name:     protoString("batches"),
                JsonName: protoString("batches"),
                Number:   protoInt32(1),
                Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
                Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
                TypeName: protoString(".q.Out"),
            }},
        }
    } else {
        in = &descriptorpb.DescriptorProto{Name: protoString(name + "Request")}
        out = &descriptorpb.DescriptorProto{
            Name: protoString(name + "Response"),
            Field: []*descriptorpb.FieldDescriptorProto{{
                Name:     protoString("data"),
                JsonName: protoString("data"),
                Number:   protoInt32(1),
                Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
            }},
        }
    }
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString(name + ".proto"),
        Package: protoString("q"),
        MessageType: []*descriptorpb.DescriptorProto{
            {Name: protoString("Item")}, {Name: protoString("Out")}, in, out,
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{
            Name: protoString(svc),
            Method: []*descriptorpb.MethodDescriptorProto{{
                Name:       protoString(name),
                InputType:  protoString(".q." + in.GetName()),
                OutputType: protoString(".q." + out.GetName()),
            }},
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath(name + ".proto")
    require.NoError(t, err)
    return fd.Services().ByName(protoreflect.Name(svc)).Methods().ByName(protoreflect.Name(name))
}

// docs §2.1
func Test_2_1_RegistrySelection_PriorityBatchResolver(t *testing.T) {
    // Register all four kinds; expect batch resolver chosen
    // Build distinct methods so we can identify by method name
    bres := buildMethod(t, "S", "BatchResolve", true)
    sres := buildMethod(t, "S", "Resolve", false)
    bldr := buildMethod(t, "S", "BatchLoad", true)
    sldr := buildMethod(t, "S", "Load", false)

    reg := NewMockRegistry().
        RegisterBatchResolver("Obj", "f", bres).
        RegisterSingleResolver("Obj", "f", sres).
        RegisterBatchLoader("Obj", "f", bldr).
        RegisterSingleLoader("Obj", "f", sldr)

    // Prepare a batch response with one item
    out := dynamicpb.NewMessage(bres.Output())
    bf := bres.Output().Fields().ByName("batches")
    l := out.Mutable(bf).List()
    l.Append(protoreflect.ValueOfMessage(dynamicpb.NewMessage(bf.Message())))
    out.Set(bf, protoreflect.ValueOfList(l))
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)

    _ = rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "f", Args: map[string]any{}}})
    calls := mt.Calls()
    require.Equal(t, 1, len(calls))
    require.Contains(t, calls[0].FullMethod, "/q.S/BatchResolve")
}

// docs §2.1
func Test_2_1_RegistrySelection_FallbackSingleResolver(t *testing.T) {
    sres := buildMethod(t, "S", "Resolve", false)
    reg := NewMockRegistry().RegisterSingleResolver("Obj", "f", sres)
    out := dynamicpb.NewMessage(sres.Output())
    out.Set(sres.Output().Fields().ByName("data"), protoreflect.ValueOfString("ok"))
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)
    _ = rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "f"}})
    calls := mt.Calls()
    require.Equal(t, 1, len(calls))
    require.Contains(t, calls[0].FullMethod, "/q.S/Resolve")
}

// docs §2.1
func Test_2_1_RegistrySelection_FallbackBatchLoader(t *testing.T) {
    bldr := buildMethod(t, "S", "BatchLoad", true)
    // Build response with empty batches
    out := dynamicpb.NewMessage(bldr.Output())
    out.Set(bldr.Output().Fields().ByName("batches"), protoreflect.ValueOfList(out.Mutable(bldr.Output().Fields().ByName("batches")).List()))
    reg := NewMockRegistry().RegisterBatchLoader("Obj", "f", bldr)
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)
    _ = rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "f"}})
    calls := mt.Calls()
    require.Equal(t, 1, len(calls))
    require.Contains(t, calls[0].FullMethod, "/q.S/BatchLoad")
}

// docs §2.1
func Test_2_1_RegistrySelection_FallbackSingleLoader(t *testing.T) {
    sldr := buildMethod(t, "S", "Load", false)
    out := dynamicpb.NewMessage(sldr.Output())
    out.Set(sldr.Output().Fields().ByName("data"), protoreflect.ValueOfString("ok"))
    reg := NewMockRegistry().RegisterSingleLoader("Obj", "f", sldr)
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)
    _ = rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "f"}})
    calls := mt.Calls()
    require.Equal(t, 1, len(calls))
    require.Contains(t, calls[0].FullMethod, "/q.S/Load")
}

// docs §2.2
func Test_2_2_Grouping_ByObjectAndField_BatchedPerGroup(t *testing.T) {
    // Two fields -> two groups -> two calls
    b1 := buildMethod(t, "S", "B1", true)
    b2 := buildMethod(t, "S", "B2", true)
    reg := NewMockRegistry().RegisterBatchResolver("Obj", "f1", b1).RegisterBatchResolver("Obj", "f2", b2)
    // Return errors to avoid needing response shape ordering across parallel calls
    mt := NewMockTransportWithErrors(nil, []error{errors.New("e1"), errors.New("e2")})
    rt := NewRuntime(reg, mt)
    _ = rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{
        {ObjectType: "Obj", Field: "f1"},
        {ObjectType: "Obj", Field: "f2"},
    })
    calls := mt.Calls()
    require.Equal(t, 2, len(calls))
    // order between groups is not guaranteed; assert both methods present
    methods := []string{calls[0].FullMethod, calls[1].FullMethod}
    require.Contains(t, methods[0]+methods[1], "/q.S/B1")
    require.Contains(t, methods[0]+methods[1], "/q.S/B2")
}
