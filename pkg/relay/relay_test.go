package relay

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryRelayPushPullAck(t *testing.T) {
	r := NewMemoryRelay()
	ctx := context.Background()

	mb := MailboxID("mb1")
	id, err := r.Push(ctx, PushRequest{Mailbox: mb, Envelope: Envelope("e1")})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	msgs, err := r.Pull(ctx, PullRequest{Mailbox: mb, Limit: 10})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].ID != id {
		t.Fatalf("id mismatch")
	}
	if err := r.Ack(ctx, AckRequest{Mailbox: mb, IDs: []MessageID{id}}); err != nil {
		t.Fatalf("Ack: %v", err)
	}
	msgs, err = r.Pull(ctx, PullRequest{Mailbox: mb, Limit: 10})
	if err != nil {
		t.Fatalf("Pull after ack: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after ack, got %d", len(msgs))
	}
}

func TestMemoryRelayTTL(t *testing.T) {
	r := NewMemoryRelay()
	ctx := context.Background()

	mb := MailboxID("mbttl")
	_, err := r.Push(ctx, PushRequest{Mailbox: mb, Envelope: Envelope("e"), TTLSec: 1})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	time.Sleep(1200 * time.Millisecond)
	msgs, err := r.Pull(ctx, PullRequest{Mailbox: mb})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after ttl, got %d", len(msgs))
	}
}

func TestMemoryRelayDelay(t *testing.T) {
	r := NewMemoryRelay()
	ctx := context.Background()

	mb := MailboxID("mbdelay")
	_, err := r.Push(ctx, PushRequest{Mailbox: mb, Envelope: Envelope("e"), DelaySec: 1})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	msgs, err := r.Pull(ctx, PullRequest{Mailbox: mb, Limit: 10})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages before delay, got %d", len(msgs))
	}
	time.Sleep(1100 * time.Millisecond)
	msgs, err = r.Pull(ctx, PullRequest{Mailbox: mb, Limit: 10})
	if err != nil {
		t.Fatalf("Pull after delay: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message after delay, got %d", len(msgs))
	}
}

func TestMemoryRelayLongPoll(t *testing.T) {
	r := NewMemoryRelay()
	mb := MailboxID("mbwait")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(150 * time.Millisecond)
		_, _ = r.Push(context.Background(), PushRequest{Mailbox: mb, Envelope: Envelope("e")})
	}()

	start := time.Now()
	msgs, err := r.Pull(ctx, PullRequest{Mailbox: mb, Limit: 10, WaitSec: 1})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("expected Pull to wait, returned too quickly")
	}
}

type fakeRelay struct {
	push func(ctx context.Context, req PushRequest) (MessageID, error)
	pull func(ctx context.Context, req PullRequest) ([]Message, error)
	ack  func(ctx context.Context, req AckRequest) error
}

func (r *fakeRelay) Push(ctx context.Context, req PushRequest) (MessageID, error) {
	if r.push == nil {
		return "", nil
	}
	return r.push(ctx, req)
}
func (r *fakeRelay) Pull(ctx context.Context, req PullRequest) ([]Message, error) {
	return r.pull(ctx, req)
}
func (r *fakeRelay) Ack(ctx context.Context, req AckRequest) error {
	if r.ack == nil {
		return nil
	}
	return r.ack(ctx, req)
}

func TestPollerConstantCadence(t *testing.T) {
	var starts []time.Time
	fr := &fakeRelay{
		pull: func(ctx context.Context, req PullRequest) ([]Message, error) {
			starts = append(starts, time.Now())
			return []Message{}, nil
		},
	}
	p := &Poller{
		Relay: fr,
		Opts: PollOptions{
			Mailbox:  MailboxID("mb"),
			WaitSec:  1,
			CycleSec: 1,
			JitterMs: 0,
			Handle:   func(ctx context.Context, msgs []Message) error { return nil },
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3200*time.Millisecond)
	defer cancel()
	err := p.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if len(starts) < 3 {
		t.Fatalf("expected at least 3 pulls, got %d", len(starts))
	}
	for i := 1; i < len(starts); i++ {
		d := starts[i].Sub(starts[i-1])
		if d < 900*time.Millisecond {
			t.Fatalf("expected cadence ~1s, got %v", d)
		}
	}
}

func TestPollerBackoffOnError(t *testing.T) {
	var starts []time.Time
	fail := 2
	fr := &fakeRelay{
		pull: func(ctx context.Context, req PullRequest) ([]Message, error) {
			starts = append(starts, time.Now())
			if fail > 0 {
				fail--
				return nil, errors.New("fail")
			}
			return []Message{}, nil
		},
	}
	p := &Poller{
		Relay: fr,
		Opts: PollOptions{
			Mailbox:       MailboxID("mb"),
			WaitSec:       1,
			CycleSec:      1,
			JitterMs:      0,
			BackoffBaseMs: 200,
			BackoffMaxMs:  800,
			Handle:        func(ctx context.Context, msgs []Message) error { return nil },
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	_ = p.Run(ctx)
	if len(starts) < 3 {
		t.Fatalf("expected at least 3 pulls, got %d", len(starts))
	}
	if starts[1].Sub(starts[0]) < 180*time.Millisecond {
		t.Fatalf("expected >= ~200ms backoff, got %v", starts[1].Sub(starts[0]))
	}
	if starts[2].Sub(starts[1]) < 350*time.Millisecond {
		t.Fatalf("expected >= ~400ms backoff, got %v", starts[2].Sub(starts[1]))
	}
}

func TestPollerAcksAfterHandle(t *testing.T) {
	var ackCalls int
	fr := &fakeRelay{
		pull: func(ctx context.Context, req PullRequest) ([]Message, error) {
			return []Message{{ID: MessageID("m1"), Mailbox: req.Mailbox, Envelope: Envelope("e"), ReceivedAt: time.Now().Unix()}}, nil
		},
		ack: func(ctx context.Context, req AckRequest) error {
			ackCalls++
			if len(req.IDs) != 1 || req.IDs[0] != MessageID("m1") {
				t.Fatalf("unexpected ack ids: %+v", req.IDs)
			}
			return nil
		},
	}
	p := &Poller{
		Relay: fr,
		Opts: PollOptions{
			Mailbox:       MailboxID("mb"),
			WaitSec:       1,
			CycleSec:      1,
			JitterMs:      0,
			Ack:           true,
			AckDelayMsMax: 1,
			AckBatchSec:   1,
			AckBatchMax:   1,
			Handle:        func(ctx context.Context, msgs []Message) error { return nil },
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = p.Run(ctx)
	if ackCalls == 0 {
		t.Fatalf("expected ack to be called")
	}
}

func TestPollerAckBatching(t *testing.T) {
	var acked []MessageID
	step := 0
	fr := &fakeRelay{
		pull: func(ctx context.Context, req PullRequest) ([]Message, error) {
			step++
			switch step {
			case 1:
				return []Message{{ID: MessageID("m1"), Mailbox: req.Mailbox, Envelope: Envelope("e1"), ReceivedAt: time.Now().Unix()}}, nil
			case 2:
				return []Message{{ID: MessageID("m2"), Mailbox: req.Mailbox, Envelope: Envelope("e2"), ReceivedAt: time.Now().Unix()}}, nil
			default:
				return []Message{}, nil
			}
		},
		ack: func(ctx context.Context, req AckRequest) error {
			acked = append(acked, req.IDs...)
			return nil
		},
	}
	p := &Poller{
		Relay: fr,
		Opts: PollOptions{
			Mailbox:       MailboxID("mb"),
			WaitSec:       1,
			CycleSec:      1,
			JitterMs:      0,
			Ack:           true,
			AckDelayMsMax: 0,
			AckBatchSec:   10,
			AckBatchMax:   2,
			Handle:        func(ctx context.Context, msgs []Message) error { return nil },
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1300*time.Millisecond)
	defer cancel()
	_ = p.Run(ctx)
	if len(acked) != 2 {
		t.Fatalf("expected 2 acked ids, got %d (%v)", len(acked), acked)
	}
}

func TestPollerCoverAckWhenIdle(t *testing.T) {
	var ackCalls int
	fr := &fakeRelay{
		pull: func(ctx context.Context, req PullRequest) ([]Message, error) {
			return []Message{}, nil
		},
		ack: func(ctx context.Context, req AckRequest) error {
			ackCalls++
			if len(req.IDs) == 0 {
				t.Fatalf("expected non-empty cover ack ids")
			}
			return nil
		},
	}
	p := &Poller{
		Relay: fr,
		Opts: PollOptions{
			Mailbox:          MailboxID("mb"),
			WaitSec:          1,
			CycleSec:         1,
			JitterMs:         0,
			CoverAckSec:      1,
			CoverAckCount:    2,
			CoverAckJitterMs: 0,
			Handle:           func(ctx context.Context, msgs []Message) error { return nil },
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1300*time.Millisecond)
	defer cancel()
	_ = p.Run(ctx)
	if ackCalls == 0 {
		t.Fatalf("expected at least 1 cover ack call")
	}
}

func TestPollerDecoyPulls(t *testing.T) {
	var pulled []MailboxID
	fr := &fakeRelay{
		pull: func(ctx context.Context, req PullRequest) ([]Message, error) {
			pulled = append(pulled, req.Mailbox)
			return []Message{}, nil
		},
	}
	p := &Poller{
		Relay: fr,
		Opts: PollOptions{
			Mailbox:           MailboxID("mb"),
			WaitSec:           1,
			CycleSec:          1,
			JitterMs:          0,
			DecoyMailboxes:    []MailboxID{"d1", "d2"},
			DecoyPullSec:      1,
			DecoyWaitSec:      1,
			DecoyLimit:        1,
			DecoyPullJitterMs: 0,
			Handle:            func(ctx context.Context, msgs []Message) error { return nil },
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1300*time.Millisecond)
	defer cancel()
	_ = p.Run(ctx)

	var primary int
	var decoy int
	for i := range pulled {
		if pulled[i] == MailboxID("mb") {
			primary++
			continue
		}
		if pulled[i] == MailboxID("d1") || pulled[i] == MailboxID("d2") {
			decoy++
		}
	}
	if primary == 0 {
		t.Fatalf("expected primary pulls")
	}
	if decoy == 0 {
		t.Fatalf("expected at least 1 decoy pull, got pulled=%v", pulled)
	}
}

func TestPushShaperDelayAndBatch(t *testing.T) {
	var pushedAt []time.Time
	fr := &fakeRelay{
		push: func(ctx context.Context, req PushRequest) (MessageID, error) {
			pushedAt = append(pushedAt, time.Now())
			return MessageID("id"), nil
		},
	}
	ps := NewPushShaper(fr, SendOptions{
		MinDelayMs:        150,
		MaxDelayMs:        150,
		BatchMax:          10,
		QueueMax:          10,
		InterPushJitterMs: 0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	go func() { _ = ps.Run(ctx) }()

	start := time.Now()
	if err := ps.Enqueue(PushRequest{Mailbox: MailboxID("mb"), Envelope: Envelope("e1")}); err != nil {
		t.Fatalf("enqueue 1: %v", err)
	}
	if err := ps.Enqueue(PushRequest{Mailbox: MailboxID("mb"), Envelope: Envelope("e2")}); err != nil {
		t.Fatalf("enqueue 2: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	if len(pushedAt) < 2 {
		t.Fatalf("expected 2 pushes, got %d", len(pushedAt))
	}
	if pushedAt[0].Sub(start) < 130*time.Millisecond {
		t.Fatalf("expected delayed push, got %v", pushedAt[0].Sub(start))
	}
	if pushedAt[1].Sub(pushedAt[0]) > 80*time.Millisecond {
		t.Fatalf("expected batched flush timing, delta=%v", pushedAt[1].Sub(pushedAt[0]))
	}
}

func TestPushShaperFlushOnBatchMax(t *testing.T) {
	var pushedAt []time.Time
	fr := &fakeRelay{
		push: func(ctx context.Context, req PushRequest) (MessageID, error) {
			pushedAt = append(pushedAt, time.Now())
			return MessageID("id"), nil
		},
	}
	ps := NewPushShaper(fr, SendOptions{
		MinDelayMs:        1000,
		MaxDelayMs:        1000,
		BatchMax:          2,
		QueueMax:          10,
		InterPushJitterMs: 0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go func() { _ = ps.Run(ctx) }()

	start := time.Now()
	_ = ps.Enqueue(PushRequest{Mailbox: MailboxID("mb"), Envelope: Envelope("e1")})
	_ = ps.Enqueue(PushRequest{Mailbox: MailboxID("mb"), Envelope: Envelope("e2")})
	time.Sleep(250 * time.Millisecond)
	if len(pushedAt) == 0 {
		t.Fatalf("expected early flush on batch max")
	}
	if pushedAt[0].Sub(start) > 220*time.Millisecond {
		t.Fatalf("expected flush before delay timeout, got %v", pushedAt[0].Sub(start))
	}
}

func TestShapedRelayPushBlocksUntilSent(t *testing.T) {
	var pushedAt []time.Time
	fr := &fakeRelay{
		push: func(ctx context.Context, req PushRequest) (MessageID, error) {
			pushedAt = append(pushedAt, time.Now())
			return MessageID("id"), nil
		},
	}
	sr, err := NewShapedRelay(fr, SendOptions{
		MinDelayMs:        150,
		MaxDelayMs:        150,
		BatchMax:          10,
		QueueMax:          10,
		InterPushJitterMs: 0,
	})
	if err != nil {
		t.Fatalf("NewShapedRelay: %v", err)
	}
	defer func() { _ = sr.Close() }()

	start := time.Now()
	id, err := sr.Push(context.Background(), PushRequest{Mailbox: MailboxID("mb"), Envelope: Envelope("e1")})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if id == "" {
		t.Fatalf("expected id")
	}
	if time.Since(start) < 130*time.Millisecond {
		t.Fatalf("expected Push to block for delay, returned too quickly")
	}
	if len(pushedAt) != 1 {
		t.Fatalf("expected 1 push, got %d", len(pushedAt))
	}
}
