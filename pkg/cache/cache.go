package cache

import (
	"sync"
	"time"
)

type Item[T any] struct {
	Value      T
	Expiration int64
}

// Cache is the original in-memory cache implementation
type Cache[T any] struct {
	items     map[string]Item[T]
	mu        sync.RWMutex
	stopCh    chan struct{}
	closeOnce sync.Once
}

// ShardedCache uses sharding to reduce lock contention
type ShardedCache[T any] struct {
	shards []*sync.RWMutex
	items  []map[string]Item[T]
}

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

func NewShardedCache[T any]() *ShardedCache[T] {
	c := &ShardedCache[T]{
		shards: make([]*sync.RWMutex, 32),
		items:  make([]map[string]Item[T], 32),
	}
	for i := range c.shards {
		c.shards[i] = &sync.RWMutex{}
		c.items[i] = make(map[string]Item[T])
	}
	return c
}

func (c *ShardedCache[T]) Get(key string) (T, bool) {
	var zero T
	shard := c.shardIndex(key)
	c.shards[shard].RLock()
	item, found := c.items[shard][key]
	c.shards[shard].RUnlock()

	if !found {
		return zero, false
	}

	if item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
		c.shards[shard].Lock()
		item, found = c.items[shard][key]
		if found && item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
			delete(c.items[shard], key)
			item = Item[T]{}
			found = false
		}
		c.shards[shard].Unlock()
		if !found {
			return zero, false
		}
		return item.Value, true
	}

	return item.Value, true
}

func (c *ShardedCache[T]) Set(key string, value T, ttl time.Duration) {
	shard := c.shardIndex(key)
	c.shards[shard].Lock()
	defer c.shards[shard].Unlock()

	exp := int64(0)
	if ttl > 0 {
		exp = time.Now().Add(ttl).UnixNano()
	}

	c.items[shard][key] = Item[T]{
		Value:      value,
		Expiration: exp,
	}
}

func (c *ShardedCache[T]) Delete(key string) {
	shard := c.shardIndex(key)
	c.shards[shard].Lock()
	delete(c.items[shard], key)
	c.shards[shard].Unlock()
}

func (c *ShardedCache[T]) shardIndex(key string) int {
	h := 0
	for i := 0; i < len(key); i++ {
		h ^= int(key[i]) + 0x9e3779b9 + (h << 6) + (h >> 2)
	}
	return h % len(c.shards)
}

// Close stops the background cleanup goroutine
func (c *Cache[T]) Close() {
	c.closeOnce.Do(func() {
		if c.stopCh != nil {
			close(c.stopCh)
		}
	})
}

// Set adds an item to the cache with a TTL (duration)
func (c *Cache[T]) Set(key string, value T, ttl time.Duration) {
	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).UnixNano()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.items == nil {
		c.items = make(map[string]Item[T])
	}
	c.items[key] = Item[T]{
		Value:      value,
		Expiration: exp,
	}
}

// Get retrieves an item from the cache
func (c *Cache[T]) Get(key string) (T, bool) {
	var zero T
	c.mu.RLock()
	item, found := c.items[key]
	if !found {
		c.mu.RUnlock()
		return zero, false
	}

	if item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
		// Upgrade to write lock and re-verify: a concurrent Set may have
		// refreshed the value between our read and the lock acquisition.
		c.mu.RUnlock()
		c.mu.Lock()
		item, found = c.items[key]
		if found && item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
			delete(c.items, key)
			item = Item[T]{}
			found = false
		}
		c.mu.Unlock()
		if !found {
			return zero, false
		}
		return item.Value, true
	}
	c.mu.RUnlock()

	return item.Value, true
}

// Delete removes an item from the cache
func (c *Cache[T]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Cleanup removes expired items
func (c *Cache[T]) Cleanup() {
	now := time.Now().UnixNano()

	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range c.items {
		if v.Expiration > 0 && now > v.Expiration {
			delete(c.items, k)
		}
	}
}

func (c *ShardedCache[T]) Cleanup() {
	now := time.Now().UnixNano()

	for shard := 0; shard < len(c.shards); shard++ {
		c.shards[shard].Lock()
		for key, item := range c.items[shard] {
			if item.Expiration > 0 && now > item.Expiration {
				delete(c.items[shard], key)
			}
		}
		c.shards[shard].Unlock()
	}
}
