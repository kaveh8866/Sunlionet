package policy

import (
	"testing"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/profile"
)

func TestAdaptiveLearningImprovesSelection(t *testing.T) {
	state := NewAdaptiveState(80)
	now := time.Now()

	profiles := []profile.Profile{
		{
			ID:      "p1",
			Enabled: true,
			Health:  profile.Health{SuccessEWMA: 0.8, MedianHandshakeMs: 300},
			Capabilities: profile.Capabilities{
				Transport: "udp",
			},
		},
		{
			ID:      "p2",
			Enabled: true,
			Health:  profile.Health{SuccessEWMA: 0.7, MedianHandshakeMs: 200},
			Capabilities: profile.Capabilities{
				Transport: "tcp",
			},
		},
	}

	// p1 starts failing, p2 starts succeeding.
	state.RecordAttempt("p1", AttemptSignal{ConnectOK: false, DNSOK: false, TCPHandshake: false, TLSSuccess: false}, "DNS_FAILURE", now)
	state.RecordAttempt("p2", AttemptSignal{LatencyMS: 180, ConnectOK: true, DNSOK: true, TCPHandshake: true, TLSSuccess: true}, "", now.Add(2*time.Second))

	engine := &Engine{AdaptiveState: state}
	selected, decision, _ := engine.SelectProfile(profiles)
	if selected.ID != "p2" {
		t.Fatalf("expected adaptive learning to prefer p2, got %s (reason: %s)", selected.ID, decision.Reason)
	}
}

func TestAdaptiveCooldownSkipsFailedProfile(t *testing.T) {
	state := NewAdaptiveState(80)
	now := time.Now()

	profiles := []profile.Profile{
		{ID: "p1", Enabled: true, Health: profile.Health{SuccessEWMA: 0.9}},
		{ID: "p2", Enabled: true, Health: profile.Health{SuccessEWMA: 0.6}},
	}

	state.RecordAttempt("p1", AttemptSignal{ConnectOK: false, DNSOK: true, TCPHandshake: false}, "TIMEOUT", now)
	engine := &Engine{AdaptiveState: state}
	selected, _, _ := engine.SelectProfile(profiles)
	if selected.ID != "p2" {
		t.Fatalf("expected cooldown to skip p1 and pick p2, got %s", selected.ID)
	}
}

func TestAdaptiveFallbackPrefersTCPAfterDNSBlock(t *testing.T) {
	state := NewAdaptiveState(80)
	now := time.Now()

	profiles := []profile.Profile{
		{
			ID:      "dns-profile",
			Enabled: true,
			Family:  profile.FamilyDNS,
			Capabilities: profile.Capabilities{
				Transport:         "udp",
				DPIResistanceTags: []string{"dns-dependent"},
			},
			Health: profile.Health{SuccessEWMA: 0.95},
		},
		{
			ID:      "tcp-safe",
			Enabled: true,
			Family:  profile.FamilyReality,
			Capabilities: profile.Capabilities{
				Transport: "tcp",
			},
			Health: profile.Health{SuccessEWMA: 0.70},
		},
	}

	state.RecordAttempt("dns-profile", AttemptSignal{ConnectOK: false, DNSOK: false, TCPHandshake: false}, "DNS_BLOCKED", now)
	engine := &Engine{AdaptiveState: state}
	selected, decision, _ := engine.SelectProfile(profiles)
	if selected.ID != "tcp-safe" {
		t.Fatalf("expected DNS fallback to select tcp-safe, got %s (reason: %s)", selected.ID, decision.Reason)
	}
}
