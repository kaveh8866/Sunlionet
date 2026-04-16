package profile

// Family represents the protocol family of the outbound strategy
type Family string

const (
	FamilyReality   Family = "reality"
	FamilyHysteria2 Family = "hysteria2"
	FamilyTUIC      Family = "tuic"
	FamilyShadowTLS Family = "shadowtls"
	FamilyDNS       Family = "dns_tunnel"
)

// Endpoint defines the remote destination
type Endpoint struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	IPVersion string `json:"ip_version"` // v4, v6, dual
}

// Capabilities defines the dpi resistance traits
type Capabilities struct {
	Transport         string   `json:"transport"`           // tcp, udp, quic
	DPIResistanceTags []string `json:"dpi_resistance_tags"` // tls_camouflage, sni_rotation, etc.
	BandwidthClass    string   `json:"bandwidth_class"`     // high, medium, low
}

type Credentials struct {
	UUID            string `json:"uuid,omitempty"`
	Password        string `json:"password,omitempty"`
	PublicKey       string `json:"public_key,omitempty"`
	ShortID         string `json:"short_id,omitempty"`
	SNI             string `json:"sni,omitempty"`
	UTLSFingerprint string `json:"utls_fingerprint,omitempty"`
	ObfsPassword    string `json:"obfs_password,omitempty"`
}

// MutationPolicy defines allowed modifications by the LLM/Policy engine
type MutationPolicy struct {
	AllowedSets         []string `json:"allowed_sets"`
	MaxMutationsPerHour int      `json:"max_mutations_per_hour"`
}

// Health tracks the local success ranking of a profile
type Health struct {
	Score             float64 `json:"score"` // Composite EWMA score
	SuccessEWMA       float64 `json:"success_ewma"`
	MedianHandshakeMs int     `json:"median_handshake_ms"`
	LastOkAt          int64   `json:"last_ok_at"`
	LastFailAt        int64   `json:"last_fail_at"`
	LastFailReason    string  `json:"last_fail_reason"`
	CooldownUntil     int64   `json:"cooldown_until"` // Unix timestamp
	FailureCount      int     `json:"failure_count"`
	ConsecutiveFails  int     `json:"consecutive_fails"`
}

type SourceInfo struct {
	Source       string `json:"source,omitempty"`
	TrustLevel   int    `json:"trust_level"`
	ImportedAt   int64  `json:"imported_at"`
	PublisherKey string `json:"publisher_key,omitempty"`
}

// Profile represents a pre-validated outbound template + metadata
type Profile struct {
	ID             string         `json:"id"`
	Family         Family         `json:"family"`
	Enabled        bool           `json:"enabled"`
	ManualDisabled bool           `json:"manual_disabled"`
	CreatedAt      int64          `json:"created_at"`
	ExpiresAt      int64          `json:"expires_at"`
	Priority       int            `json:"priority"`
	TemplateRef    string         `json:"template_ref"` // points to vetted snippets
	Endpoint       Endpoint       `json:"endpoint"`
	Capabilities   Capabilities   `json:"capabilities"`
	Credentials    Credentials    `json:"credentials"`
	Source         SourceInfo     `json:"source"`
	Notes          string         `json:"notes"`
	MutationPolicy MutationPolicy `json:"mutation_policy"`
	Health         Health         `json:"health"`
}
