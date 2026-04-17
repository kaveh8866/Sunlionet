package importctl

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
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
	trustedKeyIDs  map[string]struct{}
	seenBundleIDs  map[string]struct{}
	ageIdentity    *age.X25519Identity
	store          *profile.Store
	templateStore  *profile.TemplateStore
}

func NewImporter(store *profile.Store, trustedKeys []ed25519.PublicKey, ageIdentity *age.X25519Identity) *Importer {
	ids := make(map[string]struct{}, len(trustedKeys))
	for _, k := range trustedKeys {
		ids[bundle.Ed25519KeyID(k)] = struct{}{}
	}
	return &Importer{
		trustedPubKeys: trustedKeys,
		trustedKeyIDs:  ids,
		seenBundleIDs:  map[string]struct{}{},
		ageIdentity:    ageIdentity,
		store:          store,
	}
}

func NewImporterWithTemplates(store *profile.Store, templateStore *profile.TemplateStore, trustedKeys []ed25519.PublicKey, ageIdentity *age.X25519Identity) *Importer {
	ids := make(map[string]struct{}, len(trustedKeys))
	for _, k := range trustedKeys {
		ids[bundle.Ed25519KeyID(k)] = struct{}{}
	}
	return &Importer{
		trustedPubKeys: trustedKeys,
		trustedKeyIDs:  ids,
		seenBundleIDs:  map[string]struct{}{},
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
	if len(i.trustedPubKeys) == 0 {
		return nil, errors.New("bundle invalid: no trusted signer keys configured")
	}
	// Verify signature against all trusted public keys
	var verifyErr error
	var sawSignatureMismatch bool
	for _, pubKey := range i.trustedPubKeys {
		res, err := bundle.VerifyBundle(bundleBytes, pubKey, bundle.VerifyOptions{
			AgeIdentity:    i.ageIdentity,
			RequireDecrypt: true,
		})
		if err == nil {
			if res == nil || res.Payload == nil {
				return nil, errors.New("bundle invalid: missing payload")
			}
			if _, seen := i.seenBundleIDs[res.Header.BundleID]; seen {
				return nil, errors.New("bundle invalid: replay detected")
			}
			i.seenBundleIDs[res.Header.BundleID] = struct{}{}
			if _, ok := i.trustedKeyIDs[res.Header.PublisherKeyID]; !ok {
				return nil, errors.New("bundle invalid: unknown signer")
			}
			return res.Payload, nil
		}
		verifyErr = err
		if errors.Is(err, bundle.ErrInvalidSignature) {
			sawSignatureMismatch = true
			continue
		}
		return nil, fmt.Errorf("bundle invalid: %w", err)
	}
	if sawSignatureMismatch {
		return nil, errors.New("bundle invalid: signature mismatch")
	}
	if verifyErr != nil {
		return nil, fmt.Errorf("bundle invalid: %w", verifyErr)
	}
	return nil, errors.New("bundle invalid: signature mismatch")
}

// ProcessAndStore validates the sing-box configs and writes them to encrypted storage
func (i *Importer) ProcessAndStore(payload *bundle.BundlePayload) error {
	if payload == nil {
		return errors.New("bundle invalid: empty payload")
	}
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
	existingByID := make(map[string]profile.Profile, len(existing))
	existingByEndpoint := make(map[string]string, len(existing))
	for _, p := range existing {
		if !revokedMap[p.ID] {
			existingByID[p.ID] = p
			ek := fmt.Sprintf("%s|%s|%d", p.Family, p.Endpoint.Host, p.Endpoint.Port)
			existingByEndpoint[ek] = p.ID
			updated = append(updated, p)
		}
	}

	// Add new valid profiles
	now := time.Now().Unix()
	newByID := map[string]profile.Profile{}
	newByEndpoint := map[string]string{}
	for _, p := range payload.Profiles {
		np, err := profile.NormalizeForWire(p, now)
		if err != nil {
			return fmt.Errorf("profile invalid: id=%q: %w", p.ID, err)
		}
		p = np
		if prev, ok := existingByID[p.ID]; ok {
			if prev.Family != p.Family || prev.Endpoint.Host != p.Endpoint.Host || prev.Endpoint.Port != p.Endpoint.Port {
				return fmt.Errorf("bundle invalid: conflicting profile id=%q", p.ID)
			}
		}
		if prev, ok := newByID[p.ID]; ok {
			if prev.Family != p.Family || prev.Endpoint.Host != p.Endpoint.Host || prev.Endpoint.Port != p.Endpoint.Port {
				return fmt.Errorf("bundle invalid: conflicting duplicate profile id=%q", p.ID)
			}
		}
		ek := fmt.Sprintf("%s|%s|%d", p.Family, p.Endpoint.Host, p.Endpoint.Port)
		if owner, ok := existingByEndpoint[ek]; ok && owner != p.ID {
			return fmt.Errorf("bundle invalid: endpoint conflict profile=%q endpoint_owner=%q", p.ID, owner)
		}
		if owner, ok := newByEndpoint[ek]; ok && owner != p.ID {
			return fmt.Errorf("bundle invalid: endpoint conflict profile=%q endpoint_owner=%q", p.ID, owner)
		}

		p.Enabled = true
		if p.Source.Source == "" {
			p.Source.Source = "bundle"
		}
		if p.Source.TrustLevel == 0 {
			p.Source.TrustLevel = 80
		}
		if p.Source.ImportedAt == 0 {
			p.Source.ImportedAt = now
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
			p.Health.LastOkAt = now
		}
		newByID[p.ID] = p
		newByEndpoint[ek] = p.ID
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
