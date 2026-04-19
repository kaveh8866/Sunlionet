package relay

import (
	"errors"
	"fmt"
	"time"
)

type MailboxID string

type MessageID string

type Envelope string

type Message struct {
	ID         MessageID `json:"id"`
	Mailbox    MailboxID `json:"mailbox"`
	Envelope   Envelope  `json:"envelope"`
	ReceivedAt int64     `json:"received_at"`
}

func (m Message) Validate() error {
	if m.ID == "" {
		return errors.New("relay: message id required")
	}
	if m.Mailbox == "" {
		return errors.New("relay: mailbox required")
	}
	if m.Envelope == "" {
		return errors.New("relay: envelope required")
	}
	if m.ReceivedAt <= 0 {
		return errors.New("relay: received_at required")
	}
	return nil
}

type PushRequest struct {
	Mailbox  MailboxID `json:"mailbox"`
	Envelope Envelope  `json:"envelope"`
	TTLSec   int       `json:"ttl_sec,omitempty"`
	DelaySec int       `json:"delay_sec,omitempty"`
}

func (r PushRequest) Validate() error {
	if r.Mailbox == "" {
		return errors.New("relay: mailbox required")
	}
	if r.Envelope == "" {
		return errors.New("relay: envelope required")
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
	return nil
}

type PullRequest struct {
	Mailbox MailboxID `json:"mailbox"`
	Limit   int       `json:"limit,omitempty"`
	WaitSec int       `json:"wait_sec,omitempty"`
}

func (r PullRequest) Validate() error {
	if r.Mailbox == "" {
		return errors.New("relay: mailbox required")
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
	if r.Mailbox == "" {
		return errors.New("relay: mailbox required")
	}
	if len(r.IDs) == 0 {
		return errors.New("relay: ids required")
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
