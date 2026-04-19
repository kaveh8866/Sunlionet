package identity

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"time"
)

const (
	SchemaV1 = 1

	MaxPreKeys   = 64
	MaxDevices   = 16
	MaxMailboxes = 64
)

type PersonaID string

type Persona struct {
	ID          PersonaID `json:"id"`
	CreatedAt   int64     `json:"created_at"`
	RotatedAt   int64     `json:"rotated_at,omitempty"`
	SignPubB64  string    `json:"sign_pub_b64url"`
	SignPrivB64 string    `json:"sign_priv_b64url"`
}

func NewPersona() (*Persona, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	id, err := newPersonaID()
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	return &Persona{
		ID:          id,
		CreatedAt:   now,
		SignPubB64:  base64.RawURLEncoding.EncodeToString(pub),
		SignPrivB64: base64.RawURLEncoding.EncodeToString(priv),
	}, nil
}

func (p *Persona) SignKeypair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	if p == nil {
		return nil, nil, errors.New("identity: persona is nil")
	}
	pub, err := base64.RawURLEncoding.DecodeString(p.SignPubB64)
	if err != nil {
		return nil, nil, fmt.Errorf("identity: decode sign_pub_b64url: %w", err)
	}
	priv, err := base64.RawURLEncoding.DecodeString(p.SignPrivB64)
	if err != nil {
		return nil, nil, fmt.Errorf("identity: decode sign_priv_b64url: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return nil, nil, fmt.Errorf("identity: invalid public key size: %d", len(pub))
	}
	if len(priv) != ed25519.PrivateKeySize {
		return nil, nil, fmt.Errorf("identity: invalid private key size: %d", len(priv))
	}
	return ed25519.PublicKey(pub), ed25519.PrivateKey(priv), nil
}

func (p *Persona) RotateSigningKey() error {
	if p == nil {
		return errors.New("identity: persona is nil")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	p.SignPubB64 = base64.RawURLEncoding.EncodeToString(pub)
	p.SignPrivB64 = base64.RawURLEncoding.EncodeToString(priv)
	p.RotatedAt = time.Now().Unix()
	return nil
}

func (p *Persona) Validate() error {
	if p == nil {
		return errors.New("identity: persona is nil")
	}
	if p.ID == "" {
		return errors.New("identity: persona id required")
	}
	if p.CreatedAt <= 0 {
		return errors.New("identity: created_at required")
	}
	_, _, err := p.SignKeypair()
	return err
}

type State struct {
	SchemaVersion int              `json:"schema_version"`
	UpdatedAt     int64            `json:"updated_at"`
	Personas      []Persona        `json:"personas"`
	PreKeys       []PreKey         `json:"prekeys,omitempty"`
	Devices       []Device         `json:"devices,omitempty"`
	Mailboxes     []MailboxBinding `json:"mailboxes,omitempty"`
}

func NewState() *State {
	return &State{
		SchemaVersion: SchemaV1,
		UpdatedAt:     time.Now().Unix(),
		Personas:      []Persona{},
		PreKeys:       []PreKey{},
		Devices:       []Device{},
		Mailboxes:     []MailboxBinding{},
	}
}

func (s *State) Validate() error {
	if s == nil {
		return errors.New("identity: state is nil")
	}
	if s.SchemaVersion != SchemaV1 {
		return fmt.Errorf("identity: unsupported schema_version: %d", s.SchemaVersion)
	}
	seen := make(map[PersonaID]struct{}, len(s.Personas))
	for i := range s.Personas {
		p := s.Personas[i]
		if err := p.Validate(); err != nil {
			return err
		}
		if _, ok := seen[p.ID]; ok {
			return fmt.Errorf("identity: duplicate persona id: %q", p.ID)
		}
		seen[p.ID] = struct{}{}
	}
	if len(s.Mailboxes) > MaxMailboxes*2 {
		return fmt.Errorf("identity: too many mailboxes: %d", len(s.Mailboxes))
	}
	seenMB := make(map[PersonaID]struct{}, len(s.Mailboxes))
	for i := range s.Mailboxes {
		mb := s.Mailboxes[i]
		if err := mb.Validate(); err != nil {
			return err
		}
		if _, ok := seen[mb.PersonaID]; !ok {
			return fmt.Errorf("identity: mailbox binding references unknown persona: %q", mb.PersonaID)
		}
		if _, ok := seenMB[mb.PersonaID]; ok {
			return fmt.Errorf("identity: duplicate mailbox binding persona_id: %q", mb.PersonaID)
		}
		seenMB[mb.PersonaID] = struct{}{}
	}
	if len(s.PreKeys) > MaxPreKeys*2 {
		return fmt.Errorf("identity: too many prekeys: %d", len(s.PreKeys))
	}
	now := time.Now()
	seenPre := make(map[string]struct{}, len(s.PreKeys))
	for i := range s.PreKeys {
		k := s.PreKeys[i]
		if err := k.Validate(now); err != nil {
			return err
		}
		if _, ok := seenPre[k.ID]; ok {
			return fmt.Errorf("identity: duplicate prekey id: %q", k.ID)
		}
		seenPre[k.ID] = struct{}{}
	}
	if len(s.Devices) > MaxDevices*2 {
		return fmt.Errorf("identity: too many devices: %d", len(s.Devices))
	}
	seenDev := make(map[string]struct{}, len(s.Devices))
	for i := range s.Devices {
		d := s.Devices[i]
		if err := d.Validate(); err != nil {
			return err
		}
		key := string(d.PersonaID) + ":" + d.DeviceID
		if _, ok := seenDev[key]; ok {
			return fmt.Errorf("identity: duplicate device: %q", key)
		}
		seenDev[key] = struct{}{}
	}
	return nil
}

func (s *State) Prune(now time.Time) {
	s.UpdatedAt = now.Unix()

	personaSet := make(map[PersonaID]struct{}, len(s.Personas))
	for i := range s.Personas {
		personaSet[s.Personas[i].ID] = struct{}{}
	}
	if len(s.Mailboxes) > 0 {
		keptMB := make([]MailboxBinding, 0, len(s.Mailboxes))
		for i := range s.Mailboxes {
			if _, ok := personaSet[s.Mailboxes[i].PersonaID]; !ok {
				continue
			}
			keptMB = append(keptMB, s.Mailboxes[i])
		}
		if len(keptMB) > MaxMailboxes {
			keptMB = keptMB[len(keptMB)-MaxMailboxes:]
		}
		s.Mailboxes = keptMB
	}

	kept := make([]PreKey, 0, len(s.PreKeys))
	for i := range s.PreKeys {
		if now.Unix() <= s.PreKeys[i].ExpiresAt {
			kept = append(kept, s.PreKeys[i])
		}
	}
	if len(kept) > MaxPreKeys {
		kept = kept[len(kept)-MaxPreKeys:]
	}
	s.PreKeys = kept
}

type MailboxBinding struct {
	PersonaID PersonaID `json:"persona_id"`
	CreatedAt int64     `json:"created_at"`
	RotatedAt int64     `json:"rotated_at,omitempty"`
	SeedB64   string    `json:"seed_b64url"`
	PhaseSec  int64     `json:"phase_sec"`
}

func (b MailboxBinding) Validate() error {
	if b.PersonaID == "" {
		return errors.New("identity: mailbox persona_id required")
	}
	if b.CreatedAt <= 0 {
		return errors.New("identity: mailbox created_at required")
	}
	if b.SeedB64 == "" {
		return errors.New("identity: mailbox seed_b64url required")
	}
	seed, err := base64.RawURLEncoding.DecodeString(b.SeedB64)
	if err != nil {
		return fmt.Errorf("identity: decode mailbox seed_b64url: %w", err)
	}
	if len(seed) != 32 {
		return fmt.Errorf("identity: invalid mailbox seed size: %d", len(seed))
	}
	if b.PhaseSec < 0 {
		return errors.New("identity: mailbox phase_sec must be >= 0")
	}
	return nil
}

func (b MailboxBinding) Seed() ([]byte, error) {
	seed, err := base64.RawURLEncoding.DecodeString(b.SeedB64)
	if err != nil {
		return nil, fmt.Errorf("identity: decode mailbox seed_b64url: %w", err)
	}
	if len(seed) != 32 {
		return nil, fmt.Errorf("identity: invalid mailbox seed size: %d", len(seed))
	}
	return seed, nil
}

func (b MailboxBinding) DeriveInt(label string, min int, max int) (int, error) {
	if min > max {
		min, max = max, min
	}
	if min == max {
		return min, nil
	}
	seed, err := b.Seed()
	if err != nil {
		return 0, err
	}
	mac := hmac.New(sha256.New, seed)
	_, _ = mac.Write([]byte("sn-derive-v1:"))
	_, _ = mac.Write([]byte(label))
	sum := mac.Sum(nil)
	v := binary.BigEndian.Uint64(sum[:8])
	span := uint64(max - min + 1)
	return min + int(v%span), nil
}

func (b MailboxBinding) MailboxAt(now time.Time, rotationPeriodSec int64) (string, time.Time, error) {
	epoch, shift, period := b.mailboxEpoch(now, rotationPeriodSec)
	id, err := b.mailboxForEpoch(epoch)
	if err != nil {
		return "", time.Time{}, err
	}
	next := time.Unix((epoch+1)*period-shift, 0)
	return id, next, nil
}

func (b MailboxBinding) MailboxPrevAt(now time.Time, rotationPeriodSec int64) (string, error) {
	epoch, _, _ := b.mailboxEpoch(now, rotationPeriodSec)
	return b.mailboxForEpoch(epoch - 1)
}

func (b MailboxBinding) DecoyMailboxesAt(now time.Time, rotationPeriodSec int64, count int) ([]string, error) {
	if count <= 0 {
		return nil, nil
	}
	epoch, _, _ := b.mailboxEpoch(now, rotationPeriodSec)
	primary, err := b.mailboxForEpoch(epoch)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, count)
	for i := 0; len(out) < count && i < count*5; i++ {
		id, err := b.decoyForEpoch(epoch, i)
		if err != nil {
			continue
		}
		if id == primary {
			continue
		}
		out = append(out, id)
	}
	return out, nil
}

func (b MailboxBinding) mailboxEpoch(now time.Time, rotationPeriodSec int64) (epoch int64, shift int64, period int64) {
	period = rotationPeriodSec
	if period <= 0 {
		period = 3600
	}
	if period < 60 {
		period = 60
	}
	if period > 7*24*3600 {
		period = 7 * 24 * 3600
	}
	shift = 0
	if b.PhaseSec > 0 {
		shift = b.PhaseSec % period
	}
	epoch = (now.Unix() + shift) / period
	return epoch, shift, period
}

func (b MailboxBinding) mailboxForEpoch(epoch int64) (string, error) {
	seed, err := b.Seed()
	if err != nil {
		return "", err
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(epoch))
	mac := hmac.New(sha256.New, seed)
	_, _ = mac.Write([]byte("sn-mailbox-v1:primary:"))
	_, _ = mac.Write(buf[:])
	sum := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(sum[:16]), nil
}

func (b MailboxBinding) decoyForEpoch(epoch int64, i int) (string, error) {
	seed, err := b.Seed()
	if err != nil {
		return "", err
	}
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[:8], uint64(epoch))
	binary.BigEndian.PutUint64(buf[8:], uint64(i))
	mac := hmac.New(sha256.New, seed)
	_, _ = mac.Write([]byte("sn-mailbox-v1:decoy:"))
	_, _ = mac.Write(buf[:])
	sum := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(sum[:16]), nil
}

func (s *State) EnsureMailboxBinding(personaID PersonaID) (*MailboxBinding, bool, error) {
	if s == nil {
		return nil, false, errors.New("identity: state is nil")
	}
	if personaID == "" {
		return nil, false, errors.New("identity: personaID is empty")
	}
	for i := range s.Mailboxes {
		if s.Mailboxes[i].PersonaID == personaID {
			return &s.Mailboxes[i], false, nil
		}
	}
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return nil, false, err
	}
	phase, err := randInt63n(24 * 3600)
	if err != nil {
		return nil, false, err
	}
	now := time.Now().Unix()
	s.Mailboxes = append(s.Mailboxes, MailboxBinding{
		PersonaID: personaID,
		CreatedAt: now,
		SeedB64:   base64.RawURLEncoding.EncodeToString(seed[:]),
		PhaseSec:  phase,
	})
	return &s.Mailboxes[len(s.Mailboxes)-1], true, nil
}

func newPersonaID() (PersonaID, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return PersonaID(base64.RawURLEncoding.EncodeToString(b[:])), nil
}

func randInt63n(n int64) (int64, error) {
	if n <= 0 {
		return 0, errors.New("identity: invalid rand bound")
	}
	v, err := rand.Int(rand.Reader, big.NewInt(n))
	if err != nil {
		return 0, err
	}
	return v.Int64(), nil
}
