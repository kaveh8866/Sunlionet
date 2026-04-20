package chat

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	PayloadV1         = 1
	KindText          = "chat.text.v1"
	KindGroup         = "group.text.v1"
	KindGroupInvite   = "group.invite.v1"
	KindGroupJoin     = "group.join.v1"
	KindGroupRole     = "group.role.v1"
	KindGroupRemove   = "group.remove.v1"
	KindCommunityPost = "community.post.v1"
)

type SignedPayload struct {
	V            int             `json:"v"`
	Kind         string          `json:"kind"`
	CreatedAt    int64           `json:"created_at"`
	SenderPubB64 string          `json:"sender_pub_b64url"`
	Body         json.RawMessage `json:"body"`
	SigB64       string          `json:"sig_b64url"`
}

type signablePayload struct {
	V            int             `json:"v"`
	Kind         string          `json:"kind"`
	CreatedAt    int64           `json:"created_at"`
	SenderPubB64 string          `json:"sender_pub_b64url"`
	Body         json.RawMessage `json:"body"`
}

type TextBody struct {
	ChatID string `json:"chat_id,omitempty"`
	Text   string `json:"text"`
}

type GroupTextBody struct {
	GroupID string `json:"group_id"`
	Text    string `json:"text"`
}

type GroupInviteBody struct {
	GroupID    string        `json:"group_id"`
	GroupTitle string        `json:"group_title,omitempty"`
	InviteePub string        `json:"invitee_pub_b64url,omitempty"`
	Members    []GroupMember `json:"members,omitempty"`
}

type GroupJoinBody struct {
	GroupID string `json:"group_id"`
}

type GroupRoleBody struct {
	GroupID     string `json:"group_id"`
	SubjectPub  string `json:"subject_pub_b64url"`
	GrantedRole string `json:"granted_role"`
}

type GroupRemoveBody struct {
	GroupID    string `json:"group_id"`
	SubjectPub string `json:"subject_pub_b64url"`
}

type GroupMember struct {
	SignPubB64   string `json:"sign_pub_b64url"`
	Mailbox      string `json:"mailbox,omitempty"`
	PreKeyPubB64 string `json:"prekey_pub_b64url,omitempty"`
}

type CommunityPostBody struct {
	CommunityID string `json:"community_id"`
	RoomID      string `json:"room_id"`
	Text        string `json:"text"`
}

func EncodeText(senderPubB64 string, senderPriv ed25519.PrivateKey, chatID string, text string, now time.Time) ([]byte, error) {
	if strings.TrimSpace(senderPubB64) == "" {
		return nil, errors.New("chat: sender pub required")
	}
	if len(senderPriv) != ed25519.PrivateKeySize {
		return nil, errors.New("chat: invalid sender priv key")
	}
	if now.IsZero() {
		now = time.Now()
	}
	tb := TextBody{ChatID: strings.TrimSpace(chatID), Text: text}
	body, err := json.Marshal(tb)
	if err != nil {
		return nil, err
	}
	signable := signablePayload{
		V:            PayloadV1,
		Kind:         KindText,
		CreatedAt:    now.Unix(),
		SenderPubB64: senderPubB64,
		Body:         body,
	}
	raw, err := json.Marshal(signable)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(senderPriv, raw)
	out := SignedPayload{
		V:            signable.V,
		Kind:         signable.Kind,
		CreatedAt:    signable.CreatedAt,
		SenderPubB64: signable.SenderPubB64,
		Body:         signable.Body,
		SigB64:       base64.RawURLEncoding.EncodeToString(sig),
	}
	return json.Marshal(out)
}

func EncodeGroupText(senderPubB64 string, senderPriv ed25519.PrivateKey, groupID string, text string, now time.Time) ([]byte, error) {
	if strings.TrimSpace(senderPubB64) == "" {
		return nil, errors.New("chat: sender pub required")
	}
	if len(senderPriv) != ed25519.PrivateKeySize {
		return nil, errors.New("chat: invalid sender priv key")
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, errors.New("chat: group id required")
	}
	if now.IsZero() {
		now = time.Now()
	}
	body, err := json.Marshal(GroupTextBody{
		GroupID: groupID,
		Text:    text,
	})
	if err != nil {
		return nil, err
	}
	signable := signablePayload{
		V:            PayloadV1,
		Kind:         KindGroup,
		CreatedAt:    now.Unix(),
		SenderPubB64: senderPubB64,
		Body:         body,
	}
	raw, err := json.Marshal(signable)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(senderPriv, raw)
	out := SignedPayload{
		V:            signable.V,
		Kind:         signable.Kind,
		CreatedAt:    signable.CreatedAt,
		SenderPubB64: signable.SenderPubB64,
		Body:         signable.Body,
		SigB64:       base64.RawURLEncoding.EncodeToString(sig),
	}
	return json.Marshal(out)
}

func EncodeGroupInvite(senderPubB64 string, senderPriv ed25519.PrivateKey, groupID string, groupTitle string, inviteePubB64 string, members []GroupMember, now time.Time) ([]byte, error) {
	if strings.TrimSpace(senderPubB64) == "" {
		return nil, errors.New("chat: sender pub required")
	}
	if len(senderPriv) != ed25519.PrivateKeySize {
		return nil, errors.New("chat: invalid sender priv key")
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, errors.New("chat: group id required")
	}
	if now.IsZero() {
		now = time.Now()
	}
	body, err := json.Marshal(GroupInviteBody{
		GroupID:    groupID,
		GroupTitle: strings.TrimSpace(groupTitle),
		InviteePub: strings.TrimSpace(inviteePubB64),
		Members:    append([]GroupMember(nil), members...),
	})
	if err != nil {
		return nil, err
	}
	return encodeSignedPayload(KindGroupInvite, senderPubB64, senderPriv, body, now.Unix())
}

func EncodeGroupJoin(senderPubB64 string, senderPriv ed25519.PrivateKey, groupID string, now time.Time) ([]byte, error) {
	if strings.TrimSpace(senderPubB64) == "" {
		return nil, errors.New("chat: sender pub required")
	}
	if len(senderPriv) != ed25519.PrivateKeySize {
		return nil, errors.New("chat: invalid sender priv key")
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, errors.New("chat: group id required")
	}
	if now.IsZero() {
		now = time.Now()
	}
	body, err := json.Marshal(GroupJoinBody{
		GroupID: groupID,
	})
	if err != nil {
		return nil, err
	}
	return encodeSignedPayload(KindGroupJoin, senderPubB64, senderPriv, body, now.Unix())
}

func EncodeGroupRole(senderPubB64 string, senderPriv ed25519.PrivateKey, groupID string, subjectPubB64 string, grantedRole string, now time.Time) ([]byte, error) {
	if strings.TrimSpace(senderPubB64) == "" {
		return nil, errors.New("chat: sender pub required")
	}
	if len(senderPriv) != ed25519.PrivateKeySize {
		return nil, errors.New("chat: invalid sender priv key")
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, errors.New("chat: group id required")
	}
	subjectPubB64 = strings.TrimSpace(subjectPubB64)
	if subjectPubB64 == "" {
		return nil, errors.New("chat: subject pub required")
	}
	grantedRole = strings.TrimSpace(grantedRole)
	switch grantedRole {
	case "owner", "moderator", "member":
	default:
		return nil, errors.New("chat: invalid granted role")
	}
	if now.IsZero() {
		now = time.Now()
	}
	body, err := json.Marshal(GroupRoleBody{
		GroupID:     groupID,
		SubjectPub:  subjectPubB64,
		GrantedRole: grantedRole,
	})
	if err != nil {
		return nil, err
	}
	return encodeSignedPayload(KindGroupRole, senderPubB64, senderPriv, body, now.Unix())
}

func EncodeGroupRemove(senderPubB64 string, senderPriv ed25519.PrivateKey, groupID string, subjectPubB64 string, now time.Time) ([]byte, error) {
	if strings.TrimSpace(senderPubB64) == "" {
		return nil, errors.New("chat: sender pub required")
	}
	if len(senderPriv) != ed25519.PrivateKeySize {
		return nil, errors.New("chat: invalid sender priv key")
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, errors.New("chat: group id required")
	}
	subjectPubB64 = strings.TrimSpace(subjectPubB64)
	if subjectPubB64 == "" {
		return nil, errors.New("chat: subject pub required")
	}
	if now.IsZero() {
		now = time.Now()
	}
	body, err := json.Marshal(GroupRemoveBody{
		GroupID:    groupID,
		SubjectPub: subjectPubB64,
	})
	if err != nil {
		return nil, err
	}
	return encodeSignedPayload(KindGroupRemove, senderPubB64, senderPriv, body, now.Unix())
}

func EncodeCommunityPost(senderPubB64 string, senderPriv ed25519.PrivateKey, communityID string, roomID string, text string, now time.Time) ([]byte, error) {
	if strings.TrimSpace(senderPubB64) == "" {
		return nil, errors.New("chat: sender pub required")
	}
	if len(senderPriv) != ed25519.PrivateKeySize {
		return nil, errors.New("chat: invalid sender priv key")
	}
	communityID = strings.TrimSpace(communityID)
	roomID = strings.TrimSpace(roomID)
	if communityID == "" || roomID == "" {
		return nil, errors.New("chat: community_id and room_id required")
	}
	if now.IsZero() {
		now = time.Now()
	}
	body, err := json.Marshal(CommunityPostBody{
		CommunityID: communityID,
		RoomID:      roomID,
		Text:        text,
	})
	if err != nil {
		return nil, err
	}
	return encodeSignedPayload(KindCommunityPost, senderPubB64, senderPriv, body, now.Unix())
}

func CommunityRoomChatID(communityID, roomID string) ChatID {
	return ChatID("c:" + strings.TrimSpace(communityID) + ":" + strings.TrimSpace(roomID))
}

func encodeSignedPayload(kind string, senderPubB64 string, senderPriv ed25519.PrivateKey, body []byte, createdAt int64) ([]byte, error) {
	signable := signablePayload{
		V:            PayloadV1,
		Kind:         kind,
		CreatedAt:    createdAt,
		SenderPubB64: senderPubB64,
		Body:         body,
	}
	raw, err := json.Marshal(signable)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(senderPriv, raw)
	out := SignedPayload{
		V:            signable.V,
		Kind:         signable.Kind,
		CreatedAt:    signable.CreatedAt,
		SenderPubB64: signable.SenderPubB64,
		Body:         signable.Body,
		SigB64:       base64.RawURLEncoding.EncodeToString(sig),
	}
	return json.Marshal(out)
}

func DecodeAndVerify(raw []byte) (SignedPayload, bool, error) {
	var p SignedPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return SignedPayload{}, false, err
	}
	if p.V != PayloadV1 {
		return SignedPayload{}, false, fmt.Errorf("chat: unsupported payload version: %d", p.V)
	}
	if strings.TrimSpace(p.Kind) == "" {
		return SignedPayload{}, false, errors.New("chat: payload kind required")
	}
	if p.CreatedAt <= 0 {
		return SignedPayload{}, false, errors.New("chat: payload created_at required")
	}
	if strings.TrimSpace(p.SenderPubB64) == "" {
		return SignedPayload{}, false, errors.New("chat: payload sender pub required")
	}
	if len(p.Body) == 0 {
		return SignedPayload{}, false, errors.New("chat: payload body required")
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(p.SigB64)
	if err != nil {
		return SignedPayload{}, false, fmt.Errorf("chat: decode sig: %w", err)
	}
	if len(sigBytes) != ed25519.SignatureSize {
		return SignedPayload{}, false, fmt.Errorf("chat: invalid sig size: %d", len(sigBytes))
	}
	pubBytes, err := base64.RawURLEncoding.DecodeString(p.SenderPubB64)
	if err != nil {
		return SignedPayload{}, false, fmt.Errorf("chat: decode sender pub: %w", err)
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return SignedPayload{}, false, fmt.Errorf("chat: invalid sender pub size: %d", len(pubBytes))
	}
	signable := signablePayload{
		V:            p.V,
		Kind:         p.Kind,
		CreatedAt:    p.CreatedAt,
		SenderPubB64: p.SenderPubB64,
		Body:         p.Body,
	}
	signableRaw, err := json.Marshal(signable)
	if err != nil {
		return SignedPayload{}, false, err
	}
	ok := ed25519.Verify(ed25519.PublicKey(pubBytes), signableRaw, sigBytes)
	return p, ok, nil
}

func ContactIDFromSignPub(senderPubB64 string) ContactID {
	sum := sha256.Sum256([]byte(strings.TrimSpace(senderPubB64)))
	return ContactID(base64.RawURLEncoding.EncodeToString(sum[:16]))
}

func DirectChatID(a ContactID, b ContactID) ChatID {
	aa := string(a)
	bb := string(b)
	if aa < bb {
		return ChatID("d:" + aa + ":" + bb)
	}
	return ChatID("d:" + bb + ":" + aa)
}
