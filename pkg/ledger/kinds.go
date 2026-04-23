package ledger

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	ErrEventTimestampFuture = errors.New("ledger: event timestamp too far in future")
	ErrEventTooOld          = errors.New("ledger: event too old")
	ErrUnknownEventKind     = errors.New("ledger: unknown event kind")
)

const (
	KindLedgerEvent          = "ledger.event"
	KindChatMessage          = "chat.message"
	KindGroupCreate          = "group.create"
	KindGroupJoin            = "group.join"
	KindWitnessAttest        = "witness.attest"
	KindWitnessCheckpoint    = "witness.checkpoint"
	KindIdentityIntroduce    = "identity.introduce"
	KindIdentityRotate       = "identity.rotate"
	KindIdentityRevoke       = "identity.revoke"
	KindMisbehaviorEquivoc   = "misbehavior.equivocation"
	KindMisbehaviorReplay    = "misbehavior.replay"
	KindMisbehaviorSybil     = "misbehavior.sybil"
	KindGroupMembership      = "group.membership"
	KindAgentAction          = "agent.action"
	KindSyncSummary          = "sync.summary"
	MaxContextLen            = 96
	MaxInlinePayloadBytes    = 4096
	MaxCheckpointHeadsInline = 64
	MaxGroupIDLen            = 96
	MaxAgentIDLen            = 96
	MaxActionLen             = 96
	MaxSubjectLen            = 128
	MaxReasonLen             = 256
)

type AttestationPayload struct {
	EventID string `json:"event_id"`
	Context string `json:"ctx"`
	Level   string `json:"level,omitempty"`
}

type CheckpointPayload struct {
	Context     string   `json:"ctx"`
	WindowStart int64    `json:"win_start,omitempty"`
	WindowEnd   int64    `json:"win_end,omitempty"`
	Heads       []string `json:"heads,omitempty"`
	HeadsHash   string   `json:"heads_hash_b64url"`
	HeadsCount  int      `json:"heads_count"`
}

type IdentityIntroducePayload struct {
	Context       string `json:"ctx"`
	SubjectKeyB64 string `json:"subject_key_b64url"`
	SubjectAuthor string `json:"subject_author,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

type IdentityRotatePayload struct {
	PersonaID   string `json:"persona_id"`
	PrevKeyB64  string `json:"prev_key_b64url"`
	NewKeyB64   string `json:"new_key_b64url"`
	EffectiveAt int64  `json:"effective_at,omitempty"`
}

type IdentityRevokePayload struct {
	PersonaID   string `json:"persona_id"`
	KeyB64      string `json:"key_b64url"`
	Reason      string `json:"reason,omitempty"`
	EffectiveAt int64  `json:"effective_at,omitempty"`
}

type MisbehaviorEquivocationPayload struct {
	OffenderAuthor string `json:"offender_author"`
	OffenderKeyB64 string `json:"offender_key_b64url"`
	Seq            uint64 `json:"seq"`
	EventA         string `json:"event_a"`
	EventB         string `json:"event_b"`
	Context        string `json:"ctx,omitempty"`
}

type MisbehaviorReplayPayload struct {
	OffenderAuthor string `json:"offender_author"`
	OffenderKeyB64 string `json:"offender_key_b64url"`
	EventID        string `json:"event_id"`
	Context        string `json:"ctx,omitempty"`
}

type MisbehaviorSybilPayload struct {
	Context        string   `json:"ctx,omitempty"`
	SuspectsKeyB64 []string `json:"suspects_key_b64url"`
	Reason         string   `json:"reason,omitempty"`
}

type GroupMembershipPayload struct {
	GroupID  string `json:"group_id"`
	Action   string `json:"action"`
	Subject  string `json:"subject"`
	Role     string `json:"role,omitempty"`
	GroupVer uint64 `json:"group_ver,omitempty"`
}

type AgentActionPayload struct {
	AgentID string `json:"agent_id"`
	Action  string `json:"action"`
	Scope   string `json:"scope,omitempty"`
	Target  string `json:"target,omitempty"`
	Ref     string `json:"ref,omitempty"`
}

type SyncSummaryPayload struct {
	Context   string `json:"ctx"`
	HeadsHash string `json:"heads_hash_b64url"`
	HeadCount int    `json:"head_count"`
}

func validateWithPolicy(ev Event, p Policy) error {
	if err := ev.Validate(); err != nil {
		return err
	}
	now := time.Now()
	if p.MaxClockSkew > 0 {
		if ev.CreatedAt > now.Add(p.MaxClockSkew).Unix() {
			return ErrEventTimestampFuture
		}
	}
	if p.MaxEventAge > 0 {
		if ev.CreatedAt < now.Add(-p.MaxEventAge).Unix() {
			return ErrEventTooOld
		}
	}
	if !p.AllowUnknownKinds {
		if !isKnownKind(ev.Kind) {
			return ErrUnknownEventKind
		}
	}
	return validateKindPayload(ev, p)
}

func validateWithPolicyNoCrypto(ev Event, p Policy) ([32]byte, error) {
	if err := ev.ValidateNoCrypto(); err != nil {
		return [32]byte{}, err
	}
	now := time.Now()
	if p.MaxClockSkew > 0 {
		if ev.CreatedAt > now.Add(p.MaxClockSkew).Unix() {
			return [32]byte{}, ErrEventTimestampFuture
		}
	}
	if p.MaxEventAge > 0 {
		if ev.CreatedAt < now.Add(-p.MaxEventAge).Unix() {
			return [32]byte{}, ErrEventTooOld
		}
	}
	if !p.AllowUnknownKinds {
		if !isKnownKind(ev.Kind) {
			return [32]byte{}, ErrUnknownEventKind
		}
	}
	if err := validateKindPayload(ev, p); err != nil {
		return [32]byte{}, err
	}
	hash, err := ev.unsignedHash()
	if err != nil {
		return [32]byte{}, err
	}
	return hash, nil
}

func isKnownKind(kind string) bool {
	switch kind {
	case KindLedgerEvent,
		KindChatMessage,
		KindGroupCreate,
		KindGroupJoin,
		KindWitnessAttest,
		KindWitnessCheckpoint,
		KindIdentityIntroduce,
		KindIdentityRotate,
		KindIdentityRevoke,
		KindMisbehaviorEquivoc,
		KindMisbehaviorReplay,
		KindMisbehaviorSybil,
		KindGroupMembership,
		KindAgentAction,
		KindSyncSummary:
		return true
	default:
		return false
	}
}

func validateKindPayload(ev Event, p Policy) error {
	switch ev.Kind {
	case KindWitnessAttest:
		var pl AttestationPayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return err
		}
		if p.Trust.Weight(pl.Context, ev.AuthorKeyB64) <= 0 {
			return errors.New("ledger: witness.attest author not in witness set")
		}
		return validateAttestation(pl, p)
	case KindWitnessCheckpoint:
		var pl CheckpointPayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return err
		}
		if p.Trust.Weight(pl.Context, ev.AuthorKeyB64) <= 0 {
			return errors.New("ledger: witness.checkpoint author not in witness set")
		}
		return validateCheckpoint(pl)
	case KindIdentityIntroduce:
		var pl IdentityIntroducePayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return err
		}
		if p.Trust.Weight(pl.Context, ev.AuthorKeyB64) <= 0 {
			return errors.New("ledger: identity.introduce author not trusted")
		}
		return validateIdentityIntroduce(pl)
	case KindIdentityRotate:
		var pl IdentityRotatePayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return err
		}
		return validateIdentityRotate(pl)
	case KindIdentityRevoke:
		var pl IdentityRevokePayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return err
		}
		return validateIdentityRevoke(pl)
	case KindMisbehaviorEquivoc:
		var pl MisbehaviorEquivocationPayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return err
		}
		return validateMisbehaviorEquivocation(pl)
	case KindMisbehaviorReplay:
		var pl MisbehaviorReplayPayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return err
		}
		return validateMisbehaviorReplay(pl)
	case KindMisbehaviorSybil:
		var pl MisbehaviorSybilPayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return err
		}
		return validateMisbehaviorSybil(pl)
	case KindGroupMembership:
		var pl GroupMembershipPayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return err
		}
		return validateGroupMembership(pl)
	case KindAgentAction:
		var pl AgentActionPayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return err
		}
		return validateAgentAction(pl)
	case KindSyncSummary:
		var pl SyncSummaryPayload
		if err := requireInlinePayload(ev, &pl); err != nil {
			return err
		}
		return validateSyncSummary(pl)
	default:
		return nil
	}
}

func requireInlinePayload[T any](ev Event, out *T) error {
	raw, ok, err := DecodeInlinePayloadRef(ev.PayloadRef, MaxInlinePayloadBytes)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("ledger: inline payload required")
	}
	if err := VerifyPayloadHash(ev, raw); err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func validateAttestation(p AttestationPayload, _ Policy) error {
	if strings.TrimSpace(p.EventID) == "" {
		return errors.New("ledger: attest event_id required")
	}
	ctx := strings.TrimSpace(p.Context)
	if ctx == "" {
		return errors.New("ledger: attest ctx required")
	}
	if len(ctx) > MaxContextLen {
		return errors.New("ledger: attest ctx too long")
	}
	return nil
}

func validateCheckpoint(p CheckpointPayload) error {
	ctx := strings.TrimSpace(p.Context)
	if ctx == "" {
		return errors.New("ledger: checkpoint ctx required")
	}
	if len(ctx) > MaxContextLen {
		return errors.New("ledger: checkpoint ctx too long")
	}
	if p.WindowStart != 0 && p.WindowEnd != 0 && p.WindowEnd < p.WindowStart {
		return errors.New("ledger: checkpoint invalid window")
	}
	heads := normalizeIDSet(p.Heads)
	if len(heads) > MaxCheckpointHeadsInline {
		heads = nil
	}
	if len(heads) > 0 {
		h, cnt := ComputeHeadsHash(heads)
		if p.HeadsHash != "" && p.HeadsHash != h {
			return errors.New("ledger: checkpoint heads_hash mismatch")
		}
		if p.HeadsCount != 0 && p.HeadsCount != cnt {
			return errors.New("ledger: checkpoint heads_count mismatch")
		}
		if strings.TrimSpace(p.HeadsHash) == "" {
			return errors.New("ledger: checkpoint heads_hash_b64url required")
		}
		if p.HeadsCount <= 0 {
			return errors.New("ledger: checkpoint heads_count required")
		}
		return nil
	}
	if strings.TrimSpace(p.HeadsHash) == "" {
		return errors.New("ledger: checkpoint heads_hash_b64url required")
	}
	if p.HeadsCount <= 0 {
		return errors.New("ledger: checkpoint heads_count required")
	}
	return nil
}

func validateIdentityIntroduce(p IdentityIntroducePayload) error {
	ctx := strings.TrimSpace(p.Context)
	if ctx == "" {
		return errors.New("ledger: identity.introduce ctx required")
	}
	if len(ctx) > MaxContextLen {
		return errors.New("ledger: identity.introduce ctx too long")
	}
	if strings.TrimSpace(p.SubjectKeyB64) == "" {
		return errors.New("ledger: identity.introduce subject_key_b64url required")
	}
	if len(p.Reason) > MaxReasonLen {
		return errors.New("ledger: identity.introduce reason too long")
	}
	if len(p.SubjectAuthor) > MaxSubjectLen {
		return errors.New("ledger: identity.introduce subject_author too long")
	}
	return nil
}

func validateIdentityRotate(p IdentityRotatePayload) error {
	if strings.TrimSpace(p.PersonaID) == "" {
		return errors.New("ledger: identity.rotate persona_id required")
	}
	if strings.TrimSpace(p.PrevKeyB64) == "" {
		return errors.New("ledger: identity.rotate prev_key_b64url required")
	}
	if strings.TrimSpace(p.NewKeyB64) == "" {
		return errors.New("ledger: identity.rotate new_key_b64url required")
	}
	if p.EffectiveAt < 0 {
		return errors.New("ledger: identity.rotate effective_at invalid")
	}
	return nil
}

func validateIdentityRevoke(p IdentityRevokePayload) error {
	if strings.TrimSpace(p.PersonaID) == "" {
		return errors.New("ledger: identity.revoke persona_id required")
	}
	if strings.TrimSpace(p.KeyB64) == "" {
		return errors.New("ledger: identity.revoke key_b64url required")
	}
	if len(p.Reason) > MaxReasonLen {
		return errors.New("ledger: identity.revoke reason too long")
	}
	if p.EffectiveAt < 0 {
		return errors.New("ledger: identity.revoke effective_at invalid")
	}
	return nil
}

func validateMisbehaviorEquivocation(p MisbehaviorEquivocationPayload) error {
	if strings.TrimSpace(p.OffenderAuthor) == "" {
		return errors.New("ledger: misbehavior offender_author required")
	}
	if strings.TrimSpace(p.OffenderKeyB64) == "" {
		return errors.New("ledger: misbehavior offender_key_b64url required")
	}
	if p.Seq == 0 {
		return errors.New("ledger: misbehavior seq required")
	}
	if strings.TrimSpace(p.EventA) == "" || strings.TrimSpace(p.EventB) == "" {
		return errors.New("ledger: misbehavior event ids required")
	}
	if p.EventA == p.EventB {
		return errors.New("ledger: misbehavior event ids must differ")
	}
	if len(p.Context) > MaxContextLen {
		return errors.New("ledger: misbehavior ctx too long")
	}
	return nil
}

func validateMisbehaviorReplay(p MisbehaviorReplayPayload) error {
	if strings.TrimSpace(p.OffenderAuthor) == "" {
		return errors.New("ledger: misbehavior offender_author required")
	}
	if strings.TrimSpace(p.OffenderKeyB64) == "" {
		return errors.New("ledger: misbehavior offender_key_b64url required")
	}
	if strings.TrimSpace(p.EventID) == "" {
		return errors.New("ledger: misbehavior event_id required")
	}
	if len(p.Context) > MaxContextLen {
		return errors.New("ledger: misbehavior ctx too long")
	}
	return nil
}

func validateMisbehaviorSybil(p MisbehaviorSybilPayload) error {
	if len(p.Context) > MaxContextLen {
		return errors.New("ledger: misbehavior.sybil ctx too long")
	}
	sus := normalizeIDSet(p.SuspectsKeyB64)
	if len(sus) == 0 {
		return errors.New("ledger: misbehavior.sybil suspects required")
	}
	if len(sus) > 64 {
		return errors.New("ledger: misbehavior.sybil too many suspects")
	}
	if len(p.Reason) > MaxReasonLen {
		return errors.New("ledger: misbehavior.sybil reason too long")
	}
	return nil
}

func validateGroupMembership(p GroupMembershipPayload) error {
	if strings.TrimSpace(p.GroupID) == "" {
		return errors.New("ledger: group_id required")
	}
	if len(p.GroupID) > MaxGroupIDLen {
		return errors.New("ledger: group_id too long")
	}
	act := strings.TrimSpace(p.Action)
	if act == "" {
		return errors.New("ledger: action required")
	}
	if len(act) > MaxActionLen {
		return errors.New("ledger: action too long")
	}
	sub := strings.TrimSpace(p.Subject)
	if sub == "" {
		return errors.New("ledger: subject required")
	}
	if len(sub) > MaxSubjectLen {
		return errors.New("ledger: subject too long")
	}
	return nil
}

func validateAgentAction(p AgentActionPayload) error {
	if strings.TrimSpace(p.AgentID) == "" {
		return errors.New("ledger: agent_id required")
	}
	if len(p.AgentID) > MaxAgentIDLen {
		return errors.New("ledger: agent_id too long")
	}
	act := strings.TrimSpace(p.Action)
	if act == "" {
		return errors.New("ledger: action required")
	}
	if len(act) > MaxActionLen {
		return errors.New("ledger: action too long")
	}
	return nil
}

func validateSyncSummary(p SyncSummaryPayload) error {
	ctx := strings.TrimSpace(p.Context)
	if ctx == "" {
		return errors.New("ledger: sync.summary ctx required")
	}
	if len(ctx) > MaxContextLen {
		return errors.New("ledger: sync.summary ctx too long")
	}
	if strings.TrimSpace(p.HeadsHash) == "" {
		return errors.New("ledger: sync.summary heads_hash_b64url required")
	}
	if p.HeadCount <= 0 {
		return errors.New("ledger: sync.summary head_count required")
	}
	return nil
}

func ComputeHeadsDigest(existingHash string, existingCount int, heads []string) (string, int, error) {
	if existingHash != "" {
		if _, err := base64.RawURLEncoding.DecodeString(existingHash); err != nil {
			return "", 0, fmt.Errorf("ledger: decode heads_hash_b64url: %w", err)
		}
		if existingCount < 0 {
			return "", 0, errors.New("ledger: heads_count invalid")
		}
		return existingHash, existingCount, nil
	}
	if len(heads) == 0 {
		return "", 0, errors.New("ledger: heads required to compute digest")
	}
	cp := append([]string(nil), heads...)
	sort.Strings(cp)
	h := sha256.Sum256([]byte(strings.Join(cp, "\n")))
	return base64.RawURLEncoding.EncodeToString(h[:]), len(cp), nil
}

func ComputeHeadsHash(heads []string) (string, int) {
	cp := append([]string(nil), heads...)
	sort.Strings(cp)
	h := sha256.Sum256([]byte(strings.Join(cp, "\n")))
	return base64.RawURLEncoding.EncodeToString(h[:]), len(cp)
}
