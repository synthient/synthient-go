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
			`{"ip":"203.0.113.10","provider":"luminati","type":"RESIDENTIAL_PROXY","timestamp":1714780800,"country_code":"US","asn":15169}`+"\n"+
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
		{IP: "203.0.113.10", Provider: "luminati", Type: ProxyTypeResidentialProxy, Timestamp: 1714780800, CountryCode: "US", ASN: 15169},
		{IP: "203.0.113.11", ASN: 4294967295},
	}
	if !reflect.DeepEqual(events, want) {
		t.Errorf("events = %+v, want %+v", events, want)
	}
}

func TestStreamHeliosEvents(t *testing.T) {
	t.Parallel()
	newClient := func(t *testing.T, body string) *Client {
		t.Helper()
		srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, body)
		})
		return newTestClient(t, srv)
	}
	meta := HeliosMeta{ProxyIP: "203.0.113.10", Server: "hp-eu-1", PoolID: "prod", Provider: "luminati"}
	const metaJSON = `"meta":{"proxy_ip":"203.0.113.10","server":"hp-eu-1","pool_id":"prod","provider":"luminati"}`

	t.Run("https", func(t *testing.T) {
		t.Parallel()
		c := newClient(t, `{"timestamp":1778200137487,"tunnel_id":17257744507466107949,"domain":"example.com","port":443,"protocol":"https",`+metaJSON+`,`+
			`"details":{"sni":"example.com","handshake_version":"TLS 1.2","cipher_suites":[{"code":4865,"name":"TLS_AES_128_GCM_SHA256"}],"extensions":[{"code":0,"name":"server_name","length":16}],"has_grease":true},"raw":null}`+"\n"+
			`{"timestamp":1778200137488,"details":null,"raw":null}`+"\n")

		events := streamEvents(t, c.HeliosHTTPS)
		want := []HeliosHTTPSEvent{
			{
				Timestamp: 1778200137487,
				TunnelID:  17257744507466107949,
				Domain:    "example.com",
				Port:      443,
				Protocol:  "https",
				Meta:      meta,
				Details: &TLSClientHello{
					SNI:              "example.com",
					HandshakeVersion: "TLS 1.2",
					CipherSuites:     []TLSCodeName{{Code: 4865, Name: "TLS_AES_128_GCM_SHA256"}},
					Extensions:       []TLSExtension{{Code: 0, Name: "server_name", Length: 16}},
					HasGREASE:        true,
				},
			},
			{Timestamp: 1778200137488},
		}
		if !reflect.DeepEqual(events, want) {
			t.Errorf("events = %+v, want %+v", events, want)
		}
		if !events[0].Timestamp.Time().Equal(time.UnixMilli(1778200137487)) {
			t.Errorf("timestamp %d decoded to %v", events[0].Timestamp, events[0].Timestamp.Time())
		}
	})

	t.Run("http", func(t *testing.T) {
		t.Parallel()
		c := newClient(t, `{"timestamp":1778200137487,"protocol":"http","details":{"method":"GET","uri":"/","version":"HTTP/1.1","headers":{"Host":"example.com"}},"raw":"R0VUIC8gSFRUUC8xLjENCkhvc3Q6IGV4YW1wbGUuY29tDQoNCg=="}`+"\n")

		events := streamEvents(t, c.HeliosHTTP)
		want := []HeliosHTTPEvent{{
			Timestamp: 1778200137487,
			Protocol:  "http",
			Details:   HTTPRequestDetails{Method: "GET", URI: "/", Version: "HTTP/1.1", Headers: map[string]string{"Host": "example.com"}},
			Raw:       []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"),
		}}
		if !reflect.DeepEqual(events, want) {
			t.Errorf("events = %+v, want %+v", events, want)
		}
	})

	t.Run("dns", func(t *testing.T) {
		t.Parallel()
		c := newClient(t, `{"timestamp":1714780800123,"tunnel_id":18446744073709551615,"domain":"api.ipify.org","port":443,"protocol":"dns",`+metaJSON+`}`+"\n")

		events := streamEvents(t, c.HeliosDNS)
		want := []HeliosDNSEvent{{Timestamp: 1714780800123, TunnelID: 18446744073709551615, Domain: "api.ipify.org", Port: 443, Protocol: "dns", Meta: meta}}
		if !reflect.DeepEqual(events, want) {
			t.Errorf("events = %+v, want %+v", events, want)
		}
	})
}

func streamEvents[T any](t *testing.T, feed *Feed[T]) []T {
	t.Helper()
	var events []T
	for event, err := range feed.Stream(t.Context()) {
		if err != nil {
			t.Fatalf("Stream: %v", err)
		}
		events = append(events, event)
	}
	return events
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
			name:       "too many open streams",
			status:     http.StatusTooManyRequests,
			body:       `{"title":"Too Many Requests","status":429,"detail":"too many concurrent streams"}`,
			wantStatus: http.StatusTooManyRequests,
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

	for _, err := range c.Proxies.Stream(t.Context()) {
		if err != nil {
			t.Fatalf("Stream: %v", err)
		}
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
