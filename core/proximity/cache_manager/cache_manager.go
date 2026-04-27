package cache_manager

import (
	"container/list"
	"sync"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
)

type MessageID [16]byte

type Entry struct {
	ID        MessageID
	Sender    identity_manager.NodeID
	Timestamp time.Time
	TTLSec    uint16
	Inserted  time.Time
	ExpiresAt time.Time
	Payload   []byte
}

type Cache struct {
	mu       sync.Mutex
	maxItems int
	items    map[MessageID]*list.Element
	order    *list.List
}

func New(maxItems int) *Cache {
	if maxItems <= 0 {
		maxItems = 512
	}
	return &Cache{
		maxItems: maxItems,
		items:    make(map[MessageID]*list.Element),
		order:    list.New(),
	}
}

func (c *Cache) Has(id MessageID, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[id]
	if !ok {
		return false
	}
	e := el.Value.(Entry)
	if !e.ExpiresAt.IsZero() && !now.Before(e.ExpiresAt) {
		c.order.Remove(el)
		delete(c.items, id)
		return false
	}
	return true
}

func (c *Cache) Put(e Entry, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !e.ExpiresAt.IsZero() && !now.Before(e.ExpiresAt) {
		return
	}
	if el, ok := c.items[e.ID]; ok {
		c.order.Remove(el)
		delete(c.items, e.ID)
	}
	cp := append([]byte(nil), e.Payload...)
	e.Payload = cp
	el := c.order.PushBack(e)
	c.items[e.ID] = el
	for c.order.Len() > c.maxItems {
		front := c.order.Front()
		if front == nil {
			break
		}
		old := front.Value.(Entry)
		delete(c.items, old.ID)
		c.order.Remove(front)
	}
}

func (c *Cache) Get(id MessageID, now time.Time) (Entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[id]
	if !ok {
		return Entry{}, false
	}
	e := el.Value.(Entry)
	if !e.ExpiresAt.IsZero() && !now.Before(e.ExpiresAt) {
		c.order.Remove(el)
		delete(c.items, id)
		return Entry{}, false
	}
	return e, true
}

func (c *Cache) Sweep(now time.Time) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	removed := 0
	for el := c.order.Front(); el != nil; {
		next := el.Next()
		e := el.Value.(Entry)
		if !e.ExpiresAt.IsZero() && !now.Before(e.ExpiresAt) {
			c.order.Remove(el)
			delete(c.items, e.ID)
			removed++
		}
		el = next
	}
	return removed
}

func (c *Cache) List(now time.Time, limit int) []Entry {
	c.mu.Lock()
	defer c.mu.Unlock()
	if limit <= 0 || limit > c.order.Len() {
		limit = c.order.Len()
	}
	out := make([]Entry, 0, limit)
	for el := c.order.Back(); el != nil && len(out) < limit; el = el.Prev() {
		e := el.Value.(Entry)
		if !e.ExpiresAt.IsZero() && !now.Before(e.ExpiresAt) {
			continue
		}
		out = append(out, e)
	}
	return out
}
