package mobilebridge

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"
)

func TestOnboardingURI_RoundTripReality(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	payload := OnboardingPayload{
		IssuedAtUnix:  now.Unix(),
		ExpiresAtUnix: now.Add(time.Hour).Unix(),
		Family:        OnboardingFamilyReality,
		Host:          "edge1.example.net",
		Port:          443,
		SNI:           "cdn.example.net",
		Tag:           "qr",
		CredentialA:   []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef},
		CredentialB:   make([]byte, 32),
		CredentialC:   []byte{0xaa, 0xbb, 0xcc, 0xdd},
	}
	uri, err := EncodeOnboardingURI(payload, priv)
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.TrimPrefix(uri, OnboardingURIPrefix)) > MaxOnboardingPayloadChars {
		t.Fatalf("payload too large: %d", len(strings.TrimPrefix(uri, OnboardingURIPrefix)))
	}
	parsed, err := ParseOnboardingURI(uri, []ed25519.PublicKey{pub}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Host != "edge1.example.net" || parsed.Port != 443 || parsed.Family != OnboardingFamilyReality {
		t.Fatalf("unexpected parsed payload: %+v", parsed)
	}
	prof, err := parsed.ToProfile(now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if prof.Credentials.UUID != "12345678-90ab-cdef-1234-567890abcdef" {
		t.Fatalf("unexpected uuid: %s", prof.Credentials.UUID)
	}
}

func TestOnboardingURI_RejectsTamperExpiredAndUntrusted(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	payload := OnboardingPayload{
		IssuedAtUnix:  now.Unix(),
		ExpiresAtUnix: now.Add(time.Hour).Unix(),
		Family:        OnboardingFamilyTUIC,
		Host:          "edge2.example.net",
		Port:          4433,
		SNI:           "edge2.example.net",
		CredentialA:   []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef},
		CredentialB:   []byte("secret-pass"),
	}
	uri, err := EncodeOnboardingURI(payload, priv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseOnboardingURI(uri, []ed25519.PublicKey{otherPub}, now.Add(time.Minute)); err == nil {
		t.Fatal("expected untrusted signer rejection")
	}
	if _, err := ParseOnboardingURI(uri, []ed25519.PublicKey{pub}, now.Add(2*time.Hour)); err == nil {
		t.Fatal("expected expiry rejection")
	}
	tampered := uri[:len(uri)-1] + "A"
	if _, err := ParseOnboardingURI(tampered, []ed25519.PublicKey{pub}, now.Add(time.Minute)); err == nil {
		t.Fatal("expected tamper rejection")
	}
}

func TestJoinOnboardingQRChunks(t *testing.T) {
	joined, err := JoinOnboardingQRChunks([]string{"SLQR1:2/2:def", "SLQR1:1/2:abc"})
	if err != nil {
		t.Fatal(err)
	}
	if joined != OnboardingURIPrefix+"abcdef" {
		t.Fatalf("unexpected joined uri: %s", joined)
	}
}

func TestNormalizeOnboardingTextRejectsTraversalLikeURI(t *testing.T) {
	if _, err := NormalizeOnboardingText("file:///../../state/profiles.enc"); err == nil {
		t.Fatal("expected unsupported uri")
	}
}
