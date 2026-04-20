package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type FileRelayOptions struct {
	MaxPendingPerMailbox int
	MaxTotalPending      int
}

type FileRelay struct {
	baseDir string
	opts    FileRelayOptions

	mu sync.Mutex
}

type fileStored struct {
	Message   Message `json:"message"`
	Deadline  int64   `json:"deadline_unix,omitempty"`
	AvailAt   int64   `json:"avail_at_unix,omitempty"`
	CreatedAt int64   `json:"created_at_unix"`
}

func NewFileRelay(baseDir string, opts FileRelayOptions) (*FileRelay, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return nil, errors.New("relay: baseDir is empty")
	}
	if opts.MaxPendingPerMailbox < 0 {
		return nil, errors.New("relay: MaxPendingPerMailbox must be >= 0")
	}
	if opts.MaxTotalPending < 0 {
		return nil, errors.New("relay: MaxTotalPending must be >= 0")
	}
	if opts.MaxPendingPerMailbox == 0 {
		opts.MaxPendingPerMailbox = 10_000
	}
	if opts.MaxTotalPending == 0 {
		opts.MaxTotalPending = 250_000
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, err
	}
	return &FileRelay{baseDir: baseDir, opts: opts}, nil
}

func (r *FileRelay) Push(ctx context.Context, req PushRequest) (MessageID, error) {
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

	if err := r.pruneLocked(now); err != nil {
		return "", err
	}
	total, err := r.totalPendingLocked()
	if err != nil {
		return "", err
	}
	if total >= r.opts.MaxTotalPending {
		return "", errors.New("relay: relay storage full")
	}
	mbCount, err := r.mailboxPendingLocked(req.Mailbox)
	if err != nil {
		return "", err
	}
	if mbCount >= r.opts.MaxPendingPerMailbox {
		return "", errors.New("relay: mailbox quota exceeded")
	}

	rec := fileStored{
		Message:   msg,
		CreatedAt: now.Unix(),
	}
	if !deadline.IsZero() {
		rec.Deadline = deadline.Unix()
	}
	if !availAt.IsZero() && availAt.After(now) {
		rec.AvailAt = availAt.Unix()
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return "", err
	}
	dir := r.mailboxDir(req.Mailbox)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, fmt.Sprintf("msg_%s.json", id))
	if err := writeFileAtomic(dst, data, 0o600); err != nil {
		return "", err
	}
	return id, nil
}

func (r *FileRelay) Pull(ctx context.Context, req PullRequest) ([]Message, error) {
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
		_ = r.pruneLocked(now)
		out, _ := r.pullAvailableLocked(req.Mailbox, now, req.Limit)
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

func (r *FileRelay) Ack(ctx context.Context, req AckRequest) error {
	_ = ctx
	if err := req.Validate(); err != nil {
		return err
	}
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.pruneLocked(now)

	dir := r.mailboxDir(req.Mailbox)
	for i := range req.IDs {
		p := filepath.Join(dir, fmt.Sprintf("msg_%s.json", req.IDs[i]))
		_ = os.Remove(p)
	}
	return nil
}

func (r *FileRelay) mailboxDir(m MailboxID) string {
	return filepath.Join(r.baseDir, "mb_"+string(m))
}

func (r *FileRelay) pullAvailableLocked(mailbox MailboxID, now time.Time, limit int) ([]Message, error) {
	dir := r.mailboxDir(mailbox)
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Message{}, nil
		}
		return nil, err
	}
	type item struct {
		msg Message
		at  int64
	}
	items := make([]item, 0, len(ents))
	for _, ent := range ents {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.HasPrefix(name, "msg_") || !strings.HasSuffix(name, ".json") {
			continue
		}
		full := filepath.Join(dir, name)
		b, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		var rec fileStored
		if err := json.Unmarshal(b, &rec); err != nil {
			continue
		}
		if rec.Deadline > 0 && now.Unix() >= rec.Deadline {
			_ = os.Remove(full)
			continue
		}
		if rec.AvailAt > 0 && now.Unix() < rec.AvailAt {
			continue
		}
		if err := rec.Message.Validate(); err != nil {
			_ = os.Remove(full)
			continue
		}
		items = append(items, item{msg: rec.Message, at: rec.Message.ReceivedAt})
	}
	if len(items) == 0 {
		return []Message{}, nil
	}
	sort.Slice(items, func(i, j int) bool { return items[i].at < items[j].at })
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]Message, 0, len(items))
	for i := range items {
		out = append(out, items[i].msg)
	}
	return out, nil
}

func (r *FileRelay) pruneLocked(now time.Time) error {
	ents, err := os.ReadDir(r.baseDir)
	if err != nil {
		return err
	}
	for _, ent := range ents {
		if !ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.HasPrefix(name, "mb_") {
			continue
		}
		dir := filepath.Join(r.baseDir, name)
		msgs, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		kept := 0
		for _, m := range msgs {
			if m.IsDir() {
				continue
			}
			if !strings.HasPrefix(m.Name(), "msg_") || !strings.HasSuffix(m.Name(), ".json") {
				continue
			}
			full := filepath.Join(dir, m.Name())
			b, err := os.ReadFile(full)
			if err != nil {
				_ = os.Remove(full)
				continue
			}
			var rec fileStored
			if err := json.Unmarshal(b, &rec); err != nil {
				_ = os.Remove(full)
				continue
			}
			if rec.Deadline > 0 && now.Unix() >= rec.Deadline {
				_ = os.Remove(full)
				continue
			}
			kept++
		}
		if kept == 0 {
			_ = os.Remove(dir)
		}
	}
	return nil
}

func (r *FileRelay) mailboxPendingLocked(mailbox MailboxID) (int, error) {
	dir := r.mailboxDir(mailbox)
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	n := 0
	for _, ent := range ents {
		if ent.IsDir() {
			continue
		}
		if strings.HasPrefix(ent.Name(), "msg_") && strings.HasSuffix(ent.Name(), ".json") {
			n++
		}
	}
	return n, nil
}

func (r *FileRelay) totalPendingLocked() (int, error) {
	ents, err := os.ReadDir(r.baseDir)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, ent := range ents {
		if !ent.IsDir() {
			continue
		}
		if !strings.HasPrefix(ent.Name(), "mb_") {
			continue
		}
		dir := filepath.Join(r.baseDir, ent.Name())
		msgs, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, m := range msgs {
			if m.IsDir() {
				continue
			}
			if strings.HasPrefix(m.Name(), "msg_") && strings.HasSuffix(m.Name(), ".json") {
				total++
			}
		}
	}
	return total, nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "tmp_*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_ = tmp.Chmod(perm)
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil {
		_ = os.Remove(tmpName)
		return werr
	}
	if cerr != nil {
		_ = os.Remove(tmpName)
		return cerr
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}
