// Package synthient is the Go client for the Synthient API: IP and domain
// intelligence, account details, and the Synthient data feeds.
//
// Create a client with an API key and reuse it for every request:
//
//	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
//	if err != nil {
//		log.Fatal(err)
//	}
//	info, err := client.LookupIP(ctx, netip.MustParseAddr("8.8.8.8"))
//	if err != nil {
//		log.Fatal(err)
//	}
//	if info.Intelligence != nil {
//		fmt.Println(info.Intelligence.RiskScore)
//	}
//
// IPInfo.Network, IPInfo.Location, and IPInfo.Intelligence are nil when the
// API has no data for that section. Check presence before reading their fields.
//
// # Errors
//
// A request that fails with a non-success HTTP status returns an *APIError
// with the status code, the problem detail, the request ID, and any
// Retry-After delay:
//
//	apiErr, ok := errors.AsType[*synthient.APIError](err)
//	if ok && apiErr.StatusCode == http.StatusPaymentRequired {
//		log.Print("out of lookup credits")
//	}
//
// APIError.Error contains only the HTTP status. Server-provided details are
// available in its fields and can contain sensitive data. Body read failures
// preserve errors.Is checks for context cancellation and deadlines.
// WithResponseHook exposes request IDs and rate limits on both successful and
// failed API responses, including each retry and page.
//
// # Retries
//
// Account and feed requests are retried on 500, 502, 503, and 504
// responses, on transport errors other than TLS certificate failures and
// HTTP responses to HTTPS requests, and on 429 responses that carry
// Retry-After. A failure while reading a response body is not retried, and
// neither is the request Download sends to the storage host. Billed lookups
// (LookupIP, LookupIPs, and LookupDomain) and Health are retried only on 429
// responses that carry Retry-After, which the rate limiter returns before
// doing any work. A 429 without Retry-After means the
// key's monthly request budget is spent or it has too many open streams, so
// it is returned without a retry. Billed requests have non-rewindable bodies
// to prevent implicit transport replays; custom transports must honor that
// restriction.
// Retries back off exponentially with jitter, honor Retry-After, and stop
// when the context is done. Set the retry count with WithMaxRetries.
//
// # Feeds
//
// Each feed, such as Client.Proxies, has a real-time stream and hourly
// parquet exports that roll up into daily exports:
//
//	for event, err := range client.Proxies.Stream(ctx) {
//		if err != nil {
//			log.Fatal(err)
//		}
//		fmt.Println(event.IP, event.Provider)
//	}
//
// Download fetches an export's parquet file. The API key is sent only to the
// Synthient API; the file itself comes from a presigned storage URL that is
// requested without it.
//
// # Timeouts
//
// The client sets no overall request timeout, because streams and downloads
// are long-lived. Bound calls with a context deadline instead.
package synthient
