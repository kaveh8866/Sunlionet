package ledger

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/kaveh/sunlionet-agent/pkg/identity"
)

func TestPrompt1_ValidEventChain(t *testing.T) {
	l := New()
	p, pub, priv := mustPersonaSigner(t)

	ev1 := mustSignedEvent(t, SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       KindChatMessage,
		Payload:    json.RawMessage(`{"text":"hi"}`),
	})
	if err := l.AddEvent(&ev1); err != nil {
		t.Fatalf("add ev1: %v", err)
	}

	ev2 := mustSignedEvent(t, SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        2,
		Prev:       ev1.ID,
		Parents:    []string{ev1.ID},
		Kind:       KindGroupCreate,
		Payload:    json.RawMessage(`{"group":"g-1"}`),
	})
	if err := l.AddEvent(&ev2); err != nil {
		t.Fatalf("add ev2: %v", err)
	}

	got, ok := l.GetEvent(mustDecodeID(t, ev2.ID))
	if !ok {
		t.Fatalf("expected ev2 to be retrievable by raw id")
	}
	if got.ID != ev2.ID {
		t.Fatalf("unexpected event id: got %q want %q", got.ID, ev2.ID)
	}

	heads := l.GetHeads()
	if len(heads) != 1 {
		t.Fatalf("expected 1 head, got %d", len(heads))
	}
	if base64.RawURLEncoding.EncodeToString(heads[0]) != ev2.ID {
		t.Fatalf("head mismatch")
	}
}

func TestPrompt1_InvalidSignature(t *testing.T) {
	l := New()
	p, pub, priv := mustPersonaSigner(t)

	ev := mustSignedEvent(t, SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       KindChatMessage,
		Payload:    json.RawMessage(`{"text":"tamper-sig"}`),
	})
	ev.SigB64 = base64.RawURLEncoding.EncodeToString([]byte("bad-signature"))
	if err := l.AddEvent(&ev); err == nil {
		t.Fatalf("expected signature validation error")
	}
}

func TestPrompt1_WrongHash(t *testing.T) {
	l := New()
	p, pub, priv := mustPersonaSigner(t)

	ev := mustSignedEvent(t, SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       KindChatMessage,
		Payload:    json.RawMessage(`{"text":"tamper-id"}`),
	})
	ev.ID = base64.RawURLEncoding.EncodeToString([]byte("wrong-id"))
	if err := l.AddEvent(&ev); err == nil {
		t.Fatalf("expected id/hash mismatch error")
	}
}

func TestPrompt1_PrevViolation(t *testing.T) {
	l := New()
	p, pub, priv := mustPersonaSigner(t)

	ev1 := mustSignedEvent(t, SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       KindChatMessage,
		Payload:    json.RawMessage(`{"text":"start"}`),
	})
	if err := l.AddEvent(&ev1); err != nil {
		t.Fatalf("add ev1: %v", err)
	}

	ev2 := mustSignedEvent(t, SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        2,
		Prev:       base64.RawURLEncoding.EncodeToString([]byte("unknown-prev")),
		Kind:       KindGroupJoin,
		Payload:    json.RawMessage(`{"group":"g-1"}`),
	})
	if err := l.AddEvent(&ev2); err == nil {
		t.Fatalf("expected prev chain violation")
	}
}

func TestPrompt1_DuplicateEvent(t *testing.T) {
	l := New()
	p, pub, priv := mustPersonaSigner(t)

	ev := mustSignedEvent(t, SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       KindChatMessage,
		Payload:    json.RawMessage(`{"text":"dedupe"}`),
	})
	if err := l.AddEvent(&ev); err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	err := l.AddEvent(&ev)
	if !errors.Is(err, ErrDuplicateEvent) {
		t.Fatalf("expected ErrDuplicateEvent, got %v", err)
	}
}

func mustPersonaSigner(t *testing.T) (*identity.Persona, []byte, []byte) {
	t.Helper()
	p, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new persona: %v", err)
	}
	pub, priv, err := p.SignKeypair()
	if err != nil {
		t.Fatalf("sign keypair: %v", err)
	}
	return p, pub, priv
}

func mustSignedEvent(t *testing.T, in SignedEventInput) Event {
	t.Helper()
	ev, err := NewSignedEvent(in)
	if err != nil {
		t.Fatalf("new signed event: %v", err)
	}
	return ev
}

func mustDecodeID(t *testing.T, id string) []byte {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		t.Fatalf("decode id: %v", err)
	}
	return raw
}
