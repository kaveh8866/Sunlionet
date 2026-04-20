package community

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const SchemaV1 = 1

type CommunityID string

type Role string

const (
	RoleOwner     Role = "owner"
	RoleModerator Role = "moderator"
	RoleMember    Role = "member"
)

type EventKind string

const (
	EventPost      EventKind = "post"
	EventReply     EventKind = "reply"
	EventReaction  EventKind = "reaction"
	EventJoin      EventKind = "join"
	EventLeave     EventKind = "leave"
	EventRoleGrant EventKind = "role_grant"
	EventModerate  EventKind = "moderate"
)

type Event struct {
	ID        string      `json:"id"`
	Community CommunityID `json:"community"`
	Kind      EventKind   `json:"kind"`
	CreatedAt int64       `json:"created_at"`
	Author    string      `json:"author,omitempty"`
	CipherB64 string      `json:"cipher_b64url"`
}

type State struct {
	SchemaVersion int            `json:"schema_version"`
	UpdatedAt     int64          `json:"updated_at"`
	Communities   []Community    `json:"communities"`
	Invites       []StoredInvite `json:"invites,omitempty"`
	Members       []Member       `json:"members,omitempty"`
}

type Community struct {
	ID        CommunityID `json:"id"`
	CreatedAt int64       `json:"created_at"`
	Role      Role        `json:"role"`
}

type StoredInvite struct {
	Invite Invite `json:"invite"`
	Uses   int    `json:"uses"`
}

func NewState() *State {
	return &State{
		SchemaVersion: SchemaV1,
		UpdatedAt:     time.Now().Unix(),
		Communities:   []Community{},
		Invites:       []StoredInvite{},
		Members:       []Member{},
	}
}

func (s *State) Validate() error {
	if s == nil {
		return errors.New("community: state is nil")
	}
	if s.SchemaVersion != SchemaV1 {
		return fmt.Errorf("community: unsupported schema_version: %d", s.SchemaVersion)
	}
	seen := make(map[CommunityID]struct{}, len(s.Communities))
	for i := range s.Communities {
		c := s.Communities[i]
		if c.ID == "" {
			return errors.New("community: community id required")
		}
		if _, ok := seen[c.ID]; ok {
			return fmt.Errorf("community: duplicate community id: %q", c.ID)
		}
		seen[c.ID] = struct{}{}
		if c.CreatedAt <= 0 {
			return fmt.Errorf("community: created_at required for %q", c.ID)
		}
		switch c.Role {
		case RoleOwner, RoleModerator, RoleMember:
		default:
			return fmt.Errorf("community: invalid role for %q: %q", c.ID, c.Role)
		}
	}
	seenInv := make(map[InviteID]struct{}, len(s.Invites))
	for i := range s.Invites {
		if s.Invites[i].Uses < 0 {
			return fmt.Errorf("community: invalid invite uses for index %d", i)
		}
		inv := s.Invites[i].Invite
		if inv.Schema != SchemaV1 || inv.ID == "" || inv.Community == "" || strings.TrimSpace(inv.IssuerPubB64) == "" {
			return fmt.Errorf("community: invalid invite at index %d", i)
		}
		if err := inv.VerifySignature(); err != nil {
			return fmt.Errorf("community: invalid invite signature at index %d: %w", i, err)
		}
		if _, ok := seenInv[s.Invites[i].Invite.ID]; ok {
			return fmt.Errorf("community: duplicate invite id: %q", s.Invites[i].Invite.ID)
		}
		seenInv[s.Invites[i].Invite.ID] = struct{}{}
	}
	for i := range s.Members {
		m := s.Members[i]
		if m.Community == "" || strings.TrimSpace(m.MemberPubB64) == "" {
			return fmt.Errorf("community: invalid member at index %d", i)
		}
		switch m.Role {
		case RoleOwner, RoleModerator, RoleMember:
		default:
			return fmt.Errorf("community: invalid member role at index %d: %q", i, m.Role)
		}
		switch m.Status {
		case MemberActive, MemberRevoked, MemberBanned:
		default:
			return fmt.Errorf("community: invalid member status at index %d: %q", i, m.Status)
		}
	}
	return nil
}
