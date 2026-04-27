package profile

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	ErrCorruptStore     = errors.New("profile: corrupt store")
	ErrDecryptionFailed = errors.New("profile: decryption failed")
)

// Store represents an encrypted local store for seed configs and recent events.
// Real implementations would use SQLCipher (SQLite) or `age`.
// This mock uses AES-GCM for encrypted JSON storage on disk.
type Store struct {
	dbPath string
	key    []byte
	mu     sync.RWMutex
}

// BoundedEventBuffer holds the last N events (in-memory or serialized)
type BoundedEventBuffer struct {
	Events []interface{} `json:"events"` // Using interface{} or detector.Event
	Max    int           `json:"max"`
}

// WipeOnSuspicion securely deletes the local store and keys
func (s *Store) WipeOnSuspicion() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Overwrite file with random data before deleting
	if info, err := os.Stat(s.dbPath); err == nil {
		buf := make([]byte, info.Size())
		if _, err := io.ReadFull(rand.Reader, buf); err != nil {
			return err
		}
		if err := os.WriteFile(s.dbPath, buf, 0600); err != nil {
			return err
		}
	}

	// Zero out key in memory
	for i := range s.key {
		s.key[i] = 0
	}

	if err := os.Remove(s.dbPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func ParseMasterKey(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("missing master key")
	}
	if len(s) == 32 {
		return []byte(s), nil
	}

	hexS := s
	if strings.HasPrefix(hexS, "0x") || strings.HasPrefix(hexS, "0X") {
		hexS = hexS[2:]
	}
	if len(hexS) == 64 {
		if b, err := hex.DecodeString(hexS); err == nil && len(b) == 32 {
			return b, nil
		}
	}

	for _, enc := range []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil && len(b) == 32 {
			return b, nil
		}
	}

	return nil, errors.New("invalid master key: expected 32 raw bytes, 64 hex chars, or base64/base64url encoding of 32 bytes")
}

func NewStore(dbPath string, masterKey []byte) (*Store, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("masterKey must be 32 bytes for AES-256")
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		return nil, err
	}

	return &Store{
		dbPath: dbPath,
		key:    append([]byte(nil), masterKey...),
	}, nil
}

// Save encrypts and writes a bundle of profiles to the store
func (s *Store) Save(profiles []Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	plaintext, err := json.Marshal(profiles)
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

	// Write encrypted bundle to disk (no plaintext stored)
	return os.WriteFile(s.dbPath, ciphertext, 0600)
}

// Load reads and decrypts the stored profiles
func (s *Store) Load() ([]Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ciphertext, err := os.ReadFile(s.dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Profile{}, nil // Empty store
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

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	var profiles []Profile
	if err := json.Unmarshal(plaintext, &profiles); err != nil {
		return nil, err
	}

	return profiles, nil
}
