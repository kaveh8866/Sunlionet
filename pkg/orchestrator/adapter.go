package orchestrator

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/profile"
)

type GuardConfig struct {
	MaxSwitchRate int
	NoUDP         bool
}

func BuildRequest(now int64, profiles []profile.Profile, history DecisionHistory, guard GuardConfig, network NetworkState) DecisionRequest {
	if now == 0 {
		now = time.Now().Unix()
	}
	if guard.MaxSwitchRate <= 0 {
		guard.MaxSwitchRate = 3
	}
	req := DecisionRequest{
		Type:      "decision_request",
		Timestamp: now,
		Network:   network,
		History:   history,
		Constraints: Constraints{
			MaxSwitchRate: guard.MaxSwitchRate,
			NoUDP:         guard.NoUDP,
		},
	}
	for _, p := range profiles {
		req.Profiles = append(req.Profiles, ProfileSnapshot{
			ID:          p.ID,
			Protocol:    string(p.Family),
			LastSuccess: p.Health.LastOkAt,
			Failures:    p.Health.ConsecutiveFails,
			Cooldown:    p.Health.CooldownUntil > now,
		})
	}
	return req
}

func ValidateResponse(req DecisionRequest, resp DecisionResponse) error {
	switch resp.Action {
	case ActionSwitchProfile, ActionHoldCurrent, ActionCooldownProfile, ActionImportBundle, ActionActivateMesh:
	default:
		return fmt.Errorf("invalid action %q", resp.Action)
	}

	if math.IsNaN(resp.Confidence) || resp.Confidence < 0 || resp.Confidence > 1 {
		return fmt.Errorf("invalid confidence")
	}
	if strings.TrimSpace(resp.Reason) == "" {
		return fmt.Errorf("missing reason")
	}

	known := map[string]bool{}
	for _, p := range req.Profiles {
		known[p.ID] = true
	}
	if resp.Action == ActionSwitchProfile || resp.Action == ActionCooldownProfile {
		if strings.TrimSpace(resp.ProfileID) == "" {
			return fmt.Errorf("missing profile_id")
		}
		if !known[resp.ProfileID] {
			return fmt.Errorf("unknown profile_id %q", resp.ProfileID)
		}
	}
	for _, id := range resp.Fallback {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("empty fallback id")
		}
		if !known[id] {
			return fmt.Errorf("unknown fallback id %q", id)
		}
	}
	return nil
}

func EnforceSafety(current profile.Profile, candidates []profile.Profile, req DecisionRequest, resp DecisionResponse) (DecisionResponse, error) {
	if err := ValidateResponse(req, resp); err != nil {
		return DecisionResponse{}, err
	}
	if resp.Action == ActionSwitchProfile && req.Constraints.MaxSwitchRate > 0 {
		if len(req.History.RecentSwitches) >= req.Constraints.MaxSwitchRate {
			return DecisionResponse{}, fmt.Errorf("switch rate limit exceeded")
		}
	}

	if resp.Action == ActionSwitchProfile || resp.Action == ActionCooldownProfile {
		target, ok := findProfileByID(candidates, resp.ProfileID)
		if !ok {
			return DecisionResponse{}, fmt.Errorf("unknown target profile")
		}
		if !target.Enabled || target.ManualDisabled {
			return DecisionResponse{}, fmt.Errorf("target profile disabled")
		}
		if req.Constraints.NoUDP && target.Capabilities.Transport == "udp" {
			return DecisionResponse{}, fmt.Errorf("udp prohibited by constraints")
		}
	}

	return resp, nil
}

func findProfileByID(profiles []profile.Profile, id string) (profile.Profile, bool) {
	for _, p := range profiles {
		if p.ID == id {
			return p, true
		}
	}
	return profile.Profile{}, false
}
