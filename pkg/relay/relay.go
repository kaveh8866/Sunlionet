package relay

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"time"
)

type Relay interface {
	Push(ctx context.Context, req PushRequest) (MessageID, error)
	Pull(ctx context.Context, req PullRequest) ([]Message, error)
	Ack(ctx context.Context, req AckRequest) error
}

type PollOptions struct {
	Mailbox  MailboxID
	Limit    int
	WaitSec  int
	CycleSec int

	JitterMs int

	BackoffBaseMs int
	BackoffMaxMs  int

	Ack           bool
	AckDelayMsMax int
	AckBatchSec   int
	AckBatchMax   int

	CoverAckSec      int
	CoverAckCount    int
	CoverAckJitterMs int

	DecoyMailboxes    []MailboxID
	DecoyPullSec      int
	DecoyWaitSec      int
	DecoyLimit        int
	DecoyPullJitterMs int

	Handle func(ctx context.Context, msgs []Message) error
}

func (o PollOptions) normalize() (PollOptions, error) {
	out := o
	if out.Mailbox == "" {
		return PollOptions{}, errors.New("relay: mailbox required")
	}
	if out.Limit <= 0 {
		out.Limit = 50
	}
	if out.Limit > 200 {
		out.Limit = 200
	}
	if out.WaitSec <= 0 {
		out.WaitSec = 15
	}
	if out.WaitSec > 30 {
		out.WaitSec = 30
	}
	if out.CycleSec <= 0 {
		out.CycleSec = out.WaitSec
	}
	if out.CycleSec > 60 {
		out.CycleSec = 60
	}
	if out.JitterMs < 0 {
		out.JitterMs = 0
	}
	if out.JitterMs > 5000 {
		out.JitterMs = 5000
	}
	if out.BackoffBaseMs <= 0 {
		out.BackoffBaseMs = 250
	}
	if out.BackoffMaxMs <= 0 {
		out.BackoffMaxMs = 5000
	}
	if out.BackoffMaxMs < out.BackoffBaseMs {
		out.BackoffMaxMs = out.BackoffBaseMs
	}
	if out.BackoffMaxMs > 60000 {
		out.BackoffMaxMs = 60000
	}
	if out.AckDelayMsMax < 0 {
		out.AckDelayMsMax = 0
	}
	if out.AckDelayMsMax == 0 && out.Ack {
		out.AckDelayMsMax = 250
	}
	if out.AckDelayMsMax > 5000 {
		out.AckDelayMsMax = 5000
	}
	if out.AckBatchSec < 0 {
		out.AckBatchSec = 0
	}
	if out.AckBatchSec == 0 && out.Ack {
		out.AckBatchSec = 5
	}
	if out.AckBatchSec > 300 {
		out.AckBatchSec = 300
	}
	if out.AckBatchMax < 0 {
		out.AckBatchMax = 0
	}
	if out.AckBatchMax == 0 && out.Ack {
		out.AckBatchMax = 50
	}
	if out.AckBatchMax > 2000 {
		out.AckBatchMax = 2000
	}
	if out.CoverAckSec < 0 {
		out.CoverAckSec = 0
	}
	if out.CoverAckSec > 300 {
		out.CoverAckSec = 300
	}
	if out.CoverAckCount < 0 {
		out.CoverAckCount = 0
	}
	if out.CoverAckCount == 0 && out.CoverAckSec > 0 {
		out.CoverAckCount = 2
	}
	if out.CoverAckCount > 25 {
		out.CoverAckCount = 25
	}
	if out.CoverAckJitterMs < 0 {
		out.CoverAckJitterMs = 0
	}
	if out.CoverAckJitterMs == 0 && out.CoverAckSec > 0 {
		out.CoverAckJitterMs = 500
	}
	if out.CoverAckJitterMs > 5000 {
		out.CoverAckJitterMs = 5000
	}
	if len(out.DecoyMailboxes) > 0 {
		if len(out.DecoyMailboxes) > 10 {
			out.DecoyMailboxes = out.DecoyMailboxes[:10]
		}
		kept := make([]MailboxID, 0, len(out.DecoyMailboxes))
		seen := make(map[MailboxID]struct{}, len(out.DecoyMailboxes))
		for i := range out.DecoyMailboxes {
			mb := out.DecoyMailboxes[i]
			if mb == "" || mb == out.Mailbox {
				continue
			}
			if _, ok := seen[mb]; ok {
				continue
			}
			seen[mb] = struct{}{}
			kept = append(kept, mb)
		}
		out.DecoyMailboxes = kept
		if out.DecoyPullSec <= 0 {
			out.DecoyPullSec = 5
		}
		if out.DecoyPullSec > 300 {
			out.DecoyPullSec = 300
		}
		if out.DecoyWaitSec <= 0 {
			out.DecoyWaitSec = 1
		}
		if out.DecoyWaitSec > 30 {
			out.DecoyWaitSec = 30
		}
		if out.DecoyLimit <= 0 {
			out.DecoyLimit = 1
		}
		if out.DecoyLimit > 10 {
			out.DecoyLimit = 10
		}
		if out.DecoyPullJitterMs < 0 {
			out.DecoyPullJitterMs = 0
		}
		if out.DecoyPullJitterMs == 0 {
			out.DecoyPullJitterMs = 500
		}
		if out.DecoyPullJitterMs > 5000 {
			out.DecoyPullJitterMs = 5000
		}
	}
	if out.Handle == nil {
		return PollOptions{}, errors.New("relay: handle is required")
	}
	return out, nil
}

type Poller struct {
	Relay Relay
	Opts  PollOptions
}

func (p *Poller) Run(ctx context.Context) error {
	if p == nil || p.Relay == nil {
		return errors.New("relay: poller relay is nil")
	}
	opts, err := p.Opts.normalize()
	if err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	cycleDur := time.Duration(opts.CycleSec) * time.Second
	backoffBase := time.Duration(opts.BackoffBaseMs) * time.Millisecond
	backoffMax := time.Duration(opts.BackoffMaxMs) * time.Millisecond
	ackBatchDur := time.Duration(opts.AckBatchSec) * time.Second
	coverAckDur := time.Duration(opts.CoverAckSec) * time.Second
	decoyPullDur := time.Duration(opts.DecoyPullSec) * time.Second

	consecutiveErr := 0
	lastAckAt := time.Now()
	pending := make([]MessageID, 0, opts.AckBatchMax)
	pendingSet := make(map[MessageID]struct{}, opts.AckBatchMax)
	lastCoverAckAt := time.Now()
	lastDecoyPullAt := time.Now()
	decoyNext := 0

	flushAcks := func(ctx context.Context) {
		if !opts.Ack || len(pending) == 0 {
			return
		}
		if opts.AckDelayMsMax > 0 {
			_ = sleepCtx(ctx, jitterPositive(time.Duration(opts.AckDelayMsMax)*time.Millisecond))
		}
		ids := append([]MessageID(nil), pending...)
		pending = pending[:0]
		clear(pendingSet)
		_ = p.Relay.Ack(ctx, AckRequest{Mailbox: opts.Mailbox, IDs: ids})
		lastAckAt = time.Now()
	}

	sendCoverAck := func(ctx context.Context) {
		if coverAckDur <= 0 || opts.CoverAckCount <= 0 {
			return
		}
		ids := make([]MessageID, 0, opts.CoverAckCount)
		for i := 0; i < opts.CoverAckCount; i++ {
			id, err := newRandomMessageID()
			if err != nil {
				continue
			}
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			return
		}
		if opts.CoverAckJitterMs > 0 {
			_ = sleepCtx(ctx, jitterPositive(time.Duration(opts.CoverAckJitterMs)*time.Millisecond))
		}
		_ = p.Relay.Ack(ctx, AckRequest{Mailbox: opts.Mailbox, IDs: ids})
		lastCoverAckAt = time.Now()
	}

	doDecoyPull := func(ctx context.Context) {
		if decoyPullDur <= 0 || len(opts.DecoyMailboxes) == 0 {
			return
		}
		if time.Since(lastDecoyPullAt) < decoyPullDur {
			return
		}
		mb := opts.DecoyMailboxes[decoyNext%len(opts.DecoyMailboxes)]
		decoyNext++
		lastDecoyPullAt = time.Now()
		if opts.DecoyPullJitterMs > 0 {
			_ = sleepCtx(ctx, jitterPositive(time.Duration(opts.DecoyPullJitterMs)*time.Millisecond))
		}
		_, _ = p.Relay.Pull(ctx, PullRequest{Mailbox: mb, Limit: opts.DecoyLimit, WaitSec: opts.DecoyWaitSec})
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		cycleStart := time.Now()
		msgs, err := p.Relay.Pull(ctx, PullRequest{Mailbox: opts.Mailbox, Limit: opts.Limit, WaitSec: opts.WaitSec})
		if err != nil {
			consecutiveErr++
			sleepFor := expBackoff(backoffBase, backoffMax, consecutiveErr)
			sleepFor += jitterPositive(time.Duration(opts.JitterMs) * time.Millisecond)
			if err := sleepCtx(ctx, sleepFor); err != nil {
				return err
			}
			continue
		}
		consecutiveErr = 0

		if len(msgs) > 0 {
			if err := opts.Handle(ctx, msgs); err == nil && opts.Ack {
				for i := range msgs {
					id := msgs[i].ID
					if _, ok := pendingSet[id]; ok {
						continue
					}
					pendingSet[id] = struct{}{}
					pending = append(pending, id)
				}
				if opts.AckBatchMax > 0 && len(pending) >= opts.AckBatchMax {
					flushAcks(ctx)
				}
			}
		}

		if opts.Ack && len(pending) > 0 && ackBatchDur > 0 && time.Since(lastAckAt) >= ackBatchDur {
			flushAcks(ctx)
		}
		if coverAckDur > 0 && time.Since(lastCoverAckAt) >= coverAckDur {
			sendCoverAck(ctx)
		}
		doDecoyPull(ctx)

		elapsed := time.Since(cycleStart)
		sleepFor := cycleDur - elapsed
		sleepFor += jitterSigned(time.Duration(opts.JitterMs) * time.Millisecond)
		if sleepFor < 0 {
			sleepFor = 0
		}
		if err := sleepCtx(ctx, sleepFor); err != nil {
			return err
		}
	}
}

func expBackoff(base time.Duration, max time.Duration, n int) time.Duration {
	if base <= 0 {
		base = 250 * time.Millisecond
	}
	if max <= 0 {
		max = 5 * time.Second
	}
	if n <= 0 {
		return base
	}
	if n > 20 {
		n = 20
	}
	m := base << (n - 1)
	if m > max {
		return max
	}
	return m
}

func jitterPositive(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	n, err := randInt63n(int64(max) + 1)
	if err != nil {
		return 0
	}
	return time.Duration(n)
}

func jitterSigned(maxAbs time.Duration) time.Duration {
	if maxAbs <= 0 {
		return 0
	}
	n, err := randInt63n(int64(maxAbs) + 1)
	if err != nil {
		return 0
	}
	if n == 0 {
		return 0
	}
	signBit, err := randInt63n(2)
	if err != nil {
		return 0
	}
	if signBit == 0 {
		return -time.Duration(n)
	}
	return time.Duration(n)
}

func randInt63n(n int64) (int64, error) {
	if n <= 0 {
		return 0, fmt.Errorf("relay: invalid rand bound: %d", n)
	}
	v, err := rand.Int(rand.Reader, big.NewInt(n))
	if err != nil {
		return 0, err
	}
	return v.Int64(), nil
}

func newRandomMessageID() (MessageID, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return MessageID(base64.RawURLEncoding.EncodeToString(b[:])), nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	select {
	case <-ctx.Done():
		t.Stop()
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
