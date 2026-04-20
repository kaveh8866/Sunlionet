package ledger

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/identity"
)

func TestLTC_AttestationQuorum(t *testing.T) {
	author, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new persona: %v", err)
	}
	authorPub, authorPriv, err := author.SignKeypair()
	if err != nil {
		t.Fatalf("sign keypair: %v", err)
	}

	w1, _ := identity.NewPersona()
	w1Pub, w1Priv, _ := w1.SignKeypair()
	w2, _ := identity.NewPersona()
	w2Pub, w2Priv, _ := w2.SignKeypair()
	w3, _ := identity.NewPersona()
	w3Pub, w3Priv, _ := w3.SignKeypair()

	ctx := "group:alpha"
	w1Key := base64.RawURLEncoding.EncodeToString(w1Pub)
	w2Key := base64.RawURLEncoding.EncodeToString(w2Pub)
	pol := DefaultPolicy()
	pol.Trust = TrustPolicy{
		Witnesses: map[string]map[string]int{
			ctx: {w1Key: 1, w2Key: 1},
		},
		Thresholds: map[string]int{ctx: 2},
	}

	l := New()

	ev, err := NewSignedEvent(SignedEventInput{
		Author:     string(author.ID),
		AuthorPub:  authorPub,
		AuthorPriv: authorPriv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"hello"}`),
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new signed event: %v", err)
	}
	if _, err := l.Apply(ev, &pol, nil); err != nil {
		t.Fatalf("apply ev: %v", err)
	}

	pl := AttestationPayload{EventID: ev.ID, Context: ctx}
	raw, _ := json.Marshal(pl)
	_, ref, _ := InlinePayloadRef(raw)

	a1, err := NewSignedEvent(SignedEventInput{
		Author:     string(w1.ID),
		AuthorPub:  w1Pub,
		AuthorPriv: w1Priv,
		Seq:        1,
		Kind:       KindWitnessAttest,
		Payload:    raw,
		PayloadRef: ref,
		Parents:    l.Heads(),
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new attest 1: %v", err)
	}
	if _, err := l.Apply(a1, &pol, nil); err != nil {
		t.Fatalf("apply attest 1: %v", err)
	}
	if score, ok := l.Confirmed(ev.ID, ctx, &pol); ok || score != 1 {
		t.Fatalf("expected score=1 ok=false, got score=%d ok=%v", score, ok)
	}

	a2, err := NewSignedEvent(SignedEventInput{
		Author:     string(w2.ID),
		AuthorPub:  w2Pub,
		AuthorPriv: w2Priv,
		Seq:        1,
		Kind:       KindWitnessAttest,
		Payload:    raw,
		PayloadRef: ref,
		Parents:    l.Heads(),
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new attest 2: %v", err)
	}
	if _, err := l.Apply(a2, &pol, nil); err != nil {
		t.Fatalf("apply attest 2: %v", err)
	}
	if score, ok := l.Confirmed(ev.ID, ctx, &pol); !ok || score != 2 {
		t.Fatalf("expected score=2 ok=true, got score=%d ok=%v", score, ok)
	}

	a3, err := NewSignedEvent(SignedEventInput{
		Author:     string(w3.ID),
		AuthorPub:  w3Pub,
		AuthorPriv: w3Priv,
		Seq:        1,
		Kind:       KindWitnessAttest,
		Payload:    raw,
		PayloadRef: ref,
		Parents:    l.Heads(),
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new attest 3: %v", err)
	}
	if _, err := l.Apply(a3, &pol, nil); err == nil {
		t.Fatalf("expected non-witness attest to fail")
	}
}

func TestLTC_CheckpointQuorum(t *testing.T) {
	author, _ := identity.NewPersona()
	authorPub, authorPriv, _ := author.SignKeypair()
	w1, _ := identity.NewPersona()
	w1Pub, w1Priv, _ := w1.SignKeypair()
	w2, _ := identity.NewPersona()
	w2Pub, w2Priv, _ := w2.SignKeypair()

	ctx := "group:alpha"
	w1Key := base64.RawURLEncoding.EncodeToString(w1Pub)
	w2Key := base64.RawURLEncoding.EncodeToString(w2Pub)
	pol := DefaultPolicy()
	pol.Trust = TrustPolicy{
		Witnesses: map[string]map[string]int{
			ctx: {w1Key: 1, w2Key: 1},
		},
		Thresholds: map[string]int{ctx: 2},
	}

	l := New()
	ev1, _ := NewSignedEvent(SignedEventInput{
		Author:     string(author.ID),
		AuthorPub:  authorPub,
		AuthorPriv: authorPriv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"one"}`),
		CreatedAt:  time.Now(),
	})
	ev2, _ := NewSignedEvent(SignedEventInput{
		Author:     string(author.ID),
		AuthorPub:  authorPub,
		AuthorPriv: authorPriv,
		Seq:        2,
		Prev:       ev1.ID,
		Parents:    []string{ev1.ID},
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"two"}`),
		CreatedAt:  time.Now(),
	})
	if _, err := l.Apply(ev1, &pol, nil); err != nil {
		t.Fatalf("apply ev1: %v", err)
	}
	if _, err := l.Apply(ev2, &pol, nil); err != nil {
		t.Fatalf("apply ev2: %v", err)
	}

	heads := l.Heads()
	h, cnt := ComputeHeadsHash(heads)
	cp := CheckpointPayload{
		Context:    ctx,
		Heads:      heads,
		HeadsHash:  h,
		HeadsCount: cnt,
	}
	raw, _ := json.Marshal(cp)
	_, ref, _ := InlinePayloadRef(raw)

	c1, _ := NewSignedEvent(SignedEventInput{
		Author:     string(w1.ID),
		AuthorPub:  w1Pub,
		AuthorPriv: w1Priv,
		Seq:        1,
		Kind:       KindWitnessCheckpoint,
		Payload:    raw,
		PayloadRef: ref,
		Parents:    l.Heads(),
		CreatedAt:  time.Now(),
	})
	if _, err := l.Apply(c1, &pol, nil); err != nil {
		t.Fatalf("apply checkpoint 1: %v", err)
	}
	if score, ok := l.Checkpointed(ctx, h, &pol); ok || score != 1 {
		t.Fatalf("expected score=1 ok=false, got score=%d ok=%v", score, ok)
	}

	c2, _ := NewSignedEvent(SignedEventInput{
		Author:     string(w2.ID),
		AuthorPub:  w2Pub,
		AuthorPriv: w2Priv,
		Seq:        1,
		Kind:       KindWitnessCheckpoint,
		Payload:    raw,
		PayloadRef: ref,
		Parents:    l.Heads(),
		CreatedAt:  time.Now(),
	})
	if _, err := l.Apply(c2, &pol, nil); err != nil {
		t.Fatalf("apply checkpoint 2: %v", err)
	}
	if score, ok := l.Checkpointed(ctx, h, &pol); !ok || score != 2 {
		t.Fatalf("expected score=2 ok=true, got score=%d ok=%v", score, ok)
	}
}

func TestLedger_EquivocationDetectsAndEmitsMisbehavior(t *testing.T) {
	offender, _ := identity.NewPersona()
	offPub, offPriv, _ := offender.SignKeypair()
	observerP, _ := identity.NewPersona()
	obsPub, obsPriv, _ := observerP.SignKeypair()

	l := New()

	ev1, _ := NewSignedEvent(SignedEventInput{
		Author:     string(offender.ID),
		AuthorPub:  offPub,
		AuthorPriv: offPriv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"root"}`),
		CreatedAt:  time.Now(),
	})
	if _, err := l.Apply(ev1, nil, nil); err != nil {
		t.Fatalf("apply ev1: %v", err)
	}
	ev2a, _ := NewSignedEvent(SignedEventInput{
		Author:     string(offender.ID),
		AuthorPub:  offPub,
		AuthorPriv: offPriv,
		Seq:        2,
		Prev:       ev1.ID,
		Parents:    []string{ev1.ID},
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"A"}`),
		CreatedAt:  time.Now(),
	})
	if _, err := l.Apply(ev2a, nil, nil); err != nil {
		t.Fatalf("apply ev2a: %v", err)
	}
	ev2b, _ := NewSignedEvent(SignedEventInput{
		Author:     string(offender.ID),
		AuthorPub:  offPub,
		AuthorPriv: offPriv,
		Seq:        2,
		Prev:       ev1.ID,
		Parents:    []string{ev1.ID},
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"B"}`),
		CreatedAt:  time.Now(),
	})
	_, err := l.Apply(ev2b, nil, &Observer{
		Author:     string(observerP.ID),
		AuthorPub:  obsPub,
		AuthorPriv: obsPriv,
	})
	if err == nil {
		t.Fatalf("expected equivocation error")
	}
	if _, ok := err.(*EquivocationError); !ok {
		t.Fatalf("expected EquivocationError, got %T", err)
	}

	var found Event
	for _, ev := range l.Snapshot().Events {
		if ev.Kind == KindMisbehaviorEquivoc {
			found = ev
			break
		}
	}
	if found.ID == "" {
		t.Fatalf("expected misbehavior.equivocation event")
	}
	raw, ok, err := DecodeInlinePayloadRef(found.PayloadRef, MaxInlinePayloadBytes)
	if err != nil || !ok {
		t.Fatalf("decode misbehavior payload: ok=%v err=%v", ok, err)
	}
	var pl MisbehaviorEquivocationPayload
	if err := json.Unmarshal(raw, &pl); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if pl.EventA == "" || pl.EventB == "" {
		t.Fatalf("expected event ids in payload: %#v", pl)
	}
}

func TestPolicy_TimeWindow(t *testing.T) {
	p, _ := identity.NewPersona()
	pub, priv, _ := p.SignKeypair()

	l := New()
	pol := DefaultPolicy()
	pol.MaxClockSkew = time.Minute
	pol.MaxEventAge = time.Minute

	future, _ := NewSignedEvent(SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"future"}`),
		CreatedAt:  time.Now().Add(2 * time.Minute),
	})
	if _, err := l.Apply(future, &pol, nil); err == nil {
		t.Fatalf("expected future event to fail")
	}

	old, _ := NewSignedEvent(SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"old"}`),
		CreatedAt:  time.Now().Add(-2 * time.Minute),
	})
	if _, err := l.Apply(old, &pol, nil); err == nil {
		t.Fatalf("expected old event to fail")
	}
}

func TestLedger_EmitsMisbehaviorReplayOnTooOld(t *testing.T) {
	offender, _ := identity.NewPersona()
	offPub, offPriv, _ := offender.SignKeypair()
	observerP, _ := identity.NewPersona()
	obsPub, obsPriv, _ := observerP.SignKeypair()

	l := New()
	pol := DefaultPolicy()
	pol.MaxEventAge = time.Minute

	ev, _ := NewSignedEvent(SignedEventInput{
		Author:     string(offender.ID),
		AuthorPub:  offPub,
		AuthorPriv: offPriv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"old"}`),
		CreatedAt:  time.Now().Add(-2 * time.Minute),
	})
	_, err := l.ApplyWithContext(ev, "group:alpha", &pol, &Observer{
		Author:     string(observerP.ID),
		AuthorPub:  obsPub,
		AuthorPriv: obsPriv,
	})
	if err == nil {
		t.Fatalf("expected old event to fail")
	}
	if err != ErrEventTooOld {
		t.Fatalf("expected ErrEventTooOld, got %v", err)
	}

	var found Event
	for _, e := range l.Snapshot().Events {
		if e.Kind == KindMisbehaviorReplay {
			found = e
			break
		}
	}
	if found.ID == "" {
		t.Fatalf("expected misbehavior.replay event")
	}
	raw, ok, err := DecodeInlinePayloadRef(found.PayloadRef, MaxInlinePayloadBytes)
	if err != nil || !ok {
		t.Fatalf("decode payload: ok=%v err=%v", ok, err)
	}
	var pl MisbehaviorReplayPayload
	if err := json.Unmarshal(raw, &pl); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if pl.EventID != ev.ID || pl.OffenderAuthor != string(offender.ID) {
		t.Fatalf("unexpected payload: %#v", pl)
	}
}

func TestLedger_PruneRemovesOldEphemeral(t *testing.T) {
	p, _ := identity.NewPersona()
	pub, priv, _ := p.SignKeypair()

	ctx := "group:alpha"
	w1, _ := identity.NewPersona()
	w1Pub, w1Priv, _ := w1.SignKeypair()
	w1Key := base64.RawURLEncoding.EncodeToString(w1Pub)

	pol := DefaultPolicy()
	pol.Trust = TrustPolicy{
		Witnesses:  map[string]map[string]int{ctx: {w1Key: 1}},
		Thresholds: map[string]int{ctx: 1},
	}
	pol.RetentionByKind = map[string]time.Duration{
		KindWitnessAttest: time.Hour,
	}

	l := New()
	ev, _ := NewSignedEvent(SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"x"}`),
		CreatedAt:  time.Now(),
	})
	if _, err := l.Apply(ev, &pol, nil); err != nil {
		t.Fatalf("apply ev: %v", err)
	}

	pl := AttestationPayload{EventID: ev.ID, Context: ctx}
	raw, _ := json.Marshal(pl)
	_, ref, _ := InlinePayloadRef(raw)
	att, _ := NewSignedEvent(SignedEventInput{
		Author:     string(w1.ID),
		AuthorPub:  w1Pub,
		AuthorPriv: w1Priv,
		Seq:        1,
		Kind:       KindWitnessAttest,
		Payload:    raw,
		PayloadRef: ref,
		Parents:    l.Heads(),
		CreatedAt:  time.Now().Add(-2 * time.Hour),
	})
	if _, err := l.Apply(att, &pol, nil); err != nil {
		t.Fatalf("apply attest: %v", err)
	}

	if _, ok := l.Get(att.ID); !ok {
		t.Fatalf("expected attestation present before prune")
	}
	pruned, err := l.Prune(time.Now(), &pol)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned == 0 {
		t.Fatalf("expected at least one pruned event")
	}
	if _, ok := l.Get(att.ID); ok {
		t.Fatalf("expected attestation pruned")
	}
	if _, ok := l.Get(ev.ID); !ok {
		t.Fatalf("expected main event still present")
	}
}

func TestSync_InventoryWantAndApply(t *testing.T) {
	author, _ := identity.NewPersona()
	pub, priv, _ := author.SignKeypair()

	missing, _ := NewSignedEvent(SignedEventInput{
		Author:     string(author.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"missing"}`),
		CreatedAt:  time.Now(),
	})

	local := New()
	ev, _ := NewSignedEvent(SignedEventInput{
		Author:     string(author.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        2,
		Prev:       missing.ID,
		Parents:    []string{missing.ID},
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"needs-parent"}`),
		CreatedAt:  time.Now(),
	})
	if _, err := local.Apply(ev, nil, nil); err != nil {
		t.Fatalf("apply ev: %v", err)
	}
	if local.Have(missing.ID) {
		t.Fatalf("did not expect to already have missing event")
	}
	if local.MissingRefs()[missing.ID] == 0 {
		t.Fatalf("expected missing ref to be tracked")
	}

	peer := New()
	if _, err := peer.Apply(missing, nil, nil); err != nil {
		t.Fatalf("apply missing to peer: %v", err)
	}
	peerInv := peer.BuildInventoryMessage(32)

	plan := local.PlanSyncFromPeer(peerInv, 32)
	if len(plan.Want) != 1 || plan.Want[0] != missing.ID {
		t.Fatalf("unexpected want: %#v", plan.Want)
	}
	msg := peer.BuildEventsMessage(plan.Want, 32)
	if len(msg.Events) != 1 || msg.Events[0].ID != missing.ID {
		t.Fatalf("unexpected events message: %#v", msg.Events)
	}
	if _, err := local.ApplyEventsMessage(msg, "", nil, nil); err != nil {
		t.Fatalf("apply events message: %v", err)
	}
	if !local.Have(missing.ID) {
		t.Fatalf("expected missing event applied")
	}
	if _, ok := local.MissingRefs()[missing.ID]; ok {
		t.Fatalf("expected missing ref cleared")
	}
}
