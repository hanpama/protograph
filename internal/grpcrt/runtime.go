package grpcrt

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/hanpama/protograph/internal/executor"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Runtime implements executor.Runtime for the gRPC-backed bridge.
// Invariants and boundaries:
//   - Registry trust: When healthy, Registry returns valid descriptors. Missing
//     descriptors indicate a programming/configuration error and cause panic.
//   - Source/value shape: For object fields, source must be a protoreflect.Message.
//     Violations cause panic rather than being hidden behind recoverable errors.
//   - Loader short-circuit: Only input message JSONName fields are inspected for
//     nil to decide short-circuiting; unrelated args do not affect this.
//   - Concurrency: BatchResolveAsync groups tasks by (objectType, field) and
//     executes groups in parallel by default. Transports must be concurrency-safe.
//   - Determinism: Results preserve input ordering; partial success is supported.
type Runtime struct {
	reg       Registry
	transport Transport
}

var _ executor.Runtime = (*Runtime)(nil)

func NewRuntime(registry Registry, transport Transport) executor.Runtime {
	return &Runtime{reg: registry, transport: transport}
}

// ResolveSync resolves only physical fields from the parent source.
// It NEVER performs network I/O. All resolvers/loaders (I/O) are handled in
// BatchResolveAsync. If the field is not present on the source, return (nil, nil)
// to produce a GraphQL null for nullable fields.
//
// Source contract: the executor feeds back whatever value the runtime returned
// for a parent object. Here we expect source to be a protoreflect.Message for
// object values, and we read the physical field directly from that message.
func (r *Runtime) ResolveSync(ctx context.Context, objectType string, field string, source any, args map[string]any) (any, error) {
	// Silence unused args; no I/O and no RPC lookup in ResolveSync.
	_ = ctx
	_ = args

	msg, ok := source.(protoreflect.Message)
	if !ok {
		panic(fmt.Sprintf("ResolveSync: source for %s.%s must be protoreflect.Message, got %T", objectType, field, source))
	}
	fd := r.reg.GetSourceFieldDescriptor(objectType, field)
	if fd == nil {
		panic(fmt.Sprintf("ResolveSync: missing FieldDescriptor for %s.%s", objectType, field))
	}
	if !msg.Has(fd) {
		return nil, nil
	}
	v := msg.Get(fd)
	return r.handleValue(fd, v), nil
}

// BatchResolveAsync executes resolver/loader RPCs. All I/O happens here.
// The executor guarantees only async fields reach this method in a single batch
// per depth.
//
// Concurrency and determinism:
// - grpcrt groups tasks by (objectType, field) and executes those groups in parallel by default.
// - Results are written into pre-determined slots to preserve input ordering per task.
// - Transport implementations MUST be safe for concurrent use.
func (r *Runtime) BatchResolveAsync(ctx context.Context, tasks []executor.AsyncResolveTask) []executor.AsyncResolveResult {
	results := make([]executor.AsyncResolveResult, len(tasks))
	if len(tasks) == 0 {
		return results
	}
	// Group by objectType and field
	type groupKey struct {
		objectType string
		field      string
	}
	type group struct {
		objectType string
		field      string
		idxs       []int
	}
	groups := []group{}
	idxByKey := map[groupKey]int{}
	for i, t := range tasks {
		k := groupKey{objectType: t.ObjectType, field: t.Field}
		if gi, ok := idxByKey[k]; ok {
			groups[gi].idxs = append(groups[gi].idxs, i)
		} else {
			idxByKey[k] = len(groups)
			groups = append(groups, group{objectType: t.ObjectType, field: t.Field, idxs: []int{i}})
		}
	}
	run := func(g group) {
		if md := r.reg.GetBatchResolverDescriptor(g.objectType, g.field); md != nil {
			r.runBatchResolverGroup(ctx, md, tasks, g.idxs, results)
			return
		}
		if md := r.reg.GetSingleResolverDescriptor(g.objectType, g.field); md != nil {
			r.runSingleResolverGroup(ctx, md, tasks, g.idxs, results)
			return
		}
		if md := r.reg.GetBatchLoaderDescriptor(g.objectType, g.field); md != nil {
			r.runBatchLoaderGroup(ctx, md, tasks, g.idxs, results)
			return
		}
		if md := r.reg.GetSingleLoaderDescriptor(g.objectType, g.field); md != nil {
			r.runSingleLoaderGroup(ctx, md, tasks, g.idxs, results)
			return
		}
		panic(fmt.Sprintf("BatchResolveAsync: no resolver/loader registered for %s.%s", g.objectType, g.field))
	}

	if len(groups) > 1 {
		var wg sync.WaitGroup
		wg.Add(len(groups))
		for _, g := range groups {
			g := g // capture
			go func() {
				defer wg.Done()
				run(g)
			}()
		}
		wg.Wait()
	} else {
		for _, g := range groups {
			run(g)
		}
	}
	return results
}

// runBatchResolverGroup executes one batch resolver group and writes results in-place.
func (r *Runtime) runBatchResolverGroup(ctx context.Context, md protoreflect.MethodDescriptor, tasks []executor.AsyncResolveTask, idxs []int, results []executor.AsyncResolveResult) {
	batchRes := r.executeBatch(ctx, md, tasks, idxs)
	for j, idx := range idxs {
		results[idx] = batchRes[j]
	}
}

// runSingleResolverGroup executes single resolver calls for a group and writes results.
func (r *Runtime) runSingleResolverGroup(ctx context.Context, md protoreflect.MethodDescriptor, tasks []executor.AsyncResolveTask, idxs []int, results []executor.AsyncResolveResult) {
	for _, i := range idxs {
		results[i] = r.executeSingle(ctx, md, tasks[i])
	}
}

// runBatchLoaderGroup executes one batch loader group and writes results in-place.
func (r *Runtime) runBatchLoaderGroup(ctx context.Context, md protoreflect.MethodDescriptor, tasks []executor.AsyncResolveTask, idxs []int, results []executor.AsyncResolveResult) {
	batchRes := r.executeBatchLoader(ctx, md, tasks, idxs)
	for j, idx := range idxs {
		results[idx] = batchRes[j]
	}
}

// runSingleLoaderGroup executes single loader calls for a group and writes results.
func (r *Runtime) runSingleLoaderGroup(ctx context.Context, md protoreflect.MethodDescriptor, tasks []executor.AsyncResolveTask, idxs []int, results []executor.AsyncResolveResult) {
	for _, i := range idxs {
		results[i] = r.executeSingleLoader(ctx, md, tasks[i])
	}
}

// executeBatch builds and executes a batch RPC call and returns per-task results
func (r *Runtime) executeBatch(ctx context.Context, md protoreflect.MethodDescriptor, tasks []executor.AsyncResolveTask, idxs []int) []executor.AsyncResolveResult {
	res := make([]executor.AsyncResolveResult, len(idxs))
	imd := md.Input()
	batchesField := imd.Fields().ByName("batches")

	req := dynamicpb.NewMessage(imd)
	list := req.Mutable(batchesField).List()
	itemDesc := batchesField.Message()

	included := make([]int, 0, len(idxs)) // positions within idxs slice
	for pos, taskIdx := range idxs {
		item := dynamicpb.NewMessage(itemDesc)
		// Merge args with source-mapped fields if provided by Registry
		merged := r.mergeArgsWithSource(tasks[taskIdx].ObjectType, tasks[taskIdx].Field, tasks[taskIdx].Source, tasks[taskIdx].Args, itemDesc)
		if err := setMessageFieldsByJSON(item, merged); err != nil {
			res[pos] = executor.AsyncResolveResult{Error: err}
			continue
		}
		list.Append(protoreflect.ValueOfMessage(item))
		included = append(included, pos)
	}
	req.Set(batchesField, protoreflect.ValueOfList(list))

	if len(included) == 0 {
		return res
	}

	respMsg, err := r.transport.Call(ctx, md, req)
	if err != nil {
		for _, pos := range included {
			res[pos] = executor.AsyncResolveResult{Error: err}
		}
		return res
	}

	// Map response back to included positions
	omd := md.Output()
	bf := omd.Fields().ByName("batches")
	if bf == nil {
		for _, pos := range included {
			res[pos] = executor.AsyncResolveResult{Error: fmt.Errorf("missing batches field in response")}
		}
		return res
	}
	batchesOut := respMsg.Get(bf).List()
	for k, pos := range included {
		if k >= batchesOut.Len() {
			res[pos] = executor.AsyncResolveResult{Error: fmt.Errorf("missing batch element")}
			continue
		}
		msg := batchesOut.Get(k).Message()
		if msg == nil {
			res[pos] = executor.AsyncResolveResult{Value: nil}
			continue
		}
		val, herr := r.handleResponse(msg)
		if herr != nil {
			res[pos] = executor.AsyncResolveResult{Error: herr}
		} else {
			res[pos] = executor.AsyncResolveResult{Value: val}
		}
	}
	return res
}

// executeBatchLoader builds and executes a batch loader RPC call.
// It applies null-key short-circuit: if any task has a nil value among its args,
// that task is not included in the RPC and its result is (nil, nil).
func (r *Runtime) executeBatchLoader(ctx context.Context, md protoreflect.MethodDescriptor, tasks []executor.AsyncResolveTask, idxs []int) []executor.AsyncResolveResult {
	res := make([]executor.AsyncResolveResult, len(idxs))
	imd := md.Input()
	batchesField := imd.Fields().ByName("batches")

	req := dynamicpb.NewMessage(imd)
	list := req.Mutable(batchesField).List()
	itemDesc := batchesField.Message()

	// Track included positions within idxs slice
	included := make([]int, 0, len(idxs))
	for pos, taskIdx := range idxs {
		args := r.mergeArgsWithSource(tasks[taskIdx].ObjectType, tasks[taskIdx].Field, tasks[taskIdx].Source, tasks[taskIdx].Args, itemDesc)
		if hasNilInputFields(itemDesc, args) {
			continue // short-circuit
		}
		item := dynamicpb.NewMessage(itemDesc)
		if err := setMessageFieldsByJSON(item, args); err != nil {
			res[pos] = executor.AsyncResolveResult{Error: err}
			continue
		}
		list.Append(protoreflect.ValueOfMessage(item))
		included = append(included, pos)
	}
	req.Set(batchesField, protoreflect.ValueOfList(list))

	if len(included) == 0 {
		return res
	}

	respMsg, err := r.transport.Call(ctx, md, req)
	if err != nil {
		for _, pos := range included {
			res[pos] = executor.AsyncResolveResult{Error: err}
		}
		return res
	}

	// Map response batches back to included tasks order
	omd := md.Output()
	of := omd.Fields().ByName("batches")
	if of == nil {
		for _, pos := range included {
			res[pos] = executor.AsyncResolveResult{Error: fmt.Errorf("missing batches field in response")}
		}
		return res
	}
	batchesOut := respMsg.Get(of).List()
	for k, pos := range included {
		if k >= batchesOut.Len() {
			res[pos] = executor.AsyncResolveResult{Error: fmt.Errorf("missing batch element")}
			continue
		}
		msg := batchesOut.Get(k).Message()
		if msg == nil {
			res[pos] = executor.AsyncResolveResult{Value: nil}
			continue
		}
		val, herr := r.handleResponse(msg)
		if herr != nil {
			res[pos] = executor.AsyncResolveResult{Error: herr}
		} else {
			res[pos] = executor.AsyncResolveResult{Value: val}
		}
	}
	return res
}

// executeSingleLoader executes a single loader call or short-circuits when args contain nil.
func (r *Runtime) executeSingleLoader(ctx context.Context, md protoreflect.MethodDescriptor, task executor.AsyncResolveTask) executor.AsyncResolveResult {
	if hasNilInputFields(md.Input(), task.Args) {
		return executor.AsyncResolveResult{Value: nil}
	}
	return r.executeSingle(ctx, md, task)
}

// hasNilInputFields reports whether any of the input message's JSONName fields
// are present in args with a nil value.
func hasNilInputFields(inputDesc protoreflect.MessageDescriptor, args map[string]any) bool {
	if len(args) == 0 {
		return false
	}
	fields := inputDesc.Fields()
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		if v, ok := args[string(f.JSONName())]; ok && v == nil {
			return true
		}
	}
	return false
}

// executeSingle executes a single RPC resolver call for one async task.
func (r *Runtime) executeSingle(ctx context.Context, md protoreflect.MethodDescriptor, task executor.AsyncResolveTask) executor.AsyncResolveResult {
	req := dynamicpb.NewMessage(md.Input())
	merged := r.mergeArgsWithSource(task.ObjectType, task.Field, task.Source, task.Args, md.Input())
	if err := setMessageFieldsByJSON(req, merged); err != nil {
		return executor.AsyncResolveResult{Error: err}
	}
	respMsg, err := r.transport.Call(ctx, md, req)
	if err != nil {
		return executor.AsyncResolveResult{Error: err}
	}
	val, herr := r.handleResponse(respMsg)
	if herr != nil {
		return executor.AsyncResolveResult{Error: herr}
	}
	return executor.AsyncResolveResult{Value: val}
}

// mergeArgsWithSource augments args by copying fields from the parent source according to
// Registry-provided mapping for (objectType, field). If inputDesc is provided, only keys that
// exist in the input message are considered.
func (r *Runtime) mergeArgsWithSource(objectType, field string, source any, args map[string]any, inputDesc protoreflect.MessageDescriptor) map[string]any {
	if r == nil {
		return args
	}
	mp := r.reg.GetRequestFieldSourceMapping(objectType, field)
	if len(mp) == 0 {
		return args
	}
	out := make(map[string]any, len(args)+len(mp))
	for k, v := range args {
		out[k] = v
	}
	srcMsg, ok := source.(protoreflect.Message)
	if !ok || srcMsg == nil {
		return out
	}
	// Build a quick set of input field JSON names to avoid accidental keys
	inputFields := map[string]struct{}{}
	if inputDesc != nil {
		fs := inputDesc.Fields()
		for i := 0; i < fs.Len(); i++ {
			inputFields[string(fs.Get(i).JSONName())] = struct{}{}
		}
	}
	for dst, src := range mp {
		if _, exists := out[dst]; exists {
			continue
		}
		if _, ok := inputFields[dst]; !ok && len(inputFields) > 0 {
			continue
		}
		// Read from parent source field using Registry
		fd := r.reg.GetSourceFieldDescriptor(objectType, src)
		if fd == nil || !srcMsg.Has(fd) {
			continue
		}
		val := srcMsg.Get(fd)
		// Convert to Go value; setMessageFieldsByJSON will coerce to dest type
		out[dst] = r.handleValue(fd, val)
	}
	return out
}

// handleResponse extracts the top-level "data" field from a response message.
func (r *Runtime) handleResponse(resp protoreflect.Message) (any, error) {
	fd := resp.Descriptor().Fields().ByName("data")
	if fd == nil {
		return nil, fmt.Errorf("missing data field in response")
	}
	// If the singular message field is not present, treat as null (e.g., not found)
	if fd.Cardinality() != protoreflect.Repeated && fd.Kind() == protoreflect.MessageKind {
		if !resp.Has(fd) {
			return nil, nil
		}
	}
	v := resp.Get(fd)
	if fd.Cardinality() == protoreflect.Repeated { // container
		lst := v.List()
		out := make([]any, 0, lst.Len())
		for i := 0; i < lst.Len(); i++ {
			out = append(out, r.handleValue(fd, lst.Get(i)))
		}
		return out, nil
	}
	return r.handleValue(fd, v), nil
}

// handleValue converts a protobuf field value to a Go value for executor consumption.
func (r *Runtime) handleValue(fd protoreflect.FieldDescriptor, v protoreflect.Value) any {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return v.Bool()
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return int32(v.Int())
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return int64(v.Int())
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return uint32(v.Uint())
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return uint64(v.Uint())
	case protoreflect.FloatKind:
		return float32(v.Float())
	case protoreflect.DoubleKind:
		return v.Float()
	case protoreflect.StringKind:
		return v.String()
	case protoreflect.BytesKind:
		return []byte(v.Bytes())
	case protoreflect.EnumKind:
		if ev := fd.Enum().Values().ByNumber(v.Enum()); ev != nil {
			return string(ev.Name())
		}
		return int32(v.Enum())
	case protoreflect.MessageKind:
		msg := v.Message()
		if decoded := r.unwrapInterfaceEnvelope(msg); decoded != nil {
			return decoded
		}
		if union := r.unwrapUnionEnvelope(msg); union != nil {
			return union
		}
		return msg
	default:
		return nil
	}
}

// ResolveType resolves the concrete type of an abstract GraphQL type based on the value.
// It is used to determine the actual GraphQL object type to execute for a given value.
func (r *Runtime) ResolveType(ctx context.Context, abstractType string, value any) (string, error) {
	msg, ok := value.(protoreflect.Message)
	if !ok || msg == nil {
		return "", fmt.Errorf("ResolveType expects protoreflect.Message, got %T", value)
	}
	name := string(msg.Descriptor().Name())
	if len(name) > 6 && name[len(name)-6:] == "Source" {
		return name[:len(name)-6], nil
	}
	return "", fmt.Errorf("cannot infer concrete type from message %s", name)
}

// SerializeLeafValue serializes a scalar or enum value for transport over the wire.
// It handles nil values, basic types, and byte slices (which are base64-encoded).
func (r *Runtime) SerializeLeafValue(ctx context.Context, scalarOrEnumTypeName string, value any) (any, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case string, bool, int, int32, int64, float32, float64:
		return v, nil
	case []byte:
		return base64.StdEncoding.EncodeToString(v), nil
	default:
		return v, nil
	}
}

// ----------------- helpers -----------------

func (r *Runtime) unwrapInterfaceEnvelope(msg protoreflect.Message) protoreflect.Message {
	if r == nil || r.reg == nil || msg == nil {
		return nil
	}
	fields := msg.Descriptor().Fields()
	typenameField := fields.ByName("typename")
	payloadField := fields.ByName("payload")
	if typenameField == nil || payloadField == nil {
		return nil
	}
	if typenameField.Kind() != protoreflect.StringKind || payloadField.Kind() != protoreflect.BytesKind {
		return nil
	}
	if !msg.Has(typenameField) {
		return nil
	}
	if !msg.Has(payloadField) {
		panic(fmt.Sprintf("grpcrt: interface envelope %s missing payload", msg.Descriptor().FullName()))
	}
	typeName := msg.Get(typenameField).String()
	desc := r.reg.GetSourceMessageDescriptor(typeName)
	if desc == nil {
		panic(fmt.Sprintf("grpcrt: missing source message descriptor for %s", typeName))
	}
	payload := msg.Get(payloadField).Bytes()
	out := dynamicpb.NewMessage(desc)
	if err := proto.Unmarshal(payload, out.Interface()); err != nil {
		panic(fmt.Sprintf("grpcrt: failed to unmarshal payload for %s: %v", typeName, err))
	}
	return out
}

func (r *Runtime) unwrapUnionEnvelope(msg protoreflect.Message) protoreflect.Message {
	if msg == nil {
		return nil
	}
	desc := msg.Descriptor()
	if desc == nil || desc.Oneofs().Len() != 1 {
		return nil
	}
	oneofDesc := desc.Oneofs().Get(0)
	if oneofDesc == nil || string(oneofDesc.Name()) != "value" {
		return nil
	}
	fd := msg.WhichOneof(oneofDesc)
	if fd == nil {
		return nil
	}
	if fd.Kind() != protoreflect.MessageKind {
		panic(fmt.Sprintf("grpcrt: union envelope %s has non-message variant %s", desc.FullName(), fd.FullName()))
	}
	if !msg.Has(fd) {
		return nil
	}
	return msg.Get(fd).Message()
}

func setMessageFieldsByJSON(msg protoreflect.Message, data map[string]any) error {
	if data == nil {
		return nil
	}
	fields := msg.Descriptor().Fields()
	// Cache JSONName -> FieldDescriptor to avoid O(n*m) scans
	byJSON := make(map[string]protoreflect.FieldDescriptor, fields.Len())
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		byJSON[string(f.JSONName())] = f
	}
	for k, v := range data {
		// Find field by JSON name (GraphQL arg name)
		fd := byJSON[k]
		if fd == nil {
			continue
		}
		if fd.Cardinality() == protoreflect.Repeated {
			list := msg.Mutable(fd).List()
			switch vv := v.(type) {
			case []any:
				for _, it := range vv {
					pv, err := toProtoScalarOrMessage(fd, it)
					if err != nil {
						return err
					}
					list.Append(pv)
				}
			case []string:
				for _, s := range vv {
					list.Append(protoreflect.ValueOfString(s))
				}
			case []int:
				for _, n := range vv {
					list.Append(protoreflect.ValueOfInt64(int64(n)))
				}
			case []int32:
				for _, n := range vv {
					list.Append(protoreflect.ValueOfInt32(n))
				}
			case []int64:
				for _, n := range vv {
					list.Append(protoreflect.ValueOfInt64(n))
				}
			case []float32:
				for _, n := range vv {
					list.Append(protoreflect.ValueOfFloat32(n))
				}
			case []float64:
				for _, n := range vv {
					list.Append(protoreflect.ValueOfFloat64(n))
				}
			case []bool:
				for _, b := range vv {
					list.Append(protoreflect.ValueOfBool(b))
				}
			default:
				return fmt.Errorf("unsupported repeated arg type for %s", fd.JSONName())
			}
			msg.Set(fd, protoreflect.ValueOfList(list))
			continue
		}
		val, err := toProtoScalarOrMessage(fd, v)
		if err != nil {
			return err
		}
		msg.Set(fd, val)
	}
	return nil
}

func toProtoScalarOrMessage(fd protoreflect.FieldDescriptor, v any) (protoreflect.Value, error) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		if b, ok := v.(bool); ok {
			return protoreflect.ValueOfBool(b), nil
		}
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		if n, ok := v.(int32); ok {
			return protoreflect.ValueOfInt32(n), nil
		}
		if n, ok := v.(int); ok {
			return protoreflect.ValueOfInt32(int32(n)), nil
		}
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		if n, ok := v.(int64); ok {
			return protoreflect.ValueOfInt64(n), nil
		}
		if n, ok := v.(int); ok {
			return protoreflect.ValueOfInt64(int64(n)), nil
		}
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		if n, ok := v.(uint32); ok {
			return protoreflect.ValueOfUint32(n), nil
		}
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		if n, ok := v.(uint64); ok {
			return protoreflect.ValueOfUint64(n), nil
		}
	case protoreflect.FloatKind:
		if n, ok := v.(float32); ok {
			return protoreflect.ValueOfFloat32(n), nil
		}
		if n, ok := v.(float64); ok {
			return protoreflect.ValueOfFloat32(float32(n)), nil
		}
	case protoreflect.DoubleKind:
		if n, ok := v.(float64); ok {
			return protoreflect.ValueOfFloat64(n), nil
		}
	case protoreflect.StringKind:
		if s, ok := v.(string); ok {
			return protoreflect.ValueOfString(s), nil
		}
	case protoreflect.BytesKind:
		if b, ok := v.([]byte); ok {
			return protoreflect.ValueOfBytes(b), nil
		}
	case protoreflect.EnumKind:
		// Minimal: accept string enum name
		if s, ok := v.(string); ok {
			val := fd.Enum().Values().ByName(protoreflect.Name(s))
			if val != nil {
				return protoreflect.ValueOfEnum(val.Number()), nil
			}
		}
	case protoreflect.MessageKind:
		if mv, ok := v.(map[string]any); ok {
			msg := dynamicpb.NewMessage(fd.Message())
			if err := setMessageFieldsByJSON(msg, mv); err != nil {
				return protoreflect.Value{}, err
			}
			return protoreflect.ValueOfMessage(msg), nil
		}
	}
	return protoreflect.Value{}, fmt.Errorf("unsupported arg type for %s", fd.JSONName())
}
