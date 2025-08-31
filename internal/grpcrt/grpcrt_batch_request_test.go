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

// docs §3.4
func Test_3_4_BatchRequest_LoaderKeyNullShortCircuit_SkipsTask(t *testing.T) {
    // Build a batch loader: BatchLoadUserById(BatchReq{ batches: Item{id} }) -> BatchResp{ batches: ItemOut{data} }
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("loader.proto"),
        Package: protoString("lsvc"),
        MessageType: []*descriptorpb.DescriptorProto{
            { // Item
                Name: protoString("Item"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("id"),
                    JsonName: protoString("id"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
                }},
            },
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
                    TypeName: protoString(".lsvc.Item"),
                }},
            },
            { // Response
                Name: protoString("BatchResp"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("batches"),
                    JsonName: protoString("batches"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
                    TypeName: protoString(".lsvc.ItemOut"),
                }},
            },
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{
            Name: protoString("LoaderService"),
            Method: []*descriptorpb.MethodDescriptorProto{{
                Name:       protoString("BatchLoadUserById"),
                InputType:  protoString(".lsvc.BatchReq"),
                OutputType: protoString(".lsvc.BatchResp"),
            }},
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("loader.proto")
    require.NoError(t, err)
    md := fd.Services().ByName("LoaderService").Methods().ByName("BatchLoadUserById")

    // Prepare response with a single included element
    out := dynamicpb.NewMessage(md.Output())
    of := md.Output().Fields().ByName("batches")
    itemDesc := of.Message()
    lst := out.Mutable(of).List()
    it := dynamicpb.NewMessage(itemDesc)
    it.Set(itemDesc.Fields().ByName("data"), protoreflect.ValueOfString("OK"))
    lst.Append(protoreflect.ValueOfMessage(it))
    out.Set(of, protoreflect.ValueOfList(lst))

    reg := NewMockRegistry().RegisterBatchLoader("User", "byId", md)
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)

    // Task0 has nil key -> short-circuit; Task1 has id -> included
    tasks := []executor.AsyncResolveTask{
        {ObjectType: "User", Field: "byId", Args: map[string]any{"id": nil}},
        {ObjectType: "User", Field: "byId", Args: map[string]any{"id": "u2"}},
    }
    res := rt.BatchResolveAsync(context.Background(), tasks)
    require.Equal(t, 2, len(res))
    require.NoError(t, res[0].Error)
    require.Nil(t, res[0].Value)
    require.NoError(t, res[1].Error)
    require.Equal(t, "OK", res[1].Value)

    calls := mt.Calls()
    require.Equal(t, 1, len(calls), "only one batch call should be made")
    // Request should have only one batch item with id="u2"
    rf := md.Input().Fields().ByName("batches")
    reqList := calls[0].Request.ProtoReflect().Get(rf).List()
    require.Equal(t, 1, reqList.Len())
    idField := rf.Message().Fields().ByName("id")
    require.Equal(t, "u2", reqList.Get(0).Message().Get(idField).String())
}

func TestBatchRequest_MixedShortCircuitAndIncluded_MaintainsIndices(t *testing.T) {
    // Keep as alias of above but with more elements if needed; covered by previous test's assertions.
    t.Skip("covered by LoaderKeyNullShortCircuit test")
}

// docs §3.1
func Test_3_1_BatchRequest_BatchesFieldLengthMatchesTasks(t *testing.T) {
    // Batch resolver with item having one field
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("batch_len.proto"),
        Package: protoString("bsvc"),
        MessageType: []*descriptorpb.DescriptorProto{
            {Name: protoString("Item"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("a"), JsonName: protoString("a"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()}}},
            {Name: protoString("ItemOut")},
            {Name: protoString("BatchReq"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("batches"), JsonName: protoString("batches"), Number: protoInt32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: protoString(".bsvc.Item")}}},
            {Name: protoString("BatchResp"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("batches"), JsonName: protoString("batches"), Number: protoInt32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: protoString(".bsvc.ItemOut")}}},
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{Name: protoString("B"), Method: []*descriptorpb.MethodDescriptorProto{{Name: protoString("Batch"), InputType: protoString(".bsvc.BatchReq"), OutputType: protoString(".bsvc.BatchResp")}}}},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("batch_len.proto")
    require.NoError(t, err)
    md := fd.Services().ByName("B").Methods().ByName("Batch")
    out := dynamicpb.NewMessage(md.Output())
    reg := NewMockRegistry().RegisterBatchResolver("Obj", "f", md)
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)
    tasks := []executor.AsyncResolveTask{
        {ObjectType: "Obj", Field: "f", Args: map[string]any{"a": "x"}},
        {ObjectType: "Obj", Field: "f", Args: map[string]any{"a": "y"}},
    }
    _ = rt.BatchResolveAsync(context.Background(), tasks)
    calls := mt.Calls()
    require.Equal(t, 1, len(calls))
    bf := md.Input().Fields().ByName("batches")
    lst := calls[0].Request.ProtoReflect().Get(bf).List()
    require.Equal(t, 2, lst.Len())
}
// docs §3.1
func Test_3_1_BatchRequest_JSONNameMapping_IgnoresUnknown(t *testing.T) {
    // Build a batch resolver with item fields: a(string), b(int32)
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("batch_jsonname.proto"),
        Package: protoString("bsvc"),
        MessageType: []*descriptorpb.DescriptorProto{
            {Name: protoString("Item"), Field: []*descriptorpb.FieldDescriptorProto{
                {Name: protoString("a"), JsonName: protoString("a"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
                {Name: protoString("b"), JsonName: protoString("b"), Number: protoInt32(2), Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
            }},
            {Name: protoString("ItemOut"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("data"), JsonName: protoString("data"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()}}},
            {Name: protoString("BatchReq"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("batches"), JsonName: protoString("batches"), Number: protoInt32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: protoString(".bsvc.Item")}}},
            {Name: protoString("BatchResp"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("batches"), JsonName: protoString("batches"), Number: protoInt32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: protoString(".bsvc.ItemOut")}}},
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{Name: protoString("B"), Method: []*descriptorpb.MethodDescriptorProto{{Name: protoString("Batch"), InputType: protoString(".bsvc.BatchReq"), OutputType: protoString(".bsvc.BatchResp")}}}},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("batch_jsonname.proto")
    require.NoError(t, err)
    md := fd.Services().ByName("B").Methods().ByName("Batch")
    // Prepare empty response
    out := dynamicpb.NewMessage(md.Output())
    mt := NewMockTransport(out)
    reg := NewMockRegistry().RegisterBatchResolver("Obj", "f", md)
    rt := NewRuntime(reg, mt)
    tasks := []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "f", Args: map[string]any{"a": "x", "b": int32(7), "zzz": "ignored"}}}
    _ = rt.BatchResolveAsync(context.Background(), tasks)
    calls := mt.Calls()
    require.Equal(t, 1, len(calls))
    req := calls[0].Request.ProtoReflect()
    bf := md.Input().Fields().ByName("batches")
    item := req.Get(bf).List().Get(0).Message()
    require.Equal(t, "x", item.Get(bf.Message().Fields().ByName("a")).String())
    require.Equal(t, int32(7), int32(item.Get(bf.Message().Fields().ByName("b")).Int()))
}
// docs §3.2
func Test_3_2_BatchRequest_RepeatedArgs_AllSupportedSlices(t *testing.T) {
    // Build item with repeated fields
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("batch_repeated.proto"),
        Package: protoString("bsvc"),
        MessageType: []*descriptorpb.DescriptorProto{
            {Name: protoString("Item"), Field: []*descriptorpb.FieldDescriptorProto{
                {Name: protoString("rs"), JsonName: protoString("rs"), Number: protoInt32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
                {Name: protoString("ri32"), JsonName: protoString("ri32"), Number: protoInt32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
                {Name: protoString("ri64"), JsonName: protoString("ri64"), Number: protoInt32(3), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()},
                {Name: protoString("rf32"), JsonName: protoString("rf32"), Number: protoInt32(4), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum()},
                {Name: protoString("rf64"), JsonName: protoString("rf64"), Number: protoInt32(5), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()},
                {Name: protoString("rb"), JsonName: protoString("rb"), Number: protoInt32(6), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()},
            }},
            {Name: protoString("ItemOut")},
            {Name: protoString("BatchReq"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("batches"), JsonName: protoString("batches"), Number: protoInt32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: protoString(".bsvc.Item")}}},
            {Name: protoString("BatchResp"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("batches"), JsonName: protoString("batches"), Number: protoInt32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: protoString(".bsvc.ItemOut")}}},
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{Name: protoString("B"), Method: []*descriptorpb.MethodDescriptorProto{{Name: protoString("Batch"), InputType: protoString(".bsvc.BatchReq"), OutputType: protoString(".bsvc.BatchResp")}}}},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("batch_repeated.proto")
    require.NoError(t, err)
    md := fd.Services().ByName("B").Methods().ByName("Batch")
    out := dynamicpb.NewMessage(md.Output())
    reg := NewMockRegistry().RegisterBatchResolver("Obj", "f", md)
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)
    args := map[string]any{
        "rs":   []string{"a", "b"},
        "ri32": []int32{1, 2},
        "ri64": []int64{3, 4},
        "rf32": []float32{1.5, 2.5},
        "rf64": []float64{3.5, 4.5},
        "rb":   []bool{true, false},
    }
    _ = rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "f", Args: args}})
    req := mt.Calls()[0].Request.ProtoReflect()
    bf := md.Input().Fields().ByName("batches")
    item := req.Get(bf).List().Get(0).Message()
    getList := func(n string) protoreflect.List { return item.Get(bf.Message().Fields().ByName(protoreflect.Name(n))).List() }
    require.Equal(t, 2, getList("rs").Len())
    require.Equal(t, 2, getList("ri32").Len())
    require.Equal(t, 2, getList("ri64").Len())
    require.Equal(t, 2, getList("rf32").Len())
    require.Equal(t, 2, getList("rf64").Len())
    require.Equal(t, 2, getList("rb").Len())
}
// docs §3.2
func Test_3_2_BatchRequest_RepeatedArgs_UnsupportedType_ErrorPerTask(t *testing.T) {
    // Build a batch resolver with item having repeated int32 to exercise unsupported type
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("batch_badmap.proto"),
        Package: protoString("rsvc"),
        MessageType: []*descriptorpb.DescriptorProto{
            { // Item with repeated int32
                Name: protoString("Item"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("ri32"),
                    JsonName: protoString("ri32"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
                }},
            },
            { // ItemOut with data
                Name: protoString("ItemOut"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("data"),
                    JsonName: protoString("data"),
                    Number:   protoInt32(1),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
                }},
            },
            { // BatchReq
                Name: protoString("BatchReq"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("batches"),
                    JsonName: protoString("batches"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
                    TypeName: protoString(".rsvc.Item"),
                }},
            },
            { // BatchResp
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
    fd, err := files.FindFileByPath("batch_badmap.proto")
    require.NoError(t, err)
    md := fd.Services().ByName("RespService").Methods().ByName("BatchMethod")

    // Build one response element for the included item
    out := dynamicpb.NewMessage(md.Output())
    of := md.Output().Fields().ByName("batches")
    itemOut := of.Message()
    lst := out.Mutable(of).List()
    it := dynamicpb.NewMessage(itemOut)
    it.Set(itemOut.Fields().ByName("data"), protoreflect.ValueOfString("OK"))
    lst.Append(protoreflect.ValueOfMessage(it))
    out.Set(of, protoreflect.ValueOfList(lst))

    reg := NewMockRegistry().RegisterBatchResolver("Obj", "f", md)
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)

    tasks := []executor.AsyncResolveTask{
        // Task0: unsupported repeated type for ri32
        {ObjectType: "Obj", Field: "f", Args: map[string]any{"ri32": []struct{}{} }},
        // Task1: valid
        {ObjectType: "Obj", Field: "f", Args: map[string]any{"ri32": []int32{1, 2} }},
    }
    res := rt.BatchResolveAsync(context.Background(), tasks)
    require.Equal(t, 2, len(res))
    require.Error(t, res[0].Error)
    require.NoError(t, res[1].Error)
    require.Equal(t, "OK", res[1].Value)

    // Verify only one batch entry was sent
    calls := mt.Calls()
    require.Equal(t, 1, len(calls))
    req := calls[0].Request.ProtoReflect()
    rf := md.Input().Fields().ByName("batches")
    rlist := req.Get(rf).List()
    require.Equal(t, 1, rlist.Len())
}
// docs §3.3
func Test_3_3_BatchRequest_ScalarMessageEnumMapping(t *testing.T) {
    // Build enum, nested message and scalar mapping inside Item
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("batch_scalar_enum.proto"),
        Package: protoString("bsvc"),
        EnumType: []*descriptorpb.EnumDescriptorProto{{Name: protoString("Color"), Value: []*descriptorpb.EnumValueDescriptorProto{
            {Name: protoString("COLOR_UNSPECIFIED"), Number: protoInt32(0)}, {Name: protoString("RED"), Number: protoInt32(1)},
        }}},
        MessageType: []*descriptorpb.DescriptorProto{
            {Name: protoString("Msg"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("x"), JsonName: protoString("x"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()}}},
            {Name: protoString("Item"), Field: []*descriptorpb.FieldDescriptorProto{
                {Name: protoString("n"), JsonName: protoString("n"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
                {Name: protoString("color"), JsonName: protoString("color"), Number: protoInt32(2), Type: descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(), TypeName: protoString(".bsvc.Color")},
                {Name: protoString("m"), JsonName: protoString("m"), Number: protoInt32(3), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: protoString(".bsvc.Msg")},
            }},
            {Name: protoString("ItemOut")},
            {Name: protoString("BatchReq"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("batches"), JsonName: protoString("batches"), Number: protoInt32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: protoString(".bsvc.Item")}}},
            {Name: protoString("BatchResp"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("batches"), JsonName: protoString("batches"), Number: protoInt32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: protoString(".bsvc.ItemOut")}}},
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{Name: protoString("B"), Method: []*descriptorpb.MethodDescriptorProto{{Name: protoString("Batch"), InputType: protoString(".bsvc.BatchReq"), OutputType: protoString(".bsvc.BatchResp")}}}},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("batch_scalar_enum.proto")
    require.NoError(t, err)
    md := fd.Services().ByName("B").Methods().ByName("Batch")
    out := dynamicpb.NewMessage(md.Output())
    reg := NewMockRegistry().RegisterBatchResolver("Obj", "f", md)
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)

    args := map[string]any{"n": int32(7), "color": "RED", "m": map[string]any{"x": "hello"}}
    _ = rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "f", Args: args}})
    req := mt.Calls()[0].Request.ProtoReflect()
    bf := md.Input().Fields().ByName("batches")
    item := req.Get(bf).List().Get(0).Message()
    it := bf.Message()
    require.Equal(t, int32(7), int32(item.Get(it.Fields().ByName("n")).Int()))
    require.Equal(t, int32(1), int32(item.Get(it.Fields().ByName("color")).Enum()))
    require.Equal(t, "hello", item.Get(it.Fields().ByName("m")).Message().Get(it.Fields().ByName("m").Message().Fields().ByName("x")).String())
}
