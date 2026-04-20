package mobile

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/aipolicy"
	"github.com/kaveh/shadownet-agent/pkg/assistant"
	"github.com/kaveh/shadownet-agent/pkg/chat"
	"github.com/kaveh/shadownet-agent/pkg/community"
	"github.com/kaveh/shadownet-agent/pkg/devsync"
	"github.com/kaveh/shadownet-agent/pkg/identity"
	"github.com/kaveh/shadownet-agent/pkg/messaging"
	"github.com/kaveh/shadownet-agent/pkg/mobilebridge"
	"github.com/kaveh/shadownet-agent/pkg/profile"
	"github.com/kaveh/shadownet-agent/pkg/relay"
)

func StartAgent(config string) error {
	var cfg mobilebridge.AgentConfig
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	if err := validateConfig(&cfg); err != nil {
		return err
	}
	mobilebridge.StartAgent(config)
	return nil
}

func StopAgent() error {
	mobilebridge.StopAgent()
	return nil
}

func ImportBundle(path string) error {
	return mobilebridge.ImportBundle(path)
}

func GetStatus() string {
	return mobilebridge.GetStatus()
}

func CreatePersona(stateDir string, masterKey string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	store, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	state, err := store.Load()
	if err != nil {
		return "", err
	}
	p, err := identity.NewPersona()
	if err != nil {
		return "", err
	}
	state.Personas = append(state.Personas, *p)
	if err := store.Save(state); err != nil {
		return "", err
	}
	out := struct {
		ID         identity.PersonaID `json:"id"`
		SignPubB64 string             `json:"sign_pub_b64url"`
	}{
		ID:         p.ID,
		SignPubB64: p.SignPubB64,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func ListPersonas(stateDir string, masterKey string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	store, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	state, err := store.Load()
	if err != nil {
		return "", err
	}
	type personaView struct {
		ID         identity.PersonaID `json:"id"`
		CreatedAt  int64              `json:"created_at"`
		RotatedAt  int64              `json:"rotated_at,omitempty"`
		SignPubB64 string             `json:"sign_pub_b64url"`
	}
	out := make([]personaView, 0, len(state.Personas))
	for i := range state.Personas {
		out = append(out, personaView{
			ID:         state.Personas[i].ID,
			CreatedAt:  state.Personas[i].CreatedAt,
			RotatedAt:  state.Personas[i].RotatedAt,
			SignPubB64: state.Personas[i].SignPubB64,
		})
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func CreateContactOffer(stateDir string, masterKey string, personaID string, ttlSec int) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	store, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	state, err := store.Load()
	if err != nil {
		return "", err
	}
	var persona *identity.Persona
	for i := range state.Personas {
		if string(state.Personas[i].ID) == personaID {
			persona = &state.Personas[i]
			break
		}
	}
	if persona == nil {
		return "", fmt.Errorf("persona not found: %q", personaID)
	}
	preKey, err := identity.NewPreKey(24 * time.Hour)
	if err != nil {
		return "", err
	}
	mb, created, err := state.EnsureMailboxBinding(persona.ID)
	if err != nil {
		return "", err
	}
	now := time.Now()
	mailbox, _, err := mb.MailboxAt(now, 3600)
	if err != nil {
		return "", err
	}
	state.PreKeys = append(state.PreKeys, *preKey)
	if created {
		state.UpdatedAt = now.Unix()
	}
	if err := store.Save(state); err != nil {
		return "", err
	}
	ttl := time.Duration(ttlSec) * time.Second
	offer, err := identity.NewContactOfferV2(persona, preKey.PubB64, mailbox, nil, ttl)
	if err != nil {
		return "", err
	}
	return offer.Encode()
}

func EncryptToOfferText(offer string, plaintext string) (string, error) {
	o, err := identity.DecodeContactOffer(offer)
	if err != nil {
		return "", err
	}
	preKeyPubBytes, err := base64URL32(o.PreKeyPub)
	if err != nil {
		return "", err
	}
	env, _, err := messaging.EncryptToPreKey([]byte(plaintext), preKeyPubBytes)
	if err != nil {
		return "", err
	}
	return env.Encode()
}

func SendToOfferText(relayURL string, offer string, plaintext string) (string, error) {
	if strings.TrimSpace(relayURL) == "" {
		return "", fmt.Errorf("missing relay_url")
	}
	o, err := identity.DecodeContactOffer(offer)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(o.Mailbox) == "" {
		return "", fmt.Errorf("offer missing mailbox")
	}
	preKeyPubBytes, err := base64URL32(o.PreKeyPub)
	if err != nil {
		return "", err
	}
	env, _, err := messaging.EncryptToPreKey([]byte(plaintext), preKeyPubBytes)
	if err != nil {
		return "", err
	}
	encEnv, err := env.Encode()
	if err != nil {
		return "", err
	}

	r := relay.NewHTTPClient(strings.TrimSpace(relayURL))
	shaped, err := relay.NewShapedRelay(r, relay.SendOptions{
		MinDelayMs:        0,
		MaxDelayMs:        5000,
		BatchMax:          4,
		QueueMax:          50,
		InterPushJitterMs: 120,
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = shaped.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := shaped.Push(ctx, relay.PushRequest{
		Mailbox:  relay.MailboxID(o.Mailbox),
		Envelope: relay.Envelope(encEnv),
	})
	if err != nil {
		return "", err
	}
	return string(id), nil
}

func DecryptEnvelopeToText(stateDir string, masterKey string, envelope string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	store, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	state, err := store.Load()
	if err != nil {
		return "", err
	}
	env, err := messaging.DecodeEnvelope(envelope)
	if err != nil {
		return "", err
	}
	for i := range state.PreKeys {
		priv, err := state.PreKeys[i].DecodePrivate()
		if err != nil {
			continue
		}
		pt, _, err := messaging.DecryptWithPreKey(env, priv)
		if err != nil {
			continue
		}
		state.PreKeys = append(state.PreKeys[:i], state.PreKeys[i+1:]...)
		_ = store.Save(state)
		return string(pt), nil
	}
	return "", fmt.Errorf("no matching prekey for envelope")
}

func ChatAddContactFromOffer(stateDir string, masterKey string, alias string, offer string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	o, err := identity.DecodeContactOffer(offer)
	if err != nil {
		return "", err
	}
	cs, err := chat.NewStore(filepath.Join(stateDir, "chat.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := chat.NewService(cs)
	if err != nil {
		return "", err
	}
	id, err := svc.AddContactFromOffer(alias, o)
	if err != nil {
		return "", err
	}
	out := struct {
		ID chat.ContactID `json:"id"`
	}{
		ID: id,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func ChatList(stateDir string, masterKey string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	cs, err := chat.NewStore(filepath.Join(stateDir, "chat.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := chat.NewService(cs)
	if err != nil {
		return "", err
	}
	st := svc.Snapshot()
	raw, _ := json.Marshal(st.Chats)
	return string(raw), nil
}

func ChatMessages(stateDir string, masterKey string, chatID string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	cs, err := chat.NewStore(filepath.Join(stateDir, "chat.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := chat.NewService(cs)
	if err != nil {
		return "", err
	}
	out := svc.Conversation(chat.ChatID(strings.TrimSpace(chatID)), 0, 200)
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func ChatMarkMessageState(stateDir string, masterKey string, chatID string, messageID string, state string) error {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return fmt.Errorf("invalid master_key: %w", err)
	}
	cs, err := chat.NewStore(filepath.Join(stateDir, "chat.enc"), mk)
	if err != nil {
		return err
	}
	svc, err := chat.NewService(cs)
	if err != nil {
		return err
	}
	return svc.MarkMessageState(chat.ChatID(strings.TrimSpace(chatID)), chat.MessageID(strings.TrimSpace(messageID)), strings.TrimSpace(state))
}

func DeviceCreateJoinRequest(stateDir string, masterKey string, personaID string, deviceID string, devicePubB64 string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	idStore, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	idState, err := idStore.Load()
	if err != nil {
		return "", err
	}
	var p *identity.Persona
	for i := range idState.Personas {
		if string(idState.Personas[i].ID) == strings.TrimSpace(personaID) {
			p = &idState.Personas[i]
			break
		}
	}
	if p == nil {
		return "", fmt.Errorf("persona not found: %q", personaID)
	}
	req, err := identity.NewDeviceJoinRequestForDevice(p.ID, strings.TrimSpace(deviceID), strings.TrimSpace(devicePubB64))
	if err != nil {
		return "", err
	}
	token, err := req.Encode()
	if err != nil {
		return "", err
	}
	out := struct {
		JoinRequest string `json:"join_request"`
	}{
		JoinRequest: token,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func DeviceApproveJoinRequest(stateDir string, masterKey string, personaID string, joinRequest string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	idStore, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	idState, err := idStore.Load()
	if err != nil {
		return "", err
	}
	var p *identity.Persona
	for i := range idState.Personas {
		if string(idState.Personas[i].ID) == strings.TrimSpace(personaID) {
			p = &idState.Personas[i]
			break
		}
	}
	if p == nil {
		return "", fmt.Errorf("persona not found: %q", personaID)
	}
	req, err := identity.DecodeDeviceJoinRequest(strings.TrimSpace(joinRequest))
	if err != nil {
		return "", err
	}
	pkg, err := identity.ApproveDeviceJoinRequest(p, req)
	if err != nil {
		return "", err
	}
	token, err := pkg.Encode()
	if err != nil {
		return "", err
	}
	out := struct {
		JoinPackage string `json:"join_package"`
	}{
		JoinPackage: token,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func DeviceApplyJoinPackage(stateDir string, masterKey string, personaID string, joinPackage string) error {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return fmt.Errorf("invalid master_key: %w", err)
	}
	idStore, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return err
	}
	idState, err := idStore.Load()
	if err != nil {
		return err
	}
	var p *identity.Persona
	for i := range idState.Personas {
		if string(idState.Personas[i].ID) == strings.TrimSpace(personaID) {
			p = &idState.Personas[i]
			break
		}
	}
	if p == nil {
		return fmt.Errorf("persona not found: %q", personaID)
	}
	pkg, err := identity.DecodeDeviceJoinPackage(strings.TrimSpace(joinPackage))
	if err != nil {
		return err
	}
	if err := identity.UpsertDeviceFromJoinPackage(idState, p.SignPubB64, pkg); err != nil {
		return err
	}
	return idStore.Save(idState)
}

func SyncRecordEvent(stateDir string, masterKey string, localDeviceID string, kind string, payloadJSON string, expirySec int) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	ds, err := devsync.NewStore(filepath.Join(stateDir, "devsync.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := devsync.NewService(ds)
	if err != nil {
		return "", err
	}
	if err := svc.SetLocalDeviceID(strings.TrimSpace(localDeviceID)); err != nil {
		return "", err
	}
	expiry := time.Duration(expirySec) * time.Second
	ev, err := svc.Record(strings.TrimSpace(kind), json.RawMessage(payloadJSON), expiry)
	if err != nil {
		return "", err
	}
	raw, _ := json.Marshal(ev)
	return string(raw), nil
}

func SyncBuildBatch(stateDir string, masterKey string, localDeviceID string, peerCursorsJSON string, limit int) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	ds, err := devsync.NewStore(filepath.Join(stateDir, "devsync.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := devsync.NewService(ds)
	if err != nil {
		return "", err
	}
	if err := svc.SetLocalDeviceID(strings.TrimSpace(localDeviceID)); err != nil {
		return "", err
	}
	cursors := map[string]uint64{}
	if strings.TrimSpace(peerCursorsJSON) != "" {
		if err := json.Unmarshal([]byte(peerCursorsJSON), &cursors); err != nil {
			return "", fmt.Errorf("invalid peer_cursors_json: %w", err)
		}
	}
	b := svc.BuildBatch(cursors, limit)
	raw, _ := json.Marshal(b)
	return string(raw), nil
}

func SyncApplyBatch(stateDir string, masterKey string, localDeviceID string, batchJSON string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	ds, err := devsync.NewStore(filepath.Join(stateDir, "devsync.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := devsync.NewService(ds)
	if err != nil {
		return "", err
	}
	if err := svc.SetLocalDeviceID(strings.TrimSpace(localDeviceID)); err != nil {
		return "", err
	}
	var b devsync.Batch
	if err := json.Unmarshal([]byte(batchJSON), &b); err != nil {
		return "", fmt.Errorf("invalid batch_json: %w", err)
	}
	applied, err := svc.ApplyBatch(b)
	if err != nil {
		return "", err
	}
	out := struct {
		Applied int               `json:"applied"`
		Cursors map[string]uint64 `json:"cursors"`
	}{
		Applied: applied,
		Cursors: svc.Snapshot().Cursors,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func SyncNextOutbox(stateDir string, masterKey string, localDeviceID string, nowUnix int64) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	ds, err := devsync.NewStore(filepath.Join(stateDir, "devsync.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := devsync.NewService(ds)
	if err != nil {
		return "", err
	}
	if err := svc.SetLocalDeviceID(strings.TrimSpace(localDeviceID)); err != nil {
		return "", err
	}
	now := time.Now()
	if nowUnix > 0 {
		now = time.Unix(nowUnix, 0)
	}
	ev, ok := svc.NextOutbox(now)
	out := struct {
		HasEvent bool          `json:"has_event"`
		Event    devsync.Event `json:"event,omitempty"`
	}{
		HasEvent: ok,
		Event:    ev,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func SyncAckEvent(stateDir string, masterKey string, localDeviceID string, eventID string) error {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return fmt.Errorf("invalid master_key: %w", err)
	}
	ds, err := devsync.NewStore(filepath.Join(stateDir, "devsync.enc"), mk)
	if err != nil {
		return err
	}
	svc, err := devsync.NewService(ds)
	if err != nil {
		return err
	}
	if err := svc.SetLocalDeviceID(strings.TrimSpace(localDeviceID)); err != nil {
		return err
	}
	return svc.AckEvent(strings.TrimSpace(eventID))
}

func SyncMarkEventRetry(stateDir string, masterKey string, localDeviceID string, eventID string, nowUnix int64) error {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return fmt.Errorf("invalid master_key: %w", err)
	}
	ds, err := devsync.NewStore(filepath.Join(stateDir, "devsync.enc"), mk)
	if err != nil {
		return err
	}
	svc, err := devsync.NewService(ds)
	if err != nil {
		return err
	}
	if err := svc.SetLocalDeviceID(strings.TrimSpace(localDeviceID)); err != nil {
		return err
	}
	now := time.Now()
	if nowUnix > 0 {
		now = time.Unix(nowUnix, 0)
	}
	return svc.MarkEventRetry(strings.TrimSpace(eventID), now)
}

func ChatSendText(relayURL string, stateDir string, masterKey string, personaID string, contactID string, text string) (string, error) {
	if strings.TrimSpace(relayURL) == "" {
		return "", fmt.Errorf("missing relay_url")
	}
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	idStore, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	idState, err := idStore.Load()
	if err != nil {
		return "", err
	}
	var persona *identity.Persona
	for i := range idState.Personas {
		if string(idState.Personas[i].ID) == personaID {
			persona = &idState.Personas[i]
			break
		}
	}
	if persona == nil {
		return "", fmt.Errorf("persona not found: %q", personaID)
	}
	cs, err := chat.NewStore(filepath.Join(stateDir, "chat.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := chat.NewService(cs)
	if err != nil {
		return "", err
	}
	r := relay.NewHTTPClient(strings.TrimSpace(relayURL))
	shaped, err := relay.NewShapedRelay(r, relay.SendOptions{
		MinDelayMs:        0,
		MaxDelayMs:        5000,
		BatchMax:          4,
		QueueMax:          50,
		InterPushJitterMs: 120,
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = shaped.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	msgID, err := svc.SendTextToContact(ctx, shaped, persona, chat.ContactID(contactID), text)
	if err != nil {
		return "", err
	}
	out := struct {
		ID chat.MessageID `json:"id"`
	}{
		ID: msgID,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func ChatCreateGroup(stateDir string, masterKey string, title string, memberIDs string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	cs, err := chat.NewStore(filepath.Join(stateDir, "chat.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := chat.NewService(cs)
	if err != nil {
		return "", err
	}
	parts := strings.Split(memberIDs, ",")
	ids := make([]chat.ContactID, 0, len(parts))
	for i := range parts {
		id := strings.TrimSpace(parts[i])
		if id == "" {
			continue
		}
		ids = append(ids, chat.ContactID(id))
	}
	groupID, err := svc.CreateGroup(title, ids)
	if err != nil {
		return "", err
	}
	out := struct {
		ID chat.ChatID `json:"id"`
	}{
		ID: groupID,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func ChatSendGroupText(relayURL string, stateDir string, masterKey string, personaID string, groupID string, text string) (string, error) {
	if strings.TrimSpace(relayURL) == "" {
		return "", fmt.Errorf("missing relay_url")
	}
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	idStore, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	idState, err := idStore.Load()
	if err != nil {
		return "", err
	}
	var persona *identity.Persona
	for i := range idState.Personas {
		if string(idState.Personas[i].ID) == personaID {
			persona = &idState.Personas[i]
			break
		}
	}
	if persona == nil {
		return "", fmt.Errorf("persona not found: %q", personaID)
	}
	cs, err := chat.NewStore(filepath.Join(stateDir, "chat.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := chat.NewService(cs)
	if err != nil {
		return "", err
	}
	r := relay.NewHTTPClient(strings.TrimSpace(relayURL))
	shaped, err := relay.NewShapedRelay(r, relay.SendOptions{
		MinDelayMs:        0,
		MaxDelayMs:        5000,
		BatchMax:          4,
		QueueMax:          50,
		InterPushJitterMs: 120,
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = shaped.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	msgID, err := svc.SendGroupText(ctx, shaped, persona, chat.ChatID(groupID), text)
	if err != nil {
		return "", err
	}
	out := struct {
		ID chat.MessageID `json:"id"`
	}{
		ID: msgID,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func ChatInviteToGroup(relayURL string, stateDir string, masterKey string, personaID string, groupID string, inviteeContactID string) (string, error) {
	if strings.TrimSpace(relayURL) == "" {
		return "", fmt.Errorf("missing relay_url")
	}
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	idStore, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	idState, err := idStore.Load()
	if err != nil {
		return "", err
	}
	var persona *identity.Persona
	for i := range idState.Personas {
		if string(idState.Personas[i].ID) == personaID {
			persona = &idState.Personas[i]
			break
		}
	}
	if persona == nil {
		return "", fmt.Errorf("persona not found: %q", personaID)
	}
	cs, err := chat.NewStore(filepath.Join(stateDir, "chat.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := chat.NewService(cs)
	if err != nil {
		return "", err
	}
	r := relay.NewHTTPClient(strings.TrimSpace(relayURL))
	shaped, err := relay.NewShapedRelay(r, relay.SendOptions{
		MinDelayMs:        0,
		MaxDelayMs:        5000,
		BatchMax:          4,
		QueueMax:          50,
		InterPushJitterMs: 120,
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = shaped.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	msgID, err := svc.InviteToGroup(ctx, shaped, persona, chat.ChatID(groupID), chat.ContactID(inviteeContactID))
	if err != nil {
		return "", err
	}
	out := struct {
		ID chat.MessageID `json:"id"`
	}{
		ID: msgID,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func ChatJoinGroup(relayURL string, stateDir string, masterKey string, personaID string, groupID string) (string, error) {
	if strings.TrimSpace(relayURL) == "" {
		return "", fmt.Errorf("missing relay_url")
	}
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	idStore, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	idState, err := idStore.Load()
	if err != nil {
		return "", err
	}
	var persona *identity.Persona
	for i := range idState.Personas {
		if string(idState.Personas[i].ID) == personaID {
			persona = &idState.Personas[i]
			break
		}
	}
	if persona == nil {
		return "", fmt.Errorf("persona not found: %q", personaID)
	}
	cs, err := chat.NewStore(filepath.Join(stateDir, "chat.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := chat.NewService(cs)
	if err != nil {
		return "", err
	}
	r := relay.NewHTTPClient(strings.TrimSpace(relayURL))
	shaped, err := relay.NewShapedRelay(r, relay.SendOptions{
		MinDelayMs:        0,
		MaxDelayMs:        5000,
		BatchMax:          4,
		QueueMax:          50,
		InterPushJitterMs: 120,
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = shaped.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	msgID, err := svc.JoinGroup(ctx, shaped, persona, chat.ChatID(groupID))
	if err != nil {
		return "", err
	}
	out := struct {
		ID chat.MessageID `json:"id"`
	}{
		ID: msgID,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func ChatSetGroupRole(relayURL string, stateDir string, masterKey string, personaID string, groupID string, subjectContactID string, role string) (string, error) {
	if strings.TrimSpace(relayURL) == "" {
		return "", fmt.Errorf("missing relay_url")
	}
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	idStore, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	idState, err := idStore.Load()
	if err != nil {
		return "", err
	}
	var persona *identity.Persona
	for i := range idState.Personas {
		if string(idState.Personas[i].ID) == personaID {
			persona = &idState.Personas[i]
			break
		}
	}
	if persona == nil {
		return "", fmt.Errorf("persona not found: %q", personaID)
	}
	cs, err := chat.NewStore(filepath.Join(stateDir, "chat.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := chat.NewService(cs)
	if err != nil {
		return "", err
	}
	r := relay.NewHTTPClient(strings.TrimSpace(relayURL))
	shaped, err := relay.NewShapedRelay(r, relay.SendOptions{
		MinDelayMs:        0,
		MaxDelayMs:        5000,
		BatchMax:          4,
		QueueMax:          50,
		InterPushJitterMs: 120,
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = shaped.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	msgID, err := svc.SetGroupRole(ctx, shaped, persona, chat.ChatID(groupID), chat.ContactID(subjectContactID), role)
	if err != nil {
		return "", err
	}
	out := struct {
		ID chat.MessageID `json:"id"`
	}{
		ID: msgID,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func ChatRemoveFromGroup(relayURL string, stateDir string, masterKey string, personaID string, groupID string, subjectContactID string) (string, error) {
	if strings.TrimSpace(relayURL) == "" {
		return "", fmt.Errorf("missing relay_url")
	}
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	idStore, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	idState, err := idStore.Load()
	if err != nil {
		return "", err
	}
	var persona *identity.Persona
	for i := range idState.Personas {
		if string(idState.Personas[i].ID) == personaID {
			persona = &idState.Personas[i]
			break
		}
	}
	if persona == nil {
		return "", fmt.Errorf("persona not found: %q", personaID)
	}
	cs, err := chat.NewStore(filepath.Join(stateDir, "chat.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := chat.NewService(cs)
	if err != nil {
		return "", err
	}
	r := relay.NewHTTPClient(strings.TrimSpace(relayURL))
	shaped, err := relay.NewShapedRelay(r, relay.SendOptions{
		MinDelayMs:        0,
		MaxDelayMs:        5000,
		BatchMax:          4,
		QueueMax:          50,
		InterPushJitterMs: 120,
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = shaped.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	msgID, err := svc.RemoveFromGroup(ctx, shaped, persona, chat.ChatID(groupID), chat.ContactID(subjectContactID))
	if err != nil {
		return "", err
	}
	out := struct {
		ID chat.MessageID `json:"id"`
	}{
		ID: msgID,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func ChatCreateCommunityRoom(stateDir string, masterKey string, title string, communityID string, roomID string, memberIDs string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	commStore, err := community.NewStore(filepath.Join(stateDir, "community.enc"), mk)
	if err != nil {
		return "", err
	}
	commSvc, err := community.NewService(commStore)
	if err != nil {
		return "", err
	}
	if !commSvc.CanCreateRoom(community.CommunityID(strings.TrimSpace(communityID))) {
		return "", fmt.Errorf("community: room creation permission denied")
	}
	cs, err := chat.NewStore(filepath.Join(stateDir, "chat.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := chat.NewService(cs)
	if err != nil {
		return "", err
	}
	parts := strings.Split(memberIDs, ",")
	ids := make([]chat.ContactID, 0, len(parts))
	for i := range parts {
		id := strings.TrimSpace(parts[i])
		if id == "" {
			continue
		}
		ids = append(ids, chat.ContactID(id))
	}
	chatID, err := svc.CreateCommunityRoom(title, communityID, roomID, ids)
	if err != nil {
		return "", err
	}
	out := struct {
		ID chat.ChatID `json:"id"`
	}{
		ID: chatID,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func ChatSendCommunityPost(relayURL string, stateDir string, masterKey string, personaID string, communityID string, roomID string, text string) (string, error) {
	if strings.TrimSpace(relayURL) == "" {
		return "", fmt.Errorf("missing relay_url")
	}
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	commStore, err := community.NewStore(filepath.Join(stateDir, "community.enc"), mk)
	if err != nil {
		return "", err
	}
	commSvc, err := community.NewService(commStore)
	if err != nil {
		return "", err
	}
	if !commSvc.CanPost(community.CommunityID(strings.TrimSpace(communityID))) {
		return "", fmt.Errorf("community: post permission denied")
	}
	idStore, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	idState, err := idStore.Load()
	if err != nil {
		return "", err
	}
	var persona *identity.Persona
	for i := range idState.Personas {
		if string(idState.Personas[i].ID) == personaID {
			persona = &idState.Personas[i]
			break
		}
	}
	if persona == nil {
		return "", fmt.Errorf("persona not found: %q", personaID)
	}
	cs, err := chat.NewStore(filepath.Join(stateDir, "chat.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := chat.NewService(cs)
	if err != nil {
		return "", err
	}
	r := relay.NewHTTPClient(strings.TrimSpace(relayURL))
	shaped, err := relay.NewShapedRelay(r, relay.SendOptions{
		MinDelayMs:        0,
		MaxDelayMs:        5000,
		BatchMax:          4,
		QueueMax:          50,
		InterPushJitterMs: 120,
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = shaped.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	msgID, err := svc.SendCommunityPost(ctx, shaped, persona, communityID, roomID, text)
	if err != nil {
		return "", err
	}
	out := struct {
		ID chat.MessageID `json:"id"`
	}{
		ID: msgID,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func CommunityList(stateDir string, masterKey string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	store, err := community.NewStore(filepath.Join(stateDir, "community.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := community.NewService(store)
	if err != nil {
		return "", err
	}
	snap := svc.Snapshot()
	raw, _ := json.Marshal(snap.Communities)
	return string(raw), nil
}

func CommunityCreate(stateDir string, masterKey string, communityID string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	store, err := community.NewStore(filepath.Join(stateDir, "community.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := community.NewService(store)
	if err != nil {
		return "", err
	}
	id, err := svc.CreateCommunity(community.CommunityID(strings.TrimSpace(communityID)), community.RoleOwner)
	if err != nil {
		return "", err
	}
	out := struct {
		ID   community.CommunityID `json:"id"`
		Role community.Role        `json:"role"`
	}{
		ID:   id,
		Role: community.RoleOwner,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func CommunityCreateInvite(stateDir string, masterKey string, personaID string, communityID string, ttlSec int, maxUses int) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	idStore, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	idState, err := idStore.Load()
	if err != nil {
		return "", err
	}
	var persona *identity.Persona
	for i := range idState.Personas {
		if string(idState.Personas[i].ID) == personaID {
			persona = &idState.Personas[i]
			break
		}
	}
	if persona == nil {
		return "", fmt.Errorf("persona not found: %q", personaID)
	}
	store, err := community.NewStore(filepath.Join(stateDir, "community.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := community.NewService(store)
	if err != nil {
		return "", err
	}
	ttl := time.Duration(ttlSec) * time.Second
	token, err := svc.CreateInvite(persona, community.CommunityID(strings.TrimSpace(communityID)), ttl, maxUses)
	if err != nil {
		return "", err
	}
	out := struct {
		Invite string `json:"invite"`
	}{
		Invite: token,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func CommunityCreateJoinRequest(stateDir string, masterKey string, personaID string, invite string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	idStore, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	idState, err := idStore.Load()
	if err != nil {
		return "", err
	}
	var persona *identity.Persona
	for i := range idState.Personas {
		if string(idState.Personas[i].ID) == personaID {
			persona = &idState.Personas[i]
			break
		}
	}
	if persona == nil {
		return "", fmt.Errorf("persona not found: %q", personaID)
	}
	store, err := community.NewStore(filepath.Join(stateDir, "community.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := community.NewService(store)
	if err != nil {
		return "", err
	}
	token, err := svc.CreateJoinRequest(persona, invite)
	if err != nil {
		return "", err
	}
	out := struct {
		JoinRequest string `json:"join_request"`
	}{
		JoinRequest: token,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func CommunityApproveJoin(stateDir string, masterKey string, personaID string, invite string, joinRequest string, role string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	idStore, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	idState, err := idStore.Load()
	if err != nil {
		return "", err
	}
	var persona *identity.Persona
	for i := range idState.Personas {
		if string(idState.Personas[i].ID) == personaID {
			persona = &idState.Personas[i]
			break
		}
	}
	if persona == nil {
		return "", fmt.Errorf("persona not found: %q", personaID)
	}
	store, err := community.NewStore(filepath.Join(stateDir, "community.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := community.NewService(store)
	if err != nil {
		return "", err
	}
	token, err := svc.ApproveJoin(persona, invite, joinRequest, community.Role(strings.TrimSpace(role)))
	if err != nil {
		return "", err
	}
	out := struct {
		Approval string `json:"approval"`
	}{
		Approval: token,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

func CommunityApplyJoin(stateDir string, masterKey string, personaID string, invite string, joinRequest string, approval string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	idStore, err := identity.NewStore(filepath.Join(stateDir, "identity.enc"), mk)
	if err != nil {
		return "", err
	}
	idState, err := idStore.Load()
	if err != nil {
		return "", err
	}
	var persona *identity.Persona
	for i := range idState.Personas {
		if string(idState.Personas[i].ID) == personaID {
			persona = &idState.Personas[i]
			break
		}
	}
	if persona == nil {
		return "", fmt.Errorf("persona not found: %q", personaID)
	}
	store, err := community.NewStore(filepath.Join(stateDir, "community.enc"), mk)
	if err != nil {
		return "", err
	}
	svc, err := community.NewService(store)
	if err != nil {
		return "", err
	}
	member, err := svc.ApplyJoin(persona, invite, joinRequest, approval)
	if err != nil {
		return "", err
	}
	raw, _ := json.Marshal(member)
	return string(raw), nil
}

func AIListGrants(stateDir string, masterKey string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	store, err := aipolicy.NewStore(filepath.Join(stateDir, "aipolicy.enc"), mk)
	if err != nil {
		return "", err
	}
	p, err := store.Load()
	if err != nil {
		return "", err
	}
	raw, _ := json.Marshal(p.Grants)
	return string(raw), nil
}

func AIGrant(
	stateDir string,
	masterKey string,
	scopeType string,
	scopeID string,
	actionsJSON string,
	ttlSec int,
	maxItems int,
	purpose string,
) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	store, err := aipolicy.NewStore(filepath.Join(stateDir, "aipolicy.enc"), mk)
	if err != nil {
		return "", err
	}
	p, err := store.Load()
	if err != nil {
		return "", err
	}

	var actions []aipolicy.Action
	if err := json.Unmarshal([]byte(actionsJSON), &actions); err != nil {
		return "", fmt.Errorf("invalid actions_json: %w", err)
	}
	ttl := time.Duration(ttlSec) * time.Second
	g := aipolicy.NewGrant(aipolicy.Scope{Type: aipolicy.ScopeType(scopeType), ID: scopeID}, actions, ttl)
	g.MaxItems = maxItems
	g.Purpose = strings.TrimSpace(purpose)
	if err := g.Validate(); err != nil {
		return "", err
	}
	p.Grants = append(p.Grants, g)
	if err := store.Save(p); err != nil {
		return "", err
	}
	raw, _ := json.Marshal(g)
	return string(raw), nil
}

func AIClearScope(stateDir string, masterKey string, scopeType string, scopeID string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	store, err := aipolicy.NewStore(filepath.Join(stateDir, "aipolicy.enc"), mk)
	if err != nil {
		return "", err
	}
	p, err := store.Load()
	if err != nil {
		return "", err
	}
	kept := p.Grants[:0]
	for i := range p.Grants {
		g := p.Grants[i]
		if string(g.Scope.Type) == scopeType && g.Scope.ID == scopeID {
			continue
		}
		kept = append(kept, g)
	}
	p.Grants = kept
	if err := store.Save(p); err != nil {
		return "", err
	}
	raw, _ := json.Marshal(p.Grants)
	return string(raw), nil
}

func AIInvoke(stateDir string, masterKey string, requestJSON string, localURL string, remoteURL string) (string, error) {
	mk, err := profile.ParseMasterKey(masterKey)
	if err != nil {
		return "", fmt.Errorf("invalid master_key: %w", err)
	}
	pstore, err := aipolicy.NewStore(filepath.Join(stateDir, "aipolicy.enc"), mk)
	if err != nil {
		return "", err
	}
	pol, err := pstore.Load()
	if err != nil {
		return "", err
	}

	var req assistant.InvokeRequest
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		return "", fmt.Errorf("invalid request_json: %w", err)
	}

	ctrl := &assistant.Controller{
		Local:  &assistant.LlamaCPPProvider{ServerURL: localURL, StopOnEOT: true},
		Remote: &assistant.LlamaCPPProvider{ServerURL: remoteURL, StopOnEOT: true},
		Audit:  assistant.NoopAuditSink{},
	}
	res, err := ctrl.Invoke(context.Background(), pol, req)
	if err != nil {
		return "", err
	}
	raw, _ := json.Marshal(res)
	return string(raw), nil
}

func validateConfig(cfg *mobilebridge.AgentConfig) error {
	if strings.TrimSpace(cfg.StateDir) == "" {
		return fmt.Errorf("missing state_dir")
	}
	if _, err := profile.ParseMasterKey(cfg.MasterKey); err != nil {
		return fmt.Errorf("invalid master_key: %w", err)
	}
	if strings.TrimSpace(cfg.TemplatesDir) == "" {
		return fmt.Errorf("missing templates_dir")
	}
	if cfg.PollIntervalSec <= 0 {
		cfg.PollIntervalSec = 20
	}
	if cfg.PollIntervalSec < 10 {
		cfg.PollIntervalSec = 10
	}
	if cfg.PollIntervalSec > 300 {
		cfg.PollIntervalSec = 300
	}
	if cfg.PiTimeoutMS <= 0 {
		cfg.PiTimeoutMS = 1200
	}
	if strings.TrimSpace(cfg.PiCommand) == "" {
		cfg.PiCommand = "pi"
	}
	return nil
}

func base64URL32(s string) ([32]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return [32]byte{}, fmt.Errorf("decode base64url: %w", err)
	}
	if len(b) != 32 {
		return [32]byte{}, fmt.Errorf("expected 32 bytes, got %d", len(b))
	}
	var out [32]byte
	copy(out[:], b)
	return out, nil
}
