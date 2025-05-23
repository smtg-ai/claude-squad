package session

import (
	"sync"
	"time"
)

// CacheEntry represents a cached value with expiration
type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
}

// IsExpired checks if the cache entry has expired
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// Cache provides a thread-safe cache with TTL support
type Cache struct {
	data map[string]*CacheEntry
	mu   sync.RWMutex
	ttl  time.Duration
}

// NewCache creates a new cache with the specified TTL
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		data: make(map[string]*CacheEntry),
		ttl:  ttl,
	}
}

// Get retrieves a value from the cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	entry, exists := c.data[key]
	c.mu.RUnlock()
	
	if !exists || entry.IsExpired() {
		if exists {
			c.Delete(key) // Clean up expired entry
		}
		return nil, false
	}
	
	return entry.Value, true
}

// Set stores a value in the cache
func (c *Cache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.data[key] = &CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// Delete removes a value from the cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.data, key)
}

// Clear removes all entries from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.data = make(map[string]*CacheEntry)
}

// CleanExpired removes all expired entries
func (c *Cache) CleanExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	for key, entry := range c.data {
		if entry.IsExpired() {
			delete(c.data, key)
		}
	}
}

// Size returns the number of entries in the cache
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return len(c.data)
}

// Global caches for different types of data
var (
	tmuxContentCache *Cache
	gitStatusCache   *Cache
	diffStatsCache   *Cache
	cacheInitOnce    sync.Once
)

// initCaches initializes the global caches
func initCaches() {
	cacheInitOnce.Do(func() {
		tmuxContentCache = NewCache(5 * time.Second)   // Cache tmux content for 5 seconds
		gitStatusCache = NewCache(10 * time.Second)    // Cache git status for 10 seconds
		diffStatsCache = NewCache(10 * time.Second)    // Cache diff stats for 10 seconds
		
		// Start background cleanup routine
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			
			for range ticker.C {
				tmuxContentCache.CleanExpired()
				gitStatusCache.CleanExpired()
				diffStatsCache.CleanExpired()
			}
		}()
	})
}

// GetTmuxContentCache returns the global tmux content cache
func GetTmuxContentCache() *Cache {
	initCaches()
	return tmuxContentCache
}

// GetGitStatusCache returns the global git status cache
func GetGitStatusCache() *Cache {
	initCaches()
	return gitStatusCache
}

// GetDiffStatsCache returns the global diff stats cache
func GetDiffStatsCache() *Cache {
	initCaches()
	return diffStatsCache
}