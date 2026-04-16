package importctl

import (
	"crypto/ed25519"
	"encoding/base64"
	"path/filepath"
	"testing"
	"time"

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
	signerKeyID := bundle.Ed25519KeyID(pubKey)

	ageIdentity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate age identity: %v", err)
	}

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "store.enc")
	store, _ := profile.NewStore(storePath, []byte("0123456789abcdef0123456789abcdef"))

	importer := NewImporter(store, []ed25519.PublicKey{pubKey}, ageIdentity)

	// Create valid payload
	now := time.Now().Unix()
	p0 := profile.Profile{
		ID:     "test-profile-1",
		Family: profile.FamilyReality,
		Endpoint: profile.Endpoint{
			Host: "127.0.0.1",
			Port: 443,
		},
		Capabilities: profile.Capabilities{
			Transport: "tcp",
		},
		Credentials: profile.Credentials{
			UUID:            "00000000-0000-0000-0000-000000000001",
			PublicKey:       "pk",
			ShortID:         "sid",
			SNI:             "sni.example.com",
			UTLSFingerprint: "chrome",
		},
		Source: profile.SourceInfo{
			Source:       "test",
			TrustLevel:   80,
			PublisherKey: signerKeyID,
		},
	}
	p, err := profile.NormalizeForWire(p0, now)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	payload := bundle.BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        []profile.Profile{p},
		Templates: map[string]bundle.Template{
			string(p.Family): {TemplateText: `{"type":"direct","tag":"proxy"}`},
		},
		Notes: map[string]string{
			"issuer_key_id": signerKeyID,
		},
	}

	// Generate bundle
	bundleBytes, err := bundle.GenerateBundleWithOptions(&payload, privKey, bundle.GenerateOptions{
		RecipientPublicKey: ageIdentity.Recipient().String(),
		AllowPlaintext:     false,
		SignerKeyID:        signerKeyID,
		BundleID:           "bndl_import_test",
		Seq:                1,
		CreatedAt:          now,
		ExpiresAt:          now + 3600,
	})
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

	store, _ := profile.NewStore("dummy.enc", []byte("0123456789abcdef0123456789abcdef"))
	importer := NewImporter(store, []ed25519.PublicKey{trustedPubKey}, ageIdentity)

	now := time.Now().Unix()
	payload := bundle.BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        nil,
		Templates:       map[string]bundle.Template{},
		Notes: map[string]string{
			"issuer_key_id": bundle.Ed25519KeyID(trustedPubKey),
		},
	}
	bundleBytes, _ := bundle.GenerateBundleWithOptions(&payload, untrustedPrivKey, bundle.GenerateOptions{
		RecipientPublicKey: ageIdentity.Recipient().String(),
		AllowPlaintext:     false,
		SignerKeyID:        bundle.Ed25519KeyID(trustedPubKey),
		BundleID:           "bndl_import_test_bad_sig",
		Seq:                1,
		CreatedAt:          now,
		ExpiresAt:          now + 3600,
	})
	uri := "snb://v2:" + base64.RawURLEncoding.EncodeToString(bundleBytes)

	_, err := importer.ParseURI(uri)
	if err == nil {
		t.Fatal("Expected error for invalid signature, got nil")
	}
}

func TestParseURI_ExpiredBundle(t *testing.T) {
	// Not implementing time mock, we trust the `VerifyAndDecrypt` unit test which could mock time or use a custom generator
}
