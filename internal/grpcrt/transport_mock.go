package grpcrt

import (
    "context"
    "fmt"
    "sync"

    "google.golang.org/protobuf/proto"
    "google.golang.org/protobuf/reflect/protoreflect"
)

// CallRecord captures a single Call invocation for assertions.
type CallRecord struct {
    // Method is the descriptor invoked.
    Method protoreflect.MethodDescriptor
    // FullMethod is "/<service full name>/<method>" for convenience.
    FullMethod string
    // Request is a deep-cloned proto message snapshot of the input.
    Request proto.Message
}

// MockTransport implements Transport and returns pre-seeded responses
// in order, while recording Call invocations for inspection.
type MockTransport struct {
    mu        sync.Mutex
    responses []protoreflect.Message
    errs      []error
    idx       int
    calls     []CallRecord
}

// NewMockTransport creates a MockTransport that will return the provided
// responses in order for successive Call() invocations.
func NewMockTransport(responses ...protoreflect.Message) *MockTransport {
    cp := make([]protoreflect.Message, len(responses))
    copy(cp, responses)
    return &MockTransport{responses: cp}
}

// NewMockTransportWithErrors allows seeding per-call errors alongside responses.
// For call i, if errs[i] is non-nil, Call returns that error and ignores responses[i].
// If errs is shorter than responses, remaining calls will use responses with no error.
func NewMockTransportWithErrors(responses []protoreflect.Message, errs []error) *MockTransport {
    cp := make([]protoreflect.Message, len(responses))
    copy(cp, responses)
    ep := make([]error, len(errs))
    copy(ep, errs)
    return &MockTransport{responses: cp, errs: ep}
}

// Call records the invocation and returns the next queued response.
// If responses are exhausted, it returns an error.
func (m *MockTransport) Call(ctx context.Context, method protoreflect.MethodDescriptor, request protoreflect.Message) (protoreflect.Message, error) {
    _ = ctx
    m.mu.Lock()
    defer m.mu.Unlock()

    var reqClone proto.Message
    if request != nil {
        reqClone = proto.Clone(request.Interface())
    }

    full := ""
    if method != nil {
        full = fmt.Sprintf("/%s/%s", method.Parent().FullName(), method.Name())
    }
    m.calls = append(m.calls, CallRecord{Method: method, FullMethod: full, Request: reqClone})

    if m.idx >= len(m.responses) && m.idx >= len(m.errs) {
        return nil, fmt.Errorf("mock transport: no more responses")
    }
    // Error has precedence if provided for this index
    if m.idx < len(m.errs) {
        if err := m.errs[m.idx]; err != nil {
            m.idx++
            return nil, err
        }
    }
    var resp protoreflect.Message
    if m.idx < len(m.responses) {
        resp = m.responses[m.idx]
    }
    m.idx++
    return resp, nil
}

// Calls returns a snapshot of recorded Call invocations.
func (m *MockTransport) Calls() []CallRecord {
    m.mu.Lock()
    defer m.mu.Unlock()
    out := make([]CallRecord, len(m.calls))
    copy(out, m.calls)
    return out
}
