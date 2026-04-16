package policy

import (
	"testing"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/detector"
	"github.com/kaveh/shadownet-agent/pkg/profile"
)

func TestRankProfiles_FiltersCooldownAndSortsByScore(t *testing.T) {
	now := time.Now().Unix()
	e := &Engine{}

	profiles := []profile.Profile{
		{
			ID:      "p-cooldown",
			Family:  profile.FamilyReality,
			Enabled: true,
			Health:  profile.Health{SuccessEWMA: 1.0, MedianHandshakeMs: 50, LastOkAt: now, CooldownUntil: now + 60},
		},
		{
			ID:      "p-fast",
			Family:  profile.FamilyReality,
			Enabled: true,
			Health:  profile.Health{SuccessEWMA: 0.85, MedianHandshakeMs: 200, LastOkAt: now - 10, CooldownUntil: 0},
		},
		{
			ID:      "p-slow",
			Family:  profile.FamilyReality,
			Enabled: true,
			Health:  profile.Health{SuccessEWMA: 0.95, MedianHandshakeMs: 1500, LastOkAt: now - 10, CooldownUntil: 0},
		},
		{
			ID:      "p-stale",
			Family:  profile.FamilyReality,
			Enabled: true,
			Health:  profile.Health{SuccessEWMA: 0.98, MedianHandshakeMs: 400, LastOkAt: now - 7200, CooldownUntil: 0},
		},
	}

	ranked := e.RankProfiles(profiles)
	for _, p := range ranked {
		if p.ID == "p-cooldown" {
			t.Fatalf("expected cooldown profile to be filtered out")
		}
	}

	if len(ranked) != 3 {
		t.Fatalf("expected 3 available profiles, got %d", len(ranked))
	}

	if ranked[0].ID != "p-fast" {
		t.Fatalf("expected p-fast to rank first due to latency bonus + recency bonus, got %s", ranked[0].ID)
	}
}

func TestEvaluate_DNSPoisonCritical_EntersEmergencyDNS(t *testing.T) {
	e := &Engine{}

	act := e.Evaluate(
		[]detector.Event{
			{Type: detector.EventDNSPoisonSuspected, Severity: detector.SeverityCritical, Timestamp: time.Now()},
		},
		profile.Profile{ID: "active"},
	)

	if act.Type != ActionEnterEmergencyDNS {
		t.Fatalf("expected %s, got %s", ActionEnterEmergencyDNS, act.Type)
	}
	if act.CooldownSec == 0 {
		t.Fatalf("expected cooldown to be set")
	}
}

func TestEvaluate_UDPBlocked_SwitchesFamilyWhenActiveIsUDP(t *testing.T) {
	e := &Engine{}

	act := e.Evaluate(
		[]detector.Event{
			{Type: detector.EventUDPBlockSuspected, Severity: detector.SeverityHigh, Timestamp: time.Now()},
		},
		profile.Profile{ID: "active", Capabilities: profile.Capabilities{Transport: "udp"}},
	)

	if act.Type != ActionSwitchFamily {
		t.Fatalf("expected %s, got %s", ActionSwitchFamily, act.Type)
	}
}

func TestEvaluate_HandshakeBurst_FastSwitchProfile(t *testing.T) {
	e := &Engine{}

	act := e.Evaluate(
		[]detector.Event{
			{Type: detector.EventHandshakeBurstFailure, Severity: detector.SeverityHigh, Timestamp: time.Now()},
		},
		profile.Profile{ID: "active"},
	)

	if act.Type != ActionSwitchProfile {
		t.Fatalf("expected %s, got %s", ActionSwitchProfile, act.Type)
	}
}

func TestEvaluate_NoEvents_Keep(t *testing.T) {
	e := &Engine{}
	act := e.Evaluate(nil, profile.Profile{ID: "active"})
	if act.Type != ActionKeep {
		t.Fatalf("expected %s, got %s", ActionKeep, act.Type)
	}
}
