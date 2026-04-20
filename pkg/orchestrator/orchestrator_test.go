package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/profile"
)

func TestDecodeStrict_RejectsUnknownFields(t *testing.T) {
	var dst JSONRPCResponse
	raw := []byte(`{"jsonrpc":"2.0","id":1,"result":{"action":"hold_current","reason":"ok","confidence":0.5,"extra":1}}`)
	if err := decodeStrict(raw, &dst); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateResponse_RejectsUnknownAction(t *testing.T) {
	req := DecisionRequest{
		Type: "decision_request",
		Profiles: []ProfileSnapshot{
			{ID: "p1", Protocol: "reality"},
		},
	}
	resp := DecisionResponse{
		Action:     AllowedAction("do_thing"),
		Reason:     "x",
		Confidence: 0.5,
	}
	if err := ValidateResponse(req, resp); err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateResponse_RequiresKnownProfileID(t *testing.T) {
	req := DecisionRequest{
		Type: "decision_request",
		Profiles: []ProfileSnapshot{
			{ID: "p1", Protocol: "reality"},
		},
	}
	resp := DecisionResponse{
		Action:     ActionSwitchProfile,
		ProfileID:  "p2",
		Reason:     "x",
		Confidence: 0.5,
	}
	if err := ValidateResponse(req, resp); err == nil {
		t.Fatalf("expected error")
	}
}

func TestEnforceSafety_RejectsUDPWhenNoUDP(t *testing.T) {
	now := time.Now().Unix()
	candidates := []profile.Profile{
		{
			ID:      "p1",
			Family:  profile.FamilyReality,
			Enabled: true,
			Capabilities: profile.Capabilities{
				Transport: "tcp",
			},
		},
		{
			ID:      "p2",
			Family:  profile.FamilyHysteria2,
			Enabled: true,
			Capabilities: profile.Capabilities{
				Transport: "udp",
			},
		},
	}
	req := BuildRequest(now, candidates, DecisionHistory{}, GuardConfig{MaxSwitchRate: 3, NoUDP: true}, NetworkState{})
	resp := DecisionResponse{
		Action:     ActionSwitchProfile,
		ProfileID:  "p2",
		Reason:     "x",
		Confidence: 0.9,
	}
	_, err := EnforceSafety(candidates[0], candidates, req, resp)
	if err == nil {
		t.Fatalf("expected error")
	}
}

type stubClient struct {
	resp DecisionResponse
	err  error
}

func (s *stubClient) Decide(context.Context, DecisionRequest) (DecisionResponse, error) {
	return s.resp, s.err
}
func (s *stubClient) Close() error { return nil }

func TestDecideWithFallback_FallsBackOnClientError(t *testing.T) {
	candidates := []profile.Profile{
		{ID: "p1", Family: profile.FamilyReality, Enabled: true},
		{ID: "p2", Family: profile.FamilyReality, Enabled: true},
	}
	req := BuildRequest(time.Now().Unix(), candidates, DecisionHistory{}, GuardConfig{MaxSwitchRate: 3}, NetworkState{})

	res := DecideWithFallback(
		context.Background(),
		&stubClient{err: context.DeadlineExceeded},
		10*time.Millisecond,
		candidates[0],
		candidates,
		req,
		candidates[0],
		"policy",
		0.2,
	)
	if res.Source != "policy" {
		t.Fatalf("expected policy source, got %s", res.Source)
	}
	if res.SelectedProfile.ID != "p1" {
		t.Fatalf("expected p1, got %s", res.SelectedProfile.ID)
	}
}

func TestDecideWithFallback_SwitchProfile(t *testing.T) {
	candidates := []profile.Profile{
		{ID: "p1", Family: profile.FamilyReality, Enabled: true, Capabilities: profile.Capabilities{Transport: "tcp"}},
		{ID: "p2", Family: profile.FamilyReality, Enabled: true, Capabilities: profile.Capabilities{Transport: "tcp"}},
	}
	req := BuildRequest(time.Now().Unix(), candidates, DecisionHistory{}, GuardConfig{MaxSwitchRate: 3}, NetworkState{})

	res := DecideWithFallback(
		context.Background(),
		&stubClient{resp: DecisionResponse{Action: ActionSwitchProfile, ProfileID: "p2", Reason: "prefer p2", Confidence: 0.8}},
		200*time.Millisecond,
		candidates[0],
		candidates,
		req,
		candidates[0],
		"policy",
		0.3,
	)
	if res.Source != "orchestrator" {
		t.Fatalf("expected orchestrator source, got %s", res.Source)
	}
	if res.SelectedProfile.ID != "p2" {
		t.Fatalf("expected p2, got %s", res.SelectedProfile.ID)
	}
}
