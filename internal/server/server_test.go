package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	executor "github.com/hanpama/protograph/internal/executor"
	reqid "github.com/hanpama/protograph/internal/reqid"
	schema "github.com/hanpama/protograph/internal/schema"
	"google.golang.org/grpc/metadata"
)

func newTestHandler(t *testing.T, rt executor.Runtime, opts ...Option) *Handler {
	t.Helper()
	sdl := `type Query { hello: String }`
	sch, err := schema.BuildFromSDL(sdl)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	h, err := New(rt, sch, opts...)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	return h
}

func TestForwardedHeaders(t *testing.T) {
	rt := executor.NewMockRuntime(nil)
	var captured metadata.MD
	rt.SetResolver("Query", "hello", func(ctx context.Context, src any, args map[string]any) (any, error) {
		captured, _ = metadata.FromOutgoingContext(ctx)
		return "world", nil
	})
	h := newTestHandler(t, rt, WithMetadataHeaders("X-Test"))

	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"query":"{ hello }"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test", "abc")
	req.Header.Set("X-Other", "nope")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if captured == nil || captured.Get("x-test")[0] != "abc" || len(captured.Get("x-other")) > 0 {
		t.Fatalf("metadata not propagated correctly: %v", captured)
	}
}

func TestForwardedHeadersDefaultEmpty(t *testing.T) {
	rt := executor.NewMockRuntime(nil)
	var captured metadata.MD
	rt.SetResolver("Query", "hello", func(ctx context.Context, src any, args map[string]any) (any, error) {
		captured, _ = metadata.FromOutgoingContext(ctx)
		return "world", nil
	})
	h := newTestHandler(t, rt)

	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"query":"{ hello }"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test", "abc")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if captured != nil && len(captured.Get("x-test")) > 0 {
		t.Fatalf("header should not be forwarded by default: %v", captured)
	}
}

func TestCORSAndPreflight(t *testing.T) {
	rt := executor.NewMockRuntime(map[string]executor.MockResolver{
		"Query.hello": executor.NewMockValueResolver("world"),
	})
	h := newTestHandler(t, rt, WithCORS("*"))

	// simple request
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"query":"{ hello }"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("missing CORS header")
	}

	// preflight
	pre := httptest.NewRequest("OPTIONS", "/", nil)
	pre.Header.Set("Origin", "http://example.com")
	pre.Header.Set("Access-Control-Request-Headers", "X-Test")
	pw := httptest.NewRecorder()
	h.ServeHTTP(pw, pre)
	if pw.Code != http.StatusNoContent {
		t.Fatalf("preflight status %d", pw.Code)
	}
	if pw.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("preflight missing CORS header")
	}
	if pw.Header().Get("Access-Control-Allow-Headers") != "X-Test" {
		t.Fatalf("preflight missing allow headers")
	}
}

func TestMaxBodyBytes(t *testing.T) {
	rt := executor.NewMockRuntime(map[string]executor.MockResolver{
		"Query.hello": executor.NewMockValueResolver("world"),
	})
	h := newTestHandler(t, rt, WithMaxBodyBytes(10))

	body := bytes.NewBufferString(`{"query":"1234567890"}`)
	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 got %d", w.Code)
	}
}

func TestRequestID(t *testing.T) {
	rt := executor.NewMockRuntime(nil)
	var capturedMD metadata.MD
	var capturedID int64
	rt.SetResolver("Query", "hello", func(ctx context.Context, src any, args map[string]any) (any, error) {
		capturedMD, _ = metadata.FromOutgoingContext(ctx)
		capturedID, _ = reqid.FromContext(ctx)
		return "world", nil
	})
	h := newTestHandler(t, rt)

	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"query":"{ hello }"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if capturedID == 0 {
		t.Fatalf("missing request id in context")
	}
	if got := capturedMD.Get("graphql-request-id"); len(got) == 0 || got[0] != strconv.FormatInt(capturedID, 10) {
		t.Fatalf("metadata mismatch: %v id %d", capturedMD, capturedID)
	}
}
