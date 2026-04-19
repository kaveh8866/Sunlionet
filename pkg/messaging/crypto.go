package messaging

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

type X25519Keypair struct {
	Public  [32]byte
	Private [32]byte
}

func NewX25519Keypair() (*X25519Keypair, error) {
	var priv [32]byte
	if _, err := io.ReadFull(rand.Reader, priv[:]); err != nil {
		return nil, err
	}
	pubBytes, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return nil, err
	}
	var pub [32]byte
	copy(pub[:], pubBytes)
	return &X25519Keypair{Public: pub, Private: priv}, nil
}

func DeriveSharedKey(senderPriv [32]byte, recipientPub [32]byte, info string) ([32]byte, error) {
	shared, err := curve25519.X25519(senderPriv[:], recipientPub[:])
	if err != nil {
		return [32]byte{}, err
	}
	r := hkdf.New(sha256.New, shared, nil, []byte(info))
	var out [32]byte
	if _, err := io.ReadFull(r, out[:]); err != nil {
		return [32]byte{}, err
	}
	return out, nil
}

func NewAEAD(key [32]byte) (cipher.AEAD, error) {
	return chacha20poly1305.NewX(key[:])
}

func RandomNonce24() ([24]byte, error) {
	var n [24]byte
	if _, err := io.ReadFull(rand.Reader, n[:]); err != nil {
		return [24]byte{}, err
	}
	return n, nil
}

func Encrypt(aead cipher.AEAD, nonce [24]byte, plaintext []byte, aad []byte) ([]byte, error) {
	if aead == nil {
		return nil, errors.New("messaging: aead is nil")
	}
	return aead.Seal(nil, nonce[:], plaintext, aad), nil
}

func Decrypt(aead cipher.AEAD, nonce [24]byte, ciphertext []byte, aad []byte) ([]byte, error) {
	if aead == nil {
		return nil, errors.New("messaging: aead is nil")
	}
	plaintext, err := aead.Open(nil, nonce[:], ciphertext, aad)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
