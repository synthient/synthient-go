package synthient

import (
	"errors"
	"io"
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestAccount(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, `{
			"email": "dev@example.com",
			"first_name": "Ada",
			"last_name": "Lovelace",
			"organization": {"id": "0b9f6a52-3c1d-4e8a-9f27-6d4c1b8e2a70", "name": "Example", "relation": "OWNER"},
			"scopes": ["BASIC", "PROXY_FEEDS"],
			"lookup_quota": {"credits": 9000000000, "resets_in": 3600},
			"api_key": {"id": "6f1c2b1e-8f59-4f4b-9d5e-1f0a7d9c3b21", "name": "ci", "created_at": "2026-05-01T12:00:00Z"}
		}`)
	})
	c := newTestClient(t, srv)

	account, err := c.Account(t.Context())
	if err != nil {
		t.Fatalf("Account: %v", err)
	}
	want := Account{
		Email:        "dev@example.com",
		FirstName:    "Ada",
		LastName:     "Lovelace",
		Organization: Organization{ID: "0b9f6a52-3c1d-4e8a-9f27-6d4c1b8e2a70", Name: "Example", Relation: RelationOwner},
		Scopes:       []Scope{ScopeBasic, ScopeProxyFeeds},
		LookupQuota:  LookupQuota{Credits: 9000000000, ResetsIn: 3600},
		APIKey: AccountAPIKey{
			ID:        "6f1c2b1e-8f59-4f4b-9d5e-1f0a7d9c3b21",
			Name:      "ci",
			CreatedAt: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
		},
	}
	if !reflect.DeepEqual(*account, want) {
		t.Errorf("Account = %+v, want %+v", *account, want)
	}
}

func TestHealth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		status      int
		contentType string
		body        string
		wantStatus  int
	}{
		{
			name:        "healthy",
			status:      http.StatusOK,
			contentType: "application/json",
			body:        `{"status":"ok"}`,
		},
		{
			name:        "unhealthy",
			status:      http.StatusServiceUnavailable,
			contentType: "application/json",
			body:        `{"status":"unhealthy","problems":["clickhouse unreachable"]}`,
			wantStatus:  http.StatusServiceUnavailable,
		},
		{
			name:        "unavailable edge page",
			status:      http.StatusServiceUnavailable,
			contentType: "text/html",
			body:        "<html>unavailable</html>",
			wantStatus:  http.StatusServiceUnavailable,
		},
		{
			name:        "unauthorized",
			status:      http.StatusUnauthorized,
			contentType: "text/plain",
			body:        "401 invalid api key",
			wantStatus:  http.StatusUnauthorized,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var requests atomic.Int32
			srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			})
			c := newTestClient(t, srv)

			err := c.Health(t.Context())
			if tt.wantStatus == 0 && err != nil {
				t.Fatalf("Health: %v", err)
			}
			if tt.wantStatus != 0 {
				apiErr, ok := errors.AsType[*APIError](err)
				if !ok || apiErr.StatusCode != tt.wantStatus {
					t.Fatalf("Health error = %v, want an APIError with status %d", err, tt.wantStatus)
				}
			}
			if requests.Load() != 1 {
				t.Errorf("made %d requests, want 1", requests.Load())
			}
		})
	}
}
