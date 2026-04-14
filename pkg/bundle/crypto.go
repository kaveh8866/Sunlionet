package bundle

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"filippo.io/age"
)

var (
	ErrInvalidSignature = errors.New("invalid signature")
	ErrExpired          = errors.New("bundle expired")
	ErrDecryptionFailed = errors.New("decryption failed")
	ErrUnknownCipher    = errors.New("unknown cipher")
)

// GenerateBundle serializes, encrypts with age, and signs a BundlePayload.
func GenerateBundle(payload *BundlePayload, recipientPublicKey string, signerPrivateKey ed25519.PrivateKey, signerKeyID string) ([]byte, error) {
	// 1. Serialize payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize payload: %w", err)
	}

	// 2. Encrypt with age
	recipient, err := age.ParseX25519Recipient(recipientPublicKey)
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
	ciphertext := ciphertextBuf.Bytes()

	// 3. Create header
	now := time.Now().Unix()
	header := BundleHeader{
		Magic:          "SNB1",
		BundleID:       fmt.Sprintf("bndl_%d", now),
		PublisherKeyID: signerKeyID,
		RecipientKeyID: "default",
		Seq:            1,
		CreatedAt:      now,
		ExpiresAt:      now + (7 * 24 * 3600), // 7 days expiry
		Cipher:         "age-x25519",
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize header: %w", err)
	}

	// 4. Sign (header + ciphertext)
	var sigInput bytes.Buffer
	sigInput.Write(headerBytes)
	sigInput.Write(ciphertext)

	signature := ed25519.Sign(signerPrivateKey, sigInput.Bytes())
	header.Signature = base64.RawURLEncoding.EncodeToString(signature)

	// 5. Final wrapper
	wrapper := struct {
		Header     BundleHeader `json:"header"`
		Ciphertext string       `json:"ciphertext"` // base64 encoded
	}{
		Header:     header,
		Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
	}

	return json.Marshal(wrapper)
}

// VerifyAndDecrypt parses, verifies signature, and decrypts the bundle.
func VerifyAndDecrypt(bundleBytes []byte, trustedSigner ed25519.PublicKey, ageIdentity *age.X25519Identity) (*BundlePayload, error) {
	// 1. Parse wrapper
	var wrapper struct {
		Header     BundleHeader `json:"header"`
		Ciphertext string       `json:"ciphertext"`
	}
	if err := json.Unmarshal(bundleBytes, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse wrapper: %w", err)
	}

	if wrapper.Header.Cipher != "age-x25519" {
		return nil, ErrUnknownCipher
	}

	if time.Now().Unix() > wrapper.Header.ExpiresAt {
		return nil, ErrExpired
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(wrapper.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("invalid ciphertext base64: %w", err)
	}

	// 2. Verify Signature
	sigBytes, err := base64.RawURLEncoding.DecodeString(wrapper.Header.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature base64: %w", err)
	}

	// Reconstruct header without signature for verification
	headerCopy := wrapper.Header
	headerCopy.Signature = ""
	headerBytes, err := json.Marshal(headerCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize header for verification: %w", err)
	}

	var sigInput bytes.Buffer
	sigInput.Write(headerBytes)
	sigInput.Write(ciphertext)

	if !ed25519.Verify(trustedSigner, sigInput.Bytes(), sigBytes) {
		return nil, ErrInvalidSignature
	}

	// 3. Decrypt with age
	decReader, err := age.Decrypt(bytes.NewReader(ciphertext), ageIdentity)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	payloadBytes, err := io.ReadAll(decReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted payload: %w", err)
	}

	// 4. Parse Payload
	var payload BundlePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse decrypted payload: %w", err)
	}

	return &payload, nil
}
