package synthient

import (
	"context"
	"net/http"
	"time"
)

// Account describes the account that owns the API key.
type Account struct {
	Email        string       `json:"email"`
	FirstName    string       `json:"first_name"`
	LastName     string       `json:"last_name"`
	Organization Organization `json:"organization"`
	Scopes       []Scope      `json:"scopes"`
	LookupQuota  LookupQuota  `json:"lookup_quota"`
	// APIKey describes the API key used for the request. It is nil if the key
	// was deleted while the request was in flight.
	APIKey *AccountAPIKey `json:"api_key"`
}

// Organization is the organization an account belongs to.
type Organization struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
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
// ScopeProxyFirehose. The datasets need ScopeGeolocationFeed and
// ScopeWHOISFeed.
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
	ScopeGeolocationFeed     Scope = "GEOLOCATION_FEED"
	ScopeWHOISFeed           Scope = "WHOIS_FEED"
	// ScopeHelios is a dashboard permission. It authorizes no API operation.
	ScopeHelios Scope = "HELIOS"
	// These scopes are reserved for upcoming feeds and authorize no operation
	// yet.
	ScopeWHOISStream       Scope = "WHOIS_STREAM"
	ScopeNetworkIOCsFeed   Scope = "NETWORK_IOCS_FEED"
	ScopeNetworkIOCsStream Scope = "NETWORK_IOCS_STREAM"
	ScopeFileIOCsFeed      Scope = "FILE_IOCS_FEED"
	ScopeFileIOCsStream    Scope = "FILE_IOCS_STREAM"
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
	// ID is not the secret key itself.
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Account returns the account that owns the API key, including its scopes and
// remaining lookup credits. It costs no credits.
func (c *Client) Account(ctx context.Context) (*Account, error) {
	return doJSON[Account](ctx, c, request{method: http.MethodGet, path: "/account/me"})
}

// Health returns nil if the API and its dependencies are healthy, or a 503
// *APIError if not. It costs no credits. It needs a valid key with any scope,
// and returns a 401 *APIError without one.
func (c *Client) Health(ctx context.Context) error {
	_, err := doJSON[struct{}](ctx, c, request{method: http.MethodGet, path: "/health", policy: retryRateLimit})
	return err
}
