package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type DeviceTrust string

const (
	DeviceUnverified DeviceTrust = "unverified"
	DeviceTrusted    DeviceTrust = "trusted"
	DeviceRevoked    DeviceTrust = "revoked"
)

type Device struct {
	PersonaID   PersonaID   `json:"persona_id"`
	DeviceID    string      `json:"device_id"`
	CreatedAt   int64       `json:"created_at"`
	RevokedAt   int64       `json:"revoked_at,omitempty"`
	Trust       DeviceTrust `json:"trust"`
	Label       string      `json:"label,omitempty"`
	SignPubB64  string      `json:"sign_pub_b64url"`
	SignPrivB64 string      `json:"sign_priv_b64url"`
}

func NewDevice(personaID PersonaID, label string) (*Device, error) {
	if personaID == "" {
		return nil, errors.New("identity: persona_id required")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	id, err := newDeviceID()
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	return &Device{
		PersonaID:   personaID,
		DeviceID:    id,
		CreatedAt:   now,
		Trust:       DeviceUnverified,
		Label:       label,
		SignPubB64:  base64.RawURLEncoding.EncodeToString(pub),
		SignPrivB64: base64.RawURLEncoding.EncodeToString(priv),
	}, nil
}

func (d *Device) Validate() error {
	if d == nil {
		return errors.New("identity: device is nil")
	}
	if d.PersonaID == "" {
		return errors.New("identity: device persona_id required")
	}
	if d.DeviceID == "" {
		return errors.New("identity: device_id required")
	}
	if d.CreatedAt <= 0 {
		return errors.New("identity: device created_at required")
	}
	switch d.Trust {
	case DeviceUnverified, DeviceTrusted, DeviceRevoked:
	default:
		return fmt.Errorf("identity: invalid device trust: %q", d.Trust)
	}
	_, _, err := d.SignKeypair()
	return err
}

func (d *Device) SignKeypair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	if d == nil {
		return nil, nil, errors.New("identity: device is nil")
	}
	pub, err := base64.RawURLEncoding.DecodeString(d.SignPubB64)
	if err != nil {
		return nil, nil, fmt.Errorf("identity: decode device sign_pub_b64url: %w", err)
	}
	priv, err := base64.RawURLEncoding.DecodeString(d.SignPrivB64)
	if err != nil {
		return nil, nil, fmt.Errorf("identity: decode device sign_priv_b64url: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return nil, nil, fmt.Errorf("identity: invalid device public key size: %d", len(pub))
	}
	if len(priv) != ed25519.PrivateKeySize {
		return nil, nil, fmt.Errorf("identity: invalid device private key size: %d", len(priv))
	}
	return ed25519.PublicKey(pub), ed25519.PrivateKey(priv), nil
}

type DeviceCert struct {
	PersonaID    PersonaID `json:"persona_id"`
	DeviceID     string    `json:"device_id"`
	DevicePubB64 string    `json:"device_pub_b64url"`
	IssuedAt     int64     `json:"issued_at"`
	NonceB64     string    `json:"nonce_b64url"`
	SigB64       string    `json:"sig_b64url"`
}

type deviceCertSignable struct {
	PersonaID    PersonaID `json:"persona_id"`
	DeviceID     string    `json:"device_id"`
	DevicePubB64 string    `json:"device_pub_b64url"`
	IssuedAt     int64     `json:"issued_at"`
	NonceB64     string    `json:"nonce_b64url"`
}

func IssueDeviceCert(persona *Persona, deviceID string, devicePubB64 string) (*DeviceCert, error) {
	if persona == nil {
		return nil, errors.New("identity: persona is nil")
	}
	if strings.TrimSpace(deviceID) == "" {
		return nil, errors.New("identity: device_id required")
	}
	if devicePubB64 == "" {
		return nil, errors.New("identity: device_pub_b64url required")
	}
	pub, priv, err := persona.SignKeypair()
	if err != nil {
		return nil, err
	}
	if len(pub) != ed25519.PublicKeySize || len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("identity: invalid persona signing keypair")
	}
	b, err := base64.RawURLEncoding.DecodeString(devicePubB64)
	if err != nil {
		return nil, fmt.Errorf("identity: decode device_pub_b64url: %w", err)
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("identity: invalid device pub size: %d", len(b))
	}
	nonce, err := newNonceB64(24)
	if err != nil {
		return nil, err
	}
	signable := deviceCertSignable{
		PersonaID:    persona.ID,
		DeviceID:     strings.TrimSpace(deviceID),
		DevicePubB64: devicePubB64,
		IssuedAt:     time.Now().Unix(),
		NonceB64:     nonce,
	}
	raw, err := json.Marshal(signable)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(priv, raw)
	return &DeviceCert{
		PersonaID:    signable.PersonaID,
		DeviceID:     signable.DeviceID,
		DevicePubB64: signable.DevicePubB64,
		IssuedAt:     signable.IssuedAt,
		NonceB64:     signable.NonceB64,
		SigB64:       base64.RawURLEncoding.EncodeToString(sig),
	}, nil
}

func VerifyDeviceCert(personaPubB64 string, cert *DeviceCert) error {
	if cert == nil {
		return errors.New("identity: cert is nil")
	}
	if strings.TrimSpace(personaPubB64) == "" {
		return errors.New("identity: persona pub required")
	}
	pub, err := base64.RawURLEncoding.DecodeString(personaPubB64)
	if err != nil {
		return fmt.Errorf("identity: decode persona pub: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("identity: invalid persona pub size: %d", len(pub))
	}
	if cert.PersonaID == "" || strings.TrimSpace(cert.DeviceID) == "" || cert.DevicePubB64 == "" || cert.NonceB64 == "" || cert.SigB64 == "" || cert.IssuedAt <= 0 {
		return errors.New("identity: malformed cert")
	}
	sig, err := base64.RawURLEncoding.DecodeString(cert.SigB64)
	if err != nil {
		return fmt.Errorf("identity: decode sig: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("identity: invalid sig size: %d", len(sig))
	}
	signable := deviceCertSignable{
		PersonaID:    cert.PersonaID,
		DeviceID:     cert.DeviceID,
		DevicePubB64: cert.DevicePubB64,
		IssuedAt:     cert.IssuedAt,
		NonceB64:     cert.NonceB64,
	}
	raw, err := json.Marshal(signable)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), raw, sig) {
		return errors.New("identity: invalid device cert signature")
	}
	return nil
}

func newDeviceID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
