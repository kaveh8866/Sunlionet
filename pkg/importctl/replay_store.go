package importctl

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/kaveh/sunlionet-agent/pkg/profile"
)

// ReplayStore persists seen bundle IDs to prevent replay attacks across restarts.
// It uses the same master key as other stores for consistency.
type ReplayStore struct {
	dbPath string
	key    []byte
	mu     sync.Mutex
}

func NewReplayStore(dbPath string, masterKey []byte) (*ReplayStore, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("masterKey must be 32 bytes")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		return nil, err
	}
	return &ReplayStore{
		dbPath: dbPath,
		key:    append([]byte(nil), masterKey...),
	}, nil
}

func (s *ReplayStore) Save(seen map[string]struct{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}

	plaintext, err := json.Marshal(ids)
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
	return os.WriteFile(s.dbPath, ciphertext, 0600)
}

func (s *ReplayStore) Load() (map[string]struct{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ciphertext, err := os.ReadFile(s.dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]struct{}), nil
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
		return nil, fmt.Errorf("%w: malformed replay store", profile.ErrCorruptStore)
	}

	nonce, body := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", profile.ErrDecryptionFailed, err)
	}

	var ids []string
	if err := json.Unmarshal(plaintext, &ids); err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	return seen, nil
}
