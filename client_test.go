package synthient

import (
	"errors"
	"net/http"
	"testing"
)

func TestNewClientRequiresAPIKey(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"", "   "} {
		_, err := NewClient(key)
		if !errors.Is(err, ErrMissingAPIKey) {
			t.Errorf("NewClient(%q) error = %v, want ErrMissingAPIKey", key, err)
		}
	}
}

func TestNewClientRejectsInvalidOptions(t *testing.T) {
	t.Parallel()
	tests := map[string]Option{
		"unsupported scheme": WithBaseURL("ftp://api.synthient.com"),
		"missing host":       WithBaseURL("https://"),
		"query":              WithBaseURL("https://api.synthient.com/api/v4?debug=1"),
		"unparsable":         WithBaseURL("https://api.synthient.com/%zz"),
		"negative retries":   WithMaxRetries(-1),
	}
	for name, opt := range tests {
		_, err := NewClient(testAPIKey, opt)
		if err == nil {
			t.Errorf("%s: NewClient succeeded, want an error", name)
		}
	}
}

func TestNewClientDefaults(t *testing.T) {
	t.Parallel()
	c, err := NewClient(testAPIKey)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.baseURL.String() != DefaultBaseURL {
		t.Errorf("base URL = %q, want %q", c.baseURL, DefaultBaseURL)
	}
	if c.maxRetries != defaultMaxRetries {
		t.Errorf("max retries = %d, want %d", c.maxRetries, defaultMaxRetries)
	}
}

func TestRequestHeadersAndPath(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		want := map[string]string{
			"X-Api-Key":  testAPIKey,
			"User-Agent": "synthient-go/" + Version + " my-app/1.2",
			"Accept":     "application/json",
		}
		for name, value := range want {
			got := r.Header.Get(name)
			if got != value {
				t.Errorf("header %s = %q, want %q", name, got, value)
			}
		}
		if r.URL.Path != "/api/v4/account/me" {
			t.Errorf("path = %q, want /api/v4/account/me", r.URL.Path)
		}
		writeJSON(w, http.StatusOK, `{}`)
	})
	c := newTestClient(t, srv, WithUserAgent("my-app/1.2"))

	_, err := c.Account(t.Context())
	if err != nil {
		t.Fatalf("Account: %v", err)
	}
}

func TestWithHTTPClientIsNotModified(t *testing.T) {
	t.Parallel()
	hc := &http.Client{}
	_, err := NewClient(testAPIKey, WithHTTPClient(hc))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if hc.CheckRedirect != nil {
		t.Error("NewClient modified the caller's http.Client")
	}
}
