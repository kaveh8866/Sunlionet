package chat

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	SchemaV1 = 1

	MaxContacts = 2000
	MaxChats    = 5000
	MaxMessages = 200000
)

type ContactID string
type ChatID string
type MessageID string

type Contact struct {
	ID            ContactID `json:"id"`
	CreatedAt     int64     `json:"created_at"`
	Alias         string    `json:"alias"`
	SignPubB64    string    `json:"sign_pub_b64url"`
	Mailbox       string    `json:"mailbox,omitempty"`
	PreKeyPubB64  string    `json:"prekey_pub_b64url,omitempty"`
	RelayHints    []string  `json:"relay_hints,omitempty"`
	Trust         string    `json:"trust,omitempty"`
	LastUpdatedAt int64     `json:"last_updated_at,omitempty"`
}

type Chat struct {
	ID           ChatID            `json:"id"`
	Kind         string            `json:"kind"`
	CreatedAt    int64             `json:"created_at"`
	Title        string            `json:"title,omitempty"`
	Participants []ContactID       `json:"participants"`
	Roles        map[string]string `json:"roles,omitempty"`
	LastMsgAt    int64             `json:"last_msg_at,omitempty"`
	Unread       int               `json:"unread,omitempty"`
}

type Message struct {
	ID               MessageID `json:"id"`
	ClientID         string    `json:"client_id,omitempty"`
	ChatID           ChatID    `json:"chat_id"`
	CreatedAt        int64     `json:"created_at"`
	SortKey          int64     `json:"sort_key,omitempty"`
	DeviceID         string    `json:"device_id,omitempty"`
	SenderSignPubB64 string    `json:"sender_sign_pub_b64url,omitempty"`
	Text             string    `json:"text,omitempty"`
	Envelope         string    `json:"envelope,omitempty"`
	RelayMailbox     string    `json:"relay_mailbox,omitempty"`
	RelayMsgID       string    `json:"relay_msg_id,omitempty"`
	State            string    `json:"state,omitempty"`
	Valid            bool      `json:"valid"`
	Direction        string    `json:"direction"`
}

type State struct {
	SchemaVersion int       `json:"schema_version"`
	UpdatedAt     int64     `json:"updated_at"`
	Contacts      []Contact `json:"contacts"`
	Chats         []Chat    `json:"chats"`
	Messages      []Message `json:"messages,omitempty"`
}

func NewState() *State {
	return &State{
		SchemaVersion: SchemaV1,
		UpdatedAt:     time.Now().Unix(),
		Contacts:      []Contact{},
		Chats:         []Chat{},
		Messages:      []Message{},
	}
}

func (s *State) Validate() error {
	if s == nil {
		return errors.New("chat: state is nil")
	}
	if s.SchemaVersion != SchemaV1 {
		return fmt.Errorf("chat: unsupported schema_version: %d", s.SchemaVersion)
	}
	if len(s.Contacts) > MaxContacts {
		return fmt.Errorf("chat: too many contacts: %d", len(s.Contacts))
	}
	if len(s.Chats) > MaxChats {
		return fmt.Errorf("chat: too many chats: %d", len(s.Chats))
	}
	if len(s.Messages) > MaxMessages*2 {
		return fmt.Errorf("chat: messages unexpectedly large: %d", len(s.Messages))
	}
	seenC := make(map[ContactID]struct{}, len(s.Contacts))
	for i := range s.Contacts {
		c := s.Contacts[i]
		if c.ID == "" {
			return errors.New("chat: contact id required")
		}
		if _, ok := seenC[c.ID]; ok {
			return fmt.Errorf("chat: duplicate contact id: %q", c.ID)
		}
		seenC[c.ID] = struct{}{}
		if c.CreatedAt <= 0 {
			return fmt.Errorf("chat: contact created_at required: %q", c.ID)
		}
		if strings.TrimSpace(c.SignPubB64) == "" {
			return fmt.Errorf("chat: contact sign_pub_b64url required: %q", c.ID)
		}
		if strings.TrimSpace(c.Alias) == "" {
			return fmt.Errorf("chat: contact alias required: %q", c.ID)
		}
	}
	seenChat := make(map[ChatID]struct{}, len(s.Chats))
	for i := range s.Chats {
		ch := s.Chats[i]
		if ch.ID == "" {
			return errors.New("chat: chat id required")
		}
		if _, ok := seenChat[ch.ID]; ok {
			return fmt.Errorf("chat: duplicate chat id: %q", ch.ID)
		}
		seenChat[ch.ID] = struct{}{}
		if ch.CreatedAt <= 0 {
			return fmt.Errorf("chat: chat created_at required: %q", ch.ID)
		}
		if strings.TrimSpace(ch.Kind) == "" {
			return fmt.Errorf("chat: chat kind required: %q", ch.ID)
		}
		if len(ch.Participants) == 0 {
			return fmt.Errorf("chat: participants required: %q", ch.ID)
		}
		if ch.Kind == "group" && len(ch.Roles) > 0 {
			participantSet := make(map[string]struct{}, len(ch.Participants))
			for j := range ch.Participants {
				participantSet[string(ch.Participants[j])] = struct{}{}
			}
			for k, v := range ch.Roles {
				if strings.TrimSpace(k) == "" {
					return fmt.Errorf("chat: group role key empty: %q", ch.ID)
				}
				if _, ok := participantSet[k]; !ok {
					return fmt.Errorf("chat: group role subject not participant: %q", ch.ID)
				}
				switch v {
				case "owner", "moderator", "member":
				default:
					return fmt.Errorf("chat: group role invalid for %q: %q", ch.ID, v)
				}
			}
		}
	}
	for i := range s.Messages {
		m := s.Messages[i]
		if m.ID == "" {
			return errors.New("chat: message id required")
		}
		if m.ChatID == "" {
			return errors.New("chat: message chat_id required")
		}
		if m.CreatedAt <= 0 {
			return errors.New("chat: message created_at required")
		}
		if strings.TrimSpace(m.Direction) == "" {
			return errors.New("chat: message direction required")
		}
		switch strings.TrimSpace(m.State) {
		case "", "queued", "sent", "delivered", "read", "failed":
		default:
			return fmt.Errorf("chat: message state invalid: %q", m.State)
		}
	}
	return nil
}

func (s *State) Prune(now time.Time) {
	if s == nil {
		return
	}
	s.UpdatedAt = now.Unix()
	if len(s.Messages) == 0 {
		return
	}
	sort.Slice(s.Messages, func(i, j int) bool {
		return s.Messages[i].CreatedAt > s.Messages[j].CreatedAt
	})
	if len(s.Messages) > MaxMessages {
		s.Messages = s.Messages[:MaxMessages]
	}
}
