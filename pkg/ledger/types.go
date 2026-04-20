package ledger

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	SchemaV1 = 1

	MaxParents = 32
	MaxKindLen = 64
	MaxRefLen  = 8192
)

const InlinePayloadPrefix = "inline:"

type Event struct {
	SchemaVersion  int      `json:"v"`
	ID             string   `json:"id"`
	CreatedAt      int64    `json:"created_at"`
	Author         string   `json:"author"`
	AuthorKeyB64   string   `json:"author_key_b64url"`
	Seq            uint64   `json:"seq"`
	Prev           string   `json:"prev,omitempty"`
	Parents        []string `json:"parents,omitempty"`
	Kind           string   `json:"kind"`
	PayloadHashB64 string   `json:"payload_hash_b64url,omitempty"`
	PayloadRef     string   `json:"payload_ref,omitempty"`
	SigB64         string   `json:"sig_b64url"`
}

type SignedEventInput struct {
	Author     string
	AuthorPub  ed25519.PublicKey
	AuthorPriv ed25519.PrivateKey

	Seq     uint64
	Prev    string
	Parents []string

	Kind       string
	Payload    json.RawMessage
	PayloadRef string

	CreatedAt time.Time
}

func NewSignedEvent(in SignedEventInput) (Event, error) {
	author := strings.TrimSpace(in.Author)
	if author == "" {
		return Event{}, errors.New("ledger: author required")
	}
	if len(in.AuthorPub) != ed25519.PublicKeySize {
		return Event{}, fmt.Errorf("ledger: invalid author public key size: %d", len(in.AuthorPub))
	}
	if len(in.AuthorPriv) != ed25519.PrivateKeySize {
		return Event{}, fmt.Errorf("ledger: invalid author private key size: %d", len(in.AuthorPriv))
	}
	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		return Event{}, errors.New("ledger: kind required")
	}
	if len(kind) > MaxKindLen {
		return Event{}, fmt.Errorf("ledger: kind too long: %d", len(kind))
	}
	if len(in.Parents) > MaxParents {
		return Event{}, fmt.Errorf("ledger: too many parents: %d", len(in.Parents))
	}
	if len(in.PayloadRef) > MaxRefLen {
		return Event{}, fmt.Errorf("ledger: payload_ref too long: %d", len(in.PayloadRef))
	}

	createdAt := in.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	var payloadHashB64 string
	if len(in.Payload) > 0 {
		h := sha256.Sum256(in.Payload)
		payloadHashB64 = base64.RawURLEncoding.EncodeToString(h[:])
	}

	ev := Event{
		SchemaVersion:  SchemaV1,
		CreatedAt:      createdAt.Unix(),
		Author:         author,
		AuthorKeyB64:   base64.RawURLEncoding.EncodeToString(in.AuthorPub),
		Seq:            in.Seq,
		Prev:           strings.TrimSpace(in.Prev),
		Parents:        normalizeIDSet(in.Parents),
		Kind:           kind,
		PayloadHashB64: payloadHashB64,
		PayloadRef:     strings.TrimSpace(in.PayloadRef),
	}
	hash, err := ev.unsignedHash()
	if err != nil {
		return Event{}, err
	}
	ev.ID = base64.RawURLEncoding.EncodeToString(hash[:])
	ev.SigB64 = base64.RawURLEncoding.EncodeToString(ed25519.Sign(in.AuthorPriv, hash[:]))
	if err := ev.Validate(); err != nil {
		return Event{}, err
	}
	return ev, nil
}

func (e *Event) Validate() error {
	if e == nil {
		return errors.New("ledger: event is nil")
	}
	if e.SchemaVersion != SchemaV1 {
		return fmt.Errorf("ledger: unsupported schema version: %d", e.SchemaVersion)
	}
	if strings.TrimSpace(e.ID) == "" {
		return errors.New("ledger: id required")
	}
	if e.CreatedAt <= 0 {
		return errors.New("ledger: created_at required")
	}
	if strings.TrimSpace(e.Author) == "" {
		return errors.New("ledger: author required")
	}
	if strings.TrimSpace(e.AuthorKeyB64) == "" {
		return errors.New("ledger: author_key_b64url required")
	}
	if e.Seq == 0 {
		return errors.New("ledger: seq must be >= 1")
	}
	if e.Seq == 1 && strings.TrimSpace(e.Prev) != "" {
		return errors.New("ledger: prev must be empty for seq=1")
	}
	if e.Seq > 1 && strings.TrimSpace(e.Prev) == "" {
		return errors.New("ledger: prev required for seq>1")
	}
	if len(e.Parents) > MaxParents {
		return fmt.Errorf("ledger: too many parents: %d", len(e.Parents))
	}
	if strings.TrimSpace(e.Kind) == "" {
		return errors.New("ledger: kind required")
	}
	if len(e.Kind) > MaxKindLen {
		return fmt.Errorf("ledger: kind too long: %d", len(e.Kind))
	}
	if len(e.PayloadRef) > MaxRefLen {
		return fmt.Errorf("ledger: payload_ref too long: %d", len(e.PayloadRef))
	}
	if strings.TrimSpace(e.SigB64) == "" {
		return errors.New("ledger: sig_b64url required")
	}
	_, err := e.AuthorPublicKey()
	if err != nil {
		return err
	}
	if _, err := base64.RawURLEncoding.DecodeString(e.SigB64); err != nil {
		return fmt.Errorf("ledger: decode sig_b64url: %w", err)
	}
	return e.Verify()
}

func (e *Event) AuthorPublicKey() (ed25519.PublicKey, error) {
	if e == nil {
		return nil, errors.New("ledger: event is nil")
	}
	raw, err := base64.RawURLEncoding.DecodeString(e.AuthorKeyB64)
	if err != nil {
		return nil, fmt.Errorf("ledger: decode author_key_b64url: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("ledger: invalid author key size: %d", len(raw))
	}
	return ed25519.PublicKey(raw), nil
}

func (e *Event) Verify() error {
	if e == nil {
		return errors.New("ledger: event is nil")
	}
	hash, err := e.unsignedHash()
	if err != nil {
		return err
	}
	wantID := base64.RawURLEncoding.EncodeToString(hash[:])
	if e.ID != wantID {
		return errors.New("ledger: id mismatch")
	}
	pub, err := e.AuthorPublicKey()
	if err != nil {
		return err
	}
	sig, err := base64.RawURLEncoding.DecodeString(e.SigB64)
	if err != nil {
		return fmt.Errorf("ledger: decode sig_b64url: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("ledger: invalid signature size: %d", len(sig))
	}
	if !ed25519.Verify(pub, hash[:], sig) {
		return errors.New("ledger: signature verification failed")
	}
	return nil
}

func (e *Event) unsignedHash() ([32]byte, error) {
	raw, err := MarshalCanonicalEventUnsigned(e)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(raw), nil
}

func normalizeIDSet(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func InlinePayloadRef(payload json.RawMessage) (payloadHashB64 string, payloadRef string, err error) {
	if len(payload) == 0 {
		return "", "", errors.New("ledger: payload required")
	}
	if len(payload) > MaxInlinePayloadBytes {
		return "", "", fmt.Errorf("ledger: inline payload too large: %d", len(payload))
	}
	h := sha256.Sum256(payload)
	return base64.RawURLEncoding.EncodeToString(h[:]), InlinePayloadPrefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

func DecodeInlinePayloadRef(ref string, maxBytes int) (json.RawMessage, bool, error) {
	ref = strings.TrimSpace(ref)
	if !strings.HasPrefix(ref, InlinePayloadPrefix) {
		return nil, false, nil
	}
	enc := strings.TrimPrefix(ref, InlinePayloadPrefix)
	raw, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return nil, true, fmt.Errorf("ledger: decode inline payload: %w", err)
	}
	if maxBytes > 0 && len(raw) > maxBytes {
		return nil, true, fmt.Errorf("ledger: inline payload too large: %d", len(raw))
	}
	return json.RawMessage(raw), true, nil
}

func VerifyPayloadHash(ev Event, payload json.RawMessage) error {
	if len(payload) == 0 {
		if strings.TrimSpace(ev.PayloadHashB64) != "" {
			return errors.New("ledger: payload hash present but payload empty")
		}
		return nil
	}
	h := sha256.Sum256(payload)
	want := base64.RawURLEncoding.EncodeToString(h[:])
	if ev.PayloadHashB64 != want {
		return errors.New("ledger: payload_hash mismatch")
	}
	return nil
}
