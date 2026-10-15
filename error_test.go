package synthient

import (
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAPIError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		status      int
		header      http.Header
		contentType string
		body        string
		want        APIError
		wantMessage string
	}{
		{
			name:        "problem details",
			status:      http.StatusForbidden,
			header:      http.Header{"X-Request-Id": {"req-header"}},
			contentType: "application/problem+json",
			body:        `{"type":"https://docs.synthient.com/errors/forbidden","title":"Forbidden","status":403,"detail":"API key lacks the required scope for this feed","instance":"req-body"}`,
			want: APIError{
				StatusCode: http.StatusForbidden,
				Type:       "https://docs.synthient.com/errors/forbidden",
				Title:      "Forbidden",
				Detail:     "API key lacks the required scope for this feed",
				RequestID:  "req-header",
			},
			wantMessage: "synthient: 403 Forbidden",
		},
		{
			name:        "request id from instance",
			status:      http.StatusPaymentRequired,
			contentType: "application/problem+json",
			body:        `{"type":"about:blank","title":"Payment Required","status":402,"detail":"quota exhausted","instance":"abc123"}`,
			want: APIError{
				StatusCode: http.StatusPaymentRequired,
				Type:       "about:blank",
				Title:      "Payment Required",
				Detail:     "quota exhausted",
				RequestID:  "abc123",
			},
			wantMessage: "synthient: 402 Payment Required",
		},
		{
			name:        "validation problem",
			status:      http.StatusUnprocessableEntity,
			contentType: "application/problem+json",
			body:        `{"type":"about:blank","title":"Invalid request","status":422,"errors":{"ips":["must contain at most 1000 entries"],"domain":["too long"]}}`,
			want: APIError{
				StatusCode: http.StatusUnprocessableEntity,
				Type:       "about:blank",
				Title:      "Invalid request",
				Errors:     map[string][]string{"ips": {"must contain at most 1000 entries"}, "domain": {"too long"}},
			},
			wantMessage: "synthient: 422 Unprocessable Entity",
		},
		{
			name:        "unexpected errors shape",
			status:      http.StatusBadRequest,
			contentType: "application/json",
			body:        `{"title":"Bad Request","detail":"bad cursor","errors":["cursor"]}`,
			want:        APIError{StatusCode: http.StatusBadRequest, Title: "Bad Request", Detail: "bad cursor"},
			wantMessage: "synthient: 400 Bad Request",
		},
		{
			name:        "plain text edge response",
			status:      http.StatusUnauthorized,
			contentType: "text/plain",
			body:        "401 invalid api key",
			want:        APIError{StatusCode: http.StatusUnauthorized, Detail: "401 invalid api key"},
			wantMessage: "synthient: 401 Unauthorized",
		},
		{
			name:        "plain text router response",
			status:      http.StatusNotFound,
			contentType: "text/plain; charset=utf-8",
			body:        "404 page not found\n",
			want:        APIError{StatusCode: http.StatusNotFound, Detail: "404 page not found"},
			wantMessage: "synthient: 404 Not Found",
		},
		{
			name:        "empty body with retry after",
			status:      http.StatusTooManyRequests,
			header:      http.Header{"Retry-After": {"7"}},
			want:        APIError{StatusCode: http.StatusTooManyRequests, RetryAfter: 7 * time.Second},
			wantMessage: "synthient: 429 Too Many Requests",
		},
		{
			name:        "oversized body",
			status:      http.StatusBadGateway,
			contentType: "text/html",
			body:        strings.Repeat("x", 2*maxErrorSize),
			want:        APIError{StatusCode: http.StatusBadGateway},
			wantMessage: "synthient: 502 Bad Gateway",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				maps.Copy(w.Header(), tt.header)
				if tt.contentType != "" {
					w.Header().Set("Content-Type", tt.contentType)
				}
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			})
			c := newTestClient(t, srv, WithMaxRetries(0))

			_, err := c.Account(t.Context())
			apiErr, ok := errors.AsType[*APIError](err)
			if !ok {
				t.Fatalf("Account error = %v, want an *APIError", err)
			}
			if apiErr.StatusCode != tt.want.StatusCode || apiErr.RequestID != tt.want.RequestID || apiErr.RetryAfter != tt.want.RetryAfter {
				t.Errorf("APIError metadata = %+v, want %+v", apiErr, tt.want)
			}
			if apiErr.Type != tt.want.Type || apiErr.Title != tt.want.Title || apiErr.Detail != tt.want.Detail || !reflect.DeepEqual(apiErr.Errors, tt.want.Errors) {
				t.Errorf("APIError = %+v, want %+v", *apiErr, tt.want)
			}
			if apiErr.Unwrap() != nil {
				t.Errorf("unexpected body read error: %v", apiErr.Unwrap())
			}
			if apiErr.Error() != tt.wantMessage {
				t.Errorf("Error() = %q, want %q", apiErr.Error(), tt.wantMessage)
			}
		})
	}
}

func TestAPIErrorPreservesDeadlineDuringBodyRead(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "req-timeout")
		w.WriteHeader(http.StatusForbidden)
		err := http.NewResponseController(w).Flush()
		if err != nil {
			t.Error(err)
			return
		}
		<-r.Context().Done()
	})
	c := newTestClient(t, srv)
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	_, err := c.Account(ctx)
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok || apiErr.StatusCode != http.StatusForbidden || apiErr.RequestID != "req-timeout" {
		t.Fatalf("error = %v, want HTTP error with status and request ID", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error lost deadline identity: %v", err)
	}
}

func TestAPIErrorStringExcludesServerData(t *testing.T) {
	t.Parallel()
	const secret = "secret-key\nforged log entry https://storage.example/file?signature=secret"
	e := &APIError{
		StatusCode: http.StatusForbidden,
		Title:      secret,
		Detail:     secret,
		Errors:     map[string][]string{secret: {secret}},
		RequestID:  secret,
		cause:      errors.New(secret),
	}
	if e.Error() != "synthient: 403 Forbidden" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestAPIErrorOmitsSecrets(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, `{"title":"Not Found","status":404,"detail":"no data"}`)
	})
	c := newTestClient(t, srv)

	_, err := c.LookupDomain(t.Context(), "private.example")
	if err == nil {
		t.Fatal("LookupDomain succeeded, want an error")
	}
	for _, secret := range []string{testAPIKey, "private.example", srv.URL} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("error %q contains %q", err, secret)
		}
	}
}

func TestSuccessBodyValidation(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"trailing data": `{"email":"a@example.com"} {"email":"b@example.com"}`,
		"truncated":     `{"email":`,
		"oversized":     `{"email":"` + strings.Repeat("a", maxResponseSize) + `"}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(w, http.StatusOK, body)
			})
			c := newTestClient(t, srv)

			_, err := c.Account(t.Context())
			if err == nil {
				t.Fatal("Account succeeded, want an error")
			}
			_, isAPIError := errors.AsType[*APIError](err)
			if isAPIError {
				t.Errorf("error %v is an *APIError, want a decoding error", err)
			}
		})
	}
}
