package synthient

import (
	"context"
	"net/http"
	"time"
)

// Account describes the account that owns the API key.
type Account struct {
	// Email is the account holder's email address.
	Email string `json:"email"`
	// FirstName is the account holder's first name.
	FirstName string `json:"first_name"`
	// LastName is the account holder's last name.
	LastName string `json:"last_name"`
	// Organization is the organization the account belongs to.
	Organization Organization `json:"organization"`
	// Scopes are the permissions granted to the API key, such as "BASIC" or
	// "PROXY_FEEDS". New values may be added.
	Scopes []string `json:"scopes"`
	// LookupQuota is the remaining lookup credit balance.
	LookupQuota LookupQuota `json:"lookup_quota"`
	// APIKey describes the API key used for the request.
	APIKey AccountAPIKey `json:"api_key"`
}

// Organization is the organization an account belongs to.
type Organization struct {
	// ID identifies the organization.
	ID string `json:"id"`
	// Name is the organization's name.
	Name string `json:"name"`
	// Relation is the account's role, such as "OWNER" or "MEMBER".
	Relation string `json:"relation"`
}

// LookupQuota is an account's lookup credit balance.
type LookupQuota struct {
	// Credits is the number of lookup credits remaining.
	Credits int64 `json:"credits"`
	// ResetsIn is the number of seconds until the credits reset.
	ResetsIn int64 `json:"resets_in"`
}

// AccountAPIKey describes an API key.
type AccountAPIKey struct {
	// ID identifies the key. It is not the secret key itself.
	ID string `json:"id"`
	// Name is the key's display name.
	Name string `json:"name"`
	// CreatedAt is when the key was created.
	CreatedAt time.Time `json:"created_at"`
}

// Account returns the account that owns the API key, including its scopes and
// remaining lookup credits. It costs no credits.
func (c *Client) Account(ctx context.Context) (*Account, error) {
	var account Account
	err := c.doJSON(ctx, request{method: http.MethodGet, path: "/account/me"}, &account)
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// Health returns nil if the API and its dependencies are healthy. An
// unhealthy API returns an *APIError with status 503.
func (c *Client) Health(ctx context.Context) error {
	return c.doJSON(ctx, request{method: http.MethodGet, path: "/health", policy: retryRateLimit}, &struct{}{})
}
