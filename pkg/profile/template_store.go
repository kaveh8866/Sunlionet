package profile

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

type TemplateStore struct {
	dbPath string
	key    []byte
	mu     sync.RWMutex
}

func NewTemplateStore(dbPath string, masterKey []byte) (*TemplateStore, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("masterKey must be 32 bytes for AES-256")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, err
	}
	return &TemplateStore{
		dbPath: dbPath,
		key:    append([]byte(nil), masterKey...),
	}, nil
}

func (s *TemplateStore) Save(templates map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if templates == nil {
		templates = map[string]string{}
	}
	plaintext, err := json.Marshal(templates)
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

func (s *TemplateStore) Load() (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ciphertext, err := os.ReadFile(s.dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
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
		return nil, errors.New("malformed ciphertext")
	}
	nonce, body := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (wrong key or corrupted data): %w", err)
	}
	var templates map[string]string
	if err := json.Unmarshal(plaintext, &templates); err != nil {
		return nil, err
	}
	if templates == nil {
		templates = map[string]string{}
	}
	return templates, nil
}
