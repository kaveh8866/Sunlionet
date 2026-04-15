package comms

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

type AppID string

const (
	AppSession AppID = "session"
	AppBriar   AppID = "briar"
	AppSimpleX AppID = "simplex"
	AppMatrix  AppID = "matrix"
	AppSignal  AppID = "signal_optional"
)

type DeviceRole string

const (
	RoleInside  DeviceRole = "inside"
	RoleOutside DeviceRole = "outside"
)

type TrustLevel string

const (
	TrustTrusted     TrustLevel = "trusted"
	TrustUnverified  TrustLevel = "unverified"
	TrustCompromised TrustLevel = "compromised"
	TrustInactive    TrustLevel = "inactive"
)

const (
	SchemaV1 = 1

	// Conservative caps keep local state bounded on low-end Android devices.
	MaxInstalledApps      = 10
	MaxContacts           = 500
	MaxChannelsPerContact = 5
	MaxBundleHistory      = 200

	// Keep import history useful for incident review while limiting metadata retention.
	BundleHistoryRetentionDays = 90
)

type InstalledApp struct {
	App          AppID  `json:"app"`
	Installed    bool   `json:"installed"`
	Enabled      bool   `json:"enabled"`
	Version      string `json:"version,omitempty"`
	DeepLinkHint string `json:"deeplink_hint,omitempty"`
	LastSeenAt   int64  `json:"last_seen_at,omitempty"`
}

type ContactChannel struct {
	App            AppID  `json:"app"`
	IdentifierHint string `json:"identifier_hint,omitempty"`
	ChannelAlias   string `json:"channel_alias,omitempty"`
	LastSuccessAt  int64  `json:"last_success_at,omitempty"`
}

type TrustedContact struct {
	ID             string           `json:"id"`
	Alias          string           `json:"alias"`
	Trust          TrustLevel       `json:"trust"`
	Channels       []ContactChannel `json:"channels"`
	PreferredOrder []AppID          `json:"preferred_order,omitempty"`
	LastVerifiedAt int64            `json:"last_verified_at,omitempty"`
}

type BundleImportRecord struct {
	BundleID     string `json:"bundle_id"`
	FromContact  string `json:"from_contact,omitempty"`
	SourceApp    AppID  `json:"source_app"`
	SignatureKey string `json:"signature_key,omitempty"`
	ReceivedAt   int64  `json:"received_at"`
	SizeBytes    int    `json:"size_bytes,omitempty"`
	Status       string `json:"status"` // accepted/rejected/error
	Reason       string `json:"reason,omitempty"`
}

type BlackoutReadiness struct {
	LastChecklistAt    int64 `json:"last_checklist_at,omitempty"`
	LastBriarLocalTest int64 `json:"last_briar_local_test,omitempty"`
	HasPrimaryContact  bool  `json:"has_primary_contact"`
	HasBackupContact   bool  `json:"has_backup_contact"`
	HasOfflineContact  bool  `json:"has_offline_contact"`
}

// State is encrypted at rest by Store and is intentionally minimal.
// External app identifiers should remain short hints and aliases only.
type State struct {
	SchemaVersion int   `json:"schema_version"`
	UpdatedAt     int64 `json:"updated_at"`

	Role DeviceRole `json:"role"`

	InstalledApps []InstalledApp   `json:"installed_apps"`
	Contacts      []TrustedContact `json:"contacts"`

	// Scenario-specific preference order, e.g. "normal", "degraded", "blackout".
	ChannelPreference map[string][]AppID `json:"channel_preference,omitempty"`

	BundleHistory []BundleImportRecord `json:"bundle_history,omitempty"`
	Readiness     BlackoutReadiness    `json:"blackout_readiness"`
}

func NewState(role DeviceRole) *State {
	return &State{
		SchemaVersion:     SchemaV1,
		UpdatedAt:         time.Now().Unix(),
		Role:              role,
		InstalledApps:     []InstalledApp{},
		Contacts:          []TrustedContact{},
		ChannelPreference: map[string][]AppID{},
		BundleHistory:     []BundleImportRecord{},
	}
}

func (s *State) Validate() error {
	if s.SchemaVersion != SchemaV1 {
		return fmt.Errorf("comms: unsupported schema_version: %d", s.SchemaVersion)
	}
	if s.Role != RoleInside && s.Role != RoleOutside {
		return fmt.Errorf("comms: invalid role: %q", s.Role)
	}
	if len(s.InstalledApps) > MaxInstalledApps {
		return fmt.Errorf("comms: too many installed apps: %d", len(s.InstalledApps))
	}
	if len(s.Contacts) > MaxContacts {
		return fmt.Errorf("comms: too many contacts: %d", len(s.Contacts))
	}

	contactSeen := make(map[string]struct{}, len(s.Contacts))
	for i := range s.Contacts {
		c := s.Contacts[i]
		if c.ID == "" {
			return errors.New("comms: contact id required")
		}
		if _, exists := contactSeen[c.ID]; exists {
			return fmt.Errorf("comms: duplicate contact id: %q", c.ID)
		}
		contactSeen[c.ID] = struct{}{}
		if c.Alias == "" {
			return fmt.Errorf("comms: contact alias required for id: %q", c.ID)
		}
		if len(c.Channels) == 0 {
			return fmt.Errorf("comms: contact channels required for id: %q", c.ID)
		}
		if len(c.Channels) > MaxChannelsPerContact {
			return fmt.Errorf("comms: too many channels for contact %q", c.ID)
		}
		for j := range c.Channels {
			if c.Channels[j].App == "" {
				return fmt.Errorf("comms: channel app required for contact %q", c.ID)
			}
		}
	}

	if len(s.BundleHistory) > MaxBundleHistory*2 {
		return fmt.Errorf("comms: bundle history unexpectedly large: %d", len(s.BundleHistory))
	}
	return nil
}

func (s *State) Prune(now time.Time) {
	s.UpdatedAt = now.Unix()
	cutoff := now.Add(-BundleHistoryRetentionDays * 24 * time.Hour).Unix()

	kept := make([]BundleImportRecord, 0, len(s.BundleHistory))
	for i := range s.BundleHistory {
		if s.BundleHistory[i].ReceivedAt >= cutoff {
			kept = append(kept, s.BundleHistory[i])
		}
	}
	sort.Slice(kept, func(i, j int) bool {
		return kept[i].ReceivedAt > kept[j].ReceivedAt
	})
	if len(kept) > MaxBundleHistory {
		kept = kept[:MaxBundleHistory]
	}
	s.BundleHistory = kept
}
