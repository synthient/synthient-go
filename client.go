package synthient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the base URL of the Synthient HTTP API.
const DefaultBaseURL = "https://api.synthient.com/api/v4"

// ErrMissingAPIKey is returned by NewClient when the API key is empty.
var ErrMissingAPIKey = errors.New("synthient: missing API key")

const (
	defaultMaxRetries = 2
	maxResponseSize   = 16 << 20
	maxErrorSize      = 64 << 10
)

// Client is a Synthient API client. Create one with NewClient. A Client is
// safe for concurrent use and should be reused.
type Client struct {
	// Proxies is the residential, mobile, and datacenter proxy feed.
	Proxies *Feed[ProxyEvent]
	// Anonymizers is the VPN and anonymizer IP range feed.
	Anonymizers *Feed[AnonymizerEvent]
	// Torrents is the BitTorrent DHT and tracker feed.
	Torrents *Feed[TorrentEvent]
	// JA4T is the JA4T TCP fingerprint feed.
	JA4T *Feed[JA4TEvent]
	// HeliosHTTP is the Helios honeypot HTTP capture feed.
	HeliosHTTP *Feed[HeliosHTTPEvent]
	// HeliosHTTPS is the Helios honeypot TLS ClientHello capture feed.
	HeliosHTTPS *Feed[HeliosHTTPSEvent]
	// HeliosDNS is the Helios honeypot tunnel destination feed. Despite its
	// name, it does not carry DNS queries.
	HeliosDNS *Feed[HeliosDNSEvent]
	// HeliosADB is the Helios honeypot Android Debug Bridge command feed.
	HeliosADB *Feed[HeliosADBEvent]

	apiKey     string
	baseURL    *url.URL
	userAgent  string
	maxRetries int
	api        *http.Client
	storage    *http.Client
	wait       func(ctx context.Context, d time.Duration) error
	onResponse func(context.Context, ResponseMetadata)
}

// Option configures a Client.
type Option func(*config)

type config struct {
	baseURL    string
	httpClient *http.Client
	maxRetries int
	userAgent  string
	onResponse func(context.Context, ResponseMetadata)
}

// WithBaseURL sets the API base URL, including the version path.
// The default is DefaultBaseURL.
func WithBaseURL(u string) Option {
	return func(c *config) {
		c.baseURL = u
	}
}

// WithHTTPClient sets the HTTP client used to send requests. The client is
// copied and never modified. It should not set http.Client.Timeout, which
// would also cut off streams and downloads; use context deadlines instead.
// The default client bounds the wait for response headers to one minute.
// Custom transports must not replay requests whose bodies cannot be rewound.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *config) {
		c.httpClient = hc
	}
}

// WithMaxRetries sets how many times a failed request is retried.
// The default is 2. Zero disables retries.
func WithMaxRetries(n int) Option {
	return func(c *config) {
		c.maxRetries = n
	}
}

// WithUserAgent appends s, such as "my-app/1.2", to the User-Agent header.
func WithUserAgent(s string) Option {
	return func(c *config) {
		c.userAgent = s
	}
}

// NewClient returns a client that authenticates with apiKey. It performs no
// network I/O and returns an error if the key is empty or an option is
// invalid.
func NewClient(apiKey string, opts ...Option) (*Client, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	cfg := config{baseURL: DefaultBaseURL, maxRetries: defaultMaxRetries}
	for _, opt := range opts {
		opt(&cfg)
	}

	baseURL, err := parseBaseURL(cfg.baseURL)
	if err != nil {
		return nil, err
	}
	if cfg.maxRetries < 0 {
		return nil, errors.New("synthient: max retries must not be negative")
	}

	hc := cfg.httpClient
	if hc == nil {
		hc = &http.Client{Transport: defaultTransport()}
	}
	api := *hc
	api.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	storage := *hc
	storage.CheckRedirect = storageRedirectPolicy(hc.CheckRedirect)

	userAgent := "synthient-go/" + Version
	if cfg.userAgent != "" {
		userAgent += " " + cfg.userAgent
	}

	c := &Client{
		apiKey:     apiKey,
		baseURL:    baseURL,
		userAgent:  userAgent,
		maxRetries: cfg.maxRetries,
		api:        &api,
		storage:    &storage,
		wait:       sleep,
		onResponse: cfg.onResponse,
	}
	c.Proxies = &Feed[ProxyEvent]{client: c, path: "/feeds/proxies"}
	c.Anonymizers = &Feed[AnonymizerEvent]{client: c, path: "/feeds/anonymizers"}
	c.Torrents = &Feed[TorrentEvent]{client: c, path: "/feeds/torrents"}
	c.JA4T = &Feed[JA4TEvent]{client: c, path: "/feeds/ja4t"}
	c.HeliosHTTP = &Feed[HeliosHTTPEvent]{client: c, path: "/feeds/helio/http"}
	c.HeliosHTTPS = &Feed[HeliosHTTPSEvent]{client: c, path: "/feeds/helio/https"}
	c.HeliosDNS = &Feed[HeliosDNSEvent]{client: c, path: "/feeds/helio/dns"}
	c.HeliosADB = &Feed[HeliosADBEvent]{client: c, path: "/feeds/helio/adb"}
	return c, nil
}

func parseBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSuffix(raw, "/"))
	if err != nil {
		return nil, fmt.Errorf("synthient: invalid base URL: %w", err)
	}
	validScheme := u.Scheme == "https" || u.Scheme == "http"
	if !validScheme || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("synthient: invalid base URL %q", raw)
	}
	return u, nil
}

func defaultTransport() http.RoundTripper {
	t, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return http.DefaultTransport
	}
	t = t.Clone()
	t.ResponseHeaderTimeout = time.Minute
	return t
}

type request struct {
	method string
	path   string
	query  url.Values
	body   []byte
	accept string
	policy retryPolicy
}

func (c *Client) attempt(ctx context.Context, r request) (*http.Response, error) {
	target := c.baseURL.String() + r.path
	if len(r.query) > 0 {
		target += "?" + r.query.Encode()
	}

	var body io.Reader = http.NoBody
	if r.body != nil {
		body = bytes.NewReader(r.body)
	}
	req, err := http.NewRequestWithContext(ctx, r.method, target, body)
	if err != nil {
		return nil, fmt.Errorf("synthient: build request: %w", err)
	}
	if r.policy == retryRateLimit {
		// A non-rewindable body prevents net/http from replaying billed GETs
		// on a reused connection. An empty GET still sends zero body bytes.
		req.Body = io.NopCloser(bytes.NewReader(r.body))
		req.GetBody = nil
	}

	accept := r.accept
	if accept == "" {
		accept = "application/json"
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", accept)
	if r.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.api.Do(req)
	if err == nil && c.onResponse != nil {
		c.onResponse(ctx, ResponseMetadata{
			StatusCode: resp.StatusCode,
			RequestID:  resp.Header.Get("X-Request-ID"),
			RateLimit:  parseRateLimit(resp.Header),
			Header:     resp.Header.Clone(),
		})
	}
	return resp, err
}

func (c *Client) doJSON(ctx context.Context, r request, v any) error {
	resp, err := c.send(ctx, r)
	if err != nil {
		return err
	}
	defer closeBody(resp)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return newAPIError(resp)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return fmt.Errorf("synthient: read response: %w", err)
	}
	if len(body) > maxResponseSize {
		return fmt.Errorf("synthient: response body exceeds %d bytes", maxResponseSize)
	}
	err = json.Unmarshal(body, v)
	if err != nil {
		return fmt.Errorf("synthient: decode response: %w", err)
	}
	return nil
}

func closeBody(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorSize))
	_ = resp.Body.Close()
}
