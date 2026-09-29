package synthient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testAPIKey = "test-key"

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(t *testing.T, srv *httptest.Server, opts ...Option) *Client {
	t.Helper()
	opts = append([]Option{WithBaseURL(srv.URL + "/api/v4")}, opts...)
	c, err := NewClient(testAPIKey, opts...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.wait = func(ctx context.Context, _ time.Duration) error {
		return ctx.Err()
	}
	return c
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}
