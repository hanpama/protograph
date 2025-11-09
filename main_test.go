package main

// import (
// 	"bytes"
// 	"context"
// 	"io"
// 	"net"
// 	"net/http"
// 	"os"
// 	"os/exec"
// 	"path/filepath"
// 	"strings"
// 	"testing"
// 	"time"

// 	pb "github.com/hanpama/protograph/tests/simple/server/grpcproto"
// 	"github.com/stretchr/testify/require"
// 	grpc "google.golang.org/grpc"
// )

// type stubUserService struct {
// 	pb.UnimplementedUserServiceServer
// }

// func (stubUserService) ResolveQueryUsers(ctx context.Context, req *pb.ResolveQueryUsersRequest) (*pb.ResolveQueryUsersResponse, error) {
// 	return &pb.ResolveQueryUsersResponse{}, nil
// }

// func captureOutput(t *testing.T, fn func() error) (stdout, stderr string, err error) {
// 	t.Helper()
// 	oldOut, oldErr := os.Stdout, os.Stderr
// 	defer func() {
// 		os.Stdout, os.Stderr = oldOut, oldErr
// 	}()

// 	outR, outW, _ := os.Pipe()
// 	errR, errW, _ := os.Pipe()
// 	os.Stdout, os.Stderr = outW, errW

// 	doneOut := make(chan struct{})
// 	var bufOut bytes.Buffer
// 	go func() { io.Copy(&bufOut, outR); close(doneOut) }()

// 	doneErr := make(chan struct{})
// 	var bufErr bytes.Buffer
// 	go func() { io.Copy(&bufErr, errR); close(doneErr) }()

// 	err = fn()
// 	outW.Close()
// 	errW.Close()
// 	<-doneOut
// 	<-doneErr
// 	stdout, stderr = bufOut.String(), bufErr.String()
// 	return
// }

// func TestHelp(t *testing.T) {
// 	out, _, err := captureOutput(t, func() error {
// 		return run([]string{"help", "serve"})
// 	})
// 	require.NoError(t, err)
// 	require.Contains(t, out, "serve FLAGS")
// }

// func TestCompileSDL(t *testing.T) {
// 	root := filepath.Join("..", "..", "tests", "simple")
// 	out, _, err := captureOutput(t, func() error {
// 		return run([]string{"compile-sdl", "-root", root, "-rootpkg", "simple"})
// 	})
// 	require.NoError(t, err)
// 	require.Contains(t, out, "type Query")
// }

// func TestCompileProto(t *testing.T) {
// 	root := filepath.Join("..", "..", "tests", "simple")
// 	outDir := t.TempDir()
// 	err := run([]string{"compile-proto", "-root", root, "-rootpkg", "simple", "-out", outDir})
// 	require.NoError(t, err)
// 	if _, err := os.Stat(filepath.Join(outDir, "graphql", "user.proto")); err != nil {
// 		t.Fatalf("expected proto file: %v", err)
// 	}
// }

// func TestServe(t *testing.T) {
// 	// start stub gRPC service
// 	lis, err := net.Listen("tcp", "127.0.0.1:0")
// 	require.NoError(t, err)
// 	grpcAddr := lis.Addr().String()
// 	gsrv := grpc.NewServer()
// 	pb.RegisterUserServiceServer(gsrv, stubUserService{})
// 	go gsrv.Serve(lis)
// 	defer func() {
// 		gsrv.Stop()
// 		lis.Close()
// 	}()

// 	// choose HTTP port
// 	hLis, err := net.Listen("tcp", "127.0.0.1:0")
// 	require.NoError(t, err)
// 	addr := hLis.Addr().String()
// 	hLis.Close()

// 	root := filepath.Join("..", "..", "tests", "simple")

// 	// build CLI binary
// 	bin := filepath.Join(t.TempDir(), "protograph-test-bin")
// 	build := exec.Command("go", "build", "-o", bin, ".")
// 	build.Stdout = &bytes.Buffer{}
// 	build.Stderr = &bytes.Buffer{}
// 	require.NoError(t, build.Run())

// 	ctx, cancel := context.WithCancel(context.Background())
// 	cmd := exec.CommandContext(ctx, bin, "serve", "-root", root, "-rootpkg", "simple", "-addr", addr, "-backend", "*="+grpcAddr)
// 	cmd.Stdout = &bytes.Buffer{}
// 	cmd.Stderr = &bytes.Buffer{}
// 	require.NoError(t, cmd.Start())

// 	url := "http://" + addr + "/graphql"
// 	require.Eventually(t, func() bool {
// 		resp, err := http.Post(url, "application/json", strings.NewReader(`{"query":"{ users { id } }"}`))
// 		if err != nil {
// 			return false
// 		}
// 		defer resp.Body.Close()
// 		b, _ := io.ReadAll(resp.Body)
// 		return strings.Contains(string(b), "\"users\":")
// 	}, 10*time.Second, 200*time.Millisecond)

// 	cancel()
// 	cmd.Wait()
// }
