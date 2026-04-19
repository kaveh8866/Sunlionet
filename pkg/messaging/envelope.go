package messaging

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const EnvelopeV1 = 1

type Envelope struct {
	Version         int    `json:"v"`
	CreatedAt       int64  `json:"created_at"`
	ToMailbox       string `json:"to_mailbox,omitempty"`
	EphemeralPubB64 string `json:"eph_pub_b64url"`
	NonceB64        string `json:"nonce_b64url"`
	CiphertextB64   string `json:"ct_b64url"`
}

func EncryptToPreKey(plaintext []byte, recipientPreKeyPub [32]byte) (*Envelope, [32]byte, error) {
	sender, err := NewX25519Keypair()
	if err != nil {
		return nil, [32]byte{}, err
	}
	return EncryptToPreKeyWithSender(plaintext, sender.Private, sender.Public, recipientPreKeyPub)
}

func EncryptToPreKeyWithSender(plaintext []byte, senderPriv [32]byte, senderPub [32]byte, recipientPreKeyPub [32]byte) (*Envelope, [32]byte, error) {
	key, err := DeriveSharedKey(senderPriv, recipientPreKeyPub, "shadownet-msg-v1")
	if err != nil {
		return nil, [32]byte{}, err
	}
	aead, err := NewAEAD(key)
	if err != nil {
		return nil, [32]byte{}, err
	}
	nonce, err := RandomNonce24()
	if err != nil {
		return nil, [32]byte{}, err
	}
	ct, err := Encrypt(aead, nonce, plaintext, senderPub[:])
	if err != nil {
		return nil, [32]byte{}, err
	}
	return &Envelope{
		Version:         EnvelopeV1,
		CreatedAt:       time.Now().Unix(),
		EphemeralPubB64: base64.RawURLEncoding.EncodeToString(senderPub[:]),
		NonceB64:        base64.RawURLEncoding.EncodeToString(nonce[:]),
		CiphertextB64:   base64.RawURLEncoding.EncodeToString(ct),
	}, senderPub, nil
}

func DecryptWithPreKey(env *Envelope, recipientPreKeyPriv [32]byte) ([]byte, [32]byte, error) {
	if env == nil {
		return nil, [32]byte{}, errors.New("messaging: envelope is nil")
	}
	if env.Version != EnvelopeV1 {
		return nil, [32]byte{}, fmt.Errorf("messaging: unsupported envelope version: %d", env.Version)
	}
	ephPubBytes, err := base64.RawURLEncoding.DecodeString(env.EphemeralPubB64)
	if err != nil {
		return nil, [32]byte{}, fmt.Errorf("messaging: decode eph_pub_b64url: %w", err)
	}
	if len(ephPubBytes) != 32 {
		return nil, [32]byte{}, fmt.Errorf("messaging: invalid eph pub size: %d", len(ephPubBytes))
	}
	var ephPub [32]byte
	copy(ephPub[:], ephPubBytes)

	key, err := DeriveSharedKey(recipientPreKeyPriv, ephPub, "shadownet-msg-v1")
	if err != nil {
		return nil, [32]byte{}, err
	}
	aead, err := NewAEAD(key)
	if err != nil {
		return nil, [32]byte{}, err
	}
	nonceBytes, err := base64.RawURLEncoding.DecodeString(env.NonceB64)
	if err != nil {
		return nil, [32]byte{}, fmt.Errorf("messaging: decode nonce_b64url: %w", err)
	}
	if len(nonceBytes) != 24 {
		return nil, [32]byte{}, fmt.Errorf("messaging: invalid nonce size: %d", len(nonceBytes))
	}
	var nonce [24]byte
	copy(nonce[:], nonceBytes)
	ct, err := base64.RawURLEncoding.DecodeString(env.CiphertextB64)
	if err != nil {
		return nil, [32]byte{}, fmt.Errorf("messaging: decode ct_b64url: %w", err)
	}
	pt, err := Decrypt(aead, nonce, ct, ephPub[:])
	if err != nil {
		return nil, [32]byte{}, err
	}
	return pt, ephPub, nil
}

func (e *Envelope) Encode() (string, error) {
	if e == nil {
		return "", errors.New("messaging: envelope is nil")
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeEnvelope(s string) (*Envelope, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("messaging: decode envelope: %w", err)
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Version != EnvelopeV1 {
		return nil, fmt.Errorf("messaging: unsupported envelope version: %d", env.Version)
	}
	return &env, nil
}
