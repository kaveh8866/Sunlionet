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
	"sort"
	"strings"
	"time"
)

type PayloadStore struct {
	dir string
	gcm cipher.AEAD
}

type PruneReport struct {
	Kept          int
	Deleted       int
	BytesDeleted  int64
	SkippedRecent int
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

func (s *PayloadStore) Delete(key string) error {
	if s == nil {
		return errors.New("app: payload store is nil")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("app: payload key required")
	}
	p := filepath.Join(s.dir, sanitizeKey(key))
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *PayloadStore) Prune(now time.Time, keepKeys []string, keepUnreferencedFor time.Duration, maxFiles int, maxBytes int64) (PruneReport, error) {
	if s == nil {
		return PruneReport{}, errors.New("app: payload store is nil")
	}
	if now.IsZero() {
		now = time.Now()
	}

	keepFiles := map[string]struct{}{}
	for i := range keepKeys {
		k := strings.TrimSpace(keepKeys[i])
		if k == "" {
			continue
		}
		keepFiles[sanitizeKey(k)] = struct{}{}
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return PruneReport{}, err
	}

	type item struct {
		name    string
		size    int64
		modTime time.Time
		keep    bool
	}
	items := make([]item, 0, len(entries))
	var totalBytes int64
	totalFiles := 0
	for i := range entries {
		e := entries[i]
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "p_") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		sz := info.Size()
		totalBytes += sz
		totalFiles++
		_, keep := keepFiles[name]
		items = append(items, item{name: name, size: sz, modTime: info.ModTime(), keep: keep})
	}

	rep := PruneReport{Kept: totalFiles}
	cutoff := time.Time{}
	if keepUnreferencedFor > 0 {
		cutoff = now.Add(-keepUnreferencedFor)
	}

	deleteFile := func(it item) {
		p := filepath.Join(s.dir, it.name)
		if err := os.Remove(p); err != nil {
			return
		}
		rep.Deleted++
		rep.BytesDeleted += it.size
		rep.Kept--
		totalFiles--
		totalBytes -= it.size
	}

	for i := range items {
		it := items[i]
		if it.keep {
			continue
		}
		if !cutoff.IsZero() && it.modTime.After(cutoff) {
			rep.SkippedRecent++
			continue
		}
		deleteFile(it)
	}

	if (maxFiles > 0 && totalFiles > maxFiles) || (maxBytes > 0 && totalBytes > maxBytes) {
		cands := make([]item, 0, len(items))
		for i := range items {
			it := items[i]
			if it.keep {
				continue
			}
			p := filepath.Join(s.dir, it.name)
			info, err := os.Stat(p)
			if err != nil {
				continue
			}
			cands = append(cands, item{name: it.name, size: info.Size(), modTime: info.ModTime(), keep: false})
		}
		sort.Slice(cands, func(i, j int) bool {
			if cands[i].modTime.Equal(cands[j].modTime) {
				return cands[i].name < cands[j].name
			}
			return cands[i].modTime.Before(cands[j].modTime)
		})
		for i := range cands {
			if (maxFiles > 0 && totalFiles <= maxFiles) && (maxBytes > 0 && totalBytes <= maxBytes) {
				break
			}
			if (maxFiles > 0 && totalFiles <= maxFiles) && maxBytes <= 0 {
				break
			}
			if (maxBytes > 0 && totalBytes <= maxBytes) && maxFiles <= 0 {
				break
			}
			deleteFile(cands[i])
		}
	}
	return rep, nil
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
