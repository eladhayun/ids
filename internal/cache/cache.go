package cache

import (
	"sync"
	"time"
)

// CacheItem represents a cached item with expiration
type CacheItem struct {
	Data      interface{}
	ExpiresAt time.Time
}

// Cache represents a simple in-memory cache
type Cache struct {
	items map[string]*CacheItem
	mutex sync.RWMutex
}

// New creates a new cache instance
func New() *Cache {
	return &Cache{
		items: make(map[string]*CacheItem),
	}
}

// Get retrieves an item from the cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// Check if item has expired
	if time.Now().After(item.ExpiresAt) {
		// Item has expired, remove it
		c.mutex.RUnlock()
		c.mutex.Lock()
		delete(c.items, key)
		c.mutex.Unlock()
		c.mutex.RLock()
		return nil, false
	}

	return item.Data, true
}

// Set stores an item in the cache with TTL
func (c *Cache) Set(key string, data interface{}, ttl time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items[key] = &CacheItem{
		Data:      data,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// Delete removes an item from the cache
func (c *Cache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.items, key)
}

// Clear removes all items from the cache
func (c *Cache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.items = make(map[string]*CacheItem)
}
