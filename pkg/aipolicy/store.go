package aipolicy

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

type Store struct {
	dbPath string
	key    []byte
	mu     sync.RWMutex
}

func NewStore(dbPath string, masterKey []byte) (*Store, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("aipolicy: masterKey must be 32 bytes for AES-256")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		return nil, err
	}
	return &Store{
		dbPath: dbPath,
		key:    append([]byte(nil), masterKey...),
	}, nil
}

func (s *Store) Save(p *Policy) error {
	if s == nil {
		return errors.New("aipolicy: store is nil")
	}
	if p == nil {
		return errors.New("aipolicy: policy is nil")
	}
	if err := p.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	plaintext, err := json.Marshal(p)
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
	return os.WriteFile(s.dbPath, ciphertext, 0600)
}

func (s *Store) Load() (*Policy, error) {
	if s == nil {
		return nil, errors.New("aipolicy: store is nil")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	ciphertext, err := os.ReadFile(s.dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Policy{Grants: []Grant{}}, nil
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
		return nil, errors.New("aipolicy: malformed ciphertext")
	}
	nonce, c := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, c, nil)
	if err != nil {
		return nil, fmt.Errorf("aipolicy: decryption failed: %w", err)
	}
	var p Policy
	if err := json.Unmarshal(plaintext, &p); err != nil {
		return nil, err
	}
	if p.Grants == nil {
		p.Grants = []Grant{}
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) WipeOnSuspicion() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.key {
		s.key[i] = 0
	}
	if err := os.Remove(s.dbPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
