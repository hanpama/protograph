package grpcrt

import (
	"context"
	"testing"

	executor "github.com/hanpama/protograph/internal/executor"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func buildSingleMethod(t *testing.T, name string, out *descriptorpb.DescriptorProto) protoreflect.MethodDescriptor {
	t.Helper()
	file := &descriptorpb.FileDescriptorProto{
		Name:    protoString(name + ".proto"),
		Package: protoString("rsvc"),
		MessageType: []*descriptorpb.DescriptorProto{
			{ // Request
				Name: protoString(name + "Request"),
			},
			out,
		},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: protoString("RespService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       protoString(name),
				InputType:  protoString(".rsvc." + name + "Request"),
				OutputType: protoString(".rsvc." + out.GetName()),
			}},
		}},
		Syntax: protoString("proto3"),
	}
	set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
	files, err := protodesc.NewFiles(set)
	require.NoError(t, err)
	fd, err := files.FindFileByPath(name + ".proto")
	require.NoError(t, err)
	md := fd.Services().ByName("RespService").Methods().ByName(protoreflect.Name(name))
	require.NotNil(t, md)
	return md
}

// docs §6.1
func Test_6_1_ResponseSingle_DataScalarListMessage(t *testing.T) {
	// Scalar: data: string
	outScalar := &descriptorpb.DescriptorProto{
		Name: protoString("ScalarResp"),
		Field: []*descriptorpb.FieldDescriptorProto{{
			Name:     protoString("data"),
			JsonName: protoString("data"),
			Number:   protoInt32(1),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		}},
	}
	mdS := buildSingleMethod(t, "Scalar", outScalar)
	respS := dynamicpb.NewMessage(mdS.Output())
	respS.Set(mdS.Output().Fields().ByName("data"), protoreflect.ValueOfString("ok"))

	// List: data: repeated string
	outList := &descriptorpb.DescriptorProto{
		Name: protoString("ListResp"),
		Field: []*descriptorpb.FieldDescriptorProto{{
			Name:     protoString("data"),
			JsonName: protoString("data"),
			Number:   protoInt32(1),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		}},
	}
	mdL := buildSingleMethod(t, "List", outList)
	respL := dynamicpb.NewMessage(mdL.Output())
	lf := mdL.Output().Fields().ByName("data")
	l := respL.Mutable(lf).List()
	l.Append(protoreflect.ValueOfString("a"))
	l.Append(protoreflect.ValueOfString("b"))
	respL.Set(lf, protoreflect.ValueOfList(l))

	// Message: data: message { x: string }
	inner := &descriptorpb.DescriptorProto{
		Name: protoString("Inner"),
		Field: []*descriptorpb.FieldDescriptorProto{{
			Name:     protoString("x"),
			JsonName: protoString("x"),
			Number:   protoInt32(1),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		}},
	}
	outMsg := &descriptorpb.DescriptorProto{
		Name: protoString("MsgResp"),
		Field: []*descriptorpb.FieldDescriptorProto{{
			Name:     protoString("data"),
			JsonName: protoString("data"),
			Number:   protoInt32(1),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			TypeName: protoString(".rsvc.Inner"),
		}},
	}
	// Build file with inner + outMsg
	file := &descriptorpb.FileDescriptorProto{
		Name:    protoString("Msg.proto"),
		Package: protoString("rsvc"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: protoString("MsgRequest")},
			inner,
			outMsg,
		},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: protoString("RespService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       protoString("Msg"),
				InputType:  protoString(".rsvc.MsgRequest"),
				OutputType: protoString(".rsvc.MsgResp"),
			}},
		}},
		Syntax: protoString("proto3"),
	}
	set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
	files, err := protodesc.NewFiles(set)
	require.NoError(t, err)
	fd, err := files.FindFileByPath("Msg.proto")
	require.NoError(t, err)
	mdM := fd.Services().ByName("RespService").Methods().ByName("Msg")
	respM := dynamicpb.NewMessage(mdM.Output())
	innerDesc := mdM.Output().Fields().ByName("data").Message()
	innerMsg := dynamicpb.NewMessage(innerDesc)
	innerMsg.Set(innerDesc.Fields().ByName("x"), protoreflect.ValueOfString("val"))
	respM.Set(mdM.Output().Fields().ByName("data"), protoreflect.ValueOfMessage(innerMsg))

	// Registry and transport
	reg := NewMockRegistry().
		RegisterSingleResolver("Obj", "s", mdS).
		RegisterSingleResolver("Obj", "l", mdL).
		RegisterSingleResolver("Obj", "m", mdM)
	mt := NewMockTransport(respS, respL, respM)
	rt := NewRuntime(reg, mt)

	// Scalar
	r1 := rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "s", Args: map[string]any{}}})
	require.Equal(t, 1, len(r1))
	require.NoError(t, r1[0].Error)
	require.Equal(t, "ok", r1[0].Value)
	// List
	r2 := rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "l", Args: map[string]any{}}})
	require.Equal(t, 1, len(r2))
	require.NoError(t, r2[0].Error)
	require.ElementsMatch(t, []any{"a", "b"}, r2[0].Value.([]any))
	// Message
	r3 := rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "m", Args: map[string]any{}}})
	require.Equal(t, 1, len(r3))
	require.NoError(t, r3[0].Error)
	if _, ok := r3[0].Value.(protoreflect.Message); !ok {
		t.Fatalf("expected message, got %T", r3[0].Value)
	}
}

// docs §6.1: message data not set -> nil
func Test_6_1_ResponseSingle_MessageData_NotSet_YieldsNull(t *testing.T) {
	// Message: data: message { x: string } but not set in response
	inner := &descriptorpb.DescriptorProto{
		Name: protoString("Inner2"),
		Field: []*descriptorpb.FieldDescriptorProto{{
			Name:     protoString("x"),
			JsonName: protoString("x"),
			Number:   protoInt32(1),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		}},
	}
	outMsg := &descriptorpb.DescriptorProto{
		Name: protoString("MsgResp2"),
		Field: []*descriptorpb.FieldDescriptorProto{{
			Name:     protoString("data"),
			JsonName: protoString("data"),
			Number:   protoInt32(1),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			TypeName: protoString(".rsvc.Inner2"),
		}},
	}
	file := &descriptorpb.FileDescriptorProto{
		Name:    protoString("Msg2.proto"),
		Package: protoString("rsvc"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: protoString("Msg2Request")},
			inner,
			outMsg,
		},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: protoString("RespService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       protoString("Msg2"),
				InputType:  protoString(".rsvc.Msg2Request"),
				OutputType: protoString(".rsvc.MsgResp2"),
			}},
		}},
		Syntax: protoString("proto3"),
	}
	set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
	files, err := protodesc.NewFiles(set)
	require.NoError(t, err)
	fd, err := files.FindFileByPath("Msg2.proto")
	require.NoError(t, err)
	mdM := fd.Services().ByName("RespService").Methods().ByName("Msg2")

	// Response where data is not set (presence=false) -> expect nil
	respM := dynamicpb.NewMessage(mdM.Output())

	reg := NewMockRegistry().RegisterSingleResolver("Obj", "mnil", mdM)
	mt := NewMockTransport(respM)
	rt := NewRuntime(reg, mt)

	r := rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "mnil", Args: map[string]any{}}})
	require.Equal(t, 1, len(r))
	require.NoError(t, r[0].Error)
	require.Nil(t, r[0].Value)
}

// docs §6.2
func Test_6_2_ResponseSingle_EnumAndBytesHandling(t *testing.T) {
	// Enum
	file := &descriptorpb.FileDescriptorProto{
		Name:    protoString("EnumResp.proto"),
		Package: protoString("rsvc"),
		EnumType: []*descriptorpb.EnumDescriptorProto{{
			Name: protoString("Color"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				{Name: protoString("COLOR_UNSPECIFIED"), Number: protoInt32(0)},
				{Name: protoString("RED"), Number: protoInt32(1)},
			},
		}},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: protoString("EnumResp"),
			Field: []*descriptorpb.FieldDescriptorProto{{
				Name:     protoString("data"),
				JsonName: protoString("data"),
				Number:   protoInt32(1),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
				TypeName: protoString(".rsvc.Color"),
			}},
		}, {
			Name: protoString("BytesResp"),
			Field: []*descriptorpb.FieldDescriptorProto{{
				Name:     protoString("data"),
				JsonName: protoString("data"),
				Number:   protoInt32(1),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum(),
			}},
		}},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: protoString("RespService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       protoString("Enum"),
				InputType:  protoString(".rsvc.EnumResp"),
				OutputType: protoString(".rsvc.EnumResp"),
			}, {
				Name:       protoString("Bytes"),
				InputType:  protoString(".rsvc.BytesResp"),
				OutputType: protoString(".rsvc.BytesResp"),
			}},
		}},
		Syntax: protoString("proto3"),
	}
	set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
	files, err := protodesc.NewFiles(set)
	require.NoError(t, err)
	fd, err := files.FindFileByPath("EnumResp.proto")
	require.NoError(t, err)
	mdEnum := fd.Services().ByName("RespService").Methods().ByName("Enum")
	mdBytes := fd.Services().ByName("RespService").Methods().ByName("Bytes")

	respE := dynamicpb.NewMessage(mdEnum.Output())
	respE.Set(mdEnum.Output().Fields().ByName("data"), protoreflect.ValueOfEnum(1))
	respB := dynamicpb.NewMessage(mdBytes.Output())
	respB.Set(mdBytes.Output().Fields().ByName("data"), protoreflect.ValueOfBytes([]byte{0x01, 0x02}))

	reg := NewMockRegistry().RegisterSingleResolver("Obj", "e", mdEnum).RegisterSingleResolver("Obj", "b", mdBytes)
	mt := NewMockTransport(respE, respB)
	rt := NewRuntime(reg, mt)

	r1 := rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "e", Args: map[string]any{}}})
	require.Equal(t, 1, len(r1))
	require.NoError(t, r1[0].Error)
	require.Equal(t, "RED", r1[0].Value)

	r2 := rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "b", Args: map[string]any{}}})
	require.Equal(t, 1, len(r2))
	require.NoError(t, r2[0].Error)
	require.Equal(t, []byte{0x01, 0x02}, r2[0].Value)
}

// docs §6.1
func Test_6_1_ResponseSingle_MissingDataField_Error(t *testing.T) {
	// Build a method whose output lacks 'data'
	file := &descriptorpb.FileDescriptorProto{
		Name:    protoString("NoData.proto"),
		Package: protoString("rsvc"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: protoString("Req")},
			{Name: protoString("RespNoData")},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: protoString("RespService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       protoString("NoData"),
				InputType:  protoString(".rsvc.Req"),
				OutputType: protoString(".rsvc.RespNoData"),
			}},
		}},
		Syntax: protoString("proto3"),
	}
	set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
	files, err := protodesc.NewFiles(set)
	require.NoError(t, err)
	fd, err := files.FindFileByPath("NoData.proto")
	require.NoError(t, err)
	md := fd.Services().ByName("RespService").Methods().ByName("NoData")
	require.NotNil(t, md)

	// Response without data field
	resp := dynamicpb.NewMessage(md.Output())

	reg := NewMockRegistry().RegisterSingleResolver("Obj", "noData", md)
	mt := NewMockTransport(resp)
	rt := NewRuntime(reg, mt)

	res := rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Obj", Field: "noData", Args: map[string]any{}}})
	require.Equal(t, 1, len(res))
	require.Error(t, res[0].Error)
}

// docs §6.3
func Test_6_3_ResponseSingle_LoaderShortCircuit_YieldsNullNoError(t *testing.T) {
	// Build a single loader: LoadUserById(Request{ id }) -> Response{ data }
	file := &descriptorpb.FileDescriptorProto{
		Name:    protoString("SingleLoader.proto"),
		Package: protoString("lsvc"),
		MessageType: []*descriptorpb.DescriptorProto{
			{ // Request
				Name: protoString("LoadUserByIdRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{{
					Name:     protoString("id"),
					JsonName: protoString("id"),
					Number:   protoInt32(1),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				}},
			},
			{ // Response
				Name: protoString("LoadUserByIdResponse"),
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
			Name: protoString("LoaderService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       protoString("LoadUserById"),
				InputType:  protoString(".lsvc.LoadUserByIdRequest"),
				OutputType: protoString(".lsvc.LoadUserByIdResponse"),
			}},
		}},
		Syntax: protoString("proto3"),
	}
	set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
	files, err := protodesc.NewFiles(set)
	require.NoError(t, err)
	fd, err := files.FindFileByPath("SingleLoader.proto")
	require.NoError(t, err)
	md := fd.Services().ByName("LoaderService").Methods().ByName("LoadUserById")

	// Prepare one response for the included task
	resp := dynamicpb.NewMessage(md.Output())
	resp.Set(md.Output().Fields().ByName("data"), protoreflect.ValueOfString("OK"))

	reg := NewMockRegistry().RegisterSingleLoader("User", "byId", md)
	mt := NewMockTransport(resp)
	rt := NewRuntime(reg, mt)

	tasks := []executor.AsyncResolveTask{
		{ObjectType: "User", Field: "byId", Args: map[string]any{"id": nil}}, // short-circuit
		{ObjectType: "User", Field: "byId", Args: map[string]any{"id": "u2"}},
	}
	res := rt.BatchResolveAsync(context.Background(), tasks)
	require.Equal(t, 2, len(res))
	require.NoError(t, res[0].Error)
	require.Nil(t, res[0].Value)
	require.NoError(t, res[1].Error)
	require.Equal(t, "OK", res[1].Value)
	// Only one RPC should be invoked
	require.Equal(t, 1, len(mt.Calls()))
}

func TestInterfaceEnvelopeUnwrapsPayload(t *testing.T) {
	user := &descriptorpb.DescriptorProto{
		Name: protoString("UserSource"),
		Field: []*descriptorpb.FieldDescriptorProto{{
			Name:     protoString("id"),
			JsonName: protoString("id"),
			Number:   protoInt32(1),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		}},
	}
	iface := &descriptorpb.DescriptorProto{
		Name: protoString("NodeSource"),
		Field: []*descriptorpb.FieldDescriptorProto{{
			Name:     protoString("typename"),
			JsonName: protoString("typename"),
			Number:   protoInt32(1),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		}, {
			Name:     protoString("payload"),
			JsonName: protoString("payload"),
			Number:   protoInt32(2),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum(),
		}},
	}
	resp := &descriptorpb.DescriptorProto{
		Name: protoString("NodeResp"),
		Field: []*descriptorpb.FieldDescriptorProto{{
			Name:     protoString("data"),
			JsonName: protoString("data"),
			Number:   protoInt32(1),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			TypeName: protoString(".rsvc.NodeSource"),
		}},
	}
	file := &descriptorpb.FileDescriptorProto{
		Name:    protoString("iface.proto"),
		Package: protoString("rsvc"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: protoString("NodeReq")},
			user,
			iface,
			resp,
		},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: protoString("NodeService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       protoString("Resolve"),
				InputType:  protoString(".rsvc.NodeReq"),
				OutputType: protoString(".rsvc.NodeResp"),
			}},
		}},
		Syntax: protoString("proto3"),
	}
	set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
	files, err := protodesc.NewFiles(set)
	require.NoError(t, err)
	fd, err := files.FindFileByPath("iface.proto")
	require.NoError(t, err)
	md := fd.Services().ByName("NodeService").Methods().ByName("Resolve")
	userDesc := fd.Messages().ByName("UserSource")
	ifaceDesc := fd.Messages().ByName("NodeSource")

	userMsg := dynamicpb.NewMessage(userDesc)
	userMsg.Set(userDesc.Fields().ByName("id"), protoreflect.ValueOfString("user-1"))
	payload, err := proto.Marshal(userMsg)
	require.NoError(t, err)

	ifaceMsg := dynamicpb.NewMessage(ifaceDesc)
	ifaceMsg.Set(ifaceDesc.Fields().ByName("typename"), protoreflect.ValueOfString("User"))
	ifaceMsg.Set(ifaceDesc.Fields().ByName("payload"), protoreflect.ValueOfBytes(payload))

	respMsg := dynamicpb.NewMessage(md.Output())
	respMsg.Set(md.Output().Fields().ByName("data"), protoreflect.ValueOfMessage(ifaceMsg))

	reg := NewMockRegistry().
		RegisterSingleResolver("Query", "node", md).
		RegisterSourceMessage("User", userDesc)
	mt := NewMockTransport(respMsg)
	rt := NewRuntime(reg, mt)

	res := rt.BatchResolveAsync(context.Background(), []executor.AsyncResolveTask{{ObjectType: "Query", Field: "node"}})
	require.Equal(t, 1, len(res))
	require.NoError(t, res[0].Error)

	msg, ok := res[0].Value.(protoreflect.Message)
	require.True(t, ok, "expected message value")
	require.Equal(t, "UserSource", string(msg.Descriptor().Name()))
	require.Equal(t, "user-1", msg.Get(msg.Descriptor().Fields().ByName("id")).String())
}
