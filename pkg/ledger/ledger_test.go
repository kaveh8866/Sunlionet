package ledger

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/identity"
)

func TestLedger_AddAndHeads(t *testing.T) {
	p, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new persona: %v", err)
	}
	pub, priv, err := p.SignKeypair()
	if err != nil {
		t.Fatalf("sign keypair: %v", err)
	}

	ev1, err := NewSignedEvent(SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"hello"}`),
	})
	if err != nil {
		t.Fatalf("new signed event 1: %v", err)
	}

	l := New()
	if err := l.Add(ev1); err != nil {
		t.Fatalf("add ev1: %v", err)
	}
	heads := l.Heads()
	if len(heads) != 1 || heads[0] != ev1.ID {
		t.Fatalf("unexpected heads after ev1: %#v", heads)
	}

	ev2, err := NewSignedEvent(SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        2,
		Prev:       ev1.ID,
		Parents:    l.Heads(),
		Kind:       "chat.ack",
		Payload:    json.RawMessage(`{"mid":"m1"}`),
	})
	if err != nil {
		t.Fatalf("new signed event 2: %v", err)
	}
	if err := l.Add(ev2); err != nil {
		t.Fatalf("add ev2: %v", err)
	}
	heads = l.Heads()
	if len(heads) != 1 || heads[0] != ev2.ID {
		t.Fatalf("unexpected heads after ev2: %#v", heads)
	}

	ev2b, err := NewSignedEvent(SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        2,
		Prev:       ev1.ID,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"fork"}`),
	})
	if err != nil {
		t.Fatalf("new signed event 2b: %v", err)
	}
	if err := l.Add(ev2b); err == nil {
		t.Fatalf("expected seq conflict, got nil")
	}
}

func TestPlanSyncFromPeer_PrioritizesCriticalReferencedMissing(t *testing.T) {
	p, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new persona: %v", err)
	}
	pub, priv, err := p.SignKeypair()
	if err != nil {
		t.Fatalf("sign keypair: %v", err)
	}

	l := New()

	headsHash := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	pl := SyncSummaryPayload{Context: "ctx", HeadsHash: headsHash, HeadCount: 1}
	rawPl, err := json.Marshal(pl)
	if err != nil {
		t.Fatalf("marshal sync.summary payload: %v", err)
	}
	_, ref, err := InlinePayloadRef(rawPl)
	if err != nil {
		t.Fatalf("InlinePayloadRef: %v", err)
	}

	crit, err := NewSignedEvent(SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Parents:    []string{"missing-critical"},
		Kind:       KindSyncSummary,
		Payload:    rawPl,
		PayloadRef: ref,
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new critical: %v", err)
	}
	if err := l.Add(crit); err != nil {
		t.Fatalf("add critical: %v", err)
	}

	norm, err := NewSignedEvent(SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        2,
		Prev:       crit.ID,
		Parents:    []string{"missing-normal"},
		Kind:       KindChatMessage,
		Payload:    json.RawMessage(`{"t":"hi"}`),
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new normal: %v", err)
	}
	if err := l.Add(norm); err != nil {
		t.Fatalf("add normal: %v", err)
	}

	peer := InventoryMessage{
		SchemaVersion: SyncSchemaV1,
		Heads:         []string{},
		Have:          []string{"missing-critical", "missing-normal"},
	}
	plan := l.PlanSyncFromPeer(peer, 2)
	if len(plan.Want) != 2 {
		t.Fatalf("unexpected want len: %d want=%v", len(plan.Want), plan.Want)
	}
	if plan.Want[0] != "missing-critical" || plan.Want[1] != "missing-normal" {
		t.Fatalf("unexpected want order: %v", plan.Want)
	}
}

func TestNewSignedEvent_TimestampObfuscation(t *testing.T) {
	p, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new persona: %v", err)
	}
	pub, priv, err := p.SignKeypair()
	if err != nil {
		t.Fatalf("sign keypair: %v", err)
	}

	r0 := func(b []byte) error {
		for i := range b {
			b[i] = 0
		}
		return nil
	}

	base := time.Unix(1700000000, 123456789)
	evB, err := NewSignedEvent(SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"x"}`),
		CreatedAt:  base,

		CreatedAtBucket:    5 * time.Minute,
		CreatedAtJitterMax: 0,
		Rand:               r0,
	})
	if err != nil {
		t.Fatalf("new signed event: %v", err)
	}
	if evB.CreatedAt != base.Truncate(5*time.Minute).Unix() {
		t.Fatalf("expected bucketed created_at, got %d", evB.CreatedAt)
	}

	evJ, err := NewSignedEvent(SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"x"}`),
		CreatedAt:  base,

		CreatedAtBucket:    0,
		CreatedAtJitterMax: 10 * time.Second,
		Rand:               r0,
	})
	if err != nil {
		t.Fatalf("new signed event jitter: %v", err)
	}
	if evJ.CreatedAt != base.Add(-10*time.Second).Unix() {
		t.Fatalf("expected jittered created_at, got %d", evJ.CreatedAt)
	}
}

func TestLedger_EmitWitnessAttestAndCheckpoint(t *testing.T) {
	w, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new persona: %v", err)
	}
	wPub, wPriv, err := w.SignKeypair()
	if err != nil {
		t.Fatalf("sign keypair: %v", err)
	}
	wKey := base64.RawURLEncoding.EncodeToString(wPub)

	ctxName := "ctx"
	pol := DefaultPolicy()
	pol.Trust = TrustPolicy{
		Witnesses:  map[string]map[string]int{ctxName: {wKey: 1}},
		Thresholds: map[string]int{ctxName: 1},
	}

	obs := &Observer{Author: string(w.ID), AuthorPub: wPub, AuthorPriv: wPriv}

	l := New()
	base, err := NewSignedEvent(SignedEventInput{
		Author:     string(w.ID),
		AuthorPub:  wPub,
		AuthorPriv: wPriv,
		Seq:        1,
		Kind:       KindChatMessage,
		Payload:    json.RawMessage(`{"t":"base"}`),
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new base: %v", err)
	}
	if err := l.Add(base); err != nil {
		t.Fatalf("add base: %v", err)
	}

	if _, err := l.EmitWitnessAttest(ctxName, base.ID, &pol, obs); err != nil {
		t.Fatalf("EmitWitnessAttest: %v", err)
	}
	if _, ok := l.Confirmed(base.ID, ctxName, &pol); !ok {
		t.Fatalf("expected confirmed after attest")
	}

	expHash, _ := ComputeHeadsHash(l.Heads())
	if _, err := l.EmitWitnessCheckpoint(ctxName, &pol, obs); err != nil {
		t.Fatalf("EmitWitnessCheckpoint: %v", err)
	}
	if _, ok := l.Checkpointed(ctxName, expHash, &pol); !ok {
		t.Fatalf("expected checkpointed after checkpoint")
	}
}

func TestEvent_TamperDetected(t *testing.T) {
	p, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new persona: %v", err)
	}
	pub, priv, err := p.SignKeypair()
	if err != nil {
		t.Fatalf("sign keypair: %v", err)
	}
	ev, err := NewSignedEvent(SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"hello"}`),
	})
	if err != nil {
		t.Fatalf("new signed event: %v", err)
	}
	ev.Kind = "chat.msg2"
	if err := ev.Verify(); err == nil {
		t.Fatalf("expected verify error after tamper, got nil")
	}
}

func TestLedger_ApplyRetention_MaxEventsAndMaxAge(t *testing.T) {
	p, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new persona: %v", err)
	}
	pub, priv, err := p.SignKeypair()
	if err != nil {
		t.Fatalf("sign keypair: %v", err)
	}

	l := New()
	base := time.Unix(1700000000, 0)
	pl := json.RawMessage(`{"ctx":"x","heads_hash_b64url":"a","head_count":1}`)
	_, ref, err := InlinePayloadRef(pl)
	if err != nil {
		t.Fatalf("inline payload ref: %v", err)
	}
	for i := 0; i < 10; i++ {
		ev, err := NewSignedEvent(SignedEventInput{
			Author:     string(p.ID) + "-r" + string(rune('a'+i)),
			AuthorPub:  pub,
			AuthorPriv: priv,
			Seq:        1,
			Kind:       KindSyncSummary,
			Payload:    pl,
			PayloadRef: ref,
			CreatedAt:  base.Add(time.Duration(i) * time.Hour),
		})
		if err != nil {
			t.Fatalf("new signed event %d: %v", i, err)
		}
		if _, err := l.Apply(ev, nil, nil); err != nil {
			t.Fatalf("apply %d: %v", i, err)
		}
	}

	pruned, err := l.ApplyRetention(base.Add(12*time.Hour), RetentionPolicy{MaxEvents: 3})
	if err != nil {
		t.Fatalf("apply retention max events: %v", err)
	}
	if pruned <= 0 {
		t.Fatalf("expected some pruned events, got %d", pruned)
	}
	if anchors := l.Anchors(); len(anchors) != 1 {
		t.Fatalf("expected 1 anchor after retention, got %d", len(anchors))
	}
	if anchors := l.Anchors(); anchors[0].AnchorHashB64 == "" || anchors[0].RootHashB64 == "" {
		t.Fatalf("expected anchor hashes to be set")
	}
	if snap := l.Snapshot(); len(snap.Events) > 3 {
		t.Fatalf("expected <=3 events after max events retention, got %d", len(snap.Events))
	}

	l2 := New()
	for i := 0; i < 6; i++ {
		ev, err := NewSignedEvent(SignedEventInput{
			Author:     string(p.ID) + "-a" + string(rune('a'+i)),
			AuthorPub:  pub,
			AuthorPriv: priv,
			Seq:        1,
			Kind:       KindSyncSummary,
			Payload:    pl,
			PayloadRef: ref,
			CreatedAt:  base.Add(time.Duration(i) * time.Hour),
		})
		if err != nil {
			t.Fatalf("new signed event age %d: %v", i, err)
		}
		if _, err := l2.Apply(ev, nil, nil); err != nil {
			t.Fatalf("apply age %d: %v", i, err)
		}
	}
	pruned2, err := l2.ApplyRetention(base.Add(10*time.Hour), RetentionPolicy{MaxAge: 2 * time.Hour})
	if err != nil {
		t.Fatalf("apply retention max age: %v", err)
	}
	if pruned2 <= 0 {
		t.Fatalf("expected some pruned by age, got %d", pruned2)
	}
	if anchors := l2.Anchors(); len(anchors) != 1 {
		t.Fatalf("expected 1 anchor after retention, got %d", len(anchors))
	}
	snap2 := l2.Snapshot()
	for i := range snap2.Events {
		if snap2.Events[i].CreatedAt < base.Add(8*time.Hour).Unix() {
			t.Fatalf("expected no events older than cutoff, got created_at=%d", snap2.Events[i].CreatedAt)
		}
	}
}

func TestVerifyBatch_DedupAndCache(t *testing.T) {
	p, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new persona: %v", err)
	}
	pub, priv, err := p.SignKeypair()
	if err != nil {
		t.Fatalf("sign keypair: %v", err)
	}

	evOK, err := NewSignedEvent(SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       KindChatMessage,
		Payload:    json.RawMessage(`{"t":"ok"}`),
	})
	if err != nil {
		t.Fatalf("new signed event ok: %v", err)
	}

	evBad := evOK
	evBad, err = NewSignedEvent(SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        2,
		Prev:       evOK.ID,
		Kind:       KindChatMessage,
		Payload:    json.RawMessage(`{"t":"bad"}`),
	})
	if err != nil {
		t.Fatalf("new signed event bad: %v", err)
	}
	evBad.SigB64 = base64.RawURLEncoding.EncodeToString([]byte{1, 2, 3})

	cache := NewVerifiedEventCache(32, 5*time.Minute)
	events := []Event{evOK, evOK, evBad}

	rep1, ok1 := VerifyBatch(events, nil, VerifyBatchOptions{
		VerifyWorkers:   2,
		VerifyBatchSize: 8,
		Cache:           cache,
	})
	if rep1.Unique != 2 {
		t.Fatalf("expected unique=2, got %d", rep1.Unique)
	}
	if rep1.Verified != 1 {
		t.Fatalf("expected verified=1, got %d", rep1.Verified)
	}
	if rep1.Cached != 0 {
		t.Fatalf("expected cached=0, got %d", rep1.Cached)
	}
	if !ok1[0] || !ok1[1] || ok1[2] {
		t.Fatalf("unexpected ok flags: %#v", ok1)
	}

	rep2, ok2 := VerifyBatch(events, nil, VerifyBatchOptions{
		VerifyWorkers:   2,
		VerifyBatchSize: 8,
		Cache:           cache,
	})
	if rep2.Cached != 1 {
		t.Fatalf("expected cached=1, got %d", rep2.Cached)
	}
	if rep2.Verified != 1 {
		t.Fatalf("expected verified=1, got %d", rep2.Verified)
	}
	if !ok2[0] || !ok2[1] || ok2[2] {
		t.Fatalf("unexpected ok flags after cache: %#v", ok2)
	}
}
