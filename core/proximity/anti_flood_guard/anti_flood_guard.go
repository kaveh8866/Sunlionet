package anti_flood_guard

import (
	"sync"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
)

type tokenBucket struct {
	capacity float64
	rate     float64
	tokens   float64
	last     time.Time
}

func newBucket(capacity float64, rate float64, now time.Time) tokenBucket {
	return tokenBucket{
		capacity: capacity,
		rate:     rate,
		tokens:   capacity,
		last:     now,
	}
}

func (b *tokenBucket) refill(now time.Time) {
	if b.last.IsZero() {
		b.last = now
	}
	dt := now.Sub(b.last).Seconds()
	if dt > 0 {
		b.tokens += dt * b.rate
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
		b.last = now
	}
}

func (b *tokenBucket) can(cost float64) bool {
	return b.tokens >= cost
}

func (b *tokenBucket) consume(cost float64) {
	b.tokens -= cost
}

type Guard struct {
	mu sync.Mutex

	global tokenBucket
	per    map[identity_manager.NodeID]tokenBucket

	perCapacity float64
	perRate     float64
}

type Options struct {
	GlobalCapacity float64
	GlobalRate     float64
	PerCapacity    float64
	PerRate        float64
}

func New(opts Options, now time.Time) *Guard {
	if opts.GlobalCapacity <= 0 {
		opts.GlobalCapacity = 60
	}
	if opts.GlobalRate <= 0 {
		opts.GlobalRate = 30
	}
	if opts.PerCapacity <= 0 {
		opts.PerCapacity = 10
	}
	if opts.PerRate <= 0 {
		opts.PerRate = 4
	}
	return &Guard{
		global:      newBucket(opts.GlobalCapacity, opts.GlobalRate, now),
		per:         make(map[identity_manager.NodeID]tokenBucket),
		perCapacity: opts.PerCapacity,
		perRate:     opts.PerRate,
	}
}

func (g *Guard) Allow(sender identity_manager.NodeID, now time.Time, cost float64) bool {
	if cost <= 0 {
		cost = 1
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	b := g.per[sender]
	if b.capacity == 0 && b.rate == 0 {
		b = newBucket(g.perCapacity, g.perRate, now)
	}

	g.global.refill(now)
	b.refill(now)
	if !g.global.can(cost) || !b.can(cost) {
		g.per[sender] = b
		return false
	}
	g.global.consume(cost)
	b.consume(cost)
	g.per[sender] = b
	return true
}

func (g *Guard) AllowGlobal(now time.Time, cost float64) bool {
	if cost <= 0 {
		cost = 1
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.global.refill(now)
	if !g.global.can(cost) {
		return false
	}
	g.global.consume(cost)
	return true
}
