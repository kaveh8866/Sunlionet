package ledgersync

import (
	"crypto/sha256"
	"encoding/base64"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/ledger"
)

type PolicyDecision string

const (
	DecisionAccept PolicyDecision = "accept"
	DecisionReject PolicyDecision = "reject"
	DecisionDefer  PolicyDecision = "defer"
)

type TrustTier string

const (
	TrustLow     TrustTier = "low"
	TrustMedium  TrustTier = "medium"
	TrustHigh    TrustTier = "high"
	TrustBlocked TrustTier = "blocked"
)

type SecurityPolicy struct {
	InitialScore int
	MinScore     int
	MediumScore  int
	HighScore    int

	QuarantineBase time.Duration
	QuarantineMax  time.Duration

	SybilWarmupApplied int
	MaxRelaysPerRound  int
	MaxAgentsPerRound  int

	RelayMaxInboundBytes int
	AgentMaxInboundBytes int

	InventoryEchoThreshold    int
	InventoryClusterThreshold int
	InventoryWindow           time.Duration
}

func DefaultSecurityPolicy() SecurityPolicy {
	return SecurityPolicy{
		InitialScore: 10,
		MinScore:     -60,
		MediumScore:  35,
		HighScore:    75,

		QuarantineBase: 30 * time.Second,
		QuarantineMax:  10 * time.Minute,

		SybilWarmupApplied: 3,
		MaxRelaysPerRound:  1,
		MaxAgentsPerRound:  1,

		RelayMaxInboundBytes: 120 * 1024,
		AgentMaxInboundBytes: 64 * 1024,

		InventoryEchoThreshold:    4,
		InventoryClusterThreshold: 3,
		InventoryWindow:           3 * time.Minute,
	}
}

func (p SecurityPolicy) normalize() SecurityPolicy {
	out := p
	def := DefaultSecurityPolicy()
	if out.InitialScore == 0 {
		out.InitialScore = def.InitialScore
	}
	if out.MinScore == 0 {
		out.MinScore = def.MinScore
	}
	if out.MediumScore <= out.MinScore {
		out.MediumScore = def.MediumScore
	}
	if out.HighScore <= out.MediumScore {
		out.HighScore = def.HighScore
	}
	if out.QuarantineBase <= 0 {
		out.QuarantineBase = def.QuarantineBase
	}
	if out.QuarantineMax <= 0 {
		out.QuarantineMax = def.QuarantineMax
	}
	if out.QuarantineMax < out.QuarantineBase {
		out.QuarantineMax = out.QuarantineBase
	}
	if out.SybilWarmupApplied <= 0 {
		out.SybilWarmupApplied = def.SybilWarmupApplied
	}
	if out.MaxRelaysPerRound <= 0 {
		out.MaxRelaysPerRound = def.MaxRelaysPerRound
	}
	if out.MaxAgentsPerRound <= 0 {
		out.MaxAgentsPerRound = def.MaxAgentsPerRound
	}
	if out.RelayMaxInboundBytes <= 0 {
		out.RelayMaxInboundBytes = def.RelayMaxInboundBytes
	}
	if out.AgentMaxInboundBytes <= 0 {
		out.AgentMaxInboundBytes = def.AgentMaxInboundBytes
	}
	if out.InventoryEchoThreshold <= 0 {
		out.InventoryEchoThreshold = def.InventoryEchoThreshold
	}
	if out.InventoryClusterThreshold <= 0 {
		out.InventoryClusterThreshold = def.InventoryClusterThreshold
	}
	if out.InventoryWindow <= 0 {
		out.InventoryWindow = def.InventoryWindow
	}
	return out
}

type peerTrust struct {
	score int
	role  PeerRole

	firstSeen time.Time
	lastSeen  time.Time

	appliedTotal  int
	rejectedTotal int

	quarantineUntil time.Time
	penalty         time.Duration

	lastInvDigest string
	invStreak     int
}

type SecurityLayer struct {
	mu      sync.Mutex
	policy  SecurityPolicy
	peers   map[string]*peerTrust
	invSeen map[string]map[string]time.Time
}

func NewSecurityLayer(policy SecurityPolicy) *SecurityLayer {
	return &SecurityLayer{
		policy:  policy.normalize(),
		peers:   map[string]*peerTrust{},
		invSeen: map[string]map[string]time.Time{},
	}
}

func (s *SecurityLayer) Policy() SecurityPolicy {
	if s == nil {
		return DefaultSecurityPolicy()
	}
	return s.policy
}

func (s *SecurityLayer) ensurePeerLocked(peerID string, role PeerRole, now time.Time) *peerTrust {
	ps := s.peers[peerID]
	if ps == nil {
		ps = &peerTrust{
			score:     s.policy.InitialScore,
			firstSeen: now,
		}
		s.peers[peerID] = ps
	}
	ps.lastSeen = now
	if role != "" {
		ps.role = role
	}
	return ps
}

func (s *SecurityLayer) SetPeerRole(peerID string, role PeerRole, now time.Time) {
	if s == nil || strings.TrimSpace(peerID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := s.ensurePeerLocked(peerID, role, now)
	ps.role = role
}

func (s *SecurityLayer) TrustTier(peerID string, now time.Time) TrustTier {
	if s == nil || strings.TrimSpace(peerID) == "" {
		return TrustLow
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := s.ensurePeerLocked(peerID, "", now)
	if !ps.quarantineUntil.IsZero() && now.Before(ps.quarantineUntil) {
		return TrustBlocked
	}
	if ps.score <= s.policy.MinScore {
		return TrustBlocked
	}
	if ps.score >= s.policy.HighScore {
		return TrustHigh
	}
	if ps.score >= s.policy.MediumScore {
		return TrustMedium
	}
	return TrustLow
}

func (s *SecurityLayer) isWarmLocked(ps *peerTrust) bool {
	return ps.appliedTotal >= s.policy.SybilWarmupApplied
}

func (s *SecurityLayer) ObserveInbound(peerID string, role PeerRole, msgType string, size int, now time.Time) PolicyDecision {
	if s == nil || strings.TrimSpace(peerID) == "" {
		return DecisionAccept
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := s.ensurePeerLocked(peerID, role, now)
	if !ps.quarantineUntil.IsZero() && now.Before(ps.quarantineUntil) {
		return DecisionReject
	}
	if ps.score <= s.policy.MinScore {
		return DecisionReject
	}
	if ps.role == PeerRoleRelay && size > s.policy.RelayMaxInboundBytes {
		s.penalizeLocked(ps, 12, now)
		return DecisionReject
	}
	if ps.role == PeerRoleAgent && size > s.policy.AgentMaxInboundBytes {
		s.penalizeLocked(ps, 12, now)
		return DecisionReject
	}
	if msgType == "events" && !s.isWarmLocked(ps) && size > 96*1024 {
		return DecisionDefer
	}
	return DecisionAccept
}

func (s *SecurityLayer) ObserveInventory(peerID string, role PeerRole, inv ledger.InventoryMessage, now time.Time) PolicyDecision {
	if s == nil || strings.TrimSpace(peerID) == "" {
		return DecisionAccept
	}
	digest := inventoryDigest(inv)

	s.mu.Lock()
	defer s.mu.Unlock()
	ps := s.ensurePeerLocked(peerID, role, now)

	if digest != "" {
		if ps.lastInvDigest == digest {
			ps.invStreak++
		} else {
			ps.lastInvDigest = digest
			ps.invStreak = 1
		}
		if ps.invStreak >= s.policy.InventoryEchoThreshold {
			s.penalizeLocked(ps, 4, now)
			if ps.invStreak >= s.policy.InventoryEchoThreshold+2 {
				return DecisionDefer
			}
		}
	}

	cutoff := now.Add(-s.policy.InventoryWindow)
	for d, peers := range s.invSeen {
		for id, ts := range peers {
			if ts.Before(cutoff) {
				delete(peers, id)
			}
		}
		if len(peers) == 0 {
			delete(s.invSeen, d)
		}
	}
	if digest != "" {
		peers := s.invSeen[digest]
		if peers == nil {
			peers = map[string]time.Time{}
			s.invSeen[digest] = peers
		}
		peers[peerID] = now
		if len(peers) >= s.policy.InventoryClusterThreshold {
			s.penalizeLocked(ps, 3, now)
			return DecisionDefer
		}
	}
	return DecisionAccept
}

func (s *SecurityLayer) ObserveApplyReport(peerID string, role PeerRole, rep ledger.ApplyReport, now time.Time) {
	if s == nil || strings.TrimSpace(peerID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := s.ensurePeerLocked(peerID, role, now)

	if rep.Applied > 0 {
		ps.appliedTotal += rep.Applied
		boost := rep.Applied * 2
		if boost > 8 {
			boost = 8
		}
		ps.score += boost
	}
	if rep.Dupe > 8 {
		ps.score -= 2
	}
	if rep.Rejected > 0 {
		ps.rejectedTotal += rep.Rejected
		pen := rep.Rejected * 8
		if pen > 30 {
			pen = 30
		}
		s.penalizeLocked(ps, pen, now)
	}
	if ps.score > 120 {
		ps.score = 120
	}
}

func (s *SecurityLayer) penalizeLocked(ps *peerTrust, amount int, now time.Time) {
	if amount <= 0 {
		return
	}
	ps.score -= amount
	if ps.penalty < s.policy.QuarantineBase {
		ps.penalty = s.policy.QuarantineBase
	} else {
		ps.penalty *= 2
		if ps.penalty > s.policy.QuarantineMax {
			ps.penalty = s.policy.QuarantineMax
		}
	}
	if ps.score <= s.policy.MinScore {
		ps.quarantineUntil = now.Add(ps.penalty)
	}
}

func (s *SecurityLayer) PeerPriority(peerID string, role PeerRole, ageSeconds int64, now time.Time) int64 {
	if s == nil || strings.TrimSpace(peerID) == "" {
		return ageSeconds
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := s.ensurePeerLocked(peerID, role, now)
	if !ps.quarantineUntil.IsZero() && now.Before(ps.quarantineUntil) {
		return -1
	}
	score := int64(ps.score * 60)
	if !s.isWarmLocked(ps) {
		score -= 240
	}
	switch ps.role {
	case PeerRoleRelay:
		score -= 180
	case PeerRoleAgent:
		score -= 120
	}
	return ageSeconds + score
}

func (s *SecurityLayer) WitnessWeight(peerID string, base int, now time.Time) int {
	if s == nil || strings.TrimSpace(peerID) == "" {
		return 0
	}
	if base <= 0 {
		base = 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := s.ensurePeerLocked(peerID, "", now)
	if !ps.quarantineUntil.IsZero() && now.Before(ps.quarantineUntil) {
		return 0
	}
	if ps.score <= s.policy.MinScore {
		return 0
	}
	if ps.score >= s.policy.HighScore {
		return base
	}
	if ps.score >= s.policy.MediumScore {
		return 1
	}
	return 0
}

func inventoryDigest(inv ledger.InventoryMessage) string {
	heads := append([]string(nil), inv.Heads...)
	have := append([]string(nil), inv.Have...)
	sort.Strings(heads)
	sort.Strings(have)
	var b strings.Builder
	for i := range heads {
		h := strings.TrimSpace(heads[i])
		if h == "" {
			continue
		}
		b.WriteString("h:")
		b.WriteString(h)
		b.WriteByte('\n')
	}
	for i := range have {
		h := strings.TrimSpace(have[i])
		if h == "" {
			continue
		}
		b.WriteString("v:")
		b.WriteString(h)
		b.WriteByte('\n')
	}
	if b.Len() == 0 {
		return ""
	}
	sum := sha256.Sum256([]byte(b.String()))
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}
