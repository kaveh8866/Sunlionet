package ledgersync

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/identity"
	"github.com/kaveh/sunlionet-agent/pkg/ledger"
	"github.com/kaveh/sunlionet-agent/pkg/mesh"
	"github.com/kaveh/sunlionet-agent/pkg/messaging"
	"github.com/kaveh/sunlionet-agent/pkg/relay"
	"github.com/kaveh/sunlionet-agent/pkg/runtimecfg"
)

type pairMesh struct {
	in  chan []byte
	out chan []byte
}

func (m *pairMesh) Broadcast(data []byte) error {
	cp := append([]byte(nil), data...)
	m.out <- cp
	return nil
}

func (m *pairMesh) Receive(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case b := <-m.in:
		return b, nil
	}
}

func (m *pairMesh) RuntimeMode() runtimecfg.RuntimeMode {
	return runtimecfg.ModeSim
}

func newMeshPair(buffer int) (*pairMesh, *pairMesh) {
	if buffer <= 0 {
		buffer = 32
	}
	a2b := make(chan []byte, buffer)
	b2a := make(chan []byte, buffer)
	return &pairMesh{in: b2a, out: a2b}, &pairMesh{in: a2b, out: b2a}
}

func fixedRand(b []byte) error {
	for i := range b {
		b[i] = byte(i + 1)
	}
	return nil
}

func TestMeshSync_HeadsInventoryWantEvents(t *testing.T) {
	mA, mB := newMeshPair(16)
	cA, err := mesh.NewCrypto()
	if err != nil {
		t.Fatalf("NewCrypto A: %v", err)
	}
	cB, err := mesh.NewCrypto()
	if err != nil {
		t.Fatalf("NewCrypto B: %v", err)
	}

	persona, _ := identity.NewPersona()
	pub, priv, _ := persona.SignKeypair()

	missing, _ := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(persona.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"missing"}`),
		CreatedAt:  time.Now(),
	})
	child, _ := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(persona.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        2,
		Prev:       missing.ID,
		Parents:    []string{missing.ID},
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"child"}`),
		CreatedAt:  time.Now(),
	})

	lA := ledger.New()
	if _, err := lA.Apply(child, nil, nil); err != nil {
		t.Fatalf("apply child on A: %v", err)
	}
	lB := ledger.New()
	if _, err := lB.Apply(missing, nil, nil); err != nil {
		t.Fatalf("apply missing on B: %v", err)
	}

	sA, err := New(mA, cA, lA, nil, nil, Options{MaxHave: 0, MaxWant: 32, MaxEvents: 32})
	if err != nil {
		t.Fatalf("New A: %v", err)
	}
	sB, err := New(mB, cB, lB, nil, nil, Options{MaxHave: 0, MaxWant: 32, MaxEvents: 32})
	if err != nil {
		t.Fatalf("New B: %v", err)
	}
	sA.rand = fixedRand

	peerB := Peer{ID: "b", MeshPub: cB.PublicKey()}
	if _, err := sA.SendHeads(context.Background(), peerB, ""); err != nil {
		t.Fatalf("SendHeads: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := sB.ReceiveOnce(ctx); err != nil {
		t.Fatalf("B ReceiveOnce: %v", err)
	}
	if err := sA.ReceiveOnce(ctx); err != nil {
		t.Fatalf("A ReceiveOnce: %v", err)
	}
	if err := sB.ReceiveOnce(ctx); err != nil {
		t.Fatalf("B ReceiveOnce (want): %v", err)
	}
	if err := sA.ReceiveOnce(ctx); err != nil {
		t.Fatalf("A ReceiveOnce (events): %v", err)
	}

	if !lA.Have(missing.ID) {
		t.Fatalf("expected missing event applied")
	}
	if _, ok := lA.MissingRefs()[missing.ID]; ok {
		t.Fatalf("expected missing ref cleared")
	}
}

type noopMesh struct{}

func (m *noopMesh) Broadcast(data []byte) error { return nil }
func (m *noopMesh) Receive(ctx context.Context) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (m *noopMesh) RuntimeMode() runtimecfg.RuntimeMode { return runtimecfg.ModeSim }

func TestRelaySync_PublishAndPullEvents(t *testing.T) {
	r := relay.NewMemoryRelay()

	persona, _ := identity.NewPersona()
	pub, priv, _ := persona.SignKeypair()

	missing, _ := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(persona.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"missing"}`),
		CreatedAt:  time.Now(),
	})
	child, _ := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(persona.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        2,
		Prev:       missing.ID,
		Parents:    []string{missing.ID},
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"child"}`),
		CreatedAt:  time.Now(),
	})

	lA := ledger.New()
	if _, err := lA.Apply(child, nil, nil); err != nil {
		t.Fatalf("apply child on A: %v", err)
	}
	lB := ledger.New()
	if _, err := lB.Apply(missing, nil, nil); err != nil {
		t.Fatalf("apply missing on B: %v", err)
	}

	cA, _ := mesh.NewCrypto()
	cB, _ := mesh.NewCrypto()
	sA, err := New(&noopMesh{}, cA, lA, nil, nil, Options{MaxHave: 0, MaxWant: 64, MaxEvents: 64})
	if err != nil {
		t.Fatalf("New A: %v", err)
	}
	sB, err := New(&noopMesh{}, cB, lB, nil, nil, Options{MaxHave: 0, MaxWant: 64, MaxEvents: 64})
	if err != nil {
		t.Fatalf("New B: %v", err)
	}
	sB.rand = fixedRand

	recipient, err := messaging.NewX25519Keypair()
	if err != nil {
		t.Fatalf("NewX25519Keypair: %v", err)
	}

	mailbox := relay.MailboxID("mbA")
	sent, err := sB.PublishEventsToRelay(context.Background(), r, mailbox, recipient.Public, "", []string{missing.ID}, 60, 0)
	if err != nil {
		t.Fatalf("PublishEventsToRelay: %v", err)
	}
	if sent != 1 {
		t.Fatalf("expected 1 sent event, got %d", sent)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rep, pulled, err := sA.PullAndApplyFromRelay(ctx, r, mailbox, recipient.Private, 10, true)
	if err != nil {
		t.Fatalf("PullAndApplyFromRelay: %v", err)
	}
	if pulled == 0 {
		t.Fatalf("expected to pull at least 1 message")
	}
	if rep.Applied != 1 {
		t.Fatalf("expected 1 applied, got %#v", rep)
	}
	if !lA.Have(missing.ID) {
		t.Fatalf("expected missing event applied")
	}
	if _, ok := lA.MissingRefs()[missing.ID]; ok {
		t.Fatalf("expected missing ref cleared")
	}

	after, err := r.Pull(ctx, relay.PullRequest{Mailbox: mailbox, Limit: 10})
	if err != nil {
		t.Fatalf("pull after ack: %v", err)
	}
	if len(after) != 0 {
		t.Fatalf("expected mailbox empty after ack")
	}
}

func TestService_Stats_DuplicateAndDeferredAndRejected(t *testing.T) {
	persona, _ := identity.NewPersona()
	pub, priv, _ := persona.SignKeypair()

	ev, _ := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(persona.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"x"}`),
		CreatedAt:  time.Now(),
	})
	msg := ledger.EventsMessage{SchemaVersion: ledger.SyncSchemaV1, Events: []ledger.Event{ev}}
	pt, err := encodeWire("events", "s", "", msg)
	if err != nil {
		t.Fatalf("encode wire: %v", err)
	}

	c, _ := mesh.NewCrypto()
	l := ledger.New()
	s, err := New(&noopMesh{}, c, l, nil, nil, Options{})
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	handled, rep1, err := s.TryApplyWirePlaintext("peer1", pt)
	if err != nil || !handled || rep1.Applied != 1 {
		t.Fatalf("expected applied=1, handled=true, err=nil; got handled=%v rep=%+v err=%v", handled, rep1, err)
	}
	handled, _, _ = s.TryApplyWirePlaintext("peer1", pt)
	if !handled {
		t.Fatalf("expected handled=true on duplicate plaintext")
	}
	if st := s.Stats(); st.Duplicate != 1 {
		t.Fatalf("expected duplicate=1, got %+v", st)
	}

	ptBig, err := encodeWire("events", "s2", "", map[string]string{"x": strings.Repeat("a", 110*1024)})
	if err != nil {
		t.Fatalf("encode big wire: %v", err)
	}
	_, _, _ = s.TryApplyWirePlaintext("peer2", ptBig)
	if st := s.Stats(); st.Deferred == 0 {
		t.Fatalf("expected deferred>0, got %+v", st)
	}

	c2, _ := mesh.NewCrypto()
	l2 := ledger.New()
	sp := DefaultSecurityPolicy()
	sp.InitialScore = 10
	sp.MinScore = 11
	sp.MediumScore = 12
	sp.HighScore = 13
	s2, err := New(&noopMesh{}, c2, l2, nil, nil, Options{SecurityPolicy: sp})
	if err != nil {
		t.Fatalf("new2: %v", err)
	}
	_, _, _ = s2.TryApplyWirePlaintext("peer3", pt)
	st2 := s2.Stats()
	if st2.Rejected == 0 || st2.Quarantined == 0 {
		t.Fatalf("expected rejected>0 and quarantined>0, got %+v", st2)
	}
}
