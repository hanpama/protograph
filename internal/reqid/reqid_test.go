package reqid

import (
	"context"
	"testing"
)

func TestContextRoundTrip(t *testing.T) {
	ctx, id := NewContext(context.Background())
	got, ok := FromContext(ctx)
	if !ok || got != id {
		t.Fatalf("expected %d from context, got %d ok=%v", id, got, ok)
	}
	if _, ok := FromContext(context.Background()); ok {
		t.Fatalf("unexpected id in empty context")
	}
}
