package importctl

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/kaveh/sunlionet-agent/pkg/bundle"
	"github.com/kaveh/sunlionet-agent/pkg/profile"
)

func TestTrustUpdateRevokesSignerAndImporterRejectsBundle(t *testing.T) {
	root1Pub, root1Priv, _ := ed25519.GenerateKey(nil)
	root2Pub, root2Priv, _ := ed25519.GenerateKey(nil)
	root3Pub, _, _ := ed25519.GenerateKey(nil)
	signerPub, signerPriv, _ := ed25519.GenerateKey(nil)
	now := time.Now().Unix()

	state, err := NewTrustState([]ed25519.PublicKey{root1Pub, root2Pub, root3Pub}, []ed25519.PublicKey{signerPub}, 2, now)
	if err != nil {
		t.Fatalf("NewTrustState: %v", err)
	}
	blockRaw, err := GenerateTrustUpdateBlock(state, []ed25519.PrivateKey{root1Priv, root2Priv}, TrustUpdateBlock{
		BlockID:     "trust-revoke-1",
		Version:     1,
		IssuedAt:    now,
		EffectiveAt: now,
		ExpiresAt:   now + 3600,
		Threshold:   2,
		Operations: []TrustOperation{{
			Type:   "revoke_signer",
			KeyID:  bundle.Ed25519KeyID(signerPub),
			Reason: "compromise",
		}},
	})
	if err != nil {
		t.Fatalf("GenerateTrustUpdateBlock: %v", err)
	}
	next, err := state.ApplyUpdate(blockRaw, nil, now)
	if err != nil {
		t.Fatalf("ApplyUpdate: %v", err)
	}
	if !next.IsRevoked(bundle.Ed25519KeyID(signerPub), now) {
		t.Fatalf("expected signer to be revoked")
	}

	ageID, _ := age.GenerateX25519Identity()
	store, _ := profile.NewStore(filepath.Join(t.TempDir(), "profiles.enc"), []byte("0123456789abcdef0123456789abcdef"))
	importer := NewImporterWithTrust(store, nil, nil, []ed25519.PublicKey{signerPub}, ageID, &next)
	raw := mustTrustTestBundle(t, signerPub, signerPriv, ageID, now, "revoked-bundle")
	_, err = importer.ParseBytes(raw)
	var ie *ImportError
	if !errors.As(err, &ie) || ie.Code != CodeSignerRevoked {
		t.Fatalf("expected CodeSignerRevoked, got %T %v", err, err)
	}
}

func TestTrustUpdateRejectsRollbackAndInsufficientSignatures(t *testing.T) {
	root1Pub, root1Priv, _ := ed25519.GenerateKey(nil)
	root2Pub, _, _ := ed25519.GenerateKey(nil)
	root3Pub, _, _ := ed25519.GenerateKey(nil)
	signerPub, _, _ := ed25519.GenerateKey(nil)
	now := time.Now().Unix()

	state, err := NewTrustState([]ed25519.PublicKey{root1Pub, root2Pub, root3Pub}, []ed25519.PublicKey{signerPub}, 2, now)
	if err != nil {
		t.Fatalf("NewTrustState: %v", err)
	}
	raw, err := GenerateTrustUpdateBlock(state, []ed25519.PrivateKey{root1Priv}, TrustUpdateBlock{
		BlockID:     "trust-bad-threshold",
		Version:     1,
		IssuedAt:    now,
		EffectiveAt: now,
		ExpiresAt:   now + 3600,
		Threshold:   2,
		Operations: []TrustOperation{{
			Type:   "revoke_signer",
			KeyID:  bundle.Ed25519KeyID(signerPub),
			Reason: "test",
		}},
	})
	if err != nil {
		t.Fatalf("GenerateTrustUpdateBlock: %v", err)
	}
	if _, err := state.ApplyUpdate(raw, nil, now); !errors.Is(err, ErrTrustUpdateUnauthorized) {
		t.Fatalf("expected ErrTrustUpdateUnauthorized, got %v", err)
	}

	raw2, err := GenerateTrustUpdateBlock(state, []ed25519.PrivateKey{root1Priv}, TrustUpdateBlock{
		BlockID:     "trust-rollback",
		Version:     0,
		IssuedAt:    now,
		EffectiveAt: now,
		ExpiresAt:   now + 3600,
		Threshold:   1,
		Operations: []TrustOperation{{
			Type:   "revoke_signer",
			KeyID:  bundle.Ed25519KeyID(signerPub),
			Reason: "test",
		}},
	})
	if err != nil {
		t.Fatalf("GenerateTrustUpdateBlock rollback: %v", err)
	}
	if _, err := state.ApplyUpdate(raw2, nil, now); !errors.Is(err, ErrTrustUpdateRollback) {
		t.Fatalf("expected ErrTrustUpdateRollback, got %v", err)
	}
}

func TestTrustStoreRoundTripAndRetiredSignerStillAccepted(t *testing.T) {
	root1Pub, root1Priv, _ := ed25519.GenerateKey(nil)
	root2Pub, root2Priv, _ := ed25519.GenerateKey(nil)
	signerPub, signerPriv, _ := ed25519.GenerateKey(nil)
	newSignerPub, _, _ := ed25519.GenerateKey(nil)
	now := time.Now().Unix()

	state, err := NewTrustState([]ed25519.PublicKey{root1Pub, root2Pub}, []ed25519.PublicKey{signerPub}, 2, now)
	if err != nil {
		t.Fatalf("NewTrustState: %v", err)
	}
	raw, err := GenerateTrustUpdateBlock(state, []ed25519.PrivateKey{root1Priv, root2Priv}, TrustUpdateBlock{
		BlockID:     "trust-retire-1",
		Version:     1,
		IssuedAt:    now,
		EffectiveAt: now,
		ExpiresAt:   now + 3600,
		Threshold:   2,
		Operations: []TrustOperation{
			{
				Type:         "add_signer",
				KeyID:        bundle.Ed25519KeyID(newSignerPub),
				PublicKeyB64: base64.RawURLEncoding.EncodeToString(newSignerPub),
			},
			{
				Type:         "retire_signer",
				KeyID:        bundle.Ed25519KeyID(signerPub),
				ReplacedBy:   bundle.Ed25519KeyID(newSignerPub),
				ExpiresAfter: now + 3600,
			},
		},
	})
	if err != nil {
		t.Fatalf("GenerateTrustUpdateBlock: %v", err)
	}
	next, err := state.ApplyUpdate(raw, nil, now)
	if err != nil {
		t.Fatalf("ApplyUpdate: %v", err)
	}
	if next.IsRevoked(bundle.Ed25519KeyID(signerPub), now) {
		t.Fatalf("retired signer must not be treated as revoked")
	}
	trustStore, err := NewTrustStore(filepath.Join(t.TempDir(), "trust.enc"), []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewTrustStore: %v", err)
	}
	if err := trustStore.Save(next); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := trustStore.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	ageID, _ := age.GenerateX25519Identity()
	store, _ := profile.NewStore(filepath.Join(t.TempDir(), "profiles.enc"), []byte("0123456789abcdef0123456789abcdef"))
	importer := NewImporterWithTrust(store, nil, nil, []ed25519.PublicKey{signerPub}, ageID, &loaded)
	rawBundle := mustTrustTestBundle(t, signerPub, signerPriv, ageID, now, "retired-still-valid")
	if _, err := importer.ParseBytes(rawBundle); err != nil {
		t.Fatalf("retired signer should remain usable until bundle expiry: %v", err)
	}
}

func mustTrustTestBundle(t *testing.T, signerPub ed25519.PublicKey, signerPriv ed25519.PrivateKey, ageID *age.X25519Identity, now int64, id string) []byte {
	t.Helper()
	signerID := bundle.Ed25519KeyID(signerPub)
	payload := &bundle.BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        nil,
		Templates:       map[string]bundle.Template{},
		Notes:           map[string]string{"issuer_key_id": signerID},
	}
	raw, err := bundle.GenerateBundleWithOptions(payload, signerPriv, bundle.GenerateOptions{
		RecipientPublicKey: ageID.Recipient().String(),
		SignerKeyID:        signerID,
		BundleID:           id,
		Seq:                1,
		CreatedAt:          now,
		ExpiresAt:          now + 3600,
	})
	if err != nil {
		t.Fatalf("GenerateBundleWithOptions: %v", err)
	}
	return raw
}
