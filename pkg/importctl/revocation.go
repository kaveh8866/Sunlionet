package importctl

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/bundle"
	"github.com/kaveh/sunlionet-agent/pkg/profile"
)

const (
	TrustUpdateMagic        = "SNB-TRUST-1"
	TrustUpdateDomain       = "SUNLIONET-TRUST-UPDATE-V1\x00"
	TrustStateDomain        = "SUNLIONET-TRUST-STATE-V1\x00"
	MaxTrustUpdateBytes     = 256 << 10
	MaxTrustStateBytes      = 256 << 10
	DefaultTrustThreshold   = 2
	MaxTrustUpdateTTLSecond = 30 * 24 * 3600
)

var (
	ErrTrustUpdateRollback     = errors.New("trust update rollback")
	ErrTrustUpdateInvalid      = errors.New("invalid trust update")
	ErrTrustUpdateUnauthorized = errors.New("unauthorized trust update")
	ErrSignerRevoked           = errors.New("signer revoked")
)

type TrustState struct {
	Version        uint64                      `json:"version"`
	StateHash      string                      `json:"state_hash"`
	PreviousHash   string                      `json:"previous_hash"`
	Threshold      int                         `json:"threshold"`
	RootKeys       map[string]string           `json:"root_keys"`
	ActiveSigners  map[string]string           `json:"active_signers"`
	RetiredSigners map[string]TransitionRecord `json:"retired_signers"`
	RevokedSigners map[string]RevocationRecord `json:"revoked_signers"`
	UpdatedAt      int64                       `json:"updated_at"`
}

type RevocationRecord struct {
	Reason      string `json:"reason"`
	EffectiveAt int64  `json:"effective_at"`
	RevokedAt   int64  `json:"revoked_at"`
	BlockID     string `json:"block_id"`
}

type TransitionRecord struct {
	ReplacedBy   string `json:"replaced_by,omitempty"`
	EffectiveAt  int64  `json:"effective_at"`
	ExpiresAfter int64  `json:"expires_after"`
	BlockID      string `json:"block_id"`
}

type TrustUpdateBlock struct {
	Magic         string            `json:"magic"`
	BlockID       string            `json:"block_id"`
	Version       uint64            `json:"version"`
	PrevStateHash string            `json:"prev_state_hash"`
	IssuedAt      int64             `json:"issued_at"`
	EffectiveAt   int64             `json:"effective_at"`
	ExpiresAt     int64             `json:"expires_at"`
	Threshold     int               `json:"threshold"`
	Operations    []TrustOperation  `json:"operations"`
	Notes         map[string]string `json:"notes"`
	Signatures    []TrustSignature  `json:"signatures"`
}

type TrustOperation struct {
	Type         string `json:"type"`
	KeyID        string `json:"key_id"`
	PublicKeyB64 string `json:"public_key_b64url,omitempty"`
	Reason       string `json:"reason,omitempty"`
	ReplacedBy   string `json:"replaced_by,omitempty"`
	ExpiresAfter int64  `json:"expires_after,omitempty"`
}

type TrustSignature struct {
	RootKeyID string `json:"root_key_id"`
	SigB64URL string `json:"sig_b64url"`
}

type TrustStore struct {
	dbPath string
	key    []byte
	mu     sync.Mutex
}

func NewTrustStore(dbPath string, masterKey []byte) (*TrustStore, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("masterKey must be 32 bytes")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, err
	}
	return &TrustStore{dbPath: dbPath, key: append([]byte(nil), masterKey...)}, nil
}

func NewTrustState(rootKeys []ed25519.PublicKey, activeSigners []ed25519.PublicKey, threshold int, now int64) (TrustState, error) {
	state := TrustState{
		Version:        0,
		Threshold:      normalizeThreshold(threshold),
		RootKeys:       map[string]string{},
		ActiveSigners:  map[string]string{},
		RetiredSigners: map[string]TransitionRecord{},
		RevokedSigners: map[string]RevocationRecord{},
		UpdatedAt:      now,
	}
	for _, pub := range rootKeys {
		if len(pub) != ed25519.PublicKeySize {
			return TrustState{}, fmt.Errorf("invalid root public key size")
		}
		state.RootKeys[bundle.Ed25519KeyID(pub)] = base64.RawURLEncoding.EncodeToString(pub)
	}
	for _, pub := range activeSigners {
		if len(pub) != ed25519.PublicKeySize {
			return TrustState{}, fmt.Errorf("invalid signer public key size")
		}
		state.ActiveSigners[bundle.Ed25519KeyID(pub)] = base64.RawURLEncoding.EncodeToString(pub)
	}
	if err := state.reseal(); err != nil {
		return TrustState{}, err
	}
	return state, nil
}

func (s TrustState) IsRevoked(keyID string, now int64) bool {
	rec, ok := s.RevokedSigners[strings.TrimSpace(keyID)]
	return ok && (rec.EffectiveAt == 0 || now >= rec.EffectiveAt)
}

func (s TrustState) ActivePublicKeys() ([]ed25519.PublicKey, error) {
	keys := make([]string, 0, len(s.ActiveSigners))
	for id := range s.ActiveSigners {
		if !s.IsRevoked(id, time.Now().Unix()) {
			keys = append(keys, id)
		}
	}
	sort.Strings(keys)
	out := make([]ed25519.PublicKey, 0, len(keys))
	for _, id := range keys {
		raw, err := base64.RawURLEncoding.DecodeString(s.ActiveSigners[id])
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid active signer key %q", id)
		}
		out = append(out, ed25519.PublicKey(raw))
	}
	return out, nil
}

func (s TrustState) ExportUpdateEnvelope() ([]byte, error) {
	return json.Marshal(s)
}

func (s *TrustState) ApplyUpdate(raw []byte, configuredRootKeys []ed25519.PublicKey, now int64) (TrustState, error) {
	if len(raw) == 0 || len(raw) > MaxTrustUpdateBytes {
		return TrustState{}, ErrTrustUpdateInvalid
	}
	var block TrustUpdateBlock
	if err := decodeTrustStrict(raw, &block); err != nil {
		return TrustState{}, fmt.Errorf("%w: %v", ErrTrustUpdateInvalid, err)
	}
	return s.ApplyBlock(block, configuredRootKeys, now)
}

func (s *TrustState) ApplyBlock(block TrustUpdateBlock, configuredRootKeys []ed25519.PublicKey, now int64) (TrustState, error) {
	if now == 0 {
		now = time.Now().Unix()
	}
	if err := validateTrustBlockHeader(block, *s, now); err != nil {
		return TrustState{}, err
	}
	signable, err := canonicalTrustUpdate(block)
	if err != nil {
		return TrustState{}, err
	}
	roots, err := s.verificationRoots(configuredRootKeys)
	if err != nil {
		return TrustState{}, err
	}
	threshold := block.Threshold
	if threshold == 0 {
		threshold = s.Threshold
	}
	if threshold == 0 {
		threshold = DefaultTrustThreshold
	}
	if countValidRootSignatures(signable, block.Signatures, roots) < threshold {
		return TrustState{}, ErrTrustUpdateUnauthorized
	}

	next := s.clone()
	next.Version = block.Version
	next.PreviousHash = s.StateHash
	next.UpdatedAt = now
	if next.Threshold == 0 {
		next.Threshold = normalizeThreshold(block.Threshold)
	}
	if next.RootKeys == nil {
		next.RootKeys = map[string]string{}
	}
	if next.ActiveSigners == nil {
		next.ActiveSigners = map[string]string{}
	}
	if next.RetiredSigners == nil {
		next.RetiredSigners = map[string]TransitionRecord{}
	}
	if next.RevokedSigners == nil {
		next.RevokedSigners = map[string]RevocationRecord{}
	}

	for _, op := range block.Operations {
		if err := applyTrustOperation(&next, block, op); err != nil {
			return TrustState{}, err
		}
	}
	if err := next.reseal(); err != nil {
		return TrustState{}, err
	}
	return next, nil
}

func GenerateTrustUpdateBlock(current TrustState, rootPrivs []ed25519.PrivateKey, block TrustUpdateBlock) ([]byte, error) {
	block.Magic = TrustUpdateMagic
	block.PrevStateHash = current.StateHash
	if block.Threshold == 0 {
		block.Threshold = current.Threshold
	}
	if block.Threshold == 0 {
		block.Threshold = DefaultTrustThreshold
	}
	if block.IssuedAt == 0 {
		block.IssuedAt = time.Now().Unix()
	}
	if block.EffectiveAt == 0 {
		block.EffectiveAt = block.IssuedAt
	}
	if block.ExpiresAt == 0 {
		block.ExpiresAt = block.IssuedAt + MaxTrustUpdateTTLSecond
	}
	if strings.TrimSpace(block.BlockID) == "" {
		block.BlockID = fmt.Sprintf("trust_%d_%d", block.Version, block.IssuedAt)
	}
	block.Signatures = nil
	signable, err := canonicalTrustUpdate(block)
	if err != nil {
		return nil, err
	}
	for _, priv := range rootPrivs {
		if len(priv) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf("invalid root private key size")
		}
		pub := priv.Public().(ed25519.PublicKey)
		block.Signatures = append(block.Signatures, TrustSignature{
			RootKeyID: bundle.Ed25519KeyID(pub),
			SigB64URL: base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, signable)),
		})
	}
	sort.Slice(block.Signatures, func(i, j int) bool {
		return block.Signatures[i].RootKeyID < block.Signatures[j].RootKeyID
	})
	return json.Marshal(block)
}

func (s *TrustStore) Save(state TrustState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := state.reseal(); err != nil {
		return err
	}
	plaintext, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if len(plaintext) > MaxTrustStateBytes {
		return fmt.Errorf("%w: trust state too large", profile.ErrCorruptStore)
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	tmp := s.dbPath + ".tmp"
	if err := os.WriteFile(tmp, ciphertext, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.dbPath)
}

func (s *TrustStore) Load() (TrustState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ciphertext, err := os.ReadFile(s.dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return TrustState{}, nil
		}
		return TrustState{}, err
	}
	if len(ciphertext) > MaxTrustStateBytes+64 {
		return TrustState{}, fmt.Errorf("%w: trust state too large", profile.ErrCorruptStore)
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return TrustState{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return TrustState{}, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return TrustState{}, fmt.Errorf("%w: malformed trust store", profile.ErrCorruptStore)
	}
	nonce, body := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return TrustState{}, profile.ErrDecryptionFailed
	}
	var state TrustState
	if err := json.Unmarshal(plaintext, &state); err != nil {
		return TrustState{}, err
	}
	if err := state.verifySeal(); err != nil {
		return TrustState{}, err
	}
	state.ensureMaps()
	return state, nil
}

func validateTrustBlockHeader(block TrustUpdateBlock, current TrustState, now int64) error {
	if block.Magic != TrustUpdateMagic || strings.TrimSpace(block.BlockID) == "" {
		return ErrTrustUpdateInvalid
	}
	if block.Version <= current.Version {
		return ErrTrustUpdateRollback
	}
	if current.StateHash != "" && block.PrevStateHash != current.StateHash {
		return ErrTrustUpdateRollback
	}
	if block.IssuedAt <= 0 || block.EffectiveAt <= 0 || block.ExpiresAt <= 0 || block.ExpiresAt < block.IssuedAt {
		return ErrTrustUpdateInvalid
	}
	if block.ExpiresAt-block.IssuedAt > MaxTrustUpdateTTLSecond {
		return ErrTrustUpdateInvalid
	}
	if block.IssuedAt > now+bundle.MaxClockSkewSeconds || now > block.ExpiresAt {
		return ErrTrustUpdateInvalid
	}
	if len(block.Operations) == 0 || len(block.Operations) > 128 {
		return ErrTrustUpdateInvalid
	}
	return nil
}

func applyTrustOperation(state *TrustState, block TrustUpdateBlock, op TrustOperation) error {
	keyID := strings.TrimSpace(op.KeyID)
	if keyID == "" {
		return ErrTrustUpdateInvalid
	}
	switch op.Type {
	case "add_signer":
		pub, err := parseOperationPublicKey(op, keyID)
		if err != nil {
			return err
		}
		state.ActiveSigners[keyID] = base64.RawURLEncoding.EncodeToString(pub)
		delete(state.RetiredSigners, keyID)
	case "revoke_signer":
		delete(state.ActiveSigners, keyID)
		state.RevokedSigners[keyID] = RevocationRecord{
			Reason:      cleanReason(op.Reason),
			EffectiveAt: block.EffectiveAt,
			RevokedAt:   block.IssuedAt,
			BlockID:     block.BlockID,
		}
	case "retire_signer":
		delete(state.ActiveSigners, keyID)
		state.RetiredSigners[keyID] = TransitionRecord{
			ReplacedBy:   strings.TrimSpace(op.ReplacedBy),
			EffectiveAt:  block.EffectiveAt,
			ExpiresAfter: op.ExpiresAfter,
			BlockID:      block.BlockID,
		}
	case "add_root":
		pub, err := parseOperationPublicKey(op, keyID)
		if err != nil {
			return err
		}
		state.RootKeys[keyID] = base64.RawURLEncoding.EncodeToString(pub)
	case "retire_root":
		delete(state.RootKeys, keyID)
	default:
		return ErrTrustUpdateInvalid
	}
	return nil
}

func parseOperationPublicKey(op TrustOperation, expectedKeyID string) (ed25519.PublicKey, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(op.PublicKeyB64))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, ErrTrustUpdateInvalid
	}
	pub := ed25519.PublicKey(raw)
	if bundle.Ed25519KeyID(pub) != expectedKeyID {
		return nil, ErrTrustUpdateInvalid
	}
	return pub, nil
}

func (s TrustState) verificationRoots(configured []ed25519.PublicKey) (map[string]ed25519.PublicKey, error) {
	roots := map[string]ed25519.PublicKey{}
	for _, pub := range configured {
		if len(pub) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid configured root public key")
		}
		roots[bundle.Ed25519KeyID(pub)] = append(ed25519.PublicKey(nil), pub...)
	}
	for id, b64 := range s.RootKeys {
		raw, err := base64.RawURLEncoding.DecodeString(b64)
		if err != nil || len(raw) != ed25519.PublicKeySize || bundle.Ed25519KeyID(ed25519.PublicKey(raw)) != id {
			return nil, fmt.Errorf("invalid stored root public key")
		}
		roots[id] = ed25519.PublicKey(raw)
	}
	if len(roots) == 0 {
		return nil, ErrTrustUpdateUnauthorized
	}
	return roots, nil
}

func countValidRootSignatures(msg []byte, sigs []TrustSignature, roots map[string]ed25519.PublicKey) int {
	seen := map[string]struct{}{}
	valid := 0
	for _, sig := range sigs {
		id := strings.TrimSpace(sig.RootKeyID)
		if _, ok := seen[id]; ok {
			continue
		}
		pub, ok := roots[id]
		if !ok {
			continue
		}
		raw, err := base64.RawURLEncoding.DecodeString(sig.SigB64URL)
		if err != nil || len(raw) != ed25519.SignatureSize {
			continue
		}
		if ed25519.Verify(pub, msg, raw) {
			seen[id] = struct{}{}
			valid++
		}
	}
	return valid
}

func canonicalTrustUpdate(block TrustUpdateBlock) ([]byte, error) {
	block.Signatures = nil
	sort.Slice(block.Operations, func(i, j int) bool {
		if block.Operations[i].Type != block.Operations[j].Type {
			return block.Operations[i].Type < block.Operations[j].Type
		}
		return block.Operations[i].KeyID < block.Operations[j].KeyID
	})
	raw, err := json.Marshal(block)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.WriteString(TrustUpdateDomain)
	out.Write(raw)
	return out.Bytes(), nil
}

func (s *TrustState) reseal() error {
	s.ensureMaps()
	copyState := s.clone()
	copyState.StateHash = ""
	raw, err := canonicalTrustState(copyState)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	s.StateHash = base64.RawURLEncoding.EncodeToString(sum[:])
	return nil
}

func (s TrustState) verifySeal() error {
	if strings.TrimSpace(s.StateHash) == "" {
		return nil
	}
	copyState := s.clone()
	copyState.StateHash = ""
	raw, err := canonicalTrustState(copyState)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	if base64.RawURLEncoding.EncodeToString(sum[:]) != s.StateHash {
		return fmt.Errorf("%w: trust state seal mismatch", profile.ErrCorruptStore)
	}
	return nil
}

func canonicalTrustState(state TrustState) ([]byte, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.WriteString(TrustStateDomain)
	out.Write(raw)
	return out.Bytes(), nil
}

func (s TrustState) clone() TrustState {
	out := s
	out.RootKeys = cloneStringMap(s.RootKeys)
	out.ActiveSigners = cloneStringMap(s.ActiveSigners)
	out.RetiredSigners = cloneTransitionMap(s.RetiredSigners)
	out.RevokedSigners = cloneRevocationMap(s.RevokedSigners)
	return out
}

func (s *TrustState) ensureMaps() {
	if s.RootKeys == nil {
		s.RootKeys = map[string]string{}
	}
	if s.ActiveSigners == nil {
		s.ActiveSigners = map[string]string{}
	}
	if s.RetiredSigners == nil {
		s.RetiredSigners = map[string]TransitionRecord{}
	}
	if s.RevokedSigners == nil {
		s.RevokedSigners = map[string]RevocationRecord{}
	}
	if s.Threshold == 0 {
		s.Threshold = DefaultTrustThreshold
	}
}

func normalizeThreshold(n int) int {
	if n <= 0 {
		return DefaultTrustThreshold
	}
	return n
}

func cleanReason(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneTransitionMap(in map[string]TransitionRecord) map[string]TransitionRecord {
	out := make(map[string]TransitionRecord, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneRevocationMap(in map[string]RevocationRecord) map[string]RevocationRecord {
	out := make(map[string]RevocationRecord, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func decodeTrustStrict(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	tok, err := dec.Token()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("trailing JSON content: %v", tok)
}
