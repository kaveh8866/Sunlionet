package community

import (
	"errors"
	"fmt"
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
	SchemaVersion int         `json:"schema_version"`
	UpdatedAt     int64       `json:"updated_at"`
	Communities   []Community `json:"communities"`
}

type Community struct {
	ID        CommunityID `json:"id"`
	CreatedAt int64       `json:"created_at"`
	Role      Role        `json:"role"`
}

func NewState() *State {
	return &State{
		SchemaVersion: SchemaV1,
		UpdatedAt:     time.Now().Unix(),
		Communities:   []Community{},
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
	return nil
}
