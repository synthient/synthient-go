// Package grpcschema fetches the protobuf schema of the Synthient gRPC API
// using gRPC server reflection.
package grpcschema

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/url"
	"slices"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// DefaultEndpoint is the address of the Synthient gRPC API.
const DefaultEndpoint = "grpc.synthient.com:443"

const maxMessageSize = 32 << 20

// Options configures Fetch. The zero value fetches every service from
// DefaultEndpoint over TLS.
type Options struct {
	// Endpoint is the server address as host:port, a bare host (port 443
	// is assumed), or a grpc:// or https:// URL. Empty uses DefaultEndpoint.
	Endpoint string
	// Plaintext disables TLS, for local development servers.
	Plaintext bool
	// Symbols are fully qualified service or message names to resolve.
	// Empty resolves every service the server lists, except the reflection
	// service itself.
	Symbols []string
}

// Schema is the protobuf schema returned by Fetch.
type Schema struct {
	// Endpoint is the normalized address that was queried.
	Endpoint string
	// Symbols are the trimmed, deduplicated Options.Symbols, or the services
	// the server lists when Options.Symbols is empty.
	Symbols []string
	// DescriptorSet holds the resolved files, each after the files it imports.
	DescriptorSet *descriptorpb.FileDescriptorSet
}

// Fetch uses server reflection to fetch the protobuf files that define
// Options.Symbols, along with all of their imports. apiKey is sent as
// x-api-key metadata when it is not empty. opts is optional, and only the
// first is used.
func Fetch(ctx context.Context, apiKey string, opts ...Options) (*Schema, error) {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	endpoint, host, err := normalizeEndpoint(o.Endpoint)
	if err != nil {
		return nil, err
	}

	creds := credentials.NewTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if o.Plaintext {
		creds = insecure.NewCredentials()
	}
	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMessageSize)),
	)
	if err != nil {
		return nil, fmt.Errorf("grpcschema: connect: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	apiKey = strings.TrimSpace(apiKey)
	if apiKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-api-key", apiKey)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := reflectionpb.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("grpcschema: open reflection stream: %w", err)
	}
	r := &reflector{stream: stream, host: host}

	symbols := cleanSymbols(o.Symbols)
	if len(symbols) == 0 {
		symbols, err = r.listServices()
		if err != nil {
			return nil, err
		}
	}
	if len(symbols) == 0 {
		return nil, errors.New("grpcschema: server lists no services")
	}

	files := map[string]*descriptorpb.FileDescriptorProto{}
	for _, symbol := range symbols {
		err = r.addFiles(files, &reflectionpb.ServerReflectionRequest{
			MessageRequest: &reflectionpb.ServerReflectionRequest_FileContainingSymbol{FileContainingSymbol: symbol},
		})
		if err != nil {
			return nil, err
		}
	}
	err = r.resolveDependencies(files)
	if err != nil {
		return nil, err
	}

	return &Schema{
		Endpoint:      endpoint,
		Symbols:       symbols,
		DescriptorSet: &descriptorpb.FileDescriptorSet{File: orderedFiles(files)},
	}, nil
}

func normalizeEndpoint(raw string) (endpoint, host string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = DefaultEndpoint
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", "", fmt.Errorf("grpcschema: invalid endpoint: %w", err)
		}
		raw = u.Host
	}
	raw = strings.TrimSuffix(raw, "/")
	if raw == "" {
		return "", "", errors.New("grpcschema: empty endpoint")
	}

	h, _, err := net.SplitHostPort(raw)
	if err == nil {
		return raw, h, nil
	}
	host = strings.Trim(raw, "[]")
	return net.JoinHostPort(host, "443"), host, nil
}

// Explain returns a human-readable explanation of a common gRPC error
// returned by Fetch, or an empty string if it has none.
func Explain(err error) string {
	s, ok := status.FromError(err)
	if !ok {
		return ""
	}
	switch s.Code() {
	case codes.Unauthenticated:
		return "Missing or invalid API key."
	case codes.PermissionDenied:
		return "This API key does not have permission to inspect gRPC schemas."
	case codes.NotFound:
		return "No protobuf schema found for the requested symbol. Fetch without symbols to list the available services."
	case codes.Unimplemented:
		return "The gRPC endpoint does not expose server reflection."
	case codes.Unavailable:
		return "Could not reach the gRPC endpoint. Check the address, network access, and TLS settings."
	case codes.DeadlineExceeded:
		return "Timed out reading protobuf schemas from gRPC reflection. Use a longer context deadline."
	default:
		return ""
	}
}

type reflector struct {
	stream grpc.BidiStreamingClient[reflectionpb.ServerReflectionRequest, reflectionpb.ServerReflectionResponse]
	host   string
}

func (r *reflector) do(req *reflectionpb.ServerReflectionRequest) (*reflectionpb.ServerReflectionResponse, error) {
	req.Host = r.host
	err := r.stream.Send(req)
	if err != nil {
		return nil, fmt.Errorf("grpcschema: send reflection request: %w", err)
	}
	resp, err := r.stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("grpcschema: receive reflection response: %w", err)
	}
	errResp := resp.GetErrorResponse()
	if errResp != nil {
		return nil, status.Error(codes.Code(errResp.GetErrorCode()), errResp.GetErrorMessage()) //nolint:gosec // gRPC status codes are small non-negative values
	}
	return resp, nil
}

func (r *reflector) listServices() ([]string, error) {
	resp, err := r.do(&reflectionpb.ServerReflectionRequest{
		MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{},
	})
	if err != nil {
		return nil, err
	}
	list := resp.GetListServicesResponse()
	if list == nil {
		return nil, errors.New("grpcschema: reflection response has no service list")
	}

	var services []string
	for _, service := range list.GetService() {
		name := strings.TrimSpace(service.GetName())
		if name == "" {
			continue
		}
		if strings.HasPrefix(name, "grpc.reflection.") {
			continue
		}
		services = append(services, name)
	}
	slices.Sort(services)
	return services, nil
}

func (r *reflector) addFiles(files map[string]*descriptorpb.FileDescriptorProto, req *reflectionpb.ServerReflectionRequest) error {
	resp, err := r.do(req)
	if err != nil {
		return err
	}
	list := resp.GetFileDescriptorResponse()
	if list == nil {
		return errors.New("grpcschema: reflection response has no file descriptors")
	}
	for _, data := range list.GetFileDescriptorProto() {
		file := &descriptorpb.FileDescriptorProto{}
		err = proto.Unmarshal(data, file)
		if err != nil {
			return fmt.Errorf("grpcschema: decode file descriptor: %w", err)
		}
		name := file.GetName()
		if name == "" {
			return errors.New("grpcschema: reflection returned a file descriptor without a name")
		}
		_, ok := files[name]
		if !ok {
			files[name] = file
		}
	}
	return nil
}

func (r *reflector) resolveDependencies(files map[string]*descriptorpb.FileDescriptorProto) error {
	for {
		missing := missingDependencies(files)
		if len(missing) == 0 {
			return nil
		}
		for _, name := range missing {
			err := r.addFiles(files, &reflectionpb.ServerReflectionRequest{
				MessageRequest: &reflectionpb.ServerReflectionRequest_FileByFilename{FileByFilename: name},
			})
			if err != nil {
				return err
			}
			_, ok := files[name]
			if !ok {
				return fmt.Errorf("grpcschema: reflection did not return dependency %s", name)
			}
		}
	}
}

func missingDependencies(files map[string]*descriptorpb.FileDescriptorProto) []string {
	var missing []string
	for _, file := range files {
		for _, dep := range file.GetDependency() {
			_, ok := files[dep]
			if !ok && !slices.Contains(missing, dep) {
				missing = append(missing, dep)
			}
		}
	}
	slices.Sort(missing)
	return missing
}

func orderedFiles(files map[string]*descriptorpb.FileDescriptorProto) []*descriptorpb.FileDescriptorProto {
	ordered := make([]*descriptorpb.FileDescriptorProto, 0, len(files))
	visited := map[string]bool{}
	var visit func(name string)
	visit = func(name string) {
		file, ok := files[name]
		if !ok || visited[name] {
			return
		}
		visited[name] = true
		for _, dep := range slices.Sorted(slices.Values(file.GetDependency())) {
			visit(dep)
		}
		ordered = append(ordered, file)
	}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		visit(name)
	}
	return ordered
}

func cleanSymbols(symbols []string) []string {
	var cleaned []string
	for _, s := range symbols {
		s = strings.TrimSpace(s)
		if s != "" && !slices.Contains(cleaned, s) {
			cleaned = append(cleaned, s)
		}
	}
	return cleaned
}
