package integration

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/kaveh/shadownet-agent/pkg/bundle"
	"github.com/kaveh/shadownet-agent/pkg/importctl"
	"github.com/kaveh/shadownet-agent/pkg/profile"
)

func TestOutsideToInside_BundleRoundTrip_RevocationsApplied(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "store.enc")
	store, err := profile.NewStore(storePath, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	existing := []profile.Profile{
		{ID: "reality_01_a", Family: profile.FamilyReality, Enabled: true},
		{ID: "keep_01", Family: profile.FamilyTUIC, Enabled: true},
	}
	if err := store.Save(existing); err != nil {
		t.Fatalf("save existing: %v", err)
	}

	recipientIdentity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("age identity: %v", err)
	}
	recipientPub := recipientIdentity.Recipient().String()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("ed25519: %v", err)
	}
	signerKeyID := bundle.Ed25519KeyID(pub)
	now := time.Now().Unix()

	r0, err := profile.NormalizeForWire(profile.Profile{
		ID:     "new_01",
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
	}, now)
	if err != nil {
		t.Fatalf("normalize reality: %v", err)
	}
	h0, err := profile.NormalizeForWire(profile.Profile{
		ID:     "new_02",
		Family: profile.FamilyHysteria2,
		Endpoint: profile.Endpoint{
			Host: "127.0.0.1",
			Port: 8443,
		},
		Capabilities: profile.Capabilities{
			Transport: "udp",
		},
		Credentials: profile.Credentials{
			Password:     "pw",
			ObfsPassword: "obfs",
			SNI:          "sni.example.com",
		},
		Source: profile.SourceInfo{
			Source:       "test",
			TrustLevel:   80,
			PublisherKey: signerKeyID,
		},
	}, now)
	if err != nil {
		t.Fatalf("normalize hysteria2: %v", err)
	}

	payload := &bundle.BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        []profile.Profile{r0, h0},
		Revocations:     []string{"reality_01_a"},
		Templates: map[string]bundle.Template{
			string(profile.FamilyReality):   {TemplateText: `{"type":"direct","tag":"proxy"}`},
			string(profile.FamilyHysteria2): {TemplateText: `{"type":"direct","tag":"proxy"}`},
		},
		Notes: map[string]string{
			"issuer_key_id": signerKeyID,
		},
	}

	bundleBytes, err := bundle.GenerateBundleWithOptions(payload, priv, bundle.GenerateOptions{
		RecipientPublicKey: recipientPub,
		AllowPlaintext:     false,
		SignerKeyID:        signerKeyID,
		BundleID:           "bndl_roundtrip",
		Seq:                1,
		CreatedAt:          now,
		ExpiresAt:          now + 3600,
	})
	if err != nil {
		t.Fatalf("generate bundle: %v", err)
	}

	uri := "snb://v2:" + base64.RawURLEncoding.EncodeToString(bundleBytes)
	importer := importctl.NewImporter(store, []ed25519.PublicKey{pub}, recipientIdentity)

	decoded, err := importer.ParseURI(uri)
	if err != nil {
		t.Fatalf("parse uri: %v", err)
	}
	if err := importer.ProcessAndStore(decoded); err != nil {
		t.Fatalf("process: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	seen := map[string]bool{}
	for _, p := range got {
		seen[p.ID] = true
	}

	if seen["reality_01_a"] {
		t.Fatalf("expected revoked profile to be removed")
	}
	if !seen["keep_01"] || !seen["new_01"] || !seen["new_02"] {
		t.Fatalf("expected keep_01 + new_01 + new_02 to exist, got: %#v", seen)
	}
}

func TestOutsideToInside_BundleTamper_FailsVerification(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "store.enc")
	store, err := profile.NewStore(storePath, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	recipientIdentity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("age identity: %v", err)
	}
	recipientPub := recipientIdentity.Recipient().String()

	trustedPub, trustedPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("ed25519: %v", err)
	}
	signerKeyID := bundle.Ed25519KeyID(trustedPub)
	now := time.Now().Unix()

	r0, err := profile.NormalizeForWire(profile.Profile{
		ID:     "new_01",
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
	}, now)
	if err != nil {
		t.Fatalf("normalize reality: %v", err)
	}

	payload := &bundle.BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        []profile.Profile{r0},
		Revocations:     nil,
		Templates: map[string]bundle.Template{
			string(profile.FamilyReality): {TemplateText: `{"type":"direct","tag":"proxy"}`},
		},
		Notes: map[string]string{
			"issuer_key_id": signerKeyID,
		},
	}

	bundleBytes, err := bundle.GenerateBundleWithOptions(payload, trustedPriv, bundle.GenerateOptions{
		RecipientPublicKey: recipientPub,
		AllowPlaintext:     false,
		SignerKeyID:        signerKeyID,
		BundleID:           "bndl_tamper",
		Seq:                1,
		CreatedAt:          now,
		ExpiresAt:          now + 3600,
	})
	if err != nil {
		t.Fatalf("generate bundle: %v", err)
	}

	bundleBytes[len(bundleBytes)-1] ^= 0x01
	uri := "snb://v2:" + base64.RawURLEncoding.EncodeToString(bundleBytes)

	importer := importctl.NewImporter(store, []ed25519.PublicKey{trustedPub}, recipientIdentity)
	_, err = importer.ParseURI(uri)
	if err == nil {
		t.Fatalf("expected verification failure, got nil")
	}
	_ = os.Remove(storePath)
}
