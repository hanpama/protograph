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

// Build a simple source message: ObjSource { id: string, organizationId: string }
func buildSourceWithIDs(t *testing.T) (protoreflect.MessageDescriptor, protoreflect.FieldDescriptor, protoreflect.FieldDescriptor) {
    t.Helper()
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("src_ids.proto"),
        Package: protoString("s"),
        MessageType: []*descriptorpb.DescriptorProto{{
            Name: protoString("ObjSource"),
            Field: []*descriptorpb.FieldDescriptorProto{
                {Name: protoString("id"), JsonName: protoString("id"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
                {Name: protoString("organizationId"), JsonName: protoString("organizationId"), Number: protoInt32(2), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
            },
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("src_ids.proto")
    require.NoError(t, err)
    md := fd.Messages().ByName("ObjSource")
    fid := md.Fields().ByName("id")
    forg := md.Fields().ByName("organizationId")
    return md, fid, forg
}

func TestRequestMapping_SingleResolver_UsesParentSource(t *testing.T) {
    // Build resolver: ResolveObjF(Request{ authorId }) -> Response{ data: string }
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("req_map_single.proto"),
        Package: protoString("q"),
        MessageType: []*descriptorpb.DescriptorProto{
            {Name: protoString("Req"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("authorId"), JsonName: protoString("authorId"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()}}},
            {Name: protoString("Resp"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("data"), JsonName: protoString("data"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()}}},
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{Name: protoString("S"), Method: []*descriptorpb.MethodDescriptorProto{{Name: protoString("Resolve"), InputType: protoString(".q.Req"), OutputType: protoString(".q.Resp")}}}},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("req_map_single.proto")
    require.NoError(t, err)
    md := fd.Services().ByName("S").Methods().ByName("Resolve")

    // Prepare response
    out := dynamicpb.NewMessage(md.Output())
    out.Set(md.Output().Fields().ByName("data"), protoreflect.ValueOfString("ok"))

    // Build parent source with id="u1"
    srcMsgDesc, fid, _ := buildSourceWithIDs(t)
    src := dynamicpb.NewMessage(srcMsgDesc)
    src.Set(fid, protoreflect.ValueOfString("u1"))

    reg := NewMockRegistry().
        RegisterSingleResolver("Obj", "f", md).
        RegisterRequestSourceMap("Obj", "f", map[string]string{"authorId": "id"}).
        RegisterSourceField("Obj", "id", fid)
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)

    // No arg authorId provided; should be copied from source.id
    _ = rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "f", Source: src, Args: map[string]any{}}})
    calls := mt.Calls()
    require.Equal(t, 1, len(calls))
    req := calls[0].Request.ProtoReflect()
    rf := md.Input().Fields().ByName("authorId")
    require.Equal(t, "u1", req.Get(rf).String())
}

func TestRequestMapping_BatchLoader_UsesParentSource(t *testing.T) {
    // Build batch loader: BatchLoadObjById(BatchReq{ batches: Item{id} }) -> BatchResp{ batches: ItemOut{data} }
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("req_map_loader.proto"),
        Package: protoString("l"),
        MessageType: []*descriptorpb.DescriptorProto{
            {Name: protoString("Item"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("id"), JsonName: protoString("id"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()}}},
            {Name: protoString("ItemOut"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("data"), JsonName: protoString("data"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()}}},
            {Name: protoString("BatchReq"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("batches"), JsonName: protoString("batches"), Number: protoInt32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: protoString(".l.Item")}}},
            {Name: protoString("BatchResp"), Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("batches"), JsonName: protoString("batches"), Number: protoInt32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: protoString(".l.ItemOut")}}},
        },
        Service: []*descriptorpb.ServiceDescriptorProto{{Name: protoString("LS"), Method: []*descriptorpb.MethodDescriptorProto{{Name: protoString("BatchLoadObjById"), InputType: protoString(".l.BatchReq"), OutputType: protoString(".l.BatchResp")}}}},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    require.NoError(t, err)
    fd, err := files.FindFileByPath("req_map_loader.proto")
    require.NoError(t, err)
    md := fd.Services().ByName("LS").Methods().ByName("BatchLoadObjById")

    // Response with one item
    out := dynamicpb.NewMessage(md.Output())
    of := md.Output().Fields().ByName("batches")
    itemDesc := of.Message()
    lst := out.Mutable(of).List()
    it := dynamicpb.NewMessage(itemDesc)
    it.Set(itemDesc.Fields().ByName("data"), protoreflect.ValueOfString("ok"))
    lst.Append(protoreflect.ValueOfMessage(it))
    out.Set(of, protoreflect.ValueOfList(lst))

    // Parent source has organizationId="org-1"
    srcMsgDesc, _, forg := buildSourceWithIDs(t)
    src := dynamicpb.NewMessage(srcMsgDesc)
    src.Set(forg, protoreflect.ValueOfString("org-1"))

    reg := NewMockRegistry().
        RegisterBatchLoader("Obj", "org", md).
        RegisterRequestSourceMap("Obj", "org", map[string]string{"id": "organizationId"}).
        RegisterSourceField("Obj", "organizationId", forg)
    mt := NewMockTransport(out)
    rt := NewRuntime(reg, mt)

    // No id arg provided; should be copied from source.organizationId
    _ = rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "org", Source: src, Args: map[string]any{}}})
    calls := mt.Calls()
    require.Equal(t, 1, len(calls))
    req := calls[0].Request.ProtoReflect()
    bf := md.Input().Fields().ByName("batches")
    idField := bf.Message().Fields().ByName("id")
    require.Equal(t, "org-1", req.Get(bf).List().Get(0).Message().Get(idField).String())
}

