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
}

type AgentPolicy struct {
	Enabled        bool
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

	agentPolicy agentPolicy

	mu sync.Mutex
}

type agentPolicy struct {
	enabled        bool
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
	return &App{
		persona:      cfg.Persona,
		ledger:       cfg.Ledger,
		ledgerPolicy: cfg.LedgerPolicy,
		ledgerStore:  cfg.LedgerStore,
		sync:         cfg.Sync,
		payloads:     cfg.Payloads,
		preKeyPrivs:  cfg.PreKeyPrivs,
		agentPolicy:  ap,
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
	msgs := make([]Message, 0, 32)
	for i := range snap.Events {
		ev := snap.Events[i]
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
		switch dir {
		case "out":
			if strings.TrimSpace(pl.PayloadHashB64) != "" {
				if b, ok, _ := a.payloads.Get("outbox:" + pl.PayloadHashB64); ok {
					if sp, ok2, _ := chat.DecodeAndVerify(b); ok2 {
						if t, err := extractText(sp); err == nil {
							text = t
						}
					}
				}
			}
		default:
			if strings.TrimSpace(pl.PayloadRef) != "" {
				if envBytes, ok, _ := a.payloads.Get(pl.PayloadRef); ok {
					t, _ := decryptEnvelopeText(string(envBytes), privs)
					text = t
				}
			}
		}

		msgs = append(msgs, Message{
			EventID:        ev.ID,
			ChatID:         pl.ChatID,
			CreatedAt:      ev.CreatedAt,
			SenderPubB64:   pl.FromPubB64,
			Text:           text,
			Direction:      dir,
			PayloadRef:     pl.PayloadRef,
			PayloadHashB64: pl.PayloadHashB64,
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
	if a.agentPolicy.enabled {
		if _, ok := a.agentPolicy.allowedActions[strings.TrimSpace(action)]; !ok {
			return "", errors.New("app: agent action not allowed")
		}
	}
	raw, err := json.Marshal(ledger.AgentActionPayload{
		AgentID: strings.TrimSpace(agentID),
		Action:  strings.TrimSpace(action),
		Scope:   strings.TrimSpace(scope),
		Target:  strings.TrimSpace(target),
		Ref:     strings.TrimSpace(ref),
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
	want := make([]string, 0, len(snap.Events))
	for i := range snap.Events {
		want = append(want, snap.Events[i].ID)
	}
	return a.sync.PublishEventsToRelay(ctx, r, mailbox, recipientPreKeyPub, syncContext, want, ttlSec, powBits)
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
