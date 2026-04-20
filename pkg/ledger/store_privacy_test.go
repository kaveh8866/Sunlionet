package ledger

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaveh/sunlionet-agent/pkg/identity"
)

func TestStore_Save_DoesNotLeakPlaintext(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "ledger.enc")

	store, err := NewStore(dbPath, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	p, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new persona: %v", err)
	}
	pub, priv, err := p.SignKeypair()
	if err != nil {
		t.Fatalf("sign keypair: %v", err)
	}

	secretRef := "chat:thread/secret.example.com"
	ev, err := NewSignedEvent(SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"hello"}`),
		PayloadRef: secretRef,
	})
	if err != nil {
		t.Fatalf("new signed event: %v", err)
	}

	l := New()
	if err := l.Add(ev); err != nil {
		t.Fatalf("add event: %v", err)
	}
	if err := store.Save(l.Snapshot()); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for _, needle := range [][]byte{
		[]byte(secretRef),
		[]byte("secret.example.com"),
		[]byte("hello"),
	} {
		if bytes.Contains(raw, needle) {
			t.Fatalf("encrypted store leaked plaintext token: %q", needle)
		}
	}
}

func TestStore_Load_RoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "ledger.enc")
	store, err := NewStore(dbPath, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	p, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new persona: %v", err)
	}
	pub, priv, err := p.SignKeypair()
	if err != nil {
		t.Fatalf("sign keypair: %v", err)
	}

	ev, err := NewSignedEvent(SignedEventInput{
		Author:     string(p.ID),
		AuthorPub:  pub,
		AuthorPriv: priv,
		Seq:        1,
		Kind:       "chat.msg",
		Payload:    json.RawMessage(`{"t":"hello"}`),
	})
	if err != nil {
		t.Fatalf("new signed event: %v", err)
	}

	l := New()
	if err := l.Add(ev); err != nil {
		t.Fatalf("add event: %v", err)
	}
	if err := store.Save(l.Snapshot()); err != nil {
		t.Fatalf("save: %v", err)
	}

	snap, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	l2, err := NewFromSnapshot(snap)
	if err != nil {
		t.Fatalf("new from snapshot: %v", err)
	}
	heads := l2.Heads()
	if len(heads) != 1 || heads[0] != ev.ID {
		t.Fatalf("unexpected heads: %#v", heads)
	}
}
