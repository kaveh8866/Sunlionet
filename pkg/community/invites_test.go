package community

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

func TestInviteJoinApproveRoundtrip(t *testing.T) {
	issuerPubB64, issuerPriv := newSignKeypairB64(t)
	appPubB64, appPriv := newSignKeypairB64(t)

	inv, err := NewInvite(CommunityID("c1"), issuerPubB64, issuerPriv, 10*time.Minute, 3)
	if err != nil {
		t.Fatalf("NewInvite: %v", err)
	}
	encInv, err := inv.Encode()
	if err != nil {
		t.Fatalf("Invite Encode: %v", err)
	}
	inv2, err := DecodeInvite(encInv)
	if err != nil {
		t.Fatalf("DecodeInvite: %v", err)
	}
	if inv2.ID != inv.ID {
		t.Fatalf("invite id mismatch")
	}

	jr, err := NewJoinRequest(inv2, appPubB64, appPriv)
	if err != nil {
		t.Fatalf("NewJoinRequest: %v", err)
	}
	encJR, err := jr.Encode()
	if err != nil {
		t.Fatalf("JoinRequest Encode: %v", err)
	}
	jr2, err := DecodeJoinRequest(encJR)
	if err != nil {
		t.Fatalf("DecodeJoinRequest: %v", err)
	}

	ja, err := ApproveJoin(inv2, jr2, issuerPubB64, issuerPriv, RoleMember)
	if err != nil {
		t.Fatalf("ApproveJoin: %v", err)
	}
	encJA, err := ja.Encode()
	if err != nil {
		t.Fatalf("JoinApproval Encode: %v", err)
	}
	ja2, err := DecodeJoinApproval(encJA)
	if err != nil {
		t.Fatalf("DecodeJoinApproval: %v", err)
	}
	if ja2.GrantedRole != RoleMember {
		t.Fatalf("role mismatch")
	}
}

func newSignKeypairB64(t *testing.T) (string, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(pub), priv
}
