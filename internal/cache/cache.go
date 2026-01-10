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
	item, exists := c.items[key]
	if !exists {
		c.mutex.RUnlock()
		return nil, false
	}

	// Check if item has expired
	if time.Now().After(item.ExpiresAt) {
		c.mutex.RUnlock()
		// Item has expired, remove it with write lock
		c.mutex.Lock()
		delete(c.items, key)
		c.mutex.Unlock()
		return nil, false
	}

	data := item.Data
	c.mutex.RUnlock()
	return data, true
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

// EmbeddingCache constants
const (
	EmbeddingCacheTTL    = 5 * time.Minute // Cache embeddings for 5 minutes
	EmbeddingCachePrefix = "emb:"
)

// GetEmbedding retrieves a cached embedding for a query
func (c *Cache) GetEmbedding(query string) ([]float32, bool) {
	key := EmbeddingCachePrefix + query
	data, exists := c.Get(key)
	if !exists {
		return nil, false
	}

	embedding, ok := data.([]float32)
	if !ok {
		return nil, false
	}

	return embedding, true
}

// SetEmbedding stores an embedding in the cache
func (c *Cache) SetEmbedding(query string, embedding []float32) {
	key := EmbeddingCachePrefix + query
	c.Set(key, embedding, EmbeddingCacheTTL)
}

// EmbeddingCacheStats returns statistics about the embedding cache
func (c *Cache) EmbeddingCacheStats() (total int, embeddings int) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	total = len(c.items)
	for key := range c.items {
		if len(key) > len(EmbeddingCachePrefix) && key[:len(EmbeddingCachePrefix)] == EmbeddingCachePrefix {
			embeddings++
		}
	}
	return total, embeddings
}
