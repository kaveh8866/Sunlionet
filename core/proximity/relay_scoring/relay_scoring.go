package relay_scoring

import (
	"sync"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
)

type Stats struct {
	Neighbor identity_manager.NodeID

	SeenCount     uint32
	LastSeenAt    time.Time
	FirstSeenAt   time.Time
	ForwardOK     uint32
	ForwardFail   uint32
	InvalidFrames uint32
	RateLimited   uint32

	windowStart       time.Time
	invalidInWindow   uint32
	limitedInWindow   uint32
	QuarantineUntil   time.Time
	lastPenaltyExtend time.Time
}

type Scorer struct {
	mu    sync.RWMutex
	stats map[identity_manager.NodeID]*Stats
}

func New() *Scorer {
	return &Scorer{stats: make(map[identity_manager.NodeID]*Stats)}
}

func (s *Scorer) OnSeen(id identity_manager.NodeID, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stats[id]
	if st == nil {
		st = &Stats{Neighbor: id, FirstSeenAt: now}
		s.stats[id] = st
	}
	st.SeenCount++
	st.LastSeenAt = now
}

func (s *Scorer) OnForwardResult(id identity_manager.NodeID, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stats[id]
	if st == nil {
		st = &Stats{Neighbor: id}
		s.stats[id] = st
	}
	if ok {
		st.ForwardOK++
	} else {
		st.ForwardFail++
	}
}

func (s *Scorer) OnInvalid(id identity_manager.NodeID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stats[id]
	if st == nil {
		st = &Stats{Neighbor: id}
		s.stats[id] = st
	}
	st.InvalidFrames++
}

func (s *Scorer) OnInvalidAt(id identity_manager.NodeID, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stats[id]
	if st == nil {
		st = &Stats{Neighbor: id}
		s.stats[id] = st
	}
	st.InvalidFrames++
	s.maybeQuarantineLocked(st, now, true)
}

func (s *Scorer) OnRateLimited(id identity_manager.NodeID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stats[id]
	if st == nil {
		st = &Stats{Neighbor: id}
		s.stats[id] = st
	}
	st.RateLimited++
}

func (s *Scorer) OnRateLimitedAt(id identity_manager.NodeID, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stats[id]
	if st == nil {
		st = &Stats{Neighbor: id}
		s.stats[id] = st
	}
	st.RateLimited++
	s.maybeQuarantineLocked(st, now, false)
}

func (s *Scorer) AllowFrom(id identity_manager.NodeID, now time.Time) bool {
	s.mu.RLock()
	st := s.stats[id]
	s.mu.RUnlock()
	if st == nil {
		return true
	}
	if now.Before(st.QuarantineUntil) {
		return false
	}
	return true
}

func (s *Scorer) maybeQuarantineLocked(st *Stats, now time.Time, invalidEvent bool) {
	const window = 2 * time.Second
	if st.windowStart.IsZero() || now.Sub(st.windowStart) > window {
		st.windowStart = now
		st.invalidInWindow = 0
		st.limitedInWindow = 0
	}
	if invalidEvent {
		st.invalidInWindow++
	} else {
		st.limitedInWindow++
	}

	dur := time.Duration(0)
	if st.invalidInWindow >= 6 {
		dur = 8 * time.Second
	} else if st.invalidInWindow >= 3 {
		dur = 2 * time.Second
	} else if st.limitedInWindow >= 120 && st.invalidInWindow >= 1 {
		dur = 2 * time.Second
	}
	if dur == 0 {
		return
	}
	until := now.Add(dur)
	if until.After(st.QuarantineUntil) {
		if !st.lastPenaltyExtend.IsZero() && now.Sub(st.lastPenaltyExtend) < 1*time.Second {
			return
		}
		st.QuarantineUntil = until
		st.lastPenaltyExtend = now
	}
}

func (s *Scorer) Snapshot(id identity_manager.NodeID) (Stats, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := s.stats[id]
	if st == nil {
		return Stats{}, false
	}
	cp := *st
	return cp, true
}

func (s *Scorer) Score(id identity_manager.NodeID, now time.Time) float64 {
	s.mu.RLock()
	st := s.stats[id]
	s.mu.RUnlock()
	if st == nil {
		return 0.9
	}

	fail := float64(st.ForwardFail)
	ok := float64(st.ForwardOK)
	invalid := float64(st.InvalidFrames)
	limited := float64(st.RateLimited)

	reliability := 1.0
	if ok+fail > 0 {
		reliability = (ok + 1.0) / (ok + fail + 2.0)
	}
	penalty := 1.0 / (1.0 + 0.8*invalid + 0.9*limited)

	recency := 1.0
	if !st.LastSeenAt.IsZero() {
		age := now.Sub(st.LastSeenAt)
		if age > 0 {
			recency = 1.0 / (1.0 + age.Seconds()/60.0)
		}
	}

	score := 0.9 * reliability * penalty * recency
	if score < 0.0 {
		return 0.0
	}
	if score > 1.0 {
		return 1.0
	}
	return score
}
