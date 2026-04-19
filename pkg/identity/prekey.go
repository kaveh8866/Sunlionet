package identity

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/curve25519"
)

type PreKey struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
	PubB64    string `json:"pub_b64url"`
	PrivB64   string `json:"priv_b64url"`
}

func NewPreKey(ttl time.Duration) (*PreKey, error) {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	var priv [32]byte
	if _, err := io.ReadFull(rand.Reader, priv[:]); err != nil {
		return nil, err
	}
	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return nil, err
	}
	id, err := newOfferID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &PreKey{
		ID:        id,
		CreatedAt: now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
		PubB64:    base64.RawURLEncoding.EncodeToString(pub),
		PrivB64:   base64.RawURLEncoding.EncodeToString(priv[:]),
	}, nil
}

func (p *PreKey) DecodePublic() ([32]byte, error) {
	if p == nil {
		return [32]byte{}, errors.New("identity: prekey is nil")
	}
	b, err := base64.RawURLEncoding.DecodeString(p.PubB64)
	if err != nil {
		return [32]byte{}, fmt.Errorf("identity: decode prekey pub: %w", err)
	}
	if len(b) != 32 {
		return [32]byte{}, fmt.Errorf("identity: invalid prekey pub size: %d", len(b))
	}
	var out [32]byte
	copy(out[:], b)
	return out, nil
}

func (p *PreKey) DecodePrivate() ([32]byte, error) {
	if p == nil {
		return [32]byte{}, errors.New("identity: prekey is nil")
	}
	b, err := base64.RawURLEncoding.DecodeString(p.PrivB64)
	if err != nil {
		return [32]byte{}, fmt.Errorf("identity: decode prekey priv: %w", err)
	}
	if len(b) != 32 {
		return [32]byte{}, fmt.Errorf("identity: invalid prekey priv size: %d", len(b))
	}
	var out [32]byte
	copy(out[:], b)
	return out, nil
}

func (p *PreKey) Validate(now time.Time) error {
	if p == nil {
		return errors.New("identity: prekey is nil")
	}
	if p.ID == "" {
		return errors.New("identity: prekey id required")
	}
	if p.CreatedAt <= 0 || p.ExpiresAt <= 0 {
		return errors.New("identity: prekey created_at/expires_at required")
	}
	if now.Unix() > p.ExpiresAt {
		return errors.New("identity: prekey expired")
	}
	if _, err := p.DecodePublic(); err != nil {
		return err
	}
	if _, err := p.DecodePrivate(); err != nil {
		return err
	}
	return nil
}
