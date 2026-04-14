package detector

import "time"

type EventType string

const (
	EventDNSPoisonSuspected    EventType = "DNS_POISON_SUSPECTED"
	EventSNIBlockSuspected     EventType = "SNI_BLOCK_SUSPECTED"
	EventUDPBlockSuspected     EventType = "UDP_BLOCK_SUSPECTED"
	EventActiveResetSuspected  EventType = "ACTIVE_RESET_SUSPECTED"
	EventThroughputCollapse    EventType = "THROUGHPUT_COLLAPSE"
	EventHandshakeBurstFailure EventType = "HANDSHAKE_BURST_FAILURE"
	EventRecoveryConfirmed     EventType = "RECOVERY_CONFIRMED"
)

type Severity string

const (
	SeverityLow      Severity = "LOW"
	SeverityMedium   Severity = "MEDIUM"
	SeverityHigh     Severity = "HIGH"
	SeverityCritical Severity = "CRITICAL"
)

// Event represents a normalized observation of network interference
type Event struct {
	Type      EventType         `json:"type"`
	Severity  Severity          `json:"severity"`
	Timestamp time.Time         `json:"timestamp"`
	ProfileID string            `json:"profile_id,omitempty"` // The active profile when this happened
	Metadata  map[string]string `json:"metadata,omitempty"`   // Extra non-secret counters
}
