// Package cache provides a tiny in-memory TTL cache used to avoid hammering
// javdb.com (and to remember "no result" verdicts for doujin items).
package cache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value   V
	expires time.Time
}

// TTL is a concurrency-safe map with per-entry expiry. The zero value is not
// usable; create one with New.
type TTL[K comparable, V any] struct {
	mu  sync.RWMutex
	m   map[K]entry[V]
	now func() time.Time
}

// New returns an empty TTL cache.
func New[K comparable, V any]() *TTL[K, V] {
	return &TTL[K, V]{m: make(map[K]entry[V]), now: time.Now}
}

// Get returns the cached value for key and whether it was present and unexpired.
func (c *TTL[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok || c.now().After(e.expires) {
		var zero V
		return zero, false
	}
	return e.value, true
}

// Set stores value for key with the given time-to-live.
func (c *TTL[K, V]) Set(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	c.m[key] = entry[V]{value: value, expires: c.now().Add(ttl)}
	c.mu.Unlock()
}
