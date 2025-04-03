package cache

import (
	"sync"
)

// RenderCache provides a caching mechanism for rendered content
// to avoid expensive re-rendering operations when the underlying
// data hasn't changed.
type RenderCache struct {
	mu           sync.RWMutex
	cachedString string
	dirty        bool
	dimensions   struct {
		width  int
		height int
	}
}

// NewRenderCache creates a new RenderCache instance
func NewRenderCache() *RenderCache {
	return &RenderCache{
		dirty: true,
	}
}

// Get returns the cached string if it's valid, or calls the render function
// to generate a new one if the cache is dirty or dimensions have changed.
func (c *RenderCache) Get(width, height int, render func(width, height int) string) string {
	c.mu.RLock()
	if !c.dirty && c.dimensions.width == width && c.dimensions.height == height {
		result := c.cachedString
		c.mu.RUnlock()
		return result
	}
	c.mu.RUnlock()

	// Need to update the cache
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if !c.dirty && c.dimensions.width == width && c.dimensions.height == height {
		return c.cachedString
	}

	// Update dimensions and cached string
	c.dimensions.width = width
	c.dimensions.height = height
	c.cachedString = render(width, height)
	c.dirty = false

	return c.cachedString
}

// Invalidate marks the cache as dirty, forcing a re-render on next Get call
func (c *RenderCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dirty = true
}
