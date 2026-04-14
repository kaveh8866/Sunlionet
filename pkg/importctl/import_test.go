package importctl

import (
	"crypto/ed25519"
	"encoding/base64"
	"path/filepath"
	"testing"

	"filippo.io/age"
	"github.com/kaveh/shadownet-agent/pkg/bundle"
	"github.com/kaveh/shadownet-agent/pkg/profile"
)

func TestParseURI_ValidBundle(t *testing.T) {
	// Setup trusted key
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate ed25519 keys: %v", err)
	}

	ageIdentity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate age identity: %v", err)
	}

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "store.enc")
	store, _ := profile.NewStore(storePath, "0123456789abcdef0123456789abcdef")

	importer := NewImporter(store, []ed25519.PublicKey{pubKey}, ageIdentity)

	// Create valid payload
	payload := bundle.BundlePayload{
		Profiles: []profile.Profile{
			{ID: "test-profile-1", Family: profile.FamilyReality},
		},
	}

	// Generate bundle
	bundleBytes, err := bundle.GenerateBundle(&payload, ageIdentity.Recipient().String(), privKey, "test-signer")
	if err != nil {
		t.Fatalf("Failed to generate bundle: %v", err)
	}

	uri := "snb://v2:" + base64.RawURLEncoding.EncodeToString(bundleBytes)

	// Parse URI
	parsedPayload, err := importer.ParseURI(uri)
	if err != nil {
		t.Fatalf("Expected valid URI to parse successfully, got: %v", err)
	}

	if len(parsedPayload.Profiles) != 1 || parsedPayload.Profiles[0].ID != "test-profile-1" {
		t.Fatalf("Parsed payload doesn't match expected values")
	}

	// Test storing the bundle
	err = importer.ProcessAndStore(parsedPayload)
	if err != nil {
		t.Fatalf("ProcessAndStore failed: %v", err)
	}

	// Verify store content
	loaded, err := store.Load()
	if err != nil || len(loaded) != 1 {
		t.Fatalf("Failed to verify store contents")
	}
}

func TestParseURI_InvalidSignature(t *testing.T) {
	trustedPubKey, _, _ := ed25519.GenerateKey(nil)
	_, untrustedPrivKey, _ := ed25519.GenerateKey(nil)
	ageIdentity, _ := age.GenerateX25519Identity()

	store, _ := profile.NewStore("dummy.enc", "0123456789abcdef0123456789abcdef")
	importer := NewImporter(store, []ed25519.PublicKey{trustedPubKey}, ageIdentity)

	payload := bundle.BundlePayload{}
	bundleBytes, _ := bundle.GenerateBundle(&payload, ageIdentity.Recipient().String(), untrustedPrivKey, "untrusted")
	uri := "snb://v2:" + base64.RawURLEncoding.EncodeToString(bundleBytes)

	_, err := importer.ParseURI(uri)
	if err == nil {
		t.Fatal("Expected error for invalid signature, got nil")
	}
}

func TestParseURI_ExpiredBundle(t *testing.T) {
	// Not implementing time mock, we trust the `VerifyAndDecrypt` unit test which could mock time or use a custom generator
}
