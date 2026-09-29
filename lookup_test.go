package synthient

import (
	"encoding/json"
	"net/http"
	"net/netip"
	"reflect"
	"sync/atomic"
	"testing"
)

const ipInfoJSON = `{
	"ip": "1.1.1.1",
	"network": {
		"asn": 13335,
		"isp": "Cloudflare",
		"org": null,
		"domain": "cloudflare.com",
		"type": "DATACENTER",
		"abuse_email": null,
		"abuse_phone": null
	},
	"location": {
		"city": "Sydney",
		"country": "AU",
		"state": "New South Wales",
		"timezone": "Australia/Sydney",
		"latitude": -33.86,
		"longitude": 151.2,
		"geo_hash": "r3gx2f"
	},
	"intelligence": {
		"risk_score": 12,
		"categories": ["DATACENTER"],
		"behavior": [],
		"devices": [{"os": "LINUX", "version": "6.1", "last_seen": 1714780800}],
		"providers": [{"provider": "CLOUDFLARE_WARP", "type": "VPN", "last_seen": 1714780800}]
	}
}`

func wantIPInfo() IPInfo {
	return IPInfo{
		IP: netip.MustParseAddr("1.1.1.1"),
		Network: &Network{
			ASN:    13335,
			ISP:    "Cloudflare",
			Domain: "cloudflare.com",
			Type:   "DATACENTER",
		},
		Location: &Location{
			City:      "Sydney",
			Country:   "AU",
			State:     "New South Wales",
			Timezone:  "Australia/Sydney",
			Latitude:  -33.86,
			Longitude: 151.2,
			GeoHash:   "r3gx2f",
		},
		Intelligence: &Intelligence{
			RiskScore:  12,
			Categories: []string{"DATACENTER"},
			Behavior:   []string{},
			Devices:    []Device{{OS: "LINUX", Version: "6.1", LastSeen: 1714780800}},
			Providers:  []Provider{{Name: "CLOUDFLARE_WARP", Type: "VPN", LastSeen: 1714780800}},
		},
	}
}

func TestLookupIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ip      string
		wantURI string
	}{
		{ip: "1.1.1.1", wantURI: "/api/v4/lookup/ip/1.1.1.1"},
		{ip: "2001:db8::1", wantURI: "/api/v4/lookup/ip/2001:db8::1"},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			t.Parallel()
			srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.RequestURI != tt.wantURI {
					t.Errorf("request = %s %s, want GET %s", r.Method, r.RequestURI, tt.wantURI)
				}
				writeJSON(w, http.StatusOK, ipInfoJSON)
			})
			c := newTestClient(t, srv)

			info, err := c.LookupIP(t.Context(), netip.MustParseAddr(tt.ip))
			if err != nil {
				t.Fatalf("LookupIP: %v", err)
			}
			want := wantIPInfo()
			if !reflect.DeepEqual(*info, want) {
				t.Errorf("LookupIP = %+v, want %+v", *info, want)
			}
		})
	}
}

func TestLookupIPMissingSections(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, `{"ip":"10.0.0.1","network":null,"location":null,"intelligence":null}`)
	})
	c := newTestClient(t, srv)

	info, err := c.LookupIP(t.Context(), netip.MustParseAddr("10.0.0.1"))
	if err != nil {
		t.Fatalf("LookupIP: %v", err)
	}
	want := IPInfo{IP: netip.MustParseAddr("10.0.0.1")}
	if !reflect.DeepEqual(*info, want) {
		t.Errorf("LookupIP = %+v, want %+v", *info, want)
	}
}

func TestIPInfoSectionPresence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		body         string
		network      bool
		location     bool
		intelligence bool
	}{
		{body: `{}`},
		{body: `{"network":null,"location":null,"intelligence":null}`},
		{body: `{"network":{},"location":{},"intelligence":{"risk_score":0}}`, network: true, location: true, intelligence: true},
		{body: `{"intelligence":{"risk_score":0}}`, intelligence: true},
	}
	for _, tt := range tests {
		var info IPInfo
		err := json.Unmarshal([]byte(tt.body), &info)
		if err != nil {
			t.Fatal(err)
		}
		if (info.Network != nil) != tt.network || (info.Location != nil) != tt.location || (info.Intelligence != nil) != tt.intelligence {
			t.Errorf("section presence for %s: %+v", tt.body, info)
		}
	}
}

func TestLookupIPRejectsInvalidAddress(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	srv := newTestServer(t, func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	})
	c := newTestClient(t, srv)

	_, err := c.LookupIP(t.Context(), netip.Addr{})
	if err == nil {
		t.Fatal("LookupIP succeeded with an invalid address")
	}
	if requests.Load() != 0 {
		t.Errorf("made %d requests, want 0", requests.Load())
	}
}

func TestLookupIPs(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v4/lookup/ips" {
			t.Errorf("request = %s %s, want POST /api/v4/lookup/ips", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		var body struct {
			IPs []string `json:"ips"`
		}
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			t.Errorf("decode request body: %v", err)
		}
		want := []string{"1.1.1.1", "2001:db8::1"}
		if !reflect.DeepEqual(body.IPs, want) {
			t.Errorf("ips = %v, want %v", body.IPs, want)
		}
		writeJSON(w, http.StatusOK, `{"results":[`+ipInfoJSON+`,{"ip":"2001:db8::1"}]}`)
	})
	c := newTestClient(t, srv)

	results, err := c.LookupIPs(t.Context(), []netip.Addr{
		netip.MustParseAddr("1.1.1.1"),
		netip.MustParseAddr("2001:db8::1"),
	})
	if err != nil {
		t.Fatalf("LookupIPs: %v", err)
	}
	want := []IPInfo{wantIPInfo(), {IP: netip.MustParseAddr("2001:db8::1")}}
	if !reflect.DeepEqual(results, want) {
		t.Errorf("LookupIPs = %+v, want %+v", results, want)
	}
}

func TestLookupIPsEdgeCases(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	srv := newTestServer(t, func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	})
	c := newTestClient(t, srv)

	results, err := c.LookupIPs(t.Context(), nil)
	if err != nil || results != nil {
		t.Errorf("LookupIPs(nil) = %v, %v, want nil, nil", results, err)
	}
	_, err = c.LookupIPs(t.Context(), []netip.Addr{netip.MustParseAddr("1.1.1.1"), {}})
	if err == nil {
		t.Error("LookupIPs succeeded with an invalid address")
	}
	if requests.Load() != 0 {
		t.Errorf("made %d requests, want 0", requests.Load())
	}
}

func TestLookupDomain(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		domain  string
		params  *LookupDomainParams
		wantURI string
	}{
		{name: "plain", domain: "example.com", wantURI: "/api/v4/lookup/domain/example.com"},
		{name: "limit", domain: "example.com", params: &LookupDomainParams{Limit: 10}, wantURI: "/api/v4/lookup/domain/example.com?limit=10"},
		{name: "zero limit", domain: "example.com", params: &LookupDomainParams{}, wantURI: "/api/v4/lookup/domain/example.com"},
		{name: "reserved characters", domain: "a/b?c", wantURI: "/api/v4/lookup/domain/a%2Fb%3Fc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.RequestURI != tt.wantURI {
					t.Errorf("RequestURI = %q, want %q", r.RequestURI, tt.wantURI)
				}
				writeJSON(w, http.StatusOK, domainInfoJSON)
			})
			c := newTestClient(t, srv)

			info, err := c.LookupDomain(t.Context(), tt.domain, tt.params)
			if err != nil {
				t.Fatalf("LookupDomain: %v", err)
			}
			if info.Domain != "example.com" || info.TopASN.ASN != 15169 || len(info.RecentEvents) != 1 {
				t.Errorf("LookupDomain = %+v", info)
			}
			if info.TimeSeries[0].Date.Time().Unix() != 1777680000 {
				t.Errorf("time series date = %d, want 1777680000", info.TimeSeries[0].Date)
			}
		})
	}
}

func TestLookupDomainRejectsInvalidDomain(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	srv := newTestServer(t, func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	})
	c := newTestClient(t, srv)

	for _, domain := range []string{"", ".", ".."} {
		_, err := c.LookupDomain(t.Context(), domain, nil)
		if err == nil {
			t.Errorf("LookupDomain(%q) succeeded, want an error", domain)
		}
	}
	if requests.Load() != 0 {
		t.Errorf("made %d requests, want 0", requests.Load())
	}
}

const domainInfoJSON = `{
	"domain": "example.com",
	"status": "ok",
	"stats": {"events_24h": 12, "total_events_30d": 340},
	"unique_ips": {"value_24h": 5, "value_30d": 90, "sparkline_24h": [0, 1, 4]},
	"time_series": [{"date": 1777680000, "events": 12, "unique_ips": 5}],
	"top_asn": {"asn": 15169, "events": 7},
	"top_subdomains": [{"subdomain": "www", "count": 9}],
	"top_ports": [{"port": 443, "count": 11}],
	"geo_distribution": [{"country_code": "US", "events": 10, "unique_ips": 4}],
	"hour_dow_heatmap": [[0, 1], [2, 3]],
	"activity_stats": {"cadence": "bursty", "median_per_hour": 1, "p95_per_hour": 4, "peak_hour": 14, "quiet_hour": 3},
	"recent_events": [{"timestamp": 1777680300, "source_ip_masked": "203.0.113.x", "target_subdomain": "www", "country_code": "US", "port": 443}]
}`
