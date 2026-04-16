package outsidectl

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"filippo.io/age"
)

func LoadEd25519PrivateKeyFile(path string) (ed25519.PrivateKey, ed25519.PublicKey, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, "", err
	}
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return nil, nil, "", fmt.Errorf("empty signing key")
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, nil, "", fmt.Errorf("invalid signing key (base64url): %w", err)
	}

	var priv ed25519.PrivateKey
	switch len(b) {
	case ed25519.SeedSize:
		priv = ed25519.NewKeyFromSeed(b)
	case ed25519.PrivateKeySize:
		priv = ed25519.PrivateKey(b)
	default:
		return nil, nil, "", fmt.Errorf("invalid signing key size: %d", len(b))
	}
	pub := priv.Public().(ed25519.PublicKey)
	keyID := "ed25519:" + fingerprint16Bytes(pub)
	return priv, pub, keyID, nil
}

func LoadEd25519PublicKeyFile(path string) (ed25519.PublicKey, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return nil, "", fmt.Errorf("empty signer public key")
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, "", fmt.Errorf("invalid signer public key (base64url): %w", err)
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, "", fmt.Errorf("invalid signer public key size: %d", len(b))
	}
	pub := ed25519.PublicKey(b)
	keyID := "ed25519:" + fingerprint16Bytes(pub)
	return pub, keyID, nil
}

func LoadAgeRecipientFile(path string) (string, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return "", "", fmt.Errorf("empty recipient public key")
	}
	if _, err := age.ParseX25519Recipient(s); err != nil {
		return "", "", fmt.Errorf("invalid recipient public key: %w", err)
	}
	return s, "age-x25519:" + fingerprint16String(s), nil
}

func LoadAgeIdentityFile(path string) (*age.X25519Identity, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return nil, "", fmt.Errorf("empty age identity")
	}
	id, err := age.ParseX25519Identity(s)
	if err != nil {
		return nil, "", fmt.Errorf("invalid age identity: %w", err)
	}
	return id, "age-x25519:" + fingerprint16String(id.Recipient().String()), nil
}

func Fingerprint16(b []byte) string {
	return fingerprint16Bytes(b)
}

func Fingerprint16String(s string) string {
	return fingerprint16String(s)
}

func fingerprint16Bytes(b []byte) string {
	sum := sha256.Sum256(b)
	return base64.RawURLEncoding.EncodeToString(sum[:])[:16]
}

func fingerprint16String(s string) string {
	sum := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(sum[:])[:16]
}
