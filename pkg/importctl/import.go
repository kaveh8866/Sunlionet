package importctl

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/kaveh/shadownet-agent/pkg/bundle"
	"github.com/kaveh/shadownet-agent/pkg/profile"
)

// Importer handles receiving and validating outside configs
type Importer struct {
	trustedPubKeys []ed25519.PublicKey
	ageIdentity    *age.X25519Identity
	store          *profile.Store
	templateStore  *profile.TemplateStore
}

func NewImporter(store *profile.Store, trustedKeys []ed25519.PublicKey, ageIdentity *age.X25519Identity) *Importer {
	return &Importer{
		trustedPubKeys: trustedKeys,
		ageIdentity:    ageIdentity,
		store:          store,
	}
}

func NewImporterWithTemplates(store *profile.Store, templateStore *profile.TemplateStore, trustedKeys []ed25519.PublicKey, ageIdentity *age.X25519Identity) *Importer {
	return &Importer{
		trustedPubKeys: trustedKeys,
		ageIdentity:    ageIdentity,
		store:          store,
		templateStore:  templateStore,
	}
}

// ParseURI parses `snb://v2:<base64_json_wrapper>` into a validated payload
func (i *Importer) ParseURI(uri string) (*bundle.BundlePayload, error) {
	if !strings.HasPrefix(uri, "snb://v2:") {
		return nil, fmt.Errorf("invalid scheme or version (expected snb://v2:)")
	}

	body := strings.TrimPrefix(uri, "snb://v2:")
	bundleBytes, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode bundle base64: %w", err)
	}

	return i.ParseBytes(bundleBytes)
}

func (i *Importer) ParseBytes(bundleBytes []byte) (*bundle.BundlePayload, error) {
	// Verify signature against all trusted public keys
	var verifyErr error
	for _, pubKey := range i.trustedPubKeys {
		res, err := bundle.VerifyBundle(bundleBytes, pubKey, bundle.VerifyOptions{
			AgeIdentity:    i.ageIdentity,
			RequireDecrypt: true,
		})
		verifyErr = err
		if err == nil {
			return res.Payload, nil
		}
	}

	return nil, fmt.Errorf("SECURITY ALERT: signature verification or decryption failed. Untrusted source or wrong key. Last err: %v", verifyErr)
}

// ProcessAndStore validates the sing-box configs and writes them to encrypted storage
func (i *Importer) ProcessAndStore(payload *bundle.BundlePayload) error {
	existing, err := i.store.Load()
	if err != nil {
		existing = []profile.Profile{}
	}

	// Handle revocations
	revokedMap := make(map[string]bool)
	for _, id := range payload.Revocations {
		revokedMap[id] = true
	}

	var updated []profile.Profile
	for _, p := range existing {
		if !revokedMap[p.ID] {
			updated = append(updated, p)
		}
	}

	// Add new valid profiles
	for _, p := range payload.Profiles {
		p.Enabled = true
		if p.Source.Source == "" {
			p.Source.Source = "bundle"
		}
		if p.Source.TrustLevel == 0 {
			p.Source.TrustLevel = 80
		}
		if p.Source.ImportedAt == 0 {
			p.Source.ImportedAt = time.Now().Unix()
		}
		if p.Health.LastOkAt == 0 && p.Health.LastFailAt == 0 {
			p.Health.LastFailAt = 0
		}
		if p.Health.LastOkAt == 0 {
			p.Health.LastOkAt = 0
		}
		if p.Health.Score == 0 && p.Health.SuccessEWMA == 0 {
			p.Health.SuccessEWMA = 0.5
		}
		if p.Health.LastFailAt == 0 && p.Health.LastOkAt == 0 {
			p.Health.LastOkAt = time.Now().Unix()
		}
		updated = append(updated, p)
	}

	if i.templateStore != nil && len(payload.Templates) > 0 {
		currentTemplates, err := i.templateStore.Load()
		if err != nil {
			currentTemplates = map[string]string{}
		}
		for k, v := range payload.Templates {
			currentTemplates[k] = v.TemplateText
		}
		if err := i.templateStore.Save(currentTemplates); err != nil {
			return err
		}
	}

	return i.store.Save(updated)
}

func (i *Importer) ImportFile(path string) (*bundle.BundlePayload, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(raw))
	if strings.HasPrefix(text, "snb://v2:") {
		return i.ParseURI(text)
	}
	return i.ParseBytes(raw)
}
