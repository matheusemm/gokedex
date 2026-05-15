package pokecache

import (
	"sync"
	"time"
)

type Cache struct {
	entries map[string]cacheEntry
	mu      sync.Mutex
	ttl     time.Duration
}

type cacheEntry struct {
	val       []byte
	createdAt time.Time
}

func NewCache(ttl time.Duration) *Cache {
	cache := &Cache{
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
	}
	go cache.reapLoop()

	return cache
}

func (c *Cache) Add(key string, val []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	valCopy := make([]byte, len(val))
	copy(valCopy, val)

	c.entries[key] = cacheEntry{
		val:       valCopy,
		createdAt: time.Now(),
	}
}

func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if ok {
		valCopy := make([]byte, len(entry.val))
		copy(valCopy, entry.val)

		return valCopy, ok
	}
	return nil, ok
}

func (c *Cache) reapLoop() {
	ticker := time.NewTicker(c.ttl)
	for {
		now := <-ticker.C

		c.mu.Lock()

		for key, entry := range c.entries {
			if now.After(entry.createdAt.Add(c.ttl)) {
				delete(c.entries, key)
			}
		}

		c.mu.Unlock()
	}
}
