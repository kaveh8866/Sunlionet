package policy

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/profile"
)

const (
	defaultAdaptiveWindowSize   = 80
	adaptiveCooldownSeconds     = int64(60)
	diversityStreakLimit        = 10
	diversityScoreDelta         = 5.0
	orchestratorCloseScoreDelta = 3.0
)

type AttemptSignal struct {
	LatencyMS    int  `json:"latency_ms"`
	ConnectOK    bool `json:"connect_success"`
	DNSOK        bool `json:"dns_ok"`
	TCPHandshake bool `json:"tcp_handshake"`
	TLSSuccess   bool `json:"tls_success"`
}

type NetworkEvent struct {
	Timestamp int64  `json:"timestamp"`
	ProfileID string `json:"profile_id"`
	Success   bool   `json:"success"`
	Latency   int    `json:"latency"`
	Failure   string `json:"failure"`
}

type ScoredProfile struct {
	ProfileID string  `json:"profile"`
	Score     float64 `json:"score"`
}

type Decision struct {
	Selected        string          `json:"selected"`
	Reason          string          `json:"reason"`
	Scores          []ScoredProfile `json:"scores,omitempty"`
	RecentFailures  []string        `json:"recent_failures,omitempty"`
	UseOrchestrator bool            `json:"use_orchestrator"`
}

type AdaptiveState struct {
	Enabled          bool             `json:"enabled"`
	MaxEvents        int              `json:"max_events"`
	Events           []NetworkEvent   `json:"events"`
	Cooldown         map[string]int64 `json:"cooldown"`
	SelectionHistory []string         `json:"selection_history"`

	mu sync.Mutex `json:"-"`
}

type AdaptiveStateDisk struct {
	Enabled          bool             `json:"enabled"`
	MaxEvents        int              `json:"max_events"`
	Events           []NetworkEvent   `json:"events"`
	Cooldown         map[string]int64 `json:"cooldown"`
	SelectionHistory []string         `json:"selection_history"`
}

func NewAdaptiveState(maxEvents int) *AdaptiveState {
	if maxEvents < 50 {
		maxEvents = 50
	}
	if maxEvents > 100 {
		maxEvents = 100
	}
	return &AdaptiveState{
		Enabled:   true,
		MaxEvents: maxEvents,
		Events:    make([]NetworkEvent, 0, maxEvents),
		Cooldown:  make(map[string]int64),
	}
}

func (s *AdaptiveState) ensureDefaultsLocked() {
	if s.MaxEvents < 50 || s.MaxEvents > 100 {
		s.MaxEvents = defaultAdaptiveWindowSize
	}
	if s.Events == nil {
		s.Events = make([]NetworkEvent, 0, s.MaxEvents)
	}
	if s.Cooldown == nil {
		s.Cooldown = make(map[string]int64)
	}
}

func (s *AdaptiveState) SnapshotDisk() AdaptiveStateDisk {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultsLocked()
	snapshot := AdaptiveStateDisk{
		Enabled:          s.Enabled,
		MaxEvents:        s.MaxEvents,
		Events:           append([]NetworkEvent(nil), s.Events...),
		Cooldown:         make(map[string]int64, len(s.Cooldown)),
		SelectionHistory: append([]string(nil), s.SelectionHistory...),
	}
	for k, v := range s.Cooldown {
		snapshot.Cooldown[k] = v
	}
	return snapshot
}

func (s *AdaptiveState) ReplaceDisk(from AdaptiveStateDisk) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Enabled = from.Enabled
	s.MaxEvents = from.MaxEvents
	s.Events = append([]NetworkEvent(nil), from.Events...)
	s.Cooldown = make(map[string]int64, len(from.Cooldown))
	for k, v := range from.Cooldown {
		s.Cooldown[k] = v
	}
	s.SelectionHistory = append([]string(nil), from.SelectionHistory...)
	s.ensureDefaultsLocked()
}

func (s *AdaptiveState) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultsLocked()
	s.Events = s.Events[:0]
	s.SelectionHistory = nil
	clear(s.Cooldown)
}

func (s *AdaptiveState) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Enabled = enabled
}

func (s *AdaptiveState) RecordSelection(profileID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(profileID) == "" {
		return
	}
	s.SelectionHistory = append(s.SelectionHistory, profileID)
	if len(s.SelectionHistory) > 32 {
		s.SelectionHistory = append([]string(nil), s.SelectionHistory[len(s.SelectionHistory)-32:]...)
	}
}

func (s *AdaptiveState) RecordAttempt(profileID string, signal AttemptSignal, failure string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultsLocked()
	if !s.Enabled || strings.TrimSpace(profileID) == "" {
		return
	}

	failure = normalizeFailure(signal, failure)
	ev := NetworkEvent{
		Timestamp: now.Unix(),
		ProfileID: profileID,
		Success:   signal.ConnectOK,
		Latency:   signal.LatencyMS,
		Failure:   failure,
	}
	s.Events = append(s.Events, ev)
	if len(s.Events) > s.MaxEvents {
		s.Events = append([]NetworkEvent(nil), s.Events[len(s.Events)-s.MaxEvents:]...)
	}

	if !signal.ConnectOK {
		s.Cooldown[profileID] = now.Unix() + adaptiveCooldownSeconds
	}
}

func normalizeFailure(signal AttemptSignal, failure string) string {
	nf := strings.ToUpper(strings.TrimSpace(failure))
	if nf == "" || nf == "UNKNOWN" {
		switch {
		case !signal.DNSOK:
			return "DNS_BLOCKED"
		case signal.TCPHandshake && !signal.TLSSuccess:
			return "TLS_BLOCKED"
		case !signal.TCPHandshake:
			return "TCP_RESET"
		default:
			return "UNKNOWN"
		}
	}

	switch nf {
	case "DNS_FAILURE", "DNS_BLOCKED", "DNS_POISON_SUSPECTED":
		return "DNS_BLOCKED"
	case "UDP_BLOCK_SUSPECTED", "UDP_BLOCKED":
		return "UDP_BLOCKED"
	case "TLS_BLOCKED", "TLS_FAILURE":
		return "TLS_BLOCKED"
	case "TIMEOUT":
		return "TIMEOUT"
	case "NO_ROUTE", "NETWORK_BLOCKED":
		return "NO_ROUTE"
	case "TCP_RESET", "ACTIVE_RESET_SUSPECTED":
		return "TCP_RESET"
	default:
		return nf
	}
}

func hasTag(tags []string, wanted string) bool {
	wanted = strings.ToLower(strings.TrimSpace(wanted))
	for _, t := range tags {
		if strings.EqualFold(strings.TrimSpace(t), wanted) {
			return true
		}
	}
	return false
}

func decayWeight(now int64, eventTS int64) float64 {
	age := now - eventTS
	if age <= 0 {
		return 1
	}
	// Linear decay over ~5 minutes worth of windows.
	w := 1.0 - (float64(age) / float64(5*60))
	if w < 0.1 {
		return 0.1
	}
	return w
}

func (s *AdaptiveState) SelectProfile(profiles []profile.Profile, now time.Time) ([]profile.Profile, Decision) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultsLocked()

	decision := Decision{}
	if !s.Enabled {
		decision.Reason = "adaptive mode disabled"
		return nil, decision
	}

	nowUnix := now.Unix()
	recentFailures := make([]string, 0, 6)
	recentFailureSet := map[string]bool{}
	for i := len(s.Events) - 1; i >= 0 && len(recentFailures) < 6; i-- {
		ev := s.Events[i]
		if ev.Success || strings.TrimSpace(ev.Failure) == "" {
			continue
		}
		if !recentFailureSet[ev.Failure] {
			recentFailures = append(recentFailures, ev.Failure)
			recentFailureSet[ev.Failure] = true
		}
	}

	ranked := make([]profile.Profile, 0, len(profiles))
	scored := make([]ScoredProfile, 0, len(profiles))
	excluded := map[string]bool{
		"DNS_BLOCKED": recentFailureSet["DNS_BLOCKED"],
	}

	type aggregated struct {
		successW  float64
		totalW    float64
		latencyW  float64
		latencyWW float64
		failW     float64
	}
	ag := make(map[string]aggregated, len(profiles))
	for _, ev := range s.Events {
		w := decayWeight(nowUnix, ev.Timestamp)
		a := ag[ev.ProfileID]
		if ev.Success {
			a.successW += w
		} else {
			a.failW += w
		}
		a.totalW += w
		if ev.Latency > 0 {
			a.latencyW += float64(ev.Latency) * w
			a.latencyWW += w
		}
		ag[ev.ProfileID] = a
	}

	for _, p := range profiles {
		if !p.Enabled || p.ManualDisabled {
			continue
		}
		if p.Health.CooldownUntil > nowUnix {
			continue
		}
		if until := s.Cooldown[p.ID]; until > nowUnix {
			continue
		}
		if excluded["DNS_BLOCKED"] && (p.Family == profile.FamilyDNS || hasTag(p.Capabilities.DPIResistanceTags, "dns-dependent")) {
			continue
		}

		a := ag[p.ID]
		successRate := p.Health.SuccessEWMA
		if a.totalW > 0 {
			successRate = a.successW / a.totalW
		}
		latency := float64(p.Health.MedianHandshakeMs)
		if a.latencyWW > 0 {
			latency = a.latencyW / a.latencyWW
		}
		failCount := a.failW + float64(p.Health.ConsecutiveFails)

		score := (successRate * 100.0) - (latency / 10.0) - (failCount * 5.0)
		if p.Source.TrustLevel == 0 {
			p.Source.TrustLevel = 50
		}
		score += float64(p.Source.TrustLevel) * trustWeight
		if recentFailureSet["UDP_BLOCKED"] {
			if strings.EqualFold(p.Capabilities.Transport, "tcp") {
				score += 8
			}
			if strings.EqualFold(p.Capabilities.Transport, "udp") {
				score -= 12
			}
		}
		if recentFailureSet["TLS_BLOCKED"] && strings.TrimSpace(p.Credentials.UTLSFingerprint) != "" {
			score += 3
		}

		p.Health.Score = score
		ranked = append(ranked, p)
		scored = append(scored, ScoredProfile{ProfileID: p.ID, Score: math.Round(score*10) / 10})
	}

	// Never allow strict filtering to collapse all candidates; deterministic fallback path must remain.
	if len(ranked) == 0 {
		for _, p := range profiles {
			if !p.Enabled || p.ManualDisabled || p.Health.CooldownUntil > nowUnix {
				continue
			}
			if until := s.Cooldown[p.ID]; until > nowUnix {
				continue
			}
			p.Health.Score = p.Health.SuccessEWMA * scoreScale
			ranked = append(ranked, p)
		}
		scored = scored[:0]
		for _, p := range ranked {
			scored = append(scored, ScoredProfile{ProfileID: p.ID, Score: math.Round(p.Health.Score*10) / 10})
		}
	}

	sort.Slice(ranked, func(i, j int) bool { return ranked[i].Health.Score > ranked[j].Health.Score })
	sort.Slice(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })
	decision.Scores = scored
	decision.RecentFailures = recentFailures
	if len(ranked) == 0 {
		decision.Reason = "no selectable profiles"
		return ranked, decision
	}

	selected := ranked[0]
	reason := fmt.Sprintf("score=%.1f highest success_rate with latency/failure penalties", selected.Health.Score)
	streak := recentSelectionStreak(s.SelectionHistory, selected.ID)
	if streak >= diversityStreakLimit && len(ranked) > 1 {
		diff := ranked[0].Health.Score - ranked[1].Health.Score
		if diff <= diversityScoreDelta {
			selected = ranked[1]
			reason = fmt.Sprintf("diversity guard: %s repeated %d times, switched to close score %.1f", ranked[0].ID, streak, ranked[1].Health.Score)
		}
	}

	decision.Selected = selected.ID
	decision.Reason = reason
	decision.UseOrchestrator = shouldUseOrchestrator(scored, recentFailures)
	return ranked, decision
}

func shouldUseOrchestrator(scores []ScoredProfile, recentFailures []string) bool {
	if len(scores) > 1 {
		if math.Abs(scores[0].Score-scores[1].Score) <= orchestratorCloseScoreDelta {
			return true
		}
	}
	if len(recentFailures) >= 2 {
		return true
	}
	for _, f := range recentFailures {
		if strings.EqualFold(f, "UNKNOWN") {
			return true
		}
	}
	return false
}

func recentSelectionStreak(history []string, id string) int {
	streak := 0
	for i := len(history) - 1; i >= 0; i-- {
		if history[i] != id {
			break
		}
		streak++
	}
	return streak
}
