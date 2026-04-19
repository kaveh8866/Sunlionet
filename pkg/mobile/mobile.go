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
