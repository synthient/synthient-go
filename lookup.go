package synthient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
)

// IPInfo is the intelligence Synthient holds for an IP address. Network is nil
// when the address lies in no known network range or the enrichment lookup
// failed, and Location is nil whenever Network is. Intelligence is nil when
// the API has no data for it.
type IPInfo struct {
	IP           netip.Addr    `json:"ip"`
	Network      *Network      `json:"network,omitempty"`
	Location     *Location     `json:"location,omitempty"`
	Intelligence *Intelligence `json:"intelligence,omitempty"`
}

// Network describes the network that announces an address.
type Network struct {
	// ASN is the autonomous system number, or 0 if the address is not routed.
	ASN int64  `json:"asn"`
	ISP string `json:"isp"`
	// Org is the organization the address is registered to, or empty if
	// unknown.
	Org string `json:"org"`
	// Domain is the network operator's domain, or empty if unknown.
	Domain string      `json:"domain"`
	Type   NetworkType `json:"type"`
	// AbuseEmail is the operator's abuse contact email, or empty if unknown.
	AbuseEmail string `json:"abuse_email"`
	// AbusePhone is the operator's abuse contact phone number, or empty if
	// unknown.
	AbusePhone string `json:"abuse_phone"`
}

// NetworkType is the kind of network that announces an address. New values
// may be added.
type NetworkType string

// Network types.
const (
	NetworkTypeMobile       NetworkType = "MOBILE"
	NetworkTypeSatellite    NetworkType = "SATELLITE"
	NetworkTypeInFlightWiFi NetworkType = "IN_FLIGHT_WIFI"
	NetworkTypeResidential  NetworkType = "RESIDENTIAL"
	NetworkTypeCorporate    NetworkType = "CORPORATE"
	NetworkTypeAcademic     NetworkType = "ACADEMIC"
	NetworkTypeDatacenter   NetworkType = "DATACENTER"
	NetworkTypeGovernment   NetworkType = "GOVERNMENT"
)

// Location is the geolocation of an address.
type Location struct {
	// Country is the ISO 3166-1 alpha-2 country code.
	Country string `json:"country"`
	// State is the state or region.
	State string `json:"state"`
	City  string `json:"city"`
	// Timezone is the IANA time zone name.
	Timezone string `json:"timezone"`
	// Latitude is the latitude in degrees.
	Latitude float64 `json:"latitude"`
	// Longitude is the longitude in degrees.
	Longitude float64 `json:"longitude"`
	GeoHash   string  `json:"geo_hash"`
}

// Intelligence holds Synthient's assessment of an address.
type Intelligence struct {
	// RiskScore is the risk of the address, from 0 to 100.
	RiskScore int `json:"risk_score"`
	// Categories are the kinds of services seen on the address, such as
	// "RESIDENTIAL_PROXY". New values may be added.
	Categories []string `json:"categories"`
	// Behavior lists behaviors observed from the address. New values may be
	// added.
	Behavior []string `json:"behavior"`
	// Providers are the proxy, VPN, and anonymizer services seen using the
	// address. An empty list does not mean the address is clean.
	Providers []Provider `json:"providers"`
	Devices   []Device   `json:"devices"`
}

// Provider is a proxy, VPN, or anonymizer service seen using an address.
type Provider struct {
	// Name is the service tag, such as "BRIGHTDATA".
	Name string `json:"provider"`
	// Type is the service category.
	Type     string    `json:"type"`
	LastSeen Timestamp `json:"last_seen"`
}

// Device is a device type seen behind an address.
type Device struct {
	OS OS `json:"os"`
	// Version is the operating system version.
	Version  string    `json:"version"`
	LastSeen Timestamp `json:"last_seen"`
}

// OS is a device operating system. New values may be added.
type OS string

// Operating systems.
const (
	OSAndroid  OS = "Android"
	OSChromeOS OS = "Chrome OS"
	OSFreeBSD  OS = "FreeBSD"
	OSiOS      OS = "iOS"
	OSLinux    OS = "Linux"
	OSMacOSX   OS = "Mac OS X"
	OSWindows  OS = "Windows"
)

const maxBatchSize = 1000

type batchResponse struct {
	Results []IPInfo `json:"results"`
}

// LookupIP returns intelligence for an IPv4 or IPv6 address. Private and
// reserved addresses return a 422 *APIError. Each lookup costs one credit,
// and failed lookups are not billed.
func (c *Client) LookupIP(ctx context.Context, ip netip.Addr) (*IPInfo, error) {
	if !ip.IsValid() {
		return nil, errors.New("synthient: invalid IP address")
	}

	return doJSON[IPInfo](ctx, c, request{
		method: http.MethodGet,
		path:   "/lookup/ip/" + url.PathEscape(ip.String()),
		policy: retryRateLimit,
	})
}

// LookupIPs looks up a batch of addresses in one request. The batch cap
// depends on the plan: 100 on Free, 500 on Starter, and 1000 on Premium and
// Enterprise. A larger batch returns a 422 *APIError, and more than 1000
// addresses fail before any request. Private and reserved addresses are
// skipped and duplicates collapse, so there can be fewer results than
// addresses. Match results by IP, which the API canonicalizes, not by index.
// A batch with no address the API can look up returns a 422 *APIError. The
// API bills ceil(n*0.9) credits for the n distinct addresses it looks up, and
// bills nothing if the account cannot cover the whole batch.
func (c *Client) LookupIPs(ctx context.Context, ips []netip.Addr) ([]IPInfo, error) {
	if len(ips) == 0 {
		return nil, nil
	}
	if len(ips) > maxBatchSize {
		return nil, fmt.Errorf("synthient: %d addresses exceed the batch limit of %d", len(ips), maxBatchSize)
	}

	for i, ip := range ips {
		if !ip.IsValid() {
			return nil, fmt.Errorf("synthient: invalid IP address at index %d", i)
		}
	}
	body, err := json.Marshal(map[string][]netip.Addr{"ips": ips})
	if err != nil {
		return nil, fmt.Errorf("synthient: encode request: %w", err)
	}

	out, err := doJSON[batchResponse](ctx, c, request{
		method: http.MethodPost,
		path:   "/lookup/ips",
		body:   body,
		policy: retryRateLimit,
	})
	if err != nil {
		return nil, err
	}
	return out.Results, nil
}
