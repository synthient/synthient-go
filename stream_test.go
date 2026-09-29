package synthient

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStream(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/feeds/proxies/stream" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/x-ndjson" {
			t.Errorf("Accept = %q, want application/x-ndjson", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, "\n"+
			`{"ip":"203.0.113.10","provider":"luminati","type":"residential","timestamp":1714780800,"country_code":"US","asn":15169}`+"\n"+
			"\n  \n"+
			`{"ip":"203.0.113.11","asn":4294967295}`+"\n")
	})
	c := newTestClient(t, srv)

	var events []ProxyEvent
	for event, err := range c.Proxies.Stream(t.Context()) {
		if err != nil {
			t.Fatalf("Stream: %v", err)
		}
		events = append(events, event)
	}
	want := []ProxyEvent{
		{IP: "203.0.113.10", Provider: "luminati", Type: "residential", Timestamp: 1714780800, CountryCode: "US", ASN: 15169},
		{IP: "203.0.113.11", ASN: 4294967295},
	}
	if !reflect.DeepEqual(events, want) {
		t.Errorf("events = %+v, want %+v", events, want)
	}
}

func TestStreamHeliosHTTPSDetails(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w,
			`{"domain":"example.com","port":443,"protocol":"https","details":{"sni":"example.com"},"raw":null,"timestamp":1778200137487}`+"\n"+
				`{"domain":"example.com","port":443,"details":"{\"server_name\":\"example.com\"}","raw":"FgMBAH8BAA=="}`+"\n")
	})
	c := newTestClient(t, srv)

	var details []string
	for event, err := range c.HeliosHTTPS.Stream(t.Context()) {
		if err != nil {
			t.Fatalf("Stream: %v", err)
		}
		details = append(details, string(event.Details))
		if event.Timestamp != 0 && event.Timestamp.Time().Year() != 2026 {
			t.Errorf("timestamp %d decoded to %v", event.Timestamp, event.Timestamp.Time())
		}
	}
	want := []string{`{"sni":"example.com"}`, `"{\"server_name\":\"example.com\"}"`}
	if !reflect.DeepEqual(details, want) {
		t.Errorf("details = %q, want %q", details, want)
	}
}

func TestStreamErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		status     int
		body       string
		wantEvents int
		wantStatus int
		wantIs     error
	}{
		{
			name:       "missing scope",
			status:     http.StatusForbidden,
			body:       `{"title":"Forbidden","status":403,"detail":"Insufficient scope for this resource. Requires one of: PROXY_FIREHOSE"}`,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "truncated line",
			status:     http.StatusOK,
			body:       `{"ip":"203.0.113.10"}` + "\n" + `{"ip":`,
			wantEvents: 1,
		},
		{
			name:   "invalid json",
			status: http.StatusOK,
			body:   "not json\n",
		},
		{
			name:   "oversized line",
			status: http.StatusOK,
			body:   `{"ip":"` + strings.Repeat("1", maxEventSize) + `"}` + "\n",
			wantIs: bufio.ErrTooLong,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			})
			c := newTestClient(t, srv)

			events := 0
			var streamErr error
			for _, err := range c.Proxies.Stream(t.Context()) {
				if err != nil {
					streamErr = err
					break
				}
				events++
			}
			if events != tt.wantEvents {
				t.Errorf("yielded %d events, want %d", events, tt.wantEvents)
			}
			if streamErr == nil {
				t.Fatal("Stream yielded no error")
			}
			apiErr, ok := errors.AsType[*APIError](streamErr)
			if tt.wantStatus != 0 && (!ok || apiErr.StatusCode != tt.wantStatus) {
				t.Errorf("error = %v, want an APIError with status %d", streamErr, tt.wantStatus)
			}
			if tt.wantStatus == 0 && ok {
				t.Errorf("error = %v, want a non-API error", streamErr)
			}
			if tt.wantIs != nil && !errors.Is(streamErr, tt.wantIs) {
				t.Errorf("error = %v, want %v", streamErr, tt.wantIs)
			}
		})
	}
}

func TestStreamRetriesConnect(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			writeJSON(w, http.StatusServiceUnavailable, `{"title":"Service Unavailable","status":503}`)
			return
		}
		_, _ = io.WriteString(w, `{"ip":"203.0.113.10"}`+"\n")
	})
	c := newTestClient(t, srv)

	events := 0
	for _, err := range c.Proxies.Stream(t.Context()) {
		if err != nil {
			t.Fatalf("Stream: %v", err)
		}
		events++
	}
	if events != 1 || requests.Load() != 2 {
		t.Errorf("got %d events over %d requests, want 1 event over 2 requests", events, requests.Load())
	}
}

func TestStreamBreakClosesConnection(t *testing.T) {
	t.Parallel()
	closed := make(chan struct{})
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"ip":"203.0.113.10"}`+"\n")
		_ = http.NewResponseController(w).Flush()
		<-r.Context().Done()
		close(closed)
	})
	c := newTestClient(t, srv)

	for range c.Proxies.Stream(t.Context()) {
		break
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("the server did not see the connection close")
	}
}

func TestStreamContextCanceled(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"ip":"203.0.113.10"}`+"\n")
		_ = http.NewResponseController(w).Flush()
		<-r.Context().Done()
	})
	c := newTestClient(t, srv)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	events := 0
	var streamErr error
	for _, err := range c.Proxies.Stream(ctx) {
		if err != nil {
			streamErr = err
			break
		}
		events++
		cancel()
	}
	if events != 1 {
		t.Errorf("yielded %d events, want 1", events)
	}
	if !errors.Is(streamErr, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", streamErr)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func FuzzStream(f *testing.F) {
	f.Add([]byte(`{"ip":"203.0.113.10","asn":15169,"timestamp":1714780800}` + "\n\n"))
	f.Add([]byte("\n\n\n"))
	f.Add([]byte(`{"ip":`))
	f.Add([]byte(`{"files":[{"path":"a","length":-1}],"peers":null}` + "\r\n"))
	f.Fuzz(func(t *testing.T, body []byte) {
		transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       io.NopCloser(bytes.NewReader(body)),
				Request:    r,
			}, nil
		})
		c, err := NewClient(testAPIKey, WithHTTPClient(&http.Client{Transport: transport}))
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		for _, err := range c.Torrents.Stream(t.Context()) {
			if err != nil {
				return
			}
		}
	})
}
