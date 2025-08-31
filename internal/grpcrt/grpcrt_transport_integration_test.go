package grpcrt

import (
    "context"
    "errors"
    "testing"

    "github.com/stretchr/testify/require"
    "google.golang.org/protobuf/reflect/protodesc"
    "google.golang.org/protobuf/reflect/protoreflect"
    "google.golang.org/protobuf/types/descriptorpb"
    "google.golang.org/protobuf/types/dynamicpb"

    executor "github.com/hanpama/protograph/internal/executor"
)

// Build a proto file with a batch resolver method with the required shape.
// package: tsvc
// messages:
//   BatchItem { string arg1 = 1; }
//   BatchItemOut { string data = 1; }
//   BatchResolveUserFriendsRequest { repeated BatchItem batches = 1; }
//   BatchResolveUserFriendsResponse { repeated BatchItemOut batches = 1; }
// service TestService { rpc BatchResolveUserFriends(...) returns (...); }
func buildBatchResolverDescriptors(t *testing.T) protoreflect.MethodDescriptor {
    t.Helper()
    pkg := "tsvc"
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("tsvc.proto"),
        Package: protoString(pkg),
        MessageType: []*descriptorpb.DescriptorProto{
            { // BatchItem
                Name: protoString("BatchItem"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("arg1"),
                    JsonName: protoString("arg1"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
                }},
            },
            { // BatchItemOut
                Name: protoString("BatchItemOut"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("data"),
                    JsonName: protoString("data"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
                }},
            },
            { // Request
                Name: protoString("BatchResolveUserFriendsRequest"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("batches"),
                    JsonName: protoString("batches"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
                    TypeName: protoString(".tsvc.BatchItem"),
                }},
            },
            { // Response
                Name: protoString("BatchResolveUserFriendsResponse"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("batches"),
                    JsonName: protoString("batches"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
                    TypeName: protoString(".tsvc.BatchItemOut"),
                }},
            },
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{
            Name: protoString("TestService"),
            Method: []*descriptorpb.MethodDescriptorProto{{
                Name:       protoString("BatchResolveUserFriends"),
                InputType:  protoString(".tsvc.BatchResolveUserFriendsRequest"),
                OutputType: protoString(".tsvc.BatchResolveUserFriendsResponse"),
            }},
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("tsvc.proto")
    require.NoError(t, err)
    svc := fd.Services().ByName("TestService")
    require.NotNil(t, svc)
    m := svc.Methods().ByName("BatchResolveUserFriends")
    require.NotNil(t, m)
    return m
}

// docs §5.1
func Test_5_1_Transport_CalledOncePerBatchGroup(t *testing.T) {
    md := buildBatchResolverDescriptors(t)

    // Prepare response: two batches with data "D1", "D2"
    out := dynamicpb.NewMessage(md.Output())
    batchesField := md.Output().Fields().ByName("batches")
    itemDesc := batchesField.Message()
    lst := out.Mutable(batchesField).List()
    item1 := dynamicpb.NewMessage(itemDesc)
    item1.Set(itemDesc.Fields().ByName("data"), protoreflect.ValueOfString("D1"))
    lst.Append(protoreflect.ValueOfMessage(item1))
    item2 := dynamicpb.NewMessage(itemDesc)
    item2.Set(itemDesc.Fields().ByName("data"), protoreflect.ValueOfString("D2"))
    lst.Append(protoreflect.ValueOfMessage(item2))
    out.Set(batchesField, protoreflect.ValueOfList(lst))

    // Registry -> return batch resolver for (User, friends)
    reg := NewMockRegistry().RegisterBatchResolver("User", "friends", md)
    // Transport -> return our response, capture request
    mt := NewMockTransport(out)

    rt := NewRuntime(reg, mt)
    tasks := []executor.AsyncResolveTask{
        {ObjectType: "User", Field: "friends", Args: map[string]any{"arg1": "x"}},
        {ObjectType: "User", Field: "friends", Args: map[string]any{"arg1": "y"}},
    }
    results := rt.BatchResolveAsync(context.Background(), tasks)

    // Expect single transport call for the group
    calls := mt.Calls()
    require.Equal(t, 1, len(calls))
    // Validate request had two batch items with arg1 values x, y
    req := calls[0].Request.ProtoReflect()
    rf := md.Input().Fields().ByName("batches")
    rlist := req.Get(rf).List()
    require.Equal(t, 2, rlist.Len())
    argField := rf.Message().Fields().ByName("arg1")
    require.Equal(t, "x", rlist.Get(0).Message().Get(argField).String())
    require.Equal(t, "y", rlist.Get(1).Message().Get(argField).String())

    // Validate results ordering and values
    require.Equal(t, 2, len(results))
    require.NoError(t, results[0].Error)
    require.NoError(t, results[1].Error)
    require.Equal(t, "D1", results[0].Value)
    require.Equal(t, "D2", results[1].Value)
}

// docs §5.1
func Test_5_1_Transport_CalledOncePerSingleTask(t *testing.T) {
    // Build single resolver: ResolveUserName(Request{ arg1 }) -> Response{ data }
    pkg := "tsvc"
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("tsvc_single.proto"),
        Package: protoString(pkg),
        MessageType: []*descriptorpb.DescriptorProto{
            { // Request
                Name: protoString("ResolveUserNameRequest"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("arg1"),
                    JsonName: protoString("arg1"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
                }},
            },
            { // Response
                Name: protoString("ResolveUserNameResponse"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("data"),
                    JsonName: protoString("data"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
                }},
            },
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{
            Name: protoString("TestService"),
            Method: []*descriptorpb.MethodDescriptorProto{{
                Name:       protoString("ResolveUserName"),
                InputType:  protoString(".tsvc.ResolveUserNameRequest"),
                OutputType: protoString(".tsvc.ResolveUserNameResponse"),
            }},
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("tsvc_single.proto")
    require.NoError(t, err)
    svc := fd.Services().ByName("TestService")
    require.NotNil(t, svc)
    md := svc.Methods().ByName("ResolveUserName")
    require.NotNil(t, md)

    // Prepare two distinct responses
    out1 := dynamicpb.NewMessage(md.Output())
    out1.Set(md.Output().Fields().ByName("data"), protoreflect.ValueOfString("A"))
    out2 := dynamicpb.NewMessage(md.Output())
    out2.Set(md.Output().Fields().ByName("data"), protoreflect.ValueOfString("B"))

    reg := NewMockRegistry().RegisterSingleResolver("User", "name", md)
    mt := NewMockTransport(out1, out2)
    rt := NewRuntime(reg, mt)

    tasks := []executor.AsyncResolveTask{
        {ObjectType: "User", Field: "name", Args: map[string]any{"arg1": "x"}},
        {ObjectType: "User", Field: "name", Args: map[string]any{"arg1": "y"}},
    }
    results := rt.BatchResolveAsync(context.Background(), tasks)

    // Expect two calls (single per task)
    calls := mt.Calls()
    require.Equal(t, 2, len(calls))

    // Validate first request contains arg1=x, second contains arg1=y
    rf := md.Input().Fields().ByName("arg1")
    req1 := calls[0].Request.ProtoReflect()
    require.Equal(t, "x", req1.Get(rf).String())
    req2 := calls[1].Request.ProtoReflect()
    require.Equal(t, "y", req2.Get(rf).String())

    // Validate results in order
    require.Equal(t, 2, len(results))
    require.NoError(t, results[0].Error)
    require.NoError(t, results[1].Error)
    require.Equal(t, "A", results[0].Value)
    require.Equal(t, "B", results[1].Value)
}

// docs §5.3
func Test_5_3_Transport_ErrorPropagatesToGroupOrElement(t *testing.T) {
    // Batch resolver error -> all elements error
    bmd := buildBatchResolverDescriptors(t)
    regBatch := NewMockRegistry().RegisterBatchResolver("User", "friends", bmd)
    // Single transport call will return an error
    mtBatch := NewMockTransportWithErrors(nil, []error{errors.New("boom")})
    rtBatch := NewRuntime(regBatch, mtBatch)
    tasksBatch := []executor.AsyncResolveTask{
        {ObjectType: "User", Field: "friends", Args: map[string]any{"arg1": "x"}},
        {ObjectType: "User", Field: "friends", Args: map[string]any{"arg1": "y"}},
    }
    resBatch := rtBatch.BatchResolveAsync(context.Background(), tasksBatch)
    require.Equal(t, 2, len(resBatch))
    require.Error(t, resBatch[0].Error)
    require.Error(t, resBatch[1].Error)
    require.Nil(t, resBatch[0].Value)
    require.Nil(t, resBatch[1].Value)

    // Single resolver: first call fails, second succeeds
    // Build descriptors for single resolver
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("tsvc_err_single.proto"),
        Package: protoString("tsvc"),
        MessageType: []*descriptorpb.DescriptorProto{
            { // Request
                Name: protoString("ResolveUserNameRequest"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("arg1"),
                    JsonName: protoString("arg1"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
                }},
            },
            { // Response
                Name: protoString("ResolveUserNameResponse"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("data"),
                    JsonName: protoString("data"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
                }},
            },
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{
            Name: protoString("TestService"),
            Method: []*descriptorpb.MethodDescriptorProto{{
                Name:       protoString("ResolveUserName"),
                InputType:  protoString(".tsvc.ResolveUserNameRequest"),
                OutputType: protoString(".tsvc.ResolveUserNameResponse"),
            }},
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("tsvc_err_single.proto")
    require.NoError(t, err)
    svc := fd.Services().ByName("TestService")
    require.NotNil(t, svc)
    smd := svc.Methods().ByName("ResolveUserName")
    require.NotNil(t, smd)

    out := dynamicpb.NewMessage(smd.Output())
    out.Set(smd.Output().Fields().ByName("data"), protoreflect.ValueOfString("OK"))
    regSingle := NewMockRegistry().RegisterSingleResolver("User", "name", smd)
    mtSingle := NewMockTransportWithErrors([]protoreflect.Message{nil, out}, []error{errors.New("oops"), nil})
    rtSingle := NewRuntime(regSingle, mtSingle)
    tasksSingle := []executor.AsyncResolveTask{
        {ObjectType: "User", Field: "name", Args: map[string]any{"arg1": "x"}},
        {ObjectType: "User", Field: "name", Args: map[string]any{"arg1": "y"}},
    }
    resSingle := rtSingle.BatchResolveAsync(context.Background(), tasksSingle)
    require.Equal(t, 2, len(resSingle))
    require.Error(t, resSingle[0].Error)
    require.NoError(t, resSingle[1].Error)
    require.Equal(t, "OK", resSingle[1].Value)
}
