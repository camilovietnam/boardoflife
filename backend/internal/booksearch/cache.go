package booksearch

import (
	"sync"
	"time"
)

// cache is a bounded in-memory cache whose entries expire after a TTL.
type cache[V any] struct {
	ttl     time.Duration
	maxSize int
	now     func() time.Time

	mu      sync.Mutex
	entries map[string]cacheEntry[V]
}

type cacheEntry[V any] struct {
	value   V
	expires time.Time
}

func newCache[V any](ttl time.Duration, maxSize int, now func() time.Time) *cache[V] {
	return &cache[V]{ttl: ttl, maxSize: maxSize, now: now, entries: map[string]cacheEntry[V]{}}
}

func (c *cache[V]) get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[key]
	if ok && !c.now().Before(e.expires) {
		delete(c.entries, key)
		ok = false
	}
	return e.value, ok
}

func (c *cache[V]) set(key string, value V) {
	now := c.now()

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.maxSize {
		c.evict(now)
	}
	c.entries[key] = cacheEntry[V]{value: value, expires: now.Add(c.ttl)}
}

// evict makes room for one entry: it drops expired entries first and, if the
// cache is still full, an arbitrary one. The caller must hold c.mu.
func (c *cache[V]) evict(now time.Time) {
	for k, e := range c.entries {
		if !now.Before(e.expires) {
			delete(c.entries, k)
		}
	}
	for k := range c.entries {
		if len(c.entries) < c.maxSize {
			return
		}
		delete(c.entries, k)
	}
}
