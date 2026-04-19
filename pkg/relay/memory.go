package relay

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"sort"
	"sync"
	"time"
)

type MemoryRelay struct {
	mu       sync.Mutex
	mailboxQ map[MailboxID][]stored
}

type stored struct {
	msg      Message
	deadline time.Time
	availAt  time.Time
}

func NewMemoryRelay() *MemoryRelay {
	return &MemoryRelay{mailboxQ: make(map[MailboxID][]stored)}
}

func (r *MemoryRelay) Push(ctx context.Context, req PushRequest) (MessageID, error) {
	_ = ctx
	if err := req.Validate(); err != nil {
		return "", err
	}
	now := time.Now()
	deadline, err := TTLDeadline(now, req.TTLSec)
	if err != nil {
		return "", err
	}
	availAt := now
	if req.DelaySec > 0 {
		availAt = now.Add(time.Duration(req.DelaySec) * time.Second)
	}
	id, err := newMessageID()
	if err != nil {
		return "", err
	}
	msg := Message{
		ID:         id,
		Mailbox:    req.Mailbox,
		Envelope:   req.Envelope,
		ReceivedAt: now.Unix(),
	}
	if err := msg.Validate(); err != nil {
		return "", err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(now)
	r.mailboxQ[req.Mailbox] = append(r.mailboxQ[req.Mailbox], stored{msg: msg, deadline: deadline, availAt: availAt})
	return id, nil
}

func (r *MemoryRelay) Pull(ctx context.Context, req PullRequest) ([]Message, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req = req.Normalize()
	if ctx == nil {
		ctx = context.Background()
	}

	waitUntil := time.Time{}
	if req.WaitSec > 0 {
		waitUntil = time.Now().Add(time.Duration(req.WaitSec) * time.Second)
	}
	for {
		now := time.Now()

		r.mu.Lock()
		r.pruneLocked(now)
		out := r.pullAvailableLocked(req.Mailbox, now, req.Limit)
		r.mu.Unlock()

		if len(out) > 0 || req.WaitSec <= 0 || (!waitUntil.IsZero() && now.After(waitUntil)) {
			return out, nil
		}

		sleepFor := 50 * time.Millisecond
		if !waitUntil.IsZero() {
			rem := time.Until(waitUntil)
			if rem <= 0 {
				return []Message{}, nil
			}
			if rem < sleepFor {
				sleepFor = rem
			}
		}

		timer := time.NewTimer(sleepFor)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (r *MemoryRelay) pullAvailableLocked(mailbox MailboxID, now time.Time, limit int) []Message {
	all := r.mailboxQ[mailbox]
	if len(all) == 0 {
		return []Message{}
	}
	q := make([]stored, 0, len(all))
	for i := range all {
		if !all[i].availAt.IsZero() && now.Before(all[i].availAt) {
			continue
		}
		q = append(q, all[i])
	}
	if len(q) == 0 {
		return []Message{}
	}
	sort.Slice(q, func(i, j int) bool { return q[i].msg.ReceivedAt < q[j].msg.ReceivedAt })
	if len(q) > limit {
		q = q[:limit]
	}
	out := make([]Message, 0, len(q))
	for i := range q {
		out = append(out, q[i].msg)
	}
	return out
}

func (r *MemoryRelay) Ack(ctx context.Context, req AckRequest) error {
	_ = ctx
	if err := req.Validate(); err != nil {
		return err
	}
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(now)

	q := r.mailboxQ[req.Mailbox]
	if len(q) == 0 {
		return nil
	}
	want := make(map[MessageID]struct{}, len(req.IDs))
	for i := range req.IDs {
		want[req.IDs[i]] = struct{}{}
	}
	kept := q[:0]
	for i := range q {
		if _, ok := want[q[i].msg.ID]; ok {
			continue
		}
		kept = append(kept, q[i])
	}
	r.mailboxQ[req.Mailbox] = kept
	return nil
}

func (r *MemoryRelay) pruneLocked(now time.Time) {
	for mb, q := range r.mailboxQ {
		kept := q[:0]
		for i := range q {
			if !q[i].deadline.IsZero() && now.After(q[i].deadline) {
				continue
			}
			kept = append(kept, q[i])
		}
		if len(kept) == 0 {
			delete(r.mailboxQ, mb)
			continue
		}
		r.mailboxQ[mb] = kept
	}
}

func newMessageID() (MessageID, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return MessageID(base64.RawURLEncoding.EncodeToString(b[:])), nil
}
