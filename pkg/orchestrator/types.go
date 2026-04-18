package orchestrator

type DecisionRequest struct {
	Type        string            `json:"type"`
	Timestamp   int64             `json:"timestamp"`
	Network     NetworkState      `json:"network"`
	Profiles    []ProfileSnapshot `json:"profiles"`
	History     DecisionHistory   `json:"history"`
	Constraints Constraints       `json:"constraints"`
	Adaptive    AdaptiveInput     `json:"adaptive,omitempty"`
}

type AdaptiveInput struct {
	Scores         []ScoreHint `json:"scores,omitempty"`
	RecentFailures []string    `json:"recent_failures,omitempty"`
}

type ScoreHint struct {
	ProfileID string  `json:"profile_id"`
	Score     float64 `json:"score"`
}

type NetworkState struct {
	UDPBlocked   bool `json:"udp_blocked"`
	DNSPoisoning bool `json:"dns_poisoning"`
	LatencyMS    int  `json:"latency_ms"`
}

type ProfileSnapshot struct {
	ID          string `json:"id"`
	Protocol    string `json:"protocol"`
	LastSuccess int64  `json:"last_success"`
	Failures    int    `json:"failures"`
	Cooldown    bool   `json:"cooldown"`
}

type DecisionHistory struct {
	RecentSwitches []string `json:"recent_switches"`
	FailRate       float64  `json:"fail_rate"`
}

type Constraints struct {
	MaxSwitchRate int  `json:"max_switch_rate"`
	NoUDP         bool `json:"no_udp"`
}

type AllowedAction string

const (
	ActionSwitchProfile   AllowedAction = "switch_profile"
	ActionHoldCurrent     AllowedAction = "hold_current"
	ActionCooldownProfile AllowedAction = "cooldown_profile"
	ActionImportBundle    AllowedAction = "import_bundle"
	ActionActivateMesh    AllowedAction = "activate_mesh"
)

type DecisionResponse struct {
	Action     AllowedAction `json:"action"`
	ProfileID  string        `json:"profile_id,omitempty"`
	Reason     string        `json:"reason"`
	Confidence float64       `json:"confidence"`
	Fallback   []string      `json:"fallback,omitempty"`
}

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Method  string          `json:"method"`
	Params  DecisionRequest `json:"params"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type JSONRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      uint64           `json:"id"`
	Result  DecisionResponse `json:"result"`
	Error   *JSONRPCError    `json:"error,omitempty"`
}
