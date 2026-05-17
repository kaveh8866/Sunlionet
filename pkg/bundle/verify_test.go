package bundle

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"filippo.io/age"
	"github.com/kaveh/sunlionet-agent/pkg/profile"
)

func TestVerifyBundle_EncryptedOK(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("gen signer: %v", err)
	}
	signerKeyID := Ed25519KeyID(pub)
	ageID, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("gen age: %v", err)
	}
	now := int64(1_700_000_000)

	p0 := profile.Profile{
		ID:     "Reality_01",
		Family: profile.FamilyReality,
		Endpoint: profile.Endpoint{
			Host: "Example.COM",
			Port: 443,
		},
		Capabilities: profile.Capabilities{
			Transport: "tcp",
		},
		Credentials: profile.Credentials{
			UUID:            "u",
			PublicKey:       "pk",
			ShortID:         "sid",
			SNI:             "sni.example",
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

	payload := &BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        []profile.Profile{p},
		Revocations:     nil,
		PolicyOverrides: PolicyOverrides{},
		Templates: map[string]Template{
			string(p.Family): {TemplateText: `{}`},
		},
		Notes: map[string]string{
			"issuer_key_id": signerKeyID,
		},
	}

	b, err := GenerateBundleWithOptions(payload, priv, GenerateOptions{
		RecipientPublicKey: ageID.Recipient().String(),
		AllowPlaintext:     false,
		SignerKeyID:        signerKeyID,
		BundleID:           "bndl_test",
		Seq:                1,
		CreatedAt:          now,
		ExpiresAt:          now + 3600,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	res, err := VerifyBundle(b, pub, VerifyOptions{
		NowUnix:        now,
		AgeIdentity:    ageID,
		RequireDecrypt: true,
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if res.Payload == nil || len(res.Payload.Profiles) != 1 {
		t.Fatalf("expected decrypted payload")
	}
}

func TestVerifyBundle_WrongSignerKey(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	otherPub, _, _ := ed25519.GenerateKey(nil)
	ageID, _ := age.GenerateX25519Identity()
	now := int64(1_700_000_000)
	signerKeyID := Ed25519KeyID(pub)

	p0 := profile.Profile{
		ID:     "p1",
		Family: profile.FamilyShadowTLS,
		Endpoint: profile.Endpoint{
			Host: "example.com",
			Port: 443,
		},
		Capabilities: profile.Capabilities{
			Transport: "tcp",
		},
		Source: profile.SourceInfo{
			Source:       "test",
			TrustLevel:   80,
			PublisherKey: signerKeyID,
		},
	}
	p, _ := profile.NormalizeForWire(p0, now)
	payload := &BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        []profile.Profile{p},
		Templates: map[string]Template{
			string(p.Family): {TemplateText: `{}`},
		},
		Notes: map[string]string{
			"issuer_key_id": signerKeyID,
		},
	}

	b, _ := GenerateBundleWithOptions(payload, priv, GenerateOptions{
		RecipientPublicKey: ageID.Recipient().String(),
		AllowPlaintext:     false,
		SignerKeyID:        signerKeyID,
		BundleID:           "bndl_test",
		Seq:                1,
		CreatedAt:          now,
		ExpiresAt:          now + 3600,
	})
	_, err := VerifyBundle(b, otherPub, VerifyOptions{NowUnix: now, AgeIdentity: ageID, RequireDecrypt: true})
	if err == nil {
		t.Fatalf("expected error")
	}
	verr, ok := err.(*VerifyError)
	if !ok {
		t.Fatalf("expected VerifyError, got %T", err)
	}
	found := false
	for _, it := range verr.Issues {
		if it.Code == "issuer_mismatch" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected issuer_mismatch issue, got %#v", verr.Issues)
	}
}

func TestVerifyBundle_Expired(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	signerKeyID := Ed25519KeyID(pub)
	now := int64(1_700_000_000)

	p0 := profile.Profile{
		ID:     "p1",
		Family: profile.FamilyDNS,
		Endpoint: profile.Endpoint{
			Host: "example.com",
			Port: 53,
		},
		Capabilities: profile.Capabilities{
			Transport: "udp",
		},
		Source: profile.SourceInfo{
			Source:       "test",
			TrustLevel:   80,
			PublisherKey: signerKeyID,
		},
	}
	p, _ := profile.NormalizeForWire(p0, now)
	payload := &BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        []profile.Profile{p},
		Templates: map[string]Template{
			string(p.Family): {TemplateText: `{}`},
		},
		Notes: map[string]string{
			"issuer_key_id": signerKeyID,
		},
	}

	b, _ := GenerateBundleWithOptions(payload, priv, GenerateOptions{
		RecipientPublicKey: "",
		AllowPlaintext:     true,
		SignerKeyID:        signerKeyID,
		BundleID:           "bndl_test",
		Seq:                1,
		CreatedAt:          now - 10,
		ExpiresAt:          now - 1,
	})
	_, err := VerifyBundle(b, pub, VerifyOptions{NowUnix: now})
	if err == nil {
		t.Fatalf("expected error")
	}
	verr, ok := err.(*VerifyError)
	if !ok {
		t.Fatalf("expected VerifyError, got %T", err)
	}
	found := false
	for _, it := range verr.Issues {
		if it.Code == "expired" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected expired issue, got %#v", verr.Issues)
	}
}

func TestVerifyBundle_RejectsNonCanonicalPayload(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	signerKeyID := Ed25519KeyID(pub)
	now := int64(1_700_000_000)

	p0 := profile.Profile{
		ID:     "p1",
		Family: profile.FamilyDNS,
		Endpoint: profile.Endpoint{
			Host: "example.com",
			Port: 53,
		},
		Capabilities: profile.Capabilities{
			Transport: "udp",
		},
		Source: profile.SourceInfo{
			Source:       "test",
			TrustLevel:   80,
			PublisherKey: signerKeyID,
		},
	}
	p, _ := profile.NormalizeForWire(p0, now)
	payload := &BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        []profile.Profile{p},
		Templates: map[string]Template{
			string(p.Family): {TemplateText: `{}`},
		},
		Notes: map[string]string{
			"issuer_key_id": signerKeyID,
		},
	}

	canonical, err := MarshalCanonicalPayload(payload)
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	var decoded any
	if err := json.Unmarshal(canonical, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	pretty, _ := json.MarshalIndent(decoded, "", "  ")
	if bytes.Equal(pretty, canonical) {
		t.Fatalf("expected pretty to differ from canonical")
	}

	header := BundleHeader{
		Magic:          "SNB1",
		BundleID:       "bndl_test",
		PublisherKeyID: signerKeyID,
		RecipientKeyID: "none",
		Seq:            1,
		Nonce:          base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, BundleNonceSize)),
		CreatedAt:      now,
		ExpiresAt:      now + 3600,
		Cipher:         "none",
		Signature:      "",
	}
	headerBytes, _ := json.Marshal(header)

	sig := ed25519.Sign(priv, signatureInput(headerBytes, pretty))
	header.Signature = base64.RawURLEncoding.EncodeToString(sig)

	wrapper := struct {
		Header     BundleHeader `json:"header"`
		Ciphertext string       `json:"ciphertext"`
	}{
		Header:     header,
		Ciphertext: base64.RawURLEncoding.EncodeToString(pretty),
	}
	raw, _ := json.Marshal(wrapper)

	_, err = VerifyBundle(raw, pub, VerifyOptions{NowUnix: now, RequireDecrypt: true})
	if err == nil {
		t.Fatalf("expected error")
	}
	verr, ok := err.(*VerifyError)
	if !ok {
		t.Fatalf("expected VerifyError, got %T", err)
	}
	found := false
	for _, it := range verr.Issues {
		if it.Code == "payload_not_canonical" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected payload_not_canonical issue, got %#v", verr.Issues)
	}
}

func TestVerifyBundle_WrongRecipientIdentity(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	signerKeyID := Ed25519KeyID(pub)
	ageID1, _ := age.GenerateX25519Identity()
	ageID2, _ := age.GenerateX25519Identity()
	now := int64(1_700_000_000)

	p0 := profile.Profile{
		ID:     "p1",
		Family: profile.FamilyDNS,
		Endpoint: profile.Endpoint{
			Host: "example.com",
			Port: 53,
		},
		Capabilities: profile.Capabilities{
			Transport: "udp",
		},
		Source: profile.SourceInfo{
			Source:       "test",
			TrustLevel:   80,
			PublisherKey: signerKeyID,
		},
	}
	p, _ := profile.NormalizeForWire(p0, now)
	payload := &BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        []profile.Profile{p},
		Templates: map[string]Template{
			string(p.Family): {TemplateText: `{}`},
		},
		Notes: map[string]string{
			"issuer_key_id": signerKeyID,
		},
	}

	b, _ := GenerateBundleWithOptions(payload, priv, GenerateOptions{
		RecipientPublicKey: ageID1.Recipient().String(),
		AllowPlaintext:     false,
		SignerKeyID:        signerKeyID,
		BundleID:           "bndl_test",
		Seq:                1,
		CreatedAt:          now,
		ExpiresAt:          now + 3600,
	})

	_, err := VerifyBundle(b, pub, VerifyOptions{NowUnix: now, AgeIdentity: ageID2, RequireDecrypt: true})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("expected ErrDecryptionFailed, got %v", err)
	}
}

func TestVerifyBundle_IssuerNoteMismatch(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	signerKeyID := Ed25519KeyID(pub)
	now := int64(1_700_000_000)

	p0 := profile.Profile{
		ID:     "p1",
		Family: profile.FamilyDNS,
		Endpoint: profile.Endpoint{
			Host: "example.com",
			Port: 53,
		},
		Capabilities: profile.Capabilities{
			Transport: "udp",
		},
		Source: profile.SourceInfo{
			Source:       "test",
			TrustLevel:   80,
			PublisherKey: signerKeyID,
		},
	}
	p, _ := profile.NormalizeForWire(p0, now)
	payload := &BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        []profile.Profile{p},
		Templates: map[string]Template{
			string(p.Family): {TemplateText: `{}`},
		},
		Notes: map[string]string{
			"issuer_key_id": "ed25519:bogus",
		},
	}

	b, _ := GenerateBundleWithOptions(payload, priv, GenerateOptions{
		RecipientPublicKey: "",
		AllowPlaintext:     true,
		SignerKeyID:        signerKeyID,
		BundleID:           "bndl_test",
		Seq:                1,
		CreatedAt:          now,
		ExpiresAt:          now + 3600,
	})

	_, err := VerifyBundle(b, pub, VerifyOptions{NowUnix: now, RequireDecrypt: true})
	if err == nil {
		t.Fatalf("expected error")
	}
	verr, ok := err.(*VerifyError)
	if !ok {
		t.Fatalf("expected VerifyError, got %T", err)
	}
	found := false
	for _, it := range verr.Issues {
		if it.Code == "issuer_note_mismatch" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected issuer_note_mismatch issue, got %#v", verr.Issues)
	}
}

func TestVerifyBundle_DuplicateEndpointRejected(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	signerKeyID := Ed25519KeyID(pub)
	now := int64(1_700_000_000)

	p0 := profile.Profile{
		ID:     "p1",
		Family: profile.FamilyDNS,
		Endpoint: profile.Endpoint{
			Host: "example.com",
			Port: 53,
		},
		Capabilities: profile.Capabilities{
			Transport: "udp",
		},
		Source: profile.SourceInfo{
			Source:       "test",
			TrustLevel:   80,
			PublisherKey: signerKeyID,
		},
	}
	p1, _ := profile.NormalizeForWire(p0, now)
	p0.ID = "p2"
	p2, _ := profile.NormalizeForWire(p0, now)

	payload := &BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        []profile.Profile{p1, p2},
		Templates: map[string]Template{
			string(p1.Family): {TemplateText: `{}`},
		},
		Notes: map[string]string{
			"issuer_key_id": signerKeyID,
		},
	}

	b, _ := GenerateBundleWithOptions(payload, priv, GenerateOptions{
		RecipientPublicKey: "",
		AllowPlaintext:     true,
		SignerKeyID:        signerKeyID,
		BundleID:           "bndl_test",
		Seq:                1,
		CreatedAt:          now,
		ExpiresAt:          now + 3600,
	})

	_, err := VerifyBundle(b, pub, VerifyOptions{NowUnix: now, RequireDecrypt: true})
	if err == nil {
		t.Fatalf("expected error")
	}
	verr, ok := err.(*VerifyError)
	if !ok {
		t.Fatalf("expected VerifyError, got %T", err)
	}
	found := false
	for _, it := range verr.Issues {
		if it.Code == "profile_duplicate_endpoint" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected profile_duplicate_endpoint issue, got %#v", verr.Issues)
	}
}
