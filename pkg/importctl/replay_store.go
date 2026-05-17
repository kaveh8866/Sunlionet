package importctl

import (
	"bytes"
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

const ReplayStateVersion = 2

type ReplayState struct {
	Version   int                          `json:"version"`
	BundleIDs map[string]struct{}          `json:"bundle_ids"`
	Signers   map[string]SignerReplayState `json:"signers"`
}

type SignerReplayState struct {
	MaxSeq        uint64          `json:"max_seq"`
	LastCreatedAt int64           `json:"last_created_at"`
	Nonces        map[string]bool `json:"nonces"`
}

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
	state := ReplayState{
		Version:   ReplayStateVersion,
		BundleIDs: seen,
		Signers:   map[string]SignerReplayState{},
	}
	return s.SaveState(state)
}

func (s *ReplayStore) SaveState(state ReplayState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if state.BundleIDs == nil {
		state.BundleIDs = map[string]struct{}{}
	}
	if state.Signers == nil {
		state.Signers = map[string]SignerReplayState{}
	}
	state.Version = ReplayStateVersion
	for signer, rec := range state.Signers {
		if rec.Nonces == nil {
			rec.Nonces = map[string]bool{}
			state.Signers[signer] = rec
		}
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
	tmp := s.dbPath + ".tmp"
	if err := os.WriteFile(tmp, ciphertext, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.dbPath)
}

func (s *ReplayStore) Load() (map[string]struct{}, error) {
	state, err := s.LoadState()
	if err != nil {
		return nil, err
	}
	return state.BundleIDs, nil
}

func (s *ReplayStore) LoadState() (ReplayState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ciphertext, err := os.ReadFile(s.dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ReplayState{Version: ReplayStateVersion, BundleIDs: map[string]struct{}{}, Signers: map[string]SignerReplayState{}}, nil
		}
		return ReplayState{}, err
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return ReplayState{}, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ReplayState{}, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return ReplayState{}, fmt.Errorf("%w: malformed replay store", profile.ErrCorruptStore)
	}

	nonce, body := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return ReplayState{}, fmt.Errorf("%w: %v", profile.ErrDecryptionFailed, err)
	}

	trimmed := bytes.TrimSpace(plaintext)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var state ReplayState
		if err := json.Unmarshal(plaintext, &state); err != nil {
			return ReplayState{}, err
		}
		if state.BundleIDs == nil {
			state.BundleIDs = map[string]struct{}{}
		}
		if state.Signers == nil {
			state.Signers = map[string]SignerReplayState{}
		}
		state.Version = ReplayStateVersion
		for signer, rec := range state.Signers {
			if rec.Nonces == nil {
				rec.Nonces = map[string]bool{}
				state.Signers[signer] = rec
			}
		}
		return state, nil
	}

	var ids []string
	if err := json.Unmarshal(plaintext, &ids); err != nil {
		return ReplayState{}, err
	}

	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	return ReplayState{Version: ReplayStateVersion, BundleIDs: seen, Signers: map[string]SignerReplayState{}}, nil
}
