package bundle

import "github.com/kaveh/shadownet-agent/pkg/profile"

// BundleHeader represents the authenticated metadata wrapping the encrypted payload
type BundleHeader struct {
	Magic          string `json:"magic"` // "SNB1"
	BundleID       string `json:"bundle_id"`
	PublisherKeyID string `json:"publisher_key_id"`
	RecipientKeyID string `json:"recipient_key_id"`
	Seq            uint64 `json:"seq"`
	CreatedAt      int64  `json:"created_at"`
	ExpiresAt      int64  `json:"expires_at"`
	Cipher         string `json:"cipher"` // e.g. "x25519+chacha20poly1305"
	Signature      string `json:"sig"`    // "ed25519" signature over header + ciphertext
}

// Template represents a sing-box snippet
type Template struct {
	TemplateText string `json:"template_text"`
}

// PolicyOverrides represent updated engine constraints
type PolicyOverrides struct {
	CooldownHardFailSec int `json:"cooldown_hard_fail_sec"`
	MaxSwitchesPer10Min int `json:"max_switches_per_10min"`
}

// BundlePayload is the decrypted inner JSON contents
type BundlePayload struct {
	SchemaVersion   int                 `json:"schema_version"`
	MinAgentVersion string              `json:"min_agent_version"`
	Profiles        []profile.Profile   `json:"profiles"`
	Revocations     []string            `json:"revocations"`
	PolicyOverrides PolicyOverrides     `json:"policy_overrides"`
	Templates       map[string]Template `json:"templates"`
	Notes           map[string]string   `json:"notes"`
}
