package chat

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

var (
	ErrCorruptStore     = errors.New("chat: corrupt store")
	ErrDecryptionFailed = errors.New("chat: decryption failed")
)

type Store struct {
	dbPath string
	key    []byte
	mu     sync.RWMutex
}

func NewStore(dbPath string, masterKey []byte) (*Store, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("chat: masterKey must be 32 bytes for AES-256")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, err
	}
	return &Store{
		dbPath: dbPath,
		key:    append([]byte(nil), masterKey...),
	}, nil
}

func (s *Store) Load() (*State, error) {
	if s == nil {
		return nil, errors.New("chat: store is nil")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	ciphertext, err := os.ReadFile(s.dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewState(), nil
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
		return nil, fmt.Errorf("%w: malformed ciphertext", ErrCorruptStore)
	}
	nonce, body := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}
	var st State
	if err := json.Unmarshal(plaintext, &st); err != nil {
		return nil, err
	}
	st.Prune(time.Now())
	if err := st.Validate(); err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *Store) Save(st *State) error {
	if s == nil {
		return errors.New("chat: store is nil")
	}
	if st == nil {
		return errors.New("chat: state is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	st.Prune(now)
	if err := st.Validate(); err != nil {
		return err
	}
	plaintext, err := json.Marshal(st)
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
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return os.WriteFile(s.dbPath, ciphertext, 0o600)
}

func (s *Store) WipeOnSuspicion() error {
	if s == nil {
		return errors.New("chat: store is nil")
	}
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
	if err := os.Remove(s.dbPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
