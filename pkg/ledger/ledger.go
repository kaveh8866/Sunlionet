package ledger

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Snapshot struct {
	SchemaVersion int              `json:"schema_version"`
	UpdatedAt     int64            `json:"updated_at"`
	Events        []Event          `json:"events"`
	Heads         []string         `json:"heads"`
	Anchors       []SnapshotAnchor `json:"anchors,omitempty"`
}

type SnapshotAnchor struct {
	SchemaVersion   int    `json:"schema_version"`
	CreatedAt       int64  `json:"created_at"`
	PrunedCount     int    `json:"pruned_count"`
	FirstCreatedAt  int64  `json:"first_created_at,omitempty"`
	LastCreatedAt   int64  `json:"last_created_at,omitempty"`
	RootHashB64     string `json:"root_hash_b64url"`
	PrevAnchorHashB string `json:"prev_anchor_hash_b64url,omitempty"`
	AnchorHashB64   string `json:"anchor_hash_b64url"`
}

type Ledger struct {
	mu        sync.RWMutex
	events    map[string]Event
	heads     map[string]struct{}
	authorSeq map[string]map[uint64]string

	missingRefs      map[string]int
	missingRefWeight map[string]int
	compromised      map[string]struct{}

	attestations  map[string]map[string]map[string]struct{}
	checkpoints   map[string]map[string]map[string]struct{}
	introductions map[string]map[string]map[string]struct{}

	anchors []SnapshotAnchor
}

func New() *Ledger {
	return &Ledger{
		events:           map[string]Event{},
		heads:            map[string]struct{}{},
		authorSeq:        map[string]map[uint64]string{},
		missingRefs:      map[string]int{},
		missingRefWeight: map[string]int{},
		compromised:      map[string]struct{}{},
		attestations:     map[string]map[string]map[string]struct{}{},
		checkpoints:      map[string]map[string]map[string]struct{}{},
		introductions:    map[string]map[string]map[string]struct{}{},
	}
}

func affectsHeads(kind string) bool {
	switch kind {
	case KindWitnessAttest,
		KindWitnessCheckpoint,
		KindIdentityIntroduce,
		KindMisbehaviorEquivoc,
		KindMisbehaviorReplay,
		KindMisbehaviorSybil,
		KindSyncSummary:
		return false
	default:
		return true
	}
}

func isCriticalKind(kind string) bool {
	switch kind {
	case KindIdentityIntroduce,
		KindIdentityRotate,
		KindIdentityRevoke,
		KindMisbehaviorEquivoc,
		KindMisbehaviorReplay,
		KindMisbehaviorSybil,
		KindWitnessAttest,
		KindWitnessCheckpoint,
		KindGroupCreate,
		KindGroupJoin,
		KindGroupMembership:
		return true
	default:
		return false
	}
}

func syncPriorityWeight(kind string) int {
	switch kind {
	case KindIdentityIntroduce,
		KindIdentityRotate,
		KindIdentityRevoke,
		KindMisbehaviorEquivoc,
		KindMisbehaviorReplay,
		KindMisbehaviorSybil,
		KindWitnessAttest,
		KindWitnessCheckpoint,
		KindGroupMembership,
		KindAgentAction,
		KindSyncSummary,
		KindLedgerEvent:
		return 3
	case KindChatMessage,
		KindGroupCreate,
		KindGroupJoin:
		return 2
	default:
		return 1
	}
}

func NewFromSnapshot(s Snapshot) (*Ledger, error) {
	if s.SchemaVersion != 0 && s.SchemaVersion != SchemaV1 {
		return nil, fmt.Errorf("ledger: unsupported snapshot schema: %d", s.SchemaVersion)
	}
	l := New()
	for i := range s.Events {
		if err := l.Add(s.Events[i]); err != nil {
			return nil, err
		}
	}
	if len(s.Heads) > 0 {
		l.heads = map[string]struct{}{}
		for _, h := range s.Heads {
			if h == "" {
				continue
			}
			if _, ok := l.events[h]; ok {
				l.heads[h] = struct{}{}
			}
		}
		if len(l.heads) == 0 {
			l.recomputeHeadsLocked()
		}
	}
	if len(s.Anchors) > 0 {
		l.anchors = append([]SnapshotAnchor(nil), s.Anchors...)
	}
	return l, nil
}

func (l *Ledger) Add(ev Event) error {
	if l == nil {
		return errors.New("ledger: ledger is nil")
	}
	if err := ev.Validate(); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.events == nil {
		l.events = map[string]Event{}
	}
	if l.heads == nil {
		l.heads = map[string]struct{}{}
	}
	if l.authorSeq == nil {
		l.authorSeq = map[string]map[uint64]string{}
	}
	if _, ok := l.events[ev.ID]; ok {
		return nil
	}

	seqs, ok := l.authorSeq[ev.Author]
	if !ok {
		seqs = map[uint64]string{}
		l.authorSeq[ev.Author] = seqs
	}
	if existing, ok := seqs[ev.Seq]; ok && existing != ev.ID {
		return errors.New("ledger: author seq conflict")
	}
	if ev.Seq > 1 {
		if prev, ok := l.events[ev.Prev]; ok {
			if prev.Author != ev.Author {
				return errors.New("ledger: prev author mismatch")
			}
			if prev.Seq+1 != ev.Seq {
				return errors.New("ledger: prev seq mismatch")
			}
		}
	}

	l.events[ev.ID] = ev
	seqs[ev.Seq] = ev.ID

	delete(l.missingRefs, ev.ID)
	l.indexLocked(ev)
	l.trackMissingRefsLocked(ev, false)

	if affectsHeads(ev.Kind) {
		if ev.Prev != "" {
			delete(l.heads, ev.Prev)
		}
		for _, p := range ev.Parents {
			delete(l.heads, p)
		}
		l.heads[ev.ID] = struct{}{}
	}
	return nil
}

func (l *Ledger) CreateSnapshot() Snapshot {
	return l.Snapshot()
}

func LoadFromSnapshot(s Snapshot) (*Ledger, error) {
	return NewFromSnapshot(s)
}

type EquivocationEvidence struct {
	OffenderAuthor string
	OffenderKeyB64 string
	Seq            uint64
	EventA         string
	EventB         string
}

type EquivocationError struct {
	Evidence EquivocationEvidence
}

func (e *EquivocationError) Error() string {
	return "ledger: equivocation detected"
}

func (l *Ledger) Apply(ev Event, pol *Policy, observer *Observer) (Status, error) {
	return l.ApplyWithContext(ev, "", pol, observer)
}

func (l *Ledger) ApplyWithContext(ev Event, context string, pol *Policy, observer *Observer) (Status, error) {
	if l == nil {
		return "", errors.New("ledger: ledger is nil")
	}
	p := DefaultPolicy()
	if pol != nil {
		p = *pol
	}
	if err := validateWithPolicy(ev, p); err != nil {
		if err == ErrEventTooOld && observer != nil && strings.TrimSpace(context) != "" {
			l.mu.Lock()
			defer l.mu.Unlock()
			if l.events == nil {
				l.events = map[string]Event{}
			}
			if l.heads == nil {
				l.heads = map[string]struct{}{}
			}
			if l.authorSeq == nil {
				l.authorSeq = map[string]map[uint64]string{}
			}
			if l.missingRefs == nil {
				l.missingRefs = map[string]int{}
			}
			if l.compromised == nil {
				l.compromised = map[string]struct{}{}
			}
			if l.attestations == nil {
				l.attestations = map[string]map[string]map[string]struct{}{}
			}
			if l.checkpoints == nil {
				l.checkpoints = map[string]map[string]map[string]struct{}{}
			}
			if l.introductions == nil {
				l.introductions = map[string]map[string]map[string]struct{}{}
			}
			if observer.Valid() == nil {
				seq, prev, parents := l.nextSeqLocked(observer.Author)
				mev, berr := buildReplayMisbehavior(ev, context, *observer, seq, prev, parents)
				if berr == nil && validateWithPolicy(mev, p) == nil {
					_ = l.addLocked(mev, false)
				}
			}
		}
		return "", err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.events == nil {
		l.events = map[string]Event{}
	}
	if l.heads == nil {
		l.heads = map[string]struct{}{}
	}
	if l.authorSeq == nil {
		l.authorSeq = map[string]map[uint64]string{}
	}
	if l.missingRefs == nil {
		l.missingRefs = map[string]int{}
	}
	if l.compromised == nil {
		l.compromised = map[string]struct{}{}
	}
	if l.attestations == nil {
		l.attestations = map[string]map[string]map[string]struct{}{}
	}
	if l.checkpoints == nil {
		l.checkpoints = map[string]map[string]map[string]struct{}{}
	}
	if l.introductions == nil {
		l.introductions = map[string]map[string]map[string]struct{}{}
	}

	if _, ok := l.events[ev.ID]; ok {
		return StatusAccepted, nil
	}

	seqs, ok := l.authorSeq[ev.Author]
	if !ok {
		seqs = map[uint64]string{}
		l.authorSeq[ev.Author] = seqs
	}
	if existing, ok := seqs[ev.Seq]; ok && existing != ev.ID {
		existingEv, _ := l.events[existing]
		l.compromised[existingEv.AuthorKeyB64] = struct{}{}
		l.compromised[ev.AuthorKeyB64] = struct{}{}

		evA := existingEv.ID
		evB := ev.ID
		if evA > evB {
			evA, evB = evB, evA
		}
		eq := EquivocationEvidence{
			OffenderAuthor: ev.Author,
			OffenderKeyB64: ev.AuthorKeyB64,
			Seq:            ev.Seq,
			EventA:         evA,
			EventB:         evB,
		}
		if observer != nil {
			if err := observer.Valid(); err == nil {
				seq, prev, parents := l.nextSeqLocked(observer.Author)
				mev, err := buildEquivocationMisbehavior(eq, *observer, seq, prev, parents)
				if err == nil {
					if validateWithPolicy(mev, p) == nil {
						_ = l.addLocked(mev, false)
					}
				}
			}
		}
		return "", &EquivocationError{Evidence: eq}
	}

	if ev.Seq > 1 && p.RequireKnownPrev {
		if _, ok := l.events[ev.Prev]; !ok {
			return "", errors.New("ledger: prev not found")
		}
	}
	if ev.Seq > 1 {
		if prev, ok := l.events[ev.Prev]; ok {
			if prev.Author != ev.Author {
				return "", errors.New("ledger: prev author mismatch")
			}
			if prev.Seq+1 != ev.Seq {
				return "", errors.New("ledger: prev seq mismatch")
			}
		}
	}

	switch ev.Kind {
	case KindWitnessAttest:
		var pl AttestationPayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return "", err
		}
		if _, ok := l.events[pl.EventID]; !ok {
			return "", errors.New("ledger: attest event not found")
		}
	}

	if err := l.addLocked(ev, p.RequireKnownPrev); err != nil {
		return "", err
	}
	return StatusAccepted, nil
}

func (l *Ledger) ApplyVerified(ev Event, pol *Policy, observer *Observer) (Status, error) {
	return l.ApplyVerifiedWithContext(ev, "", pol, observer)
}

func (l *Ledger) ApplyVerifiedWithContext(ev Event, context string, pol *Policy, observer *Observer) (Status, error) {
	if l == nil {
		return "", errors.New("ledger: ledger is nil")
	}
	p := DefaultPolicy()
	if pol != nil {
		p = *pol
	}
	hash, err := validateWithPolicyNoCrypto(ev, p)
	if err != nil {
		if err == ErrEventTooOld && observer != nil && strings.TrimSpace(context) != "" {
			l.mu.Lock()
			defer l.mu.Unlock()
			if l.events == nil {
				l.events = map[string]Event{}
			}
			if l.heads == nil {
				l.heads = map[string]struct{}{}
			}
			if l.authorSeq == nil {
				l.authorSeq = map[string]map[uint64]string{}
			}
			if l.missingRefs == nil {
				l.missingRefs = map[string]int{}
			}
			if l.compromised == nil {
				l.compromised = map[string]struct{}{}
			}
			if l.attestations == nil {
				l.attestations = map[string]map[string]map[string]struct{}{}
			}
			if l.checkpoints == nil {
				l.checkpoints = map[string]map[string]map[string]struct{}{}
			}
			if l.introductions == nil {
				l.introductions = map[string]map[string]map[string]struct{}{}
			}
			if observer.Valid() == nil {
				seq, prev, parents := l.nextSeqLocked(observer.Author)
				mev, berr := buildReplayMisbehavior(ev, context, *observer, seq, prev, parents)
				if berr == nil && validateWithPolicy(mev, p) == nil {
					_ = l.addLocked(mev, false)
				}
			}
		}
		return "", err
	}
	wantID := base64.RawURLEncoding.EncodeToString(hash[:])
	if ev.ID != wantID {
		return "", errors.New("ledger: id mismatch")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.events == nil {
		l.events = map[string]Event{}
	}
	if l.heads == nil {
		l.heads = map[string]struct{}{}
	}
	if l.authorSeq == nil {
		l.authorSeq = map[string]map[uint64]string{}
	}
	if l.missingRefs == nil {
		l.missingRefs = map[string]int{}
	}
	if l.compromised == nil {
		l.compromised = map[string]struct{}{}
	}
	if l.attestations == nil {
		l.attestations = map[string]map[string]map[string]struct{}{}
	}
	if l.checkpoints == nil {
		l.checkpoints = map[string]map[string]map[string]struct{}{}
	}
	if l.introductions == nil {
		l.introductions = map[string]map[string]map[string]struct{}{}
	}

	if _, ok := l.events[ev.ID]; ok {
		return StatusAccepted, nil
	}

	seqs, ok := l.authorSeq[ev.Author]
	if !ok {
		seqs = map[uint64]string{}
		l.authorSeq[ev.Author] = seqs
	}
	if existing, ok := seqs[ev.Seq]; ok && existing != ev.ID {
		existingEv, _ := l.events[existing]
		l.compromised[existingEv.AuthorKeyB64] = struct{}{}
		l.compromised[ev.AuthorKeyB64] = struct{}{}

		evA := existingEv.ID
		evB := ev.ID
		if evA > evB {
			evA, evB = evB, evA
		}
		eq := EquivocationEvidence{
			OffenderAuthor: ev.Author,
			OffenderKeyB64: ev.AuthorKeyB64,
			Seq:            ev.Seq,
			EventA:         evA,
			EventB:         evB,
		}
		if observer != nil {
			if err := observer.Valid(); err == nil {
				seq, prev, parents := l.nextSeqLocked(observer.Author)
				mev, err := buildEquivocationMisbehavior(eq, *observer, seq, prev, parents)
				if err == nil {
					if validateWithPolicy(mev, p) == nil {
						_ = l.addLocked(mev, false)
					}
				}
			}
		}
		return "", &EquivocationError{Evidence: eq}
	}

	if ev.Seq > 1 && p.RequireKnownPrev {
		if _, ok := l.events[ev.Prev]; !ok {
			return "", errors.New("ledger: prev not found")
		}
	}
	if ev.Seq > 1 {
		if prev, ok := l.events[ev.Prev]; ok {
			if prev.Author != ev.Author {
				return "", errors.New("ledger: prev author mismatch")
			}
			if prev.Seq+1 != ev.Seq {
				return "", errors.New("ledger: prev seq mismatch")
			}
		}
	}

	switch ev.Kind {
	case KindWitnessAttest:
		var pl AttestationPayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return "", err
		}
		if _, ok := l.events[pl.EventID]; !ok {
			return "", errors.New("ledger: attest event not found")
		}
	}

	if err := l.addLocked(ev, p.RequireKnownPrev); err != nil {
		return "", err
	}
	return StatusAccepted, nil
}

func buildEquivocationMisbehavior(eq EquivocationEvidence, observer Observer, seq uint64, prev string, parents []string) (Event, error) {
	pl := MisbehaviorEquivocationPayload{
		OffenderAuthor: eq.OffenderAuthor,
		OffenderKeyB64: eq.OffenderKeyB64,
		Seq:            eq.Seq,
		EventA:         eq.EventA,
		EventB:         eq.EventB,
	}
	raw, err := json.Marshal(pl)
	if err != nil {
		return Event{}, err
	}
	_, ref, err := InlinePayloadRef(raw)
	if err != nil {
		return Event{}, err
	}
	return NewSignedEvent(SignedEventInput{
		Author:     observer.Author,
		AuthorPub:  observer.AuthorPub,
		AuthorPriv: observer.AuthorPriv,
		Seq:        seq,
		Prev:       prev,
		Parents:    parents,
		Kind:       KindMisbehaviorEquivoc,
		Payload:    raw,
		PayloadRef: ref,
		CreatedAt:  time.Now(),
	})
}

func buildReplayMisbehavior(offender Event, context string, observer Observer, seq uint64, prev string, parents []string) (Event, error) {
	pl := MisbehaviorReplayPayload{
		OffenderAuthor: offender.Author,
		OffenderKeyB64: offender.AuthorKeyB64,
		EventID:        offender.ID,
		Context:        strings.TrimSpace(context),
	}
	raw, err := json.Marshal(pl)
	if err != nil {
		return Event{}, err
	}
	_, ref, err := InlinePayloadRef(raw)
	if err != nil {
		return Event{}, err
	}
	return NewSignedEvent(SignedEventInput{
		Author:     observer.Author,
		AuthorPub:  observer.AuthorPub,
		AuthorPriv: observer.AuthorPriv,
		Seq:        seq,
		Prev:       prev,
		Parents:    parents,
		Kind:       KindMisbehaviorReplay,
		Payload:    raw,
		PayloadRef: ref,
		CreatedAt:  time.Now(),
	})
}

func buildSybilMisbehavior(context string, suspects []string, reason string, observer Observer, seq uint64, prev string, parents []string) (Event, error) {
	pl := MisbehaviorSybilPayload{
		Context:        strings.TrimSpace(context),
		SuspectsKeyB64: suspects,
		Reason:         strings.TrimSpace(reason),
	}
	raw, err := json.Marshal(pl)
	if err != nil {
		return Event{}, err
	}
	_, ref, err := InlinePayloadRef(raw)
	if err != nil {
		return Event{}, err
	}
	return NewSignedEvent(SignedEventInput{
		Author:     observer.Author,
		AuthorPub:  observer.AuthorPub,
		AuthorPriv: observer.AuthorPriv,
		Seq:        seq,
		Prev:       prev,
		Parents:    parents,
		Kind:       KindMisbehaviorSybil,
		Payload:    raw,
		PayloadRef: ref,
		CreatedAt:  time.Now(),
	})
}

func (l *Ledger) EmitSybilMisbehavior(context string, suspects []string, reason string, pol *Policy, observer *Observer) (Event, error) {
	if l == nil {
		return Event{}, errors.New("ledger: ledger is nil")
	}
	if observer == nil {
		return Event{}, errors.New("ledger: observer is nil")
	}
	if err := observer.Valid(); err != nil {
		return Event{}, err
	}
	p := DefaultPolicy()
	if pol != nil {
		p = *pol
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.events == nil {
		l.events = map[string]Event{}
	}
	if l.heads == nil {
		l.heads = map[string]struct{}{}
	}
	if l.authorSeq == nil {
		l.authorSeq = map[string]map[uint64]string{}
	}
	if l.missingRefs == nil {
		l.missingRefs = map[string]int{}
	}
	if l.compromised == nil {
		l.compromised = map[string]struct{}{}
	}
	if l.attestations == nil {
		l.attestations = map[string]map[string]map[string]struct{}{}
	}
	if l.checkpoints == nil {
		l.checkpoints = map[string]map[string]map[string]struct{}{}
	}
	if l.introductions == nil {
		l.introductions = map[string]map[string]map[string]struct{}{}
	}

	seq, prev, parents := l.nextSeqLocked(observer.Author)
	ev, err := buildSybilMisbehavior(context, suspects, reason, *observer, seq, prev, parents)
	if err != nil {
		return Event{}, err
	}
	if err := validateWithPolicy(ev, p); err != nil {
		return Event{}, err
	}
	if err := l.addLocked(ev, false); err != nil {
		return Event{}, err
	}
	return ev, nil
}

func (l *Ledger) Confirmed(eventID string, context string, pol *Policy) (int, bool) {
	if l == nil {
		return 0, false
	}
	p := DefaultPolicy()
	if pol != nil {
		p = *pol
	}
	thr := p.Trust.Threshold(context)
	if thr <= 0 {
		return 0, false
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	ctxMap := l.attestations[eventID]
	if ctxMap == nil {
		return 0, false
	}
	witnesses := ctxMap[context]
	if witnesses == nil {
		return 0, false
	}
	score := 0
	for key := range witnesses {
		if _, bad := l.compromised[key]; bad {
			continue
		}
		w := p.Trust.Weight(context, key)
		if w <= 0 {
			continue
		}
		score += w
		if score >= thr {
			return score, true
		}
	}
	return score, false
}

func (l *Ledger) EventStatus(eventID string, context string, pol *Policy) Status {
	if l == nil {
		return ""
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return ""
	}
	if !l.Have(eventID) {
		return ""
	}
	if _, ok := l.Confirmed(eventID, context, pol); ok {
		return StatusConfirmed
	}
	return StatusAccepted
}

func (l *Ledger) Introduced(subjectKeyB64 string, context string, pol *Policy) (int, bool) {
	if l == nil {
		return 0, false
	}
	p := DefaultPolicy()
	if pol != nil {
		p = *pol
	}
	thr := p.Trust.Threshold(context)
	if thr <= 0 {
		return 0, false
	}
	subjectKeyB64 = strings.TrimSpace(subjectKeyB64)
	if subjectKeyB64 == "" {
		return 0, false
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	ctxMap := l.introductions[context]
	if ctxMap == nil {
		return 0, false
	}
	intros := ctxMap[subjectKeyB64]
	if intros == nil {
		return 0, false
	}
	score := 0
	for introducerKey := range intros {
		if _, bad := l.compromised[introducerKey]; bad {
			continue
		}
		w := p.Trust.Weight(context, introducerKey)
		if w <= 0 {
			continue
		}
		score += w
		if score >= thr {
			return score, true
		}
	}
	return score, false
}

func (l *Ledger) Checkpointed(context string, headsHash string, pol *Policy) (int, bool) {
	if l == nil {
		return 0, false
	}
	p := DefaultPolicy()
	if pol != nil {
		p = *pol
	}
	thr := p.Trust.Threshold(context)
	if thr <= 0 {
		return 0, false
	}
	headsHash = strings.TrimSpace(headsHash)
	if headsHash == "" {
		return 0, false
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	ctxMap := l.checkpoints[context]
	if ctxMap == nil {
		return 0, false
	}
	witnesses := ctxMap[headsHash]
	if witnesses == nil {
		return 0, false
	}
	score := 0
	for key := range witnesses {
		if _, bad := l.compromised[key]; bad {
			continue
		}
		w := p.Trust.Weight(context, key)
		if w <= 0 {
			continue
		}
		score += w
		if score >= thr {
			return score, true
		}
	}
	return score, false
}

func (l *Ledger) Prune(now time.Time, pol *Policy) (int, error) {
	if l == nil {
		return 0, errors.New("ledger: ledger is nil")
	}
	p := DefaultPolicy()
	if pol != nil {
		p = *pol
	}
	if len(p.RetentionByKind) == 0 {
		return 0, nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	referenced := map[string]struct{}{}
	for _, ev := range l.events {
		if ev.Prev != "" {
			referenced[ev.Prev] = struct{}{}
		}
		for _, par := range ev.Parents {
			referenced[par] = struct{}{}
		}
	}

	pruned := 0
	for id, ev := range l.events {
		ret := p.RetentionByKind[ev.Kind]
		if ret <= 0 {
			continue
		}
		if ev.CreatedAt >= now.Add(-ret).Unix() {
			continue
		}
		if _, ok := l.heads[id]; ok {
			continue
		}
		if _, ok := referenced[id]; ok {
			continue
		}
		delete(l.events, id)
		pruned++
	}
	if pruned > 0 {
		l.rebuildLocked()
	}
	return pruned, nil
}

type RetentionPolicy struct {
	MaxEvents    int
	MaxAge       time.Duration
	CriticalOnly bool
}

func rootHashIDs(events []Event) string {
	if len(events) == 0 {
		return ""
	}
	out := append([]Event(nil), events...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt == out[j].CreatedAt {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt < out[j].CreatedAt
	})
	h := sha256.New()
	for i := range out {
		_, _ = h.Write([]byte(out[i].ID))
		_, _ = h.Write([]byte{'\n'})
	}
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func anchorHash(prev string, root string, prunedCount int, firstCreatedAt int64, lastCreatedAt int64, createdAt int64) string {
	h := sha256.New()
	_, _ = h.Write([]byte(prev))
	_, _ = h.Write([]byte{'\n'})
	_, _ = h.Write([]byte(root))
	_, _ = h.Write([]byte{'\n'})
	_, _ = h.Write([]byte(fmt.Sprintf("%d|%d|%d|%d", prunedCount, firstCreatedAt, lastCreatedAt, createdAt)))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func (l *Ledger) ApplyRetention(now time.Time, pol RetentionPolicy) (int, error) {
	if l == nil {
		return 0, errors.New("ledger: ledger is nil")
	}
	if now.IsZero() {
		now = time.Now()
	}
	if pol.MaxEvents <= 0 && pol.MaxAge <= 0 {
		return 0, nil
	}
	if pol.MaxEvents < 0 {
		pol.MaxEvents = 0
	}
	if pol.MaxAge < 0 {
		pol.MaxAge = 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	referenced := map[string]struct{}{}
	for _, ev := range l.events {
		if ev.Prev != "" {
			referenced[ev.Prev] = struct{}{}
		}
		for _, par := range ev.Parents {
			referenced[par] = struct{}{}
		}
	}

	prunedEvents := make([]Event, 0, 32)
	pruned := 0
	if pol.MaxAge > 0 {
		cutoff := now.Add(-pol.MaxAge).Unix()
		for id, ev := range l.events {
			if ev.CreatedAt >= cutoff {
				continue
			}
			if _, ok := l.heads[id]; ok {
				continue
			}
			if _, ok := referenced[id]; ok {
				continue
			}
			if pol.CriticalOnly && isCriticalKind(ev.Kind) {
				continue
			}
			prunedEvents = append(prunedEvents, ev)
			delete(l.events, id)
			pruned++
		}
	}

	if pol.MaxEvents > 0 && len(l.events) > pol.MaxEvents {
		type candidate struct {
			id  string
			ev  Event
			age int64
		}
		all := make([]candidate, 0, len(l.events))
		for id, ev := range l.events {
			all = append(all, candidate{id: id, ev: ev, age: ev.CreatedAt})
		}
		sort.Slice(all, func(i, j int) bool {
			if all[i].age == all[j].age {
				return all[i].id < all[j].id
			}
			return all[i].age < all[j].age
		})

		target := pol.MaxEvents
		for i := 0; i < len(all) && len(l.events) > target; i++ {
			id := all[i].id
			ev := all[i].ev
			if _, ok := l.heads[id]; ok {
				continue
			}
			if _, ok := referenced[id]; ok {
				continue
			}
			if pol.CriticalOnly && isCriticalKind(ev.Kind) {
				continue
			}
			prunedEvents = append(prunedEvents, ev)
			delete(l.events, id)
			pruned++
		}
	}

	if pruned > 0 {
		if len(prunedEvents) > 0 {
			firstCreatedAt := prunedEvents[0].CreatedAt
			lastCreatedAt := prunedEvents[0].CreatedAt
			for i := range prunedEvents {
				if prunedEvents[i].CreatedAt < firstCreatedAt {
					firstCreatedAt = prunedEvents[i].CreatedAt
				}
				if prunedEvents[i].CreatedAt > lastCreatedAt {
					lastCreatedAt = prunedEvents[i].CreatedAt
				}
			}
			root := rootHashIDs(prunedEvents)
			prev := ""
			if n := len(l.anchors); n > 0 {
				prev = l.anchors[n-1].AnchorHashB64
			}
			createdAt := now.Unix()
			l.anchors = append(l.anchors, SnapshotAnchor{
				SchemaVersion:   SchemaV1,
				CreatedAt:       createdAt,
				PrunedCount:     len(prunedEvents),
				FirstCreatedAt:  firstCreatedAt,
				LastCreatedAt:   lastCreatedAt,
				RootHashB64:     root,
				PrevAnchorHashB: prev,
				AnchorHashB64:   anchorHash(prev, root, len(prunedEvents), firstCreatedAt, lastCreatedAt, createdAt),
			})
		}
		l.rebuildLocked()
	}
	return pruned, nil
}

func (l *Ledger) Have(id string) bool {
	if l == nil {
		return false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, ok := l.events[id]
	return ok
}

func (l *Ledger) MissingRefs() map[string]int {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make(map[string]int, len(l.missingRefs))
	for k, v := range l.missingRefs {
		out[k] = v
	}
	return out
}

func (l *Ledger) CompromisedKeys() []string {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]string, 0, len(l.compromised))
	for k := range l.compromised {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (l *Ledger) Get(id string) (Event, bool) {
	if l == nil {
		return Event{}, false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	ev, ok := l.events[id]
	return ev, ok
}

func (l *Ledger) Heads() []string {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]string, 0, len(l.heads))
	for id := range l.heads {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (l *Ledger) Snapshot() Snapshot {
	if l == nil {
		return Snapshot{SchemaVersion: SchemaV1}
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	events := make([]Event, 0, len(l.events))
	for _, ev := range l.events {
		events = append(events, ev)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].CreatedAt == events[j].CreatedAt {
			return events[i].ID < events[j].ID
		}
		return events[i].CreatedAt < events[j].CreatedAt
	})
	heads := make([]string, 0, len(l.heads))
	for h := range l.heads {
		heads = append(heads, h)
	}
	sort.Strings(heads)
	anchors := append([]SnapshotAnchor(nil), l.anchors...)
	return Snapshot{
		SchemaVersion: SchemaV1,
		UpdatedAt:     time.Now().Unix(),
		Events:        events,
		Heads:         heads,
		Anchors:       anchors,
	}
}

func (l *Ledger) Anchors() []SnapshotAnchor {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]SnapshotAnchor(nil), l.anchors...)
}

func (l *Ledger) addLocked(ev Event, requireKnownPrev bool) error {
	if _, ok := l.events[ev.ID]; ok {
		return nil
	}
	if ev.Seq > 1 && requireKnownPrev {
		if _, ok := l.events[ev.Prev]; !ok {
			return errors.New("ledger: prev not found")
		}
	}

	seqs, ok := l.authorSeq[ev.Author]
	if !ok {
		seqs = map[uint64]string{}
		l.authorSeq[ev.Author] = seqs
	}
	if existing, ok := seqs[ev.Seq]; ok && existing != ev.ID {
		return errors.New("ledger: author seq conflict")
	}
	seqs[ev.Seq] = ev.ID
	l.events[ev.ID] = ev
	delete(l.missingRefs, ev.ID)
	delete(l.missingRefWeight, ev.ID)
	l.indexLocked(ev)
	l.trackMissingRefsLocked(ev, requireKnownPrev)

	if affectsHeads(ev.Kind) {
		if ev.Prev != "" {
			delete(l.heads, ev.Prev)
		}
		for _, p := range ev.Parents {
			delete(l.heads, p)
		}
		l.heads[ev.ID] = struct{}{}
	}
	return nil
}

func (l *Ledger) nextSeqLocked(author string) (seq uint64, prev string, parents []string) {
	seq = 1
	if l.authorSeq != nil {
		if m := l.authorSeq[author]; m != nil {
			var max uint64
			var maxID string
			for s, id := range m {
				if s > max {
					max = s
					maxID = id
				}
			}
			if max > 0 {
				seq = max + 1
				prev = maxID
			}
		}
	}
	parents = make([]string, 0, len(l.heads))
	for h := range l.heads {
		parents = append(parents, h)
	}
	sort.Strings(parents)
	return seq, prev, parents
}

func (l *Ledger) rebuildLocked() {
	evs := make([]Event, 0, len(l.events))
	for _, ev := range l.events {
		evs = append(evs, ev)
	}
	l.authorSeq = map[string]map[uint64]string{}
	l.heads = map[string]struct{}{}
	l.missingRefs = map[string]int{}
	l.missingRefWeight = map[string]int{}
	l.attestations = map[string]map[string]map[string]struct{}{}
	l.checkpoints = map[string]map[string]map[string]struct{}{}
	l.introductions = map[string]map[string]map[string]struct{}{}
	for i := range evs {
		_ = l.addLocked(evs[i], false)
	}
	l.recomputeHeadsLocked()
}

func (l *Ledger) trackMissingRefsLocked(ev Event, requireKnownPrev bool) {
	if l.missingRefs == nil {
		l.missingRefs = map[string]int{}
	}
	if l.missingRefWeight == nil {
		l.missingRefWeight = map[string]int{}
	}
	w := syncPriorityWeight(ev.Kind)
	if ev.Seq > 1 && ev.Prev != "" {
		if _, ok := l.events[ev.Prev]; !ok && !requireKnownPrev {
			l.missingRefs[ev.Prev]++
			l.missingRefWeight[ev.Prev] += w
		}
	}
	for _, p := range ev.Parents {
		if _, ok := l.events[p]; !ok {
			l.missingRefs[p]++
			l.missingRefWeight[p] += w
		}
	}
}

func (l *Ledger) indexLocked(ev Event) {
	switch ev.Kind {
	case KindWitnessAttest:
		var pl AttestationPayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return
		}
		wk := ev.AuthorKeyB64
		if l.attestations[pl.EventID] == nil {
			l.attestations[pl.EventID] = map[string]map[string]struct{}{}
		}
		if l.attestations[pl.EventID][pl.Context] == nil {
			l.attestations[pl.EventID][pl.Context] = map[string]struct{}{}
		}
		l.attestations[pl.EventID][pl.Context][wk] = struct{}{}
	case KindWitnessCheckpoint:
		var pl CheckpointPayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return
		}
		wk := ev.AuthorKeyB64
		ctx := pl.Context
		h := pl.HeadsHash
		if ctx == "" || h == "" {
			return
		}
		if l.checkpoints[ctx] == nil {
			l.checkpoints[ctx] = map[string]map[string]struct{}{}
		}
		if l.checkpoints[ctx][h] == nil {
			l.checkpoints[ctx][h] = map[string]struct{}{}
		}
		l.checkpoints[ctx][h][wk] = struct{}{}
	case KindIdentityIntroduce:
		var pl IdentityIntroducePayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return
		}
		introducerKey := ev.AuthorKeyB64
		ctx := strings.TrimSpace(pl.Context)
		subjectKey := strings.TrimSpace(pl.SubjectKeyB64)
		if ctx == "" || subjectKey == "" {
			return
		}
		if l.introductions[ctx] == nil {
			l.introductions[ctx] = map[string]map[string]struct{}{}
		}
		if l.introductions[ctx][subjectKey] == nil {
			l.introductions[ctx][subjectKey] = map[string]struct{}{}
		}
		l.introductions[ctx][subjectKey][introducerKey] = struct{}{}
	}
}

func (l *Ledger) recomputeHeadsLocked() {
	l.heads = map[string]struct{}{}
	for id, ev := range l.events {
		if affectsHeads(ev.Kind) {
			l.heads[id] = struct{}{}
		}
	}
	for _, ev := range l.events {
		if !affectsHeads(ev.Kind) {
			continue
		}
		if ev.Prev != "" {
			delete(l.heads, ev.Prev)
		}
		for _, p := range ev.Parents {
			delete(l.heads, p)
		}
	}
}
