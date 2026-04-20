package policy

import (
	"fmt"
	"sort"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/detector"
	"github.com/kaveh/sunlionet-agent/pkg/profile"
)

type ActionType string

const (
	ActionKeep              ActionType = "KEEP"
	ActionSwitchProfile     ActionType = "SWITCH_PROFILE"
	ActionSwitchFamily      ActionType = "SWITCH_FAMILY"
	ActionEnterEmergencyDNS ActionType = "ENTER_EMERGENCY_DNS"
	ActionReduceProbes      ActionType = "REDUCE_PROBES"
	ActionRollbackLastGood  ActionType = "ROLLBACK_LAST_GOOD"
	ActionInvokeLLM         ActionType = "INVOKE_LLM" // Explicit handoff
)

const (
	scoreScale             = 100.0
	latencyPenalty         = 10.0
	recentSuccessBonus     = 5.0
	recentSuccessWindowSec = int64(3600)
	highLatencyMs          = 1000
	lowLatencyMs           = 300
	trustWeight            = 0.2
	freshImportWindowSec   = int64(7 * 24 * 3600)
	freshImportBonus       = 3.0
	consecutiveFailPenalty = 15.0
	failCountPenalty       = 1.0
	tcpBonus               = 1.0
)

// Action is the output of the policy engine or LLM advisor
type Action struct {
	Type          ActionType `json:"action"`
	TargetProfile string     `json:"target_profile_id,omitempty"`
	MutationSet   string     `json:"mutation_set,omitempty"`
	CooldownSec   int        `json:"cooldown_sec,omitempty"`
	ReasonCode    string     `json:"reason_code,omitempty"`
	Confidence    float64    `json:"confidence,omitempty"`
}

// Advisor interface for the bounded LLM planner
type Advisor interface {
	ProposeAction(
		fingerprint string,
		currentProfile profile.Profile,
		candidates []profile.Profile,
		recentEvents []detector.Event,
	) (Action, error)
}

// Engine evaluates events deterministically before calling the LLM
type Engine struct {
	MaxBurstFailures int
	AdaptiveState    *AdaptiveState
}

// RankProfiles sorts profiles by health score, prioritizing high-success, low-cooldown profiles
func (e *Engine) RankProfiles(profiles []profile.Profile) []profile.Profile {
	if e != nil && e.AdaptiveState != nil {
		if ranked, _ := e.AdaptiveState.SelectProfile(profiles, time.Now()); len(ranked) > 0 {
			return ranked
		}
	}

	// Filter out profiles currently in cooldown
	now := time.Now().Unix()
	var available []profile.Profile
	for _, p := range profiles {
		if !p.Enabled {
			continue
		}
		if p.ManualDisabled {
			continue
		}
		if p.Health.CooldownUntil > now {
			continue
		}

		// Calculate dynamic health score
		// Base score is EWMA of success rate (0.0 to 1.0)
		score := p.Health.SuccessEWMA * scoreScale

		// Penalize high handshake latency
		if p.Health.MedianHandshakeMs > highLatencyMs {
			score -= latencyPenalty
		} else if p.Health.MedianHandshakeMs < lowLatencyMs && p.Health.MedianHandshakeMs > 0 {
			score += latencyPenalty
		}

		// Reward recently successful profiles
		if now-p.Health.LastOkAt < recentSuccessWindowSec {
			score += recentSuccessBonus
		}

		if p.Source.TrustLevel == 0 {
			p.Source.TrustLevel = 50
		}
		score += float64(p.Source.TrustLevel) * trustWeight

		if p.Source.ImportedAt > 0 && now-p.Source.ImportedAt < freshImportWindowSec {
			score += freshImportBonus
		}

		score -= float64(p.Health.ConsecutiveFails) * consecutiveFailPenalty
		score -= float64(p.Health.FailureCount) * failCountPenalty

		if p.Capabilities.Transport == "tcp" {
			score += tcpBonus
		}

		p.Health.Score = score
		available = append(available, p)
	}

	// Sort descending by score
	sort.Slice(available, func(i, j int) bool {
		return available[i].Health.Score > available[j].Health.Score
	})

	return available
}

func (e *Engine) SelectProfile(profiles []profile.Profile) (profile.Profile, Decision, []profile.Profile) {
	decision := Decision{}
	if e != nil && e.AdaptiveState != nil && e.AdaptiveState.Enabled {
		ranked, d := e.AdaptiveState.SelectProfile(profiles, time.Now())
		if len(ranked) > 0 {
			return ranked[0], d, ranked
		}
		decision = d
	}

	ranked := e.RankProfiles(profiles)
	if len(ranked) == 0 {
		decision.Reason = "no viable profiles after deterministic ranking"
		return profile.Profile{}, decision, ranked
	}
	decision.Selected = ranked[0].ID
	decision.Reason = fmt.Sprintf("deterministic fallback score=%.1f", ranked[0].Health.Score)
	decision.Scores = make([]ScoredProfile, 0, len(ranked))
	for _, p := range ranked {
		decision.Scores = append(decision.Scores, ScoredProfile{ProfileID: p.ID, Score: p.Health.Score})
	}
	return ranked[0], decision, ranked
}

// Evaluate applies hardcoded deterministic rules
func (e *Engine) Evaluate(events []detector.Event, activeProfile profile.Profile) Action {
	// Simple mock rule evaluation logic for V1

	// If no events, we keep the profile
	if len(events) == 0 {
		return Action{Type: ActionKeep, ReasonCode: "stable"}
	}

	lastEvent := events[len(events)-1]

	// 1. High-confidence DNS Poisoning -> Switch to secure DNS tunnel mode or rotate DNS
	if lastEvent.Type == detector.EventDNSPoisonSuspected && lastEvent.Severity == detector.SeverityCritical {
		return Action{
			Type:        ActionEnterEmergencyDNS,
			ReasonCode:  "dns_poison_critical",
			CooldownSec: 1800,
		}
	}

	// 2. High-confidence UDP Blocking -> Leave UDP family (Hysteria/TUIC) for TCP (Reality/ShadowTLS)
	if lastEvent.Type == detector.EventUDPBlockSuspected && activeProfile.Capabilities.Transport == "udp" {
		return Action{
			Type:       ActionSwitchFamily,
			ReasonCode: "udp_block_suspected",
			// Target Family would be chosen by supervisor or passed explicitly
		}
	}

	// 3. Handshake bursts -> Switch profile within same family
	if lastEvent.Type == detector.EventHandshakeBurstFailure {
		return Action{
			Type:       ActionSwitchProfile,
			ReasonCode: "burst_failures",
			// Needs next best profile ID
		}
	}

	// 4. Ambiguous or multi-signal cases -> Handoff to LLM advisor
	// (e.g. SNI block + UDP block signals happening together)
	return Action{
		Type:       ActionInvokeLLM,
		ReasonCode: "ambiguous_signals",
	}
}
