package relay

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

type MailboxID string

type MessageID string

type Envelope string

const (
	maxMailboxLen   = 128
	maxEnvelopeLen  = 256 * 1024
	maxMessageIDLen = 128
	maxAckIDs       = 2000
)

func validateMailboxID(m MailboxID) error {
	if m == "" {
		return errors.New("relay: mailbox required")
	}
	if len(m) > maxMailboxLen {
		return fmt.Errorf("relay: mailbox too large: %d", len(m))
	}
	for i := 0; i < len(m); i++ {
		c := m[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			continue
		}
		return errors.New("relay: mailbox has invalid characters")
	}
	return nil
}

func validateMessageID(id MessageID) error {
	if id == "" {
		return errors.New("relay: message id required")
	}
	if len(id) > maxMessageIDLen {
		return fmt.Errorf("relay: message id too large: %d", len(id))
	}
	b, err := base64.RawURLEncoding.DecodeString(string(id))
	if err != nil || len(b) != 16 {
		return errors.New("relay: invalid message id")
	}
	return nil
}

func validateEnvelope(e Envelope) error {
	if e == "" {
		return errors.New("relay: envelope required")
	}
	if len(e) > maxEnvelopeLen {
		return fmt.Errorf("relay: envelope too large: %d", len(e))
	}
	return nil
}

type Message struct {
	ID         MessageID `json:"id"`
	Mailbox    MailboxID `json:"mailbox"`
	Envelope   Envelope  `json:"envelope"`
	ReceivedAt int64     `json:"received_at"`
}

func (m Message) Validate() error {
	if err := validateMessageID(m.ID); err != nil {
		return err
	}
	if err := validateMailboxID(m.Mailbox); err != nil {
		return err
	}
	if err := validateEnvelope(m.Envelope); err != nil {
		return err
	}
	if m.ReceivedAt <= 0 {
		return errors.New("relay: received_at required")
	}
	return nil
}

type PushRequest struct {
	Mailbox        MailboxID `json:"mailbox"`
	Envelope       Envelope  `json:"envelope"`
	TTLSec         int       `json:"ttl_sec,omitempty"`
	DelaySec       int       `json:"delay_sec,omitempty"`
	PoWBits        int       `json:"pow_bits,omitempty"`
	PoWNonceB64URL string    `json:"pow_nonce_b64url,omitempty"`
}

func (r PushRequest) Validate() error {
	if err := validateMailboxID(r.Mailbox); err != nil {
		return err
	}
	if err := validateEnvelope(r.Envelope); err != nil {
		return err
	}
	if r.TTLSec < 0 {
		return errors.New("relay: ttl_sec must be >= 0")
	}
	if r.DelaySec < 0 {
		return errors.New("relay: delay_sec must be >= 0")
	}
	if r.DelaySec > 3600 {
		return fmt.Errorf("relay: delay_sec too large: %d", r.DelaySec)
	}
	if r.PoWBits < 0 {
		return errors.New("relay: pow_bits must be >= 0")
	}
	if r.PoWBits > 28 {
		return fmt.Errorf("relay: pow_bits too large: %d", r.PoWBits)
	}
	if r.PoWBits > 0 {
		if r.PoWNonceB64URL == "" {
			return errors.New("relay: pow_nonce_b64url required when pow_bits > 0")
		}
		nonce, err := base64.RawURLEncoding.DecodeString(r.PoWNonceB64URL)
		if err != nil {
			return errors.New("relay: invalid pow_nonce_b64url")
		}
		if len(nonce) < 8 || len(nonce) > 32 {
			return errors.New("relay: invalid pow nonce length")
		}
	}
	return nil
}

type PullRequest struct {
	Mailbox MailboxID `json:"mailbox"`
	Limit   int       `json:"limit,omitempty"`
	WaitSec int       `json:"wait_sec,omitempty"`
}

func (r PullRequest) Validate() error {
	if err := validateMailboxID(r.Mailbox); err != nil {
		return err
	}
	if r.WaitSec < 0 {
		return errors.New("relay: wait_sec must be >= 0")
	}
	if r.WaitSec > 30 {
		return fmt.Errorf("relay: wait_sec too large: %d", r.WaitSec)
	}
	return nil
}

func (r PullRequest) Normalize() PullRequest {
	out := r
	if out.Limit <= 0 {
		out.Limit = 50
	}
	if out.Limit > 200 {
		out.Limit = 200
	}
	if out.WaitSec > 30 {
		out.WaitSec = 30
	}
	return out
}

type AckRequest struct {
	Mailbox MailboxID   `json:"mailbox"`
	IDs     []MessageID `json:"ids"`
}

func (r AckRequest) Validate() error {
	if err := validateMailboxID(r.Mailbox); err != nil {
		return err
	}
	if len(r.IDs) == 0 {
		return errors.New("relay: ids required")
	}
	if len(r.IDs) > maxAckIDs {
		return fmt.Errorf("relay: too many ids: %d", len(r.IDs))
	}
	for i := range r.IDs {
		if err := validateMessageID(r.IDs[i]); err != nil {
			return err
		}
	}
	return nil
}

func TTLDeadline(now time.Time, ttlSec int) (time.Time, error) {
	if ttlSec < 0 {
		return time.Time{}, errors.New("relay: ttl_sec must be >= 0")
	}
	if ttlSec == 0 {
		return time.Time{}, nil
	}
	if ttlSec > 7*24*3600 {
		return time.Time{}, fmt.Errorf("relay: ttl_sec too large: %d", ttlSec)
	}
	return now.Add(time.Duration(ttlSec) * time.Second), nil
}

func VerifyPoW(mailbox MailboxID, envelope Envelope, nonceB64URL string, bits int) error {
	if bits <= 0 {
		return nil
	}
	if bits > 28 {
		return fmt.Errorf("relay: pow_bits too large: %d", bits)
	}
	if err := validateMailboxID(mailbox); err != nil {
		return err
	}
	if err := validateEnvelope(envelope); err != nil {
		return err
	}
	if nonceB64URL == "" {
		return errors.New("relay: pow_nonce_b64url required")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(nonceB64URL)
	if err != nil {
		return errors.New("relay: invalid pow_nonce_b64url")
	}
	if len(nonce) < 8 || len(nonce) > 32 {
		return errors.New("relay: invalid pow nonce length")
	}

	h := sha256.New()
	_, _ = h.Write([]byte(mailbox))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(envelope))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(nonce)
	sum := h.Sum(nil)
	if !hashHasLeadingZeroBits(sum, bits) {
		return errors.New("relay: invalid proof of work")
	}
	return nil
}

func hashHasLeadingZeroBits(sum []byte, bits int) bool {
	fullBytes := bits / 8
	remBits := bits % 8
	for i := 0; i < fullBytes; i++ {
		if sum[i] != 0 {
			return false
		}
	}
	if remBits == 0 {
		return true
	}
	mask := byte(0xFF) << (8 - remBits)
	return (sum[fullBytes] & mask) == 0
}
