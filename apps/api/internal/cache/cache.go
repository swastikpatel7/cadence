// Package cache is a tiny in-process TTL cache used by the API in place
// of Redis for short-lived per-request state (OAuth state tokens, the
// clerk_user_id → internal UUID resolution cache).
//
// Why in-process: at v1 the API runs as a single instance, both the
// OAuth state TTL (~5 min) and the JWT cache (5 min) tolerate loss on
// restart, and dropping Redis removes a service from the deploy.
//
// If the API is ever scaled to multiple instances:
//   - OAuth state must move to a shared store (Postgres `oauth_states`
//     table with `expires_at` is the smallest change).
//   - JWT cache is already cache-aside; multi-instance just means each
//     instance warms its own copy. Acceptable.
package cache

import (
	"sync"
	"time"
)

// Cache is a goroutine-safe map with per-entry expiry. Expired entries
// are evicted lazily on Get/Take — there is no background reaper, so a
// long-idle key stays in memory until next access. That's fine for the
// two call sites today (both bounded by request volume).
type Cache[V any] struct {
	mu      sync.Mutex
	entries map[string]entry[V]
}

type entry[V any] struct {
	value    V
	expireAt time.Time
}

// New constructs an empty Cache.
func New[V any]() *Cache[V] {
	return &Cache[V]{entries: make(map[string]entry[V])}
}

// Set stores v under key for ttl. A zero ttl stores indefinitely.
func (c *Cache[V]) Set(key string, v V, ttl time.Duration) {
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.mu.Lock()
	c.entries[key] = entry[V]{value: v, expireAt: exp}
	c.mu.Unlock()
}

// Get returns the value and true if present and not expired. Expired
// entries are evicted on read.
func (c *Cache[V]) Get(key string) (V, bool) {
	var zero V
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return zero, false
	}
	if !e.expireAt.IsZero() && time.Now().After(e.expireAt) {
		delete(c.entries, key)
		return zero, false
	}
	return e.value, true
}

// Take is GetDelete: returns the value and true if present and not
// expired, removing the entry atomically. Used for one-shot tokens
// (OAuth state) so replay attacks fail.
func (c *Cache[V]) Take(key string) (V, bool) {
	var zero V
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return zero, false
	}
	delete(c.entries, key)
	if !e.expireAt.IsZero() && time.Now().After(e.expireAt) {
		return zero, false
	}
	return e.value, true
}

// Delete removes the entry, if any.
func (c *Cache[V]) Delete(key string) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}
