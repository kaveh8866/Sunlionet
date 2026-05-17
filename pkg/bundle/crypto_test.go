package bundle

import (
	"crypto/ed25519"
	"testing"

	"filippo.io/age"
	"github.com/kaveh/sunlionet-agent/pkg/profile"
)

func TestBundleGenerateAndVerify(t *testing.T) {
	// Setup keys
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to gen ed25519: %v", err)
	}
	signerKeyID := Ed25519KeyID(pubKey)

	ageIdentity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("failed to gen age key: %v", err)
	}

	// Create payload
	payload := &BundlePayload{
		SchemaVersion: 1,
		Profiles: []profile.Profile{
			{ID: "p1", Family: profile.FamilyReality},
		},
	}

	// Generate
	bundleBytes, err := GenerateBundle(payload, ageIdentity.Recipient().String(), privKey, signerKeyID)
	if err != nil {
		t.Fatalf("GenerateBundle failed: %v", err)
	}

	// Verify
	parsed, err := VerifyAndDecrypt(bundleBytes, pubKey, ageIdentity)
	if err != nil {
		t.Fatalf("VerifyAndDecrypt failed: %v", err)
	}

	if len(parsed.Profiles) != 1 || parsed.Profiles[0].ID != "p1" {
		t.Fatalf("parsed payload mismatch: %+v", parsed)
	}
}

func TestBundleInvalidSignature(t *testing.T) {
	pubKey, _, _ := ed25519.GenerateKey(nil)
	badPubKey, badPrivKey, _ := ed25519.GenerateKey(nil)
	ageIdentity, _ := age.GenerateX25519Identity()

	payload := &BundlePayload{}
	bundleBytes, err := GenerateBundle(payload, ageIdentity.Recipient().String(), badPrivKey, Ed25519KeyID(badPubKey))
	if err != nil {
		t.Fatalf("GenerateBundle failed: %v", err)
	}

	_, err = VerifyAndDecrypt(bundleBytes, pubKey, ageIdentity)
	if err != ErrInvalidSignature {
		t.Fatalf("expected ErrInvalidSignature, got: %v", err)
	}
}

func TestBundleInvalidAgeKey(t *testing.T) {
	pubKey, privKey, _ := ed25519.GenerateKey(nil)
	signerKeyID := Ed25519KeyID(pubKey)
	ageIdentity1, _ := age.GenerateX25519Identity()
	ageIdentity2, _ := age.GenerateX25519Identity()

	payload := &BundlePayload{}
	bundleBytes, _ := GenerateBundle(payload, ageIdentity1.Recipient().String(), privKey, signerKeyID)

	_, err := VerifyAndDecrypt(bundleBytes, pubKey, ageIdentity2) // wrong age identity
	if err == nil {
		t.Fatalf("expected decryption failure with wrong age key")
	}
}
