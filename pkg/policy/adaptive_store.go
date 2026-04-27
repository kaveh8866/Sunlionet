package policy

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrCorruptAdaptiveStore     = errors.New("policy: corrupt adaptive store")
	ErrDecryptionFailedAdaptive = errors.New("policy: adaptive store decryption failed")
)

type AdaptiveStore struct {
	path string
	key  []byte
	mu   sync.RWMutex
}

func NewAdaptiveStore(path string, key []byte) (*AdaptiveStore, error) {
	if len(key) != 32 {
		return nil, errors.New("adaptive store key must be 32 bytes")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return &AdaptiveStore{
		path: path,
		key:  append([]byte(nil), key...),
	}, nil
}

func (s *AdaptiveStore) Save(state *AdaptiveState) error {
	if state == nil {
		return errors.New("nil adaptive state")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot := state.SnapshotDisk()
	plaintext, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return os.WriteFile(s.path, ciphertext, 0o600)
}

func (s *AdaptiveStore) Load() (*AdaptiveState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ciphertext, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewAdaptiveState(defaultAdaptiveWindowSize), nil
		}
		return nil, err
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("%w: malformed ciphertext", ErrCorruptAdaptiveStore)
	}
	nonce, enc := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, enc, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailedAdaptive, err)
	}

	var snapshot AdaptiveStateDisk
	if err := json.Unmarshal(plaintext, &snapshot); err != nil {
		return nil, err
	}
	state := NewAdaptiveState(snapshot.MaxEvents)
	state.ReplaceDisk(snapshot)
	return state, nil
}

func (s *AdaptiveStore) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *AdaptiveStore) WipeOnSuspicion() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.key {
		s.key[i] = 0
	}
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
