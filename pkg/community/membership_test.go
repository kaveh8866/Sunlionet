package community

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

func TestRegistryMaxUsesAndRevocation(t *testing.T) {
	issuerPubB64, issuerPriv := newSignKeypairB64(t)

	reg := NewRegistry()
	inv, err := NewInvite(CommunityID("c1"), issuerPubB64, issuerPriv, 10*time.Minute, 1)
	if err != nil {
		t.Fatalf("NewInvite: %v", err)
	}
	if err := reg.AddInvite(inv); err != nil {
		t.Fatalf("AddInvite: %v", err)
	}

	app1PubB64, app1Priv := newSignKeypairB64(t)
	jr1, err := NewJoinRequest(inv, app1PubB64, app1Priv)
	if err != nil {
		t.Fatalf("NewJoinRequest: %v", err)
	}
	ja1, err := ApproveJoin(inv, jr1, issuerPubB64, issuerPriv, RoleMember)
	if err != nil {
		t.Fatalf("ApproveJoin: %v", err)
	}
	m, err := reg.ApplyJoin(inv, jr1, ja1, JoinRateLimit{MaxPerHour: 10})
	if err != nil {
		t.Fatalf("ApplyJoin: %v", err)
	}
	if m.Status != MemberActive {
		t.Fatalf("expected active member")
	}

	app2PubB64, app2Priv := newSignKeypairB64(t)
	jr2, err := NewJoinRequest(inv, app2PubB64, app2Priv)
	if err != nil {
		t.Fatalf("NewJoinRequest 2: %v", err)
	}
	ja2, err := ApproveJoin(inv, jr2, issuerPubB64, issuerPriv, RoleMember)
	if err != nil {
		t.Fatalf("ApproveJoin 2: %v", err)
	}
	if _, err := reg.ApplyJoin(inv, jr2, ja2, JoinRateLimit{MaxPerHour: 10}); err == nil {
		t.Fatalf("expected max uses exceeded error")
	}

	rev, err := NewRevocation(inv.Community, BanMember, app1PubB64, issuerPubB64, issuerPriv)
	if err != nil {
		t.Fatalf("NewRevocation: %v", err)
	}
	_, ok, err := reg.ApplyRevocation(rev)
	if err != nil {
		t.Fatalf("ApplyRevocation: %v", err)
	}
	if !ok {
		t.Fatalf("expected member updated")
	}
}

func TestRegistryJoinRateLimit(t *testing.T) {
	issuerPubB64, issuerPriv := newSignKeypairB64(t)
	reg := NewRegistry()
	inv, err := NewInvite(CommunityID("c1"), issuerPubB64, issuerPriv, 10*time.Minute, 0)
	if err != nil {
		t.Fatalf("NewInvite: %v", err)
	}
	if err := reg.AddInvite(inv); err != nil {
		t.Fatalf("AddInvite: %v", err)
	}

	appPubB64, appPriv := newSignKeypairB64(t)
	for i := 0; i < 3; i++ {
		jr, err := NewJoinRequest(inv, appPubB64, appPriv)
		if err != nil {
			t.Fatalf("NewJoinRequest: %v", err)
		}
		ja, err := ApproveJoin(inv, jr, issuerPubB64, issuerPriv, RoleMember)
		if err != nil {
			t.Fatalf("ApproveJoin: %v", err)
		}
		_, err = reg.ApplyJoin(inv, jr, ja, JoinRateLimit{MaxPerHour: 2})
		if i < 2 && err != nil {
			t.Fatalf("ApplyJoin %d: %v", i, err)
		}
		if i == 2 && err == nil {
			t.Fatalf("expected rate limit error")
		}
	}
}

func TestRevocationEncodeDecode(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pubB64 := base64.RawURLEncoding.EncodeToString(pub)
	subPubB64, _, err := newMemberKeypairB64()
	if err != nil {
		t.Fatalf("newMemberKeypairB64: %v", err)
	}
	rev, err := NewRevocation(CommunityID("c1"), RevokeMember, subPubB64, pubB64, priv)
	if err != nil {
		t.Fatalf("NewRevocation: %v", err)
	}
	enc, err := rev.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	dec, err := DecodeRevocation(enc)
	if err != nil {
		t.Fatalf("DecodeRevocation: %v", err)
	}
	if dec.Kind != RevokeMember {
		t.Fatalf("kind mismatch")
	}
}
