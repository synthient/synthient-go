# synthient-go

![Synthient](assets/readme-header.svg)

[![Go Reference](https://pkg.go.dev/badge/github.com/synthient/synthient-go.svg)](https://pkg.go.dev/github.com/synthient/synthient-go)
[![test](https://github.com/synthient/synthient-go/actions/workflows/test.yml/badge.svg)](https://github.com/synthient/synthient-go/actions/workflows/test.yml)
[![lint](https://github.com/synthient/synthient-go/actions/workflows/lint.yml/badge.svg)](https://github.com/synthient/synthient-go/actions/workflows/lint.yml)

The official Go client for the [Synthient API](https://docs.synthient.com): IP and domain intelligence, account details, and real-time and bulk data feeds.

## Install

```sh
go get github.com/synthient/synthient-go
```

Requires Go 1.26 or later.

## Usage

```go
client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
if err != nil {
	log.Fatal(err)
}

info, err := client.LookupIP(ctx, netip.MustParseAddr("8.8.8.8"))
if err != nil {
	log.Fatal(err)
}
fmt.Println(info.Network.ISP, info.Intelligence.RiskScore)
```

Look up many addresses in one request:

```go
results, err := client.LookupIPs(ctx, []netip.Addr{
	netip.MustParseAddr("8.8.8.8"),
	netip.MustParseAddr("1.1.1.1"),
})
if err != nil {
	log.Fatal(err)
}
for _, result := range results {
	fmt.Println(result.IP, result.Intelligence.RiskScore)
}
```

Consume a real-time feed:

```go
for event, err := range client.Proxies.Stream(ctx) {
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(event.IP, event.Provider)
}
```

## Documentation

The full reference, with examples for errors, retries, exports, and downloads, is on [pkg.go.dev](https://pkg.go.dev/github.com/synthient/synthient-go). Runnable programs are in [_examples](_examples), for example `go run ./_examples/lookup_ip 8.8.8.8`. Guides and field references are at [docs.synthient.com](https://docs.synthient.com). The [grpcschema](https://pkg.go.dev/github.com/synthient/synthient-go/grpcschema) package fetches the protobuf schema of the Synthient gRPC API.

## License

[Apache 2.0](LICENSE)
