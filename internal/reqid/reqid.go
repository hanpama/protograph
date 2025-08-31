package reqid

import (
	"context"
	"math/rand"
	"time"
)

// key is the context key for the request ID.
type key struct{}

// NewContext returns a copy of parent with a new random request ID stored.
// It also returns the generated ID.
func NewContext(parent context.Context) (context.Context, int64) {
	id := rand.Int63()
	return context.WithValue(parent, key{}, id), id
}

// FromContext extracts the request ID from ctx.
// It returns the ID and whether it was present.
func FromContext(ctx context.Context) (int64, bool) {
	v := ctx.Value(key{})
	id, ok := v.(int64)
	return id, ok
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
