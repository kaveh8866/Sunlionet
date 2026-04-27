package replay_protection

import (
	"sync"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/cache_manager"
)

type MessageID = cache_manager.MessageID

type Protector struct {
	mu sync.Mutex

	bucketDur time.Duration
	windowDur time.Duration
	buckets   []map[MessageID]struct{}
	zero      time.Time
}

type Options struct {
	BucketDuration time.Duration
	WindowDuration time.Duration
}

func New(opts Options) *Protector {
	if opts.BucketDuration <= 0 {
		opts.BucketDuration = 10 * time.Second
	}
	if opts.WindowDuration <= 0 {
		opts.WindowDuration = 10 * time.Minute
	}
	if opts.WindowDuration < opts.BucketDuration {
		opts.WindowDuration = opts.BucketDuration
	}
	n := int(opts.WindowDuration / opts.BucketDuration)
	if n < 1 {
		n = 1
	}
	buckets := make([]map[MessageID]struct{}, n)
	for i := range buckets {
		buckets[i] = make(map[MessageID]struct{})
	}
	return &Protector{
		bucketDur: opts.BucketDuration,
		windowDur: opts.WindowDuration,
		buckets:   buckets,
	}
}

func (p *Protector) SeenOrAdd(id MessageID, now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rotateLocked(now)
	for i := range p.buckets {
		if _, ok := p.buckets[i][id]; ok {
			return true
		}
	}
	p.buckets[len(p.buckets)-1][id] = struct{}{}
	return false
}

func (p *Protector) Seen(id MessageID, now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rotateLocked(now)
	for i := range p.buckets {
		if _, ok := p.buckets[i][id]; ok {
			return true
		}
	}
	return false
}

func (p *Protector) rotateLocked(now time.Time) {
	if p.zero.IsZero() {
		p.zero = now
		return
	}
	elapsed := now.Sub(p.zero)
	if elapsed < p.bucketDur {
		return
	}
	shift := int(elapsed / p.bucketDur)
	if shift >= len(p.buckets) {
		for i := range p.buckets {
			clear(p.buckets[i])
		}
		p.zero = now
		return
	}
	for i := 0; i < shift; i++ {
		p.buckets = append(p.buckets[1:], make(map[MessageID]struct{}))
		p.zero = p.zero.Add(p.bucketDur)
	}
}
