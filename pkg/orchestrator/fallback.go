package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/profile"
)

type DecisionResult struct {
	SelectedProfile profile.Profile
	Reason          string
	Confidence      float64
	Source          string
}

func DecideWithFallback(
	ctx context.Context,
	client Client,
	timeout time.Duration,
	current profile.Profile,
	candidates []profile.Profile,
	req DecisionRequest,
	policySelected profile.Profile,
	policyReason string,
	policyConfidence float64,
) DecisionResult {
	if client == nil {
		return DecisionResult{
			SelectedProfile: policySelected,
			Reason:          policyReason,
			Confidence:      policyConfidence,
			Source:          "policy",
		}
	}
	if timeout <= 0 {
		timeout = 1200 * time.Millisecond
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, err := client.Decide(cctx, req)
	if err != nil {
		_ = client.Close()
		return DecisionResult{
			SelectedProfile: policySelected,
			Reason:          fmt.Sprintf("%s (orchestrator_unavailable)", policyReason),
			Confidence:      policyConfidence,
			Source:          "policy",
		}
	}

	resp, err = EnforceSafety(current, candidates, req, resp)
	if err != nil {
		return DecisionResult{
			SelectedProfile: policySelected,
			Reason:          fmt.Sprintf("%s (orchestrator_rejected: %v)", policyReason, err),
			Confidence:      policyConfidence,
			Source:          "policy",
		}
	}

	switch resp.Action {
	case ActionHoldCurrent:
		return DecisionResult{
			SelectedProfile: current,
			Reason:          resp.Reason,
			Confidence:      resp.Confidence,
			Source:          "orchestrator",
		}
	case ActionSwitchProfile:
		p, ok := findProfileByID(candidates, resp.ProfileID)
		if !ok {
			return DecisionResult{
				SelectedProfile: policySelected,
				Reason:          fmt.Sprintf("%s (orchestrator_invalid_target)", policyReason),
				Confidence:      policyConfidence,
				Source:          "policy",
			}
		}
		return DecisionResult{
			SelectedProfile: p,
			Reason:          resp.Reason,
			Confidence:      resp.Confidence,
			Source:          "orchestrator",
		}
	case ActionCooldownProfile:
		return DecisionResult{
			SelectedProfile: policySelected,
			Reason:          resp.Reason,
			Confidence:      resp.Confidence,
			Source:          "orchestrator",
		}
	case ActionImportBundle, ActionActivateMesh:
		return DecisionResult{
			SelectedProfile: policySelected,
			Reason:          fmt.Sprintf("%s (orchestrator_action_not_supported: %s)", policyReason, resp.Action),
			Confidence:      policyConfidence,
			Source:          "policy",
		}
	default:
		return DecisionResult{
			SelectedProfile: policySelected,
			Reason:          fmt.Sprintf("%s (orchestrator_unknown_action)", policyReason),
			Confidence:      policyConfidence,
			Source:          "policy",
		}
	}
}
