package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	eventbus "github.com/hanpama/protograph/internal/eventbus"
	events "github.com/hanpama/protograph/internal/events"
	executor "github.com/hanpama/protograph/internal/executor"
	language "github.com/hanpama/protograph/internal/language"
	reqid "github.com/hanpama/protograph/internal/reqid"
	schema "github.com/hanpama/protograph/internal/schema"
	"google.golang.org/grpc/metadata"
)

// Handler is an http.Handler that serves a GraphQL endpoint.
// It parses requests, runs the executor, and formats responses per GraphQL spec.
type Handler struct {
	exec *executor.Executor
	opt  Options
}

type Options struct {
	// Timeout sets a default timeout if the incoming request context has none.
	// 0 means no default timeout.
	Timeout time.Duration

	// Pretty enables indented JSON responses (useful for dev).
	Pretty bool

	// MaxBodyBytes limits the size of the request body. 0 means unlimited.
	MaxBodyBytes int64

	// CORS configuration. If AllowedOrigins is empty, CORS is disabled.
	CORS CORSOptions

	// MetadataHeaders lists HTTP headers to forward into gRPC metadata.
	// Header names are case-insensitive. Default is none.
	MetadataHeaders []string

	// GraphiQL enables the in-browser IDE when true.
	GraphiQL bool
}

type Option func(*Options)

func WithTimeout(d time.Duration) Option { return func(o *Options) { o.Timeout = d } }
func WithPretty() Option                 { return func(o *Options) { o.Pretty = true } }
func WithMaxBodyBytes(n int64) Option    { return func(o *Options) { o.MaxBodyBytes = n } }
func WithCORS(origins ...string) Option {
	return func(o *Options) { o.CORS.AllowedOrigins = origins }
}
func WithMetadataHeaders(headers ...string) Option {
	return func(o *Options) { o.MetadataHeaders = headers }
}

// CORSOptions holds simple CORS settings.
type CORSOptions struct {
	AllowedOrigins []string
}

func WithGraphiQL(enable bool) Option { return func(o *Options) { o.GraphiQL = enable } }

// New creates a new GraphQL HTTP handler using the given runtime and schema.
func New(runtime executor.Runtime, schema *schema.Schema, opts ...Option) (*Handler, error) {
	exec := executor.NewExecutor(runtime, schema)
	op := Options{Timeout: 10 * time.Second, GraphiQL: true}
	for _, f := range opts {
		f(&op)
	}
	return &Handler{exec: exec, opt: op}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if _, ok := ctx.Deadline(); !ok && h.opt.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.opt.Timeout)
		defer cancel()
	}

	ctx, rid := reqid.NewContext(ctx)
	status := http.StatusOK
	start := time.Now()
	eventbus.Publish(ctx, events.HTTPStart{Request: r})
	defer func() {
		eventbus.Publish(ctx, events.HTTPFinish{Request: r, Status: status, Duration: time.Since(start)})
	}()

	if r.Method == http.MethodOptions {
		if len(h.opt.CORS.AllowedOrigins) > 0 {
			setCORSHeaders(w, r, h.opt.CORS)
		}
		status = http.StatusNoContent
		w.WriteHeader(status)
		return
	}

	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		status = http.StatusMethodNotAllowed
		writeJSON(w, status, errorResponse(nil, &language.Error{Message: "method not allowed"}), h.opt.Pretty)
		return
	}

	// Serve GraphiQL IDE when enabled and the client expects HTML.
	if r.Method == http.MethodGet && h.opt.GraphiQL && acceptsHTML(r.Header.Get("Accept")) && r.URL.Query().Get("query") == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(graphiqlPage)
		return
	}

	// Map configured headers into metadata
	md := metadata.MD{}
	if len(h.opt.MetadataHeaders) > 0 {
		allowed := make(map[string]struct{}, len(h.opt.MetadataHeaders))
		for _, hdr := range h.opt.MetadataHeaders {
			allowed[strings.ToLower(hdr)] = struct{}{}
		}
		for k, v := range r.Header {
			if _, ok := allowed[strings.ToLower(k)]; ok {
				md[strings.ToLower(k)] = v
			}
		}
	}
	md["graphql-request-id"] = []string{strconv.FormatInt(rid, 10)}
	ctx = metadata.NewOutgoingContext(ctx, md)

	req, batch, berr := parseRequest(r, h.opt.MaxBodyBytes)
	if berr != nil {
		status = http.StatusBadRequest
		if berr.Message == errBodyTooLargeMessage {
			status = http.StatusRequestEntityTooLarge
		}
		writeJSON(w, status, errorResponse(nil, berr), h.opt.Pretty)
		return
	}

	if len(h.opt.CORS.AllowedOrigins) > 0 {
		setCORSHeaders(w, r, h.opt.CORS)
	}

	if batch != nil {
		// Batched requests
		op := make([]any, len(batch))
		for i := range batch {
			res := h.executeOne(ctx, batch[i])
			op[i] = res
		}
		writeJSON(w, status, op, h.opt.Pretty)
		return
	}

	res := h.executeOne(ctx, req)
	writeJSON(w, status, res, h.opt.Pretty)
}

func (h *Handler) executeOne(ctx context.Context, req GraphQLRequest) any {
	// Parse query (syntax validation)
	doc, err := language.ParseQuery(req.Query)
	if err != nil {
		if ge, ok := err.(*language.Error); ok {
			return errorResponse(nil, ge)
		}
		return errorResponse(nil, &language.Error{Message: err.Error()})
	}

	opDef := doc.Operations.ForName(req.OperationName)
	if opDef == nil && len(doc.Operations) == 1 {
		opDef = doc.Operations[0]
	}
	opType := ""
	if opDef != nil {
		opType = string(opDef.Operation)
	}

	start := time.Now()
	eventbus.Publish(ctx, events.GraphQLStart{Query: req.Query, OperationName: req.OperationName, OperationType: opType})
	result := h.exec.ExecuteRequest(ctx, doc, req.OperationName, req.Variables, nil)
	errs := make([]error, len(result.Errors))
	for i := range result.Errors {
		errs[i] = result.Errors[i]
	}
	eventbus.Publish(ctx, events.GraphQLFinish{
		Query:         req.Query,
		OperationName: req.OperationName,
		OperationType: opType,
		Errors:        errs,
		Duration:      time.Since(start),
	})
	if len(result.Errors) > 0 {
		return toSpecResult(result)
	}
	return result
}

// ------------------ Request parsing ------------------

type GraphQLRequest struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName,omitempty"`
	Variables     map[string]any `json:"variables,omitempty"`
	Extensions    map[string]any `json:"extensions,omitempty"`
}

func parseRequest(r *http.Request, maxBody int64) (GraphQLRequest, []GraphQLRequest, *language.Error) {
	if r.Method == http.MethodGet {
		q := r.URL.Query().Get("query")
		if q == "" {
			return GraphQLRequest{}, nil, &language.Error{Message: "missing 'query'"}
		}
		vars := map[string]any{}
		if v := r.URL.Query().Get("variables"); v != "" {
			if err := json.Unmarshal([]byte(v), &vars); err != nil {
				return GraphQLRequest{}, nil, &language.Error{Message: "invalid 'variables' JSON"}
			}
		}
		op := r.URL.Query().Get("operationName")
		return GraphQLRequest{Query: q, Variables: vars, OperationName: op}, nil, nil
	}

	// POST
	ct := r.Header.Get("Content-Type")
	if ct == "" || ct == "application/json" || startsWith(ct, "application/json;") {
		reader := io.Reader(r.Body)
		if maxBody > 0 {
			reader = io.LimitReader(r.Body, maxBody+1)
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			return GraphQLRequest{}, nil, &language.Error{Message: "failed to read body"}
		}
		defer r.Body.Close()
		if maxBody > 0 && int64(len(body)) > maxBody {
			return GraphQLRequest{}, nil, &language.Error{Message: errBodyTooLargeMessage}
		}

		// Try array (batch)
		var arr []GraphQLRequest
		if len(body) > 0 && body[0] == '[' {
			if err := json.Unmarshal(body, &arr); err != nil {
				return GraphQLRequest{}, nil, &language.Error{Message: "invalid JSON"}
			}
			if len(arr) == 0 {
				return GraphQLRequest{}, nil, &language.Error{Message: "empty batch"}
			}
			return GraphQLRequest{}, arr, nil
		}
		// Single
		var req GraphQLRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return GraphQLRequest{}, nil, &language.Error{Message: "invalid JSON"}
		}
		if req.Query == "" {
			return GraphQLRequest{}, nil, &language.Error{Message: "missing 'query'"}
		}
		if req.Variables == nil {
			req.Variables = map[string]any{}
		}
		return req, nil, nil
	}

	return GraphQLRequest{}, nil, &language.Error{Message: "unsupported Content-Type"}
}

// ------------------ Response formatting ------------------

type specLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type specError struct {
	Message    string         `json:"message"`
	Locations  []specLocation `json:"locations,omitempty"`
	Path       []any          `json:"path,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

type specResult struct {
	Data   any         `json:"data"`
	Errors []specError `json:"errors,omitempty"`
}

func errorResponse(data any, err *language.Error) specResult {
	se := specError{Message: err.Message}
	return specResult{Data: data, Errors: []specError{se}}
}

func toSpecResult(res *executor.ExecutionResult) specResult {
	out := specResult{Data: res.Data}
	if len(res.Errors) == 0 {
		return out
	}
	out.Errors = make([]specError, len(res.Errors))
	for i, e := range res.Errors {
		se := specError{Message: e.Message, Extensions: e.Extensions}
		// Path
		if len(e.Path) > 0 {
			se.Path = make([]any, len(e.Path))
			for j, pe := range e.Path {
				switch v := pe.(type) {
				case string:
					se.Path[j] = v
				case int:
					se.Path[j] = v
				default:
					se.Path[j] = toString(v)
				}
			}
		}
		out.Errors[i] = se
	}
	// Per spec, when errors present, data may still be partially present; we preserve it.
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any, pretty bool) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	if pretty {
		enc.SetIndent("", "  ")
	}
	_ = enc.Encode(v)
}

func startsWith(s, prefix string) bool { return len(s) >= len(prefix) && s[:len(prefix)] == prefix }
func toString(v any) string            { b, _ := json.Marshal(v); return string(b) }

const errBodyTooLargeMessage = "body too large"

func setCORSHeaders(w http.ResponseWriter, r *http.Request, opts CORSOptions) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	allowed := false
	for _, o := range opts.AllowedOrigins {
		if o == "*" || o == origin {
			allowed = true
			break
		}
	}
	if !allowed {
		return
	}
	if contains(opts.AllowedOrigins, "*") {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
	}
	if r.Method == http.MethodOptions {
		if hdr := r.Header.Get("Access-Control-Request-Headers"); hdr != "" {
			w.Header().Set("Access-Control-Allow-Headers", hdr)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func acceptsHTML(accept string) bool {
	if accept == "" {
		return false
	}
	parts := strings.Split(accept, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if startsWith(p, "text/html") || p == "*/*" {
			return true
		}
	}
	return false
}
