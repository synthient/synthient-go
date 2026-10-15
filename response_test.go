package synthient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestResponseHookSeesRetriesPagesAndDecodeFailures(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		w.Header().Set("X-Request-ID", fmt.Sprintf("req-%d", n))
		w.Header().Set("Ratelimit-Limit", "200")
		w.Header().Set("Ratelimit-Remaining", "199")
		w.Header().Set("Ratelimit-Reset", "10")
		if n == 1 {
			w.Header().Set("Retry-After", "3")
			writeJSON(w, http.StatusTooManyRequests, `{}`)
			return
		}
		if r.URL.Query().Get("cursor") == "page2" {
			writeJSON(w, http.StatusOK, `{"feeds":`)
			return
		}
		writeJSON(w, http.StatusOK, `{"feeds":[{"id":"2026-05-03"}],"next_cursor":"page2"}`)
	})
	ctx := t.Context()
	var responses []ResponseMetadata
	c := newTestClient(t, srv, WithResponseHook(func(gotCtx context.Context, meta ResponseMetadata) {
		if gotCtx != ctx {
			t.Error("hook received a different operation context")
		}
		responses = append(responses, meta)
		meta.Header.Set("Retry-After", "0")
	}))
	c.wait = func(_ context.Context, delay time.Duration) error {
		if delay < 3*time.Second {
			t.Errorf("hook changed retry policy: delay = %v", delay)
		}
		return nil
	}
	var iterErr error
	for _, err := range c.Proxies.AllExports(ctx) {
		if err != nil {
			iterErr = err
		}
	}
	if iterErr == nil || len(responses) != 3 {
		t.Fatalf("got %d responses, iteration error = %v", len(responses), iterErr)
	}
	for i, meta := range responses {
		wantStatus := http.StatusOK
		if i == 0 {
			wantStatus = http.StatusTooManyRequests
		}
		if meta.StatusCode != wantStatus || meta.RequestID != fmt.Sprintf("req-%d", i+1) {
			t.Errorf("response %d = %+v", i, meta)
		}
		if meta.Header.Get("Ratelimit-Limit") != "200" || meta.Header.Get("Ratelimit-Remaining") != "199" || meta.Header.Get("Ratelimit-Reset") != "10" {
			t.Errorf("missing rate limit headers: %v", meta.Header)
		}
		want := RateLimit{Limit: 200, Remaining: 199, Reset: 10 * time.Second}
		if meta.RateLimit == nil || *meta.RateLimit != want {
			t.Errorf("response %d rate limit = %+v, want %+v", i, meta.RateLimit, want)
		}
	}
}

func TestParseRateLimit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		header http.Header
		want   *RateLimit
	}{
		{
			name:   "all headers",
			header: http.Header{"Ratelimit-Limit": {"20"}, "Ratelimit-Remaining": {"0"}, "Ratelimit-Reset": {"3"}},
			want:   &RateLimit{Limit: 20, Remaining: 0, Reset: 3 * time.Second},
		},
		{
			name:   "negative reset",
			header: http.Header{"Ratelimit-Limit": {"20"}, "Ratelimit-Remaining": {"5"}, "Ratelimit-Reset": {"-1"}},
			want:   &RateLimit{Limit: 20, Remaining: 5},
		},
		{name: "no headers", header: http.Header{}},
		{name: "missing reset", header: http.Header{"Ratelimit-Limit": {"20"}, "Ratelimit-Remaining": {"5"}}},
		{name: "malformed limit", header: http.Header{"Ratelimit-Limit": {"lots"}, "Ratelimit-Remaining": {"5"}, "Ratelimit-Reset": {"3"}}},
	}
	for _, tt := range tests {
		got := parseRateLimit(tt.header)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: parseRateLimit = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

func TestResponseHookConcurrentRequests(t *testing.T) {
	t.Parallel()
	type requestKey struct{}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"X-Request-Id": {fmt.Sprint(req.Context().Value(requestKey{}))}},
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Request:    req,
		}, nil
	})
	var calls atomic.Int32
	c, err := NewClient(testAPIKey,
		WithHTTPClient(&http.Client{Transport: transport}),
		WithResponseHook(func(ctx context.Context, meta ResponseMetadata) {
			calls.Add(1)
			if meta.RequestID != fmt.Sprint(ctx.Value(requestKey{})) {
				t.Errorf("metadata mixed between operations: %+v", meta)
			}
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			_, callErr := c.Account(context.WithValue(t.Context(), requestKey{}, i))
			if callErr != nil {
				t.Error(callErr)
			}
		})
	}
	wg.Wait()
	if calls.Load() != 20 {
		t.Errorf("hook called %d times, want 20", calls.Load())
	}
}
