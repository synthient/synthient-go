package synthient

import "encoding/json"

// ProxyEvent is a proxy sighting from the Proxies feed.
type ProxyEvent struct {
	// IP is the proxy exit address.
	IP string `json:"ip"`
	// Provider is the proxy service tag.
	Provider string `json:"provider"`
	// Type is the proxy category, such as "RESIDENTIAL_PROXY".
	Type string `json:"type"`
	// CountryCode is the ISO 3166-1 alpha-2 country of the address.
	CountryCode string `json:"country_code"`
	// ASN is the autonomous system number of the address.
	ASN int64 `json:"asn"`
	// Timestamp is when the proxy was seen.
	Timestamp Timestamp `json:"timestamp"`
}

// AnonymizerEvent is a VPN or anonymizer IP range from the Anonymizers feed.
type AnonymizerEvent struct {
	// RangeStart is the first address of the range.
	RangeStart string `json:"range_start"`
	// RangeEnd is the last address of the range.
	RangeEnd string `json:"range_end"`
	// Provider is the service tag.
	Provider string `json:"provider"`
	// Type is the service category, such as "COMMERCIAL_VPN".
	Type string `json:"type"`
	// CountryCode is the ISO 3166-1 alpha-2 country of RangeStart.
	CountryCode string `json:"country_code"`
	// ASN is the autonomous system number of RangeStart.
	ASN int64 `json:"asn"`
	// Timestamp is when the range was seen.
	Timestamp Timestamp `json:"timestamp"`
}

// TorrentEvent is a torrent observed on the BitTorrent DHT or a tracker.
type TorrentEvent struct {
	// InfoHash is the hex-encoded SHA-1 info hash.
	InfoHash string `json:"info_hash"`
	// Name is the torrent name.
	Name string `json:"name"`
	// MagnetURI is the torrent's magnet link.
	MagnetURI string `json:"magnet_uri"`
	// TotalSize is the total size of the torrent's files in bytes.
	TotalSize int64 `json:"total_size"`
	// PieceLength is the piece size in bytes.
	PieceLength int64 `json:"piece_length"`
	// FileCount is the number of files in the torrent.
	FileCount int `json:"file_count"`
	// Files are the torrent's files.
	Files []TorrentFile `json:"files"`
	// Peers are the peers seen sharing the torrent.
	Peers []TorrentPeer `json:"peers"`
	// Timestamp is when the torrent was seen.
	Timestamp Timestamp `json:"timestamp"`
}

// TorrentFile is a file in a torrent.
type TorrentFile struct {
	// Path is the file path within the torrent.
	Path string `json:"path"`
	// Length is the file size in bytes.
	Length int64 `json:"length"`
}

// TorrentPeer is a peer seen sharing a torrent.
type TorrentPeer struct {
	// IP is the peer's address.
	IP string `json:"ip"`
	// Port is the peer's port.
	Port int `json:"port"`
	// Source is where the peer was found, such as "DHT", "PEX", or "tracker".
	Source string `json:"source"`
	// Encrypted reports whether the peer supports encrypted connections.
	Encrypted bool `json:"encrypted"`
}

// JA4TEvent is a JA4T TCP fingerprint sighting.
type JA4TEvent struct {
	// IP is the address the fingerprint was seen from.
	IP string `json:"ip"`
	// JA4T is the JA4T fingerprint.
	JA4T string `json:"ja4t"`
	// Provider is the service tag of the address.
	Provider string `json:"provider"`
	// Type is the service category, such as "DATACENTER_PROXY".
	Type string `json:"type"`
	// CountryCode is the ISO 3166-1 alpha-2 country of the address.
	CountryCode string `json:"country_code"`
	// ASN is the autonomous system number of the address.
	ASN int64 `json:"asn"`
	// Timestamp is when the fingerprint was seen.
	Timestamp Timestamp `json:"timestamp"`
}

// HeliosMeta describes the Helios honeypot sensor that captured an event.
type HeliosMeta struct {
	// ProxyIP is the proxy exit address the traffic arrived through.
	ProxyIP string `json:"proxy_ip"`
	// Server is the sensor that captured the traffic.
	Server string `json:"server"`
	// PoolID is the sensor pool.
	PoolID string `json:"pool_id"`
	// Provider is the proxy service the traffic arrived through.
	Provider string `json:"provider"`
}

// HeliosHTTPEvent is an HTTP request captured by a Helios honeypot.
type HeliosHTTPEvent struct {
	// Timestamp is when the request was captured.
	Timestamp Timestamp `json:"timestamp"`
	// TunnelID identifies the proxy tunnel the request arrived through.
	TunnelID int64 `json:"tunnel_id"`
	// Domain is the requested domain.
	Domain string `json:"domain"`
	// Port is the destination port.
	Port int `json:"port"`
	// Protocol is "http".
	Protocol string `json:"protocol"`
	// Meta describes the capturing sensor.
	Meta HeliosMeta `json:"meta"`
	// Details is the parsed request.
	Details HTTPRequestDetails `json:"details"`
	// Raw is the raw request.
	Raw string `json:"raw"`
}

// HTTPRequestDetails is a parsed HTTP request.
type HTTPRequestDetails struct {
	// Method is the request method.
	Method string `json:"method"`
	// URI is the request target.
	URI string `json:"uri"`
	// Version is the HTTP version, such as "HTTP/1.1".
	Version string `json:"version"`
	// Headers are the request headers.
	Headers map[string]string `json:"headers"`
}

// HeliosHTTPSEvent is a TLS ClientHello captured by a Helios honeypot.
type HeliosHTTPSEvent struct {
	// Timestamp is when the handshake was captured.
	Timestamp Timestamp `json:"timestamp"`
	// TunnelID identifies the proxy tunnel the handshake arrived through.
	TunnelID int64 `json:"tunnel_id"`
	// Domain is the requested domain.
	Domain string `json:"domain"`
	// Port is the destination port.
	Port int `json:"port"`
	// Protocol is "https".
	Protocol string `json:"protocol"`
	// Meta describes the capturing sensor.
	Meta HeliosMeta `json:"meta"`
	// Details is the parsed ClientHello as raw JSON. The API has described it
	// both as an object and as a JSON-encoded string, so it is left raw.
	Details json.RawMessage `json:"details"`
	// Raw is the base64-encoded ClientHello record, when captured.
	Raw string `json:"raw"`
}

// HeliosDNSEvent is a DNS query captured by a Helios honeypot.
type HeliosDNSEvent struct {
	// Timestamp is when the query was captured.
	Timestamp Timestamp `json:"timestamp"`
	// TunnelID identifies the proxy tunnel the query arrived through.
	TunnelID int64 `json:"tunnel_id"`
	// Domain is the queried domain.
	Domain string `json:"domain"`
	// Port is the destination port: 53 for DNS, 853 for DoT, or 5353 for mDNS.
	Port int `json:"port"`
	// Meta describes the capturing sensor.
	Meta HeliosMeta `json:"meta"`
}

// HeliosADBEvent is an Android Debug Bridge shell command captured by a
// Helios honeypot.
type HeliosADBEvent struct {
	// Session identifies the ADB session.
	Session string `json:"session"`
	// SequentialID orders the commands within a session.
	SequentialID int64 `json:"sequential_id"`
	// Command is the shell command.
	Command string `json:"command"`
	// Hash is the hash of the command.
	Hash string `json:"hash"`
}
