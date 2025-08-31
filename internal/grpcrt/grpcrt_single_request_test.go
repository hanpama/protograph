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

// docs §4.1
func Test_4_1_SingleRequest_JSONNameMapping(t *testing.T) {
    // Build single resolver with two fields: arg1(string), arg2(int32)
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("single_req.proto"),
        Package: protoString("qsvc"),
        MessageType: []*descriptorpb.DescriptorProto{
            {
                Name: protoString("Req"),
                Field: []*descriptorpb.FieldDescriptorProto{
                    {Name: protoString("arg1"), JsonName: protoString("arg1"), Number: protoInt32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
                    {Name: protoString("arg2"), JsonName: protoString("arg2"), Number: protoInt32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
                },
            },
            {Name: protoString("Resp"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("data"), JsonName: protoString("data"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()}}},
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{
            Name: protoString("Q"),
            Method: []*descriptorpb.MethodDescriptorProto{{Name: protoString("Resolve"), InputType: protoString(".qsvc.Req"), OutputType: protoString(".qsvc.Resp")}},
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("single_req.proto")
    require.NoError(t, err)
    md := fd.Services().ByName("Q").Methods().ByName("Resolve")

    reg := NewMockRegistry().RegisterSingleResolver("Obj", "f", md)
    // Provide a non-nil response to avoid runtime reading nil
    out := dynamicpb.NewMessage(md.Output())
    out.Set(md.Output().Fields().ByName("data"), protoreflect.ValueOfString("ok"))
    mt := NewMockTransport(out) // response content irrelevant
    rt := NewRuntime(reg, mt)

    // Args include an unknown key 'zzz' which should be ignored
    tasks := []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "f", Args: map[string]any{"arg1": "x", "arg2": int32(7), "zzz": "ignored"}}}
    _ = rt.BatchResolveAsync(context.Background(), tasks)

    calls := mt.Calls()
    require.Equal(t, 1, len(calls))
    req := calls[0].Request.ProtoReflect()
    f1 := md.Input().Fields().ByName("arg1")
    f2 := md.Input().Fields().ByName("arg2")
    require.Equal(t, "x", req.Get(f1).String())
    require.Equal(t, int32(7), int32(req.Get(f2).Int()))
}

// docs §4.1
func Test_4_1_SingleRequest_RepeatedAndEnumAndMessage_Mapping(t *testing.T) {
    // Build input: rs: repeated string, ri32: repeated int32, ri64: repeated int64, rf32: repeated float, rf64: repeated double, rb: repeated bool, color: enum, msg: message{a:string}
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("single_req_map.proto"),
        Package: protoString("qsvc"),
        EnumType: []*descriptorpb.EnumDescriptorProto{{
            Name: protoString("Color"),
            Value: []*descriptorpb.EnumValueDescriptorProto{{Name: protoString("COLOR_UNSPECIFIED"), Number: protoInt32(0)}, {Name: protoString("RED"), Number: protoInt32(1)}},
        }},
        MessageType: []*descriptorpb.DescriptorProto{
            {Name: protoString("Msg"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("a"), JsonName: protoString("a"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()}}},
            {Name: protoString("Req"), Field: []*descriptorpb.FieldDescriptorProto{
                {Name: protoString("rs"), JsonName: protoString("rs"), Number: protoInt32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
                {Name: protoString("ri32"), JsonName: protoString("ri32"), Number: protoInt32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
                {Name: protoString("ri64"), JsonName: protoString("ri64"), Number: protoInt32(3), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()},
                {Name: protoString("rf32"), JsonName: protoString("rf32"), Number: protoInt32(4), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum()},
                {Name: protoString("rf64"), JsonName: protoString("rf64"), Number: protoInt32(5), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()},
                {Name: protoString("rb"), JsonName: protoString("rb"), Number: protoInt32(6), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()},
                {Name: protoString("color"), JsonName: protoString("color"), Number: protoInt32(7), Type: descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(), TypeName: protoString(".qsvc.Color")},
                {Name: protoString("msg"), JsonName: protoString("msg"), Number: protoInt32(8), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: protoString(".qsvc.Msg")},
            }},
            {Name: protoString("Resp"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("data"), JsonName: protoString("data"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()}}},
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{Name: protoString("Q"), Method: []*descriptorpb.MethodDescriptorProto{{Name: protoString("Resolve"), InputType: protoString(".qsvc.Req"), OutputType: protoString(".qsvc.Resp")}}}},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("single_req_map.proto")
    require.NoError(t, err)
    md := fd.Services().ByName("Q").Methods().ByName("Resolve")

    reg := NewMockRegistry().RegisterSingleResolver("Obj", "f", md)
    // Provide a non-nil response to avoid runtime reading nil
    out := dynamicpb.NewMessage(md.Output())
    out.Set(md.Output().Fields().ByName("data"), protoreflect.ValueOfString("ok"))
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)

    args := map[string]any{
        "rs":   []string{"a", "b"},
        "ri32": []int32{1, 2},
        "ri64": []int64{3, 4},
        "rf32": []float32{1.5, 2.5},
        "rf64": []float64{3.5, 4.5},
        "rb":   []bool{true, false},
        "color": "RED",
        "msg":  map[string]any{"a": "x"},
    }
    _ = rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "f", Args: args}})
    calls := mt.Calls()
    require.Equal(t, 1, len(calls))
    req := calls[0].Request.ProtoReflect()

    // Validate list lengths and contents
    getList := func(name string) protoreflect.List { return req.Get(md.Input().Fields().ByName(protoreflect.Name(name))).List() }
    lrs := getList("rs")
    require.Equal(t, 2, lrs.Len())
    require.Equal(t, "a", lrs.Get(0).String())
    require.Equal(t, "b", lrs.Get(1).String())
    l32 := getList("ri32")
    require.Equal(t, int32(1), int32(l32.Get(0).Int()))
    require.Equal(t, int32(2), int32(l32.Get(1).Int()))
    l64 := getList("ri64")
    require.Equal(t, int64(3), int64(l64.Get(0).Int()))
    require.Equal(t, int64(4), int64(l64.Get(1).Int()))
    lf32 := getList("rf32")
    require.InDelta(t, 1.5, lf32.Get(0).Float(), 1e-6)
    require.InDelta(t, 2.5, lf32.Get(1).Float(), 1e-6)
    lf64 := getList("rf64")
    require.InDelta(t, 3.5, lf64.Get(0).Float(), 1e-6)
    require.InDelta(t, 4.5, lf64.Get(1).Float(), 1e-6)
    lb := getList("rb")
    require.Equal(t, true, lb.Get(0).Bool())
    require.Equal(t, false, lb.Get(1).Bool())

    // Enum mapping → number 1 (RED)
    ef := md.Input().Fields().ByName("color")
    require.Equal(t, int32(1), int32(req.Get(ef).Enum()))
    // Message mapping
    mf := md.Input().Fields().ByName("msg")
    require.Equal(t, "x", req.Get(mf).Message().Get(mf.Message().Fields().ByName("a")).String())
}

// docs §4.2
func Test_4_2_SingleRequest_MappingUnsupportedType_ReturnsErrorOnlyForTask(t *testing.T) {
    // Reuse the mapping schema with a simple request/response
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("single_req_badmap.proto"),
        Package: protoString("qsvc"),
        MessageType: []*descriptorpb.DescriptorProto{
            { // Req with repeated int32 field to trigger unsupported type path
                Name: protoString("Req"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("ri32"),
                    JsonName: protoString("ri32"),
                    Number:   protoInt32(1),
                    Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
                }},
            },
            { // Resp with data
                Name: protoString("Resp"),
                Field: []*descriptorpb.FieldDescriptorProto{{
                    Name:     protoString("data"),
                    JsonName: protoString("data"),
                    Number:   protoInt32(1),
                    Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
                }},
            },
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{
            Name: protoString("Q"),
            Method: []*descriptorpb.MethodDescriptorProto{{
                Name:       protoString("Resolve"),
                InputType:  protoString(".qsvc.Req"),
                OutputType: protoString(".qsvc.Resp"),
            }},
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("single_req_badmap.proto")
    require.NoError(t, err)
    md := fd.Services().ByName("Q").Methods().ByName("Resolve")

    // Response for the valid task
    out := dynamicpb.NewMessage(md.Output())
    out.Set(md.Output().Fields().ByName("data"), protoreflect.ValueOfString("OK"))

    reg := NewMockRegistry().RegisterSingleResolver("Obj", "f", md)
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)

    // Task0: unsupported repeated arg type ([]struct{}), mapping should error and skip transport
    // Task1: valid value
    tasks := []executor.AsyncResolveTask{
        {ObjectType: "Obj", Field: "f", Args: map[string]any{"ri32": []struct{}{} }},
        {ObjectType: "Obj", Field: "f", Args: map[string]any{"ri32": []int32{1, 2} }},
    }
    res := rt.BatchResolveAsync(context.Background(), tasks)
    require.Equal(t, 2, len(res))
    require.Error(t, res[0].Error)
    require.NoError(t, res[1].Error)
    require.Equal(t, "OK", res[1].Value)

    // Only one transport call expected
    require.Equal(t, 1, len(mt.Calls()))
}
