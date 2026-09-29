package synthient

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// storageURL returns a URL for srv on a different host name than the API
// test server, so the HTTP client treats it as a separate origin.
func storageURL(srv *httptest.Server, path string) string {
	return strings.Replace(srv.URL, "127.0.0.1", "localhost", 1) + path + "?X-Amz-Signature=secret-signature"
}

func redirectTo(t *testing.T, location string) *httptest.Server {
	t.Helper()
	return newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != testAPIKey {
			t.Errorf("API request X-Api-Key = %q, want %q", r.Header.Get("X-Api-Key"), testAPIKey)
		}
		http.Redirect(w, r, location, http.StatusTemporaryRedirect)
	})
}

func TestDownloadDoesNotSendAPIKeyToStorage(t *testing.T) {
	t.Parallel()
	var storageRequests atomic.Int32
	storage := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		storageRequests.Add(1)
		if r.Header.Get("X-Api-Key") != "" {
			t.Error("storage host received the API key")
		}
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "synthient-go/") {
			t.Errorf("storage User-Agent = %q", r.Header.Get("User-Agent"))
		}
		_, _ = io.WriteString(w, "PAR1 parquet bytes")
	})
	var apiPath atomic.Value
	api := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		apiPath.Store(r.URL.Path)
		http.Redirect(w, r, storageURL(storage, "/proxies/2026-05-03.parquet"), http.StatusTemporaryRedirect)
	})
	c := newTestClient(t, api)

	body, err := c.Proxies.Download(t.Context(), "2026-05-03")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	data, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	if string(data) != "PAR1 parquet bytes" {
		t.Errorf("Download = %q, want the storage body", data)
	}
	if apiPath.Load() != "/api/v4/feeds/proxies/export/2026-05-03" {
		t.Errorf("API path = %v", apiPath.Load())
	}
	if storageRequests.Load() != 1 {
		t.Errorf("storage received %d requests, want 1", storageRequests.Load())
	}
}

func TestDownloadURL(t *testing.T) {
	t.Parallel()
	location := "https://storage.example/proxies/latest.parquet?X-Amz-Signature=secret-signature"
	c := newTestClient(t, redirectTo(t, location))

	u, err := c.Proxies.DownloadURL(t.Context(), LatestExport)
	if err != nil {
		t.Fatalf("DownloadURL: %v", err)
	}
	if u.String() != location {
		t.Errorf("DownloadURL = %q, want %q", u, location)
	}
}

func TestDownloadRejectsInsecureRedirect(t *testing.T) {
	t.Parallel()
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://storage.example/proxies.parquet", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(api.Close)
	c, err := NewClient(testAPIKey, WithBaseURL(api.URL+"/api/v4"), WithHTTPClient(api.Client()))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = c.Proxies.DownloadURL(t.Context(), LatestExport)
	if err == nil {
		t.Fatal("DownloadURL accepted an http redirect from an https API")
	}
}

func TestDownloadRejectsStorageDowngrade(t *testing.T) {
	t.Parallel()
	var insecureRequests int
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		resp := &http.Response{Request: req, Header: http.Header{}, Body: http.NoBody}
		switch req.URL.Host {
		case "api.synthient.com":
			resp.StatusCode = http.StatusTemporaryRedirect
			resp.Header.Set("Location", "https://storage.example/file?signature=secret")
		case "storage.example":
			if req.Header.Get("X-Api-Key") != "" {
				t.Error("storage received API key")
			}
			if req.URL.Scheme == "https" {
				resp.StatusCode = http.StatusTemporaryRedirect
				resp.Header.Set("Location", "http://storage.example/file?signature=secret")
			} else {
				insecureRequests++
				resp.StatusCode = http.StatusOK
			}
		}
		return resp, nil
	})
	hc := &http.Client{Transport: transport}
	c, err := NewClient(testAPIKey, WithHTTPClient(hc))
	if err != nil {
		t.Fatal(err)
	}
	body, err := c.Proxies.Download(t.Context(), LatestExport)
	if body != nil {
		_ = body.Close()
	}
	if err == nil || insecureRequests != 0 {
		t.Fatalf("download made %d insecure requests, error = %v", insecureRequests, err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("error exposed presigned URL: %v", err)
	}
	if hc.CheckRedirect != nil {
		t.Error("client modified the caller's redirect policy")
	}
}

func TestDownloadPreservesStorageRedirectPolicy(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("caller rejected redirect")
	var checks int
	hc := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			checks++
			return wantErr
		},
	}
	storage := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/next", http.StatusTemporaryRedirect)
	})
	c := newTestClient(t, redirectTo(t, storageURL(storage, "/file")), WithHTTPClient(hc))
	body, err := c.Proxies.Download(t.Context(), LatestExport)
	if body != nil {
		_ = body.Close()
	}
	if !errors.Is(err, wantErr) || checks != 1 {
		t.Errorf("redirect checks = %d, error = %v", checks, err)
	}
}

func TestStorageRedirectPolicy(t *testing.T) {
	t.Parallel()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://storage.example/file", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	for _, scheme := range []string{"http", "ftp"} {
		next := request.Clone(t.Context())
		check := storageRedirectPolicy(func(req *http.Request, _ []*http.Request) error {
			req.URL.Scheme = scheme
			return nil
		})
		err = check(next, []*http.Request{request})
		if err == nil {
			t.Errorf("accepted %s URL after caller changed redirect", scheme)
		}
	}
	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = request
	}
	err = storageRedirectPolicy(nil)(request, via)
	if err == nil {
		t.Error("accepted more than ten storage requests")
	}
}

func TestDownloadErrors(t *testing.T) {
	t.Parallel()
	storage := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	tests := []struct {
		name           string
		api            *httptest.Server
		wantStatus     int
		wantRetryAfter time.Duration
	}{
		{
			name: "export not ready",
			api: newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "60")
				writeJSON(w, http.StatusNotFound, `{"title":"Not Found","status":404,"detail":"daily rollup in progress"}`)
			}),
			wantStatus:     http.StatusNotFound,
			wantRetryAfter: time.Minute,
		},
		{
			name: "not a redirect",
			api: newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(w, http.StatusOK, `{}`)
			}),
		},
		{
			name:       "storage rejects",
			api:        redirectTo(t, storageURL(storage, "/proxies.parquet")),
			wantStatus: http.StatusForbidden,
		},
		{
			name: "storage unreachable",
			api:  redirectTo(t, storageURL(closed, "/proxies.parquet")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t, tt.api)

			_, err := c.Proxies.Download(t.Context(), LatestExport)
			if err == nil {
				t.Fatal("Download succeeded, want an error")
			}
			if strings.Contains(err.Error(), "secret-signature") {
				t.Errorf("error %q exposes the presigned URL", err)
			}
			apiErr, ok := errors.AsType[*APIError](err)
			if tt.wantStatus == 0 {
				if ok {
					t.Errorf("error = %v, want a non-API error", err)
				}
				return
			}
			if !ok || apiErr.StatusCode != tt.wantStatus || apiErr.RetryAfter != tt.wantRetryAfter {
				t.Errorf("error = %#v, want status %d and RetryAfter %v", err, tt.wantStatus, tt.wantRetryAfter)
			}
		})
	}
}
