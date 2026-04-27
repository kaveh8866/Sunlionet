package low_bandwidth_queue

import (
	"errors"
	"sync"
	"time"
)

type Item struct {
	Payload   []byte
	Inserted  time.Time
	ExpiresAt time.Time
}

type Queue struct {
	mu sync.Mutex

	maxBytes int
	bytes    int
	items    []Item
}

func New(maxBytes int) *Queue {
	if maxBytes <= 0 {
		maxBytes = 8 * 1024
	}
	return &Queue{maxBytes: maxBytes}
}

func (q *Queue) Enqueue(payload []byte, now time.Time, ttl time.Duration) error {
	if len(payload) == 0 {
		return errors.New("empty payload")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.sweepLocked(now)
	if len(payload) > q.maxBytes {
		return errors.New("too large")
	}
	for q.bytes+len(payload) > q.maxBytes && len(q.items) > 0 {
		q.bytes -= len(q.items[0].Payload)
		q.items = q.items[1:]
	}
	cp := append([]byte(nil), payload...)
	q.items = append(q.items, Item{Payload: cp, Inserted: now, ExpiresAt: now.Add(ttl)})
	q.bytes += len(cp)
	return nil
}

func (q *Queue) Dequeue(now time.Time) (Item, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.sweepLocked(now)
	if len(q.items) == 0 {
		return Item{}, false
	}
	it := q.items[0]
	q.items = q.items[1:]
	q.bytes -= len(it.Payload)
	return it, true
}

func (q *Queue) Len(now time.Time) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.sweepLocked(now)
	return len(q.items)
}

func (q *Queue) sweepLocked(now time.Time) {
	n := 0
	for i := range q.items {
		if now.Before(q.items[i].ExpiresAt) {
			q.items[n] = q.items[i]
			n++
		} else {
			q.bytes -= len(q.items[i].Payload)
		}
	}
	q.items = q.items[:n]
}
