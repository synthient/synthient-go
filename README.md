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

The [gRPC schema fetcher](#grpc-schema) is the `grpcschema` package in the same module.

## Requirements

Go 1.26 or later. The `synthient` package uses only the standard library.

## API Keys

Every request needs a Synthient API key. Keys are issued from the Synthient dashboard; if you don't have one, [contact us](mailto:contact@synthient.com). A key's scopes decide what it can call: `BASIC` covers the lookups and `Account`, and each feed has its own scopes, listed under [Feeds](#feeds).

If a request fails even though you are sending a key:

- **401 Unauthorized**: the key is malformed, unknown, or revoked.
- **402 Payment Required**: the team is out of lookup credits.
- **403 Forbidden**: the key lacks the scope for that endpoint or feed.

`Account` shows the key's scopes and the team's remaining credits.

## Authentication

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

## SDK Example Usage

### Look up an IP address

Get the network, location, and risk intelligence for an IPv4 or IPv6 address. Each lookup costs one credit; failed lookups are free.

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

`Network`, `Location`, and `Intelligence` are nil when the API has no data for them. The examples below reuse `client` and `ctx`, and runnable programs are in [_examples](_examples).

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

### Check your account

Get the key's account, scopes, and remaining lookup credits, at no cost.

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
- [`Account`](https://pkg.go.dev/github.com/synthient/synthient-go#Client.Account) - Get the key's account, scopes, and remaining lookup credits
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

## Pagination

`ListExports` returns one page of a feed's exports, newest first. Pass `page.NextCursor` as `Cursor` to get the next page; it is empty on the last one.

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

On the first page, the newest hourly export has the ID `latest`, a moving alias for the freshest export. Its `Date` and `Hour` give its stable ID, such as `2026-10-14/21`.

## Downloads

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

## Retries

The client retries a failed request up to twice, backing off exponentially with jitter and honoring `Retry-After`:

| Requests                    | Retried on                                                                                   |
| --------------------------- | -------------------------------------------------------------------------------------------- |
| Account and feed requests   | 500, 502, 503, and 504, most transport errors, and 429 with `Retry-After`                    |
| Billed lookups and `Health` | Only 429 with `Retry-After`, which comes before any work, so a lookup is never charged twice |

A 429 without `Retry-After` means the monthly request budget is spent or too many streams are open, so it is returned at once. Downloads from the storage host are never retried.

Set the number of retries with `WithMaxRetries`. Zero disables them:

```go
client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"),
	synthient.WithMaxRetries(4),
)
if err != nil {
	log.Fatal(err)
}
```

## Timeouts

The client sets no overall timeout, because streams and downloads are long-lived. Bound a call with a context deadline:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

info, err := client.LookupIP(ctx, netip.MustParseAddr("8.8.8.8"))
```

The default HTTP client waits at most one minute for response headers.

## Error Handling

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

### Error Types

- [`*APIError`](https://pkg.go.dev/github.com/synthient/synthient-go#APIError): the API, or the storage host during `Download`, returned a non-success status.
- [`ErrMissingAPIKey`](https://pkg.go.dev/github.com/synthient/synthient-go#ErrMissingAPIKey): `NewClient` was given an empty key.
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

## Server Selection

### Override Server URL Per-Client

The default server is `https://api.synthient.com/api/v4`. Use `WithBaseURL` to send requests elsewhere, such as a mock server in tests:

```go
client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"),
	synthient.WithBaseURL("http://localhost:8080/api/v4"),
)
if err != nil {
	log.Fatal(err)
}
```

## Custom HTTP Client

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

## Debugging

`WithResponseHook` calls a function after every API response, including retries, pages, and stream handshakes, with the status, request ID, rate limit, and headers:

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

The hook must be safe for concurrent use.

> [!WARNING]
> Response headers can contain secrets, such as presigned download URLs. Avoid logging `meta.Header` in production.

## gRPC Schema

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

# Development

## Maturity

This SDK follows [Semantic Versioning](https://semver.org). Breaking changes ship only in a new major version, with a new import path such as `github.com/synthient/synthient-go/v2`.

## Contributions

Bug reports and feature requests are welcome as [GitHub issues](https://github.com/synthient/synthient-go/issues). For questions about the API or your account, contact [Synthient support](https://docs.synthient.com/misc/support).

## License

[Apache 2.0](LICENSE)
