package ledgersync

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/identity"
	"github.com/kaveh/sunlionet-agent/pkg/ledger"
)

func TestPrompt2_BuildHeadsAndInventory(t *testing.T) {
	p, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new persona: %v", err)
	}
	pub, priv, err := p.SignKeypair()
	if err != nil {
		t.Fatalf("sign keypair: %v", err)
	}

	ev1, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       ledger.KindChatMessage,
		Payload:    json.RawMessage(`{"t":"a"}`),
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new signed event 1: %v", err)
	}
	ev2, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        2,
		Prev:       ev1.ID,
		Parents:    []string{ev1.ID},
		Kind:       ledger.KindGroupCreate,
		Payload:    json.RawMessage(`{"t":"b"}`),
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new signed event 2: %v", err)
	}

	l := ledger.New()
	if _, err := l.Apply(ev1, nil, nil); err != nil {
		t.Fatalf("apply ev1: %v", err)
	}
	if _, err := l.Apply(ev2, nil, nil); err != nil {
		t.Fatalf("apply ev2: %v", err)
	}

	s := NewSync(l)
	hm := s.BuildHeadsMessage()
	if len(hm.Heads) != 1 {
		t.Fatalf("expected 1 head, got %d", len(hm.Heads))
	}
	headID := base64.RawURLEncoding.EncodeToString(hm.Heads[0])
	if headID != ev2.ID {
		t.Fatalf("unexpected head: got %q want %q", headID, ev2.ID)
	}

	inv := s.BuildInventoryMessage(0)
	if len(inv.Heads) != 1 {
		t.Fatalf("expected 1 inv head, got %d", len(inv.Heads))
	}
	invHead := base64.RawURLEncoding.EncodeToString(inv.Heads[0])
	if invHead != ev2.ID {
		t.Fatalf("unexpected inv head: got %q want %q", invHead, ev2.ID)
	}
	if len(inv.Have) != 0 {
		t.Fatalf("expected have to be empty with maxHave=0")
	}
}

func TestPrompt2_WantPlanning_HeadsFirstThenMissingRefs(t *testing.T) {
	p, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new persona: %v", err)
	}
	pub, priv, err := p.SignKeypair()
	if err != nil {
		t.Fatalf("sign keypair: %v", err)
	}

	parent, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       ledger.KindChatMessage,
		Payload:    json.RawMessage(`{"t":"parent"}`),
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new signed event parent: %v", err)
	}
	child, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        2,
		Prev:       parent.ID,
		Parents:    []string{parent.ID},
		Kind:       ledger.KindGroupJoin,
		Payload:    json.RawMessage(`{"t":"child"}`),
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new signed event child: %v", err)
	}

	local := ledger.New()
	if _, err := local.Apply(child, nil, nil); err != nil {
		t.Fatalf("apply child: %v", err)
	}
	if local.MissingRefs()[parent.ID] == 0 {
		t.Fatalf("expected missing ref to be tracked")
	}

	s := NewSync(local)
	peerInv := InventoryMessage{
		Heads: [][]byte{mustDecodeID(t, child.ID)},
		Have:  [][]byte{mustDecodeID(t, parent.ID)},
	}
	want := s.PlanSyncFromPeer(peerInv, 8)
	if len(want.Want) < 1 {
		t.Fatalf("expected at least 1 wanted id, got %d", len(want.Want))
	}
	gotParent := false
	gotChild := false
	for _, w := range want.Want {
		id := base64.RawURLEncoding.EncodeToString(w)
		if id == parent.ID {
			gotParent = true
		}
		if id == child.ID {
			gotChild = true
		}
	}
	if !gotParent {
		t.Fatalf("expected want to include missing parent")
	}
	if gotChild {
		t.Fatalf("did not expect want to include already-known head")
	}
}

func TestPrompt2_ApplyEvents_DedupeAndResume(t *testing.T) {
	p, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new persona: %v", err)
	}
	pub, priv, err := p.SignKeypair()
	if err != nil {
		t.Fatalf("sign keypair: %v", err)
	}

	parent, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       ledger.KindChatMessage,
		Payload:    json.RawMessage(`{"t":"parent"}`),
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new signed event parent: %v", err)
	}
	child, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        2,
		Prev:       parent.ID,
		Parents:    []string{parent.ID},
		Kind:       ledger.KindGroupJoin,
		Payload:    json.RawMessage(`{"t":"child"}`),
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new signed event child: %v", err)
	}

	peerLedger := ledger.New()
	if _, err := peerLedger.Apply(parent, nil, nil); err != nil {
		t.Fatalf("apply parent to peer: %v", err)
	}
	if _, err := peerLedger.Apply(child, nil, nil); err != nil {
		t.Fatalf("apply child to peer: %v", err)
	}

	localLedger := ledger.New()
	localSync := NewSync(localLedger)

	part := EventsMessage{Events: []*ledger.Event{&child}}
	rep, err := localSync.ApplyEventsMessage(part)
	if err != nil {
		t.Fatalf("apply partial: %v", err)
	}
	if rep.Applied != 1 || rep.Rejected != 0 {
		t.Fatalf("unexpected report: %+v", rep)
	}
	if localLedger.MissingRefs()[parent.ID] == 0 {
		t.Fatalf("expected missing ref after child-only apply")
	}

	rep2, err := localSync.ApplyEventsMessage(part)
	if err != nil {
		t.Fatalf("apply duplicate: %v", err)
	}
	if rep2.Dupe != 1 {
		t.Fatalf("expected dupe=1, got %+v", rep2)
	}

	peerSync := NewSync(peerLedger)
	peerInv := InventoryMessage{
		Heads: [][]byte{mustDecodeID(t, child.ID)},
		Have:  [][]byte{mustDecodeID(t, parent.ID)},
	}
	want := localSync.PlanSyncFromPeer(peerInv, 8)
	gotParent := false
	for _, w := range want.Want {
		if base64.RawURLEncoding.EncodeToString(w) == parent.ID {
			gotParent = true
			break
		}
	}
	if !gotParent {
		t.Fatalf("expected want to include missing parent")
	}

	reply := peerSync.BuildEventsMessage(WantMessage{Want: [][]byte{mustDecodeID(t, parent.ID)}}, 8)
	rep3, err := localSync.ApplyEventsMessage(reply)
	if err != nil {
		t.Fatalf("apply parent: %v", err)
	}
	if rep3.Applied != 1 {
		t.Fatalf("expected applied=1, got %+v", rep3)
	}
	if _, ok := localLedger.MissingRefs()[parent.ID]; ok {
		t.Fatalf("expected missing ref to be cleared after parent arrives")
	}
}

func TestPrompt2_RejectInvalidEvent(t *testing.T) {
	p, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new persona: %v", err)
	}
	pub, priv, err := p.SignKeypair()
	if err != nil {
		t.Fatalf("sign keypair: %v", err)
	}

	ev, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       ledger.KindChatMessage,
		Payload:    json.RawMessage(`{"t":"x"}`),
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new signed event: %v", err)
	}
	ev.SigB64 = base64.RawURLEncoding.EncodeToString([]byte("bad-signature"))

	l := ledger.New()
	s := NewSync(l)
	rep, err := s.ApplyEventsMessage(EventsMessage{Events: []*ledger.Event{&ev}})
	if err != nil {
		t.Fatalf("apply events: %v", err)
	}
	if rep.Rejected != 1 {
		t.Fatalf("expected rejected=1, got %+v", rep)
	}
	if l.Have(ev.ID) {
		t.Fatalf("expected invalid event not to be stored")
	}
}

func mustDecodeID(t *testing.T, id string) []byte {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		t.Fatalf("decode id: %v", err)
	}
	return raw
}
