package synthient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
)

// IPInfo is the intelligence Synthient holds for an IP address. Network,
// Location, and Intelligence are nil when the API has no data for them.
type IPInfo struct {
	// IP is the address that was looked up.
	IP netip.Addr `json:"ip"`
	// Network describes the network that announces the address.
	Network *Network `json:"network,omitempty"`
	// Location is the address's geolocation.
	Location *Location `json:"location,omitempty"`
	// Intelligence holds the risk score and the proxy, VPN, and device
	// observations for the address.
	Intelligence *Intelligence `json:"intelligence,omitempty"`
}

// Network describes the network that announces an address.
type Network struct {
	// ASN is the autonomous system number, or 0 if the address is not routed.
	ASN int64 `json:"asn"`
	// ISP is the name of the internet service provider.
	ISP string `json:"isp"`
	// Org is the organization the address is registered to, or empty if
	// unknown.
	Org string `json:"org"`
	// Domain is the network operator's domain, or empty if unknown.
	Domain string `json:"domain"`
	// Type is the network type.
	Type NetworkType `json:"type"`
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
	// City is the city.
	City string `json:"city"`
	// Timezone is the IANA time zone name.
	Timezone string `json:"timezone"`
	// Latitude is the latitude in degrees.
	Latitude float64 `json:"latitude"`
	// Longitude is the longitude in degrees.
	Longitude float64 `json:"longitude"`
	// GeoHash is the geohash of the coordinates.
	GeoHash string `json:"geo_hash"`
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
	// Devices are the device types seen behind the address.
	Devices []Device `json:"devices"`
}

// Provider is a proxy, VPN, or anonymizer service seen using an address.
type Provider struct {
	// Name is the service tag, such as "BRIGHTDATA".
	Name string `json:"provider"`
	// Type is the service category.
	Type string `json:"type"`
	// LastSeen is when the service was last seen using the address.
	LastSeen Timestamp `json:"last_seen"`
}

// Device is a device type seen behind an address.
type Device struct {
	// OS is the operating system.
	OS OS `json:"os"`
	// Version is the operating system version.
	Version string `json:"version"`
	// LastSeen is when the device was last seen.
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

// DomainInfo is the honeypot traffic Synthient has observed for a domain.
type DomainInfo struct {
	// Domain is the domain that was looked up.
	Domain string `json:"domain"`
	// Status is whether the domain had events in the last 24 hours.
	Status DomainStatus `json:"status"`
	// Stats holds aggregate event counts.
	Stats DomainStats `json:"stats"`
	// UniqueIPs counts the distinct source addresses.
	UniqueIPs UniqueIPs `json:"unique_ips"`
	// TimeSeries holds daily event counts.
	TimeSeries []TimeSeriesPoint `json:"time_series"`
	// TopASN is the network that sent the most events. It is zero when
	// there are no events.
	TopASN ASNCount `json:"top_asn"`
	// TopSubdomains are the most targeted subdomains.
	TopSubdomains []SubdomainCount `json:"top_subdomains"`
	// TopPorts are the most targeted ports.
	TopPorts []PortCount `json:"top_ports"`
	// GeoDistribution counts events by source country.
	GeoDistribution []CountryCount `json:"geo_distribution"`
	// HourDOWHeatmap counts the last 30 days of events by UTC day of week and
	// hour, indexed as HourDOWHeatmap[day][hour]. Day 0 is Monday, unlike
	// time.Weekday, where 0 is Sunday.
	HourDOWHeatmap [][]int64 `json:"hour_dow_heatmap"`
	// ActivityStats describes the hourly activity pattern.
	ActivityStats ActivityStats `json:"activity_stats"`
	// RecentEvents are the most recent events, capped by
	// LookupDomainParams.Limit.
	RecentEvents []DomainEvent `json:"recent_events"`
}

// DomainStatus is whether a domain has recent activity.
type DomainStatus string

// Domain statuses.
const (
	// DomainStatusActive means the domain had events in the last 24 hours.
	DomainStatusActive DomainStatus = "active"
	// DomainStatusInactive means the domain had no events in the last 24
	// hours, or has never been observed.
	DomainStatusInactive DomainStatus = "inactive"
)

// DomainStats holds aggregate event counts for a domain.
type DomainStats struct {
	// Events24h is the number of events in the last 24 hours.
	Events24h int64 `json:"events_24h"`
	// TotalEvents30d is the number of events in the last 30 days.
	TotalEvents30d int64 `json:"total_events_30d"`
}

// UniqueIPs counts the distinct source addresses seen for a domain.
type UniqueIPs struct {
	// Value24h is the count for the last 24 hours.
	Value24h int64 `json:"value_24h"`
	// Value30d is the count for the last 30 days.
	Value30d int64 `json:"value_30d"`
	// Sparkline24h holds hourly counts for the last 24 hours.
	Sparkline24h []int64 `json:"sparkline_24h"`
}

// TimeSeriesPoint holds the event counts for one day.
type TimeSeriesPoint struct {
	// Date is the start of the day.
	Date Timestamp `json:"date"`
	// Events is the number of events that day.
	Events int64 `json:"events"`
	// UniqueIPs is the number of distinct source addresses that day.
	UniqueIPs int64 `json:"unique_ips"`
}

// ASNCount is an autonomous system and its event count.
type ASNCount struct {
	// ASN is the autonomous system number.
	ASN int64 `json:"asn"`
	// Events is the number of events from the network.
	Events int64 `json:"events"`
}

// SubdomainCount is a subdomain and its event count.
type SubdomainCount struct {
	// Subdomain is the targeted subdomain.
	Subdomain string `json:"subdomain"`
	// Count is the number of events that targeted it.
	Count int64 `json:"count"`
}

// PortCount is a port and its event count.
type PortCount struct {
	// Port is the targeted port.
	Port int `json:"port"`
	// Count is the number of events that targeted it.
	Count int64 `json:"count"`
}

// CountryCount is a source country and its counts.
type CountryCount struct {
	// CountryCode is the ISO 3166-1 alpha-2 country code.
	CountryCode string `json:"country_code"`
	// Events is the number of events from the country.
	Events int64 `json:"events"`
	// UniqueIPs is the number of distinct addresses from the country.
	UniqueIPs int64 `json:"unique_ips"`
}

// ActivityStats describes the hourly activity pattern of a domain.
type ActivityStats struct {
	// Cadence describes how events are spread over the last 30 days.
	Cadence Cadence `json:"cadence"`
	// MedianPerHour is the median number of events per hour.
	MedianPerHour int64 `json:"median_per_hour"`
	// P95PerHour is the 95th percentile of events per hour.
	P95PerHour int64 `json:"p95_per_hour"`
	// PeakHour is the busiest UTC hour of the day.
	PeakHour int `json:"peak_hour"`
	// QuietHour is the quietest UTC hour of the day.
	QuietHour int `json:"quiet_hour"`
}

// Cadence is a sentence describing a domain's activity pattern.
type Cadence string

// Activity cadences.
const (
	CadenceNoActivity        Cadence = "No activity in the last 30 days."
	CadenceEvenlyDistributed Cadence = "Activity is evenly distributed across the day."
	CadenceBusinessHours     Cadence = "Activity follows a weekday business-hours pattern."
	CadenceWeekend           Cadence = "Activity follows a weekend pattern."
	CadenceBurstDriven       Cadence = "Activity is burst-driven with significant peaks."
)

// DomainEvent is a single honeypot event for a domain.
type DomainEvent struct {
	// Timestamp is when the event happened.
	Timestamp Timestamp `json:"timestamp"`
	// SourceIP is the full proxy exit address that carried the event. The API
	// names the field source_ip_masked, but the address is not masked.
	SourceIP string `json:"source_ip_masked"`
	// TargetSubdomain is the subdomain that was targeted.
	TargetSubdomain string `json:"target_subdomain"`
	// CountryCode is the source country.
	CountryCode string `json:"country_code"`
	// Port is the targeted port.
	Port int `json:"port"`
}

// LookupDomainParams holds optional parameters for LookupDomain.
type LookupDomainParams struct {
	// Limit caps the number of recent events returned, from 1 to 1000.
	// Zero uses the API default of 50.
	Limit int
}

const maxBatchSize = 1000

type batchRequest struct {
	IPs []string `json:"ips"`
}

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

// LookupIPs looks up as many as 1000 addresses in one request. Private and
// reserved addresses are skipped and duplicates collapse into one result, so
// there can be fewer results than addresses. Match results by IP, which the
// API canonicalizes, rather than by index. A batch with no address the API
// can look up returns a 422 *APIError. The API bills ceil(n*0.9) credits for
// the n distinct addresses it looks up, and bills nothing if the account
// cannot cover the whole batch.
func (c *Client) LookupIPs(ctx context.Context, ips []netip.Addr) ([]IPInfo, error) {
	if len(ips) == 0 {
		return nil, nil
	}
	if len(ips) > maxBatchSize {
		return nil, fmt.Errorf("synthient: %d addresses exceed the batch limit of %d", len(ips), maxBatchSize)
	}

	addrs := make([]string, len(ips))
	for i, ip := range ips {
		if !ip.IsValid() {
			return nil, fmt.Errorf("synthient: invalid IP address at index %d", i)
		}
		addrs[i] = ip.String()
	}
	body, err := json.Marshal(batchRequest{IPs: addrs})
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

// LookupDomain returns the honeypot traffic observed for a domain.
// Internationalized names must be sent in Punycode. params may be nil. A
// domain that has never been observed returns DomainStatusInactive and zero
// counts rather than an error. Each lookup costs one credit.
func (c *Client) LookupDomain(ctx context.Context, domain string, params *LookupDomainParams) (*DomainInfo, error) {
	if domain == "" || domain == "." || domain == ".." {
		return nil, fmt.Errorf("synthient: invalid domain %q", domain)
	}

	query := url.Values{}
	if params != nil && params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}

	return doJSON[DomainInfo](ctx, c, request{
		method: http.MethodGet,
		path:   "/lookup/domain/" + url.PathEscape(domain),
		query:  query,
		policy: retryRateLimit,
	})
}
