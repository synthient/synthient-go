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

- [Installation](#installation)
- [Quick start](#quick-start)
- [Authentication](#authentication)
- [Available Resources and Operations](#available-resources-and-operations)
- [IP and domain lookups](#ip-and-domain-lookups)
- [Account](#account)
- [Feeds](#feeds)
  - [Streaming](#streaming)
  - [Pagination](#pagination)
  - [Downloads](#downloads)
- [Errors](#errors)
- [Retries and timeouts](#retries-and-timeouts)
- [Configuration](#configuration)
- [Response metadata](#response-metadata)
- [gRPC schema](#grpc-schema)
- [Requirements](#requirements)
- [Versioning](#versioning)
- [Documentation](#documentation)
- [Contributions](#contributions)
- [License](#license)

## Installation

```sh
go get github.com/synthient/synthient-go
```

The package is named `synthient`. The [gRPC schema fetcher](#grpc-schema) is the `grpcschema` package in the same module.

## Quick start

Create a client with the API key in `SYNTHIENT_API_KEY` and look up one address:

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
	ctx := context.Background()

	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	info, err := client.LookupIP(ctx, netip.MustParseAddr("8.8.8.8"))
	if err != nil {
		log.Fatal(err)
	}
	if info.Intelligence != nil {
		fmt.Println(info.Intelligence.RiskScore, info.Intelligence.Categories)
	}
}
```

A `Client` is safe for concurrent use, so create one and reuse it. The examples below reuse `client` and `ctx`, and runnable programs are in [_examples](_examples). The [Go SDK guide](https://docs.synthient.com/sdks/go) walks through the same steps.

## Authentication

Every request needs a Synthient API key. Keys are issued from the Synthient dashboard; if you don't have one, [contact us](mailto:contact@synthient.com). A key's scopes decide what it can call: `BASIC` covers the lookups and `Account`, and each feed has its own scopes, listed under [Feeds](#feeds). See [Authentication](https://docs.synthient.com/authentication) for how keys and scopes work.

This SDK supports the following security scheme:

| Name     | Type    | Scheme             |
| -------- | ------- | ------------------ |
| `apiKey` | API key | `x-api-key` header |

Pass the key to `NewClient`, reading it from the environment or a secret manager:

```go
client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
if err != nil {
	log.Fatal(err)
}
```

`NewClient` makes no network calls. To check a key, call `Account`, which costs no credits.

If a request fails even though you are sending a key:

- **401 Unauthorized**: the key is malformed, unknown, or revoked.
- **402 Payment Required**: the team is out of lookup credits.
- **403 Forbidden**: the key lacks the scope for that endpoint or feed.

`Account` shows the key's scopes and the team's remaining credits.

## Available Resources and Operations

<details open>
<summary>Available methods</summary>

### [Client](https://pkg.go.dev/github.com/synthient/synthient-go#Client)

- [`LookupIP`](https://pkg.go.dev/github.com/synthient/synthient-go#Client.LookupIP) - Look up an IPv4 or IPv6 address
- [`LookupIPs`](https://pkg.go.dev/github.com/synthient/synthient-go#Client.LookupIPs) - Look up as many as 1000 addresses in one request
- [`LookupDomain`](https://pkg.go.dev/github.com/synthient/synthient-go#Client.LookupDomain) - Look up the honeypot traffic observed for a domain
- [`Account`](https://pkg.go.dev/github.com/synthient/synthient-go#Client.Account) - Get the key's account, scopes, and remaining lookup credits
- [`Health`](https://pkg.go.dev/github.com/synthient/synthient-go#Client.Health) - Check that the API and its dependencies are healthy

### [Feed](https://pkg.go.dev/github.com/synthient/synthient-go#Feed)

Each feed is a field of the client, such as `client.Proxies`, and has the same methods:

- [`Stream`](https://pkg.go.dev/github.com/synthient/synthient-go#Feed.Stream) - Iterate over the feed's real-time events
- [`ListExports`](https://pkg.go.dev/github.com/synthient/synthient-go#Feed.ListExports) - List one page of the feed's Parquet exports, newest first
- [`AllExports`](https://pkg.go.dev/github.com/synthient/synthient-go#Feed.AllExports) - Iterate over every export, fetching pages as needed
- [`ExportMeta`](https://pkg.go.dev/github.com/synthient/synthient-go#Feed.ExportMeta) - Get an export's checksum, size, row count, and Parquet schema
- [`Download`](https://pkg.go.dev/github.com/synthient/synthient-go#Feed.Download) - Download an export's Parquet file
- [`DownloadURL`](https://pkg.go.dev/github.com/synthient/synthient-go#Feed.DownloadURL) - Get a presigned URL for an export's Parquet file, valid for 24 hours

</details>

## IP and domain lookups

`LookupIP`, `LookupIPs`, and `LookupDomain` are covered in the [lookups reference](https://docs.synthient.com/sdks/go/lookups), and the fields they return in the [IP API reference](https://docs.synthient.com/ipapi) and the [types reference](https://docs.synthient.com/sdks/go/types).

### Look up an IP address

Get the network, location, and risk intelligence for an IPv4 or IPv6 address. Each lookup costs one credit; failed lookups are free.

```go
info, err := client.LookupIP(ctx, netip.MustParseAddr("8.8.8.8"))
if err != nil {
	log.Fatal(err)
}
if info.Network != nil {
	fmt.Println(info.Network.ISP, info.Network.Type)
}
if info.Location != nil {
	fmt.Println(info.Location.City, info.Location.Country)
}
```

`Network`, `Location`, and `Intelligence` are nil when the API has no data for them. Private and reserved addresses return a 422 `*APIError`.

### Look up many IP addresses

Look up as many as 1000 addresses in one request for `ceil(n * 0.9)` credits. Private and reserved addresses are skipped and duplicates are merged, so match results by `IP`.

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

Get the honeypot traffic observed for a domain and its most recent events. Each lookup costs one credit.

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

`Limit` caps the recent events returned, from 1 to 1000, and zero uses the API default of 50. Internationalized names must be sent in Punycode. A domain that has never been observed returns the status `inactive` and zero counts, not an error.

## Account

Get the key's account, scopes, and remaining lookup credits, at no cost.

```go
account, err := client.Account(ctx)
if err != nil {
	log.Fatal(err)
}
fmt.Println(account.Organization.Name, account.Scopes, account.LookupQuota.Credits)
```

`Health` returns nil if the API and its dependencies are healthy, or a 503 `*APIError` if not. It costs no credits and does not check the key:

```go
err = client.Health(ctx)
if err != nil {
	log.Fatal(err)
}
```

## Feeds

Each feed has a real-time stream and hourly Parquet exports that roll up into daily exports. Streaming and exports need separate scopes on the key:

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

See the [feeds reference](https://docs.synthient.com/sdks/go/feeds) for every method and the [types reference](https://docs.synthient.com/sdks/go/types) for the event fields. The data behind the feeds is described under [Firehose](https://docs.synthient.com/enterprise/firehose) and [Feeds](https://docs.synthient.com/enterprise/feeds).

### Streaming

`Stream` iterates over a feed's real-time events. Each `range` opens a connection, and breaking out of the loop closes it:

```go
for ctx.Err() == nil {
	for event, err := range client.Proxies.Stream(ctx) {
		if err != nil {
			apiErr, ok := errors.AsType[*synthient.APIError](err)
			if ok && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden) {
				log.Fatal(err)
			}
			log.Printf("proxy stream: %v", err)
			// Avoid reconnecting in a tight loop.
			time.Sleep(2 * time.Second)
			break
		}
		fmt.Println(event.IP, event.Provider, event.Type)
	}
}
```

- The server closes each stream after up to 30 minutes. That ends the inner loop without an error, and the outer loop reconnects at once.
- A 401 or 403 stops the loop, because reconnecting can't fix a rejected key or a missing scope.
- Any other failure ends the iteration, and the loop waits 2 seconds before reconnecting.
- Events sent while disconnected are missed. The API has no resume cursor.

### Pagination

`ListExports` returns one page of a feed's exports, newest first. Pass `page.NextCursor` as `Cursor` to get the next page; it is empty on the last one. `Limit` is the page size, from 1 to 500, and zero uses the API default of 100.

```go
page, err := client.Proxies.ListExports(ctx, synthient.ListExportsParams{Limit: 50})
if err != nil {
	log.Fatal(err)
}
for _, export := range page.Exports {
	fmt.Println(export.ID, export.Kind, export.RowCount)
}
```

`AllExports` fetches the pages for you as the loop needs them:

```go
for export, err := range client.Proxies.AllExports(ctx) {
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(export.ID, export.Kind, export.RowCount)
}
```

On the first page, the newest hourly export may have the ID `latest`, a moving alias for the freshest export. Its `Date` and `Hour` give its stable ID, such as `2026-10-14/21`.

### Downloads

Exports are Parquet files. `Download` returns a file's contents, which you must close:

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

- Name an export by its `ID` from a listing, by `synthient.LatestExport`, or by `synthient.DailyExport(t)` or `synthient.HourlyExport(t)` for a UTC day or hour.
- `ExportMeta` returns the file's SHA-256 checksum. Use a fixed ID for both calls, because `latest` can move between them.
- `DownloadURL` returns a presigned URL that works without a key for 24 hours, so treat it as a secret. Your API key is never sent to the storage host.
- Hourly exports roll into the daily export shortly after 00:30 UTC the next day. After that, the hour returns a 400.

## Errors

A request that fails with a non-success status returns an [`*APIError`](https://pkg.go.dev/github.com/synthient/synthient-go#APIError):

| Field        | Type                  | Description                                    |
| ------------ | --------------------- | ---------------------------------------------- |
| `StatusCode` | `int`                 | HTTP status code, such as `404`                |
| `Type`       | `string`              | URI identifying the problem type               |
| `Title`      | `string`              | Short summary of the problem type              |
| `Detail`     | `string`              | Explanation of this occurrence                 |
| `Errors`     | `map[string][]string` | Validation messages by request field           |
| `RequestID`  | `string`              | Request ID to give Synthient support           |
| `RetryAfter` | `time.Duration`       | How long the server asked you to wait, or zero |

`Error()` returns only the HTTP status, so it is safe to log. The fields hold untrusted server data.

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

### Error types

- [`*APIError`](https://pkg.go.dev/github.com/synthient/synthient-go#APIError): the API, or the storage host during `Download`, returned a non-success status.
- [`ErrMissingAPIKey`](https://pkg.go.dev/github.com/synthient/synthient-go#ErrMissingAPIKey): `NewClient` was given an empty or all-whitespace key.
- Context errors: match `context.Canceled` or `context.DeadlineExceeded` with `errors.Is`.
- Transport errors: network failures, as the `*url.Error` from the HTTP client.
- Other errors: invalid arguments, rejected before any request is sent, and responses that can't be decoded. Their messages start with `synthient:`.

<details><summary>Status codes</summary>

<br />

| Status | Meaning                                                                                         |
| ------ | ----------------------------------------------------------------------------------------------- |
| 400    | The request is invalid, such as an hourly export that was rolled up                             |
| 401    | The API key is malformed, unknown, or revoked                                                   |
| 402    | The team is out of lookup credits                                                               |
| 403    | The key lacks the scope for the endpoint or feed                                                |
| 404    | The export doesn't exist, or a daily export is still rolling up (`RetryAfter` is set)           |
| 422    | The input is invalid, such as a private address or a malformed domain                           |
| 429    | Rate limited if `RetryAfter` is set; otherwise the budget is spent or too many streams are open |
| 5xx    | The server failed                                                                               |

</details>

See [Errors and retries](https://docs.synthient.com/sdks/go/errors) for the SDK's error reference and [Errors](https://docs.synthient.com/errors) for what each status means.

## Retries and timeouts

The client retries a failed request up to twice, backing off exponentially with jitter and honoring `Retry-After`:

| Requests                    | Retried on                                                                                   |
| --------------------------- | -------------------------------------------------------------------------------------------- |
| Account and feed requests   | 500, 502, 503, and 504, most transport errors, and 429 with `Retry-After`                    |
| Billed lookups and `Health` | Only 429 with `Retry-After`, which comes before any work, so a lookup is never charged twice |

A 429 without `Retry-After` means the monthly request budget is spent or too many streams are open, so it is returned at once. Downloads from the storage host are never retried.

Waits start at one second and double up to 30 seconds. A `Retry-After` longer than a minute, or a wait that would pass the context deadline, returns the response instead of retrying.

Set the number of retries with `WithMaxRetries`. Zero disables them:

```go
client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"),
	synthient.WithMaxRetries(4),
)
if err != nil {
	log.Fatal(err)
}
```

The client sets no overall timeout, because streams and downloads are long-lived. Bound a call with a context deadline:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

info, err := client.LookupIP(ctx, netip.MustParseAddr("8.8.8.8"))
```

The default HTTP client waits at most one minute for response headers.

## Configuration

Each option is a function passed to `NewClient`. An invalid option, such as a malformed base URL or a negative retry count, makes `NewClient` return an error. See the [client reference](https://docs.synthient.com/sdks/go/client).

| Option             | Default                              | Description                                       |
| ------------------ | ------------------------------------ | ------------------------------------------------- |
| `WithBaseURL`      | `https://api.synthient.com/api/v4`   | API base URL, including the version path          |
| `WithHTTPClient`   | One-minute wait for response headers | The `*http.Client` used to send requests          |
| `WithMaxRetries`   | `2`                                  | How many times a failed request is retried        |
| `WithUserAgent`    | `synthient-go/` and the SDK version  | Product token appended to the `User-Agent` header |
| `WithResponseHook` | None                                 | Function called after every API response          |

### Server URL

The default server is `https://api.synthient.com/api/v4`. Use `WithBaseURL` to send requests elsewhere, such as a mock server in tests:

```go
client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"),
	synthient.WithBaseURL("http://localhost:8080/api/v4"),
)
if err != nil {
	log.Fatal(err)
}
```

### Custom HTTP client

Pass your own `*http.Client` with `WithHTTPClient`, for example to use a proxy. Clone the default transport to keep its connection and TLS settings:

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

- Set `ResponseHeaderTimeout` yourself, because your client replaces the default one-minute limit.
- Don't set `http.Client.Timeout`. It would also cut off streams and downloads.
- The transport must not replay requests whose bodies can't be rewound, so a billed lookup is never sent twice.

`WithUserAgent("my-app/1.2")` adds your product token to the `User-Agent` header.

## Response metadata

`WithResponseHook` calls a function after every API response, including retries, pages, and stream handshakes, with the status, request ID, rate limit, and headers. Responses from the storage host during `Download` are excluded:

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

| Field        | Type          | Description                                                        |
| ------------ | ------------- | ------------------------------------------------------------------ |
| `StatusCode` | `int`         | HTTP status code of the response                                   |
| `RequestID`  | `string`      | The `X-Request-ID` header                                          |
| `RateLimit`  | `*RateLimit`  | `Limit`, `Remaining`, and `Reset`, or nil if the response has none |
| `Header`     | `http.Header` | An independent copy of the response headers                        |

`RateLimit` comes from the `RateLimit-Limit`, `RateLimit-Remaining`, and `RateLimit-Reset` headers, and `Reset` is a `time.Duration`. Limits are shared by every key of a team; see [Rate Limits](https://docs.synthient.com/rate-limits).

The hook must be safe for concurrent use.

> [!WARNING]
> Response headers can contain secrets, such as presigned download URLs. Avoid logging `meta.Header` in production.

## gRPC schema

The [`grpcschema`](https://pkg.go.dev/github.com/synthient/synthient-go/grpcschema) package fetches the protobuf schema of the [Synthient gRPC API](https://docs.synthient.com/grpc) through server reflection:

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

`schema.DescriptorSet` is a `*descriptorpb.FileDescriptorSet`. `grpcschema.Explain` describes common gRPC errors in plain language.

`Fetch` takes an optional `grpcschema.Options`. The zero value fetches every service from `grpcschema.DefaultEndpoint`, `grpc.synthient.com:443`, over TLS. Set its fields to query another server, such as a local one:

```go
schema, err := grpcschema.Fetch(ctx, os.Getenv("SYNTHIENT_API_KEY"), grpcschema.Options{
	Endpoint:  "localhost:50051",
	Plaintext: true,
})
```

- `Endpoint` is `host:port`, a bare host (port 443 is assumed), or a `grpc://` or `https://` URL.
- `Plaintext` disables TLS, for local development servers.
- `Symbols` are the fully qualified service or message names to resolve. Empty resolves every service the server lists.

See [gRPC schema](https://docs.synthient.com/sdks/go/grpc-schema) for the full reference.

## Requirements

Go 1.26 or later. The `synthient` package uses only the standard library; the `grpcschema` package depends on `google.golang.org/grpc` and `google.golang.org/protobuf`.

## Versioning

This SDK follows [Semantic Versioning](https://semver.org). Breaking changes ship only in a new major version, with a new import path such as `github.com/synthient/synthient-go/v2`. The running version is the `synthient.Version` constant, which is sent in the `User-Agent` header.

## Documentation

- [Go SDK guide](https://docs.synthient.com/sdks/go): [client](https://docs.synthient.com/sdks/go/client), [lookups](https://docs.synthient.com/sdks/go/lookups), [feeds](https://docs.synthient.com/sdks/go/feeds), [errors and retries](https://docs.synthient.com/sdks/go/errors), [types](https://docs.synthient.com/sdks/go/types), and [gRPC schema](https://docs.synthient.com/sdks/go/grpc-schema)
- [API reference on pkg.go.dev](https://pkg.go.dev/github.com/synthient/synthient-go)
- [Synthient API documentation](https://docs.synthient.com): [IP API](https://docs.synthient.com/ipapi), [authentication](https://docs.synthient.com/authentication), [rate limits](https://docs.synthient.com/rate-limits), [errors](https://docs.synthient.com/errors), [feeds](https://docs.synthient.com/enterprise/feeds), and [firehose](https://docs.synthient.com/enterprise/firehose)

## Contributions

Bug reports and feature requests are welcome as [GitHub issues](https://github.com/synthient/synthient-go/issues). For questions about the API or your account, contact [Synthient support](https://docs.synthient.com/misc/support).

## License

[Apache 2.0](LICENSE)
