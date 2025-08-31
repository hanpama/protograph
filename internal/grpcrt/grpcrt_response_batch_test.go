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
)

// helper to build a batch resolver with output batches of items with data:string
func buildBatchForResponseTests(t *testing.T) protoreflect.MethodDescriptor {
    t.Helper()
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("resp_batch.proto"),
        Package: protoString("rsvc"),
        MessageType: []*descriptorpb.DescriptorProto{
            { // ItemOut
                Name: protoString("ItemOut"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("data"),
                    JsonName: protoString("data"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
                }},
            },
            { // Request
                Name: protoString("BatchReq"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("batches"),
                    JsonName: protoString("batches"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
                    TypeName: protoString(".rsvc.ItemOut"), // reuse shape for simplicity
                }},
            },
            { // Response OK shape
                Name: protoString("BatchResp"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("batches"),
                    JsonName: protoString("batches"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
                    TypeName: protoString(".rsvc.ItemOut"),
                }},
            },
            { // Response MISSING batches
                Name: protoString("BatchRespNoBatches"),
                Field: []*descriptorpb.FieldDescriptorProto{},
            },
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{
            Name: protoString("RespService"),
            Method: []*descriptorpb.MethodDescriptorProto{{
                Name:       protoString("BatchMethod"),
                InputType:  protoString(".rsvc.BatchReq"),
                OutputType: protoString(".rsvc.BatchResp"),
            }},
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("resp_batch.proto")
    require.NoError(t, err)
    m := fd.Services().ByName("RespService").Methods().ByName("BatchMethod")
    require.NotNil(t, m)
    return m
}

// docs §7.2
func Test_7_2_ResponseBatch_MissingElements_ErrorForMissing(t *testing.T) {
    md := buildBatchForResponseTests(t)
    // Prepare response with only one element, but we'll request two
    out := dynamicpb.NewMessage(md.Output())
    of := md.Output().Fields().ByName("batches")
    itemDesc := of.Message()
    list := out.Mutable(of).List()
    item := dynamicpb.NewMessage(itemDesc)
    item.Set(itemDesc.Fields().ByName("data"), protoreflect.ValueOfString("first"))
    list.Append(protoreflect.ValueOfMessage(item))
    out.Set(of, protoreflect.ValueOfList(list))

    reg := NewMockRegistry().RegisterBatchResolver("User", "friends", md)
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)
    tasks := []executor.AsyncResolveTask{
        {ObjectType: "User", Field: "friends", Args: map[string]any{}},
        {ObjectType: "User", Field: "friends", Args: map[string]any{}},
    }
    res := rt.BatchResolveAsync(context.Background(), tasks)
    require.Equal(t, 2, len(res))
    require.NoError(t, res[0].Error)
    require.Equal(t, "first", res[0].Value)
    require.Error(t, res[1].Error)
}

// docs §7.3
func Test_7_3_ResponseBatch_MissingBatchesField_GroupError(t *testing.T) {
    // Build two methods: normal and one with output lacking 'batches'
    // We'll reuse builder and then mutate output type by constructing a new descriptor set.
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("resp_nobatches.proto"),
        Package: protoString("rsvc"),
        MessageType: []*descriptorpb.DescriptorProto{
            { // ItemOut
                Name: protoString("ItemOut"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("data"),
                    JsonName: protoString("data"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
                }},
            },
            { // Request
                Name: protoString("BatchReq"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("batches"),
                    JsonName: protoString("batches"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
                    TypeName: protoString(".rsvc.ItemOut"),
                }},
            },
            { // Response without batches
                Name: protoString("BatchRespNoBatches"),
                Field: []*descriptorpb.FieldDescriptorProto{},
            },
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{
            Name: protoString("RespService"),
            Method: []*descriptorpb.MethodDescriptorProto{{
                Name:       protoString("BatchMethod"),
                InputType:  protoString(".rsvc.BatchReq"),
                OutputType: protoString(".rsvc.BatchRespNoBatches"),
            }},
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("resp_nobatches.proto")
    require.NoError(t, err)
    md := fd.Services().ByName("RespService").Methods().ByName("BatchMethod")
    require.NotNil(t, md)

    // Response message without batches
    out := dynamicpb.NewMessage(md.Output())
    reg := NewMockRegistry().RegisterBatchResolver("User", "friends", md)
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)
    tasks := []executor.AsyncResolveTask{{ObjectType: "User", Field: "friends"}}
    res := rt.BatchResolveAsync(context.Background(), tasks)
    require.Equal(t, 1, len(res))
    require.Error(t, res[0].Error)
}

// docs §7.1
func Test_7_1_ResponseBatch_BatchesLengthMatchesExpected_IndexMapping(t *testing.T) {
    // Covered by TestTransport_CalledOncePerBatchGroup; keep as placeholder if more mapping cases needed.
    t.Skip("covered in integration test")
}

// docs §7.4
func Test_7_4_ResponseBatch_MixedShortCircuit_IndexMappingPreserved(t *testing.T) {
    // Build a batch loader with id key
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("resp_mixed_short.proto"),
        Package: protoString("lsvc"),
        MessageType: []*descriptorpb.DescriptorProto{
            {Name: protoString("Item"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("id"), JsonName: protoString("id"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()}}},
            {Name: protoString("ItemOut"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("data"), JsonName: protoString("data"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()}}},
            {Name: protoString("BatchReq"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("batches"), JsonName: protoString("batches"), Number: protoInt32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: protoString(".lsvc.Item")}}},
            {Name: protoString("BatchResp"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("batches"), JsonName: protoString("batches"), Number: protoInt32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: protoString(".lsvc.ItemOut")}}},
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{Name: protoString("LoaderService"), Method: []*descriptorpb.MethodDescriptorProto{{Name: protoString("BatchLoad"), InputType: protoString(".lsvc.BatchReq"), OutputType: protoString(".lsvc.BatchResp")}}}},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("resp_mixed_short.proto")
    require.NoError(t, err)
    md := fd.Services().ByName("LoaderService").Methods().ByName("BatchLoad")

    // Response with two elements (matching included tasks positions 0 and 2)
    out := dynamicpb.NewMessage(md.Output())
    of := md.Output().Fields().ByName("batches")
    itemDesc := of.Message()
    lst := out.Mutable(of).List()
    it1 := dynamicpb.NewMessage(itemDesc)
    it1.Set(itemDesc.Fields().ByName("data"), protoreflect.ValueOfString("A"))
    lst.Append(protoreflect.ValueOfMessage(it1))
    it2 := dynamicpb.NewMessage(itemDesc)
    it2.Set(itemDesc.Fields().ByName("data"), protoreflect.ValueOfString("B"))
    lst.Append(protoreflect.ValueOfMessage(it2))
    out.Set(of, protoreflect.ValueOfList(lst))

    reg := NewMockRegistry().RegisterBatchLoader("Obj", "byId", md)
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)
    tasks := []executor.AsyncResolveTask{
        {ObjectType: "Obj", Field: "byId", Args: map[string]any{"id": "u1"}},
        {ObjectType: "Obj", Field: "byId", Args: map[string]any{"id": nil}},
        {ObjectType: "Obj", Field: "byId", Args: map[string]any{"id": "u3"}},
    }
    res := rt.BatchResolveAsync(context.Background(), tasks)
    require.Equal(t, 3, len(res))
    require.NoError(t, res[0].Error)
    require.Equal(t, "A", res[0].Value)
    require.NoError(t, res[1].Error)
    require.Nil(t, res[1].Value)
    require.NoError(t, res[2].Error)
    require.Equal(t, "B", res[2].Value)
}
