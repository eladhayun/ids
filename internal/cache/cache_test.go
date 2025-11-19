package cache

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	cache := New()
	assert.NotNil(t, cache)
	assert.NotNil(t, cache.items)
	assert.Empty(t, cache.items)
}

func TestCache_SetAndGet(t *testing.T) {
	cache := New()

	// Test basic set and get
	cache.Set("key1", "value1", 10*time.Second)
	val, exists := cache.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", val)

	// Test non-existent key
	val, exists = cache.Get("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, val)
}

func TestCache_SetDifferentTypes(t *testing.T) {
	cache := New()

	// Test different data types
	cache.Set("string", "value", 10*time.Second)
	cache.Set("int", 42, 10*time.Second)
	cache.Set("bool", true, 10*time.Second)
	cache.Set("slice", []int{1, 2, 3}, 10*time.Second)
	cache.Set("map", map[string]int{"a": 1}, 10*time.Second)

	val, exists := cache.Get("string")
	assert.True(t, exists)
	assert.Equal(t, "value", val)

	val, exists = cache.Get("int")
	assert.True(t, exists)
	assert.Equal(t, 42, val)

	val, exists = cache.Get("bool")
	assert.True(t, exists)
	assert.Equal(t, true, val)

	val, exists = cache.Get("slice")
	assert.True(t, exists)
	assert.Equal(t, []int{1, 2, 3}, val)

	val, exists = cache.Get("map")
	assert.True(t, exists)
	assert.Equal(t, map[string]int{"a": 1}, val)
}

func TestCache_Expiration(t *testing.T) {
	cache := New()

	// Set with short TTL
	cache.Set("expiring", "value", 100*time.Millisecond)

	// Should exist immediately
	val, exists := cache.Get("expiring")
	assert.True(t, exists)
	assert.Equal(t, "value", val)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should not exist after expiration
	val, exists = cache.Get("expiring")
	assert.False(t, exists)
	assert.Nil(t, val)

	// Verify item is removed from cache
	cache.mutex.RLock()
	_, itemExists := cache.items["expiring"]
	cache.mutex.RUnlock()
	assert.False(t, itemExists)
}

func TestCache_UpdateValue(t *testing.T) {
	cache := New()

	// Set initial value
	cache.Set("key", "value1", 10*time.Second)
	val, exists := cache.Get("key")
	assert.True(t, exists)
	assert.Equal(t, "value1", val)

	// Update value
	cache.Set("key", "value2", 10*time.Second)
	val, exists = cache.Get("key")
	assert.True(t, exists)
	assert.Equal(t, "value2", val)
}

func TestCache_Delete(t *testing.T) {
	cache := New()

	// Set value
	cache.Set("key", "value", 10*time.Second)
	val, exists := cache.Get("key")
	assert.True(t, exists)
	assert.Equal(t, "value", val)

	// Delete
	cache.Delete("key")
	val, exists = cache.Get("key")
	assert.False(t, exists)
	assert.Nil(t, val)

	// Delete non-existent key (should not panic)
	cache.Delete("nonexistent")
}

func TestCache_Clear(t *testing.T) {
	cache := New()

	// Set multiple values
	cache.Set("key1", "value1", 10*time.Second)
	cache.Set("key2", "value2", 10*time.Second)
	cache.Set("key3", "value3", 10*time.Second)

	// Verify all exist
	_, exists1 := cache.Get("key1")
	_, exists2 := cache.Get("key2")
	_, exists3 := cache.Get("key3")
	assert.True(t, exists1)
	assert.True(t, exists2)
	assert.True(t, exists3)

	// Clear all
	cache.Clear()

	// Verify all are gone
	_, exists1 = cache.Get("key1")
	_, exists2 = cache.Get("key2")
	_, exists3 = cache.Get("key3")
	assert.False(t, exists1)
	assert.False(t, exists2)
	assert.False(t, exists3)

	// Verify items map is empty
	cache.mutex.RLock()
	assert.Empty(t, cache.items)
	cache.mutex.RUnlock()
}

func TestCache_ConcurrentAccess(t *testing.T) {
	cache := New()
	iterations := 100
	var wg sync.WaitGroup

	// Concurrent writes
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func(n int) {
			defer wg.Done()
			cache.Set("key", n, 10*time.Second)
		}(i)
	}
	wg.Wait()

	// Verify cache is still functional
	val, exists := cache.Get("key")
	assert.True(t, exists)
	assert.NotNil(t, val)

	// Concurrent reads and writes
	wg.Add(iterations * 3)
	for i := 0; i < iterations; i++ {
		// Writer
		go func(n int) {
			defer wg.Done()
			cache.Set("key", n, 10*time.Second)
		}(i)

		// Reader
		go func() {
			defer wg.Done()
			cache.Get("key")
		}()

		// Deleter
		go func(n int) {
			defer wg.Done()
			if n%10 == 0 {
				cache.Delete("key")
			}
		}(i)
	}
	wg.Wait()

	// Cache should still be functional
	cache.Set("final", "value", 10*time.Second)
	val, exists = cache.Get("final")
	assert.True(t, exists)
	assert.Equal(t, "value", val)
}

func TestCache_ConcurrentClear(t *testing.T) {
	cache := New()
	iterations := 50
	var wg sync.WaitGroup

	// Populate cache
	for i := 0; i < 100; i++ {
		cache.Set("key", i, 10*time.Second)
	}

	// Concurrent operations with clears
	wg.Add(iterations * 3)
	for i := 0; i < iterations; i++ {
		go func(n int) {
			defer wg.Done()
			cache.Set("key", n, 10*time.Second)
		}(i)

		go func() {
			defer wg.Done()
			cache.Get("key")
		}()

		go func(n int) {
			defer wg.Done()
			if n%5 == 0 {
				cache.Clear()
			}
		}(i)
	}
	wg.Wait()

	// Cache should still be functional
	cache.Set("test", "value", 10*time.Second)
	val, exists := cache.Get("test")
	assert.True(t, exists)
	assert.Equal(t, "value", val)
}

func TestCache_TTLVariations(t *testing.T) {
	cache := New()

	// Very short TTL
	cache.Set("short", "value", 1*time.Nanosecond)
	time.Sleep(10 * time.Millisecond)
	_, exists := cache.Get("short")
	assert.False(t, exists)

	// Long TTL
	cache.Set("long", "value", 1*time.Hour)
	val, exists := cache.Get("long")
	assert.True(t, exists)
	assert.Equal(t, "value", val)

	// Zero TTL (expires immediately)
	cache.Set("zero", "value", 0)
	time.Sleep(10 * time.Millisecond)
	_, exists = cache.Get("zero")
	assert.False(t, exists)

	// Negative TTL (expires in the past)
	cache.Set("negative", "value", -1*time.Second)
	_, exists = cache.Get("negative")
	assert.False(t, exists)
}

func TestCache_MultipleExpiredItems(t *testing.T) {
	cache := New()

	// Create multiple items with different expiration times
	cache.Set("expire1", "value1", 50*time.Millisecond)
	cache.Set("expire2", "value2", 100*time.Millisecond)
	cache.Set("expire3", "value3", 150*time.Millisecond)
	cache.Set("persist", "value_persist", 10*time.Second)

	// Wait for first item to expire
	time.Sleep(60 * time.Millisecond)
	_, exists1 := cache.Get("expire1")
	_, exists2 := cache.Get("expire2")
	_, exists3 := cache.Get("expire3")
	_, existsPersist := cache.Get("persist")
	assert.False(t, exists1)
	assert.True(t, exists2)
	assert.True(t, exists3)
	assert.True(t, existsPersist)

	// Wait for second item to expire
	time.Sleep(60 * time.Millisecond)
	_, exists2 = cache.Get("expire2")
	_, exists3 = cache.Get("expire3")
	_, existsPersist = cache.Get("persist")
	assert.False(t, exists2)
	assert.True(t, exists3)
	assert.True(t, existsPersist)

	// Wait for third item to expire
	time.Sleep(60 * time.Millisecond)
	_, exists3 = cache.Get("expire3")
	_, existsPersist = cache.Get("persist")
	assert.False(t, exists3)
	assert.True(t, existsPersist)
}

func TestCache_EmptyKey(t *testing.T) {
	cache := New()

	// Empty key should work like any other key
	cache.Set("", "empty_key_value", 10*time.Second)
	val, exists := cache.Get("")
	assert.True(t, exists)
	assert.Equal(t, "empty_key_value", val)

	cache.Delete("")
	_, exists = cache.Get("")
	assert.False(t, exists)
}

func TestCache_NilValue(t *testing.T) {
	cache := New()

	// Nil value should be storable
	cache.Set("nil", nil, 10*time.Second)
	val, exists := cache.Get("nil")
	assert.True(t, exists)
	assert.Nil(t, val)
}

func BenchmarkCache_Set(b *testing.B) {
	cache := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set("key", "value", 10*time.Second)
	}
}

func BenchmarkCache_Get(b *testing.B) {
	cache := New()
	cache.Set("key", "value", 10*time.Second)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("key")
	}
}

func BenchmarkCache_ConcurrentSetGet(b *testing.B) {
	cache := New()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				cache.Set("key", i, 10*time.Second)
			} else {
				cache.Get("key")
			}
			i++
		}
	})
}
