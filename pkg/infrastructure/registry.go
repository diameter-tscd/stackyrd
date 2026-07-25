package infrastructure

import (
	"fmt"
	"stackyrd/config"
	"stackyrd/pkg/logger"
	"sync"
)

type ComponentRegistry struct {
	components     map[string]InfrastructureComponent
	factories      map[string]ComponentFactory
	allNames       []string
	componentsMu   sync.RWMutex
	factoriesMu    sync.Mutex
}

var (
	globalRegistry *ComponentRegistry
	registryOnce   sync.Once
)

func GetGlobalRegistry() *ComponentRegistry {
	registryOnce.Do(func() {
		globalRegistry = &ComponentRegistry{}
	})
	return globalRegistry
}

func RegisterComponent(name string, factory ComponentFactory) {
	GetGlobalRegistry().Register(name, factory)
}

func (r *ComponentRegistry) Register(name string, factory ComponentFactory) {
	r.factoriesMu.Lock()
	defer r.factoriesMu.Unlock()
	if r.factories == nil {
		r.factories = make(map[string]ComponentFactory)
	}
	r.factories[name] = factory
}

func (r *ComponentRegistry) Initialize(cfg *config.Config, logger *logger.Logger) error {
	r.factoriesMu.Lock()
	defer r.factoriesMu.Unlock()

	r.allNames = make([]string, 0, len(r.factories))
	for name := range r.factories {
		r.allNames = append(r.allNames, name)
	}
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

func (r *ComponentRegistry) SetComponent(name string, component InfrastructureComponent) {
	r.componentsMu.Lock()
	defer r.componentsMu.Unlock()
	if r.components == nil {
		r.components = make(map[string]InfrastructureComponent)
	}
	r.components[name] = component
}

func (r *ComponentRegistry) RegisteredNames() []string {
	return r.allNames
}

func (r *ComponentRegistry) Get(name string) (InfrastructureComponent, bool) {
	r.componentsMu.RLock()
	defer r.componentsMu.RUnlock()
	comp, ok := r.components[name]
	return comp, ok
}

func (r *ComponentRegistry) GetAll() map[string]InfrastructureComponent {
	r.componentsMu.RLock()
	defer r.componentsMu.RUnlock()
	result := make(map[string]InfrastructureComponent, len(r.components))
	for k, v := range r.components {
		result[k] = v
	}
	return result
}

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
