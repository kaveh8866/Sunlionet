package identity_manager

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/curve25519"
)

type NodeID [8]byte

type Identity struct {
	NodeID NodeID
	PubKey [32]byte
}

type Manager struct {
	mu             sync.Mutex
	rotationPeriod time.Duration
	current        Identity
	currentPriv    [32]byte
	rotatedAt      time.Time
}

func New(rotationPeriod time.Duration) (*Manager, error) {
	if rotationPeriod <= 0 {
		return nil, errors.New("rotation_period must be > 0")
	}
	m := &Manager{rotationPeriod: rotationPeriod}
	m.rotateLocked(time.Now())
	return m, nil
}

func (m *Manager) Current(now time.Time) Identity {
	m.mu.Lock()
	defer m.mu.Unlock()
	if now.Sub(m.rotatedAt) >= m.rotationPeriod {
		m.rotateLocked(now)
	}
	return m.current
}

func (m *Manager) PrivateKey(now time.Time) [32]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	if now.Sub(m.rotatedAt) >= m.rotationPeriod {
		m.rotateLocked(now)
	}
	return m.currentPriv
}

func (m *Manager) rotateLocked(now time.Time) {
	var priv [32]byte
	_, _ = rand.Read(priv[:])
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	pub, _ := curve25519.X25519(priv[:], curve25519.Basepoint)
	var pub32 [32]byte
	copy(pub32[:], pub)

	bucket := uint64(now.UnixNano() / m.rotationPeriod.Nanoseconds())
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], bucket)
	sum := blake2s.Sum256(append(pub32[:], b[:]...))
	var nid NodeID
	copy(nid[:], sum[:8])

	m.currentPriv = priv
	m.current = Identity{NodeID: nid, PubKey: pub32}
	m.rotatedAt = now
}
