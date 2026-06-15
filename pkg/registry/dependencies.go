package registry

import (
	"sync"
	"sync/atomic"
	"time"
)

// Dependencies holds all infrastructure dependencies that services might need
type Dependencies struct {
	// Dynamic component store - no static declarations
	components map[string]interface{}
	mu         sync.RWMutex
	// TTL cache for GetAll() to avoid copying the entire map on every health check
	cachedAll    map[string]interface{}
	cacheExpiryN atomic.Int64 // UnixNano timestamp, 0 means expired
	cacheTTL     time.Duration
}

// NewDependencies creates a new dependencies container
func NewDependencies() *Dependencies {
	return &Dependencies{
		components: make(map[string]interface{}),
		cacheTTL:   2 * time.Second, // reduced copy frequency 4x from 500ms default
	}
}

// Set stores a component by name
func (d *Dependencies) Set(name string, component interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.components[name] = component
	// Invalidate cache on mutation
	d.cachedAll = nil
	d.cacheExpiryN.Store(0)
}

// Get retrieves a component by name
func (d *Dependencies) Get(name string) (interface{}, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	comp, ok := d.components[name]
	return comp, ok
}

// GetAll returns all registered components — returns a TTL-cached snapshot
// to avoid allocating and copying the entire map on every /health/dependencies call.
func (d *Dependencies) GetAll() map[string]interface{} {
	d.mu.RLock()
	if time.Now().UnixNano() < d.cacheExpiryN.Load() && d.cachedAll != nil {
		result := d.cachedAll
		d.mu.RUnlock()
		return result
	}
	d.mu.RUnlock()

	d.mu.Lock()
	defer d.mu.Unlock()
	// Re-check after acquiring write lock
	if time.Now().UnixNano() < d.cacheExpiryN.Load() && d.cachedAll != nil {
		return d.cachedAll
	}
	result := make(map[string]interface{}, len(d.components))
	for k, v := range d.components {
		result[k] = v
	}
	d.cachedAll = result
	d.cacheExpiryN.Store(time.Now().Add(d.cacheTTL).UnixNano())
	return result
}

// GetTyped retrieves component with type assertion
func GetTyped[T any](d *Dependencies, name string) (T, bool) {
	var zero T

	comp, ok := d.Get(name)
	if !ok {
		return zero, false
	}

	typed, ok := comp.(T)
	return typed, ok
}
