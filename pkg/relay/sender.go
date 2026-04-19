package relay

import (
	"context"
	"errors"
	"sync"
	"time"
)

type SendOptions struct {
	MinDelayMs int
	MaxDelayMs int

	BatchMax int
	QueueMax int

	InterPushJitterMs int

	HandleResult func(ctx context.Context, req PushRequest, id MessageID, err error)
}

func (o SendOptions) normalize() SendOptions {
	out := o
	if out.MinDelayMs < 0 {
		out.MinDelayMs = 0
	}
	if out.MaxDelayMs < 0 {
		out.MaxDelayMs = 0
	}
	if out.MinDelayMs == 0 && out.MaxDelayMs == 0 {
		out.MinDelayMs = 200
		out.MaxDelayMs = 5000
	}
	if out.MaxDelayMs == 0 {
		out.MaxDelayMs = out.MinDelayMs
	}
	if out.MaxDelayMs < out.MinDelayMs {
		out.MaxDelayMs = out.MinDelayMs
	}
	if out.MaxDelayMs > 60000 {
		out.MaxDelayMs = 60000
	}
	if out.BatchMax <= 0 {
		out.BatchMax = 8
	}
	if out.BatchMax > 200 {
		out.BatchMax = 200
	}
	if out.QueueMax <= 0 {
		out.QueueMax = 1000
	}
	if out.QueueMax > 100000 {
		out.QueueMax = 100000
	}
	if out.InterPushJitterMs < 0 {
		out.InterPushJitterMs = 0
	}
	if out.InterPushJitterMs > 5000 {
		out.InterPushJitterMs = 5000
	}
	return out
}

type PushShaper struct {
	Relay Relay
	Opts  SendOptions

	mu    sync.Mutex
	queue []pushItem
	wake  chan struct{}
}

type pushItem struct {
	ctx   context.Context
	req   PushRequest
	resCh chan pushResult
}

type pushResult struct {
	id  MessageID
	err error
}

func NewPushShaper(r Relay, opts SendOptions) *PushShaper {
	return &PushShaper{
		Relay: r,
		Opts:  opts,
		wake:  make(chan struct{}, 1),
	}
}

func (s *PushShaper) Enqueue(req PushRequest) error {
	if s == nil {
		return errors.New("relay: push shaper is nil")
	}
	if err := req.Validate(); err != nil {
		return err
	}
	opts := s.Opts.normalize()

	s.mu.Lock()
	if len(s.queue) >= opts.QueueMax {
		s.mu.Unlock()
		return errors.New("relay: push queue full")
	}
	s.queue = append(s.queue, pushItem{ctx: context.Background(), req: req, resCh: nil})
	s.mu.Unlock()

	select {
	case s.wake <- struct{}{}:
	default:
	}
	return nil
}

func (s *PushShaper) EnqueueWait(ctx context.Context, req PushRequest) (MessageID, error) {
	if s == nil {
		return "", errors.New("relay: push shaper is nil")
	}
	if err := req.Validate(); err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	opts := s.Opts.normalize()
	resCh := make(chan pushResult, 1)

	s.mu.Lock()
	if len(s.queue) >= opts.QueueMax {
		s.mu.Unlock()
		return "", errors.New("relay: push queue full")
	}
	s.queue = append(s.queue, pushItem{ctx: ctx, req: req, resCh: resCh})
	s.mu.Unlock()

	select {
	case s.wake <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-resCh:
		return res.id, res.err
	}
}

func (s *PushShaper) Run(ctx context.Context) error {
	if s == nil || s.Relay == nil {
		return errors.New("relay: push shaper relay is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	opts := s.Opts.normalize()
	defer s.failPending(ctx)

	var nextFlushAt time.Time
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		s.mu.Lock()
		qlen := len(s.queue)
		s.mu.Unlock()

		if qlen == 0 {
			nextFlushAt = time.Time{}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-s.wake:
			}
			continue
		}

		if nextFlushAt.IsZero() {
			d := time.Duration(opts.MinDelayMs) * time.Millisecond
			if opts.MaxDelayMs > opts.MinDelayMs {
				span := time.Duration(opts.MaxDelayMs-opts.MinDelayMs) * time.Millisecond
				d += jitterPositive(span)
			}
			nextFlushAt = time.Now().Add(d)
		}

		if qlen >= opts.BatchMax || time.Now().After(nextFlushAt) {
			items := s.dequeue(opts.BatchMax)
			for i := range items {
				item := items[i]
				if item.ctx == nil {
					item.ctx = ctx
				}
				select {
				case <-item.ctx.Done():
					if item.resCh != nil {
						item.resCh <- pushResult{err: item.ctx.Err()}
					}
					if opts.HandleResult != nil {
						opts.HandleResult(item.ctx, item.req, "", item.ctx.Err())
					}
					continue
				default:
				}

				id, err := s.Relay.Push(item.ctx, item.req)
				if item.resCh != nil {
					item.resCh <- pushResult{id: id, err: err}
				}
				if opts.HandleResult != nil {
					opts.HandleResult(item.ctx, item.req, id, err)
				}
				if opts.InterPushJitterMs > 0 && i+1 < len(items) {
					_ = sleepCtx(ctx, jitterPositive(time.Duration(opts.InterPushJitterMs)*time.Millisecond))
				}
			}
			nextFlushAt = time.Time{}
			continue
		}

		waitFor := time.Until(nextFlushAt)
		if waitFor <= 0 {
			continue
		}
		if waitFor > 250*time.Millisecond {
			waitFor = 250 * time.Millisecond
		}
		t := time.NewTimer(waitFor)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-s.wake:
			t.Stop()
		case <-t.C:
		}
	}
}

func (s *PushShaper) dequeue(max int) []pushItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queue) == 0 {
		return nil
	}
	if max <= 0 || max > len(s.queue) {
		max = len(s.queue)
	}
	out := append([]pushItem(nil), s.queue[:max]...)
	s.queue = s.queue[max:]
	return out
}

func (s *PushShaper) failPending(ctx context.Context) {
	err := context.Canceled
	if ctx != nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	s.mu.Lock()
	pending := append([]pushItem(nil), s.queue...)
	s.queue = nil
	s.mu.Unlock()
	for i := range pending {
		if pending[i].resCh != nil {
			pending[i].resCh <- pushResult{err: err}
		}
	}
}

type ShapedRelay struct {
	base   Relay
	shaper *PushShaper
	cancel context.CancelFunc
	done   chan error
}

func NewShapedRelay(base Relay, opts SendOptions) (*ShapedRelay, error) {
	if base == nil {
		return nil, errors.New("relay: base relay is nil")
	}
	ctx, cancel := context.WithCancel(context.Background())
	shaper := NewPushShaper(base, opts)
	done := make(chan error, 1)
	go func() { done <- shaper.Run(ctx) }()
	return &ShapedRelay{base: base, shaper: shaper, cancel: cancel, done: done}, nil
}

func (r *ShapedRelay) Close() error {
	if r == nil || r.cancel == nil || r.done == nil {
		return nil
	}
	r.cancel()
	<-r.done
	return nil
}

func (r *ShapedRelay) Push(ctx context.Context, req PushRequest) (MessageID, error) {
	if r == nil || r.shaper == nil {
		return "", errors.New("relay: shaped relay is nil")
	}
	return r.shaper.EnqueueWait(ctx, req)
}

func (r *ShapedRelay) Pull(ctx context.Context, req PullRequest) ([]Message, error) {
	if r == nil || r.base == nil {
		return nil, errors.New("relay: shaped relay is nil")
	}
	return r.base.Pull(ctx, req)
}

func (r *ShapedRelay) Ack(ctx context.Context, req AckRequest) error {
	if r == nil || r.base == nil {
		return errors.New("relay: shaped relay is nil")
	}
	return r.base.Ack(ctx, req)
}
