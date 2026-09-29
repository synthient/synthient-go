package grpcschema

import (
	"context"
	"errors"
	"net"
	"slices"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/descriptorpb"
)

// startServer starts a loopback gRPC server with reflection and the health
// service. It returns the server address and a channel that receives the
// x-api-key metadata of each reflection stream.
func startServer(t *testing.T) (addr string, keys <-chan []string) {
	t.Helper()
	var config net.ListenConfig
	listener, err := config.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	seen := make(chan []string, 10)
	server := grpc.NewServer(grpc.StreamInterceptor(func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		md, _ := metadata.FromIncomingContext(ss.Context())
		seen <- md.Get("x-api-key")
		return handler(srv, ss)
	}))
	healthpb.RegisterHealthServer(server, health.NewServer())
	reflection.Register(server)
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(server.Stop)
	return listener.Addr().String(), seen
}

func TestFetchAllServices(t *testing.T) {
	t.Parallel()
	addr, keys := startServer(t)

	schema, err := Fetch(t.Context(), "test-key", &Options{Endpoint: addr, Plaintext: true})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if schema.Endpoint != addr {
		t.Errorf("Endpoint = %q, want %q", schema.Endpoint, addr)
	}
	if !slices.Equal(schema.Symbols, []string{"grpc.health.v1.Health"}) {
		t.Errorf("Symbols = %v, want only the health service", schema.Symbols)
	}
	if !hasFile(schema, "grpc/health/v1/health.proto") {
		t.Errorf("descriptor set is missing grpc/health/v1/health.proto")
	}
	key := <-keys
	if !slices.Equal(key, []string{"test-key"}) {
		t.Errorf("x-api-key metadata = %v, want [test-key]", key)
	}
}

func TestFetchOptions(t *testing.T) {
	t.Parallel()
	addr, keys := startServer(t)

	schema, err := Fetch(t.Context(), "", &Options{Endpoint: "grpc://" + addr, Plaintext: true, IncludeReflection: true})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !slices.Contains(schema.Symbols, "grpc.reflection.v1.ServerReflection") {
		t.Errorf("Symbols = %v, want the reflection service included", schema.Symbols)
	}
	key := <-keys
	if len(key) != 0 {
		t.Errorf("x-api-key metadata = %v, want none for an empty key", key)
	}

	schema, err = Fetch(t.Context(), "test-key", &Options{
		Endpoint:  addr,
		Plaintext: true,
		Symbols:   []string{" grpc.health.v1.HealthCheckRequest ", "grpc.health.v1.HealthCheckRequest"},
	})
	if err != nil {
		t.Fatalf("Fetch with symbols: %v", err)
	}
	if !slices.Equal(schema.Symbols, []string{"grpc.health.v1.HealthCheckRequest"}) {
		t.Errorf("Symbols = %v, want the cleaned symbol list", schema.Symbols)
	}
}

func TestFetchUnknownSymbol(t *testing.T) {
	t.Parallel()
	addr, _ := startServer(t)

	_, err := Fetch(t.Context(), "test-key", &Options{Endpoint: addr, Plaintext: true, Symbols: []string{"synthient.v1.Missing"}})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("Fetch error = %v, want NotFound", err)
	}
	if Explain(err) == "" {
		t.Error("Explain returned no explanation for NotFound")
	}
}

func TestFetchCanceled(t *testing.T) {
	t.Parallel()
	addr, _ := startServer(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := Fetch(ctx, "test-key", &Options{Endpoint: addr, Plaintext: true})
	if status.Code(err) != codes.Canceled {
		t.Errorf("Fetch error = %v, want Canceled", err)
	}
}

func TestNormalizeEndpoint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw      string
		endpoint string
		host     string
	}{
		{raw: "", endpoint: DefaultEndpoint, host: "grpc.synthient.com"},
		{raw: "grpc.synthient.com", endpoint: "grpc.synthient.com:443", host: "grpc.synthient.com"},
		{raw: "localhost:50051/", endpoint: "localhost:50051", host: "localhost"},
		{raw: "grpc://localhost:50051", endpoint: "localhost:50051", host: "localhost"},
		{raw: "https://grpc.synthient.com", endpoint: "grpc.synthient.com:443", host: "grpc.synthient.com"},
		{raw: "[::1]:9000", endpoint: "[::1]:9000", host: "::1"},
		{raw: "::1", endpoint: "[::1]:443", host: "::1"},
	}
	for _, tt := range tests {
		endpoint, host, err := NormalizeEndpoint(tt.raw)
		if err != nil || endpoint != tt.endpoint || host != tt.host {
			t.Errorf("NormalizeEndpoint(%q) = %q, %q, %v, want %q, %q", tt.raw, endpoint, host, err, tt.endpoint, tt.host)
		}
	}

	_, _, err := NormalizeEndpoint("grpc://")
	if err == nil {
		t.Error("NormalizeEndpoint(grpc://) succeeded, want an error")
	}
}

func TestExplain(t *testing.T) {
	t.Parallel()
	explained := []codes.Code{codes.Unauthenticated, codes.PermissionDenied, codes.NotFound, codes.Unimplemented, codes.Unavailable, codes.DeadlineExceeded}
	for _, code := range explained {
		if Explain(status.Error(code, "x")) == "" {
			t.Errorf("Explain(%v) is empty", code)
		}
	}
	if Explain(status.Error(codes.Internal, "x")) != "" {
		t.Error("Explain(Internal) should be empty")
	}
	if Explain(errors.New("plain error")) != "" {
		t.Error("Explain(plain error) should be empty")
	}
}

func TestOrderedFiles(t *testing.T) {
	t.Parallel()
	files := map[string]*descriptorpb.FileDescriptorProto{
		"a.proto": {Name: new("a.proto"), Dependency: []string{"b.proto", "c.proto"}},
		"b.proto": {Name: new("b.proto"), Dependency: []string{"c.proto"}},
		"c.proto": {Name: new("c.proto")},
		"d.proto": {Name: new("d.proto"), Dependency: []string{"missing.proto"}},
	}

	var names []string
	for _, file := range orderedFiles(files) {
		names = append(names, file.GetName())
	}
	want := []string{"c.proto", "b.proto", "a.proto", "d.proto"}
	if !slices.Equal(names, want) {
		t.Errorf("orderedFiles = %v, want %v", names, want)
	}
	if !slices.Equal(missingDependencies(files), []string{"missing.proto"}) {
		t.Errorf("missingDependencies = %v, want [missing.proto]", missingDependencies(files))
	}
}

func hasFile(schema *Schema, name string) bool {
	for _, file := range schema.DescriptorSet.GetFile() {
		if file.GetName() == name {
			return true
		}
	}
	return false
}
