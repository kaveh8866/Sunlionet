package ledger

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
		return nil, errors.New("ledger: masterKey must be 32 bytes for AES-256")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, err
	}
	return &Store{
		dbPath: dbPath,
		key:    append([]byte(nil), masterKey...),
	}, nil
}

func (s *Store) Load() (Snapshot, error) {
	if s == nil {
		return Snapshot{}, errors.New("ledger: store is nil")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	raw, err := os.ReadFile(s.dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{SchemaVersion: SchemaV1}, nil
		}
		return Snapshot{}, err
	}
	if len(raw) == 0 {
		return Snapshot{SchemaVersion: SchemaV1}, nil
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return Snapshot{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Snapshot{}, err
	}
	if len(raw) < gcm.NonceSize() {
		return Snapshot{}, errors.New("ledger: corrupted store (nonce)")
	}
	nonce := raw[:gcm.NonceSize()]
	ct := raw[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("ledger: decrypt store: %w", err)
	}
	var snap Snapshot
	if err := json.Unmarshal(pt, &snap); err != nil {
		return Snapshot{}, err
	}
	if snap.SchemaVersion == 0 {
		snap.SchemaVersion = SchemaV1
	}
	if snap.SchemaVersion != SchemaV1 {
		return Snapshot{}, fmt.Errorf("ledger: unsupported snapshot schema: %d", snap.SchemaVersion)
	}
	return snap, nil
}

func (s *Store) Save(snap Snapshot) error {
	if s == nil {
		return errors.New("ledger: store is nil")
	}
	if snap.SchemaVersion == 0 {
		snap.SchemaVersion = SchemaV1
	}
	if snap.SchemaVersion != SchemaV1 {
		return fmt.Errorf("ledger: unsupported snapshot schema: %d", snap.SchemaVersion)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	plaintext, err := json.Marshal(snap)
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
