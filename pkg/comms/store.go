package comms

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
	"time"
)

// Store keeps communication metadata encrypted on disk (AES-256-GCM).
type Store struct {
	dbPath string
	key    []byte
	mu     sync.RWMutex
}

func NewStore(dbPath string, masterKey []byte) (*Store, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("masterKey must be 32 bytes for AES-256")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, err
	}
	return &Store{
		dbPath: dbPath,
		key:    append([]byte(nil), masterKey...),
	}, nil
}

func (s *Store) Save(state *State) error {
	if state == nil {
		return errors.New("comms: state is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	state.Prune(time.Now())
	if err := state.Validate(); err != nil {
		return err
	}

	plaintext, err := json.Marshal(state)
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
	return os.WriteFile(s.dbPath, ciphertext, 0o600)
}

func (s *Store) Load(defaultRole DeviceRole) (*State, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ciphertext, err := os.ReadFile(s.dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewState(defaultRole), nil
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
		return nil, errors.New("comms: malformed ciphertext")
	}

	nonce, body := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("comms: decryption failed: %w", err)
	}

	var state State
	if err := json.Unmarshal(plaintext, &state); err != nil {
		return nil, err
	}
	if err := state.Validate(); err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *Store) WipeOnSuspicion() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if info, err := os.Stat(s.dbPath); err == nil {
		buf := make([]byte, info.Size())
		_, _ = rand.Read(buf)
		_ = os.WriteFile(s.dbPath, buf, 0o600)
	}
	for i := range s.key {
		s.key[i] = 0
	}
	return os.Remove(s.dbPath)
}
