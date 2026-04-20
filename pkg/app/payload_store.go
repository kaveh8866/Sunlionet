package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type PayloadStore struct {
	dir string
	gcm cipher.AEAD
}

func NewPayloadStore(dir string, masterKey []byte) (*PayloadStore, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("app: payload store dir required")
	}
	if len(masterKey) != 32 {
		return nil, errors.New("app: masterKey must be 32 bytes for AES-256")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &PayloadStore{dir: dir, gcm: gcm}, nil
}

func (s *PayloadStore) Put(key string, plaintext []byte) error {
	if s == nil {
		return errors.New("app: payload store is nil")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("app: payload key required")
	}
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ct := s.gcm.Seal(nil, nonce, plaintext, nil)
	buf := append(nonce, ct...)
	p := filepath.Join(s.dir, sanitizeKey(key))
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(p)
		return os.Rename(tmp, p)
	}
	return nil
}

func (s *PayloadStore) Get(key string) ([]byte, bool, error) {
	if s == nil {
		return nil, false, errors.New("app: payload store is nil")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false, errors.New("app: payload key required")
	}
	p := filepath.Join(s.dir, sanitizeKey(key))
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(raw) < s.gcm.NonceSize() {
		return nil, false, errors.New("app: payload corrupted (nonce)")
	}
	nonce := raw[:s.gcm.NonceSize()]
	ct := raw[s.gcm.NonceSize():]
	pt, err := s.gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, false, fmt.Errorf("app: decrypt payload: %w", err)
	}
	return pt, true, nil
}

func (s *PayloadStore) Has(key string) bool {
	if s == nil {
		return false
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	p := filepath.Join(s.dir, sanitizeKey(key))
	_, err := os.Stat(p)
	return err == nil
}

func sanitizeKey(key string) string {
	sum := sha256B64(key)
	return "p_" + sum
}

func sha256B64(s string) string {
	h := sha256Sum([]byte(s))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func sha256Sum(b []byte) [32]byte {
	return sha256.Sum256(b)
}
