package security

import (
	"crypto/ed25519"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/kaveh/shadownet-agent/pkg/bundle"
	"github.com/kaveh/shadownet-agent/pkg/importctl"
	"github.com/kaveh/shadownet-agent/pkg/profile"
	"github.com/kaveh/shadownet-agent/pkg/sbctl"
)

func TestInvalidBundleRejected(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	id, _ := age.GenerateX25519Identity()
	store, _ := profile.NewStore(filepath.Join(t.TempDir(), "profiles.enc"), []byte("0123456789abcdef0123456789abcdef"))
	imp := importctl.NewImporter(store, []ed25519.PublicKey{pub}, id)
	if _, err := imp.ParseBytes([]byte("{invalid-json")); err == nil || !strings.Contains(err.Error(), "bundle invalid") {
		t.Fatalf("expected strict invalid bundle error, got %v", err)
	}
}

func TestTamperedSignatureRejected(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	signerID := bundle.Ed25519KeyID(pub)
	id, _ := age.GenerateX25519Identity()
	store, _ := profile.NewStore(filepath.Join(t.TempDir(), "profiles.enc"), []byte("0123456789abcdef0123456789abcdef"))
	imp := importctl.NewImporter(store, []ed25519.PublicKey{pub}, id)
	now := time.Now().Unix()
	p := mustValidProfile(t, signerID, now)
	raw := mustBundle(t, &bundle.BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        []profile.Profile{p},
		Templates: map[string]bundle.Template{
			string(p.Family): {TemplateText: `{"type":"direct","tag":"proxy"}`},
		},
		Notes: map[string]string{"issuer_key_id": signerID},
	}, priv, id.Recipient().String(), signerID, "bundle-tamper", now)

	var wrapper map[string]any
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatalf("unmarshal wrapper: %v", err)
	}
	header := wrapper["header"].(map[string]any)
	header["sig"] = header["sig"].(string) + "A"
	tampered, _ := json.Marshal(wrapper)
	if _, err := imp.ParseBytes(tampered); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("expected signature mismatch, got %v", err)
	}
}

func TestMalformedConfigAndInjectionRejected(t *testing.T) {
	now := time.Now().Unix()
	p := mustValidProfile(t, "ed25519:testsigner", now)
	if _, err := sbctl.RenderConfigToFile(p, `{"type":"socks","tag":"proxy","server":"example.com","server_port":1080}`, filepath.Join(t.TempDir(), "cfg.json")); err == nil {
		t.Fatalf("expected unknown outbound type rejection")
	}
	if _, err := sbctl.RenderConfigToFile(p, `{"type":"vless",`, filepath.Join(t.TempDir(), "cfg2.json")); err == nil {
		t.Fatalf("expected malformed template rejection")
	}
}

func TestInvalidProfileBundleRejected(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	signerID := bundle.Ed25519KeyID(pub)
	id, _ := age.GenerateX25519Identity()
	store, _ := profile.NewStore(filepath.Join(t.TempDir(), "profiles.enc"), []byte("0123456789abcdef0123456789abcdef"))
	imp := importctl.NewImporter(store, []ed25519.PublicKey{pub}, id)
	now := time.Now().Unix()
	bad := profile.Profile{
		ID:     "bad-profile",
		Family: profile.FamilyReality,
		Endpoint: profile.Endpoint{
			Host: "example.com",
			Port: 70000,
		},
		Capabilities: profile.Capabilities{Transport: "tcp"},
		Credentials: profile.Credentials{
			UUID:            "",
			PublicKey:       "pk",
			ShortID:         "sid",
			SNI:             "example.com",
			UTLSFingerprint: "chrome",
		},
		Source:  profile.SourceInfo{Source: "bundle", TrustLevel: 80, PublisherKey: signerID, ImportedAt: now},
		Enabled: true,
	}
	raw := mustBundle(t, &bundle.BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        []profile.Profile{bad},
		Templates: map[string]bundle.Template{
			string(profile.FamilyReality): {TemplateText: `{"type":"vless","tag":"proxy","server":"{{.Endpoint.Host}}","server_port":{{.Endpoint.Port}}}`},
		},
		Notes: map[string]string{"issuer_key_id": signerID},
	}, priv, id.Recipient().String(), signerID, "bundle-invalid-profile", now)
	if _, err := imp.ParseBytes(raw); err == nil || !strings.Contains(err.Error(), "bundle invalid") {
		t.Fatalf("expected invalid profile rejection, got %v", err)
	}
}

func TestReplayBundleRejected(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	signerID := bundle.Ed25519KeyID(pub)
	id, _ := age.GenerateX25519Identity()
	store, _ := profile.NewStore(filepath.Join(t.TempDir(), "profiles.enc"), []byte("0123456789abcdef0123456789abcdef"))
	imp := importctl.NewImporter(store, []ed25519.PublicKey{pub}, id)
	now := time.Now().Unix()
	p := mustValidProfile(t, signerID, now)
	raw := mustBundle(t, &bundle.BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        []profile.Profile{p},
		Templates: map[string]bundle.Template{
			string(p.Family): {TemplateText: `{"type":"direct","tag":"proxy"}`},
		},
		Notes: map[string]string{"issuer_key_id": signerID},
	}, priv, id.Recipient().String(), signerID, "bundle-replay", now)
	if _, err := imp.ParseBytes(raw); err != nil {
		t.Fatalf("first parse should succeed: %v", err)
	}
	if _, err := imp.ParseBytes(raw); err == nil || !strings.Contains(err.Error(), "replay detected") {
		t.Fatalf("expected replay rejection, got %v", err)
	}
}

func mustValidProfile(t *testing.T, signerID string, now int64) profile.Profile {
	t.Helper()
	p, err := profile.NormalizeForWire(profile.Profile{
		ID:     "reality-1",
		Family: profile.FamilyReality,
		Endpoint: profile.Endpoint{
			Host: "example.com",
			Port: 443,
		},
		Capabilities: profile.Capabilities{
			Transport: "tcp",
		},
		Credentials: profile.Credentials{
			UUID:            "00000000-0000-0000-0000-000000000001",
			PublicKey:       "pk",
			ShortID:         "sid",
			SNI:             "example.com",
			UTLSFingerprint: "chrome",
		},
		Source: profile.SourceInfo{
			Source:       "bundle",
			TrustLevel:   80,
			ImportedAt:   now,
			PublisherKey: signerID,
		},
	}, now)
	if err != nil {
		t.Fatalf("normalize profile: %v", err)
	}
	return p
}

func mustBundle(t *testing.T, payload *bundle.BundlePayload, signer ed25519.PrivateKey, recipient string, signerID string, bundleID string, now int64) []byte {
	t.Helper()
	raw, err := bundle.GenerateBundleWithOptions(payload, signer, bundle.GenerateOptions{
		RecipientPublicKey: recipient,
		SignerKeyID:        signerID,
		BundleID:           bundleID,
		Seq:                1,
		CreatedAt:          now,
		ExpiresAt:          now + 3600,
	})
	if err != nil {
		t.Fatalf("generate bundle: %v", err)
	}
	return raw
}
