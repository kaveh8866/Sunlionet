package bundle

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"filippo.io/age"
)

var (
	ErrInvalidSignature = errors.New("invalid signature")
	ErrExpired          = errors.New("bundle expired")
	ErrDecryptionFailed = errors.New("decryption failed")
	ErrUnknownCipher    = errors.New("unknown cipher")
)

const (
	BundleSignatureDomain = "SUNLIONET-BUNDLE-V2\x00"
	BundleNonceSize       = 24
	MaxBundleBytes        = 2 << 20
	MaxPayloadBytes       = 1 << 20
	MaxCiphertextBytes    = 2 << 20
	MaxBundleTTLSeconds   = 14 * 24 * 3600
	MaxClockSkewSeconds   = 10 * 60
)

type GenerateOptions struct {
	RecipientPublicKey string
	AllowPlaintext     bool
	SignerKeyID        string
	BundleID           string
	Seq                uint64
	Nonce              string
	CreatedAt          int64
	ExpiresAt          int64
}

// GenerateBundle serializes, encrypts with age, and signs a BundlePayload.
func GenerateBundle(payload *BundlePayload, recipientPublicKey string, signerPrivateKey ed25519.PrivateKey, signerKeyID string) ([]byte, error) {
	return GenerateBundleWithAllowPlaintext(payload, recipientPublicKey, signerPrivateKey, signerKeyID, false)
}

func GenerateBundleWithAllowPlaintext(payload *BundlePayload, recipientPublicKey string, signerPrivateKey ed25519.PrivateKey, signerKeyID string, allowPlaintext bool) ([]byte, error) {
	now := time.Now().Unix()
	opts := GenerateOptions{
		RecipientPublicKey: recipientPublicKey,
		AllowPlaintext:     allowPlaintext,
		SignerKeyID:        signerKeyID,
		BundleID:           fmt.Sprintf("bndl_%d", now),
		Seq:                1,
		CreatedAt:          now,
		ExpiresAt:          now + (7 * 24 * 3600),
	}
	return GenerateBundleWithOptions(payload, signerPrivateKey, opts)
}

func GenerateBundleWithOptions(payload *BundlePayload, signerPrivateKey ed25519.PrivateKey, opts GenerateOptions) ([]byte, error) {
	payloadBytes, err := MarshalCanonicalPayload(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize payload: %w", err)
	}

	if strings.TrimSpace(opts.BundleID) == "" {
		opts.BundleID = fmt.Sprintf("bndl_%d", time.Now().Unix())
	}
	if opts.Seq == 0 {
		opts.Seq = 1
	}
	if opts.CreatedAt == 0 {
		opts.CreatedAt = time.Now().Unix()
	}
	if opts.ExpiresAt == 0 {
		opts.ExpiresAt = opts.CreatedAt + (7 * 24 * 3600)
	}
	if opts.Nonce == "" {
		nonce := make([]byte, BundleNonceSize)
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			return nil, fmt.Errorf("failed to generate bundle nonce: %w", err)
		}
		opts.Nonce = base64.RawURLEncoding.EncodeToString(nonce)
	}
	if err := validateSigningKey(signerPrivateKey, opts.SignerKeyID); err != nil {
		return nil, err
	}

	var ciphertext []byte
	var cipher string
	var recipientKeyID string
	if strings.TrimSpace(opts.RecipientPublicKey) != "" {
		recipient, err := age.ParseX25519Recipient(opts.RecipientPublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse recipient public key: %w", err)
		}

		var ciphertextBuf bytes.Buffer
		encWriter, err := age.Encrypt(&ciphertextBuf, recipient)
		if err != nil {
			return nil, fmt.Errorf("failed to create age encryptor: %w", err)
		}
		if _, err := encWriter.Write(payloadBytes); err != nil {
			return nil, fmt.Errorf("failed to encrypt payload: %w", err)
		}
		if err := encWriter.Close(); err != nil {
			return nil, fmt.Errorf("failed to close age encryptor: %w", err)
		}
		ciphertext = ciphertextBuf.Bytes()
		cipher = "age-x25519"
		recipientKeyID = "age-x25519:" + fingerprint16(opts.RecipientPublicKey)
	} else {
		if !opts.AllowPlaintext {
			return nil, fmt.Errorf("refusing to generate unencrypted bundle without explicit allow flag")
		}
		ciphertext = payloadBytes
		cipher = "none"
		recipientKeyID = "none"
	}

	header := BundleHeader{
		Magic:          "SNB1",
		BundleID:       opts.BundleID,
		PublisherKeyID: opts.SignerKeyID,
		RecipientKeyID: recipientKeyID,
		Seq:            opts.Seq,
		Nonce:          opts.Nonce,
		CreatedAt:      opts.CreatedAt,
		ExpiresAt:      opts.ExpiresAt,
		Cipher:         cipher,
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize header: %w", err)
	}

	signature := ed25519.Sign(signerPrivateKey, signatureInput(headerBytes, ciphertext))
	header.Signature = base64.RawURLEncoding.EncodeToString(signature)

	wrapper := struct {
		Header     BundleHeader `json:"header"`
		Ciphertext string       `json:"ciphertext"`
	}{
		Header:     header,
		Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
	}

	return json.Marshal(wrapper)
}

func validateSigningKey(priv ed25519.PrivateKey, signerKeyID string) error {
	if len(priv) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid signer private key size: %d", len(priv))
	}
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok || len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid signer public key")
	}
	if strings.TrimSpace(signerKeyID) == "" {
		return fmt.Errorf("missing signer key id")
	}
	if signerKeyID != Ed25519KeyID(pub) {
		return fmt.Errorf("signer key id does not match private key")
	}
	return nil
}

func signatureInput(headerBytes, ciphertext []byte) []byte {
	var sigInput bytes.Buffer
	sigInput.Grow(len(BundleSignatureDomain) + len(headerBytes) + 1 + len(ciphertext))
	sigInput.WriteString(BundleSignatureDomain)
	sigInput.Write(headerBytes)
	sigInput.WriteByte(0)
	sigInput.Write(ciphertext)
	return sigInput.Bytes()
}

// VerifyAndDecrypt parses, verifies signature, and decrypts the bundle.
func VerifyAndDecrypt(bundleBytes []byte, trustedSigner ed25519.PublicKey, ageIdentity *age.X25519Identity) (*BundlePayload, error) {
	// 1. Parse wrapper
	var wrapper struct {
		Header     BundleHeader `json:"header"`
		Ciphertext string       `json:"ciphertext"`
	}
	if len(bundleBytes) > MaxBundleBytes {
		return nil, fmt.Errorf("bundle too large")
	}
	if err := json.Unmarshal(bundleBytes, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse wrapper: %w", err)
	}

	if wrapper.Header.Cipher != "age-x25519" && wrapper.Header.Cipher != "none" {
		return nil, ErrUnknownCipher
	}

	now := time.Now().Unix()
	if err := validateHeaderBasics(wrapper.Header, now, Ed25519KeyID(trustedSigner)); err != nil {
		return nil, err
	}
	if time.Now().Unix() > wrapper.Header.ExpiresAt {
		return nil, ErrExpired
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(wrapper.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("invalid ciphertext base64: %w", err)
	}
	if len(ciphertext) > MaxCiphertextBytes {
		return nil, fmt.Errorf("ciphertext too large")
	}

	// 2. Verify Signature
	sigBytes, err := base64.RawURLEncoding.DecodeString(wrapper.Header.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature base64: %w", err)
	}
	if len(trustedSigner) != ed25519.PublicKeySize || len(sigBytes) != ed25519.SignatureSize {
		return nil, ErrInvalidSignature
	}

	// Reconstruct header without signature for verification
	headerCopy := wrapper.Header
	headerCopy.Signature = ""
	headerBytes, err := json.Marshal(headerCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize header for verification: %w", err)
	}

	if !ed25519.Verify(trustedSigner, signatureInput(headerBytes, ciphertext), sigBytes) {
		return nil, ErrInvalidSignature
	}

	var payloadBytes []byte
	if wrapper.Header.Cipher == "age-x25519" {
		if ageIdentity == nil {
			return nil, fmt.Errorf("%w: missing age identity", ErrDecryptionFailed)
		}
		decReader, err := age.Decrypt(bytes.NewReader(ciphertext), ageIdentity)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
		}

		payloadBytes, err = io.ReadAll(io.LimitReader(decReader, MaxPayloadBytes+1))
		if err != nil {
			return nil, fmt.Errorf("failed to read decrypted payload: %w", err)
		}
		if len(payloadBytes) > MaxPayloadBytes {
			return nil, fmt.Errorf("decrypted payload too large")
		}
	} else {
		payloadBytes = ciphertext
	}

	// 4. Parse Payload
	var payload BundlePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse decrypted payload: %w", err)
	}

	return &payload, nil
}

func VerifyHeader(bundleBytes []byte, trustedSigner ed25519.PublicKey) (*BundleHeader, error) {
	var wrapper struct {
		Header     BundleHeader `json:"header"`
		Ciphertext string       `json:"ciphertext"`
	}
	if len(bundleBytes) > MaxBundleBytes {
		return nil, fmt.Errorf("bundle too large")
	}
	if err := json.Unmarshal(bundleBytes, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse wrapper: %w", err)
	}

	if err := validateHeaderBasics(wrapper.Header, time.Now().Unix(), Ed25519KeyID(trustedSigner)); err != nil {
		return nil, err
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(wrapper.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("invalid ciphertext base64: %w", err)
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(wrapper.Header.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature base64: %w", err)
	}
	if len(trustedSigner) != ed25519.PublicKeySize || len(sigBytes) != ed25519.SignatureSize {
		return nil, ErrInvalidSignature
	}

	headerCopy := wrapper.Header
	headerCopy.Signature = ""
	headerBytes, err := json.Marshal(headerCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize header for verification: %w", err)
	}

	if !ed25519.Verify(trustedSigner, signatureInput(headerBytes, ciphertext), sigBytes) {
		return nil, ErrInvalidSignature
	}

	return &wrapper.Header, nil
}

func validateHeaderBasics(header BundleHeader, now int64, expectedSignerKeyID string) error {
	if header.Cipher != "age-x25519" && header.Cipher != "none" {
		return ErrUnknownCipher
	}
	if strings.TrimSpace(header.PublisherKeyID) == "" || header.PublisherKeyID != expectedSignerKeyID {
		return ErrInvalidSignature
	}
	if header.CreatedAt <= 0 || header.ExpiresAt <= 0 || header.ExpiresAt < header.CreatedAt {
		return fmt.Errorf("invalid bundle time window")
	}
	if header.ExpiresAt-header.CreatedAt > MaxBundleTTLSeconds {
		return fmt.Errorf("bundle ttl too large")
	}
	if header.CreatedAt > now+MaxClockSkewSeconds {
		return fmt.Errorf("bundle created_at too far in the future")
	}
	if now > header.ExpiresAt {
		return ErrExpired
	}
	if _, err := decodeBundleNonce(header.Nonce); err != nil {
		return err
	}
	return nil
}

func decodeBundleNonce(s string) ([]byte, error) {
	if strings.TrimSpace(s) == "" {
		return nil, fmt.Errorf("missing bundle nonce")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid bundle nonce")
	}
	if len(nonce) != BundleNonceSize {
		return nil, fmt.Errorf("invalid bundle nonce size")
	}
	return nonce, nil
}

func fingerprint16(s string) string {
	sum := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(sum[:])[:16]
}
