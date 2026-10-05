package synthient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBilledLookupsDoNotReplayOnReusedConnection(t *testing.T) {
	t.Parallel()
	calls := map[string]func(context.Context, *Client) error{
		"IP": func(ctx context.Context, c *Client) error {
			_, err := c.LookupIP(ctx, netip.MustParseAddr("1.1.1.1"))
			return err
		},
		"domain": func(ctx context.Context, c *Client) error {
			_, err := c.LookupDomain(ctx, "example.com", nil)
			return err
		},
		"batch": func(ctx context.Context, c *Client) error {
			_, err := c.LookupIPs(ctx, []netip.Addr{netip.MustParseAddr("1.1.1.1")})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var lookups atomic.Int32
			srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				if strings.Contains(r.URL.Path, "/lookup/") && lookups.Add(1) == 1 {
					conn, _, err := http.NewResponseController(w).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = conn.Close()
					return
				}
				writeJSON(w, http.StatusOK, `{}`)
			})
			c := newTestClient(t, srv, WithMaxRetries(0))
			_, err := c.Account(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			var reused atomic.Bool
			ctx := httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{
				GotConn: func(info httptrace.GotConnInfo) {
					if info.Reused {
						reused.Store(true)
					}
				},
			})
			err = call(ctx, c)
			if err == nil || lookups.Load() != 1 {
				t.Errorf("lookup made %d requests, error = %v; want one request and a transport error", lookups.Load(), err)
			}
			if !reused.Load() {
				t.Error("lookup did not reuse the warmed connection")
			}
		})
	}
}

func TestBilledRequestsPreserveWireFormatAndRetry429(t *testing.T) {
	t.Parallel()
	for _, http2 := range []bool{false, true} {
		for _, batch := range []bool{false, true} {
			name := "HTTP1/GET"
			if http2 {
				name = "HTTP2/GET"
			}
			if batch {
				name = strings.ReplaceAll(name, "GET", "POST")
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				var requests atomic.Int32
				srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					wantMethod, wantBody := http.MethodGet, ""
					if batch {
						wantMethod, wantBody = http.MethodPost, `{"ips":["1.1.1.1"]}`
					}
					wantLength := int64(len(wantBody))
					if http2 && !batch {
						// HTTP/2 closes the empty body with END_STREAM instead of
						// advertising a Content-Length for a non-rewindable reader.
						wantLength = -1
					}
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					if r.Method != wantMethod || string(body) != wantBody || r.ContentLength != wantLength || len(r.TransferEncoding) != 0 {
						t.Errorf("request = %s, body %q, length %d, transfer encoding %v", r.Method, body, r.ContentLength, r.TransferEncoding)
					}
					if (r.ProtoMajor == 2) != http2 {
						t.Errorf("protocol = %s, want HTTP2: %v", r.Proto, http2)
					}
					if requests.Add(1) == 1 {
						w.Header().Set("Retry-After", "1")
						writeJSON(w, http.StatusTooManyRequests, `{}`)
						return
					}
					writeJSON(w, http.StatusOK, `{}`)
				}))
				srv.EnableHTTP2 = http2
				srv.StartTLS()
				t.Cleanup(srv.Close)
				c := newTestClient(t, srv, WithHTTPClient(srv.Client()))
				var err error
				if batch {
					_, err = c.LookupIPs(t.Context(), []netip.Addr{netip.MustParseAddr("1.1.1.1")})
				} else {
					_, err = c.LookupIP(t.Context(), netip.MustParseAddr("1.1.1.1"))
				}
				if err != nil || requests.Load() != 2 {
					t.Errorf("lookup made %d requests, error = %v; want two requests and success", requests.Load(), err)
				}
			})
		}
	}
}

func TestRetryPolicy(t *testing.T) {
	t.Parallel()
	account := func(ctx context.Context, c *Client) error {
		_, err := c.Account(ctx)
		return err
	}
	lookupIP := func(ctx context.Context, c *Client) error {
		_, err := c.LookupIP(ctx, netip.MustParseAddr("1.1.1.1"))
		return err
	}
	tests := []struct {
		name         string
		statuses     []int
		retryAfter   string
		call         func(context.Context, *Client) error
		wantRequests int32
		wantStatus   int
	}{
		{name: "free operation retries 503", statuses: []int{503, 502, 200}, call: account, wantRequests: 3},
		{name: "free operation gives up", statuses: []int{503, 503, 503, 200}, call: account, wantRequests: 3, wantStatus: 503},
		{name: "free operation does not retry 400", statuses: []int{400, 200}, call: account, wantRequests: 1, wantStatus: 400},
		{name: "free operation retries 429 with Retry-After", statuses: []int{429, 200}, retryAfter: "0", call: account, wantRequests: 2},
		{name: "free operation does not retry 429 without Retry-After", statuses: []int{429, 200}, call: account, wantRequests: 1, wantStatus: 429},
		{name: "billed lookup retries 429 with Retry-After", statuses: []int{429, 200}, retryAfter: "1", call: lookupIP, wantRequests: 2},
		{name: "billed lookup does not retry 429 without Retry-After", statuses: []int{429, 200}, call: lookupIP, wantRequests: 1, wantStatus: 429},
		{name: "billed lookup does not retry 500", statuses: []int{500, 200}, call: lookupIP, wantRequests: 1, wantStatus: 500},
		{name: "billed lookup does not retry 503", statuses: []int{503, 200}, call: lookupIP, wantRequests: 1, wantStatus: 503},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var requests atomic.Int32
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				status := tt.statuses[requests.Add(1)-1]
				if status == http.StatusTooManyRequests && tt.retryAfter != "" {
					w.Header().Set("Retry-After", tt.retryAfter)
				}
				writeJSON(w, status, `{}`)
			})
			c := newTestClient(t, srv)

			err := tt.call(t.Context(), c)
			if requests.Load() != tt.wantRequests {
				t.Errorf("made %d requests, want %d", requests.Load(), tt.wantRequests)
			}
			if tt.wantStatus == 0 {
				if err != nil {
					t.Errorf("call failed: %v", err)
				}
				return
			}
			apiErr, ok := errors.AsType[*APIError](err)
			if !ok || apiErr.StatusCode != tt.wantStatus {
				t.Errorf("error = %v, want an APIError with status %d", err, tt.wantStatus)
			}
		})
	}
}

func TestRetryHonorsRetryAfter(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			w.Header().Set("Retry-After", "3")
			writeJSON(w, http.StatusTooManyRequests, `{"title":"Too Many Requests","status":429}`)
			return
		}
		writeJSON(w, http.StatusOK, `{}`)
	})
	c := newTestClient(t, srv)
	var mu sync.Mutex
	var delays []time.Duration
	c.wait = func(_ context.Context, d time.Duration) error {
		mu.Lock()
		defer mu.Unlock()
		delays = append(delays, d)
		return nil
	}

	_, err := c.Account(t.Context())
	if err != nil {
		t.Fatalf("Account: %v", err)
	}
	if len(delays) != 1 || delays[0] < 3*time.Second {
		t.Errorf("delays = %v, want one delay of at least 3s", delays)
	}
}

func TestRetryStopsWhenServerAsksForTooLong(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Retry-After", "120")
		writeJSON(w, http.StatusTooManyRequests, `{"title":"Too Many Requests","status":429}`)
	})
	c := newTestClient(t, srv)

	_, err := c.Account(t.Context())
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok || apiErr.RetryAfter != 120*time.Second {
		t.Fatalf("error = %v, want an APIError with RetryAfter 2m0s", err)
	}
	if requests.Load() != 1 {
		t.Errorf("made %d requests, want 1", requests.Load())
	}
}

func TestRetryStopsBeforeDeadline(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeJSON(w, http.StatusServiceUnavailable, `{}`)
	})
	c := newTestClient(t, srv)
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	_, err := c.Account(ctx)
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok || apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("error = %v, want an APIError with status 503", err)
	}
	if requests.Load() != 1 {
		t.Errorf("made %d requests, want 1", requests.Load())
	}
}

func TestRetryCanceledDuringBackoff(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusServiceUnavailable, `{}`)
	})
	c := newTestClient(t, srv)
	ctx, cancel := context.WithCancel(t.Context())
	c.wait = func(ctx context.Context, d time.Duration) error {
		cancel()
		return sleep(ctx, d)
	}

	_, err := c.Account(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestRetryDisabled(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeJSON(w, http.StatusServiceUnavailable, `{}`)
	})
	c := newTestClient(t, srv, WithMaxRetries(0))

	_, err := c.Account(t.Context())
	if err == nil {
		t.Fatal("Account succeeded, want an error")
	}
	if requests.Load() != 1 {
		t.Errorf("made %d requests, want 1", requests.Load())
	}
}

func TestRetryTransportErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		call         func(context.Context, *Client) error
		wantRequests int32
		wantErr      bool
	}{
		{
			name: "free operation retries",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.Account(ctx)
				return err
			},
			wantRequests: 2,
		},
		{
			name: "billed lookup does not retry",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.LookupIP(ctx, netip.MustParseAddr("1.1.1.1"))
				return err
			},
			wantRequests: 1,
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var requests atomic.Int32
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				if requests.Add(1) == 1 {
					conn, _, err := http.NewResponseController(w).Hijack()
					if err != nil {
						t.Errorf("hijack: %v", err)
						return
					}
					_ = conn.Close()
					return
				}
				writeJSON(w, http.StatusOK, `{}`)
			})
			c := newTestClient(t, srv)

			err := tt.call(t.Context(), c)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, want error: %v", err, tt.wantErr)
			}
			if requests.Load() != tt.wantRequests {
				t.Errorf("made %d requests, want %d", requests.Load(), tt.wantRequests)
			}
		})
	}
}

func TestBackoff(t *testing.T) {
	t.Parallel()
	for attempt := range 10 {
		base := min(time.Second<<attempt, maxBackoff)
		d := backoff(attempt)
		if d < base*3/4 || d > base*5/4 {
			t.Errorf("backoff(%d) = %v, want within 25%% of %v", attempt, d, base)
		}
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()
	tests := map[string]time.Duration{
		"":                              0,
		"5":                             5 * time.Second,
		" 12 ":                          12 * time.Second,
		"-3":                            0,
		"soon":                          0,
		"99999999999":                   (1 << 31) * time.Second,
		"Wed, 21 Oct 2015 07:28:00 GMT": 0,
	}
	for value, want := range tests {
		got := parseRetryAfter(http.Header{"Retry-After": {value}})
		if got != want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", value, got, want)
		}
	}

	future := time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)
	got := parseRetryAfter(http.Header{"Retry-After": {future}})
	if got < 59*time.Minute || got > time.Hour {
		t.Errorf("parseRetryAfter(%q) = %v, want about 1h", future, got)
	}
}
