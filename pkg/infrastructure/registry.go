package infrastructure

import (
	"fmt"
	"stackyard/config"
	"stackyard/pkg/logger"
	"sync"
)

// ComponentRegistry manages all infrastructure components
type ComponentRegistry struct {
	components map[string]InfrastructureComponent
	factories  map[string]ComponentFactory
	mu         sync.RWMutex
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
			components: make(map[string]InfrastructureComponent),
			factories:  make(map[string]ComponentFactory),
		}
	})
	return globalRegistry
}

// RegisterComponent registers a component factory with the global registry
func RegisterComponent(name string, factory ComponentFactory) {
	GetGlobalRegistry().Register(name, factory)
}

// Register registers a component factory
func (r *ComponentRegistry) Register(name string, factory ComponentFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Initialize initializes all registered components
func (r *ComponentRegistry) Initialize(cfg *config.Config, logger *logger.Logger) error {
	r.mu.Lock()
	defer r.mu.Unlock()

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

// Get retrieves a component by name
func (r *ComponentRegistry) Get(name string) (InfrastructureComponent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	component, exists := r.components[name]
	return component, exists
}

// GetAll returns all components
func (r *ComponentRegistry) GetAll() map[string]InfrastructureComponent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]InfrastructureComponent)
	for k, v := range r.components {
		result[k] = v
	}
	return result
}

// CloseAll closes all components
func (r *ComponentRegistry) CloseAll() []error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errors []error
	for name, component := range r.components {
		if err := component.Close(); err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", name, err))
		}
	}
	return errors
}
