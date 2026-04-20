package community

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/identity"
)

func TestServiceInviteApproveApplyJoin(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	mk := make([]byte, 32)
	ownerStore, err := NewStore(filepath.Join(tmp, "owner-community.enc"), mk)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ownerSvc, err := NewService(ownerStore)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	memberStore, err := NewStore(filepath.Join(tmp, "member-community.enc"), mk)
	if err != nil {
		t.Fatalf("new member store: %v", err)
	}
	memberSvc, err := NewService(memberStore)
	if err != nil {
		t.Fatalf("new member service: %v", err)
	}
	owner, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new owner persona: %v", err)
	}
	member, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("new member persona: %v", err)
	}

	communityID, err := ownerSvc.CreateCommunity("community-test", RoleOwner)
	if err != nil {
		t.Fatalf("create community: %v", err)
	}
	if !ownerSvc.CanInvite(communityID) {
		t.Fatalf("owner should be able to invite")
	}

	inviteToken, err := ownerSvc.CreateInvite(owner, communityID, 5*time.Minute, 1)
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	joinReqToken, err := memberSvc.CreateJoinRequest(member, inviteToken)
	if err != nil {
		t.Fatalf("create join request: %v", err)
	}
	approvalToken, err := ownerSvc.ApproveJoin(owner, inviteToken, joinReqToken, RoleMember)
	if err != nil {
		t.Fatalf("approve join: %v", err)
	}
	gotMember, err := memberSvc.ApplyJoin(member, inviteToken, joinReqToken, approvalToken)
	if err != nil {
		t.Fatalf("apply join: %v", err)
	}
	if gotMember.Role != RoleMember {
		t.Fatalf("expected role member, got %q", gotMember.Role)
	}

	role, ok := memberSvc.RoleOf(communityID)
	if !ok {
		t.Fatalf("expected role for joined community")
	}
	if role != RoleMember {
		t.Fatalf("expected joined role member, got %q", role)
	}
	if !memberSvc.CanPost(communityID) {
		t.Fatalf("member should be able to post")
	}
}
