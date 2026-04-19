package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	OfferV1 = 1
	OfferV2 = 2
)

type ContactOffer struct {
	Version      int       `json:"v"`
	OfferID      string    `json:"id"`
	CreatedAt    int64     `json:"created_at"`
	ExpiresAt    int64     `json:"expires_at"`
	PersonaID    PersonaID `json:"persona_id"`
	PersonaPub   string    `json:"persona_pub_b64url"`
	PreKeyPub    string    `json:"prekey_pub_b64url"`
	Mailbox      string    `json:"mailbox,omitempty"`
	RelayHints   []string  `json:"relay_hints,omitempty"`
	NonceB64     string    `json:"nonce_b64url"`
	SignatureB64 string    `json:"sig_b64url"`
}

type contactOfferSignable struct {
	Version    int       `json:"v"`
	OfferID    string    `json:"id"`
	CreatedAt  int64     `json:"created_at"`
	ExpiresAt  int64     `json:"expires_at"`
	PersonaID  PersonaID `json:"persona_id"`
	PersonaPub string    `json:"persona_pub_b64url"`
	PreKeyPub  string    `json:"prekey_pub_b64url"`
	Mailbox    string    `json:"mailbox,omitempty"`
	RelayHints []string  `json:"relay_hints,omitempty"`
	NonceB64   string    `json:"nonce_b64url"`
}

func NewContactOffer(p *Persona, preKeyPubB64 string, relayHints []string, ttl time.Duration) (*ContactOffer, error) {
	return newContactOffer(p, preKeyPubB64, "", relayHints, ttl, OfferV1)
}

func NewContactOfferV2(p *Persona, preKeyPubB64 string, mailbox string, relayHints []string, ttl time.Duration) (*ContactOffer, error) {
	return newContactOffer(p, preKeyPubB64, mailbox, relayHints, ttl, OfferV2)
}

func newContactOffer(p *Persona, preKeyPubB64 string, mailbox string, relayHints []string, ttl time.Duration, version int) (*ContactOffer, error) {
	if p == nil {
		return nil, errors.New("identity: persona is nil")
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	pub, priv, err := p.SignKeypair()
	if err != nil {
		return nil, err
	}
	if len(pub) != ed25519.PublicKeySize || len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("identity: invalid persona signing keypair")
	}
	if preKeyPubB64 == "" {
		return nil, errors.New("identity: missing prekey_pub_b64url")
	}
	preKeyPub, err := base64.RawURLEncoding.DecodeString(preKeyPubB64)
	if err != nil {
		return nil, fmt.Errorf("identity: decode prekey_pub_b64url: %w", err)
	}
	if len(preKeyPub) != 32 {
		return nil, fmt.Errorf("identity: invalid prekey_pub size: %d", len(preKeyPub))
	}
	if version == OfferV2 && mailbox == "" {
		return nil, errors.New("identity: missing mailbox")
	}

	offerID, err := newOfferID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	nonceB64, err := newNonceB64(24)
	if err != nil {
		return nil, err
	}

	signable := contactOfferSignable{
		Version:    version,
		OfferID:    offerID,
		CreatedAt:  now.Unix(),
		ExpiresAt:  now.Add(ttl).Unix(),
		PersonaID:  p.ID,
		PersonaPub: p.SignPubB64,
		PreKeyPub:  preKeyPubB64,
		Mailbox:    mailbox,
		RelayHints: append([]string(nil), relayHints...),
		NonceB64:   nonceB64,
	}
	msg, err := json.Marshal(signable)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(priv, msg)

	return &ContactOffer{
		Version:      signable.Version,
		OfferID:      signable.OfferID,
		CreatedAt:    signable.CreatedAt,
		ExpiresAt:    signable.ExpiresAt,
		PersonaID:    signable.PersonaID,
		PersonaPub:   signable.PersonaPub,
		PreKeyPub:    signable.PreKeyPub,
		Mailbox:      signable.Mailbox,
		RelayHints:   signable.RelayHints,
		NonceB64:     signable.NonceB64,
		SignatureB64: base64.RawURLEncoding.EncodeToString(sig),
	}, nil
}

func (o *ContactOffer) Validate(now time.Time) error {
	if o == nil {
		return errors.New("identity: offer is nil")
	}
	if o.Version != OfferV1 && o.Version != OfferV2 {
		return fmt.Errorf("identity: unsupported offer version: %d", o.Version)
	}
	if o.OfferID == "" {
		return errors.New("identity: offer id required")
	}
	if o.PersonaID == "" {
		return errors.New("identity: persona_id required")
	}
	if o.CreatedAt <= 0 || o.ExpiresAt <= 0 {
		return errors.New("identity: created_at/expires_at required")
	}
	if now.Unix() > o.ExpiresAt {
		return errors.New("identity: offer expired")
	}
	if o.PersonaPub == "" || o.PreKeyPub == "" || o.NonceB64 == "" || o.SignatureB64 == "" {
		return errors.New("identity: missing required offer fields")
	}
	if o.Version == OfferV2 && o.Mailbox == "" {
		return errors.New("identity: mailbox required")
	}
	pub, err := base64.RawURLEncoding.DecodeString(o.PersonaPub)
	if err != nil {
		return fmt.Errorf("identity: decode persona_pub_b64url: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("identity: invalid persona_pub size: %d", len(pub))
	}
	preKeyPub, err := base64.RawURLEncoding.DecodeString(o.PreKeyPub)
	if err != nil {
		return fmt.Errorf("identity: decode prekey_pub_b64url: %w", err)
	}
	if len(preKeyPub) != 32 {
		return fmt.Errorf("identity: invalid prekey_pub size: %d", len(preKeyPub))
	}
	sig, err := base64.RawURLEncoding.DecodeString(o.SignatureB64)
	if err != nil {
		return fmt.Errorf("identity: decode sig_b64url: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("identity: invalid signature size: %d", len(sig))
	}

	signable := contactOfferSignable{
		Version:    o.Version,
		OfferID:    o.OfferID,
		CreatedAt:  o.CreatedAt,
		ExpiresAt:  o.ExpiresAt,
		PersonaID:  o.PersonaID,
		PersonaPub: o.PersonaPub,
		PreKeyPub:  o.PreKeyPub,
		Mailbox:    o.Mailbox,
		RelayHints: append([]string(nil), o.RelayHints...),
		NonceB64:   o.NonceB64,
	}
	msg, err := json.Marshal(signable)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), msg, sig) {
		return errors.New("identity: invalid offer signature")
	}
	return nil
}

func (o *ContactOffer) Encode() (string, error) {
	if o == nil {
		return "", errors.New("identity: offer is nil")
	}
	raw, err := json.Marshal(o)
	if err != nil {
		return "", err
	}
	return "sn4:" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeContactOffer(s string) (*ContactOffer, error) {
	if s == "" {
		return nil, errors.New("identity: offer string is empty")
	}
	if len(s) < 4 || s[:4] != "sn4:" {
		return nil, errors.New("identity: invalid offer prefix")
	}
	raw, err := base64.RawURLEncoding.DecodeString(s[4:])
	if err != nil {
		return nil, fmt.Errorf("identity: decode offer: %w", err)
	}
	var offer ContactOffer
	if err := json.Unmarshal(raw, &offer); err != nil {
		return nil, err
	}
	if err := offer.Validate(time.Now()); err != nil {
		return nil, err
	}
	return &offer, nil
}

func newOfferID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func newNonceB64(n int) (string, error) {
	if n <= 0 {
		return "", errors.New("identity: nonce size must be positive")
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
