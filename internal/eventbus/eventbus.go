package eventbus

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
)

// Handler processes events of type T.
type Handler[T any] func(context.Context, T)

// Bus is a simple in-process event dispatcher.
type Bus struct {
	mu       sync.RWMutex
	handlers map[reflect.Type][]any // Handler[T] stored without type
}

// New creates a new Bus.
func New() *Bus { return &Bus{handlers: make(map[reflect.Type][]any)} }

func (b *Bus) subscribe(t reflect.Type, h any) (unsubscribe func()) {
	b.mu.Lock()
	hs := b.handlers[t]
	b.handlers[t] = append(hs, h)
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		hs := b.handlers[t]
		for i, fn := range hs {
			if reflect.ValueOf(fn).Pointer() == reflect.ValueOf(h).Pointer() {
				hs = append(hs[:i], hs[i+1:]...)
				break
			}
		}
		if len(hs) == 0 {
			delete(b.handlers, t)
		} else {
			b.handlers[t] = hs
		}
	}
}

// Emit dispatches e to all handlers of its dynamic type.
func (b *Bus) emit(ctx context.Context, e any) {
	if b == nil {
		return
	}
	t := reflect.TypeOf(e)
	b.mu.RLock()
	hs := b.handlers[t]
	if len(hs) == 0 {
		b.mu.RUnlock()
		return
	}
	copied := append([]any(nil), hs...)
	b.mu.RUnlock()
	for _, fn := range copied {
		fn.(func(context.Context, any))(ctx, e)
	}
}

var global atomic.Pointer[Bus]

// Use sets the global bus. Passing nil disables event publishing.
func Use(b *Bus) { global.Store(b) }

// Subscribe registers h with the global bus.
func Subscribe[T any](h Handler[T]) (unsubscribe func()) {
	if b := global.Load(); b != nil {
		t := reflect.TypeOf((*T)(nil)).Elem()
		wrapped := func(ctx context.Context, v any) { h(ctx, v.(T)) }
		return b.subscribe(t, wrapped)
	}
	return func() {}
}

// Publish sends e through the global bus.
func Publish[T any](ctx context.Context, e T) {
	if b := global.Load(); b != nil {
		b.emit(ctx, e)
	}
}
