<p align="center">
  <a href="https://synthient.com">
    <img src="assets/logo.svg" height="96" alt="Synthient logo">
  </a>
</p>
<h3 align="center">Synthient</h3>
<p align="center">High-fidelity data on residential proxies, VPNs, and other anonymized IPs.</p>
<p align="center">
  <a href="https://docs.synthient.com">Documentation</a> ·
  <a href="https://pkg.go.dev/github.com/synthient/synthient-go">API reference</a> ·
  <a href="https://docs.synthient.com/misc/support">Support</a>
</p>

# synthient-go

`synthient-go` is the official Go SDK for the [Synthient API](https://docs.synthient.com): IP and domain intelligence, account details, and real-time and bulk data feeds.

[![Go Reference](https://pkg.go.dev/badge/github.com/synthient/synthient-go.svg)](https://pkg.go.dev/github.com/synthient/synthient-go)
[![test](https://github.com/synthient/synthient-go/actions/workflows/test.yml/badge.svg)](https://github.com/synthient/synthient-go/actions/workflows/test.yml)
[![lint](https://github.com/synthient/synthient-go/actions/workflows/lint.yml/badge.svg)](https://github.com/synthient/synthient-go/actions/workflows/lint.yml)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

## Table of Contents

- [synthient-go](#synthient-go)
  - [SDK Installation](#sdk-installation)
  - [Requirements](#requirements)
  - [API Keys](#api-keys)
  - [Authentication](#authentication)
  - [SDK Example Usage](#sdk-example-usage)
  - [Available Resources and Operations](#available-resources-and-operations)
  - [Streaming](#streaming)
  - [Pagination](#pagination)
  - [Downloads](#downloads)
  - [Retries](#retries)
  - [Timeouts](#timeouts)
  - [Error Handling](#error-handling)
  - [Server Selection](#server-selection)
  - [Custom HTTP Client](#custom-http-client)
  - [Debugging](#debugging)
  - [gRPC Schema](#grpc-schema)
- [Development](#development)
  - [Maturity](#maturity)
  - [Contributions](#contributions)
  - [License](#license)

## SDK Installation

```sh
go get github.com/synthient/synthient-go
```

The [gRPC schema fetcher](#grpc-schema) is the `grpcschema` package in the same module. The first time you import it, run `go mod tidy` to add its gRPC dependencies.

## Requirements

Go 1.26 or later. The `synthient` package uses only the standard library. The `grpcschema` package also uses `google.golang.org/grpc` and `google.golang.org/protobuf`, which your program compiles in only if it imports `grpcschema`.

## API Keys

Every request to the HTTP API needs a Synthient API key. Keys are issued from the Synthient dashboard; if you don't have one yet, [contact us](mailto:contact@synthient.com). Each key carries scopes that decide what it can call. `BASIC` covers the lookups and `Account`, and each feed has one scope for its real-time stream and one for its exports, listed under [Feeds](#feeds). See [Authentication](https://docs.synthient.com/authentication) for how scopes work.

If a request fails even though you are sending a key, it is usually one of these:

- **401 Unauthorized**: the key is malformed, unknown, or revoked. Check that it was loaded from the environment and isn't truncated.
- **402 Payment Required**: the team is out of lookup credits. `Account` reports the remaining balance and when it resets.
- **403 Forbidden**: the key lacks the scope for that endpoint or feed. `Account` lists the key's scopes.

## Authentication

This SDK supports the following security scheme:

| Name     | Type    | Scheme             |
| -------- | ------- | ------------------ |
| `apiKey` | API key | `x-api-key` header |

Pass the key to `NewClient`. Read it from the environment or a secret manager rather than writing it into your code:

```go
client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
if err != nil {
	log.Fatal(err)
}
```

`NewClient` trims surrounding whitespace from the key and returns `ErrMissingAPIKey` if nothing is left. It makes no network calls, so an invalid key shows up as a 401 on the first request that checks it. `Health` only rejects a malformed key, so call `Account`, which costs no credits, to test one. The key is sent only to the Synthient API, never to the storage host that serves export downloads.

## SDK Example Usage

### Look up an IP address

Get the network, location, and risk intelligence for an IPv4 or IPv6 address. Each lookup costs one credit, and failed lookups are free.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"os"

	"github.com/synthient/synthient-go"
)

func main() {
	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	info, err := client.LookupIP(context.Background(), netip.MustParseAddr("8.8.8.8"))
	if err != nil {
		log.Fatal(err)
	}
	if info.Intelligence != nil {
		fmt.Println(info.Intelligence.RiskScore, info.Intelligence.Categories)
	}
}
```

`Network`, `Location`, and `Intelligence` are nil when the API has no data for that section, so check them before reading their fields. The examples below reuse `client` and a `ctx`. Runnable programs are in [_examples](_examples), for example `go run ./_examples/lookup_ip 8.8.8.8`.

### Look up many IP addresses

Look up as many as 1000 addresses in one request, billed at `ceil(n * 0.9)` credits for the `n` distinct addresses looked up. Private and reserved addresses are skipped and duplicates collapse into one result, so match results by `IP` rather than by index.

```go
results, err := client.LookupIPs(ctx, []netip.Addr{
	netip.MustParseAddr("8.8.8.8"),
	netip.MustParseAddr("1.1.1.1"),
})
if err != nil {
	log.Fatal(err)
}
for _, result := range results {
	if result.Intelligence != nil {
		fmt.Println(result.IP, result.Intelligence.RiskScore)
	}
}
```

### Look up a domain

Get the honeypot traffic observed for a domain, with its most recent events. A domain that has never been observed returns an inactive status and zero counts rather than an error. Each lookup costs one credit.

```go
domain, err := client.LookupDomain(ctx, "example.com", synthient.LookupDomainParams{Limit: 10})
if err != nil {
	log.Fatal(err)
}
fmt.Println(domain.Status, domain.Stats.Events24h)
for _, event := range domain.RecentEvents {
	fmt.Println(event.Timestamp.Time(), event.TargetSubdomain, event.Port)
}
```

### Check your account

Get the account that owns the API key, with the key's scopes and the team's remaining lookup credits. It costs no credits.

```go
account, err := client.Account(ctx)
if err != nil {
	log.Fatal(err)
}
fmt.Println(account.Organization.Name, account.Scopes, account.LookupQuota.Credits)
```

## Available Resources and Operations

<details open>
<summary>Available methods</summary>

### [Client](https://pkg.go.dev/github.com/synthient/synthient-go#Client)

- [`LookupIP`](https://pkg.go.dev/github.com/synthient/synthient-go#Client.LookupIP) - Look up an IPv4 or IPv6 address
- [`LookupIPs`](https://pkg.go.dev/github.com/synthient/synthient-go#Client.LookupIPs) - Look up as many as 1000 addresses in one request
- [`LookupDomain`](https://pkg.go.dev/github.com/synthient/synthient-go#Client.LookupDomain) - Look up the honeypot traffic observed for a domain
- [`Account`](https://pkg.go.dev/github.com/synthient/synthient-go#Client.Account) - Get the account that owns the API key, the key's scopes, and the team's remaining lookup credits
- [`Health`](https://pkg.go.dev/github.com/synthient/synthient-go#Client.Health) - Check that the API and its dependencies are healthy

### [Feeds](https://pkg.go.dev/github.com/synthient/synthient-go#Feed)

Each feed is a field of the client, such as `client.Proxies`, and has the same methods:

- [`Stream`](https://pkg.go.dev/github.com/synthient/synthient-go#Feed.Stream) - Iterate over the feed's real-time events
- [`ListExports`](https://pkg.go.dev/github.com/synthient/synthient-go#Feed.ListExports) - List one page of the feed's Parquet exports, newest first
- [`AllExports`](https://pkg.go.dev/github.com/synthient/synthient-go#Feed.AllExports) - Iterate over every export, fetching pages as needed
- [`ExportMeta`](https://pkg.go.dev/github.com/synthient/synthient-go#Feed.ExportMeta) - Get an export's checksum, size, row count, and Parquet schema
- [`Download`](https://pkg.go.dev/github.com/synthient/synthient-go#Feed.Download) - Download an export's Parquet file
- [`DownloadURL`](https://pkg.go.dev/github.com/synthient/synthient-go#Feed.DownloadURL) - Get a presigned URL for an export's Parquet file, valid for 24 hours

| Field         | Data                                          | Event type         | Stream scope            | Export scope          |
| ------------- | --------------------------------------------- | ------------------ | ----------------------- | --------------------- |
| `Proxies`     | Residential, mobile, and datacenter proxies   | `ProxyEvent`       | `PROXY_FIREHOSE`        | `PROXY_FEEDS`         |
| `Anonymizers` | VPN and anonymizer IP ranges                  | `AnonymizerEvent`  | `ANONYMIZERS_STREAM`    | `ANONYMIZERS_FEED`    |
| `Torrents`    | BitTorrent DHT and tracker peers              | `TorrentEvent`     | `TORRENTS_STREAM`       | `TORRENTS_FEED`       |
| `JA4T`        | JA4T TCP fingerprints                         | `JA4TEvent`        | `JA4T_STREAM`           | `JA4T_FEED`           |
| `HeliosHTTP`  | Helios honeypot HTTP requests                 | `HeliosHTTPEvent`  | `HONEYPOT_HTTP_STREAM`  | `HONEYPOT_HTTP_FEED`  |
| `HeliosHTTPS` | Helios honeypot TLS ClientHellos              | `HeliosHTTPSEvent` | `HONEYPOT_HTTPS_STREAM` | `HONEYPOT_HTTPS_FEED` |
| `HeliosDNS`   | Helios honeypot tunnel destinations           | `HeliosDNSEvent`   | `HONEYPOT_DNS_STREAM`   | `HONEYPOT_DNS_FEED`   |
| `HeliosADB`   | Helios honeypot Android Debug Bridge commands | `HeliosADBEvent`   | `HONEYPOT_ADB_STREAM`   | `HONEYPOT_ADB_FEED`   |

</details>

## Streaming

`Stream` returns an iterator over a feed's real-time events. Each `range` over it opens a new connection, and breaking out of the loop closes it. The server keeps a stream open for up to 30 minutes and then closes it, which ends the loop without an error. The API has no resume cursor, so loop again to reconnect:

```go
for ctx.Err() == nil {
	for event, err := range client.Proxies.Stream(ctx) {
		if err != nil {
			apiErr, ok := errors.AsType[*synthient.APIError](err)
			if ok && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden) {
				log.Fatal(err)
			}
			log.Printf("proxy stream: %v", err)
			// Pause after a failure so a failing stream doesn't reconnect
			// in a tight loop.
			time.Sleep(2 * time.Second)
			break
		}
		fmt.Println(event.IP, event.Provider, event.Type)
	}
}
```

The loop stops on a 401 or 403, because reconnecting can't fix a rejected key or a missing scope. Other failures, such as a dropped connection or a canceled context, are yielded once and end the iteration. After a failure the loop waits 2 seconds before reconnecting, and after the server's routine close it reconnects at once. A 429 `*APIError` without `RetryAfter` means the key already has too many open streams.

## Pagination

`ListExports` returns one page of a feed's exports, newest first:

```go
page, err := client.Proxies.ListExports(ctx, synthient.ListExportsParams{Limit: 50})
if err != nil {
	log.Fatal(err)
}
for _, export := range page.Exports {
	fmt.Println(export.ID, export.Kind, export.RowCount)
}
```

To get the next page, pass `page.NextCursor` as `ListExportsParams.Cursor`. It is empty on the last page. `AllExports` does the paging for you. It fetches pages as the loop needs them and stops fetching when the loop breaks:

```go
for export, err := range client.Proxies.AllExports(ctx) {
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(export.ID, export.Kind, export.RowCount)
}
```

On the first page, the newest hourly export has the ID `latest`, a moving alias for the freshest export. To keep a stable ID, build it from the export's `Date` and `Hour`, such as `2026-10-14/21`.

## Downloads

Exports are Parquet files. Identify one by the `ID` of an `Export` from a listing, by `synthient.LatestExport` for the freshest one, or by `synthient.DailyExport(t)` or `synthient.HourlyExport(t)` for a UTC day or hour. `Download` returns the file's contents, which you must close:

```go
body, err := client.Proxies.Download(ctx, synthient.LatestExport)
if err != nil {
	log.Fatal(err)
}
defer body.Close()

file, err := os.Create("proxies.parquet")
if err != nil {
	log.Fatal(err)
}
_, err = io.Copy(file, body)
if err != nil {
	log.Fatal(err)
}
err = file.Close()
if err != nil {
	log.Fatal(err)
}
```

The file comes from a presigned storage URL that is requested without your API key. `ExportMeta` returns the SHA-256 checksum to verify it against; ask both for the same fixed ID, because `LatestExport` can move to a newer export between the two requests. `DownloadURL` returns the presigned URL itself for tools that read Parquet over HTTPS. The URL grants access without a key for 24 hours, so treat it as a secret.

An hourly export is available until its day's daily export is rolled up, shortly after 00:30 UTC the next day. After that, requests for the hour return a 400. While a daily export is being rolled up, requests for it return a 404 with `RetryAfter` set.

## Retries

The client retries a failed request up to twice by default, backing off exponentially with jitter and honoring `Retry-After`:

- Account and feed requests are retried on 500, 502, 503, and 504 responses, on transport errors other than TLS certificate failures and HTTP responses to HTTPS requests, and on 429 responses that carry `Retry-After`. A failure while reading a response body is not retried, and neither is the request `Download` makes to the storage host for the file itself.
- Billed lookups (`LookupIP`, `LookupIPs`, and `LookupDomain`) are retried only on 429 responses that carry `Retry-After`, which the rate limiter returns before doing any work, so a lookup is never charged twice. `Health` is retried the same way, so an unhealthy API is reported at once.

A 429 without `Retry-After` means the key's monthly request budget is spent or it has too many open streams, so it is returned without a retry. A `Retry-After` longer than a minute is returned to you instead of waited out, and so is any retryable response whose wait, the longer of the backoff and `Retry-After`, would outlast the context's deadline.

To change the number of retries, pass `WithMaxRetries` when creating the client. Zero disables retries:

```go
client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"),
	synthient.WithMaxRetries(4),
)
if err != nil {
	log.Fatal(err)
}
```

## Timeouts

The client sets no overall request timeout, because streams and downloads are long-lived. Bound a call with a context deadline instead:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

info, err := client.LookupIP(ctx, netip.MustParseAddr("8.8.8.8"))
```

The default HTTP client waits at most one minute for response headers.

## Error Handling

A request that fails with a non-success HTTP status returns an [`*APIError`](https://pkg.go.dev/github.com/synthient/synthient-go#APIError). It has the following fields:

| Field        | Type                  | Description                                                                                 |
| ------------ | --------------------- | ------------------------------------------------------------------------------------------- |
| `StatusCode` | `int`                 | HTTP response status code, such as `404`                                                    |
| `Type`       | `string`              | URI identifying the problem type, such as `https://docs.synthient.com/errors/validation-failed` |
| `Title`      | `string`              | Short summary of the problem type                                                           |
| `Detail`     | `string`              | Explanation of this occurrence. For a plain-text error, the response body                   |
| `Errors`     | `map[string][]string` | Validation messages by request field                                                        |
| `RequestID`  | `string`              | Request ID to give Synthient support                                                        |
| `RetryAfter` | `time.Duration`       | How long the server asked you to wait before retrying, or zero                              |

`Error()` returns only the HTTP status. The other fields hold untrusted server data that can be sensitive, so log them deliberately.

### Example

```go
_, err = client.LookupIP(ctx, netip.MustParseAddr("8.8.8.8"))
apiErr, ok := errors.AsType[*synthient.APIError](err)
if ok {
	switch {
	case apiErr.StatusCode == http.StatusPaymentRequired:
		fmt.Println("out of lookup credits")
	case apiErr.StatusCode == http.StatusTooManyRequests && apiErr.RetryAfter > 0:
		fmt.Println("rate limited, retry after", apiErr.RetryAfter)
	case apiErr.StatusCode == http.StatusTooManyRequests:
		fmt.Println("monthly request budget spent")
	default:
		fmt.Println("request", apiErr.RequestID, "failed:", cmp.Or(apiErr.Detail, apiErr.Title))
	}
}
```

### Error Types

- [`*APIError`](https://pkg.go.dev/github.com/synthient/synthient-go#APIError): the API, or the storage host during `Download`, returned a non-success status.
- [`ErrMissingAPIKey`](https://pkg.go.dev/github.com/synthient/synthient-go#ErrMissingAPIKey): `NewClient` was given an empty or all-whitespace key.
- Context errors: a canceled or expired context returns an error that matches `context.Canceled` or `context.DeadlineExceeded` with `errors.Is`.
- Transport errors: a network failure on an API request is returned as the `*url.Error` the HTTP client reported. `Download` removes that wrapper from storage failures to keep the presigned URL out of the message, and a failure while reading a response is wrapped with a `synthient:` prefix.
- Other errors: an invalid argument, such as an invalid address, more than 1000 addresses, or a malformed export ID, is rejected before any request is sent, and a response that can't be decoded also fails. These are plain errors whose messages start with `synthient:`.

<details><summary>Status codes</summary>

<br />

| Status | Meaning                                                                                                            |
| ------ | ------------------------------------------------------------------------------------------------------------------ |
| 400    | The request is invalid, such as asking for an hourly export that has been rolled up                                |
| 401    | The API key is malformed, unknown, or revoked                                                                      |
| 402    | The team is out of lookup credits                                                                                  |
| 403    | The key lacks the scope for the endpoint or feed                                                                   |
| 404    | The export does not exist, or a daily export is still being rolled up and `RetryAfter` is set                      |
| 422    | The input is invalid, such as a private or reserved address, a malformed domain, or a batch with no usable address |
| 429    | With `RetryAfter`, the rate limit was hit. Without it, the request budget is spent or too many streams are open    |
| 5xx    | The server failed. Account and feed requests are retried on 500, 502, 503, and 504                                 |

</details>

## Server Selection

### Override Server URL Per-Client

The default server is `https://api.synthient.com/api/v4`. To send requests elsewhere, such as through a gateway or to a mock server in tests, pass `WithBaseURL` when creating the client. The URL includes the version path:

```go
client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"),
	synthient.WithBaseURL("http://localhost:8080/api/v4"),
)
if err != nil {
	log.Fatal(err)
}
```

## Custom HTTP Client

The client sends requests with a standard `*http.Client`. Pass your own with `WithHTTPClient`, for example to route requests through a proxy. Your client replaces the default one, including its one-minute limit on waiting for response headers, so clone the default transport to keep its connection and TLS settings, and set `ResponseHeaderTimeout` yourself:

```go
proxyURL, err := url.Parse("http://proxy.example.com:8080")
if err != nil {
	log.Fatal(err)
}
transport := http.DefaultTransport.(*http.Transport).Clone()
transport.Proxy = http.ProxyURL(proxyURL)
transport.ResponseHeaderTimeout = time.Minute

client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"),
	synthient.WithHTTPClient(&http.Client{Transport: transport}),
)
if err != nil {
	log.Fatal(err)
}
```

The client is copied and never modified. Don't set `http.Client.Timeout`, which would also cut off streams and downloads; use context deadlines instead. A custom transport must not replay requests whose bodies can't be rewound, because that is what keeps a billed lookup from being sent twice.

To identify your application, `WithUserAgent("my-app/1.2")` appends a product token to the `User-Agent` header.

## Debugging

`WithResponseHook` calls a function after every API response arrives, including each retry, page, and stream handshake. It receives the status code, the request ID to give Synthient support, the operation's rate limit, and a copy of the response headers:

```go
client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"),
	synthient.WithResponseHook(func(_ context.Context, meta synthient.ResponseMetadata) {
		log.Printf("status=%d request_id=%s", meta.StatusCode, meta.RequestID)
		if meta.RateLimit != nil {
			log.Printf("rate limit: %d of %d left", meta.RateLimit.Remaining, meta.RateLimit.Limit)
		}
	}),
)
if err != nil {
	log.Fatal(err)
}
```

The hook runs synchronously and must be safe for concurrent use. Storage download responses are not passed to it.

> [!WARNING]
> Response headers can contain secrets, such as presigned download URLs. Avoid logging `meta.Header` in production.

## gRPC Schema

The [`grpcschema`](https://pkg.go.dev/github.com/synthient/synthient-go/grpcschema) package fetches the protobuf schema of the [Synthient gRPC API](https://docs.synthient.com/grpc) through gRPC server reflection:

```go
schema, err := grpcschema.Fetch(ctx, os.Getenv("SYNTHIENT_API_KEY"))
if err != nil {
	explanation := grpcschema.Explain(err)
	if explanation != "" {
		log.Fatalf("%s %v", explanation, err)
	}
	log.Fatal(err)
}
for _, file := range schema.DescriptorSet.GetFile() {
	fmt.Println(file.GetName(), file.GetPackage())
}
```

`schema.DescriptorSet` is a `*descriptorpb.FileDescriptorSet` with each file after the files it imports. `grpcschema.Explain` describes common gRPC errors in plain language, and returns an empty string for the rest.

# Development

## Maturity

This SDK follows [Semantic Versioning](https://semver.org). Breaking changes to its public API ship only in a new major version, which for a Go module also means a new import path, such as `github.com/synthient/synthient-go/v2`.

## Contributions

Bug reports and feature requests are welcome as [GitHub issues](https://github.com/synthient/synthient-go/issues). For questions about the API or your account, contact [Synthient support](https://docs.synthient.com/misc/support).

## License

[Apache 2.0](LICENSE)
