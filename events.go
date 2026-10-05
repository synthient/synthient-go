package synthient

// ProxyType is the kind of service seen on an address, such as
// ProxyTypeResidentialProxy. New values may be added.
type ProxyType string

// Proxy types.
const (
	ProxyTypeFreeVPN          ProxyType = "FREE_VPN"
	ProxyTypeCommercialVPN    ProxyType = "COMMERCIAL_VPN"
	ProxyTypeEnterpriseVPN    ProxyType = "ENTERPRISE_VPN"
	ProxyTypeMobileProxy      ProxyType = "MOBILE_PROXY"
	ProxyTypeBlockchainProxy  ProxyType = "BLOCKCHAIN_PROXY"
	ProxyTypeResidentialProxy ProxyType = "RESIDENTIAL_PROXY"
	ProxyTypePublicProxy      ProxyType = "PUBLIC_PROXY"
	ProxyTypeDatacenterProxy  ProxyType = "DATACENTER_PROXY"
	ProxyTypeTorNode          ProxyType = "TOR_NODE"
	ProxyTypePrivateRelay     ProxyType = "PRIVATE_RELAY"
	ProxyTypeBotnet           ProxyType = "BOTNET"
	ProxyTypeSearchEngine     ProxyType = "SEARCH_ENGINE"
	ProxyTypeAICrawler        ProxyType = "AI_CRAWLER"
	ProxyTypeSocialMedia      ProxyType = "SOCIAL_MEDIA"
	ProxyTypeUptimeMonitor    ProxyType = "UPTIME_MONITOR"
	ProxyTypeLinkPreview      ProxyType = "LINK_PREVIEW"
	ProxyTypeSEOCrawler       ProxyType = "SEO_CRAWLER"
	ProxyTypeWebArchiver      ProxyType = "WEB_ARCHIVER"
	ProxyTypeWebhookProvider  ProxyType = "WEBHOOK_PROVIDER"
	ProxyTypePaymentProcessor ProxyType = "PAYMENT_PROCESSOR"
	ProxyTypeInternetScanner  ProxyType = "INTERNET_SCANNER"
)

// ProxyEvent is a proxy sighting from the Proxies feed.
type ProxyEvent struct {
	// IP is the proxy exit address.
	IP string `json:"ip"`
	// Provider is the proxy service tag.
	Provider string `json:"provider"`
	// Type is the proxy category.
	Type ProxyType `json:"type"`
	// CountryCode is the ISO 3166-1 alpha-2 country of the address.
	CountryCode string `json:"country_code"`
	// ASN is the autonomous system number of the address.
	ASN int64 `json:"asn"`
	// Timestamp is when the proxy was seen, in Unix seconds.
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
	// Type is the service category, such as ProxyTypeCommercialVPN.
	Type ProxyType `json:"type"`
	// CountryCode is the ISO 3166-1 alpha-2 country of RangeStart.
	CountryCode string `json:"country_code"`
	// ASN is the autonomous system number of RangeStart.
	ASN int64 `json:"asn"`
	// Timestamp is when the range was seen, in Unix milliseconds.
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
	// Timestamp is when the torrent was seen, in Unix seconds.
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
	// Type is the service category, such as ProxyTypeDatacenterProxy.
	Type ProxyType `json:"type"`
	// CountryCode is the ISO 3166-1 alpha-2 country of the address.
	CountryCode string `json:"country_code"`
	// ASN is the autonomous system number of the address.
	ASN int64 `json:"asn"`
	// Timestamp is when the fingerprint was seen, in Unix seconds.
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
	// Timestamp is when the request was captured, in Unix milliseconds.
	Timestamp Timestamp `json:"timestamp"`
	// TunnelID identifies the proxy tunnel the request arrived through.
	TunnelID int64 `json:"tunnel_id"`
	// Domain is the host the tunnel was opened to.
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
	Raw []byte `json:"raw"`
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
	// Timestamp is when the handshake was captured, in Unix milliseconds.
	Timestamp Timestamp `json:"timestamp"`
	// TunnelID identifies the proxy tunnel the handshake arrived through.
	TunnelID int64 `json:"tunnel_id"`
	// Domain is the host the tunnel was opened to.
	Domain string `json:"domain"`
	// Port is the destination port.
	Port int `json:"port"`
	// Protocol is "https".
	Protocol string `json:"protocol"`
	// Meta describes the capturing sensor.
	Meta HeliosMeta `json:"meta"`
	// Details is the parsed ClientHello, or nil if the sensor could not parse
	// the handshake.
	Details *TLSClientHello `json:"details"`
	// Raw is always empty, because the sensors do not capture handshake bytes.
	Raw string `json:"raw"`
}

// TLSClientHello is a TLS ClientHello parsed by a Helios sensor. Fields the
// sensor could not parse are empty.
type TLSClientHello struct {
	// RecordVersion is the record layer version, such as "TLS 1.0".
	RecordVersion string `json:"record_version"`
	// HandshakeVersion is the ClientHello version, such as "TLS 1.2".
	HandshakeVersion string `json:"handshake_version"`
	// ClientRandom is the hex-encoded 32-byte client random.
	ClientRandom string `json:"client_random"`
	// SessionID is the hex-encoded session ID.
	SessionID string `json:"session_id"`
	// SessionIDLength is the length of the session ID in bytes.
	SessionIDLength int `json:"session_id_length"`
	// CipherSuites are the offered cipher suites.
	CipherSuites []TLSCodeName `json:"cipher_suites"`
	// CompressionMethods are the offered compression methods.
	CompressionMethods []int `json:"compression_methods"`
	// Extensions are the extensions in wire order.
	Extensions []TLSExtension `json:"extensions"`
	// SNI is the server_name extension value.
	SNI string `json:"sni"`
	// ALPN lists the offered application protocols.
	ALPN []string `json:"alpn"`
	// SupportedVersions are the offered TLS versions.
	SupportedVersions []string `json:"supported_versions"`
	// SupportedGroups are the offered key exchange groups.
	SupportedGroups []TLSCodeName `json:"supported_groups"`
	// SignatureAlgorithms are the offered signature algorithms.
	SignatureAlgorithms []TLSCodeName `json:"signature_algorithms"`
	// ECPointFormats are the offered elliptic curve point formats.
	ECPointFormats []string `json:"ec_point_formats"`
	// KeyShareGroups are the groups the client sent key shares for.
	KeyShareGroups []string `json:"key_share_groups"`
	// PSKKeyExchangeModes are the offered pre-shared key exchange modes.
	PSKKeyExchangeModes []string `json:"psk_key_exchange_modes"`
	// CompressCertificate lists the offered certificate compression algorithms.
	CompressCertificate []string `json:"compress_certificate"`
	// HasGREASE reports whether the ClientHello contains GREASE values.
	HasGREASE bool `json:"has_grease"`
	// ApplicationSettings reports whether the client offered application
	// settings (ALPS).
	ApplicationSettings bool `json:"application_settings"`
	// DelegatedCredentials reports whether the client offered delegated
	// credentials.
	DelegatedCredentials bool `json:"delegated_credentials"`
	// EncryptThenMAC reports whether the client offered encrypt-then-MAC.
	EncryptThenMAC bool `json:"encrypt_then_mac"`
	// ExtendedMasterSecret reports whether the client offered the extended
	// master secret.
	ExtendedMasterSecret bool `json:"extended_master_secret"`
	// PostHandshakeAuth reports whether the client offered post-handshake
	// authentication.
	PostHandshakeAuth bool `json:"post_handshake_auth"`
	// RenegotiationInfo reports whether the client sent renegotiation info.
	RenegotiationInfo bool `json:"renegotiation_info"`
	// SignedCertificateTimestamps reports whether the client requested signed
	// certificate timestamps.
	SignedCertificateTimestamps bool `json:"signed_certificate_timestamps"`
	// StatusRequest reports whether the client requested OCSP stapling.
	StatusRequest bool `json:"status_request"`
}

// TLSCodeName is a TLS registry code point and its name. GREASE values are
// named "GREASE(0x....)".
type TLSCodeName struct {
	// Code is the code point.
	Code int `json:"code"`
	// Name is the code point's registered name.
	Name string `json:"name"`
}

// TLSExtension is an extension in a TLS ClientHello.
type TLSExtension struct {
	// Code is the extension type.
	Code int `json:"code"`
	// Name is the extension's registered name.
	Name string `json:"name"`
	// Length is the extension body length in bytes.
	Length int `json:"length"`
}

// HeliosDNSEvent is a destination that a Helios honeypot tunnel was asked to
// reach. Despite the feed's name, it is not a DNS query: names resolved inside
// a tunneled DNS session are not visible.
type HeliosDNSEvent struct {
	// Timestamp is when the event was captured, in Unix milliseconds.
	Timestamp Timestamp `json:"timestamp"`
	// TunnelID identifies the proxy tunnel.
	TunnelID int64 `json:"tunnel_id"`
	// Domain is the host name or address literal the tunnel was opened to.
	Domain string `json:"domain"`
	// Port is the destination port.
	Port int `json:"port"`
	// Protocol is "dns".
	Protocol string `json:"protocol"`
	// Meta describes the capturing sensor.
	Meta HeliosMeta `json:"meta"`
}

// HeliosADBEvent is an Android Debug Bridge shell command captured by a
// Helios honeypot. It has no timestamp because the sensor sends none.
type HeliosADBEvent struct {
	// Session identifies the ADB session as 32 hex characters.
	Session string `json:"session"`
	// SequentialID is the 1-based position of the command within its session.
	SequentialID int64 `json:"sequential_id"`
	// Command is the shell command. Invalid UTF-8 bytes are replaced with
	// U+FFFD.
	Command string `json:"command"`
	// Hash is the hex-encoded SHA-256 the sensor assigns to the session's
	// payload. It is not a hash of Command: every command in a session shares
	// it, and sessions that replay the same script share it too.
	Hash string `json:"hash"`
}
