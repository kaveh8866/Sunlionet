package messaging

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptWithPreKey(t *testing.T) {
	recipient, err := NewX25519Keypair()
	if err != nil {
		t.Fatalf("NewX25519Keypair(recipient): %v", err)
	}
	plaintext := []byte("hello")
	env, _, err := EncryptToPreKey(plaintext, recipient.Public)
	if err != nil {
		t.Fatalf("EncryptToPreKey: %v", err)
	}
	got, _, err := DecryptWithPreKey(env, recipient.Private)
	if err != nil {
		t.Fatalf("DecryptWithPreKey: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("plaintext mismatch: %q != %q", string(got), string(plaintext))
	}
}

func TestEnvelopeEncodeDecode(t *testing.T) {
	recipient, err := NewX25519Keypair()
	if err != nil {
		t.Fatalf("NewX25519Keypair(recipient): %v", err)
	}
	env, _, err := EncryptToPreKey([]byte("x"), recipient.Public)
	if err != nil {
		t.Fatalf("EncryptToPreKey: %v", err)
	}
	enc, err := env.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	dec, err := DecodeEnvelope(enc)
	if err != nil {
		t.Fatalf("DecodeEnvelope: %v", err)
	}
	if dec.EphemeralPubB64 != env.EphemeralPubB64 {
		t.Fatalf("ephemeral pub mismatch")
	}
}
