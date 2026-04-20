package chat

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/identity"
	"github.com/kaveh/sunlionet-agent/pkg/messaging"
	"github.com/kaveh/sunlionet-agent/pkg/relay"
)

type Service struct {
	store *Store
	mu    sync.Mutex
	state *State
}

func NewService(store *Store) (*Service, error) {
	if store == nil {
		return nil, errors.New("chat: store is nil")
	}
	st, err := store.Load()
	if err != nil {
		return nil, err
	}
	return &Service{store: store, state: st}, nil
}

func (s *Service) Snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *s.state
	cp.Contacts = append([]Contact(nil), s.state.Contacts...)
	cp.Chats = append([]Chat(nil), s.state.Chats...)
	cp.Messages = append([]Message(nil), s.state.Messages...)
	return cp
}

func (s *Service) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.Save(s.state)
}

func (s *Service) Conversation(chatID ChatID, beforeCreatedAt int64, limit int) []Message {
	if s == nil || chatID == "" {
		return nil
	}
	if limit <= 0 {
		limit = 200
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Message, 0, min(limit, 64))
	for i := range s.state.Messages {
		m := s.state.Messages[i]
		if m.ChatID != chatID {
			continue
		}
		if beforeCreatedAt > 0 && m.CreatedAt >= beforeCreatedAt {
			continue
		}
		out = append(out, m)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (s *Service) MarkMessageState(chatID ChatID, messageID MessageID, state string) error {
	if s == nil {
		return errors.New("chat: service is nil")
	}
	if chatID == "" || messageID == "" {
		return errors.New("chat: chat_id and message_id required")
	}
	state = strings.TrimSpace(state)
	switch state {
	case "sent", "delivered", "read", "failed":
	default:
		return errors.New("chat: invalid message state")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Messages {
		if s.state.Messages[i].ChatID == chatID && s.state.Messages[i].ID == messageID {
			s.state.Messages[i].State = state
			s.state.UpdatedAt = time.Now().Unix()
			return s.store.Save(s.state)
		}
	}
	return errors.New("chat: message not found")
}

func (s *Service) AddContactFromOffer(alias string, offer *identity.ContactOffer) (ContactID, error) {
	if s == nil {
		return "", errors.New("chat: service is nil")
	}
	if offer == nil {
		return "", errors.New("chat: offer is nil")
	}
	if err := offer.Validate(time.Now()); err != nil {
		return "", err
	}
	alias = strings.TrimSpace(alias)
	if alias == "" {
		alias = "contact-" + offer.PersonaPub[:min(8, len(offer.PersonaPub))]
	}
	id := ContactIDFromSignPub(offer.PersonaPub)

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	for i := range s.state.Contacts {
		if s.state.Contacts[i].ID != id {
			continue
		}
		s.state.Contacts[i].Alias = alias
		s.state.Contacts[i].Mailbox = offer.Mailbox
		s.state.Contacts[i].PreKeyPubB64 = offer.PreKeyPub
		s.state.Contacts[i].RelayHints = append([]string(nil), offer.RelayHints...)
		s.state.Contacts[i].LastUpdatedAt = now
		s.state.UpdatedAt = now
		return id, s.store.Save(s.state)
	}
	s.state.Contacts = append(s.state.Contacts, Contact{
		ID:            id,
		CreatedAt:     now,
		Alias:         alias,
		SignPubB64:    offer.PersonaPub,
		Mailbox:       offer.Mailbox,
		PreKeyPubB64:  offer.PreKeyPub,
		RelayHints:    append([]string(nil), offer.RelayHints...),
		Trust:         "unverified",
		LastUpdatedAt: now,
	})
	s.state.UpdatedAt = now
	return id, s.store.Save(s.state)
}

func (s *Service) SendTextToContact(ctx context.Context, r relay.Relay, persona *identity.Persona, contactID ContactID, text string) (MessageID, error) {
	if s == nil {
		return "", errors.New("chat: service is nil")
	}
	if r == nil {
		return "", errors.New("chat: relay is nil")
	}
	if persona == nil {
		return "", errors.New("chat: persona is nil")
	}
	_, priv, err := persona.SignKeypair()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	var c *Contact
	for i := range s.state.Contacts {
		if s.state.Contacts[i].ID == contactID {
			c = &s.state.Contacts[i]
			break
		}
	}
	s.mu.Unlock()
	if c == nil {
		return "", fmt.Errorf("chat: unknown contact: %q", contactID)
	}
	if strings.TrimSpace(c.Mailbox) == "" {
		return "", errors.New("chat: contact mailbox missing")
	}
	if strings.TrimSpace(c.PreKeyPubB64) == "" {
		return "", errors.New("chat: contact prekey missing")
	}
	preKeyBytes, err := base64.RawURLEncoding.DecodeString(c.PreKeyPubB64)
	if err != nil {
		return "", fmt.Errorf("chat: decode contact prekey: %w", err)
	}
	if len(preKeyBytes) != 32 {
		return "", fmt.Errorf("chat: invalid contact prekey size: %d", len(preKeyBytes))
	}
	var preKeyPub [32]byte
	copy(preKeyPub[:], preKeyBytes)

	selfID := ContactIDFromSignPub(persona.SignPubB64)
	chatID := DirectChatID(selfID, contactID)
	payload, err := EncodeText(persona.SignPubB64, ed25519.PrivateKey(priv), string(chatID), text, time.Now())
	if err != nil {
		return "", err
	}
	env, _, err := messaging.EncryptToPreKey(payload, preKeyPub)
	if err != nil {
		return "", err
	}
	envStr, err := env.Encode()
	if err != nil {
		return "", err
	}
	_, err = r.Push(ctx, relay.PushRequest{
		Mailbox:  relay.MailboxID(c.Mailbox),
		Envelope: relay.Envelope(envStr),
	})
	if err != nil {
		return "", err
	}

	now := time.Now().Unix()
	msgID := MessageID(base64.RawURLEncoding.EncodeToString(random16()))

	s.mu.Lock()
	defer s.mu.Unlock()
	ensureDirectChatLocked(s.state, chatID, contactID, c.Alias, now)
	s.state.Messages = append(s.state.Messages, Message{
		ID:               msgID,
		ClientID:         string(msgID),
		ChatID:           chatID,
		CreatedAt:        now,
		SortKey:          now,
		SenderSignPubB64: persona.SignPubB64,
		Text:             text,
		Envelope:         envStr,
		State:            "sent",
		Valid:            true,
		Direction:        "out",
	})
	s.state.UpdatedAt = now
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == chatID {
			s.state.Chats[i].LastMsgAt = now
			break
		}
	}
	return msgID, s.store.Save(s.state)
}

func (s *Service) CreateGroup(title string, members []ContactID) (ChatID, error) {
	if s == nil {
		return "", errors.New("chat: service is nil")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return "", errors.New("chat: group title required")
	}
	dedup := make(map[ContactID]struct{}, len(members))
	outMembers := make([]ContactID, 0, len(members))
	for i := range members {
		id := ContactID(strings.TrimSpace(string(members[i])))
		if id == "" {
			continue
		}
		if _, ok := dedup[id]; ok {
			continue
		}
		dedup[id] = struct{}{}
		outMembers = append(outMembers, id)
	}
	if len(outMembers) == 0 {
		return "", errors.New("chat: at least one member required")
	}
	now := time.Now().Unix()
	chatID := ChatID("g:" + base64.RawURLEncoding.EncodeToString(random16()))

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range outMembers {
		if !hasContactLocked(s.state, outMembers[i]) {
			return "", fmt.Errorf("chat: unknown group member: %q", outMembers[i])
		}
	}
	s.state.Chats = append(s.state.Chats, Chat{
		ID:           chatID,
		Kind:         "group",
		CreatedAt:    now,
		Title:        title,
		Participants: outMembers,
		LastMsgAt:    now,
	})
	s.state.UpdatedAt = now
	return chatID, s.store.Save(s.state)
}

func (s *Service) SendGroupText(ctx context.Context, r relay.Relay, persona *identity.Persona, groupID ChatID, text string) (MessageID, error) {
	if s == nil {
		return "", errors.New("chat: service is nil")
	}
	if r == nil {
		return "", errors.New("chat: relay is nil")
	}
	if persona == nil {
		return "", errors.New("chat: persona is nil")
	}
	_, priv, err := persona.SignKeypair()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	var ch *Chat
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == groupID {
			ch = &s.state.Chats[i]
			break
		}
	}
	if ch == nil {
		s.mu.Unlock()
		return "", fmt.Errorf("chat: unknown group: %q", groupID)
	}
	if ch.Kind != "group" {
		s.mu.Unlock()
		return "", fmt.Errorf("chat: chat is not group: %q", groupID)
	}
	selfID := ContactIDFromSignPub(persona.SignPubB64)
	if !containsContactID(ch.Participants, selfID) {
		if len(ch.Roles) != 0 {
			s.mu.Unlock()
			return "", errors.New("chat: not a group member")
		}
		now := time.Now().Unix()
		_ = ensureContactLocked(s.state, selfID, persona.SignPubB64, now)
		ch.Participants = append(ch.Participants, selfID)
		if ch.Roles == nil {
			ch.Roles = make(map[string]string, 4)
		}
		if !hasOwnerRole(ch.Roles) {
			ch.Roles[string(selfID)] = "owner"
		}
	}
	participants := append([]ContactID(nil), ch.Participants...)
	contactMap := make(map[ContactID]Contact, len(participants))
	for i := range participants {
		c, ok := getContactLocked(s.state, participants[i])
		if !ok {
			s.mu.Unlock()
			return "", fmt.Errorf("chat: group member not found: %q", participants[i])
		}
		contactMap[participants[i]] = c
	}
	s.mu.Unlock()

	payload, err := EncodeGroupText(persona.SignPubB64, ed25519.PrivateKey(priv), string(groupID), text, time.Now())
	if err != nil {
		return "", err
	}
	delivered := 0
	for i := range participants {
		if sendPayloadToContact(ctx, r, contactMap[participants[i]], payload) {
			delivered++
		}
	}
	if delivered == 0 {
		return "", errors.New("chat: group send failed for all recipients")
	}

	now := time.Now().Unix()
	msgID := MessageID(base64.RawURLEncoding.EncodeToString(random16()))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Messages = append(s.state.Messages, Message{
		ID:               msgID,
		ClientID:         string(msgID),
		ChatID:           groupID,
		CreatedAt:        now,
		SortKey:          now,
		SenderSignPubB64: persona.SignPubB64,
		Text:             text,
		State:            "sent",
		Valid:            true,
		Direction:        "out",
	})
	s.state.UpdatedAt = now
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == groupID {
			s.state.Chats[i].LastMsgAt = now
			break
		}
	}
	return msgID, s.store.Save(s.state)
}

func (s *Service) InviteToGroup(ctx context.Context, r relay.Relay, persona *identity.Persona, groupID ChatID, invitee ContactID) (MessageID, error) {
	if s == nil {
		return "", errors.New("chat: service is nil")
	}
	if r == nil {
		return "", errors.New("chat: relay is nil")
	}
	if persona == nil {
		return "", errors.New("chat: persona is nil")
	}
	_, priv, err := persona.SignKeypair()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	var group *Chat
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == groupID {
			group = &s.state.Chats[i]
			break
		}
	}
	inviteeContact, ok := getContactLocked(s.state, invitee)
	if group == nil || group.Kind != "group" || !ok {
		s.mu.Unlock()
		return "", errors.New("chat: invalid group or invitee")
	}
	now := time.Now().Unix()
	selfID := ContactIDFromSignPub(persona.SignPubB64)
	_ = ensureContactLocked(s.state, selfID, persona.SignPubB64, now)
	ensureGroupParticipantLocked(s.state, groupID, selfID)
	if group.Roles == nil {
		group.Roles = make(map[string]string, 4)
	}
	if !hasOwnerRole(group.Roles) {
		group.Roles[string(selfID)] = "owner"
	}
	if !canInviteLocked(group.Roles, selfID) {
		s.mu.Unlock()
		return "", errors.New("chat: invite permission denied")
	}
	members := make([]GroupMember, 0, len(group.Participants))
	for i := range group.Participants {
		if group.Participants[i] == invitee {
			continue
		}
		c, ok := getContactLocked(s.state, group.Participants[i])
		if !ok {
			continue
		}
		if strings.TrimSpace(c.SignPubB64) == "" {
			continue
		}
		members = append(members, GroupMember{
			SignPubB64:   c.SignPubB64,
			Mailbox:      strings.TrimSpace(c.Mailbox),
			PreKeyPubB64: strings.TrimSpace(c.PreKeyPubB64),
		})
	}
	payload, err := EncodeGroupInvite(persona.SignPubB64, ed25519.PrivateKey(priv), string(groupID), group.Title, inviteeContact.SignPubB64, members, time.Now())
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	s.mu.Unlock()
	if !sendPayloadToContact(ctx, r, inviteeContact, payload) {
		return "", errors.New("chat: failed to send group invite")
	}

	msgID := MessageID(base64.RawURLEncoding.EncodeToString(random16()))
	s.mu.Lock()
	defer s.mu.Unlock()
	ensureGroupParticipantLocked(s.state, groupID, invitee)
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == groupID {
			if s.state.Chats[i].Roles == nil {
				s.state.Chats[i].Roles = make(map[string]string, 4)
			}
			s.state.Chats[i].Roles[string(invitee)] = "member"
			break
		}
	}
	s.state.Messages = append(s.state.Messages, Message{
		ID:               msgID,
		ClientID:         string(msgID),
		ChatID:           groupID,
		CreatedAt:        now,
		SortKey:          now,
		SenderSignPubB64: persona.SignPubB64,
		Text:             "group invite sent",
		State:            "sent",
		Valid:            true,
		Direction:        "out",
	})
	s.state.UpdatedAt = now
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == groupID {
			s.state.Chats[i].LastMsgAt = now
			break
		}
	}
	return msgID, s.store.Save(s.state)
}

func (s *Service) JoinGroup(ctx context.Context, r relay.Relay, persona *identity.Persona, groupID ChatID) (MessageID, error) {
	if s == nil {
		return "", errors.New("chat: service is nil")
	}
	if r == nil {
		return "", errors.New("chat: relay is nil")
	}
	if persona == nil {
		return "", errors.New("chat: persona is nil")
	}
	_, priv, err := persona.SignKeypair()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	var group *Chat
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == groupID {
			group = &s.state.Chats[i]
			break
		}
	}
	if group == nil || group.Kind != "group" {
		s.mu.Unlock()
		return "", fmt.Errorf("chat: group not found: %q", groupID)
	}
	participants := append([]ContactID(nil), group.Participants...)
	contacts := make([]Contact, 0, len(participants))
	for i := range participants {
		c, ok := getContactLocked(s.state, participants[i])
		if ok {
			contacts = append(contacts, c)
		}
	}
	s.mu.Unlock()

	payload, err := EncodeGroupJoin(persona.SignPubB64, ed25519.PrivateKey(priv), string(groupID), time.Now())
	if err != nil {
		return "", err
	}
	delivered := 0
	for i := range contacts {
		if sendPayloadToContact(ctx, r, contacts[i], payload) {
			delivered++
		}
	}
	if delivered == 0 {
		return "", errors.New("chat: failed to send group join")
	}

	now := time.Now().Unix()
	msgID := MessageID(base64.RawURLEncoding.EncodeToString(random16()))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Messages = append(s.state.Messages, Message{
		ID:               msgID,
		ClientID:         string(msgID),
		ChatID:           groupID,
		CreatedAt:        now,
		SortKey:          now,
		SenderSignPubB64: persona.SignPubB64,
		Text:             "joined group",
		State:            "sent",
		Valid:            true,
		Direction:        "out",
	})
	s.state.UpdatedAt = now
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == groupID {
			s.state.Chats[i].LastMsgAt = now
			break
		}
	}
	return msgID, s.store.Save(s.state)
}

func (s *Service) SetGroupRole(ctx context.Context, r relay.Relay, persona *identity.Persona, groupID ChatID, subject ContactID, role string) (MessageID, error) {
	if s == nil {
		return "", errors.New("chat: service is nil")
	}
	if r == nil {
		return "", errors.New("chat: relay is nil")
	}
	if persona == nil {
		return "", errors.New("chat: persona is nil")
	}
	role = strings.TrimSpace(role)
	switch role {
	case "owner", "moderator", "member":
	default:
		return "", errors.New("chat: invalid role")
	}
	_, priv, err := persona.SignKeypair()
	if err != nil {
		return "", err
	}

	selfID := ContactIDFromSignPub(persona.SignPubB64)
	s.mu.Lock()
	var group *Chat
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == groupID {
			group = &s.state.Chats[i]
			break
		}
	}
	subjectContact, ok := getContactLocked(s.state, subject)
	if group == nil || group.Kind != "group" || !ok {
		s.mu.Unlock()
		return "", errors.New("chat: invalid group or subject")
	}
	if !canManageRolesLocked(s.state, groupID, selfID) {
		s.mu.Unlock()
		return "", errors.New("chat: role permission denied")
	}
	participants := append([]ContactID(nil), group.Participants...)
	contacts := make([]Contact, 0, len(participants))
	for i := range participants {
		c, ok := getContactLocked(s.state, participants[i])
		if ok {
			contacts = append(contacts, c)
		}
	}
	s.mu.Unlock()

	payload, err := EncodeGroupRole(persona.SignPubB64, ed25519.PrivateKey(priv), string(groupID), subjectContact.SignPubB64, role, time.Now())
	if err != nil {
		return "", err
	}
	delivered := 0
	for i := range contacts {
		if sendPayloadToContact(ctx, r, contacts[i], payload) {
			delivered++
		}
	}
	if !containsContact(contacts, subjectContact) {
		if sendPayloadToContact(ctx, r, subjectContact, payload) {
			delivered++
		}
	}
	if delivered == 0 {
		return "", errors.New("chat: failed to send group role")
	}

	now := time.Now().Unix()
	msgID := MessageID(base64.RawURLEncoding.EncodeToString(random16()))
	s.mu.Lock()
	defer s.mu.Unlock()
	ensureGroupParticipantLocked(s.state, groupID, subject)
	ensureGroupRoleLocked(s.state, groupID, subject, role)
	s.state.Messages = append(s.state.Messages, Message{
		ID:               msgID,
		ClientID:         string(msgID),
		ChatID:           groupID,
		CreatedAt:        now,
		SortKey:          now,
		SenderSignPubB64: persona.SignPubB64,
		Text:             "group role updated",
		State:            "sent",
		Valid:            true,
		Direction:        "out",
	})
	s.state.UpdatedAt = now
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == groupID {
			s.state.Chats[i].LastMsgAt = now
			break
		}
	}
	return msgID, s.store.Save(s.state)
}

func (s *Service) RemoveFromGroup(ctx context.Context, r relay.Relay, persona *identity.Persona, groupID ChatID, subject ContactID) (MessageID, error) {
	if s == nil {
		return "", errors.New("chat: service is nil")
	}
	if r == nil {
		return "", errors.New("chat: relay is nil")
	}
	if persona == nil {
		return "", errors.New("chat: persona is nil")
	}
	_, priv, err := persona.SignKeypair()
	if err != nil {
		return "", err
	}

	selfID := ContactIDFromSignPub(persona.SignPubB64)
	s.mu.Lock()
	var group *Chat
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == groupID {
			group = &s.state.Chats[i]
			break
		}
	}
	subjectContact, ok := getContactLocked(s.state, subject)
	if group == nil || group.Kind != "group" || !ok {
		s.mu.Unlock()
		return "", errors.New("chat: invalid group or subject")
	}
	if !canManageMembershipLocked(s.state, groupID, selfID) {
		s.mu.Unlock()
		return "", errors.New("chat: remove permission denied")
	}
	if group.Roles != nil && group.Roles[string(subject)] == "owner" {
		ownerCount := 0
		for _, v := range group.Roles {
			if v == "owner" {
				ownerCount++
			}
		}
		if ownerCount <= 1 {
			s.mu.Unlock()
			return "", errors.New("chat: cannot remove last owner")
		}
	}
	participants := append([]ContactID(nil), group.Participants...)
	contacts := make([]Contact, 0, len(participants))
	for i := range participants {
		c, ok := getContactLocked(s.state, participants[i])
		if ok {
			contacts = append(contacts, c)
		}
	}
	s.mu.Unlock()

	payload, err := EncodeGroupRemove(persona.SignPubB64, ed25519.PrivateKey(priv), string(groupID), subjectContact.SignPubB64, time.Now())
	if err != nil {
		return "", err
	}
	delivered := 0
	for i := range contacts {
		if sendPayloadToContact(ctx, r, contacts[i], payload) {
			delivered++
		}
	}
	if !containsContact(contacts, subjectContact) {
		if sendPayloadToContact(ctx, r, subjectContact, payload) {
			delivered++
		}
	}
	if delivered == 0 {
		return "", errors.New("chat: failed to send group remove")
	}

	now := time.Now().Unix()
	msgID := MessageID(base64.RawURLEncoding.EncodeToString(random16()))
	s.mu.Lock()
	defer s.mu.Unlock()
	removeGroupParticipantLocked(s.state, groupID, subject)
	s.state.Messages = append(s.state.Messages, Message{
		ID:               msgID,
		ClientID:         string(msgID),
		ChatID:           groupID,
		CreatedAt:        now,
		SortKey:          now,
		SenderSignPubB64: persona.SignPubB64,
		Text:             "removed from group",
		State:            "sent",
		Valid:            true,
		Direction:        "out",
	})
	s.state.UpdatedAt = now
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == groupID {
			s.state.Chats[i].LastMsgAt = now
			break
		}
	}
	return msgID, s.store.Save(s.state)
}

func (s *Service) CreateCommunityRoom(title string, communityID string, roomID string, members []ContactID) (ChatID, error) {
	if s == nil {
		return "", errors.New("chat: service is nil")
	}
	title = strings.TrimSpace(title)
	communityID = strings.TrimSpace(communityID)
	roomID = strings.TrimSpace(roomID)
	if title == "" || communityID == "" || roomID == "" {
		return "", errors.New("chat: title, community_id and room_id required")
	}
	dedup := make(map[ContactID]struct{}, len(members))
	outMembers := make([]ContactID, 0, len(members))
	for i := range members {
		id := ContactID(strings.TrimSpace(string(members[i])))
		if id == "" {
			continue
		}
		if _, ok := dedup[id]; ok {
			continue
		}
		dedup[id] = struct{}{}
		outMembers = append(outMembers, id)
	}
	if len(outMembers) == 0 {
		return "", errors.New("chat: at least one member required")
	}
	chatID := CommunityRoomChatID(communityID, roomID)
	now := time.Now().Unix()

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range outMembers {
		if !hasContactLocked(s.state, outMembers[i]) {
			return "", fmt.Errorf("chat: unknown community member: %q", outMembers[i])
		}
	}
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == chatID {
			return chatID, nil
		}
	}
	s.state.Chats = append(s.state.Chats, Chat{
		ID:           chatID,
		Kind:         "community",
		CreatedAt:    now,
		Title:        title,
		Participants: outMembers,
		LastMsgAt:    now,
	})
	s.state.UpdatedAt = now
	return chatID, s.store.Save(s.state)
}

func (s *Service) SendCommunityPost(ctx context.Context, r relay.Relay, persona *identity.Persona, communityID string, roomID string, text string) (MessageID, error) {
	if s == nil {
		return "", errors.New("chat: service is nil")
	}
	if r == nil {
		return "", errors.New("chat: relay is nil")
	}
	if persona == nil {
		return "", errors.New("chat: persona is nil")
	}
	_, priv, err := persona.SignKeypair()
	if err != nil {
		return "", err
	}
	chatID := CommunityRoomChatID(communityID, roomID)

	s.mu.Lock()
	var ch *Chat
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == chatID {
			ch = &s.state.Chats[i]
			break
		}
	}
	if ch == nil || ch.Kind != "community" {
		s.mu.Unlock()
		return "", fmt.Errorf("chat: community room not found: %q", chatID)
	}
	participants := append([]ContactID(nil), ch.Participants...)
	contactMap := make(map[ContactID]Contact, len(participants))
	for i := range participants {
		c, ok := getContactLocked(s.state, participants[i])
		if !ok {
			s.mu.Unlock()
			return "", fmt.Errorf("chat: community member not found: %q", participants[i])
		}
		contactMap[participants[i]] = c
	}
	s.mu.Unlock()

	payload, err := EncodeCommunityPost(persona.SignPubB64, ed25519.PrivateKey(priv), communityID, roomID, text, time.Now())
	if err != nil {
		return "", err
	}
	delivered := 0
	for i := range participants {
		if sendPayloadToContact(ctx, r, contactMap[participants[i]], payload) {
			delivered++
		}
	}
	if delivered == 0 {
		return "", errors.New("chat: community post failed for all recipients")
	}

	now := time.Now().Unix()
	msgID := MessageID(base64.RawURLEncoding.EncodeToString(random16()))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Messages = append(s.state.Messages, Message{
		ID:               msgID,
		ClientID:         string(msgID),
		ChatID:           chatID,
		CreatedAt:        now,
		SortKey:          now,
		SenderSignPubB64: persona.SignPubB64,
		Text:             text,
		State:            "sent",
		Valid:            true,
		Direction:        "out",
	})
	s.state.UpdatedAt = now
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == chatID {
			s.state.Chats[i].LastMsgAt = now
			break
		}
	}
	return msgID, s.store.Save(s.state)
}

func (s *Service) ApplyIncoming(persona *identity.Persona, relayMsg relay.Message, envStr string, plaintext []byte) error {
	if s == nil {
		return errors.New("chat: service is nil")
	}
	if persona == nil {
		return errors.New("chat: persona is nil")
	}
	p, ok, err := DecodeAndVerify(plaintext)
	if err != nil {
		return err
	}
	if p.Kind != KindText && p.Kind != KindGroup && p.Kind != KindGroupInvite && p.Kind != KindGroupJoin && p.Kind != KindGroupRole && p.Kind != KindGroupRemove && p.Kind != KindCommunityPost {
		return nil
	}
	var (
		text   string
		chatID ChatID
	)
	contactID := ContactIDFromSignPub(p.SenderPubB64)
	selfID := ContactIDFromSignPub(persona.SignPubB64)
	if p.Kind == KindText {
		var tb TextBody
		if err := json.Unmarshal(p.Body, &tb); err != nil {
			return err
		}
		text = tb.Text
		chatID = DirectChatID(selfID, contactID)
	}
	if p.Kind == KindGroup {
		var gb GroupTextBody
		if err := json.Unmarshal(p.Body, &gb); err != nil {
			return err
		}
		text = gb.Text
		chatID = ChatID(gb.GroupID)
	}
	if p.Kind == KindGroupInvite {
		var ib GroupInviteBody
		if err := json.Unmarshal(p.Body, &ib); err != nil {
			return err
		}
		if strings.TrimSpace(ib.InviteePub) != "" && strings.TrimSpace(ib.InviteePub) != strings.TrimSpace(persona.SignPubB64) {
			return nil
		}
		text = "group invite"
		chatID = ChatID(strings.TrimSpace(ib.GroupID))
	}
	if p.Kind == KindGroupJoin {
		var jb GroupJoinBody
		if err := json.Unmarshal(p.Body, &jb); err != nil {
			return err
		}
		text = "group join"
		chatID = ChatID(jb.GroupID)
	}
	if p.Kind == KindGroupRole {
		var rb GroupRoleBody
		if err := json.Unmarshal(p.Body, &rb); err != nil {
			return err
		}
		text = "group role"
		chatID = ChatID(strings.TrimSpace(rb.GroupID))
	}
	if p.Kind == KindGroupRemove {
		var rb GroupRemoveBody
		if err := json.Unmarshal(p.Body, &rb); err != nil {
			return err
		}
		text = "group remove"
		chatID = ChatID(strings.TrimSpace(rb.GroupID))
	}
	if p.Kind == KindCommunityPost {
		var cb CommunityPostBody
		if err := json.Unmarshal(p.Body, &cb); err != nil {
			return err
		}
		text = cb.Text
		chatID = CommunityRoomChatID(cb.CommunityID, cb.RoomID)
	}
	now := time.Now().Unix()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Relay may redeliver envelopes; keep persistence idempotent.
	for i := range s.state.Messages {
		if s.state.Messages[i].Direction != "in" {
			continue
		}
		if s.state.Messages[i].RelayMsgID == string(relayMsg.ID) && s.state.Messages[i].RelayMailbox == string(relayMsg.Mailbox) {
			return nil
		}
	}

	contactAlias := ensureContactLocked(s.state, contactID, p.SenderPubB64, now)
	if p.Kind == KindText {
		ensureDirectChatLocked(s.state, chatID, contactID, contactAlias, now)
	}
	if p.Kind == KindGroup {
		ensureGroupChatLocked(s.state, chatID, contactID, contactAlias, now)
	}
	if p.Kind == KindGroupInvite || p.Kind == KindGroupJoin || p.Kind == KindGroupRole || p.Kind == KindGroupRemove {
		ensureGroupChatLocked(s.state, chatID, contactID, contactAlias, now)
	}
	if p.Kind == KindGroupInvite {
		var ib GroupInviteBody
		if err := json.Unmarshal(p.Body, &ib); err == nil {
			if strings.TrimSpace(ib.GroupTitle) != "" {
				for i := range s.state.Chats {
					if s.state.Chats[i].ID == chatID && (strings.TrimSpace(s.state.Chats[i].Title) == "" || strings.HasPrefix(s.state.Chats[i].Title, "group-")) {
						s.state.Chats[i].Title = strings.TrimSpace(ib.GroupTitle)
						break
					}
				}
			}
			for i := range ib.Members {
				m := ib.Members[i]
				if strings.TrimSpace(m.SignPubB64) == "" {
					continue
				}
				mid := ContactIDFromSignPub(m.SignPubB64)
				ensureContactFromInviteLocked(s.state, mid, m, now)
				ensureGroupParticipantLocked(s.state, chatID, mid)
			}
			ensureGroupParticipantLocked(s.state, chatID, selfID)
			ensureGroupParticipantLocked(s.state, chatID, contactID)
			ensureGroupInviteRolesLocked(s.state, chatID, contactID, selfID)
		}
	}
	if p.Kind == KindGroupJoin {
		ensureGroupParticipantLocked(s.state, chatID, contactID)
		ensureGroupRoleLocked(s.state, chatID, contactID, "member")
	}
	if p.Kind == KindGroupRole {
		var rb GroupRoleBody
		if err := json.Unmarshal(p.Body, &rb); err == nil {
			subjectID := ContactIDFromSignPub(rb.SubjectPub)
			if canManageRolesLocked(s.state, chatID, contactID) {
				ensureGroupParticipantLocked(s.state, chatID, subjectID)
				ensureGroupRoleLocked(s.state, chatID, subjectID, strings.TrimSpace(rb.GrantedRole))
			}
		}
	}
	if p.Kind == KindGroupRemove {
		var rb GroupRemoveBody
		if err := json.Unmarshal(p.Body, &rb); err == nil {
			subjectID := ContactIDFromSignPub(rb.SubjectPub)
			if canManageMembershipLocked(s.state, chatID, contactID) {
				removeGroupParticipantLocked(s.state, chatID, subjectID)
			}
		}
	}
	if p.Kind == KindCommunityPost {
		ensureCommunityChatLocked(s.state, chatID, contactID, "community", now)
	}

	msgID := MessageID(base64.RawURLEncoding.EncodeToString(random16()))
	s.state.Messages = append(s.state.Messages, Message{
		ID:               msgID,
		ClientID:         string(msgID),
		ChatID:           chatID,
		CreatedAt:        p.CreatedAt,
		SortKey:          p.CreatedAt,
		SenderSignPubB64: p.SenderPubB64,
		Text:             text,
		Envelope:         envStr,
		RelayMailbox:     string(relayMsg.Mailbox),
		RelayMsgID:       string(relayMsg.ID),
		State:            map[bool]string{true: "delivered", false: "failed"}[ok],
		Valid:            ok,
		Direction:        "in",
	})
	s.state.UpdatedAt = now
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == chatID {
			s.state.Chats[i].LastMsgAt = now
			s.state.Chats[i].Unread++
			break
		}
	}
	return s.store.Save(s.state)
}

func (s *Service) ApplyIncomingWithRelay(ctx context.Context, r relay.Relay, persona *identity.Persona, relayMsg relay.Message, envStr string, plaintext []byte) error {
	if s == nil {
		return errors.New("chat: service is nil")
	}
	if persona == nil {
		return errors.New("chat: persona is nil")
	}
	p, ok, err := DecodeAndVerify(plaintext)
	if err != nil {
		return err
	}
	if err := s.ApplyIncoming(persona, relayMsg, envStr, plaintext); err != nil {
		return err
	}
	if r == nil || ctx == nil {
		return nil
	}
	if !ok || p.Kind != KindGroupJoin {
		return nil
	}
	var jb GroupJoinBody
	if err := json.Unmarshal(p.Body, &jb); err != nil {
		return nil
	}
	groupID := ChatID(strings.TrimSpace(jb.GroupID))
	if groupID == "" {
		return nil
	}
	return s.broadcastGroupMemberAdded(ctx, r, persona, groupID, p.SenderPubB64)
}

func ensureContactLocked(st *State, id ContactID, signPubB64 string, now int64) string {
	for i := range st.Contacts {
		if st.Contacts[i].ID != id {
			continue
		}
		if st.Contacts[i].SignPubB64 == "" {
			st.Contacts[i].SignPubB64 = signPubB64
		}
		if st.Contacts[i].LastUpdatedAt < now {
			st.Contacts[i].LastUpdatedAt = now
		}
		if strings.TrimSpace(st.Contacts[i].Alias) != "" {
			return st.Contacts[i].Alias
		}
		break
	}
	alias := "contact-" + signPubB64[:min(8, len(signPubB64))]
	st.Contacts = append(st.Contacts, Contact{
		ID:            id,
		CreatedAt:     now,
		Alias:         alias,
		SignPubB64:    signPubB64,
		Trust:         "unverified",
		LastUpdatedAt: now,
	})
	return alias
}

func ensureContactFromInviteLocked(st *State, id ContactID, m GroupMember, now int64) {
	for i := range st.Contacts {
		if st.Contacts[i].ID != id {
			continue
		}
		if strings.TrimSpace(st.Contacts[i].SignPubB64) == "" {
			st.Contacts[i].SignPubB64 = strings.TrimSpace(m.SignPubB64)
		}
		if strings.TrimSpace(st.Contacts[i].Mailbox) == "" && strings.TrimSpace(m.Mailbox) != "" {
			st.Contacts[i].Mailbox = strings.TrimSpace(m.Mailbox)
		}
		if strings.TrimSpace(st.Contacts[i].PreKeyPubB64) == "" && strings.TrimSpace(m.PreKeyPubB64) != "" {
			st.Contacts[i].PreKeyPubB64 = strings.TrimSpace(m.PreKeyPubB64)
		}
		if st.Contacts[i].LastUpdatedAt < now {
			st.Contacts[i].LastUpdatedAt = now
		}
		return
	}
	alias := "contact-" + strings.TrimSpace(m.SignPubB64)[:min(8, len(strings.TrimSpace(m.SignPubB64)))]
	st.Contacts = append(st.Contacts, Contact{
		ID:            id,
		CreatedAt:     now,
		Alias:         alias,
		SignPubB64:    strings.TrimSpace(m.SignPubB64),
		Mailbox:       strings.TrimSpace(m.Mailbox),
		PreKeyPubB64:  strings.TrimSpace(m.PreKeyPubB64),
		Trust:         "unverified",
		LastUpdatedAt: now,
	})
}

func hasOwnerRole(roles map[string]string) bool {
	for _, v := range roles {
		if v == "owner" {
			return true
		}
	}
	return false
}

func canInviteLocked(roles map[string]string, actor ContactID) bool {
	r := roles[string(actor)]
	return r == "owner" || r == "moderator"
}

func canManageMembershipLocked(st *State, groupID ChatID, actor ContactID) bool {
	for i := range st.Chats {
		if st.Chats[i].ID != groupID || st.Chats[i].Kind != "group" {
			continue
		}
		if st.Chats[i].Roles == nil {
			return false
		}
		r := st.Chats[i].Roles[string(actor)]
		return r == "owner" || r == "moderator"
	}
	return false
}

func canManageRolesLocked(st *State, groupID ChatID, actor ContactID) bool {
	for i := range st.Chats {
		if st.Chats[i].ID != groupID || st.Chats[i].Kind != "group" {
			continue
		}
		if st.Chats[i].Roles == nil {
			return false
		}
		return st.Chats[i].Roles[string(actor)] == "owner"
	}
	return false
}

func ensureGroupRoleLocked(st *State, groupID ChatID, member ContactID, role string) {
	role = strings.TrimSpace(role)
	switch role {
	case "owner", "moderator", "member":
	default:
		return
	}
	for i := range st.Chats {
		if st.Chats[i].ID != groupID || st.Chats[i].Kind != "group" {
			continue
		}
		if st.Chats[i].Roles == nil {
			st.Chats[i].Roles = make(map[string]string, 4)
		}
		st.Chats[i].Roles[string(member)] = role
		return
	}
}

func ensureGroupInviteRolesLocked(st *State, groupID ChatID, inviter ContactID, self ContactID) {
	for i := range st.Chats {
		if st.Chats[i].ID != groupID || st.Chats[i].Kind != "group" {
			continue
		}
		if st.Chats[i].Roles == nil {
			st.Chats[i].Roles = make(map[string]string, 4)
		}
		if !hasOwnerRole(st.Chats[i].Roles) {
			st.Chats[i].Roles[string(inviter)] = "owner"
		}
		if _, ok := st.Chats[i].Roles[string(self)]; !ok {
			st.Chats[i].Roles[string(self)] = "member"
		}
		if _, ok := st.Chats[i].Roles[string(inviter)]; !ok {
			st.Chats[i].Roles[string(inviter)] = "owner"
		}
		return
	}
}

func removeGroupParticipantLocked(st *State, groupID ChatID, member ContactID) {
	for i := range st.Chats {
		if st.Chats[i].ID != groupID || st.Chats[i].Kind != "group" {
			continue
		}
		parts := st.Chats[i].Participants[:0]
		for j := range st.Chats[i].Participants {
			if st.Chats[i].Participants[j] == member {
				continue
			}
			parts = append(parts, st.Chats[i].Participants[j])
		}
		st.Chats[i].Participants = parts
		if st.Chats[i].Roles != nil {
			delete(st.Chats[i].Roles, string(member))
			if len(st.Chats[i].Roles) == 0 {
				st.Chats[i].Roles = nil
			}
		}
		return
	}
}

func (s *Service) broadcastGroupMemberAdded(ctx context.Context, r relay.Relay, persona *identity.Persona, groupID ChatID, newMemberPubB64 string) error {
	if s == nil || persona == nil || r == nil {
		return nil
	}
	newMemberPubB64 = strings.TrimSpace(newMemberPubB64)
	if newMemberPubB64 == "" {
		return nil
	}
	if strings.TrimSpace(persona.SignPubB64) == newMemberPubB64 {
		return nil
	}
	_, priv, err := persona.SignKeypair()
	if err != nil {
		return nil
	}
	newMemberID := ContactIDFromSignPub(newMemberPubB64)

	s.mu.Lock()
	var ch *Chat
	for i := range s.state.Chats {
		if s.state.Chats[i].ID == groupID {
			ch = &s.state.Chats[i]
			break
		}
	}
	if ch == nil || ch.Kind != "group" {
		s.mu.Unlock()
		return nil
	}
	selfID := ContactIDFromSignPub(persona.SignPubB64)
	if !canInviteLocked(ch.Roles, selfID) {
		s.mu.Unlock()
		return nil
	}
	title := ch.Title
	participants := append([]ContactID(nil), ch.Participants...)
	contactMap := make(map[ContactID]Contact, len(participants))
	for i := range participants {
		c, ok := getContactLocked(s.state, participants[i])
		if ok {
			contactMap[participants[i]] = c
		}
	}
	newMemberContact, ok := contactMap[newMemberID]
	if !ok {
		newMemberContact, ok = getContactLocked(s.state, newMemberID)
	}
	s.mu.Unlock()
	if !ok {
		return nil
	}
	newMember := GroupMember{
		SignPubB64:   newMemberPubB64,
		Mailbox:      strings.TrimSpace(newMemberContact.Mailbox),
		PreKeyPubB64: strings.TrimSpace(newMemberContact.PreKeyPubB64),
	}
	if newMember.Mailbox == "" || newMember.PreKeyPubB64 == "" {
		return nil
	}

	updatePayload, err := EncodeGroupInvite(persona.SignPubB64, ed25519.PrivateKey(priv), string(groupID), title, "", []GroupMember{newMember}, time.Now())
	if err == nil {
		for i := range participants {
			if participants[i] == newMemberID {
				continue
			}
			c, ok := contactMap[participants[i]]
			if ok {
				_ = sendPayloadToContact(ctx, r, c, updatePayload)
			}
		}
	}

	fullMembers := make([]GroupMember, 0, len(participants))
	for i := range participants {
		if participants[i] == newMemberID {
			continue
		}
		c, ok := contactMap[participants[i]]
		if !ok {
			continue
		}
		if strings.TrimSpace(c.SignPubB64) == "" {
			continue
		}
		fullMembers = append(fullMembers, GroupMember{
			SignPubB64:   strings.TrimSpace(c.SignPubB64),
			Mailbox:      strings.TrimSpace(c.Mailbox),
			PreKeyPubB64: strings.TrimSpace(c.PreKeyPubB64),
		})
	}
	syncPayload, err := EncodeGroupInvite(persona.SignPubB64, ed25519.PrivateKey(priv), string(groupID), title, newMemberPubB64, fullMembers, time.Now())
	if err == nil {
		_ = sendPayloadToContact(ctx, r, newMemberContact, syncPayload)
	}
	return nil
}

func ensureDirectChatLocked(st *State, chatID ChatID, contactID ContactID, title string, now int64) {
	for i := range st.Chats {
		if st.Chats[i].ID == chatID {
			if st.Chats[i].Title == "" {
				st.Chats[i].Title = title
			}
			return
		}
	}
	st.Chats = append(st.Chats, Chat{
		ID:           chatID,
		Kind:         "direct",
		CreatedAt:    now,
		Title:        title,
		Participants: []ContactID{contactID},
	})
}

func ensureGroupChatLocked(st *State, chatID ChatID, contactID ContactID, title string, now int64) {
	for i := range st.Chats {
		if st.Chats[i].ID != chatID {
			continue
		}
		if st.Chats[i].Kind == "" {
			st.Chats[i].Kind = "group"
		}
		if st.Chats[i].Title == "" {
			st.Chats[i].Title = "group-" + title
		}
		if !containsContactID(st.Chats[i].Participants, contactID) {
			st.Chats[i].Participants = append(st.Chats[i].Participants, contactID)
		}
		return
	}
	st.Chats = append(st.Chats, Chat{
		ID:           chatID,
		Kind:         "group",
		CreatedAt:    now,
		Title:        "group-" + title,
		Participants: []ContactID{contactID},
	})
}

func ensureGroupParticipantLocked(st *State, groupID ChatID, member ContactID) {
	for i := range st.Chats {
		if st.Chats[i].ID != groupID {
			continue
		}
		if !containsContactID(st.Chats[i].Participants, member) {
			st.Chats[i].Participants = append(st.Chats[i].Participants, member)
		}
		return
	}
}

func ensureCommunityChatLocked(st *State, chatID ChatID, contactID ContactID, title string, now int64) {
	for i := range st.Chats {
		if st.Chats[i].ID != chatID {
			continue
		}
		if st.Chats[i].Kind == "" {
			st.Chats[i].Kind = "community"
		}
		if st.Chats[i].Title == "" {
			st.Chats[i].Title = title
		}
		if !containsContactID(st.Chats[i].Participants, contactID) {
			st.Chats[i].Participants = append(st.Chats[i].Participants, contactID)
		}
		return
	}
	st.Chats = append(st.Chats, Chat{
		ID:           chatID,
		Kind:         "community",
		CreatedAt:    now,
		Title:        title,
		Participants: []ContactID{contactID},
	})
}

func sendPayloadToContact(ctx context.Context, r relay.Relay, c Contact, payload []byte) bool {
	if r == nil {
		return false
	}
	if strings.TrimSpace(c.Mailbox) == "" || strings.TrimSpace(c.PreKeyPubB64) == "" {
		return false
	}
	preKeyBytes, err := base64.RawURLEncoding.DecodeString(c.PreKeyPubB64)
	if err != nil || len(preKeyBytes) != 32 {
		return false
	}
	var preKeyPub [32]byte
	copy(preKeyPub[:], preKeyBytes)
	env, _, err := messaging.EncryptToPreKey(payload, preKeyPub)
	if err != nil {
		return false
	}
	envStr, err := env.Encode()
	if err != nil {
		return false
	}
	_, err = r.Push(ctx, relay.PushRequest{
		Mailbox:  relay.MailboxID(c.Mailbox),
		Envelope: relay.Envelope(envStr),
	})
	return err == nil
}

func hasContactLocked(st *State, id ContactID) bool {
	for i := range st.Contacts {
		if st.Contacts[i].ID == id {
			return true
		}
	}
	return false
}

func getContactLocked(st *State, id ContactID) (Contact, bool) {
	for i := range st.Contacts {
		if st.Contacts[i].ID == id {
			return st.Contacts[i], true
		}
	}
	return Contact{}, false
}

func containsContactID(ids []ContactID, id ContactID) bool {
	for i := range ids {
		if ids[i] == id {
			return true
		}
	}
	return false
}

func containsContact(cs []Contact, c Contact) bool {
	for i := range cs {
		if cs[i].ID == c.ID {
			return true
		}
	}
	return false
}

func random16() []byte {
	var b [16]byte
	_, _ = io.ReadFull(rand.Reader, b[:])
	return b[:]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
