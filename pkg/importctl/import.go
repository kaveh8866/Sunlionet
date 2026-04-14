package importctl

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"strings"

	"filippo.io/age"
	"github.com/kaveh/shadownet-agent/pkg/bundle"
	"github.com/kaveh/shadownet-agent/pkg/profile"
)

// Importer handles receiving and validating outside configs
type Importer struct {
	trustedPubKeys []ed25519.PublicKey
	ageIdentity    *age.X25519Identity
	store          *profile.Store
}

func NewImporter(store *profile.Store, trustedKeys []ed25519.PublicKey, ageIdentity *age.X25519Identity) *Importer {
	return &Importer{
		trustedPubKeys: trustedKeys,
		ageIdentity:    ageIdentity,
		store:          store,
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

	// Verify signature against all trusted public keys
	var payload *bundle.BundlePayload
	var verifyErr error
	for _, pubKey := range i.trustedPubKeys {
		payload, verifyErr = bundle.VerifyAndDecrypt(bundleBytes, pubKey, i.ageIdentity)
		if verifyErr == nil {
			break
		}
	}

	if verifyErr != nil {
		return nil, fmt.Errorf("SECURITY ALERT: signature verification or decryption failed. Untrusted source or wrong key. Last err: %v", verifyErr)
	}

	return payload, nil
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
	updated = append(updated, payload.Profiles...)

	return i.store.Save(updated)
}
