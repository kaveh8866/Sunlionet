package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/chat"
	"github.com/kaveh/sunlionet-agent/pkg/identity"
	"github.com/kaveh/sunlionet-agent/pkg/ledger"
	"github.com/kaveh/sunlionet-agent/pkg/ledgersync"
	"github.com/kaveh/sunlionet-agent/pkg/mesh"
	"github.com/kaveh/sunlionet-agent/pkg/messaging"
	"github.com/kaveh/sunlionet-agent/pkg/relay"
	"github.com/kaveh/sunlionet-agent/pkg/runtimecfg"
)

type pairMesh struct {
	ch    chan []byte
	other *pairMesh
}

func newPairMesh() (*pairMesh, *pairMesh) {
	a := &pairMesh{ch: make(chan []byte, 128)}
	b := &pairMesh{ch: make(chan []byte, 128)}
	a.other = b
	b.other = a
	return a, b
}

func (m *pairMesh) Broadcast(data []byte) error {
	m.other.ch <- append([]byte(nil), data...)
	return nil
}

func (m *pairMesh) Receive(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case b := <-m.ch:
		return b, nil
	}
}

func (m *pairMesh) RuntimeMode() runtimecfg.RuntimeMode { return runtimecfg.ModeSim }

type noopMesh struct{}

func (m *noopMesh) Broadcast(data []byte) error { return nil }
func (m *noopMesh) Receive(ctx context.Context) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (m *noopMesh) RuntimeMode() runtimecfg.RuntimeMode { return runtimecfg.ModeSim }

func syncExchange(t *testing.T, a, b *ledgersync.Service, pa, pb ledgersync.Peer, ctxName string) {
	t.Helper()
	ctx := context.Background()
	_, _ = a.SendHeads(ctx, pb, ctxName)
	_, _ = b.SendHeads(ctx, pa, ctxName)
	for i := 0; i < 20; i++ {
		ra := receiveOnce(a, 25*time.Millisecond)
		rb := receiveOnce(b, 25*time.Millisecond)
		if !ra && !rb {
			break
		}
	}
}

func receiveOnce(s *ledgersync.Service, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := s.ReceiveOnce(ctx); err != nil {
		return false
	}
	return true
}

func TestApp_SecureChat_E2E(t *testing.T) {
	dir := t.TempDir()
	r := relay.NewMemoryRelay()

	alice, _ := identity.NewPersona()
	bob, _ := identity.NewPersona()

	alicePK, _ := identity.NewPreKey(24 * time.Hour)
	bobPK, _ := identity.NewPreKey(24 * time.Hour)

	alicePriv, _ := alicePK.DecodePrivate()
	bobPriv, _ := bobPK.DecodePrivate()

	aliceLedger := ledger.New()
	bobLedger := ledger.New()

	mA, mB := newPairMesh()
	cA, _ := mesh.NewCrypto()
	cB, _ := mesh.NewCrypto()

	pol := ledger.ProductionPolicy()
	pol.RequireKnownPrev = false

	sA, _ := ledgersync.New(mA, cA, aliceLedger, &pol, nil, ledgersync.Options{})
	sB, _ := ledgersync.New(mB, cB, bobLedger, &pol, nil, ledgersync.Options{})

	aStore, _ := ledger.NewStore(filepath.Join(dir, "alice_ledger.enc"), make([]byte, 32))
	bStore, _ := ledger.NewStore(filepath.Join(dir, "bob_ledger.enc"), make([]byte, 32))

	aPayloads, _ := NewPayloadStore(filepath.Join(dir, "alice_payloads"), make([]byte, 32))
	bPayloads, _ := NewPayloadStore(filepath.Join(dir, "bob_payloads"), make([]byte, 32))

	appA, err := New(Config{
		Persona:      alice,
		Ledger:       aliceLedger,
		LedgerPolicy: &pol,
		LedgerStore:  aStore,
		Sync:         sA,
		Payloads:     aPayloads,
		PreKeyPrivs: func() ([][32]byte, error) {
			return [][32]byte{alicePriv}, nil
		},
	})
	if err != nil {
		t.Fatalf("new app alice: %v", err)
	}
	appB, err := New(Config{
		Persona:      bob,
		Ledger:       bobLedger,
		LedgerPolicy: &pol,
		LedgerStore:  bStore,
		Sync:         sB,
		Payloads:     bPayloads,
		PreKeyPrivs: func() ([][32]byte, error) {
			return [][32]byte{bobPriv}, nil
		},
	})
	if err != nil {
		t.Fatalf("new app bob: %v", err)
	}

	toBob := Contact{
		ID:           "bob",
		Alias:        "Bob",
		SignPubB64:   bob.SignPubB64,
		Mailbox:      "mb-bob",
		PreKeyPubB64: bobPK.PubB64,
	}

	evID, err := appA.SendMessage(context.Background(), r, toBob, "hi bob")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if evID == "" {
		t.Fatalf("expected event id")
	}

	msgs, err := r.Pull(context.Background(), relay.PullRequest{Mailbox: relay.MailboxID("mb-bob"), Limit: 10})
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 relay msg, got %d", len(msgs))
	}
	pt, _, err := ledgersync.DecodeFromRelay(string(msgs[0].Envelope), bobPriv)
	if err != nil {
		t.Fatalf("decrypt relay: %v", err)
	}
	if err := appB.HandleRelayEnvelope(context.Background(), msgs[0], string(msgs[0].Envelope), pt); err != nil {
		t.Fatalf("handle relay: %v", err)
	}

	peerA := ledgersync.Peer{ID: "a", MeshPub: cA.PublicKey(), Role: ledgersync.PeerRoleNormal}
	peerB := ledgersync.Peer{ID: "b", MeshPub: cB.PublicKey(), Role: ledgersync.PeerRoleNormal}
	syncExchange(t, sA, sB, peerA, peerB, "chat")

	chatID := string(chat.DirectChatID(chat.ContactIDFromSignPub(alice.SignPubB64), chat.ContactIDFromSignPub(bob.SignPubB64)))
	got, err := appB.ListMessages(chatID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(got))
	}
	if got[0].Text != "hi bob" {
		t.Fatalf("expected text %q, got %q", "hi bob", got[0].Text)
	}
	if got[0].Direction != "in" {
		t.Fatalf("expected direction in, got %q", got[0].Direction)
	}
}

func TestApp_GroupMessage_E2E(t *testing.T) {
	dir := t.TempDir()
	r := relay.NewMemoryRelay()

	alice, _ := identity.NewPersona()
	bob, _ := identity.NewPersona()

	alicePK, _ := identity.NewPreKey(24 * time.Hour)
	bobPK, _ := identity.NewPreKey(24 * time.Hour)

	alicePriv, _ := alicePK.DecodePrivate()
	bobPriv, _ := bobPK.DecodePrivate()

	aliceLedger := ledger.New()
	bobLedger := ledger.New()

	mA, mB := newPairMesh()
	cA, _ := mesh.NewCrypto()
	cB, _ := mesh.NewCrypto()

	pol := ledger.ProductionPolicy()
	pol.RequireKnownPrev = false

	sA, _ := ledgersync.New(mA, cA, aliceLedger, &pol, nil, ledgersync.Options{})
	sB, _ := ledgersync.New(mB, cB, bobLedger, &pol, nil, ledgersync.Options{})

	aStore, _ := ledger.NewStore(filepath.Join(dir, "alice_ledger.enc"), make([]byte, 32))
	bStore, _ := ledger.NewStore(filepath.Join(dir, "bob_ledger.enc"), make([]byte, 32))

	aPayloads, _ := NewPayloadStore(filepath.Join(dir, "alice_payloads"), make([]byte, 32))
	bPayloads, _ := NewPayloadStore(filepath.Join(dir, "bob_payloads"), make([]byte, 32))

	appA, err := New(Config{
		Persona:      alice,
		Ledger:       aliceLedger,
		LedgerPolicy: &pol,
		LedgerStore:  aStore,
		Sync:         sA,
		Payloads:     aPayloads,
		PreKeyPrivs: func() ([][32]byte, error) {
			return [][32]byte{alicePriv}, nil
		},
	})
	if err != nil {
		t.Fatalf("new app alice: %v", err)
	}
	appB, err := New(Config{
		Persona:      bob,
		Ledger:       bobLedger,
		LedgerPolicy: &pol,
		LedgerStore:  bStore,
		Sync:         sB,
		Payloads:     bPayloads,
		PreKeyPrivs: func() ([][32]byte, error) {
			return [][32]byte{bobPriv}, nil
		},
	})
	if err != nil {
		t.Fatalf("new app bob: %v", err)
	}

	groupID, err := appA.CreateGroup(time.Now())
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	peerA := ledgersync.Peer{ID: "a", MeshPub: cA.PublicKey(), Role: ledgersync.PeerRoleNormal}
	peerB := ledgersync.Peer{ID: "b", MeshPub: cB.PublicKey(), Role: ledgersync.PeerRoleNormal}
	syncExchange(t, sA, sB, peerA, peerB, "group")

	members, err := appB.GroupMembers(groupID)
	if err != nil {
		t.Fatalf("members: %v", err)
	}
	if members[alice.SignPubB64] != "owner" {
		t.Fatalf("expected alice owner membership")
	}

	toBob := Contact{
		ID:           "bob",
		Alias:        "Bob",
		SignPubB64:   bob.SignPubB64,
		Mailbox:      "mb-bob",
		PreKeyPubB64: bobPK.PubB64,
	}

	_, err = appA.SendGroupMessage(context.Background(), r, groupID, "hi group", []Contact{toBob})
	if err != nil {
		t.Fatalf("send group: %v", err)
	}

	msgs, err := r.Pull(context.Background(), relay.PullRequest{Mailbox: relay.MailboxID("mb-bob"), Limit: 10})
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 relay msg, got %d", len(msgs))
	}
	pt, _, err := ledgersync.DecodeFromRelay(string(msgs[0].Envelope), bobPriv)
	if err != nil {
		t.Fatalf("decrypt relay: %v", err)
	}
	if err := appB.HandleRelayEnvelope(context.Background(), msgs[0], string(msgs[0].Envelope), pt); err != nil {
		t.Fatalf("handle relay: %v", err)
	}

	syncExchange(t, sA, sB, peerA, peerB, "group")

	got, err := appB.ListMessages("g:" + groupID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(got))
	}
	if got[0].Text != "hi group" {
		t.Fatalf("expected text %q, got %q", "hi group", got[0].Text)
	}
}

func TestApp_RelayOnly_OfflineSync_AndRecovery(t *testing.T) {
	dir := t.TempDir()
	r := relay.NewMemoryRelay()

	alice, _ := identity.NewPersona()
	bob, _ := identity.NewPersona()

	alicePK, _ := identity.NewPreKey(24 * time.Hour)
	bobPK, _ := identity.NewPreKey(24 * time.Hour)

	alicePriv, _ := alicePK.DecodePrivate()
	bobPriv, _ := bobPK.DecodePrivate()
	bobPub, _ := bobPK.DecodePublic()

	aliceLedger := ledger.New()
	bobLedger := ledger.New()

	cA, _ := mesh.NewCrypto()
	cB, _ := mesh.NewCrypto()

	pol := ledger.ProductionPolicy()
	pol.RequireKnownPrev = false

	sA, _ := ledgersync.New(&noopMesh{}, cA, aliceLedger, &pol, nil, ledgersync.Options{})
	sB, _ := ledgersync.New(&noopMesh{}, cB, bobLedger, &pol, nil, ledgersync.Options{})

	aStore, _ := ledger.NewStore(filepath.Join(dir, "alice_ledger.enc"), make([]byte, 32))
	bStore, _ := ledger.NewStore(filepath.Join(dir, "bob_ledger.enc"), make([]byte, 32))

	aPayloadDir := filepath.Join(dir, "alice_payloads")
	bPayloadDir := filepath.Join(dir, "bob_payloads")
	aPayloads, _ := NewPayloadStore(aPayloadDir, make([]byte, 32))
	bPayloads, _ := NewPayloadStore(bPayloadDir, make([]byte, 32))

	appA, err := New(Config{
		Persona:      alice,
		Ledger:       aliceLedger,
		LedgerPolicy: &pol,
		LedgerStore:  aStore,
		Sync:         sA,
		Payloads:     aPayloads,
		PreKeyPrivs: func() ([][32]byte, error) {
			return [][32]byte{alicePriv}, nil
		},
	})
	if err != nil {
		t.Fatalf("new app alice: %v", err)
	}
	appB, err := New(Config{
		Persona:      bob,
		Ledger:       bobLedger,
		LedgerPolicy: &pol,
		LedgerStore:  bStore,
		Sync:         sB,
		Payloads:     bPayloads,
		PreKeyPrivs: func() ([][32]byte, error) {
			return [][32]byte{bobPriv}, nil
		},
	})
	if err != nil {
		t.Fatalf("new app bob: %v", err)
	}

	toBob := Contact{
		ID:           "bob",
		Alias:        "Bob",
		SignPubB64:   bob.SignPubB64,
		Mailbox:      "mb-bob",
		PreKeyPubB64: bobPK.PubB64,
	}

	_, err = appA.SendMessage(context.Background(), r, toBob, "hi bob")
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	envMsgs, err := r.Pull(context.Background(), relay.PullRequest{Mailbox: relay.MailboxID("mb-bob"), Limit: 10})
	if err != nil {
		t.Fatalf("pull env: %v", err)
	}
	if len(envMsgs) != 1 {
		t.Fatalf("expected 1 env msg, got %d", len(envMsgs))
	}
	pt, _, err := ledgersync.DecodeFromRelay(string(envMsgs[0].Envelope), bobPriv)
	if err != nil {
		t.Fatalf("decrypt env: %v", err)
	}
	if err := appB.HandleRelayEnvelope(context.Background(), envMsgs[0], string(envMsgs[0].Envelope), pt); err != nil {
		t.Fatalf("handle env: %v", err)
	}

	_, err = appA.PublishAllEventsToRelay(context.Background(), r, relay.MailboxID("mb-ledger-bob"), bobPub, "chat", 300, 0)
	if err != nil {
		t.Fatalf("publish ledger: %v", err)
	}
	if _, _, err := appB.PullAndApplyFromRelay(context.Background(), r, relay.MailboxID("mb-ledger-bob"), bobPriv, 50, true); err != nil {
		t.Fatalf("pull ledger: %v", err)
	}

	chatID := string(chat.DirectChatID(chat.ContactIDFromSignPub(alice.SignPubB64), chat.ContactIDFromSignPub(bob.SignPubB64)))
	got, err := appB.ListMessages(chatID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].Text != "hi bob" {
		t.Fatalf("expected %q via relay-only sync, got %+v", "hi bob", got)
	}

	bPayloads2, _ := NewPayloadStore(bPayloadDir, make([]byte, 32))
	appB2, err := Open(Config{
		Persona:      bob,
		Ledger:       nil,
		LedgerPolicy: &pol,
		LedgerStore:  bStore,
		Sync:         nil,
		Payloads:     bPayloads2,
		PreKeyPrivs: func() ([][32]byte, error) {
			return [][32]byte{bobPriv}, nil
		},
	})
	if err != nil {
		t.Fatalf("open bob: %v", err)
	}
	got2, err := appB2.ListMessages(chatID)
	if err != nil {
		t.Fatalf("list after recovery: %v", err)
	}
	if len(got2) != 1 || got2[0].Text != "hi bob" {
		t.Fatalf("expected %q after recovery, got %+v", "hi bob", got2)
	}
}

func TestApp_AgentPolicy(t *testing.T) {
	persona, _ := identity.NewPersona()
	l := ledger.New()
	pol := ledger.DefaultPolicy()
	ps, _ := NewPayloadStore(t.TempDir(), make([]byte, 32))

	a, err := New(Config{
		Persona:      persona,
		Ledger:       l,
		LedgerPolicy: &pol,
		Payloads:     ps,
		AgentPolicy: AgentPolicy{
			Enabled:        true,
			AllowedActions: []string{"reply"},
		},
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, err := a.AgentAction(time.Now(), "agent-1", "delete_all", "chat", "x", ""); err == nil {
		t.Fatalf("expected restricted agent action to fail")
	}
	if _, err := a.AgentReply(time.Now(), "agent-1", "chat-1", "ref"); err != nil {
		t.Fatalf("expected reply to succeed: %v", err)
	}
}

func TestApp_ListMessages_WaitingForPayload(t *testing.T) {
	persona, _ := identity.NewPersona()
	l := ledger.New()
	pol := ledger.DefaultPolicy()
	ps, _ := NewPayloadStore(t.TempDir(), make([]byte, 32))
	a, err := New(Config{
		Persona:      persona,
		Ledger:       l,
		LedgerPolicy: &pol,
		Payloads:     ps,
		PreKeyPrivs:  func() ([][32]byte, error) { return nil, nil },
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	chatID := "c1"
	pl := ChatMessageEvent{
		ChatID:         chatID,
		PayloadRef:     "payload:sha256:does-not-exist",
		PayloadHashB64: "deadbeef",
		FromPubB64:     "other",
		ToPubB64:       persona.SignPubB64,
		ClientID:       "c",
	}
	raw, _ := json.Marshal(pl)
	_, inlineRef, _ := ledger.InlinePayloadRef(raw)
	pub, priv, _ := persona.SignKeypair()
	ev, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(persona.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Prev:       "",
		Parents:    nil,
		Kind:       KindChatMessage,
		Payload:    raw,
		PayloadRef: inlineRef,
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new signed: %v", err)
	}
	if _, err := l.Apply(ev, &pol, nil); err != nil {
		t.Fatalf("apply: %v", err)
	}
	msgs, err := a.ListMessages(chatID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}
	if msgs[0].State != "waiting_for_payload" {
		t.Fatalf("expected waiting_for_payload, got %q", msgs[0].State)
	}
	if msgs[0].PayloadAvailable {
		t.Fatalf("expected payload to be unavailable")
	}
}

func TestApp_ListMessages_BlockedOnHashMismatch(t *testing.T) {
	persona, _ := identity.NewPersona()
	l := ledger.New()
	pol := ledger.DefaultPolicy()
	ps, _ := NewPayloadStore(t.TempDir(), make([]byte, 32))
	a, err := New(Config{
		Persona:      persona,
		Ledger:       l,
		LedgerPolicy: &pol,
		Payloads:     ps,
		PreKeyPrivs:  func() ([][32]byte, error) { return nil, nil },
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	chatID := "c2"
	env := "not-a-real-envelope"
	ref := "payload:sha256:" + sha256B64Bytes([]byte(env))
	if err := ps.Put(ref, []byte(env)); err != nil {
		t.Fatalf("put: %v", err)
	}
	pl := ChatMessageEvent{
		ChatID:         chatID,
		PayloadRef:     ref,
		PayloadHashB64: "mismatch",
		FromPubB64:     "other",
		ToPubB64:       persona.SignPubB64,
		ClientID:       "c",
	}
	raw, _ := json.Marshal(pl)
	_, inlineRef, _ := ledger.InlinePayloadRef(raw)
	pub, priv, _ := persona.SignKeypair()
	ev, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(persona.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Prev:       "",
		Parents:    nil,
		Kind:       KindChatMessage,
		Payload:    raw,
		PayloadRef: inlineRef,
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new signed: %v", err)
	}
	if _, err := l.Apply(ev, &pol, nil); err != nil {
		t.Fatalf("apply: %v", err)
	}
	msgs, err := a.ListMessages(chatID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(msgs))
	}
	if msgs[0].State != "blocked" {
		t.Fatalf("expected blocked, got %q", msgs[0].State)
	}
	if !msgs[0].PayloadAvailable {
		t.Fatalf("expected payload to be available")
	}
	if msgs[0].PayloadVerified {
		t.Fatalf("expected payload to be unverified")
	}
}

func TestApp_MessageBeforePayload_ThenPayloadArrives(t *testing.T) {
	alice, _ := identity.NewPersona()
	bob, _ := identity.NewPersona()
	bobPK, _ := identity.NewPreKey(24 * time.Hour)
	bobPriv, _ := bobPK.DecodePrivate()
	bobPub, _ := bobPK.DecodePublic()

	l := ledger.New()
	pol := ledger.DefaultPolicy()
	ps, _ := NewPayloadStore(t.TempDir(), make([]byte, 32))
	a, err := New(Config{
		Persona:      bob,
		Ledger:       l,
		LedgerPolicy: &pol,
		Payloads:     ps,
		PreKeyPrivs: func() ([][32]byte, error) {
			return [][32]byte{bobPriv}, nil
		},
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	selfID := chat.ContactIDFromSignPub(bob.SignPubB64)
	fromID := chat.ContactIDFromSignPub(alice.SignPubB64)
	chatID := string(chat.DirectChatID(selfID, fromID))

	alicePub, alicePriv, _ := alice.SignKeypair()
	payload, err := chat.EncodeText(alice.SignPubB64, alicePriv, chatID, "hi later", time.Now())
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}
	env, _, err := messaging.EncryptToPreKey(payload, bobPub)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	envStr, err := env.Encode()
	if err != nil {
		t.Fatalf("encode env: %v", err)
	}
	hashB64 := sha256B64Bytes([]byte(envStr))
	ref := "payload:sha256:" + hashB64

	pl := ChatMessageEvent{
		ChatID:         chatID,
		PayloadRef:     ref,
		PayloadHashB64: hashB64,
		FromPubB64:     alice.SignPubB64,
		ToPubB64:       bob.SignPubB64,
		ClientID:       "c",
	}
	raw, _ := json.Marshal(pl)
	_, inlineRef, _ := ledger.InlinePayloadRef(raw)
	ev, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(alice.ID),
		AuthorPub:  alicePub,
		AuthorPriv: alicePriv,
		Seq:        1,
		Prev:       "",
		Parents:    nil,
		Kind:       KindChatMessage,
		Payload:    raw,
		PayloadRef: inlineRef,
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new signed: %v", err)
	}
	if _, err := l.Apply(ev, &pol, nil); err != nil {
		t.Fatalf("apply: %v", err)
	}

	msgs1, err := a.ListMessages(chatID)
	if err != nil {
		t.Fatalf("list1: %v", err)
	}
	if len(msgs1) != 1 || msgs1[0].State != "waiting_for_payload" {
		t.Fatalf("expected waiting_for_payload, got %+v", msgs1)
	}

	if err := a.HandleRelayEnvelope(context.Background(), relay.Message{}, envStr, payload); err != nil {
		t.Fatalf("handle envelope: %v", err)
	}
	msgs2, err := a.ListMessages(chatID)
	if err != nil {
		t.Fatalf("list2: %v", err)
	}
	if len(msgs2) != 1 || msgs2[0].Text != "hi later" || msgs2[0].State != "synced" {
		t.Fatalf("expected synced text, got %+v", msgs2)
	}

	st := a.Stats()
	if st.PayloadMissing == 0 {
		t.Fatalf("expected payload missing counter to increase")
	}
}

func TestApp_OversizeEnvelope_Dropped(t *testing.T) {
	persona, _ := identity.NewPersona()
	l := ledger.New()
	pol := ledger.DefaultPolicy()
	ps, _ := NewPayloadStore(t.TempDir(), make([]byte, 32))
	a, err := New(Config{
		Persona:          persona,
		Ledger:           l,
		LedgerPolicy:     &pol,
		Payloads:         ps,
		MaxEnvelopeBytes: 8,
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	oversize := strings.Repeat("x", 16)
	if err := a.HandleRelayEnvelope(context.Background(), relay.Message{}, oversize, []byte("x")); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	st := a.Stats()
	if st.OversizeEnvelopeDrop != 1 {
		t.Fatalf("expected oversize drop counter to be 1, got %d", st.OversizeEnvelopeDrop)
	}
}

func TestApp_PayloadPrune_RemovesUnreferenced(t *testing.T) {
	persona, _ := identity.NewPersona()
	l := ledger.New()
	pol := ledger.DefaultPolicy()
	ps, _ := NewPayloadStore(t.TempDir(), make([]byte, 32))
	a, err := New(Config{
		Persona:      persona,
		Ledger:       l,
		LedgerPolicy: &pol,
		Payloads:     ps,
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	keepRef := "payload:sha256:keep"
	dropRef := "payload:sha256:drop"
	if err := ps.Put(keepRef, []byte("k")); err != nil {
		t.Fatalf("put keep: %v", err)
	}
	if err := ps.Put(dropRef, []byte("d")); err != nil {
		t.Fatalf("put drop: %v", err)
	}

	pl := ChatMessageEvent{
		ChatID:     "c",
		PayloadRef: keepRef,
		FromPubB64: "other",
		ToPubB64:   persona.SignPubB64,
		ClientID:   "c",
	}
	raw, _ := json.Marshal(pl)
	_, inlineRef, _ := ledger.InlinePayloadRef(raw)
	pub, priv, _ := persona.SignKeypair()
	ev, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(persona.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       KindChatMessage,
		Payload:    raw,
		PayloadRef: inlineRef,
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new signed: %v", err)
	}
	if _, err := l.Apply(ev, &pol, nil); err != nil {
		t.Fatalf("apply: %v", err)
	}

	rep, err := a.Prune(time.Now(), RetentionPolicy{
		KeepUnreferencedFor: 0,
		MaxPayloadFiles:     0,
		MaxPayloadBytes:     0,
		MaxScanEvents:       1000,
	})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if rep.Payloads.Deleted == 0 {
		t.Fatalf("expected some payloads deleted, got %+v", rep.Payloads)
	}
	if !ps.Has(keepRef) {
		t.Fatalf("expected keepRef to remain")
	}
	if ps.Has(dropRef) {
		t.Fatalf("expected dropRef to be pruned")
	}
	st := a.Stats()
	if st.PayloadsPruned == 0 {
		t.Fatalf("expected payloads pruned counter to increase")
	}
}

func TestApp_AgentPolicy_Categories(t *testing.T) {
	persona, _ := identity.NewPersona()
	l := ledger.New()
	pol := ledger.DefaultPolicy()
	ps, _ := NewPayloadStore(t.TempDir(), make([]byte, 32))

	a, err := New(Config{
		Persona:      persona,
		Ledger:       l,
		LedgerPolicy: &pol,
		Payloads:     ps,
		AgentPolicy: AgentPolicy{
			Enabled:         true,
			AllowWriteChats: true,
		},
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, err := a.AgentReply(time.Now(), "agent-1", "chat-1", "ref"); err != nil {
		t.Fatalf("expected reply to succeed: %v", err)
	}

	b, err := New(Config{
		Persona:      persona,
		Ledger:       ledger.New(),
		LedgerPolicy: &pol,
		Payloads:     ps,
		AgentPolicy: AgentPolicy{
			Enabled:        true,
			AllowReadChats: true,
		},
	})
	if err != nil {
		t.Fatalf("new b: %v", err)
	}
	if _, err := b.AgentReply(time.Now(), "agent-1", "chat-1", "ref"); err == nil {
		t.Fatalf("expected reply to be blocked without write permission")
	}
}
