package infrastructure

import (
	"fmt"
	"stackyrd/config"
	"stackyrd/pkg/logger"
	"sync"
	"time"
)

// ComponentRegistry manages all infrastructure components.
// After boot the component and factory maps are write-once, so a
// regular map protected by sync.RWMutex is cheaper than sync.Map for
// the hot read path (no interface boxing/type assertions on every access).
type ComponentRegistry struct {
	components     map[string]InfrastructureComponent // write-once after boot
	factories      map[string]ComponentFactory        // write-once at init
	componentsMu   sync.RWMutex                       // guards components map
	factoriesMu    sync.Mutex                         // guards factories map (init phase only)
	cachedSnapshot map[string]InfrastructureComponent // TTL-cached GetAll copy; nil = stale
	cacheExpiry    time.Time
	cacheMu        sync.Mutex
	cacheTTL       time.Duration
}

// Global registry instance
var (
	globalRegistry *ComponentRegistry
	registryOnce   sync.Once
)

// GetGlobalRegistry returns the singleton registry instance
func GetGlobalRegistry() *ComponentRegistry {
	registryOnce.Do(func() {
		globalRegistry = &ComponentRegistry{
			cacheTTL: 2 * time.Second, // reduced copy frequency 4x from 500ms default
		}
	})
	return globalRegistry
}

// RegisterComponent registers a component factory with the global registry
func RegisterComponent(name string, factory ComponentFactory) {
	GetGlobalRegistry().Register(name, factory)
}

// Register registers a component factory (called from init() during registration phase)
func (r *ComponentRegistry) Register(name string, factory ComponentFactory) {
	r.factoriesMu.Lock()
	defer r.factoriesMu.Unlock()
	if r.factories == nil {
		r.factories = make(map[string]ComponentFactory)
	}
	r.factories[name] = factory
}

// Initialize creates and stores every registered component.  Called once at
// boot; after this all component writes are complete.
func (r *ComponentRegistry) Initialize(cfg *config.Config, logger *logger.Logger) error {
	r.factoriesMu.Lock()
	defer r.factoriesMu.Unlock()

	if r.components == nil {
		r.components = make(map[string]InfrastructureComponent)
	}
	for name, factory := range r.factories {
		component, err := factory(cfg, logger)
		if err != nil {
			logger.Error("Failed to initialize "+name, err)
			continue
		}
		if component != nil {
			r.components[name] = component
			logger.Info(name + " initialized")
		}
	}
	return nil
}

// SetComponent directly inserts a component into the registry after initialization.
// This is used by subsystems (e.g. plugins) that bootstrap after Initialize completes.
func (r *ComponentRegistry) SetComponent(name string, component InfrastructureComponent) {
	r.componentsMu.Lock()
	defer r.componentsMu.Unlock()
	if r.components == nil {
		r.components = make(map[string]InfrastructureComponent)
	}
	r.components[name] = component
	// Invalidate snapshot cache
	r.cacheMu.Lock()
	r.cachedSnapshot = nil
	r.cacheExpiry = time.Time{}
	r.cacheMu.Unlock()
}

// Get retrieves a component by name — RLock read path, no interface boxing.
func (r *ComponentRegistry) Get(name string) (InfrastructureComponent, bool) {
	r.componentsMu.RLock()
	defer r.componentsMu.RUnlock()
	comp, ok := r.components[name]
	return comp, ok
}

// GetAll returns a TTL-cached read-only snapshot of all components.
// Callers that only need component names should use maps.Keys on the result
// instead of requesting a full key+value copy.
func (r *ComponentRegistry) GetAll() map[string]InfrastructureComponent {
	// Fast path — cached snapshot still valid
	r.cacheMu.Lock()
	if time.Now().Before(r.cacheExpiry) && r.cachedSnapshot != nil {
		result := r.cachedSnapshot
		r.cacheMu.Unlock()
		return result
	}
	r.cacheMu.Unlock()

	// Slow path — rebuild snapshot.
	// Always re-take cacheMu as writer so the store+expiry update is atomic.
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	if time.Now().Before(r.cacheExpiry) && r.cachedSnapshot != nil {
		return r.cachedSnapshot
	}

	// Read lock-free: components map is write-once after boot (no concurrent writers).
	r.componentsMu.RLock()
	result := make(map[string]InfrastructureComponent, len(r.components))
	for k, v := range r.components {
		result[k] = v
	}
	r.componentsMu.RUnlock()

	r.cachedSnapshot = result
	r.cacheExpiry = time.Now().Add(r.cacheTTL)
	return result
}

// CloseAll closes all components and returns any errors.
func (r *ComponentRegistry) CloseAll() []error {
	r.componentsMu.RLock()
	names := make([]string, 0, len(r.components))
	for name := range r.components {
		names = append(names, name)
	}
	r.componentsMu.RUnlock()

	var errors []error
	for _, name := range names {
		r.componentsMu.RLock()
		comp, ok := r.components[name]
		r.componentsMu.RUnlock()
		if !ok {
			continue
		}
		if err := comp.Close(); err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", name, err))
		}
	}
	return errors
}
