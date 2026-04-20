package community

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/identity"
)

type Service struct {
	store *Store
	mu    sync.RWMutex
	state *State
}

func NewService(store *Store) (*Service, error) {
	if store == nil {
		return nil, errors.New("community: store is nil")
	}
	st, err := store.Load()
	if err != nil {
		return nil, err
	}
	if st == nil {
		st = NewState()
	}
	if err := st.Validate(); err != nil {
		return nil, err
	}
	return &Service{
		store: store,
		state: st,
	}, nil
}

func (s *Service) Snapshot() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := State{
		SchemaVersion: s.state.SchemaVersion,
		UpdatedAt:     s.state.UpdatedAt,
		Communities:   append([]Community(nil), s.state.Communities...),
		Invites:       append([]StoredInvite(nil), s.state.Invites...),
		Members:       append([]Member(nil), s.state.Members...),
	}
	return out
}

func (s *Service) CreateCommunity(id CommunityID, role Role) (CommunityID, error) {
	if s == nil {
		return "", errors.New("community: service is nil")
	}
	if role == "" {
		role = RoleOwner
	}
	switch role {
	case RoleOwner, RoleModerator, RoleMember:
	default:
		return "", fmt.Errorf("community: invalid role: %q", role)
	}
	if strings.TrimSpace(string(id)) == "" {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		id = CommunityID(base64.RawURLEncoding.EncodeToString(b[:]))
	}
	now := time.Now().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Communities {
		if s.state.Communities[i].ID == id {
			return id, nil
		}
	}
	s.state.Communities = append(s.state.Communities, Community{
		ID:        id,
		CreatedAt: now,
		Role:      role,
	})
	s.state.UpdatedAt = now
	return id, s.store.Save(s.state)
}

func (s *Service) RoleOf(communityID CommunityID) (Role, bool) {
	if s == nil {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.state.Communities {
		if s.state.Communities[i].ID == communityID {
			return s.state.Communities[i].Role, true
		}
	}
	return "", false
}

func (s *Service) CanInvite(communityID CommunityID) bool {
	role, ok := s.RoleOf(communityID)
	if !ok {
		return false
	}
	return role == RoleOwner || role == RoleModerator
}

func (s *Service) CanCreateRoom(communityID CommunityID) bool {
	role, ok := s.RoleOf(communityID)
	if !ok {
		return false
	}
	return role == RoleOwner || role == RoleModerator
}

func (s *Service) CanPost(communityID CommunityID) bool {
	role, ok := s.RoleOf(communityID)
	if !ok {
		return false
	}
	return role == RoleOwner || role == RoleModerator || role == RoleMember
}

func (s *Service) CreateInvite(persona *identity.Persona, communityID CommunityID, ttl time.Duration, maxUses int) (string, error) {
	if s == nil {
		return "", errors.New("community: service is nil")
	}
	if persona == nil {
		return "", errors.New("community: persona is nil")
	}
	if !s.CanInvite(communityID) {
		return "", errors.New("community: invite permission denied")
	}
	_, priv, err := persona.SignKeypair()
	if err != nil {
		return "", err
	}
	inv, err := NewInvite(communityID, persona.SignPubB64, priv, ttl, maxUses)
	if err != nil {
		return "", err
	}
	token, err := inv.Encode()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Invites = upsertStoredInviteLocked(s.state.Invites, *inv)
	s.state.UpdatedAt = time.Now().Unix()
	return token, s.store.Save(s.state)
}

func (s *Service) CreateJoinRequest(persona *identity.Persona, inviteToken string) (string, error) {
	if persona == nil {
		return "", errors.New("community: persona is nil")
	}
	inv, err := DecodeInvite(strings.TrimSpace(inviteToken))
	if err != nil {
		return "", err
	}
	_, priv, err := persona.SignKeypair()
	if err != nil {
		return "", err
	}
	req, err := NewJoinRequest(inv, persona.SignPubB64, priv)
	if err != nil {
		return "", err
	}
	return req.Encode()
}

func (s *Service) ApproveJoin(persona *identity.Persona, inviteToken string, joinRequestToken string, role Role) (string, error) {
	if s == nil {
		return "", errors.New("community: service is nil")
	}
	if persona == nil {
		return "", errors.New("community: persona is nil")
	}
	inv, err := DecodeInvite(strings.TrimSpace(inviteToken))
	if err != nil {
		return "", err
	}
	if !s.CanInvite(inv.Community) {
		return "", errors.New("community: approve permission denied")
	}
	if strings.TrimSpace(inv.IssuerPubB64) != strings.TrimSpace(persona.SignPubB64) {
		return "", errors.New("community: invite issuer mismatch")
	}
	req, err := DecodeJoinRequest(strings.TrimSpace(joinRequestToken))
	if err != nil {
		return "", err
	}
	_, priv, err := persona.SignKeypair()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if stInv, ok := findStoredInviteLocked(s.state.Invites, inv.ID); ok {
		if stInv.Invite.MaxUses > 0 && stInv.Uses >= stInv.Invite.MaxUses {
			return "", errors.New("community: invite max uses exceeded")
		}
	}
	approval, err := ApproveJoin(inv, req, persona.SignPubB64, priv, role)
	if err != nil {
		return "", err
	}
	token, err := approval.Encode()
	if err != nil {
		return "", err
	}
	for i := range s.state.Invites {
		if s.state.Invites[i].Invite.ID == inv.ID {
			s.state.Invites[i].Uses++
			break
		}
	}
	s.state.UpdatedAt = time.Now().Unix()
	return token, s.store.Save(s.state)
}

func (s *Service) ApplyJoin(persona *identity.Persona, inviteToken string, joinRequestToken string, approvalToken string) (Member, error) {
	if s == nil {
		return Member{}, errors.New("community: service is nil")
	}
	if persona == nil {
		return Member{}, errors.New("community: persona is nil")
	}
	inv, err := DecodeInvite(strings.TrimSpace(inviteToken))
	if err != nil {
		return Member{}, err
	}
	req, err := DecodeJoinRequest(strings.TrimSpace(joinRequestToken))
	if err != nil {
		return Member{}, err
	}
	approval, err := DecodeJoinApproval(strings.TrimSpace(approvalToken))
	if err != nil {
		return Member{}, err
	}
	if inv.ID != req.InviteID || inv.Community != req.Community {
		return Member{}, errors.New("community: invite/request mismatch")
	}
	if approval.InviteID != inv.ID || approval.Community != inv.Community {
		return Member{}, errors.New("community: approval mismatch")
	}
	if strings.TrimSpace(approval.IssuerPubB64) != strings.TrimSpace(inv.IssuerPubB64) {
		return Member{}, errors.New("community: issuer mismatch")
	}
	if strings.TrimSpace(req.ApplicantPubB64) != strings.TrimSpace(persona.SignPubB64) {
		return Member{}, errors.New("community: applicant mismatch")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.state.Invites {
		if s.state.Invites[i].Invite.ID != inv.ID {
			continue
		}
		if s.state.Invites[i].Invite.MaxUses > 0 && s.state.Invites[i].Uses >= s.state.Invites[i].Invite.MaxUses {
			return Member{}, errors.New("community: invite max uses exceeded")
		}
		s.state.Invites[i].Uses++
	}
	now := time.Now().Unix()
	member := Member{
		Community:        inv.Community,
		MemberPubB64:     req.ApplicantPubB64,
		Role:             approval.GrantedRole,
		Status:           MemberActive,
		JoinedAt:         now,
		ApprovedByPubB64: approval.IssuerPubB64,
		InviteID:         inv.ID,
	}
	s.state.Members = upsertMemberLocked(s.state.Members, member)
	updated := false
	for i := range s.state.Communities {
		if s.state.Communities[i].ID == inv.Community {
			s.state.Communities[i].Role = approval.GrantedRole
			updated = true
			break
		}
	}
	if !updated {
		s.state.Communities = append(s.state.Communities, Community{
			ID:        inv.Community,
			CreatedAt: now,
			Role:      approval.GrantedRole,
		})
	}
	s.state.UpdatedAt = now
	return member, s.store.Save(s.state)
}

func upsertStoredInviteLocked(invites []StoredInvite, inv Invite) []StoredInvite {
	for i := range invites {
		if invites[i].Invite.ID == inv.ID {
			invites[i].Invite = inv
			return invites
		}
	}
	return append(invites, StoredInvite{Invite: inv, Uses: 0})
}

func findStoredInviteLocked(invites []StoredInvite, inviteID InviteID) (StoredInvite, bool) {
	for i := range invites {
		if invites[i].Invite.ID == inviteID {
			return invites[i], true
		}
	}
	return StoredInvite{}, false
}

func upsertMemberLocked(members []Member, m Member) []Member {
	for i := range members {
		if members[i].Community == m.Community && members[i].MemberPubB64 == m.MemberPubB64 {
			members[i] = m
			return members
		}
	}
	return append(members, m)
}
