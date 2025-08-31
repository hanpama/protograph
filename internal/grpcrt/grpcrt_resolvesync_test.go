package grpcrt

import (
    "context"
    "testing"

    "google.golang.org/protobuf/reflect/protodesc"
    "google.golang.org/protobuf/reflect/protoreflect"
    "google.golang.org/protobuf/types/descriptorpb"
    "google.golang.org/protobuf/types/dynamicpb"
)

// use MockRegistry from registry_mock.go

// buildTestMessage returns a dynamic message descriptor for:
// message UserSource { string title = 1; }
func buildTestMessage(t *testing.T) (msgDesc protoreflect.MessageDescriptor, field protoreflect.FieldDescriptor) {
    t.Helper()
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("test.proto"),
        Package: protoString("testpkg"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: protoString("UserSource"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     protoString("title"),
						JsonName: protoString("title"),
						Number:   protoInt32(1),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					},
				},
			},
		},
		Syntax: protoString("proto3"),
	}

    // Wrap in a FileDescriptorSet to satisfy protodesc.NewFiles
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
	if err != nil {
		t.Fatalf("failed to build files: %v", err)
	}
    fd, err := files.FindFileByPath("test.proto")
    if err != nil {
        t.Fatalf("failed to find file: %v", err)
    }
    md := fd.Messages().ByName("UserSource")
    fld := md.Fields().ByName("title")
    return md, fld
}

func protoString(s string) *string { return &s }
func protoInt32(n int32) *int32    { return &n }

// Strategic first test: happy-path ResolveSync reading a source field without I/O.
// docs §1.1
func Test_1_1_ResolveSync_ReturnsValueFromSourceField(t *testing.T) {
	md, fd := buildTestMessage(t)
	msg := dynamicpb.NewMessage(md)
	// Set title = "Hello"
	msg.Set(fd, protoreflect.ValueOfString("Hello"))

	reg := NewMockRegistry().RegisterSourceField("User", "title", fd)

	// Transport is not used in ResolveSync; pass nil
	rt := NewRuntime(reg, nil)
	got, err := rt.ResolveSync(context.Background(), "User", "title", msg, nil)
	if err != nil {
		t.Fatalf("ResolveSync error: %v", err)
	}
	s, ok := got.(string)
	if !ok || s != "Hello" {
		t.Fatalf("got %v (%T), want 'Hello' (string)", got, got)
	}
}

// docs §1.1
func Test_1_1_ResolveSync_MissingField_ReturnsNil(t *testing.T) {
    md, fd := buildTestMessage(t)
    msg := dynamicpb.NewMessage(md)
    // Do not set the field; presence should be false in proto3 when unset.

	reg := NewMockRegistry().RegisterSourceField("User", "title", fd)

    rt := NewRuntime(reg, nil)
    got, err := rt.ResolveSync(context.Background(), "User", "title", msg, nil)
    if err != nil {
        t.Fatalf("ResolveSync error: %v", err)
    }
    if got != nil {
        t.Fatalf("expected nil for missing field, got %v (%T)", got, got)
    }
}

// docs §1.2
func Test_1_2_ResolveSync_SourceNotMessage_Panics(t *testing.T) {
    reg := NewMockRegistry().RegisterSourceField("User", "title", nil)
    rt := NewRuntime(reg, nil)
    defer func() {
        if r := recover(); r == nil {
            t.Fatalf("expected panic when source is not a protoreflect.Message")
        }
    }()
    // Pass a non-message source to trigger panic
    _, _ = rt.ResolveSync(context.Background(), "User", "title", 123, nil)
}

// docs §1.3
func Test_1_3_ResolveSync_HandleValue_ScalarKinds(t *testing.T) {
    // Build message with various scalar fields; test one by one via registry mapping
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("scalars.proto"),
        Package: protoString("sc"),
        MessageType: []*descriptorpb.DescriptorProto{{
            Name: protoString("S"),
            Field: []*descriptorpb.FieldDescriptorProto{
                {Name: protoString("b"), JsonName: protoString("b"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()},
                {Name: protoString("i32"), JsonName: protoString("i32"), Number: protoInt32(2), Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
                {Name: protoString("i64"), JsonName: protoString("i64"), Number: protoInt32(3), Type: descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()},
                {Name: protoString("u32"), JsonName: protoString("u32"), Number: protoInt32(4), Type: descriptorpb.FieldDescriptorProto_TYPE_UINT32.Enum()},
                {Name: protoString("u64"), JsonName: protoString("u64"), Number: protoInt32(5), Type: descriptorpb.FieldDescriptorProto_TYPE_UINT64.Enum()},
                {Name: protoString("f32"), JsonName: protoString("f32"), Number: protoInt32(6), Type: descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum()},
                {Name: protoString("f64"), JsonName: protoString("f64"), Number: protoInt32(7), Type: descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()},
                {Name: protoString("s"), JsonName: protoString("s"), Number: protoInt32(8), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
                {Name: protoString("bs"), JsonName: protoString("bs"), Number: protoInt32(9), Type: descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum()},
            },
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    if err != nil { t.Fatalf("files: %v", err) }
    fd, err := files.FindFileByPath("scalars.proto")
    if err != nil { t.Fatalf("find: %v", err) }
    md := fd.Messages().ByName("S")
    msg := dynamicpb.NewMessage(md)
    // Set values
    setf := func(n string, v protoreflect.Value) { msg.Set(md.Fields().ByName(protoreflect.Name(n)), v) }
    setf("b", protoreflect.ValueOfBool(true))
    setf("i32", protoreflect.ValueOfInt32(10))
    setf("i64", protoreflect.ValueOfInt64(11))
    setf("u32", protoreflect.ValueOfUint32(12))
    setf("u64", protoreflect.ValueOfUint64(13))
    setf("f32", protoreflect.ValueOfFloat32(1.5))
    setf("f64", protoreflect.ValueOfFloat64(2.5))
    setf("s", protoreflect.ValueOfString("x"))
    setf("bs", protoreflect.ValueOfBytes([]byte{1,2}))

    // Helper
    run := func(field string) any {
        reg := NewMockRegistry().RegisterSourceField("S", field, md.Fields().ByName(protoreflect.Name(field)))
        rt := NewRuntime(reg, nil)
        v, err := rt.ResolveSync(context.Background(), "S", field, msg, nil)
        if err != nil { t.Fatalf("resolve %s: %v", field, err) }
        return v
    }
    if got := run("b"); got != true { t.Fatalf("b got %v", got) }
    if got := run("i32"); got.(int32) != 10 { t.Fatalf("i32 got %v", got) }
    if got := run("i64"); got.(int64) != 11 { t.Fatalf("i64 got %v", got) }
    if got := run("u32"); got.(uint32) != 12 { t.Fatalf("u32 got %v", got) }
    if got := run("u64"); got.(uint64) != 13 { t.Fatalf("u64 got %v", got) }
    if got := run("f32"); got.(float32) != float32(1.5) { t.Fatalf("f32 got %v", got) }
    if got := run("f64"); got.(float64) != 2.5 { t.Fatalf("f64 got %v", got) }
    if got := run("s"); got.(string) != "x" { t.Fatalf("s got %v", got) }
    if bs, ok := run("bs").([]byte); !ok || len(bs) != 2 || bs[0] != 1 || bs[1] != 2 { t.Fatalf("bs got %v", run("bs")) }
}

// docs §1.3
func Test_1_3_ResolveSync_HandleValue_EnumNameOrNumber(t *testing.T) {
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("enum.proto"),
        Package: protoString("e"),
        EnumType: []*descriptorpb.EnumDescriptorProto{{Name: protoString("Color"), Value: []*descriptorpb.EnumValueDescriptorProto{{Name: protoString("COLOR_UNSPECIFIED"), Number: protoInt32(0)}, {Name: protoString("RED"), Number: protoInt32(1)}}}},
        MessageType: []*descriptorpb.DescriptorProto{{
            Name: protoString("E"),
            Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("color"), JsonName: protoString("color"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(), TypeName: protoString(".e.Color")}},
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    if err != nil { t.Fatalf("files: %v", err) }
    fd, err := files.FindFileByPath("enum.proto")
    if err != nil { t.Fatalf("find: %v", err) }
    md := fd.Messages().ByName("E")
    f := md.Fields().ByName("color")
    msg := dynamicpb.NewMessage(md)
    // Known value 1 -> "RED"
    msg.Set(f, protoreflect.ValueOfEnum(1))
    reg := NewMockRegistry().RegisterSourceField("E", "color", f)
    rt := NewRuntime(reg, nil)
    v, err := rt.ResolveSync(context.Background(), "E", "color", msg, nil)
    if err != nil { t.Fatalf("err: %v", err) }
    if v.(string) != "RED" { t.Fatalf("want RED got %v", v) }
    // Unknown number -> int32
    msg2 := dynamicpb.NewMessage(md)
    msg2.Set(f, protoreflect.ValueOfEnum(99))
    v2, err := rt.ResolveSync(context.Background(), "E", "color", msg2, nil)
    if err != nil { t.Fatalf("err: %v", err) }
    if v2.(int32) != 99 { t.Fatalf("want 99 got %v", v2) }
}

// docs §1.3
func Test_1_3_ResolveSync_HandleValue_MessagePassThrough(t *testing.T) {
    // Message field should be passed as protoreflect.Message
    file := &descriptorpb.FileDescriptorProto{
        Name:    protoString("msg.proto"),
        Package: protoString("m"),
        MessageType: []*descriptorpb.DescriptorProto{{
            Name: protoString("Inner"),
            Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("x"), JsonName: protoString("x"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()}},
        }, {
            Name: protoString("Wrap"),
            Field: []*descriptorpb.FieldDescriptorProto{{Name: protoString("m"), JsonName: protoString("m"), Number: protoInt32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: protoString(".m.Inner")}},
        }},
        Syntax: protoString("proto3"),
    }
    set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
    files, err := protodesc.NewFiles(set)
    if err != nil { t.Fatalf("files: %v", err) }
    fd, err := files.FindFileByPath("msg.proto")
    if err != nil { t.Fatalf("find: %v", err) }
    wrap := fd.Messages().ByName("Wrap")
    inner := fd.Messages().ByName("Inner")
    f := wrap.Fields().ByName("m")
    innerMsg := dynamicpb.NewMessage(inner)
    innerMsg.Set(inner.Fields().ByName("x"), protoreflect.ValueOfString("v"))
    msg := dynamicpb.NewMessage(wrap)
    msg.Set(f, protoreflect.ValueOfMessage(innerMsg))
    reg := NewMockRegistry().RegisterSourceField("Wrap", "m", f)
    rt := NewRuntime(reg, nil)
    v, err := rt.ResolveSync(context.Background(), "Wrap", "m", msg, nil)
    if err != nil { t.Fatalf("err: %v", err) }
    if _, ok := v.(protoreflect.Message); !ok { t.Fatalf("expected message, got %T", v) }
}
