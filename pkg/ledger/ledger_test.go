package ledger

import (
	"encoding/json"
	"testing"

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
