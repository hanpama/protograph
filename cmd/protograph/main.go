package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hanpama/protograph/internal/eventbus"
	"github.com/hanpama/protograph/internal/grpcrt"
	"github.com/hanpama/protograph/internal/grpctp"
	"github.com/hanpama/protograph/internal/introspection"
	"github.com/hanpama/protograph/internal/ir"
	"github.com/hanpama/protograph/internal/otel"
	"github.com/hanpama/protograph/internal/protoreg"
	"github.com/hanpama/protograph/internal/schema"
	"github.com/hanpama/protograph/internal/server"
)

const rootUsage = `protograph — GraphQL ↔ gRPC bridge & tools

USAGE:
  protograph <command> [flags]

COMMANDS:
  serve            Run the HTTP GraphQL gateway backed by gRPC services
  compile-sdl      Merge & validate GraphQL SDL into a single schema
  compile-proto    Generate .proto files from the GraphQL project
  help             Show help for any command
`

const serveUsage = `serve FLAGS:
  -graphql.root <dir>                 GraphQL schema root (default: .)
  -graphql.rootpkg <name>             GraphQL root package (required)
  -graphql.introspection <bool>       Enable GraphQL introspection (default: true)
  -server.addr <addr>                 HTTP listen address (default: :8080)
  -server.pretty                      Pretty-print JSON responses
  -server.timeout <duration>          Per-request timeout, e.g. 10s (default: 10s)
  -server.metadata-header <name>      Forward HTTP header to gRPC metadata. Repeatable
  -transport.backend <Svc=host:port>  Map gRPC service to endpoint. Repeatable; at least
                                      one mapping required. Use wildcard to set default:
                                        -transport.backend *=host:port
                                      Specific mappings override the wildcard.
  -transport.max-conns-per-endpoint N Max TCP conns per endpoint (default: 2)
  -transport.rpc-timeout <duration>   RPC timeout, e.g. 3s (default: 3s)
  -otel.endpoint <addr>               OTLP collector endpoint
  -otel.service <name>                OpenTelemetry service name (default: protograph)
`

const compileSDLUsage = `compile-sdl FLAGS:
  -graphql.root <dir>      GraphQL project root (default: .)
  -graphql.rootpkg <name>  GraphQL root package (required)
  -out  <file>             Write compiled SDL to file (default: stdout)
  (Validation always runs; exits non-zero on errors)
`

const compileProtoUsage = `compile-proto FLAGS:
  -graphql.root <dir>      GraphQL project root (default: .)
  -graphql.rootpkg <name>  GraphQL root package (required)
  -out  <dir>              Output directory for generated .proto files (required)
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	global := flag.NewFlagSet("protograph", flag.ContinueOnError)
	global.SetOutput(new(bytes.Buffer)) // silence automatic output
	if err := global.Parse(args); err != nil {
		// print usage on parse error
		fmt.Fprint(os.Stderr, rootUsage)
		return err
	}
	remaining := global.Args()
	if len(remaining) == 0 {
		fmt.Fprint(os.Stderr, rootUsage)
		return fmt.Errorf("missing command")
	}

	cmd := remaining[0]
	cmdArgs := remaining[1:]
	switch cmd {
	case "serve":
		return cmdServe(cmdArgs)
	case "compile-sdl":
		return cmdCompileSDL(cmdArgs)
	case "compile-proto":
		return cmdCompileProto(cmdArgs)
	case "help":
		return cmdHelp(cmdArgs)
	default:
		fmt.Fprint(os.Stderr, rootUsage)
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func cmdHelp(args []string) error {
	if len(args) == 0 {
		fmt.Print(rootUsage)
		return nil
	}
	switch args[0] {
	case "serve":
		fmt.Print(serveUsage)
	case "compile-sdl":
		fmt.Print(compileSDLUsage)
	case "compile-proto":
		fmt.Print(compileProtoUsage)
	default:
		return fmt.Errorf("unknown help topic %q", args[0])
	}
	return nil
}

type backendFlag struct {
	m map[string][]string
}

func (b *backendFlag) String() string { return "" }

func (b *backendFlag) Set(v string) error {
	parts := strings.SplitN(v, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid backend %q", v)
	}
	svc := strings.TrimSpace(parts[0])
	ep := strings.TrimSpace(parts[1])
	if svc == "" || ep == "" {
		return fmt.Errorf("invalid backend %q", v)
	}
	if b.m == nil {
		b.m = map[string][]string{}
	}
	b.m[svc] = append(b.m[svc], ep)
	return nil
}

type stringListFlag []string

func (s *stringListFlag) String() string { return "" }

func (s *stringListFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func cmdServe(args []string) error {
	// Defaults mirror the old config defaults for consistency
	rootDir := "."
	rootPkg := ""
	addr := ":8080"
	pretty := false
	timeout := 10 * time.Second
	maxConns := 2
	rpcTimeout := 3 * time.Second
	enableIntrospection := true
	otelEndpoint := ""
	otelService := "protograph"
	backends := map[string][]string{}
	var metadataHeaders stringListFlag

	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(new(bytes.Buffer))
	fs.StringVar(&rootDir, "graphql.root", rootDir, "GraphQL schema root")
	fs.StringVar(&rootPkg, "graphql.rootpkg", rootPkg, "GraphQL root package")
	fs.BoolVar(&enableIntrospection, "graphql.introspection", enableIntrospection, "Enable GraphQL introspection")
	fs.StringVar(&addr, "server.addr", addr, "HTTP listen address")
	fs.BoolVar(&pretty, "server.pretty", pretty, "Pretty-print JSON responses")
	fs.DurationVar(&timeout, "server.timeout", timeout, "Per-request timeout")
	fs.Var(&metadataHeaders, "server.metadata-header", "Forward HTTP header to gRPC metadata")
	var bf backendFlag
	fs.Var(&bf, "transport.backend", "Map gRPC service to endpoint")
	fs.IntVar(&maxConns, "transport.max-conns-per-endpoint", maxConns, "Max conns per endpoint")
	fs.DurationVar(&rpcTimeout, "transport.rpc-timeout", rpcTimeout, "RPC timeout")
	fs.StringVar(&otelEndpoint, "otel.endpoint", otelEndpoint, "OTLP collector endpoint")
	fs.StringVar(&otelService, "otel.service", otelService, "OpenTelemetry service name")
	if err := fs.Parse(args); err != nil {
		fmt.Fprint(os.Stderr, serveUsage)
		return err
	}
	if rootPkg == "" {
		fmt.Fprint(os.Stderr, serveUsage)
		return fmt.Errorf("-graphql.rootpkg is required")
	}
	for svc, eps := range bf.m {
		backends[svc] = eps
	}

	proj, err := ir.Load(rootDir, rootPkg)
	if err != nil {
		return fmt.Errorf("load project: %w", err)
	}
	reg, err := protoreg.Build(proj)
	if err != nil {
		return fmt.Errorf("protoreg build: %w", err)
	}

	wildcard := backends["*"]
	providers := map[string][]string{}
	for _, fd := range reg.GetAllServiceFiles() {
		for i := range fd.Services().Len() {
			svc := fd.Services().Get(i)
			fn := string(svc.FullName())

			eps := backends[fn]
			if len(eps) == 0 {
				eps = wildcard
			}
			if len(eps) == 0 {
				return fmt.Errorf("no backend mapping for %s", svc)
			}
			providers[fn] = eps
		}
	}
	if len(providers) == 0 {
		return fmt.Errorf("no backend mappings provided")
	}
	provider := grpctp.NewStaticEndpoints(providers)

	eventbus.Use(eventbus.New())
	shutdown, err := otel.Setup(otelEndpoint, otelService)
	if err != nil {
		return fmt.Errorf("otel setup: %w", err)
	}
	defer func() { _ = shutdown(context.Background()) }()
	trOpts := []grpctp.Option{grpctp.WithProvider(provider), grpctp.WithMaxConnsPerEndpoint(maxConns)}
	if rpcTimeout > 0 {
		trOpts = append(trOpts, grpctp.WithRPCTimeout(rpcTimeout))
	}
	transport := grpctp.New(trOpts...)
	runtime := grpcrt.NewRuntime(reg, transport)

	sch, err := schema.BuildFromIR(proj)
	if err != nil {
		return fmt.Errorf("build schema: %w", err)
	}

	// Only wrap with introspection if enabled
	if enableIntrospection {
		var wrapper *introspection.IntrospectionWrapper = introspection.Wrap(runtime, sch)
		runtime = wrapper.Runtime
		sch = wrapper.Schema
	}

	var sopts []server.Option
	if pretty {
		sopts = append(sopts, server.WithPretty())
	}
	if timeout > 0 {
		sopts = append(sopts, server.WithTimeout(timeout))
	}
	if len(metadataHeaders) > 0 {
		sopts = append(sopts, server.WithMetadataHeaders(metadataHeaders...))
	}
	h, err := server.New(runtime, sch, sopts...)
	if err != nil {
		return fmt.Errorf("server init: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/graphql", h)

	log.Printf("GraphQL server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func cmdCompileSDL(args []string) error {
	rootDir := "."
	rootPkg := ""
	outFile := ""
	fs := flag.NewFlagSet("compile-sdl", flag.ContinueOnError)
	fs.SetOutput(new(bytes.Buffer))
	fs.StringVar(&rootDir, "graphql.root", rootDir, "GraphQL project root")
	fs.StringVar(&rootPkg, "graphql.rootpkg", rootPkg, "GraphQL root package")
	fs.StringVar(&outFile, "out", outFile, "Write compiled SDL to file")
	if err := fs.Parse(args); err != nil {
		fmt.Fprint(os.Stderr, compileSDLUsage)
		return err
	}
	if rootPkg == "" {
		fmt.Fprint(os.Stderr, compileSDLUsage)
		return fmt.Errorf("-graphql.rootpkg is required")
	}

	proj, err := ir.Load(rootDir, rootPkg)
	if err != nil {
		return fmt.Errorf("load project: %w", err)
	}
	sch, err := schema.BuildFromIR(proj)
	if err != nil {
		return fmt.Errorf("build schema: %w", err)
	}
	sdl := schema.Render(sch)
	if outFile == "" {
		fmt.Print(sdl)
		return nil
	}
	if err := os.WriteFile(outFile, []byte(sdl), 0644); err != nil {
		return err
	}
	return nil
}

func cmdCompileProto(args []string) error {
	rootDir := "."
	rootPkg := ""
	outDir := ""
	fs := flag.NewFlagSet("compile-proto", flag.ContinueOnError)
	fs.SetOutput(new(bytes.Buffer))
	fs.StringVar(&rootDir, "graphql.root", rootDir, "GraphQL project root")
	fs.StringVar(&rootPkg, "graphql.rootpkg", rootPkg, "GraphQL root package")
	fs.StringVar(&outDir, "out", outDir, "Output directory for generated .proto files")
	if err := fs.Parse(args); err != nil {
		fmt.Fprint(os.Stderr, compileProtoUsage)
		return err
	}
	if outDir == "" {
		return fmt.Errorf("-out is required")
	}
	if rootPkg == "" {
		fmt.Fprint(os.Stderr, compileProtoUsage)
		return fmt.Errorf("-graphql.rootpkg is required")
	}
	proj, err := ir.Load(rootDir, rootPkg)
	if err != nil {
		return fmt.Errorf("load project: %w", err)
	}
	reg, err := protoreg.Build(proj)
	if err != nil {
		return fmt.Errorf("protoreg build: %w", err)
	}
	if err := protoreg.Render(reg, outDir); err != nil {
		return fmt.Errorf("render proto: %w", err)
	}
	return nil
}
