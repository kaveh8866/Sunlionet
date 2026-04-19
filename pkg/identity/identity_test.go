package identity

import (
	"testing"
	"time"
)

func TestPersonaRoundTripKeys(t *testing.T) {
	p, err := NewPersona()
	if err != nil {
		t.Fatalf("NewPersona: %v", err)
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	_, _, err = p.SignKeypair()
	if err != nil {
		t.Fatalf("SignKeypair: %v", err)
	}
}

func TestContactOfferEncodeDecode(t *testing.T) {
	p, err := NewPersona()
	if err != nil {
		t.Fatalf("NewPersona: %v", err)
	}
	preKey, err := NewPreKey(24 * time.Hour)
	if err != nil {
		t.Fatalf("NewPreKey: %v", err)
	}
	offer, err := NewContactOffer(p, preKey.PubB64, []string{"relay1"}, 2*time.Minute)
	if err != nil {
		t.Fatalf("NewContactOffer: %v", err)
	}
	encoded, err := offer.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := DecodeContactOffer(encoded)
	if err != nil {
		t.Fatalf("DecodeContactOffer: %v", err)
	}
	if decoded.OfferID != offer.OfferID {
		t.Fatalf("OfferID mismatch: %q != %q", decoded.OfferID, offer.OfferID)
	}
	if decoded.PersonaID != offer.PersonaID {
		t.Fatalf("PersonaID mismatch: %q != %q", decoded.PersonaID, offer.PersonaID)
	}
}

func TestMailboxBindingRotation(t *testing.T) {
	s := NewState()
	p, err := NewPersona()
	if err != nil {
		t.Fatalf("NewPersona: %v", err)
	}
	s.Personas = append(s.Personas, *p)
	b, created, err := s.EnsureMailboxBinding(p.ID)
	if err != nil {
		t.Fatalf("EnsureMailboxBinding: %v", err)
	}
	if !created {
		t.Fatalf("expected created=true")
	}
	t0 := time.Unix(1700000000, 0)
	m1, next, err := b.MailboxAt(t0, 60)
	if err != nil {
		t.Fatalf("MailboxAt: %v", err)
	}
	if m1 == "" {
		t.Fatalf("expected mailbox id")
	}
	if next.Before(t0) || next.Equal(t0) {
		t.Fatalf("expected next rotation in the future")
	}
	m2, _, err := b.MailboxAt(t0.Add(2*time.Minute), 60)
	if err != nil {
		t.Fatalf("MailboxAt: %v", err)
	}
	if m2 == m1 {
		t.Fatalf("expected mailbox to rotate")
	}
}

func TestContactOfferV2IncludesMailbox(t *testing.T) {
	p, err := NewPersona()
	if err != nil {
		t.Fatalf("NewPersona: %v", err)
	}
	preKey, err := NewPreKey(24 * time.Hour)
	if err != nil {
		t.Fatalf("NewPreKey: %v", err)
	}
	offer, err := NewContactOfferV2(p, preKey.PubB64, "mbx", []string{"relay1"}, 2*time.Minute)
	if err != nil {
		t.Fatalf("NewContactOfferV2: %v", err)
	}
	encoded, err := offer.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := DecodeContactOffer(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Mailbox != "mbx" {
		t.Fatalf("Mailbox mismatch: %q != %q", decoded.Mailbox, "mbx")
	}
}
