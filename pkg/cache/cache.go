package cache

import (
	"sync"
	"time"
)

type Item[T any] struct {
	Value      T
	Expiration int64
}

type Cache[T any] struct {
	items  map[string]Item[T]
	mu     sync.RWMutex
	stopCh chan struct{}
}

// New creates a new in-memory cache with a background cleanup goroutine
// that evicts expired items every 5 minutes. Call Close to stop the
// cleanup goroutine.
func New[T any]() *Cache[T] {
	c := &Cache[T]{
		items:  make(map[string]Item[T]),
		stopCh: make(chan struct{}),
	}
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.Cleanup()
			case <-c.stopCh:
				return
			}
		}
	}()
	return c
}

// Close stops the background cleanup goroutine.
func (c *Cache[T]) Close() {
	close(c.stopCh)
}

// Set adds an item to the cache with a TTL (duration).
// If ttl is 0, the item never expires.
func (c *Cache[T]) Set(key string, value T, ttl time.Duration) {
	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).UnixNano()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = Item[T]{
		Value:      value,
		Expiration: exp,
	}
}

// Get retrieves an item from the cache.
// Returns the value and true if found and not expired.
// Returns zero value and false otherwise.
func (c *Cache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, found := c.items[key]
	if !found {
		var zero T
		return zero, false
	}

	if item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
		var zero T
		return zero, false
	}

	return item.Value, true
}

// Delete removes an item from the cache
func (c *Cache[T]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Cleanup removes expired items. Run this in a goroutine for periodic cleanup.
func (c *Cache[T]) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()
	for k, v := range c.items {
		if v.Expiration > 0 && now > v.Expiration {
			delete(c.items, k)
		}
	}
}
