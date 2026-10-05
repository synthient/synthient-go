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
	// Scopes are the permissions granted to the API key.
	Scopes []Scope `json:"scopes"`
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
	// Relation is the account's role in the organization.
	Relation Relation `json:"relation"`
}

// Relation is an account's role in its organization.
type Relation string

// Organization roles.
const (
	RelationOwner  Relation = "OWNER"
	RelationAdmin  Relation = "ADMIN"
	RelationMember Relation = "MEMBER"
)

// Scope is a permission granted to an API key. New values may be added.
type Scope string

// API key scopes. ScopeBasic authorizes lookups and Account. Each feed has one
// scope for its exports and one for its stream, such as ScopeTorrentsFeed and
// ScopeTorrentsStream. The proxy feed's are ScopeProxyFeeds and
// ScopeProxyFirehose.
const (
	ScopeBasic               Scope = "BASIC"
	ScopeProxyFeeds          Scope = "PROXY_FEEDS"
	ScopeProxyFirehose       Scope = "PROXY_FIREHOSE"
	ScopeAnonymizersFeed     Scope = "ANONYMIZERS_FEED"
	ScopeAnonymizersStream   Scope = "ANONYMIZERS_STREAM"
	ScopeJA3Feed             Scope = "JA3_FEED"
	ScopeJA3Stream           Scope = "JA3_STREAM"
	ScopeTorrentsFeed        Scope = "TORRENTS_FEED"
	ScopeTorrentsStream      Scope = "TORRENTS_STREAM"
	ScopeHoneypotHTTPFeed    Scope = "HONEYPOT_HTTP_FEED"
	ScopeHoneypotHTTPStream  Scope = "HONEYPOT_HTTP_STREAM"
	ScopeHoneypotHTTPSFeed   Scope = "HONEYPOT_HTTPS_FEED"
	ScopeHoneypotHTTPSStream Scope = "HONEYPOT_HTTPS_STREAM"
	ScopeHoneypotADBFeed     Scope = "HONEYPOT_ADB_FEED"
	ScopeHoneypotADBStream   Scope = "HONEYPOT_ADB_STREAM"
	ScopeHoneypotDNSFeed     Scope = "HONEYPOT_DNS_FEED"
	ScopeHoneypotDNSStream   Scope = "HONEYPOT_DNS_STREAM"
	ScopeJA4Feed             Scope = "JA4_FEED"
	ScopeJA4Stream           Scope = "JA4_STREAM"
	ScopeJA4TFeed            Scope = "JA4T_FEED"
	ScopeJA4TStream          Scope = "JA4T_STREAM"
	ScopeAdminProxies        Scope = "ADMIN_PROXIES"
)

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
// unhealthy API returns an *APIError with status 503. It costs no credits.
// The API does not check the key, but a malformed key returns a 401
// *APIError.
func (c *Client) Health(ctx context.Context) error {
	return c.doJSON(ctx, request{method: http.MethodGet, path: "/health", policy: retryRateLimit}, &struct{}{})
}
