package grpcrt

import "testing"

// docs §9.1
func Test_9_1_SerializeLeafValue_PassThrough_Primitives(t *testing.T) {
	rt := NewRuntime(nil, nil)
	cases := []any{"s", true, int(1), int32(2), int64(3), float32(1.5), float64(2.5)}
	for _, in := range cases {
		out, err := rt.SerializeLeafValue(t.Context(), "", in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != in {
			t.Fatalf("pass-through failed: in=%v (%T) out=%v (%T)", in, in, out, out)
		}
	}
}

// docs §9.2
func Test_9_2_SerializeLeafValue_Nil(t *testing.T) {
	rt := NewRuntime(nil, nil)
	out, err := rt.SerializeLeafValue(t.Context(), "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil, got %v (%T)", out, out)
	}
}

// docs §9.3
func Test_9_3_SerializeLeafValue_BytesBase64(t *testing.T) {
	rt := NewRuntime(nil, nil)
	in := []byte{0x01, 0x02, 0xFF}
	out, err := rt.SerializeLeafValue(t.Context(), "Bytes", in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s, ok := out.(string); !ok || s != "AQL/" {
		t.Fatalf("expected base64 'AQL/', got %v (%T)", out, out)
	}
}
