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
// WithResponseHook exposes request IDs and rate limits on every API
// response, including retries and pages.
//
// # Retries
//
// Account and feed requests are retried on 500, 502, 503, and 504
// responses, on transport errors other than TLS certificate failures and
// HTTP responses to HTTPS requests, and on 429 responses that carry
// Retry-After. Billed lookups (LookupIP, LookupIPs, and LookupDomain) and
// Health are retried only on 429 responses with Retry-After, which the rate
// limiter returns before doing any work. A 429 without Retry-After means the
// key's monthly request budget is spent or it has too many open streams.
// Response body read failures and the storage request Download sends are
// never retried. Billed requests use non-rewindable bodies so the transport
// cannot replay them. Retries back off exponentially with jitter, honor
// Retry-After, and stop when the context is done. Set the retry count with
// WithMaxRetries.
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
// # Timeouts
//
// The client sets no overall request timeout, because streams and downloads
// are long-lived. Bound calls with a context deadline instead.
package synthient
