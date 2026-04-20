package ledgersync

import (
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/ledger"
	"github.com/kaveh/sunlionet-agent/pkg/mesh"
)

func TestSecurityLayer_QuarantineOnRepeatedRejected(t *testing.T) {
	pol := DefaultSecurityPolicy()
	sec := NewSecurityLayer(pol)
	now := time.Now()
	peerID := "peer-mal"

	for i := 0; i < 6; i++ {
		sec.ObserveApplyReport(peerID, PeerRoleNormal, ledger.ApplyReport{Rejected: 3}, now.Add(time.Duration(i)*time.Second))
	}
	if tier := sec.TrustTier(peerID, now.Add(7*time.Second)); tier != TrustBlocked {
		t.Fatalf("expected blocked tier, got %q", tier)
	}
	if d := sec.ObserveInbound(peerID, PeerRoleNormal, "events", 1024, now.Add(8*time.Second)); d != DecisionReject {
		t.Fatalf("expected reject decision, got %q", d)
	}
}

func TestSecurityLayer_InventoryClusterDefer(t *testing.T) {
	pol := DefaultSecurityPolicy()
	pol.InventoryClusterThreshold = 3
	sec := NewSecurityLayer(pol)
	now := time.Now()

	inv := ledger.InventoryMessage{
		SchemaVersion: ledger.SyncSchemaV1,
		Heads:         []string{"h1", "h2"},
		Have:          []string{"e1", "e2"},
	}
	if d := sec.ObserveInventory("p1", PeerRoleNormal, inv, now); d != DecisionAccept {
		t.Fatalf("p1 expected accept, got %q", d)
	}
	if d := sec.ObserveInventory("p2", PeerRoleNormal, inv, now.Add(time.Second)); d != DecisionAccept {
		t.Fatalf("p2 expected accept, got %q", d)
	}
	if d := sec.ObserveInventory("p3", PeerRoleNormal, inv, now.Add(2*time.Second)); d != DecisionDefer {
		t.Fatalf("p3 expected defer on cluster, got %q", d)
	}
}

func TestSecurityLayer_WarmupAndTrustUpgrade(t *testing.T) {
	sec := NewSecurityLayer(DefaultSecurityPolicy())
	now := time.Now()
	peerID := "peer-ok"

	if tier := sec.TrustTier(peerID, now); tier != TrustLow {
		t.Fatalf("expected low tier, got %q", tier)
	}

	for i := 0; i < 4; i++ {
		sec.ObserveApplyReport(peerID, PeerRoleNormal, ledger.ApplyReport{Applied: 4}, now.Add(time.Duration(i)*time.Second))
	}
	if tier := sec.TrustTier(peerID, now.Add(5*time.Second)); tier == TrustLow {
		t.Fatalf("expected tier to upgrade from low")
	}
}

func TestSecurityLayer_WantOversizeDeferOrReject(t *testing.T) {
	p := DefaultSecurityPolicy()
	p.MaxWantsPerRound = 4
	sec := NewSecurityLayer(p)
	now := time.Now()

	want := ledger.WantMessage{SchemaVersion: ledger.SyncSchemaV1}
	for i := 0; i < 5; i++ {
		want.Want = append(want.Want, "id")
	}
	if d := sec.ObserveWant("p1", PeerRoleNormal, want, now); d != DecisionDefer {
		t.Fatalf("expected defer, got %q", d)
	}
	for i := 0; i < 10; i++ {
		want.Want = append(want.Want, "id")
	}
	if d := sec.ObserveWant("p1", PeerRoleNormal, want, now.Add(time.Second)); d != DecisionReject {
		t.Fatalf("expected reject, got %q", d)
	}
}

func TestSelectPeers_EnforcesRelayAndAgentCaps(t *testing.T) {
	m := &noopMesh{}
	c, err := mesh.NewCrypto()
	if err != nil {
		t.Fatalf("NewCrypto: %v", err)
	}
	l := ledger.New()
	opts := Options{
		MaxPeersPerRound: 5,
		SecurityPolicy: SecurityPolicy{
			MaxRelaysPerRound: 1,
			MaxAgentsPerRound: 1,
		},
	}
	svc, err := New(m, c, l, nil, nil, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	now := time.Now()
	cands := []Peer{
		{ID: "n1", Role: PeerRoleNormal},
		{ID: "r1", Role: PeerRoleRelay},
		{ID: "r2", Role: PeerRoleRelay},
		{ID: "a1", Role: PeerRoleAgent},
		{ID: "a2", Role: PeerRoleAgent},
	}
	out := svc.SelectPeers(cands, now)
	relays := 0
	agents := 0
	for i := range out {
		switch out[i].Role {
		case PeerRoleRelay:
			relays++
		case PeerRoleAgent:
			agents++
		}
	}
	if relays > 1 {
		t.Fatalf("expected max 1 relay, got %d", relays)
	}
	if agents > 1 {
		t.Fatalf("expected max 1 agent, got %d", agents)
	}
}
