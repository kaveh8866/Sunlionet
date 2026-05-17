package importctl

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"filippo.io/age"
	"github.com/kaveh/sunlionet-agent/pkg/bundle"
	"github.com/kaveh/sunlionet-agent/pkg/profile"
)

type ErrorCode string

const (
	CodeMalformedBundle    ErrorCode = "malformed_bundle"
	CodeCipherNotAllowed   ErrorCode = "cipher_not_allowed"
	CodeNoTrustedSigners   ErrorCode = "no_trusted_signers"
	CodeUntrustedSigner    ErrorCode = "untrusted_signer"
	CodeInvalidSignature   ErrorCode = "invalid_signature"
	CodeSignerRevoked      ErrorCode = "signer_revoked"
	CodeDecryptRequired    ErrorCode = "decrypt_required"
	CodeDecryptFailed      ErrorCode = "decrypt_failed"
	CodeReplayDetected     ErrorCode = "replay_detected"
	CodeInvalidPayload     ErrorCode = "invalid_payload"
	CodeStoreNotConfigured ErrorCode = "store_not_configured"
)

type ImportError struct {
	Code  ErrorCode
	Cause error
}

func (e *ImportError) Error() string {
	if e == nil {
		return "import failed"
	}
	if e.Cause == nil {
		return "import failed: " + string(e.Code)
	}
	return "import failed: " + string(e.Code) + ": " + e.Cause.Error()
}

func (e *ImportError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Importer handles receiving and validating outside configs
type Importer struct {
	mu             sync.Mutex
	trustedPubKeys []ed25519.PublicKey
	trustedByKeyID map[string]ed25519.PublicKey
	trustedKeyIDs  map[string]struct{}
	seenBundleIDs  map[string]struct{}
	replayState    ReplayState
	pendingReplay  map[string]bundle.BundleHeader
	pendingPayload map[*bundle.BundlePayload]string
	ageIdentity    *age.X25519Identity
	store          *profile.Store
	templateStore  *profile.TemplateStore
	replayStore    *ReplayStore
	trustState     *TrustState
}

func NewImporter(store *profile.Store, trustedKeys []ed25519.PublicKey, ageIdentity *age.X25519Identity) *Importer {
	return NewImporterWithAll(store, nil, nil, trustedKeys, ageIdentity)
}

func NewImporterWithTemplates(store *profile.Store, templateStore *profile.TemplateStore, trustedKeys []ed25519.PublicKey, ageIdentity *age.X25519Identity) *Importer {
	return NewImporterWithAll(store, templateStore, nil, trustedKeys, ageIdentity)
}

func NewImporterWithAll(store *profile.Store, templateStore *profile.TemplateStore, replayStore *ReplayStore, trustedKeys []ed25519.PublicKey, ageIdentity *age.X25519Identity) *Importer {
	return NewImporterWithTrust(store, templateStore, replayStore, trustedKeys, ageIdentity, nil)
}

func NewImporterWithTrust(store *profile.Store, templateStore *profile.TemplateStore, replayStore *ReplayStore, trustedKeys []ed25519.PublicKey, ageIdentity *age.X25519Identity, trustState *TrustState) *Importer {
	if len(trustedKeys) == 0 && trustState != nil {
		if active, err := trustState.ActivePublicKeys(); err == nil {
			trustedKeys = active
		}
	}
	ids := make(map[string]struct{}, len(trustedKeys))
	byID := make(map[string]ed25519.PublicKey, len(trustedKeys))
	for _, k := range trustedKeys {
		if len(k) != ed25519.PublicKeySize {
			continue
		}
		id := bundle.Ed25519KeyID(k)
		ids[id] = struct{}{}
		byID[id] = append(ed25519.PublicKey(nil), k...)
	}

	seen := make(map[string]struct{})
	state := ReplayState{Version: ReplayStateVersion, Signers: map[string]SignerReplayState{}}
	if replayStore != nil {
		if loaded, err := replayStore.LoadState(); err == nil {
			state = loaded
			seen = state.BundleIDs
		} else if s, err := replayStore.Load(); err == nil {
			seen = s
			state.BundleIDs = s
		}
	}
	if state.BundleIDs == nil {
		state.BundleIDs = seen
	}
	if state.Signers == nil {
		state.Signers = map[string]SignerReplayState{}
	}

	return &Importer{
		trustedPubKeys: trustedKeys,
		trustedByKeyID: byID,
		trustedKeyIDs:  ids,
		seenBundleIDs:  seen,
		replayState:    state,
		pendingReplay:  map[string]bundle.BundleHeader{},
		pendingPayload: map[*bundle.BundlePayload]string{},
		ageIdentity:    ageIdentity,
		store:          store,
		templateStore:  templateStore,
		replayStore:    replayStore,
		trustState:     trustState,
	}
}

func (i *Importer) SetTrustState(state *TrustState) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.trustState = state
}

// ParseURI parses `snb://v2:<base64_json_wrapper>` into a validated payload
func (i *Importer) ParseURI(uri string) (*bundle.BundlePayload, error) {
	if !strings.HasPrefix(uri, "snb://v2:") {
		return nil, &ImportError{Code: CodeMalformedBundle, Cause: errors.New("invalid scheme or version (expected snb://v2:)")}
	}
	if len(uri) > base64.RawURLEncoding.EncodedLen(bundle.MaxBundleBytes)+len("snb://v2:") {
		return nil, &ImportError{Code: CodeMalformedBundle, Cause: errors.New("bundle URI too large")}
	}

	body := strings.TrimPrefix(uri, "snb://v2:")
	bundleBytes, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, &ImportError{Code: CodeMalformedBundle, Cause: fmt.Errorf("failed to decode bundle base64: %w", err)}
	}

	return i.ParseBytes(bundleBytes)
}

func (i *Importer) ParseBytes(bundleBytes []byte) (*bundle.BundlePayload, error) {
	if len(bundleBytes) == 0 || len(bundleBytes) > bundle.MaxBundleBytes {
		return nil, &ImportError{Code: CodeMalformedBundle, Cause: errors.New("bundle size is invalid")}
	}
	if len(i.trustedByKeyID) == 0 {
		return nil, &ImportError{Code: CodeNoTrustedSigners, Cause: errors.New("no trusted signer keys configured")}
	}

	hdr, err := parseHeader(bundleBytes)
	if err != nil {
		return nil, &ImportError{Code: CodeMalformedBundle, Cause: err}
	}

	if hdr.Cipher != "age-x25519" {
		return nil, &ImportError{Code: CodeCipherNotAllowed, Cause: fmt.Errorf("unsupported cipher %q (expected %q)", hdr.Cipher, "age-x25519")}
	}
	if i.isSignerRevoked(hdr.PublisherKeyID, time.Now().Unix()) {
		return nil, &ImportError{Code: CodeSignerRevoked, Cause: ErrSignerRevoked}
	}

	if _, ok := i.trustedKeyIDs[hdr.PublisherKeyID]; !ok {
		return nil, &ImportError{Code: CodeUntrustedSigner, Cause: errors.New("publisher_key_id is not trusted")}
	}
	pubKey, ok := i.trustedByKeyID[hdr.PublisherKeyID]
	if !ok {
		return nil, &ImportError{Code: CodeUntrustedSigner, Cause: errors.New("trusted signer key not available")}
	}

	res, err := bundle.VerifyBundle(bundleBytes, pubKey, bundle.VerifyOptions{
		AgeIdentity:    i.ageIdentity,
		RequireDecrypt: true,
	})
	if err != nil {
		if errors.Is(err, bundle.ErrInvalidSignature) {
			return nil, &ImportError{Code: CodeInvalidSignature, Cause: err}
		}
		if errors.Is(err, bundle.ErrDecryptionFailed) {
			return nil, &ImportError{Code: CodeDecryptFailed, Cause: err}
		}
		var verr *bundle.VerifyError
		if errors.As(err, &verr) {
			if verifyErrorHasAny(verr, "signature_b64", "signature_size") {
				return nil, &ImportError{Code: CodeInvalidSignature, Cause: err}
			}
			return nil, &ImportError{Code: CodeInvalidPayload, Cause: err}
		}
		return nil, &ImportError{Code: CodeInvalidPayload, Cause: err}
	}
	if res == nil || res.Payload == nil {
		return nil, &ImportError{Code: CodeDecryptRequired, Cause: errors.New("missing decrypted payload")}
	}
	if err := i.reserveReplay(res.Header, res.Payload); err != nil {
		return nil, err
	}
	return res.Payload, nil
}

func (i *Importer) ParseErasureChunkLines(lines []string) (*bundle.BundlePayload, error) {
	if len(lines) == 0 {
		return nil, &ImportError{Code: CodeMalformedBundle, Cause: errors.New("no chunk lines")}
	}
	reassembler := bundle.NewChunkReassembler(bundle.DefaultMaxCacheByte, 10*time.Minute)
	now := time.Now()
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		chunk, err := bundle.ParseEncodedChunkText(line)
		if err != nil {
			return nil, &ImportError{Code: CodeMalformedBundle, Cause: err}
		}
		payload, done, err := reassembler.AddChunk(chunk, now)
		if err != nil {
			return nil, &ImportError{Code: CodeMalformedBundle, Cause: err}
		}
		if done {
			return i.ParseBytes(payload)
		}
	}
	return nil, &ImportError{Code: CodeMalformedBundle, Cause: errors.New("insufficient erasure chunks")}
}

// ProcessAndStore validates the sing-box configs and writes them to encrypted storage
func (i *Importer) ProcessAndStore(payload *bundle.BundlePayload) error {
	if payload == nil {
		return &ImportError{Code: CodeInvalidPayload, Cause: errors.New("empty payload")}
	}
	if i.store == nil {
		return &ImportError{Code: CodeStoreNotConfigured, Cause: errors.New("profiles store is nil")}
	}
	header, replayKey := i.pendingHeader(payload)
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

	if err := i.store.Save(updated); err != nil {
		return err
	}
	if replayKey != "" {
		if err := i.commitReplay(header, replayKey); err != nil {
			return err
		}
	}
	return nil
}

func (i *Importer) ImportFile(path string) (*bundle.BundlePayload, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, bundle.MaxBundleBytes*4+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > bundle.MaxBundleBytes*4 {
		return nil, &ImportError{Code: CodeMalformedBundle, Cause: errors.New("bundle file too large")}
	}
	text := strings.TrimSpace(string(raw))
	if strings.HasPrefix(text, "SNBEC/1 ") {
		return i.ParseErasureChunkLines(strings.Split(text, "\n"))
	}
	if len(raw) > bundle.MaxBundleBytes {
		return nil, &ImportError{Code: CodeMalformedBundle, Cause: errors.New("bundle file too large")}
	}
	if strings.HasPrefix(text, "snb://v2:") {
		return i.ParseURI(text)
	}
	return i.ParseBytes(raw)
}

func parseHeader(raw []byte) (bundle.BundleHeader, error) {
	if len(raw) == 0 || len(raw) > bundle.MaxBundleBytes {
		return bundle.BundleHeader{}, errors.New("bundle size is invalid")
	}
	var wrapper struct {
		Header     bundle.BundleHeader `json:"header"`
		Ciphertext json.RawMessage     `json:"ciphertext"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wrapper); err != nil {
		return bundle.BundleHeader{}, err
	}
	tok, err := dec.Token()
	if err == io.EOF {
		return wrapper.Header, nil
	}
	if err != nil {
		return bundle.BundleHeader{}, err
	}
	return bundle.BundleHeader{}, fmt.Errorf("trailing JSON content: %v", tok)
}

func (i *Importer) reserveReplay(header bundle.BundleHeader, payload *bundle.BundlePayload) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	key := replayKey(header)
	if _, seen := i.seenBundleIDs[header.BundleID]; seen {
		return &ImportError{Code: CodeReplayDetected, Cause: errors.New("bundle id replay detected")}
	}
	if _, pending := i.pendingReplay[key]; pending {
		return &ImportError{Code: CodeReplayDetected, Cause: errors.New("bundle replay already pending")}
	}
	if signer, ok := i.replayState.Signers[header.PublisherKeyID]; ok {
		if signer.Nonces[header.Nonce] {
			return &ImportError{Code: CodeReplayDetected, Cause: errors.New("bundle nonce replay detected")}
		}
		if header.Seq <= signer.MaxSeq {
			return &ImportError{Code: CodeReplayDetected, Cause: errors.New("stale bundle sequence")}
		}
	}

	i.pendingReplay[key] = header
	i.pendingPayload[payload] = key
	return nil
}

func (i *Importer) pendingHeader(payload *bundle.BundlePayload) (bundle.BundleHeader, string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	key := i.pendingPayload[payload]
	if key == "" {
		return bundle.BundleHeader{}, ""
	}
	return i.pendingReplay[key], key
}

func (i *Importer) commitReplay(header bundle.BundleHeader, key string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	delete(i.pendingReplay, key)
	for p, pendingKey := range i.pendingPayload {
		if pendingKey == key {
			delete(i.pendingPayload, p)
			break
		}
	}
	if header.BundleID == "" {
		return nil
	}
	if i.replayState.BundleIDs == nil {
		i.replayState.BundleIDs = map[string]struct{}{}
	}
	if i.replayState.Signers == nil {
		i.replayState.Signers = map[string]SignerReplayState{}
	}
	i.replayState.BundleIDs[header.BundleID] = struct{}{}
	i.seenBundleIDs = i.replayState.BundleIDs
	signer := i.replayState.Signers[header.PublisherKeyID]
	if signer.Nonces == nil {
		signer.Nonces = map[string]bool{}
	}
	signer.Nonces[header.Nonce] = true
	if header.Seq > signer.MaxSeq {
		signer.MaxSeq = header.Seq
	}
	if header.CreatedAt > signer.LastCreatedAt {
		signer.LastCreatedAt = header.CreatedAt
	}
	i.replayState.Signers[header.PublisherKeyID] = signer
	if i.replayStore != nil {
		return i.replayStore.SaveState(i.replayState)
	}
	return nil
}

func replayKey(header bundle.BundleHeader) string {
	return header.PublisherKeyID + "|" + fmt.Sprint(header.Seq) + "|" + header.Nonce + "|" + header.BundleID
}

func verifyErrorHasAny(verr *bundle.VerifyError, codes ...string) bool {
	if verr == nil {
		return false
	}
	want := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		want[code] = struct{}{}
	}
	for _, issue := range verr.Issues {
		if _, ok := want[issue.Code]; ok {
			return true
		}
	}
	return false
}

func (i *Importer) isSignerRevoked(keyID string, now int64) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.trustState == nil {
		return false
	}
	return i.trustState.IsRevoked(keyID, now)
}
