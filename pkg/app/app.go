package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/chat"
	"github.com/kaveh/sunlionet-agent/pkg/identity"
	"github.com/kaveh/sunlionet-agent/pkg/ledger"
	"github.com/kaveh/sunlionet-agent/pkg/ledgersync"
	"github.com/kaveh/sunlionet-agent/pkg/messaging"
	"github.com/kaveh/sunlionet-agent/pkg/relay"
)

const (
	KindChatMessage = "chat.message"
)

const (
	DefaultMaxViewEvents        = 200000
	DefaultMaxEnvelopeBytes     = 256 * 1024
	DefaultMaxGroupParticipants = 128
)

type ChatMessageEvent struct {
	ChatID         string `json:"chat_id"`
	PayloadRef     string `json:"payload_ref"`
	PayloadHashB64 string `json:"payload_hash_b64url"`
	FromPubB64     string `json:"from_pub_b64url"`
	ToPubB64       string `json:"to_pub_b64url"`
	SenderMailbox  string `json:"sender_mailbox,omitempty"`
	ClientID       string `json:"client_id,omitempty"`
}

type Config struct {
	Persona *identity.Persona

	Ledger       *ledger.Ledger
	LedgerPolicy *ledger.Policy
	LedgerStore  *ledger.Store

	Sync *ledgersync.Service

	Payloads *PayloadStore

	PreKeyPrivs func() ([][32]byte, error)

	AgentPolicy AgentPolicy

	MaxViewEvents        int
	MaxEnvelopeBytes     int
	MaxGroupParticipants int
}

type AgentPolicy struct {
	Enabled bool

	AllowReadChats     bool
	AllowWriteChats    bool
	AllowGroupActions  bool
	AllowAssistActions bool

	AllowedActions []string
}

type App struct {
	persona *identity.Persona

	ledger       *ledger.Ledger
	ledgerPolicy *ledger.Policy
	ledgerStore  *ledger.Store

	sync *ledgersync.Service

	payloads *PayloadStore

	preKeyPrivs func() ([][32]byte, error)

	agentPolicy          agentPolicy
	maxViewEvents        int
	maxEnvelopeBytes     int
	maxGroupParticipants int
	stats                appStats

	mu sync.Mutex
}

type AppStats struct {
	PayloadMissing       uint64
	PayloadHashMismatch  uint64
	PayloadDecryptFailed uint64
	OversizeEnvelopeDrop uint64
	AgentDisabled        uint64
	AgentActionBlocked   uint64
	AgentActionAccepted  uint64
	PayloadsPruned       uint64
	PayloadBytesPruned   uint64
}

type appStats struct {
	payloadMissing       atomic.Uint64
	payloadHashMismatch  atomic.Uint64
	payloadDecryptFailed atomic.Uint64
	oversizeEnvelopeDrop atomic.Uint64
	agentDisabled        atomic.Uint64
	agentActionBlocked   atomic.Uint64
	agentActionAccepted  atomic.Uint64
	payloadsPruned       atomic.Uint64
	payloadBytesPruned   atomic.Uint64
}

type agentPolicy struct {
	enabled        bool
	allowReadChat  bool
	allowWriteChat bool
	allowGroup     bool
	allowAssist    bool
	allowedActions map[string]struct{}
}

func New(cfg Config) (*App, error) {
	if cfg.Persona == nil {
		return nil, errors.New("app: persona is nil")
	}
	if cfg.Ledger == nil {
		return nil, errors.New("app: ledger is nil")
	}
	if cfg.Payloads == nil {
		return nil, errors.New("app: payload store is nil")
	}
	if cfg.PreKeyPrivs == nil {
		cfg.PreKeyPrivs = func() ([][32]byte, error) { return nil, nil }
	}
	ap := normalizeAgentPolicy(cfg.AgentPolicy)
	maxViewEvents := cfg.MaxViewEvents
	if maxViewEvents <= 0 {
		maxViewEvents = DefaultMaxViewEvents
	}
	maxEnvelopeBytes := cfg.MaxEnvelopeBytes
	if maxEnvelopeBytes <= 0 {
		maxEnvelopeBytes = DefaultMaxEnvelopeBytes
	}
	maxGroupParticipants := cfg.MaxGroupParticipants
	if maxGroupParticipants <= 0 {
		maxGroupParticipants = DefaultMaxGroupParticipants
	}
	return &App{
		persona:              cfg.Persona,
		ledger:               cfg.Ledger,
		ledgerPolicy:         cfg.LedgerPolicy,
		ledgerStore:          cfg.LedgerStore,
		sync:                 cfg.Sync,
		payloads:             cfg.Payloads,
		preKeyPrivs:          cfg.PreKeyPrivs,
		agentPolicy:          ap,
		maxViewEvents:        maxViewEvents,
		maxEnvelopeBytes:     maxEnvelopeBytes,
		maxGroupParticipants: maxGroupParticipants,
	}, nil
}

func Open(cfg Config) (*App, error) {
	if cfg.Ledger == nil {
		if cfg.LedgerStore == nil {
			return nil, errors.New("app: ledger is nil")
		}
		snap, err := cfg.LedgerStore.Load()
		if err != nil {
			return nil, err
		}
		l, err := ledger.NewFromSnapshot(snap)
		if err != nil {
			return nil, err
		}
		cfg.Ledger = l
	}
	return New(cfg)
}

func (a *App) SelfSignPubB64() string {
	if a == nil || a.persona == nil {
		return ""
	}
	return a.persona.SignPubB64
}

func (a *App) Stats() AppStats {
	if a == nil {
		return AppStats{}
	}
	return AppStats{
		PayloadMissing:       a.stats.payloadMissing.Load(),
		PayloadHashMismatch:  a.stats.payloadHashMismatch.Load(),
		PayloadDecryptFailed: a.stats.payloadDecryptFailed.Load(),
		OversizeEnvelopeDrop: a.stats.oversizeEnvelopeDrop.Load(),
		AgentDisabled:        a.stats.agentDisabled.Load(),
		AgentActionBlocked:   a.stats.agentActionBlocked.Load(),
		AgentActionAccepted:  a.stats.agentActionAccepted.Load(),
		PayloadsPruned:       a.stats.payloadsPruned.Load(),
		PayloadBytesPruned:   a.stats.payloadBytesPruned.Load(),
	}
}

func (a *App) SendMessage(ctx context.Context, r relay.Relay, to Contact, text string) (string, error) {
	if a == nil {
		return "", errors.New("app: app is nil")
	}
	if r == nil {
		return "", errors.New("app: relay is nil")
	}
	if strings.TrimSpace(to.Mailbox) == "" {
		return "", errors.New("app: contact mailbox required")
	}
	if strings.TrimSpace(to.PreKeyPubB64) == "" {
		return "", errors.New("app: contact prekey required")
	}
	if strings.TrimSpace(to.SignPubB64) == "" {
		return "", errors.New("app: contact sign pub required")
	}

	_, priv, err := a.persona.SignKeypair()
	if err != nil {
		return "", err
	}

	toPreKey, err := decodePreKeyPub(to.PreKeyPubB64)
	if err != nil {
		return "", err
	}

	selfID := chat.ContactIDFromSignPub(a.persona.SignPubB64)
	toID := chat.ContactIDFromSignPub(to.SignPubB64)
	chatID := string(chat.DirectChatID(selfID, toID))

	clientID := base64.RawURLEncoding.EncodeToString(random16())
	payload, err := chat.EncodeText(a.persona.SignPubB64, ed25519.PrivateKey(priv), chatID, text, time.Now())
	if err != nil {
		return "", err
	}

	env, _, err := messaging.EncryptToPreKey(payload, toPreKey)
	if err != nil {
		return "", err
	}
	envStr, err := env.Encode()
	if err != nil {
		return "", err
	}
	if a.maxEnvelopeBytes > 0 && len(envStr) > a.maxEnvelopeBytes {
		return "", errors.New("app: envelope too large")
	}
	payloadHashB64 := sha256B64Bytes([]byte(envStr))
	payloadRef := "payload:sha256:" + payloadHashB64

	now := time.Now()

	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.payloads.Put(payloadRef, []byte(envStr)); err != nil {
		return "", err
	}
	if err := a.payloads.Put("outbox:"+payloadHashB64, payload); err != nil {
		return "", err
	}

	evID, err := a.appendChatMessageEventLocked(ChatMessageEvent{
		ChatID:         chatID,
		PayloadRef:     payloadRef,
		PayloadHashB64: payloadHashB64,
		FromPubB64:     a.persona.SignPubB64,
		ToPubB64:       to.SignPubB64,
		SenderMailbox:  "",
		ClientID:       clientID,
	}, now)
	if err != nil {
		return "", err
	}

	if _, err := r.Push(ctx, relay.PushRequest{
		Mailbox:  relay.MailboxID(to.Mailbox),
		Envelope: relay.Envelope(envStr),
	}); err != nil {
		return evID, err
	}
	return evID, nil
}

func (a *App) HandleRelayEnvelope(ctx context.Context, msg relay.Message, envelope string, plaintext []byte) error {
	_ = ctx
	_ = msg
	if a == nil {
		return errors.New("app: app is nil")
	}
	if a.maxEnvelopeBytes > 0 && len(envelope) > a.maxEnvelopeBytes {
		a.stats.oversizeEnvelopeDrop.Add(1)
		return nil
	}
	if len(plaintext) == 0 {
		return nil
	}
	p, ok, err := chat.DecodeAndVerify(plaintext)
	if err != nil || !ok {
		return nil
	}
	switch p.Kind {
	case chat.KindText, chat.KindGroup:
	default:
		return nil
	}
	hashB64 := sha256B64Bytes([]byte(envelope))
	ref := "payload:sha256:" + hashB64
	if a.payloads.Has(ref) {
		return nil
	}
	return a.payloads.Put(ref, []byte(envelope))
}

func (a *App) ListMessages(chatID string) ([]Message, error) {
	if a == nil {
		return nil, errors.New("app: app is nil")
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil, errors.New("app: chat_id required")
	}
	privs, err := a.preKeyPrivs()
	if err != nil {
		return nil, err
	}
	self := a.SelfSignPubB64()

	snap := a.ledger.Snapshot()
	evs := snap.Events
	if a.maxViewEvents > 0 && len(evs) > a.maxViewEvents {
		evs = evs[len(evs)-a.maxViewEvents:]
	}
	msgs := make([]Message, 0, 32)
	for i := range evs {
		ev := evs[i]
		if ev.Kind != KindChatMessage {
			continue
		}
		raw, ok, err := ledger.DecodeInlinePayloadRef(ev.PayloadRef, ledger.MaxInlinePayloadBytes)
		if err != nil || !ok || len(raw) == 0 {
			continue
		}
		var pl ChatMessageEvent
		if err := json.Unmarshal(raw, &pl); err != nil {
			continue
		}
		if strings.TrimSpace(pl.ChatID) != chatID {
			continue
		}
		dir := "in"
		if strings.TrimSpace(pl.FromPubB64) == self {
			dir = "out"
		}

		var text string
		state := "pending"
		payloadAvailable := false
		payloadVerified := false
		switch dir {
		case "out":
			if strings.TrimSpace(pl.PayloadHashB64) != "" {
				if b, ok, _ := a.payloads.Get("outbox:" + pl.PayloadHashB64); ok {
					payloadAvailable = true
					payloadVerified = true
					if sp, ok2, _ := chat.DecodeAndVerify(b); ok2 {
						if t, err := extractText(sp); err == nil {
							text = t
						}
					}
				}
			}
			state = "pending"
		default:
			if strings.TrimSpace(pl.PayloadRef) != "" {
				if envBytes, ok, _ := a.payloads.Get(pl.PayloadRef); ok {
					payloadAvailable = true
					if strings.TrimSpace(pl.PayloadHashB64) != "" {
						got := sha256B64Bytes(envBytes)
						if got != strings.TrimSpace(pl.PayloadHashB64) {
							a.stats.payloadHashMismatch.Add(1)
							state = "blocked"
							payloadVerified = false
							break
						}
					}
					payloadVerified = true
					t, ok := decryptEnvelopeText(string(envBytes), privs)
					if ok {
						text = t
						state = "synced"
					} else {
						a.stats.payloadDecryptFailed.Add(1)
						state = "blocked"
					}
				} else {
					a.stats.payloadMissing.Add(1)
					state = "waiting_for_payload"
				}
			} else {
				state = "blocked"
			}
		}

		msgs = append(msgs, Message{
			EventID:          ev.ID,
			ChatID:           pl.ChatID,
			CreatedAt:        ev.CreatedAt,
			SenderPubB64:     pl.FromPubB64,
			Text:             text,
			State:            state,
			Direction:        dir,
			PayloadRef:       pl.PayloadRef,
			PayloadHashB64:   pl.PayloadHashB64,
			PayloadAvailable: payloadAvailable,
			PayloadVerified:  payloadVerified,
		})
	}
	sort.Slice(msgs, func(i, j int) bool {
		if msgs[i].CreatedAt == msgs[j].CreatedAt {
			return msgs[i].EventID < msgs[j].EventID
		}
		return msgs[i].CreatedAt < msgs[j].CreatedAt
	})
	return msgs, nil
}

func (a *App) CreateGroup(now time.Time) (string, error) {
	if a == nil {
		return "", errors.New("app: app is nil")
	}
	if now.IsZero() {
		now = time.Now()
	}
	groupID := base64.RawURLEncoding.EncodeToString(random16())
	a.mu.Lock()
	defer a.mu.Unlock()
	_, err := a.appendGroupMembershipEventLocked(groupID, "create", a.persona.SignPubB64, "owner", now)
	return groupID, err
}

func (a *App) JoinGroup(groupID string, now time.Time) error {
	if a == nil {
		return errors.New("app: app is nil")
	}
	if now.IsZero() {
		now = time.Now()
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return errors.New("app: group_id required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_, err := a.appendGroupMembershipEventLocked(groupID, "join", a.persona.SignPubB64, "member", now)
	return err
}

func (a *App) LeaveGroup(groupID string, now time.Time) error {
	if a == nil {
		return errors.New("app: app is nil")
	}
	if now.IsZero() {
		now = time.Now()
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return errors.New("app: group_id required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_, err := a.appendGroupMembershipEventLocked(groupID, "leave", a.persona.SignPubB64, "", now)
	return err
}

func (a *App) SendGroupMessage(ctx context.Context, r relay.Relay, groupID string, text string, participants []Contact) (string, error) {
	if a == nil {
		return "", errors.New("app: app is nil")
	}
	if r == nil {
		return "", errors.New("app: relay is nil")
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return "", errors.New("app: group_id required")
	}
	if len(participants) == 0 {
		return "", errors.New("app: participants required")
	}
	if a.maxGroupParticipants > 0 && len(participants) > a.maxGroupParticipants {
		return "", errors.New("app: too many participants")
	}
	if !a.IsGroupMember(groupID, a.persona.SignPubB64) {
		return "", errors.New("app: not a group member")
	}
	_, priv, err := a.persona.SignKeypair()
	if err != nil {
		return "", err
	}
	payload, err := chat.EncodeGroupText(a.persona.SignPubB64, ed25519.PrivateKey(priv), groupID, text, time.Now())
	if err != nil {
		return "", err
	}

	var firstRef string
	var firstHash string
	for i := range participants {
		to := participants[i]
		if strings.TrimSpace(to.Mailbox) == "" || strings.TrimSpace(to.PreKeyPubB64) == "" || strings.TrimSpace(to.SignPubB64) == "" {
			continue
		}
		preKey, err := decodePreKeyPub(to.PreKeyPubB64)
		if err != nil {
			continue
		}
		env, _, err := messaging.EncryptToPreKey(payload, preKey)
		if err != nil {
			continue
		}
		envStr, err := env.Encode()
		if err != nil {
			continue
		}
		if a.maxEnvelopeBytes > 0 && len(envStr) > a.maxEnvelopeBytes {
			continue
		}
		hashB64 := sha256B64Bytes([]byte(envStr))
		ref := "payload:sha256:" + hashB64
		a.mu.Lock()
		_ = a.payloads.Put(ref, []byte(envStr))
		a.mu.Unlock()
		_, _ = r.Push(ctx, relay.PushRequest{
			Mailbox:  relay.MailboxID(to.Mailbox),
			Envelope: relay.Envelope(envStr),
		})
		if firstRef == "" {
			firstRef = ref
			firstHash = hashB64
		}
	}
	if firstRef == "" {
		return "", errors.New("app: no valid participants")
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	return a.appendChatMessageEventLocked(ChatMessageEvent{
		ChatID:         "g:" + groupID,
		PayloadRef:     firstRef,
		PayloadHashB64: firstHash,
		FromPubB64:     a.persona.SignPubB64,
		ToPubB64:       "",
		SenderMailbox:  "",
		ClientID:       base64.RawURLEncoding.EncodeToString(random16()),
	}, time.Now())
}

func (a *App) AgentReply(now time.Time, agentID string, chatID string, ref string) (string, error) {
	if a == nil {
		return "", errors.New("app: app is nil")
	}
	if now.IsZero() {
		now = time.Now()
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return "", errors.New("app: agent_id required")
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return "", errors.New("app: chat_id required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.appendAgentActionLocked(agentID, "reply", "chat", chatID, ref, now)
}

func (a *App) appendChatMessageEventLocked(pl ChatMessageEvent, now time.Time) (string, error) {
	if now.IsZero() {
		now = time.Now()
	}
	raw, err := json.Marshal(pl)
	if err != nil {
		return "", err
	}
	_, inlineRef, err := ledger.InlinePayloadRef(raw)
	if err != nil {
		return "", err
	}
	ev, err := a.newSignedEventLocked(KindChatMessage, raw, inlineRef, now)
	if err != nil {
		return "", err
	}
	if _, err := a.ledger.Apply(ev, a.ledgerPolicy, nil); err != nil {
		return "", err
	}
	if a.ledgerStore != nil {
		_ = a.ledgerStore.Save(a.ledger.Snapshot())
	}
	return ev.ID, nil
}

func (a *App) AgentAction(now time.Time, agentID string, action string, scope string, target string, ref string) (string, error) {
	if a == nil {
		return "", errors.New("app: app is nil")
	}
	if now.IsZero() {
		now = time.Now()
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return "", errors.New("app: agent_id required")
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return "", errors.New("app: action required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.appendAgentActionLocked(agentID, action, scope, target, ref, now)
}

func (a *App) appendGroupMembershipEventLocked(groupID string, action string, subjectPubB64 string, role string, now time.Time) (string, error) {
	if now.IsZero() {
		now = time.Now()
	}
	raw, err := json.Marshal(ledger.GroupMembershipPayload{
		GroupID:  groupID,
		Action:   strings.TrimSpace(action),
		Subject:  strings.TrimSpace(subjectPubB64),
		Role:     strings.TrimSpace(role),
		GroupVer: 0,
	})
	if err != nil {
		return "", err
	}
	_, inlineRef, err := ledger.InlinePayloadRef(raw)
	if err != nil {
		return "", err
	}
	ev, err := a.newSignedEventLocked(ledger.KindGroupMembership, raw, inlineRef, now)
	if err != nil {
		return "", err
	}
	if _, err := a.ledger.Apply(ev, a.ledgerPolicy, nil); err != nil {
		return "", err
	}
	if a.ledgerStore != nil {
		_ = a.ledgerStore.Save(a.ledger.Snapshot())
	}
	return ev.ID, nil
}

func (a *App) appendAgentActionLocked(agentID string, action string, scope string, target string, ref string, now time.Time) (string, error) {
	if !a.agentPolicy.enabled {
		a.stats.agentDisabled.Add(1)
		return "", errors.New("app: agent disabled")
	}

	action = strings.TrimSpace(action)
	scope = strings.TrimSpace(scope)
	target = strings.TrimSpace(target)
	ref = strings.TrimSpace(ref)

	cat, key, ok := classifyAgentAction(scope, action)
	if !ok {
		a.stats.agentActionBlocked.Add(1)
		return "", errors.New("app: agent action not allowed")
	}
	if !a.agentPolicy.allowed(key, action, cat) {
		a.stats.agentActionBlocked.Add(1)
		return "", errors.New("app: agent action not allowed")
	}
	a.stats.agentActionAccepted.Add(1)
	raw, err := json.Marshal(ledger.AgentActionPayload{
		AgentID: strings.TrimSpace(agentID),
		Action:  action,
		Scope:   scope,
		Target:  target,
		Ref:     ref,
	})
	if err != nil {
		return "", err
	}
	_, inlineRef, err := ledger.InlinePayloadRef(raw)
	if err != nil {
		return "", err
	}
	ev, err := a.newSignedEventLocked(ledger.KindAgentAction, raw, inlineRef, now)
	if err != nil {
		return "", err
	}
	if _, err := a.ledger.Apply(ev, a.ledgerPolicy, nil); err != nil {
		return "", err
	}
	if a.ledgerStore != nil {
		_ = a.ledgerStore.Save(a.ledger.Snapshot())
	}
	return ev.ID, nil
}

func (a *App) PublishAllEventsToRelay(ctx context.Context, r relay.Relay, mailbox relay.MailboxID, recipientPreKeyPub [32]byte, syncContext string, ttlSec int, powBits int) (int, error) {
	if a == nil {
		return 0, errors.New("app: app is nil")
	}
	if a.sync == nil {
		return 0, errors.New("app: sync is nil")
	}
	snap := a.ledger.Snapshot()
	evs := snap.Events
	if len(evs) > 5000 {
		evs = evs[len(evs)-5000:]
	}
	sort.SliceStable(evs, func(i, j int) bool {
		pi := relayEventPriority(evs[i].Kind)
		pj := relayEventPriority(evs[j].Kind)
		if pi != pj {
			return pi < pj
		}
		if evs[i].CreatedAt != evs[j].CreatedAt {
			return evs[i].CreatedAt > evs[j].CreatedAt
		}
		return evs[i].ID > evs[j].ID
	})
	want := make([]string, 0, len(evs))
	for i := range evs {
		want = append(want, evs[i].ID)
	}
	return a.sync.PublishEventsToRelay(ctx, r, mailbox, recipientPreKeyPub, syncContext, want, ttlSec, powBits)
}

func relayEventPriority(kind string) int {
	switch strings.TrimSpace(kind) {
	case ledger.KindIdentityRevoke, ledger.KindIdentityRotate:
		return 0
	case ledger.KindMisbehaviorEquivoc, ledger.KindMisbehaviorReplay:
		return 1
	case ledger.KindGroupMembership, ledger.KindGroupCreate, ledger.KindGroupJoin:
		return 2
	case ledger.KindAgentAction:
		return 3
	case ledger.KindChatMessage:
		return 4
	case ledger.KindWitnessCheckpoint, ledger.KindWitnessAttest:
		return 6
	case ledger.KindSyncSummary, ledger.KindLedgerEvent:
		return 7
	default:
		return 10
	}
}

type RetentionPolicy struct {
	KeepUnreferencedFor time.Duration
	MaxPayloadFiles     int
	MaxPayloadBytes     int64

	MaxScanEvents int
}

type RetentionReport struct {
	ReferencedPayloadKeys int
	ScannedEvents         int
	Payloads              PruneReport
}

func (a *App) Prune(now time.Time, pol RetentionPolicy) (RetentionReport, error) {
	if a == nil {
		return RetentionReport{}, errors.New("app: app is nil")
	}
	if a.payloads == nil {
		return RetentionReport{}, errors.New("app: payload store is nil")
	}
	if now.IsZero() {
		now = time.Now()
	}
	maxScan := pol.MaxScanEvents
	if maxScan <= 0 {
		maxScan = 50000
	}

	snap := a.ledger.Snapshot()
	evs := snap.Events
	if len(evs) > maxScan {
		evs = evs[len(evs)-maxScan:]
	}

	keep := map[string]struct{}{}
	for i := range evs {
		ev := evs[i]
		if ev.Kind != KindChatMessage {
			continue
		}
		raw, ok, err := ledger.DecodeInlinePayloadRef(ev.PayloadRef, ledger.MaxInlinePayloadBytes)
		if err != nil || !ok || len(raw) == 0 {
			continue
		}
		var pl ChatMessageEvent
		if err := json.Unmarshal(raw, &pl); err != nil {
			continue
		}
		if strings.TrimSpace(pl.PayloadRef) != "" {
			keep[strings.TrimSpace(pl.PayloadRef)] = struct{}{}
		}
		if strings.TrimSpace(pl.PayloadHashB64) != "" {
			keep["outbox:"+strings.TrimSpace(pl.PayloadHashB64)] = struct{}{}
		}
	}

	keys := make([]string, 0, len(keep))
	for k := range keep {
		keys = append(keys, k)
	}
	rep, err := a.payloads.Prune(now, keys, pol.KeepUnreferencedFor, pol.MaxPayloadFiles, pol.MaxPayloadBytes)
	if err != nil {
		return RetentionReport{}, err
	}
	if rep.Deleted > 0 {
		a.stats.payloadsPruned.Add(uint64(rep.Deleted))
		a.stats.payloadBytesPruned.Add(uint64(rep.BytesDeleted))
	}
	return RetentionReport{
		ReferencedPayloadKeys: len(keys),
		ScannedEvents:         len(evs),
		Payloads:              rep,
	}, nil
}

func (a *App) PullAndApplyFromRelay(ctx context.Context, r relay.Relay, mailbox relay.MailboxID, recipientPreKeyPriv [32]byte, limit int, ack bool) (ledger.ApplyReport, int, error) {
	if a == nil {
		return ledger.ApplyReport{}, 0, errors.New("app: app is nil")
	}
	if a.sync == nil {
		return ledger.ApplyReport{}, 0, errors.New("app: sync is nil")
	}
	rep, n, err := a.sync.PullAndApplyFromRelay(ctx, r, mailbox, recipientPreKeyPriv, limit, ack)
	if err != nil {
		return rep, n, err
	}
	if a.ledgerStore != nil {
		_ = a.ledgerStore.Save(a.ledger.Snapshot())
	}
	return rep, n, nil
}

func (a *App) ListGroups() ([]string, error) {
	if a == nil {
		return nil, errors.New("app: app is nil")
	}
	snap := a.ledger.Snapshot()
	seen := map[string]struct{}{}
	for i := range snap.Events {
		ev := snap.Events[i]
		if ev.Kind != ledger.KindGroupMembership {
			continue
		}
		raw, ok, err := ledger.DecodeInlinePayloadRef(ev.PayloadRef, ledger.MaxInlinePayloadBytes)
		if err != nil || !ok || len(raw) == 0 {
			continue
		}
		var pl ledger.GroupMembershipPayload
		if err := json.Unmarshal(raw, &pl); err != nil {
			continue
		}
		gid := strings.TrimSpace(pl.GroupID)
		if gid == "" {
			continue
		}
		seen[gid] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for gid := range seen {
		out = append(out, gid)
	}
	sort.Strings(out)
	return out, nil
}

func (a *App) IsGroupMember(groupID string, subjectPubB64 string) bool {
	m, err := a.GroupMembers(groupID)
	if err != nil || m == nil {
		return false
	}
	_, ok := m[strings.TrimSpace(subjectPubB64)]
	return ok
}

func (a *App) GroupMembers(groupID string) (map[string]string, error) {
	if a == nil {
		return nil, errors.New("app: app is nil")
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, errors.New("app: group_id required")
	}
	snap := a.ledger.Snapshot()
	evs := append([]ledger.Event(nil), snap.Events...)
	sort.Slice(evs, func(i, j int) bool {
		if evs[i].CreatedAt == evs[j].CreatedAt {
			return evs[i].ID < evs[j].ID
		}
		return evs[i].CreatedAt < evs[j].CreatedAt
	})
	out := map[string]string{}
	for i := range evs {
		ev := evs[i]
		if ev.Kind != ledger.KindGroupMembership {
			continue
		}
		raw, ok, err := ledger.DecodeInlinePayloadRef(ev.PayloadRef, ledger.MaxInlinePayloadBytes)
		if err != nil || !ok || len(raw) == 0 {
			continue
		}
		var pl ledger.GroupMembershipPayload
		if err := json.Unmarshal(raw, &pl); err != nil {
			continue
		}
		if strings.TrimSpace(pl.GroupID) != groupID {
			continue
		}
		sub := strings.TrimSpace(pl.Subject)
		act := strings.TrimSpace(pl.Action)
		if sub == "" || act == "" {
			continue
		}
		if sub != strings.TrimSpace(ev.AuthorKeyB64) {
			continue
		}
		switch act {
		case "create":
			out[sub] = "owner"
		case "join":
			role := strings.TrimSpace(pl.Role)
			if role == "" {
				role = "member"
			}
			out[sub] = role
		case "leave":
			delete(out, sub)
		default:
		}
	}
	return out, nil
}

func (a *App) newSignedEventLocked(kind string, payload json.RawMessage, payloadRef string, now time.Time) (ledger.Event, error) {
	pub, priv, err := a.persona.SignKeypair()
	if err != nil {
		return ledger.Event{}, err
	}
	seq, prev := a.nextSeqPrevLocked(string(a.persona.ID))
	parents := a.ledger.Heads()
	return ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(a.persona.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        seq,
		Prev:       prev,
		Parents:    parents,
		Kind:       kind,
		Payload:    payload,
		PayloadRef: payloadRef,
		CreatedAt:  now,
	})
}

func (a *App) nextSeqPrevLocked(author string) (uint64, string) {
	snap := a.ledger.Snapshot()
	var max uint64
	var prev string
	for i := range snap.Events {
		ev := snap.Events[i]
		if ev.Author != author {
			continue
		}
		if ev.Seq > max {
			max = ev.Seq
			prev = ev.ID
		}
	}
	if max == 0 {
		return 1, ""
	}
	return max + 1, prev
}

func decodePreKeyPub(b64 string) ([32]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return [32]byte{}, fmt.Errorf("app: decode prekey pub: %w", err)
	}
	if len(b) != 32 {
		return [32]byte{}, fmt.Errorf("app: invalid prekey pub size: %d", len(b))
	}
	var out [32]byte
	copy(out[:], b)
	return out, nil
}

func sha256B64Bytes(b []byte) string {
	sum := sha256.Sum256(b)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func decryptEnvelopeText(envStr string, privs [][32]byte) (string, bool) {
	env, err := messaging.DecodeEnvelope(strings.TrimSpace(envStr))
	if err != nil {
		return "", false
	}
	for i := range privs {
		pt, _, err := messaging.DecryptWithPreKey(env, privs[i])
		if err != nil {
			continue
		}
		p, ok, err := chat.DecodeAndVerify(pt)
		if err != nil || !ok {
			continue
		}
		text, err := extractText(p)
		if err != nil {
			continue
		}
		return text, true
	}
	return "", false
}

func extractText(p chat.SignedPayload) (string, error) {
	switch p.Kind {
	case chat.KindText:
		var b chat.TextBody
		if err := json.Unmarshal(p.Body, &b); err != nil {
			return "", err
		}
		return b.Text, nil
	case chat.KindGroup:
		var b chat.GroupTextBody
		if err := json.Unmarshal(p.Body, &b); err != nil {
			return "", err
		}
		return b.Text, nil
	default:
		return "", errors.New("app: unsupported payload kind")
	}
}

func random16() []byte {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return b[:]
}

func normalizeAgentPolicy(p AgentPolicy) agentPolicy {
	out := agentPolicy{
		enabled:        p.Enabled,
		allowReadChat:  p.AllowReadChats,
		allowWriteChat: p.AllowWriteChats,
		allowGroup:     p.AllowGroupActions,
		allowAssist:    p.AllowAssistActions,
		allowedActions: map[string]struct{}{},
	}
	for i := range p.AllowedActions {
		act := strings.TrimSpace(p.AllowedActions[i])
		if act == "" {
			continue
		}
		out.allowedActions[act] = struct{}{}
	}
	if !out.enabled {
		out.allowedActions = map[string]struct{}{}
	}
	return out
}

type agentActionCategory string

const (
	agentCatReadChat  agentActionCategory = "read_chat"
	agentCatWriteChat agentActionCategory = "write_chat"
	agentCatGroup     agentActionCategory = "group"
	agentCatAssist    agentActionCategory = "assist"
)

func classifyAgentAction(scope string, action string) (agentActionCategory, string, bool) {
	scope = strings.TrimSpace(scope)
	action = strings.TrimSpace(action)
	if scope == "" || action == "" {
		return "", "", false
	}
	switch scope {
	case "chat":
		switch action {
		case "read", "summarize":
			return agentCatReadChat, "chat." + action, true
		case "reply", "send":
			return agentCatWriteChat, "chat." + action, true
		case "suggest", "draft":
			return agentCatAssist, "chat." + action, true
		default:
			return "", "", false
		}
	case "group":
		switch action {
		case "send", "invite", "join", "leave":
			return agentCatGroup, "group." + action, true
		default:
			return "", "", false
		}
	default:
		return "", "", false
	}
}

func (p agentPolicy) allowed(canonicalKey string, rawAction string, cat agentActionCategory) bool {
	if !p.enabled {
		return false
	}
	if _, ok := p.allowedActions[canonicalKey]; ok {
		return true
	}
	if _, ok := p.allowedActions[strings.TrimSpace(rawAction)]; ok {
		return true
	}
	switch cat {
	case agentCatReadChat:
		return p.allowReadChat
	case agentCatWriteChat:
		return p.allowWriteChat
	case agentCatGroup:
		return p.allowGroup
	case agentCatAssist:
		return p.allowAssist
	default:
		return false
	}
}
